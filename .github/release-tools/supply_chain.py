#!/usr/bin/env python3
"""生成和核对发布 SBOM、未签名 provenance 及 OCI 内容寻址证明。"""

import argparse
from contextlib import contextmanager
from datetime import datetime, timezone
import hashlib
import json
import os
from pathlib import Path
import platform
import re
import shutil
import subprocess
import tarfile
import tempfile
from urllib.request import urlopen
import zipfile

from release_matrix import archive_name, sha256, targets

POLICY = Path(__file__).with_name('supply-chain-tools.json')
SPDX = 'https://spdx.dev/Document'
SLSA = 'https://slsa.dev/provenance/v1'
STATEMENT = 'https://in-toto.io/Statement/v1'
DIGEST = re.compile(r'sha256:([0-9a-f]{64})')


def save(path, data):
    path.parent.mkdir(parents=True, exist_ok=True)
    path.write_text(json.dumps(data, ensure_ascii=False, indent=2, sort_keys=True) + '\n')


def installed(tool):
    command = [tool, 'version']
    text = subprocess.check_output(command, text=True)
    version = re.search(r'^Version:\s+(\d+\.\d+\.\d+)\s*$', text, re.M)
    expected = json.loads(POLICY.read_text())[tool]['version']
    if version is None or version[1] != expected:
        raise ValueError(f'{tool} 实际版本与固定策略不符')
    print(f'{tool}: {version[1]}')
    return version[1]


def install(args):
    machine = {'x86_64': 'amd64', 'arm64': 'arm64', 'aarch64': 'arm64'}[platform.machine()]
    target = f'{platform.system().lower()}_{machine}'
    destination = Path(args.output)
    destination.mkdir(parents=True, exist_ok=True)
    for tool in args.tools:
        pin = json.loads(POLICY.read_text())[tool]
        expected = pin['archives'][target]
        name = f"{tool}_{pin['version']}_{target}.tar.gz"
        url = f"https://github.com/{pin['repository']}/releases/download/v{pin['version']}/{name}"
        with tempfile.TemporaryDirectory() as directory:
            archive = Path(directory) / name
            with urlopen(url, timeout=60) as response, archive.open('wb') as output:
                shutil.copyfileobj(response, output)
            if sha256(archive) != expected:
                raise ValueError(f'{tool} 官方归档 SHA-256 不符')
            with tarfile.open(archive, 'r:gz') as source:
                members = [m for m in source.getmembers() if m.name == tool]
                if len(members) != 1 or not members[0].isfile():
                    raise ValueError(f'{tool} 安装归档没有唯一普通可执行文件')
                with source.extractfile(members[0]) as data, (destination / tool).open('wb') as output:
                    shutil.copyfileobj(data, output)
            (destination / tool).chmod(0o755)
    os.environ['PATH'] = str(destination.resolve()) + os.pathsep + os.environ['PATH']
    for tool in args.tools:
        installed(tool)


def identity():
    repo = os.environ['GITHUB_REPOSITORY']
    workflow_sha = os.environ['GITHUB_SHA']
    run = os.environ['GITHUB_RUN_ID']
    if not re.fullmatch(r'[A-Za-z0-9_.-]+/[A-Za-z0-9_.-]+', repo) or not re.fullmatch(r'[0-9a-f]{40}', workflow_sha) or not run.isdigit():
        raise ValueError('构建仓库、工作流 SHA 或 run ID 无效')
    return repo, workflow_sha, run


def provenance(subjects, sha, version, target, materials):
    repo, workflow_sha, run = identity()
    return {
        '_type': STATEMENT, 'subject': subjects, 'predicateType': SLSA,
        'predicate': {
            'buildDefinition': {
                'buildType': f'https://github.com/{repo}/.github/workflows/release.yml@{workflow_sha}',
                'externalParameters': {'source': f'https://github.com/{repo}', 'source_sha': sha,
                                       'version': version, 'target': target},
                'internalParameters': {'workflow_sha': workflow_sha, 'ci_run_id': os.environ.get('CI_RUN_ID') or None,
                                       'dry_run': os.environ.get('DRY_RUN') == 'true'},
                'resolvedDependencies': materials,
            },
            'runDetails': {
                'builder': {'id': f'https://github.com/{repo}/actions'},
                'metadata': {'invocationId': f'https://github.com/{repo}/actions/runs/{run}',
                             'finishedOn': datetime.now(timezone.utc).isoformat()},
            },
        },
    }


