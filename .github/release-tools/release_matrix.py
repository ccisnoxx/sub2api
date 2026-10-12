#!/usr/bin/env python3
"""Split GoReleaser builds across runners without requiring GoReleaser Pro."""
import argparse
import hashlib
import itertools
import json
import os
from pathlib import Path
import re
import shutil
import stat
import subprocess
import tarfile
import zipfile
from datetime import datetime, timezone

import yaml

FULL_CONFIG = Path('.goreleaser.yaml')
SIMPLE_CONFIG = Path('.goreleaser.simple.yaml')
VERSION_FILE = Path('backend/cmd/server/VERSION')
VERSION_RE = re.compile(r'\d+\.\d+\.\d+(?:-[0-9A-Za-z.-]+)?')
SOURCE_URL = 'https://github.com/ccisnoxx/sub2api'
LEGAL_FILES = ('LICENSE', 'LICENSE-GPL-3.0.txt', 'NOTICE', 'THIRD_PARTY_LICENSES.txt')
MATERIAL_FILES = (*LEGAL_FILES, 'SOURCE.json')


def legal_paths():
    return {name: Path('backend/internal/web/dist') / name if name == 'THIRD_PARTY_LICENSES.txt'
            else Path(name) for name in LEGAL_FILES}


def source_metadata(sha, version, digests):
    if not re.fullmatch(r'[0-9a-f]{40}', sha) or not VERSION_RE.fullmatch(version):
        raise ValueError('源码 SHA 或版本格式无效')
    return {'sha': sha, 'version': version, 'repository': SOURCE_URL,
            'source': f'{SOURCE_URL}/tree/{sha}',
            'source_archive': f'{SOURCE_URL}/archive/{sha}.tar.gz',
            'licenses': digests}


def materials(args):
    actual = subprocess.check_output(['git', 'rev-parse', 'HEAD'], text=True).strip()
    if actual != args.sha:
        raise ValueError('许可材料必须来自所选源码 SHA')
    digests = {}
    for name, path in legal_paths().items():
        if not path.is_file() or not path.read_bytes().strip():
            raise ValueError(f'缺少非空许可材料: {name}')
        digests[name] = sha256(path)
    Path('SOURCE.json').write_text(json.dumps(source_metadata(args.sha, args.version, digests),
                                            indent=2, ensure_ascii=False) + '\n')


def archive_materials(path, target):
    """只读取固定根路径的普通文件，避免声明来自另一份源码或不完整产物。"""
    binary = 'sub2api.exe' if target['goos'] == 'windows' else 'sub2api'
    required = (*MATERIAL_FILES, binary)
    result = {}
    if path.suffix == '.zip':
        with zipfile.ZipFile(path) as archive:
            for name in required:
                members = [m for m in archive.infolist() if m.filename in (name, './' + name)]
                if (len(members) != 1 or members[0].is_dir()
                        or stat.S_IFMT(members[0].external_attr >> 16) not in (0, stat.S_IFREG)):
                    raise ValueError(f'归档必须包含唯一普通文件: {name}')
                if name != binary:
                    result[name] = archive.read(members[0])
    else:
        with tarfile.open(path, 'r:gz') as archive:
            for name in required:
                members = [m for m in archive.getmembers() if m.name in (name, './' + name)]
                if len(members) != 1 or not members[0].isfile():
                    raise ValueError(f'归档必须包含唯一普通文件: {name}')
                if name != binary:
                    result[name] = archive.extractfile(members[0]).read()
    return result


def verify_materials(path, target, sha, version, compare_frontend=False):
    content = archive_materials(path, target)
    digests = {name: hashlib.sha256(content[name]).hexdigest() for name in LEGAL_FILES}
    if json.loads(content['SOURCE.json']) != source_metadata(sha, version, digests):
        raise ValueError('归档源码来源或许可摘要不一致')
    for name, local in legal_paths().items():
        if not content[name].strip():
            raise ValueError(f'归档许可材料为空: {name}')
        if name != 'THIRD_PARTY_LICENSES.txt' or compare_frontend:
            if content[name] != local.read_bytes():
                raise ValueError(f'归档许可材料与所选源码不一致: {name}')
    return content


def config(simple=False):
    return yaml.safe_load((SIMPLE_CONFIG if simple else FULL_CONFIG).read_text())


