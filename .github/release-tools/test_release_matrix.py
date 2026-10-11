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

    def fixture_artifacts(self, simple=False):
        directory = Path('release-input')
        directory.mkdir()
        for target in release.targets(simple):
            name = release.archive_name('9.8.7', target)
            archive = directory / name
            if target['goos'] == 'linux':
                with tarfile.open(archive, 'w:gz') as out:
                    info = tarfile.TarInfo('sub2api')
                    info.size = 7
                    info.mode = 0o755
                    out.addfile(info, io.BytesIO(b'fixture'))
            else:
                archive.write_bytes(b'fixture archive')
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
        Path('release-evidence').mkdir()
        evidence = [{'glob': 'release-evidence/example.json', 'name_template': 'run-1234-attempt-1-example.json'}]
        Path('release-evidence/publish-assets.json').write_text(json.dumps(evidence))
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
                self.assertEqual(data['release']['mode'], 'append')
                if simple:
                    self.assertTrue(data['checksum']['disable'])
                    self.assertFalse(data['release']['skip_upload'])
                    self.assertEqual(data['release']['extra_files'], evidence)
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

    def image_commands(self, simple=False, dry_run=False, personal=False, reuse=False, gate_fail=False, proof_fail=False,
                       gate_fail_at=0, digest_changed=False):
        fake_bin = Path('bin')
        fake_bin.mkdir(exist_ok=True)
        (Path(self.temp.name) / 'release-tools').mkdir(exist_ok=True)
        shutil.copy2(ROOT / '.github/release-tools/supply-chain-tools.json', Path(self.temp.name) / 'release-tools')
        for tool in ('docker', 'oras'):
            executable = fake_bin / tool
            executable.write_text('#!/bin/sh\nprintf "%s %s\\n" "' + tool + '" "$*" >> "$DOCKER_LOG"\nif [ "$1" = resolve ]; then printf "sha256:dddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddd\\n"; fi\n')
            executable.chmod(0o755)
        python = fake_bin / 'python3'
        python.write_text(f'''#!{sys.executable}
import json, os, sys
if len(sys.argv) > 2 and sys.argv[1].endswith(('/personal_release.py', '/supply_chain.py')):
    with open(os.environ['DOCKER_LOG'], 'a') as out:
        out.write(sys.argv[2] + '\\n')
    if sys.argv[2] == 'authorize':
        if '/release-guard/release-tools/' not in sys.argv[1]:
            raise SystemExit('没有使用 main 控制面 guard')
        count = open(os.environ['DOCKER_LOG']).read().splitlines().count('authorize')
        if os.environ['GATE_FAIL'] == 'true' or count == int(os.environ['GATE_FAIL_AT']):
            raise SystemExit('personal 基础已变化')
        digest = ('e' if os.environ['DIGEST_CHANGED'] == 'true' and count == 2 else 'd') * 64
        print(json.dumps({{'reuse_image': os.environ['REUSE_IMAGE'] == 'true', 'digest': 'sha256:' + digest}}))
    elif sys.argv[2] == 'image':
        if os.environ['PROOF_FAIL'] == 'true':
            raise SystemExit('镜像证明缺失')
        print('sha256:' + 'd' * 64)
    else:
        print(json.dumps({{'digest': 'sha256:' + 'd' * 64}}))
else:
    os.execv({sys.executable!r}, [{sys.executable!r}, *sys.argv[1:]])
''')
        python.chmod(0o755)
        log_path = Path('docker.log').resolve()
        if log_path.exists():
            log_path.unlink()
        env = {**os.environ, 'PATH': str(fake_bin.resolve()) + os.pathsep + os.environ['PATH'],
               'DOCKER_LOG': str(log_path), 'RUNNER_TEMP': self.temp.name,
               'RELEASE_VERSION': '9.8.7-klno.3-tps.1', 'RELEASE_SHA': 'a' * 40,
               'RELEASE_DATE': '2026-10-11T12:00:00Z', 'RELEASE_TAG': 'v9.8.7-klno.3-tps.1',
               'GITHUB_REPOSITORY': 'ccisnoxx/sub2api' if personal else 'ExampleOwner/sub2api',
               'DRY_RUN': str(dry_run).lower(), 'SIMPLE_RELEASE': str(simple).lower(),
               'DOCKERHUB_USERNAME': 'skip', 'REUSE_IMAGE': str(reuse).lower(),
               'GATE_FAIL': str(gate_fail).lower(), 'PROOF_FAIL': str(proof_fail).lower(),
               'GATE_FAIL_AT': str(gate_fail_at), 'DIGEST_CHANGED': str(digest_changed).lower()}
        for name in ('GH_TOKEN', 'GITHUB_TOKEN', 'DOCKERHUB_TOKEN'):
            env.pop(name, None)
        result = subprocess.run(['bash', str(ROOT / '.github/release-tools/release-images.sh')], env=env,
                                stdout=subprocess.PIPE, stderr=subprocess.PIPE, text=True)
        return result, log_path.read_text()

    def test_dry_run_generates_proofs_for_each_platform_without_registry_writes(self):
        result, log = self.image_commands(dry_run=True)
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertEqual(log.count('buildx build'), 2)
        self.assertEqual(log.count('\nimage\n'), 2)
        self.assertIn('linux/arm64', log)
        self.assertIn('--attest type=sbom,generator=', log)
        self.assertIn('--attest type=provenance,mode=max', log)
        self.assertIn('--tag ghcr.io/exampleowner/sub2api:9.8.7-klno.3-tps.1-amd64', log)
        self.assertNotIn('oras ', log)
        self.assertNotIn('--push', log)
        self.assertNotIn('--load', log)

    def test_published_full_and_simple_image_tags(self):
        for simple in (False, True):
            with self.subTest(simple=simple):
                result, log = self.image_commands(simple=simple)
                self.assertEqual(result.returncode, 0, result.stderr)
                self.assertEqual(log.count('buildx build'), 1 if simple else 2)
                self.assertEqual(log.count('oras cp --from-oci-layout'), 1 if simple else 2)
                self.assertIn('oras cp --to-oci-layout', log)
                self.assertNotIn('--load', log)
                if simple:
                    self.assertNotIn('imagetools create', log)
                    self.assertIn('oras tag ghcr.io/exampleowner/sub2api@', log)
                else:
                    self.assertEqual(log.count('imagetools create'), 1)
                    self.assertIn('9.8.7-klno.3-tps.1-amd64', log)
                    self.assertIn('9.8.7-klno.3-tps.1-arm64', log)
                    self.assertIn(' latest 9.8 9', log)

    def test_personal_oci_copy_preserves_guard_order_and_existing_image_recovery(self):
        for reuse, gate_fail, proof_fail in ((False, False, False), (True, False, False), (False, True, False), (False, False, True)):
            with self.subTest(reuse=reuse, gate_fail=gate_fail, proof_fail=proof_fail):
                result, log = self.image_commands(simple=True, personal=True, reuse=reuse, gate_fail=gate_fail, proof_fail=proof_fail)
                self.assertEqual(result.returncode, 1 if gate_fail or proof_fail else 0, result.stderr)
                self.assertNotIn('--load', log)
                if gate_fail or proof_fail:
                    self.assertNotIn('oras cp --from-oci-layout', log)
                    self.assertNotIn('oras tag', log)
                elif reuse:
                    self.assertNotIn('buildx build', log)
                    self.assertNotIn('oras cp --from-oci-layout', log)
                    self.assertIn('oras cp --to-oci-layout', log)
                    self.assertIn('oras tag', log)
                else:
                    self.assertLess(log.index('buildx build'), log.index('\nimage\n'))
                    self.assertLess(log.index('\nimage\n'), log.index('authorize'))
                    self.assertLess(log.index('authorize'), log.index('oras cp --from-oci-layout'))
                    self.assertLess(log.index('oras cp --to-oci-layout'), log.index('oras tag'))

    def test_personal_changes_or_digest_replacement_after_push_prevent_latest(self):
        for change in ('source_or_ci', 'digest'):
            with self.subTest(change=change):
                result, log = self.image_commands(simple=True, personal=True,
                    gate_fail_at=2 if change == 'source_or_ci' else 0, digest_changed=change == 'digest')
                self.assertEqual(result.returncode, 1, result.stderr)
                self.assertEqual(log.splitlines().count('authorize'), 2)
                self.assertIn('oras cp --from-oci-layout', log)
                self.assertNotIn('oras tag', log)

    def test_personal_dry_run_needs_no_publication_guard_or_registry_permissions(self):
        result, log = self.image_commands(simple=True, dry_run=True, personal=True, gate_fail=True)
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertEqual(log.count('buildx build'), 1)
        self.assertNotIn('authorize', log)
        self.assertNotIn('verify-image', log)
        self.assertNotIn('oras ', log)


if __name__ == '__main__':
    unittest.main()