def subject(path):
    return {'name': path.name, 'digest': {'sha256': sha256(path)}}


def spdx(document):
    if (document.get('spdxVersion') != 'SPDX-2.3' or document.get('SPDXID') != 'SPDXRef-DOCUMENT'
            or document.get('dataLicense') != 'CC0-1.0' or not document.get('packages')):
        raise ValueError('SPDX SBOM 格式无效或缺少依赖')


def binary(args):
    target = {'goos': args.goos, 'goarch': args.goarch}
    archive = Path(args.input) / archive_name(args.version, target)
    manifest = json.loads((Path(args.input) / f'manifest-{args.goos}-{args.goarch}.json').read_text())
    if manifest != {'sha': args.sha, 'version': args.version, 'target': target,
                    'archive': archive.name, 'sha256': sha256(archive)}:
        raise ValueError('归档与源码、目标或校验清单不匹配')
    report = json.loads(Path(args.toolchain).read_text())
    if report['source_sha'] != args.sha or report['workflow_sha'] != identity()[1] or report['run_id'] != identity()[2]:
        raise ValueError('工具元数据未绑定实际源码与当前 run')
    installed('syft')
    output = Path(args.output)
    output.mkdir(parents=True, exist_ok=True)
    sbom = output / (archive.name + '.spdx.json')
    # 扫描实际归档里的 Go 构建信息，并登记前端锁文件中的完整构建依赖。
    # 前端 lock 清单含开发依赖；不声称所有登记项均进入运行时 bundle。
    with tempfile.TemporaryDirectory() as directory:
        stage = Path(directory)
        binary_name = 'sub2api.exe' if args.goos == 'windows' else 'sub2api'
        if args.goos == 'windows':
            with zipfile.ZipFile(archive) as source:
                members = [m for m in source.infolist() if m.filename in (binary_name, './' + binary_name)]
                if len(members) != 1 or members[0].is_dir():
                    raise ValueError('归档必须含唯一应用二进制')
                (stage / binary_name).write_bytes(source.read(members[0]))
        else:
            with tarfile.open(archive, 'r:gz') as source:
                members = [m for m in source.getmembers() if m.name in (binary_name, './' + binary_name)]
                if len(members) != 1 or not members[0].isfile():
                    raise ValueError('归档必须含唯一普通应用二进制')
                with source.extractfile(members[0]) as data, (stage / binary_name).open('wb') as dest:
                    shutil.copyfileobj(data, dest)
        for name in ('package.json', 'pnpm-lock.yaml'):
            shutil.copy2(Path('frontend') / name, stage / name)
        subprocess.run(['syft', 'scan', 'dir:' + directory, '--source-name', archive.name,
                        '--source-version', args.version, '-o', 'spdx-json=' + str(sbom.resolve())],
                       env={**os.environ, 'SYFT_CHECK_FOR_APP_UPDATE': 'false'}, check=True)
    spdx(json.loads(sbom.read_text()))
    materials = [{'uri': f'git+https://github.com/{identity()[0]}', 'digest': {'gitCommit': args.sha}},
                 {'uri': 'binary-toolchain.json', 'digest': {'sha256': sha256(Path(args.toolchain))},
                  'annotations': {'actual': {**report['actual'], 'syft': json.loads(POLICY.read_text())['syft']['version']}, 'inputs_sha256': report['inputs_sha256']}},
                 {'uri': POLICY.name, 'digest': {'sha256': sha256(POLICY)}}]
    save(output / (archive.name + '.provenance.json'), provenance([subject(archive), subject(sbom)], args.sha, args.version, target, materials))


def verify_binaries(args):
    evidence = Path(args.evidence)
    expected = set()
    for target in targets(args.simple):
        archive = Path(args.input) / archive_name(args.version, target)
        sbom = evidence / (archive.name + '.spdx.json')
        proof = evidence / (archive.name + '.provenance.json')
        expected.update((sbom.name, proof.name))
        spdx(json.loads(sbom.read_text()))
        statement = json.loads(proof.read_text())
        definition = statement['predicate']['buildDefinition']
        if (statement['_type'] != STATEMENT or statement['predicateType'] != SLSA
                or statement['subject'] != [subject(archive), subject(sbom)]
                or definition['externalParameters'] != {'source': f'https://github.com/{identity()[0]}',
                    'source_sha': args.sha, 'version': args.version, 'target': target}
                or definition['internalParameters']['workflow_sha'] != identity()[1]
                or statement['predicate']['runDetails']['metadata']['invocationId'] != f'https://github.com/{identity()[0]}/actions/runs/{identity()[2]}'):
            raise ValueError('二进制 SBOM/provenance 绑定不匹配')
    if {p.name for p in evidence.iterdir()} != expected:
        raise ValueError('证明矩阵缺失或存在额外文件')


