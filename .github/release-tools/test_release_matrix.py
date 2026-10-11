import argparse
import hashlib
import importlib.util
import io
import json
import os
from pathlib import Path
import shutil
import subprocess
import sys
import tarfile
import tempfile
import unittest
import zipfile
from unittest.mock import patch

import yaml

ROOT = Path(__file__).resolve().parents[2]
spec = importlib.util.spec_from_file_location('release_matrix', Path(__file__).with_name('release_matrix.py'))
release = importlib.util.module_from_spec(spec)
spec.loader.exec_module(release)


class ReleaseMatrixTest(unittest.TestCase):
    def setUp(self):
        self.previous = Path.cwd()
        self.temp = tempfile.TemporaryDirectory()
        self.addCleanup(self.temp.cleanup)
        os.chdir(self.temp.name)
        self.addCleanup(os.chdir, self.previous)
        for name in ('.goreleaser.yaml', '.goreleaser.simple.yaml'):
            shutil.copyfile(ROOT / name, name)
        Path('backend/cmd/server').mkdir(parents=True)
        release.VERSION_FILE.write_text('9.8.7\n')
        for name, path in release.legal_paths().items():
            path.parent.mkdir(parents=True, exist_ok=True)
            path.write_text(f'{name}: copyright and license fixture\n')

    def fixture_artifacts(self, simple=False):
        directory = Path('release-input')
        directory.mkdir()
        with patch.object(subprocess, 'check_output', return_value='a' * 40 + '\n'):
            release.materials(argparse.Namespace(sha='a' * 40, version='9.8.7'))
        content = {name: path.read_bytes() for name, path in release.legal_paths().items()}
        content['SOURCE.json'] = Path('SOURCE.json').read_bytes()
        for target in release.targets(simple):
            name = release.archive_name('9.8.7', target)
            archive = directory / name
            binary = 'sub2api.exe' if target['goos'] == 'windows' else 'sub2api'
            files = {binary: b'fixture', **content}
            if target['goos'] != 'windows':
                with tarfile.open(archive, 'w:gz') as out:
                    for filename, data in files.items():
                        info = tarfile.TarInfo(filename)
                        info.size = len(data)
                        info.mode = 0o755 if filename == binary else 0o644
                        out.addfile(info, io.BytesIO(data))
            else:
                with zipfile.ZipFile(archive, 'w') as out:
                    for filename, data in files.items():
                        out.writestr(filename, data)
            metadata = {'version': '9.8.7', 'sha': 'a' * 40, 'target': target,
                        'archive': name, 'sha256': release.sha256(archive)}
            (directory / f"manifest-{target['goos']}-{target['goarch']}.json").write_text(json.dumps(metadata))
        return argparse.Namespace(input='release-input', version='9.8.7', sha='a' * 40, simple=simple, output='contexts')

    def test_full_and_simple_matrix_match_existing_targets(self):
        full = release.targets()
        self.assertEqual(len(full), 5)
        self.assertNotIn({'goos': 'windows', 'goarch': 'arm64'}, full)
        self.assertEqual(release.targets(True), [{'goos': 'linux', 'goarch': 'amd64'}])

    def test_leaf_keeps_packaging_and_selects_only_one_target(self):
        original = release.config()
        release.generate_config(argparse.Namespace(mode='build', simple=False, goos='darwin', goarch='arm64', output='leaf.yaml'))
        leaf = yaml.safe_load(Path('leaf.yaml').read_text())
        self.assertEqual(leaf['builds'][0]['goos'], ['darwin'])
        self.assertEqual(leaf['builds'][0]['goarch'], ['arm64'])
        self.assertEqual(leaf['builds'][0]['ignore'], [])
        self.assertEqual(leaf['archives'], original['archives'])
        self.assertEqual(leaf['release'], original['release'])
        self.assertFalse(leaf['dockers'])
        self.assertIn('{{ .Env.RELEASE_DATE }}', '\n'.join(leaf['builds'][0]['ldflags']))

    def test_publication_config_has_no_compilation_or_docker_work(self):
        for simple in (False, True):
            with self.subTest(simple=simple):
                original = release.config(simple)
                release.generate_config(argparse.Namespace(mode='publish', simple=simple, output='publisher.yaml'))
                data = yaml.safe_load(Path('publisher.yaml').read_text())
                self.assertTrue(data['builds'][0]['skip'])
                self.assertFalse(data['archives'])
                self.assertFalse(data['dockers'])
                self.assertEqual(data['release']['header'], original['release']['header'])
                self.assertEqual(data['release']['footer'], original['release']['footer'])
                if simple:
                    self.assertTrue(data['checksum']['disable'])
                    self.assertTrue(data['release']['skip_upload'])
                else:
                    self.assertEqual(data['checksum']['extra_files'], data['release']['extra_files'])

    def test_collect_and_verify_hash_and_source_binding(self):
        args = self.fixture_artifacts()
        release.verify(args)
        file = next(Path(args.input).glob('*.tar.gz'))
        file.write_bytes(b'corrupted')
        with self.assertRaisesRegex(ValueError, 'checksum mismatch'):
            release.verify(args)

    def test_missing_extra_and_wrong_commit_artifacts_are_rejected(self):
        args = self.fixture_artifacts(True)
        args.sha = 'b' * 40
        with self.assertRaises(ValueError):
            release.verify(args)
        args.sha = 'a' * 40
        Path('release-input/unexpected').write_text('not an asset')
        with self.assertRaises(ValueError):
            release.verify(args)
        Path('release-input/unexpected').unlink()
        next(Path('release-input').glob('*.tar.gz')).unlink()
        with self.assertRaises(FileNotFoundError):
            release.verify(args)

    def test_windows_archive_rejects_non_regular_license(self):
        args = self.fixture_artifacts()
        path = next(Path(args.input).glob('*.zip'))
        with zipfile.ZipFile(path) as archive:
            content = {name: archive.read(name) for name in archive.namelist()}
        with zipfile.ZipFile(path, 'w') as archive:
            for name, data in content.items():
                info = zipfile.ZipInfo(name)
                if name == 'NOTICE':
                    info.external_attr = (0o020644 << 16)  # 特殊文件不能作为许可材料
                archive.writestr(info, data)
        manifest = Path(args.input) / 'manifest-windows-amd64.json'
        metadata = json.loads(manifest.read_text())
        metadata['sha256'] = release.sha256(path)
        manifest.write_text(json.dumps(metadata))
        with self.assertRaisesRegex(ValueError, '普通文件: NOTICE'):
            release.verify(args)

    def test_linux_context_preserves_binary_executable_mode(self):
        args = self.fixture_artifacts()
        Path('Dockerfile.goreleaser').write_text('FROM scratch\nCOPY sub2api /sub2api\n')
        Path('deploy').mkdir()
        Path('deploy/docker-entrypoint.sh').write_text('#!/bin/sh\nexec /app/sub2api\n')
        Path('backend/resources').mkdir()
        Path('backend/resources/data').write_text('fixture')
        release.contexts(args)
        for arch in ('amd64', 'arm64'):
            binary = Path('contexts') / arch / 'sub2api'
            self.assertEqual(binary.read_bytes(), b'fixture')
            self.assertEqual(binary.stat().st_mode & 0o777, 0o755)
            for name in release.MATERIAL_FILES:
                path = Path('contexts') / arch / ('backend/internal/web/dist/' + name if name == 'THIRD_PARTY_LICENSES.txt' else name)
                self.assertTrue(path.read_bytes().strip())
            self.assertEqual(json.loads((binary.parent / 'SOURCE.json').read_text())['sha'], 'a' * 40)

    def test_materials_require_matching_source_and_complete_licenses(self):
        args = argparse.Namespace(sha='a' * 40, version='9.8.7')
        with patch.object(subprocess, 'check_output', return_value='b' * 40):
            with self.assertRaisesRegex(ValueError, '源码 SHA'):
                release.materials(args)
        Path('NOTICE').write_text('')
        with patch.object(subprocess, 'check_output', return_value='a' * 40):
            with self.assertRaisesRegex(ValueError, 'NOTICE'):
                release.materials(args)

    def test_archive_rejects_missing_license_and_mismatched_source_even_with_new_checksum(self):
        args = self.fixture_artifacts(True)
        path = next(Path(args.input).glob('*.tar.gz'))
        content = release.archive_materials(path, {'goos': 'linux', 'goarch': 'amd64'})
        for change in ('missing', 'source', 'license', 'duplicate', 'symlink'):
            with self.subTest(change=change):
                files = {**content, 'sub2api': b'fixture'}
                if change == 'missing':
                    del files['NOTICE']
                elif change == 'source':
                    metadata = json.loads(files['SOURCE.json'])
                    metadata['sha'] = 'b' * 40
                    files['SOURCE.json'] = json.dumps(metadata).encode()
                elif change == 'license':
                    files['LICENSE'] = b'changed copyright'
                with tarfile.open(path, 'w:gz') as out:
                    for name, data in files.items():
                        info = tarfile.TarInfo(name)
                        info.size = len(data)
                        if name == 'NOTICE' and change == 'symlink':
                            info.type = tarfile.SYMTYPE
                            info.linkname = '../NOTICE'
                            info.size = 0
                        out.addfile(info, io.BytesIO(data))
                        if name == 'NOTICE' and change == 'duplicate':
                            out.addfile(info, io.BytesIO(data))
                manifest = next(Path(args.input).glob('manifest-*.json'))
                metadata = json.loads(manifest.read_text())
                metadata['sha256'] = release.sha256(path)
                manifest.write_text(json.dumps(metadata))
                with self.assertRaises(ValueError):
                    release.verify(args)

    def test_plan_requires_a_tag_for_publication(self):
        args = argparse.Namespace(ref='main', dry_run=False, simple=False)
        with patch.object(subprocess, 'check_output', return_value='a' * 40 + '\n'):
            with self.assertRaisesRegex(ValueError, 'version tag'):
                release.plan(args)
        args.ref = 'v9.8.7'
        with patch.object(subprocess, 'check_output', side_effect=['a' * 40 + '\n', 'b' * 40 + '\n']):
            with self.assertRaisesRegex(ValueError, 'does not match'):
                release.plan(args)

    def test_dry_run_plan_resolves_matrix_without_a_new_tag(self):
        with patch.dict(os.environ, {'GITHUB_OUTPUT': 'outputs', 'GITHUB_REPOSITORY_OWNER': 'ExampleOwner'}), patch.object(subprocess, 'check_output', return_value='a' * 40 + '\n'):
            release.plan(argparse.Namespace(ref='feature/matrix', dry_run=True, simple=False))
        output = dict(line.split('=', 1) for line in Path('outputs').read_text().splitlines())
        self.assertEqual(output['dry_run'], 'true')
        self.assertEqual(output['owner_lower'], 'exampleowner')
        self.assertEqual(len(json.loads(output['matrix'])['include']), 5)

    def test_docker_commands_do_not_publish_during_dry_run(self):
        fake_bin = Path('bin')
        fake_bin.mkdir()
        docker = fake_bin / 'docker'
        docker.write_text('#!/bin/sh\nprintf "%s\\n" "$*" >> "$DOCKER_LOG"\n')
        docker.chmod(0o755)
        env = {**os.environ, 'PATH': str(fake_bin.resolve()) + os.pathsep + os.environ['PATH'],
               'DOCKER_LOG': str(Path('docker.log').resolve()), 'RUNNER_TEMP': self.temp.name,
               'RELEASE_VERSION': '9.8.7', 'RELEASE_SHA': 'a' * 40, 'GITHUB_REPOSITORY': 'ExampleOwner/sub2api',
               'DRY_RUN': 'true', 'SIMPLE_RELEASE': 'false', 'DOCKERHUB_USERNAME': 'skip'}
        subprocess.run(['bash', str(ROOT / '.github/release-tools/release-images.sh')], env=env, check=True)
        log = Path('docker.log').read_text()
        self.assertEqual(log.count('buildx build'), 2)
        self.assertIn('linux/arm64', log)
        self.assertNotIn('--push', log)
        self.assertNotIn('imagetools', log)
        self.assertNotIn('skip/sub2api', log)
        self.assertIn('ghcr.io/exampleowner/sub2api', log)


    def test_published_full_and_simple_image_tags(self):
        fake_bin = Path('bin')
        fake_bin.mkdir()
        docker = fake_bin / 'docker'
        docker.write_text('#!/bin/sh\nprintf "%s\\n" "$*" >> "$DOCKER_LOG"\n')
        docker.chmod(0o755)
        for simple in (False, True):
            with self.subTest(simple=simple):
                log_path = Path(f'docker-{simple}.log').resolve()
                env = {**os.environ, 'PATH': str(fake_bin.resolve()) + os.pathsep + os.environ['PATH'],
                       'DOCKER_LOG': str(log_path), 'RUNNER_TEMP': self.temp.name,
                       'RELEASE_VERSION': '9.8.7', 'RELEASE_SHA': 'a' * 40, 'GITHUB_REPOSITORY': 'ExampleOwner/sub2api',
                       'DRY_RUN': 'false', 'SIMPLE_RELEASE': str(simple).lower(), 'DOCKERHUB_USERNAME': 'fixturehub'}
                subprocess.run(['bash', str(ROOT / '.github/release-tools/release-images.sh')], env=env, check=True)
                log = log_path.read_text()
                self.assertIn('--push', log)
                self.assertEqual(log.count('buildx build'), 1 if simple else 2)
                if simple:
                    self.assertNotIn('fixturehub', log)
                    self.assertNotIn('imagetools', log)
                    self.assertIn('ghcr.io/exampleowner/sub2api:latest', log)
                else:
                    self.assertEqual(log.count('imagetools create'), 2)
                    self.assertIn('fixturehub/sub2api:9.8', log)
                    self.assertIn('ghcr.io/exampleowner/sub2api:9', log)

    def test_personal_image_push_is_after_build_and_final_gate_and_recovery_never_rebuilds(self):
        fake_bin = Path('bin')
        fake_bin.mkdir()
        docker = fake_bin / 'docker'
        docker.write_text('#!/bin/sh\nprintf "%s\\n" "$*" >> "$DOCKER_LOG"\n')
        docker.chmod(0o755)
        python = fake_bin / 'python3'
        python.write_text(f'''#!{sys.executable}
import json, os, sys
if len(sys.argv) > 2 and sys.argv[1].endswith('/personal_release.py'):
    with open(os.environ['DOCKER_LOG'], 'a') as out:
        out.write(sys.argv[2] + '\\n')
    if sys.argv[2] == 'authorize':
        if os.environ['GATE_FAIL'] == 'true':
            raise SystemExit('personal 基础已变化')
        print(json.dumps({{'reuse_image': os.environ['REUSE_IMAGE'] == 'true'}}))
    else:
        print(json.dumps({{'digest': 'sha256:' + 'd' * 64}}))
else:
    os.execv({sys.executable!r}, [{sys.executable!r}, *sys.argv[1:]])
''')
        python.chmod(0o755)
        for reuse, gate_fail in ((False, False), (True, False), (False, True)):
            with self.subTest(reuse=reuse, gate_fail=gate_fail):
                log_path = Path(f'personal-{reuse}-{gate_fail}.log').resolve()
                env = {**os.environ, 'PATH': str(fake_bin.resolve()) + os.pathsep + os.environ['PATH'],
                       'DOCKER_LOG': str(log_path), 'RUNNER_TEMP': self.temp.name,
                       'RELEASE_VERSION': '9.8.7-klno.3-tps.1', 'RELEASE_SHA': 'a' * 40,
                       'RELEASE_TAG': 'v9.8.7-klno.3-tps.1', 'GITHUB_REPOSITORY': 'ccisnoxx/sub2api',
                       'DRY_RUN': 'false', 'SIMPLE_RELEASE': 'true', 'REUSE_IMAGE': str(reuse).lower(),
                       'GATE_FAIL': str(gate_fail).lower()}
                result = subprocess.run(['bash', str(ROOT / '.github/release-tools/release-images.sh')], env=env,
                                        stdout=subprocess.PIPE, stderr=subprocess.PIPE, text=True)
                log = log_path.read_text()
                self.assertEqual(result.returncode, 1 if gate_fail else 0)
                self.assertNotIn('--push', log)
                self.assertNotIn('-tps.1-amd64', log)
                self.assertNotIn('--tag ghcr.io/ccisnoxx/sub2api:latest --load', log)
                if gate_fail:
                    self.assertNotIn('\npush ', log)
                    self.assertNotIn('imagetools', log)
                elif reuse:
                    self.assertNotIn('buildx build', log)
                    self.assertNotIn('\npush ', log)
                    self.assertIn('imagetools create', log)
                else:
                    self.assertLess(log.index('buildx build'), log.index('authorize'))
                    self.assertLess(log.index('authorize'), log.index('\npush '))
                    self.assertLess(log.index('\npush '), log.index('imagetools create'))



if __name__ == '__main__':
    unittest.main()