def targets(simple=False):
    build = config()['builds'][0]
    result = []
    for goos, goarch in itertools.product(build['goos'], build['goarch']):
        item = {'goos': goos, 'goarch': goarch}
        if any(all(item.get(k) == v for k, v in rule.items()) for rule in build.get('ignore', [])):
            continue
        if not simple or item == {'goos': 'linux', 'goarch': 'amd64'}:
            result.append(item)
    if not result:
        raise ValueError('empty release target matrix')
    return result


def archive_name(version, target):
    if not VERSION_RE.fullmatch(version) or target not in targets():
        raise ValueError('invalid release version or target')
    suffix = 'zip' if target['goos'] == 'windows' else 'tar.gz'
    return f"sub2api_{version}_{target['goos']}_{target['goarch']}.{suffix}"


def sha256(path):
    with path.open('rb') as stream:
        return hashlib.file_digest(stream, 'sha256').hexdigest()


def plan(args):
    sha = subprocess.check_output(['git', 'rev-parse', 'HEAD'], text=True).strip()
    if args.dry_run:
        version = VERSION_FILE.read_text().strip()
        tag = 'v' + version
    else:
        tag = args.ref
        version = tag.removeprefix('v')
        if not tag.startswith('v') or not VERSION_RE.fullmatch(version):
            raise ValueError('publishing requires a v-prefixed release version tag')
        tagged_sha = subprocess.check_output(['git', 'rev-parse', '--verify', f'refs/tags/{tag}^{{commit}}'], text=True).strip()
        if sha != tagged_sha:
            raise ValueError('checkout does not match the selected release tag')
    if not VERSION_RE.fullmatch(version):
        raise ValueError('invalid VERSION')
    VERSION_FILE.write_text(version + '\n')
    result = {'sha': sha, 'tag': tag, 'version': version,
              'owner_lower': os.environ.get('GITHUB_REPOSITORY_OWNER', '').lower(),
              'simple': str(args.simple).lower(), 'dry_run': str(args.dry_run).lower(),
              'date': datetime.now(timezone.utc).strftime('%Y-%m-%dT%H:%M:%SZ'),
              'matrix': json.dumps({'include': targets(args.simple)}, separators=(',', ':'))}
    with Path(os.environ['GITHUB_OUTPUT']).open('a') as output:
        for key, value in result.items():
            output.write(f'{key}={value}\n')


def generate_config(args):
    data = config(args.simple if args.mode == 'publish' else False)
    data['snapshot'] = {'version_template': '{{ .Env.RELEASE_VERSION }}'}
    data['dockers'] = []
    data['docker_manifests'] = []
    if args.mode == 'build':
        target = {'goos': args.goos, 'goarch': args.goarch}
        if target not in targets():
            raise ValueError('unsupported build target')
        for build in data['builds']:
            build['goos'], build['goarch'], build['ignore'] = [args.goos], [args.goarch], []
            build['ldflags'] = [re.sub(r'{{\s*\.Date\s*}}', '{{ .Env.RELEASE_DATE }}', flag)
                                for flag in build.get('ldflags', [])]
    else:
        # Artifacts are supplied through the OSS extra_files mechanism. No build
        # is repeated on the publishing runner, and release templates stay intact.
        data['before'] = {'hooks': []}
        data['builds'] = [{'id': 'sub2api', 'skip': True}]
        data['archives'] = []
        # 新证明按 run/attempt 命名；draft 恢复保留旧说明并追加本次完整材料。
        data['release']['mode'] = 'append'
        evidence = json.loads(Path('release-evidence/publish-assets.json').read_text())
        extra = [{'glob': 'release-input/sub2api_*.tar.gz'}, {'glob': 'release-input/sub2api_*.zip'}, *evidence]
        if args.simple:
            data['release']['skip_upload'] = False
            data['release']['extra_files'] = evidence
            data['checksum'] = {'disable': True}
        else:
            data['release']['extra_files'] = extra
            data['checksum'] = {'name_template': 'checksums.txt', 'algorithm': 'sha256', 'extra_files': extra}
    Path(args.output).write_text(yaml.safe_dump(data, sort_keys=False, allow_unicode=True))