@contextmanager
def layout_reader(path):
    path = Path(path)
    if path.is_dir():
        def read(name):
            return (path / name).read_bytes()
        yield read
    else:
        with tarfile.open(path, 'r') as archive:
            def read(name):
                member = archive.getmember(name)
                if not member.isfile():
                    raise ValueError('OCI 内容必须为普通文件')
                return archive.extractfile(member).read()
            yield read


def inspect_image(path, sha, version, expected_platforms, created=None):
    with layout_reader(path) as read:
        def blob(desc):
            match = DIGEST.fullmatch(desc['digest'])
            if not match:
                raise ValueError('OCI descriptor digest 无效')
            data = read('blobs/sha256/' + match[1])
            if hashlib.sha256(data).hexdigest() != match[1] or len(data) != desc['size']:
                raise ValueError('OCI blob 与 digest/size 不符')
            return data
        roots = json.loads(read('index.json'))['manifests']
        if len(roots) != 1:
            raise ValueError('OCI 布局必须含唯一根索引')
        root = roots[0]
        index = json.loads(blob(root))
        runnable, attestations, dates = {}, {}, set()
        for desc in index['manifests']:
            image = json.loads(blob(desc))
            if desc.get('annotations', {}).get('vnd.docker.reference.type') == 'attestation-manifest':
                associated = desc['annotations']['vnd.docker.reference.digest']
                if associated in attestations:
                    raise ValueError('同一运行镜像存在重复证明清单')
                statements = {}
                blob(image['config'])
                for layer in image['layers']:
                    statement = json.loads(blob(layer))
                    kind = statement['predicateType']
                    if kind in statements or statement.get('_type') not in (STATEMENT, 'https://in-toto.io/Statement/v0.1'):
                        raise ValueError('证明类型重复或 statement 无效')
                    if not statement.get('subject') or any(s['digest'].get('sha256') != associated.removeprefix('sha256:') for s in statement['subject']):
                        raise ValueError('镜像证明 subject 未绑定运行镜像 digest')
                    statements[kind] = statement
                attestations[associated] = statements
                continue
            config = json.loads(blob(image['config']))
            platform_name = config['os'] + '/' + config['architecture']
            if platform_name in runnable or desc.get('platform') != {'os': config['os'], 'architecture': config['architecture']}:
                raise ValueError('运行平台重复或 descriptor/config 不一致')
            labels = config.get('config', {}).get('Labels', {})
            expected_labels = {'source': f'https://github.com/{identity()[0]}', 'revision': sha, 'version': version}
            date = labels.get('org.opencontainers.image.created', '')
            if not date.endswith('Z') or datetime.fromisoformat(date).utcoffset().total_seconds() != 0 or (created and date != created):
                raise ValueError('OCI created 不匹配或不是 UTC 时间')
            dates.add(date)
            if any(labels.get('org.opencontainers.image.' + k) != v for k, v in expected_labels.items()):
                raise ValueError('OCI source/revision/version/created 不匹配')
            for layer in image['layers']:
                blob(layer)
            runnable[platform_name] = desc['digest']
        if set(runnable) != set(expected_platforms) or set(attestations) != set(runnable.values()):
            raise ValueError('运行或证明平台矩阵不完整')
        for digest, statements in attestations.items():
            if SPDX not in statements or not any(k.startswith('https://slsa.dev/provenance/') for k in statements):
                raise ValueError('镜像缺少 SPDX 或原生 BuildKit provenance')
            spdx(statements[SPDX]['predicate'])
        if len(dates) != 1:
            raise ValueError('各镜像平台 created 不一致')
        return root['digest'], runnable, attestations, dates.pop()


def image(args):
    digest, platforms, attestations, created = inspect_image(args.layout, args.sha, args.version, args.platforms, args.created)
    dest = Path(args.output)
    report = {'sha': args.sha, 'version': args.version, 'created': created, 'digest': digest,
              'platforms': platforms, 'verified': True, 'dry_run': os.environ.get('DRY_RUN') == 'true',
              'workflow_sha': identity()[1], 'release_run': f'https://github.com/{identity()[0]}/actions/runs/{identity()[2]}'}
    subjects = [{'name': 'container-image', 'digest': {'sha256': digest.removeprefix('sha256:')}}]
    for name, leaf_digest in platforms.items():
        prefix = dest.name + '-' + name.replace('/', '-')
        for kind, statement in attestations[leaf_digest].items():
            suffix = 'spdx' if kind == SPDX else 'buildkit-provenance'
            output = dest / f'{prefix}.{suffix}.json'
            save(output, statement)
            subjects.append(subject(output))
    materials = [{'uri': f'git+https://github.com/{identity()[0]}', 'digest': {'gitCommit': args.sha}}]
    save(dest / (dest.name + '.pipeline-provenance.json'), provenance(subjects, args.sha, args.version, sorted(platforms), materials))
    save(dest / (dest.name + '.verification.json'), report)
    print(digest)



def finalize(args):
    directory = Path(args.input)
    files = sorted(directory.glob('*/*.json'))
    if not files or len({p.name for p in files}) != len(files):
        raise ValueError('证明材料为空或公开资产名称重复')
    report_paths = sorted(directory.glob('image-*/image-*.verification.json'))
    reports = [json.loads(p.read_text()) for p in report_paths]
    expected = {'linux/amd64'} if args.simple else {'linux/amd64', 'linux/arm64'}
    stage = 'built' if os.environ.get('DRY_RUN') == 'true' else 'published'
    selected = [r for p, r in zip(report_paths, reports) if p.parent.name.endswith('-' + stage)]
    observed = {platform for report in selected for platform in report['platforms']}
    if observed != expected or not selected or any(not r['verified'] or r['sha'] != args.sha or r['version'] != args.version for r in selected):
        raise ValueError('最终验证材料与源码或平台矩阵不匹配')
    repo, workflow_sha, run = identity()
    ci = os.environ.get('CI_RUN_ID') or None
    if os.environ.get('DRY_RUN') != 'true' and (ci is None or not ci.isdigit()):
        raise ValueError('正式发布证明缺少成功 CI run ID')
    summary = {'source_sha': args.sha, 'workflow_sha': workflow_sha, 'version': args.version,
               'ci_run': f'https://github.com/{repo}/actions/runs/{ci}' if ci else None,
               'release_run': f'https://github.com/{repo}/actions/runs/{run}',
               'dry_run': os.environ.get('DRY_RUN') == 'true', 'images': selected,
               'signature': 'unsigned; verify digest and trusted Actions artifact transport'}
    save(directory / 'summary/release-summary.json', summary)
    files = sorted(directory.glob('*/*.json'))
    (directory / 'evidence-checksums.txt').write_text(''.join(f'{sha256(path)}  {path.name}\n' for path in files))


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    commands = parser.add_subparsers(dest='command', required=True)
    p = commands.add_parser('install')
    p.add_argument('--tools', nargs='+', choices=('syft', 'oras'), required=True)
    p.add_argument('--output', required=True)
    p.set_defaults(run=install)
    p = commands.add_parser('binary')
    for arg in ('input', 'output', 'sha', 'version', 'goos', 'goarch', 'toolchain'):
        p.add_argument('--' + arg, required=True)
    p.set_defaults(run=binary)
    p = commands.add_parser('verify-binaries')
    for arg in ('input', 'evidence', 'sha', 'version'):
        p.add_argument('--' + arg, required=True)
    p.add_argument('--simple', action='store_true')
    p.set_defaults(run=verify_binaries)
    p = commands.add_parser('image')
    for arg in ('layout', 'output', 'sha', 'version'):
        p.add_argument('--' + arg, required=True)
    p.add_argument('--created')
    p.add_argument('--platforms', nargs='+', required=True)
    p.set_defaults(run=image)
    p = commands.add_parser('finalize')
    for arg in ('input', 'sha', 'version'):
        p.add_argument('--' + arg, required=True)
    p.add_argument('--simple', action='store_true')
    p.set_defaults(run=finalize)
    args = parser.parse_args()
    args.run(args)


if __name__ == '__main__':
    main()