def collect(args):
    target = {'goos': args.goos, 'goarch': args.goarch}
    name = archive_name(args.version, target)
    source = Path('dist') / name
    checksums = {line.split()[1].lstrip('*'): line.split()[0] for line in Path('dist/checksums.txt').read_text().splitlines()}
    digest = sha256(source)
    if checksums.get(name) != digest:
        raise ValueError('archive does not match the build checksum')
    verify_materials(source, target, args.sha, args.version, compare_frontend=True)
    output = Path(args.output)
    output.mkdir(parents=True, exist_ok=True)
    shutil.copy2(source, output / name)
    manifest = {'sha': args.sha, 'version': args.version, 'target': target, 'archive': name, 'sha256': digest}
    (output / f"manifest-{args.goos}-{args.goarch}.json").write_text(json.dumps(manifest) + '\n')


def verify(args):
    directory = Path(args.input)
    expected = set()
    for target in targets(args.simple):
        name = archive_name(args.version, target)
        manifest_name = f"manifest-{target['goos']}-{target['goarch']}.json"
        expected.update((name, manifest_name))
        manifest = json.loads((directory / manifest_name).read_text())
        if manifest != {'sha': args.sha, 'version': args.version, 'target': target,
                        'archive': name, 'sha256': sha256(directory / name)}:
            raise ValueError(f'build provenance or checksum mismatch: {name}')
        verify_materials(directory / name, target, args.sha, args.version)
    if {p.name for p in directory.iterdir()} != expected:
        raise ValueError('missing or unexpected release artifacts')


def contexts(args):
    verify(args)
    for target in targets(args.simple):
        if target['goos'] != 'linux':
            continue
        dest = Path(args.output) / target['goarch']
        dest.mkdir(parents=True, exist_ok=True)
        content = verify_materials(Path(args.input) / archive_name(args.version, target),
                                  target, args.sha, args.version)
        for name, data in content.items():
            output = dest / ('backend/internal/web/dist/' + name if name == 'THIRD_PARTY_LICENSES.txt' else name)
            output.parent.mkdir(parents=True, exist_ok=True)
            output.write_bytes(data)
        with tarfile.open(Path(args.input) / archive_name(args.version, target), 'r:gz') as archive:
            members = [member for member in archive.getmembers() if member.name in ('sub2api', './sub2api')]
            if len(members) != 1 or not members[0].isfile():
                raise ValueError('archive must contain one regular sub2api binary')
            with archive.extractfile(members[0]) as source, (dest / 'sub2api').open('wb') as output:
                shutil.copyfileobj(source, output)
        (dest / 'sub2api').chmod(0o755)
        shutil.copy2('Dockerfile.goreleaser', dest / 'Dockerfile')
        (dest / 'deploy').mkdir(exist_ok=True)
        shutil.copy2('deploy/docker-entrypoint.sh', dest / 'deploy/docker-entrypoint.sh')
        shutil.copytree('backend/resources', dest / 'backend/resources', dirs_exist_ok=True)


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    commands = parser.add_subparsers(dest='command', required=True)
    p = commands.add_parser('plan')
    p.add_argument('--ref', required=True)
    p.add_argument('--simple', action='store_true')
    p.add_argument('--dry-run', action='store_true')
    p.set_defaults(run=plan)
    p = commands.add_parser('materials')
    p.add_argument('--sha', required=True)
    p.add_argument('--version', required=True)
    p.set_defaults(run=materials)
    p = commands.add_parser('config')
    p.add_argument('mode', choices=['build', 'publish'])
    p.add_argument('--goos')
    p.add_argument('--goarch')
    p.add_argument('--simple', action='store_true')
    p.add_argument('--output', required=True)
    p.set_defaults(run=generate_config)
    p = commands.add_parser('collect')
    for arg in ('version', 'sha', 'goos', 'goarch', 'output'):
        p.add_argument('--' + arg, required=True)
    p.set_defaults(run=collect)
    for command, handler in [('verify', verify), ('contexts', contexts)]:
        p = commands.add_parser(command)
        for arg in ('version', 'sha', 'input'):
            p.add_argument('--' + arg, required=True)
        p.add_argument('--simple', action='store_true')
        if command == 'contexts':
            p.add_argument('--output', required=True)
        p.set_defaults(run=handler)
    args = parser.parse_args()
    args.run(args)


if __name__ == '__main__':
    main()
