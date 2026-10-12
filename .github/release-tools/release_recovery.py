#!/usr/bin/env python3
"""从 main 恢复一次失败的 simple 发布，只读取原成功产物和既有 OCI。"""

import argparse
from datetime import datetime, timezone
import hashlib
import io
import json
import os
from pathlib import Path, PurePosixPath
import re
import stat
import subprocess
import sys
import tempfile
import uuid
from urllib.error import HTTPError
from urllib.parse import urlencode
from urllib.request import HTTPRedirectHandler, Request, build_opener, urlopen
import zipfile

# 入口以 -I 执行；只加入当前 main 保存的 sibling 工具，不导入应用目录。
sys.path.insert(0, str(Path(__file__).resolve().parent))
import personal_release as release
import release_matrix as matrix
import supply_chain as supply

WORKFLOW = 'release-recovery.yml'
TARGET = {'goos': 'linux', 'goarch': 'amd64'}
ARTIFACT_JOBS = {
    'version-file': 'prepare', 'release-guard': 'prepare',
    'release-binary-linux-amd64': 'build-binaries (linux, amd64)',
    'release-evidence-binary-linux-amd64': 'build-binaries (linux, amd64)',
    'toolchain-binary-linux-amd64': 'build-binaries (linux, amd64)',
}
RUN_FIELDS = ('id', 'workflow_id', 'path', 'head_branch', 'head_sha', 'event', 'status', 'conclusion', 'run_attempt')
JOB_FIELDS = ('id', 'name', 'run_id', 'run_attempt', 'head_sha', 'status', 'conclusion', 'started_at', 'completed_at')
ARTIFACT_FIELDS = ('id', 'name', 'digest', 'size_in_bytes', 'created_at', 'updated_at', 'workflow_run')


def require(condition, message):
    if not condition:
        raise release.SyncError(message)


def numeric(value):
    return bool(re.fullmatch(r'[1-9][0-9]*', str(value)))


def selected(data, fields):
    return {key: data[key] for key in fields}


def control_identity():
    repo, control_sha, run_id = supply.identity()
    require(repo == release.REPOSITORY and numeric(run_id) and os.environ['GITHUB_REF'] == 'refs/heads/main'
            and os.environ['GITHUB_EVENT_NAME'] == 'workflow_dispatch'
            and os.environ['GITHUB_WORKFLOW_SHA'] == control_sha
            and os.environ['GITHUB_WORKFLOW_REF'] == f'{repo}/.github/workflows/{WORKFLOW}@refs/heads/main'
            and numeric(os.environ['GITHUB_RUN_ATTEMPT']), '恢复只允许从同仓库 main 的手动工作流执行')
    workflow = release.api(f'repos/{repo}/actions/workflows/{WORKFLOW}')
    run = release.api(f'repos/{repo}/actions/runs/{run_id}')
    require(str(run['id']) == run_id and run['repository']['full_name'] == repo and run['head_repository']['full_name'] == repo
            and run['head_sha'] == control_sha and run['head_branch'] == 'main'
            and run['workflow_id'] == workflow['id'] and run['path'] == f'.github/workflows/{WORKFLOW}'
            and run['event'] == 'workflow_dispatch' and str(run['run_attempt']) == os.environ['GITHUB_RUN_ATTEMPT'],
            '当前恢复运行的仓库、工作流或控制 SHA 绑定不匹配')
    require(release.git('ls-remote', '--heads', 'origin', 'refs/heads/main').split() == [control_sha, 'refs/heads/main'],
            'main 控制 SHA 已变化，停止恢复')
    return {'control_sha': control_sha, 'workflow': WORKFLOW, 'run_id': run_id,
            'run_attempt': os.environ['GITHUB_RUN_ATTEMPT'], 'mode': 'verify-only'}


def original_run(run_id, sha):
    workflow = release.api(f'repos/{release.REPOSITORY}/actions/workflows/release.yml')
    run = release.api(f'repos/{release.REPOSITORY}/actions/runs/{run_id}')
    require(str(run['id']) == run_id and run['workflow_id'] == workflow['id']
            and run['path'] == '.github/workflows/release.yml' and run['head_sha'] == sha
            and run['head_branch'] == 'personal' and run['event'] == 'workflow_dispatch'
            and run['repository']['full_name'] == release.REPOSITORY
            and run['head_repository']['full_name'] == release.REPOSITORY
            and run['repository']['id'] == run['head_repository']['id']
            and run['status'] == 'completed' and run['conclusion'] == 'failure',
            '原运行不是同仓库同 personal SHA 的失败手动 Release')
    jobs = release.pages(f'repos/{release.REPOSITORY}/actions/runs/{run_id}/jobs?filter=all', 'jobs')
    expected = {*ARTIFACT_JOBS.values(), 'build-frontend', 'release'}
    require(len(jobs) == len(expected) and {j['name'] for j in jobs} == expected,
            '原发布必须有唯一 simple 生产矩阵；重复 attempt 或额外生产 job 禁止恢复')
    result = {}
    for job in jobs:
        require(job['run_id'] == run['id'] and job['head_sha'] == sha and numeric(job['run_attempt'])
                and job['run_attempt'] <= run['run_attempt'] and job['status'] == 'completed'
                and job['conclusion'] == ('failure' if job['name'] == 'release' else 'success'),
                '原发布的生产 job 未成功或失败发布 job 身份不匹配')
        result[job['name']] = selected(job, JOB_FIELDS)
    require(result['release']['run_attempt'] == run['run_attempt'], '原发布失败 attempt 不匹配')
    return selected(run, RUN_FIELDS), result, run['repository']['id']


def artifacts_for(run, jobs, repository_id):
    artifacts = release.pages(f'repos/{release.REPOSITORY}/actions/runs/{run["id"]}/artifacts', 'artifacts')
    # 数据矩阵与 successful producer 矩阵一致；不下载旧 release-tooling 来执行。
    names = {a['name'] for a in artifacts if a['name'].startswith(('release-binary-', 'release-evidence-binary-'))}
    require(names == {'release-binary-linux-amd64', 'release-evidence-binary-linux-amd64'},
            '原 artifact 不是唯一 simple 数据矩阵')
    result = {}
    for name, job_name in ARTIFACT_JOBS.items():
        matches = [a for a in artifacts if a['name'] == name]
        require(len(matches) == 1, f'原 artifact 缺失或重名：{name}')
        artifact = matches[0]
        job = jobs[job_name]
        result[name] = transported_artifact(artifact, run, job, repository_id)
    return result


def transported_artifact(artifact, run, job, repository_id):
    origin = artifact['workflow_run']
    require(not artifact['expired'] and numeric(artifact['id']) and artifact['size_in_bytes'] > 0
            and supply.DIGEST.fullmatch(artifact.get('digest', ''))
            and origin == {'id': run['id'], 'head_sha': run['head_sha'], 'head_branch': run['head_branch'],
                           'repository_id': repository_id, 'head_repository_id': repository_id}
            and datetime.fromisoformat(job['started_at']) <= datetime.fromisoformat(artifact['created_at'])
                <= datetime.fromisoformat(artifact['updated_at']) <= datetime.fromisoformat(job['completed_at']),
            f'artifact 已过期、运输 digest 缺失或不属于成功生产者：{artifact["name"]}')
    return selected(artifact, ARTIFACT_FIELDS)


class ArtifactRedirect(HTTPRedirectHandler):
    def redirect_request(self, req, fp, code, msg, headers, newurl):
        require(newurl.startswith('https://'), 'artifact 重定向必须使用 HTTPS')
        redirected = super().redirect_request(req, fp, code, msg, headers, newurl)
        redirected.remove_header('Authorization')
        return redirected


def download(artifact):
    endpoint = f'https://api.github.com/repos/{release.REPOSITORY}/actions/artifacts/{artifact["id"]}/zip'
    req = Request(endpoint, headers={'Authorization': 'Bearer ' + os.environ['GH_TOKEN'],
        'Accept': 'application/vnd.github+json', 'X-GitHub-Api-Version': '2026-03-10'})
    try:
        with build_opener(ArtifactRedirect()).open(req, timeout=60) as response:
            data = response.read()
    except HTTPError as error:
        raise release.SyncError(f'artifact 下载失败：HTTP {error.code}，id {artifact["id"]}') from error
    require(len(data) == artifact['size_in_bytes'] and 'sha256:' + hashlib.sha256(data).hexdigest() == artifact['digest'],
            f'artifact 运输 digest/size 不匹配：{artifact["name"]}')
    return data


def zip_data(data):
    result = {}
    with zipfile.ZipFile(io.BytesIO(data)) as archive:
        for member in archive.infolist():
            path = PurePosixPath(member.filename)
            require(not path.is_absolute() and '..' not in path.parts and '\\' not in member.filename
                    and str(path) == member.filename.rstrip('/') and member.filename not in result
                    and stat.S_IFMT(member.external_attr >> 16) in (0, stat.S_IFREG, stat.S_IFDIR),
                    'artifact 必须只有安全、唯一的普通文件路径')
            if member.is_dir():
                continue
            result[member.filename] = archive.read(member)
    return result


def write_data(destination, content, expected):
    require(set(content) == set(expected), 'artifact 文件清单缺失或存在额外文件')
    destination.mkdir(parents=True, exist_ok=False)
    for name, data in content.items():
        (destination / name).write_bytes(data)


def authorize(plan):
    # 所有远端写入共用原发布的源码、CI、tag、来源和镜像占用门禁。
    result = release.publication_authorization(plan['sha'], plan['tag'], plan['ci_run_id'])
    require(str(result['ci_run_id']) == plan['ci_run_id'] and result['reuse_image']
            and result['digest'] == plan['expected_digest'], '既有版本镜像、CI 或固定 digest 已漂移，禁止恢复')
    return result


def recheck(plan):
    require(control_identity() == plan['recovery_control'], '恢复控制运行身份已变化')
    run, jobs, repository_id = original_run(plan['original_run_id'], plan['sha'])
    require(run == plan['original_run'] and jobs == plan['jobs'], '原发布或生产 attempt 已变化')
    require(artifacts_for(run, jobs, repository_id) == plan['artifacts'], '原 artifact 绑定已变化')
    return authorize(plan)


def load_plan(state):
    return json.loads(Path(state).read_text())


def verify_original_data(plan, data, root):
    control, run, jobs = plan['recovery_control'], plan['original_run'], plan['jobs']
    require(data['version-file'] == {'VERSION': (plan['version'] + '\n').encode()}, '原 prepare 版本不匹配')
    guard = data['release-guard']
    old_control = guard['control-sha'].decode().strip()
    require(release.SHA.fullmatch(old_control), '原 main 控制 SHA 无效')
    subprocess.run(['git', 'merge-base', '--is-ancestor', old_control, control['control_sha']], check=True)
    for name in ('release-tools/supply_chain.py', 'release-tools/personal_release.py',
                 'release-tools/supply-chain-tools.json', 'personal-sync/sync.py'):
        expected = subprocess.check_output(['git', 'show', f'{old_control}:.github/{name}'])
        require(guard[name] == expected, '原 prepare guard 数据不等于已记录 main 控制提交')
    producer = {'repository': release.REPOSITORY, 'workflow_sha': run['head_sha'], 'run_id': str(run['id']),
                'run_attempt': str(jobs['build-binaries (linux, amd64)']['run_attempt'])}
    archive_name = matrix.archive_name(plan['version'], TARGET)
    prefix = archive_name + f'.run-{producer["run_id"]}-attempt-{producer["run_attempt"]}'
    write_data(root / 'input', data['release-binary-linux-amd64'], (archive_name, 'manifest-linux-amd64.json'))
    write_data(root / 'evidence/binaries', data['release-evidence-binary-linux-amd64'],
               (prefix + '.provenance.json', prefix + '.spdx.json'))
    write_data(root / 'evidence/original-toolchain', data['toolchain-binary-linux-amd64'], ('binary-toolchain.json',))
    binary_args = argparse.Namespace(input=str(root / 'input'), evidence=str(root / 'evidence/binaries'),
                                     sha=plan['sha'], version=plan['version'], simple=True)
    matrix.verify(binary_args)
    supply.verify_binaries(binary_args, producer=producer)
    report_path = root / 'evidence/original-toolchain/binary-toolchain.json'
    report = json.loads(report_path.read_text())
    require(report['source_sha'] == plan['sha'] and report['workflow_sha'] == producer['workflow_sha']
            and report['run_id'] == producer['run_id'], '原二进制工具元数据身份不匹配')
    proof = json.loads((root / 'evidence/binaries' / (prefix + '.provenance.json')).read_text())
    policy = json.loads(guard['release-tools/supply-chain-tools.json'])
    expected_materials = [
        {'uri': f'git+https://github.com/{release.REPOSITORY}', 'digest': {'gitCommit': plan['sha']}},
        {'uri': 'binary-toolchain.json', 'digest': {'sha256': supply.sha256(report_path)},
         'annotations': {'actual': {**report['actual'], 'syft': policy['syft']['version']}, 'inputs_sha256': report['inputs_sha256']}},
        {'uri': 'supply-chain-tools.json', 'digest': {'sha256': hashlib.sha256(guard['release-tools/supply-chain-tools.json']).hexdigest()}},
    ]
    require(proof['predicate']['buildDefinition']['resolvedDependencies'] == expected_materials,
            '原二进制工具材料 digest 或注解不匹配')
    plan['recovery'] = {**control, 'original_release': {'run_id': str(run['id']), 'run_attempt': str(run['run_attempt']),
        'workflow_sha': run['head_sha'], 'control_sha': old_control}, 'binary_producer': producer,
        'producer_jobs': jobs, 'artifacts': list(plan['artifacts'].values())}


def prepare(args):
    require(release.SHA.fullmatch(args.sha) and release.PERSONAL_TAG.fullmatch(args.tag)
            and numeric(args.original_run_id) and numeric(args.ci_run_id)
            and supply.DIGEST.fullmatch(args.expected_digest), '恢复输入的 SHA/tag/run/digest 无效')
    control = control_identity()
    require(args.original_run_id != control['run_id'], '恢复不能把当前运行作为原发布')
    plan = {'sha': args.sha, 'tag': args.tag, 'version': args.tag[1:], 'ci_run_id': args.ci_run_id,
            'expected_digest': args.expected_digest, 'original_run_id': args.original_run_id,
            'recovery_control': control}
    run, jobs, repository_id = original_run(args.original_run_id, args.sha)
    authorized = authorize(plan)
    if authorized['skip']:
        release.output({'skip': True, 'state': 'already_published', **plan})
        return
    plan.update(original_run=run, jobs=jobs, artifacts=artifacts_for(run, jobs, repository_id))
    root = Path(args.state).parent
    root.mkdir(parents=True, exist_ok=False)
    data = {name: zip_data(download(artifact)) for name, artifact in plan['artifacts'].items()}
    verify_original_data(plan, data, root)
    supply.save(Path(args.state), plan)
    # 下载期间原 run、artifact、源码、CI、tag 或版本占用变化也不能进入恢复阶段。
    require(not recheck(plan)['skip'], '下载期间 Release 已完成；停止并重新执行以幂等退出')
    release.output({'skip': False, 'state': 'verified_original_data', 'version': plan['version'],
                    'digest': plan['expected_digest'], 'recovery': plan['recovery']})


def verify(args):
    plan = load_plan(args.state)
    authorized = recheck(plan)
    if authorized['skip']:
        release.output({'skip': True, 'state': 'already_published'})
        return
    root = Path(args.state).parent
    supply.installed('oras')
    subprocess.run(['oras', 'cp', '--to-oci-layout', release.IMAGE + '@' + plan['expected_digest'],
                    str(root / 'image-layout') + ':verified'], check=True)
    supply.image(argparse.Namespace(layout=str(root / 'image-layout'), sha=plan['sha'], version=plan['version'],
        platforms=['linux/amd64'], created=None, expected_digest=plan['expected_digest'],
        output=str(root / 'evidence/image-amd64-published')), workflow=WORKFLOW)
    supply.finalize(argparse.Namespace(input=str(root / 'evidence'), sha=plan['sha'], version=plan['version'], simple=True),
                    recovery=plan['recovery'])
    require(not recheck(plan)['skip'], 'OCI 校验期间 Release 已完成；停止并重新执行以幂等退出')
    release.output({'skip': False, 'state': 'verified_existing_image', 'digest': plan['expected_digest']})


def verification_prefix(control):
    require(numeric(control['run_id']) and numeric(control['run_attempt']), '验证运行身份无效')
    return f'run-{control["run_id"]}-attempt-{control["run_attempt"]}'


def release_assets(plan):
    prefix = verification_prefix(plan['recovery_control'])
    root = Path(plan['state']).parent / 'evidence'
    entries = json.loads((root / 'publish-assets.json').read_text())
    result = {}
    for entry in entries:
        path = Path(entry['glob'])
        name = entry['name_template']
        require(path.is_file() and path.resolve().is_relative_to(root.resolve()) and path.name != 'publish-assets.json'
                and re.fullmatch(r'[A-Za-z0-9_.-]+', name) and prefix in name and name not in result,
                '恢复发布资产路径或本次 run/attempt 名称无效')
        result[name] = path
    summary = root / 'summary' / (prefix + '.release-summary.json')
    data = json.loads(summary.read_text())
    require(data['source_sha'] == plan['sha'] and data['workflow_sha'] == plan['recovery_control']['control_sha']
            and data['version'] == plan['version'] and data['recovery'] == plan['recovery']
            and data['ci_run'] == f'https://github.com/{release.REPOSITORY}/actions/runs/{plan["ci_run_id"]}'
            and data['verification_run'] == f'https://github.com/{release.REPOSITORY}/actions/runs/{plan["recovery_control"]["run_id"]}'
            and data['release_run'] is None and data['publication'] == {'method': 'verification-only', 'performed': False}
            and data['dry_run'] is False and len(data['images']) == 1
            and data['images'][0]['digest'] == plan['expected_digest'] and data['images'][0]['verified'] is True,
            '恢复摘要未绑定本次控制与原发布身份')
    checksums = root / (prefix + '-evidence-checksums.txt')
    require(set(result) == {*(a['public_name'] for a in data['assets']), summary.name, checksums.name},
            '恢复公开资产清单不完整或存在额外资产')
    for asset in data['assets']:
        require(asset['public_name'] in result and supply.sha256(result[asset['public_name']]) == asset['sha256'],
                '恢复资产与已验证摘要哈希不匹配')
    require(checksums.read_text() == ''.join(f'{supply.sha256(path)}  {name}\n' for name, path in result.items()
                                          if path != checksums), '恢复证明校验清单不匹配')
    return result, summary.read_text()


def upload_asset(release_id, name, path):
    endpoint = f'https://uploads.github.com/repos/{release.REPOSITORY}/releases/{release_id}/assets?' + urlencode({'name': name})
    req = Request(endpoint, data=path.read_bytes(), headers={'Authorization': 'Bearer ' + os.environ['GH_TOKEN'],
        'Accept': 'application/vnd.github+json', 'Content-Type': 'application/octet-stream',
        'X-GitHub-Api-Version': '2026-03-10'}, method='POST')
    try:
        with urlopen(req, timeout=60) as response:
            return json.loads(response.read())
    except HTTPError as error:
        raise release.SyncError(f'Release 资产上传失败：HTTP {error.code}，{name}') from error


def published_asset(asset, name, path):
    return (asset['name'] == name and asset['state'] == 'uploaded' and asset['size'] == path.stat().st_size
            and asset.get('digest') == 'sha256:' + supply.sha256(path))


def verification_run(run_id):
    require(numeric(run_id), 'verification run ID 无效')
    workflow = release.api(f'repos/{release.REPOSITORY}/actions/workflows/{WORKFLOW}')
    run = release.api(f'repos/{release.REPOSITORY}/actions/runs/{run_id}')
    require(str(run['id']) == run_id and run['workflow_id'] == workflow['id']
            and run['path'] == f'.github/workflows/{WORKFLOW}' and release.SHA.fullmatch(run['head_sha'])
            and run['head_branch'] == 'main' and run['event'] == 'workflow_dispatch'
            and run['repository']['full_name'] == release.REPOSITORY
            and run['head_repository']['full_name'] == release.REPOSITORY
            and run['repository']['id'] == run['head_repository']['id']
            and run['status'] == 'completed' and run['conclusion'] == 'success' and numeric(run['run_attempt']),
            'verification 不是同仓库 main 的成功手动恢复验证')
    jobs = release.pages(f'repos/{release.REPOSITORY}/actions/runs/{run_id}/attempts/{run["run_attempt"]}/jobs', 'jobs')
    require(len(jobs) == 1 and jobs[0]['name'] == 'release' and jobs[0]['run_id'] == run['id']
            and jobs[0]['head_sha'] == run['head_sha'] and jobs[0]['run_attempt'] == run['run_attempt']
            and jobs[0]['status'] == 'completed' and jobs[0]['conclusion'] == 'success',
            'verification 必须有本 attempt 的唯一成功验证 job')
    return selected(run, RUN_FIELDS), selected(jobs[0], JOB_FIELDS), run['repository']['id']


def verification_artifact(run, job, repository_id):
    name = f'release-recovery-supply-chain-{run["id"]}-{run["run_attempt"]}'
    rows = release.pages(f'repos/{release.REPOSITORY}/actions/runs/{run["id"]}/artifacts', 'artifacts')
    matches = [row for row in rows if row['name'] == name]
    require(len(matches) == 1, 'verification artifact 缺失或重名')
    return transported_artifact(matches[0], run, job, repository_id)


def local_control(control_sha, compare_bytes=False):
    # 本机发布只能执行验证运行使用的已提交 main 工具，不靠伪造 Actions 环境身份。
    root = Path(__file__).resolve().parents[2]
    head = subprocess.check_output(['git', '-C', str(root), 'rev-parse', 'HEAD'], text=True).strip()
    paths = ('.github/release-tools', '.github/personal-sync', '.github/workflows/' + WORKFLOW)
    status = subprocess.check_output(['git', '-C', str(root), 'status', '--porcelain', '--untracked-files=all', '--', *paths], text=True)
    require(head == control_sha and not status.strip(), '本机工具必须来自 verification 的同一已提交 main 控制 SHA')
    require(release.git('ls-remote', '--heads', 'origin', 'refs/heads/main').split() == [control_sha, 'refs/heads/main'],
            'verification 之后 main 已变化，禁止使用另一控制提交发布')
    if compare_bytes:
        files = subprocess.check_output(['git', '-C', str(root), 'ls-tree', '-r', '--name-only', control_sha, '--', *paths], text=True).splitlines()
        require(files, '本机 main 工具清单为空')
        for name in files:
            expected = subprocess.check_output(['git', '-C', str(root), 'show', f'{control_sha}:{name}'])
            require((root / name).is_file() and (root / name).read_bytes() == expected, '本机 main 控制文件内容不匹配')


def local_recheck(plan):
    local_control(plan['recovery_control']['control_sha'])
    run, job, repository_id = verification_run(plan['recovery_control']['run_id'])
    current = {'run': run, 'job': job, 'artifact': verification_artifact(run, job, repository_id)}
    require(current == plan['verification'], '成功 verification 的运行、attempt 或运输已变化')
    original, jobs, repository_id = original_run(plan['original_run_id'], plan['sha'])
    require(original == plan['original_run'] and jobs == plan['jobs']
            and artifacts_for(original, jobs, repository_id) == plan['artifacts'], '原生产者或 artifact 绑定已变化')
    # 本机进程不占用 Actions 队列。旧 main 调度的 Personal release 仍可能
    # checkout 当前 main/personal；API 没有 dispatch 输入，不能据 head SHA
    # 判定它无关。Release 保留固定 source 筛选，Personal release 必须全部结束。
    queries = (f'release.yml/runs?head_sha={plan["sha"]}', 'personal-release.yml/runs')
    for query in queries:
        runs = release.pages(f'repos/{release.REPOSITORY}/actions/workflows/{query}', 'workflow_runs')
        require(all(run['status'] == 'completed' for run in runs), '存在尚未结束的 Release/Personal release，禁止本机并发发布')
    return authorize(plan)


def restore_verification(data, root):
    entries = json.loads(data['publish-assets.json'])
    require(entries, 'verification 公开资产清单为空')
    mapping, roots = [], set()
    for entry in entries:
        path = PurePosixPath(entry['glob'])
        require(path.is_absolute() and '..' not in path.parts, 'verification 资产原路径无效')
        original_root = path.parent if path.suffix == '.txt' else path.parent.parent
        roots.add(original_root)
        relative = str(path.relative_to(original_root))
        require(relative in data, 'verification artifact 缺少已登记资产')
        mapping.append({'glob': str(root / 'evidence' / relative), 'name_template': entry['name_template']})
    require(len(roots) == 1 and set(data) == {'publish-assets.json', *(str(PurePosixPath(e['glob']).relative_to(root / 'evidence')) for e in mapping)},
            'verification artifact 文件矩阵不完整或存在额外文件')
    for name, content in data.items():
        path = root / 'evidence' / name
        path.parent.mkdir(parents=True, exist_ok=True)
        path.write_bytes(content)
    supply.save(root / 'evidence/publish-assets.json', mapping)


def verify_pipeline(plan, root):
    assets, summary = release_assets(plan)
    document = json.loads(summary)
    image = root / 'evidence/image-amd64-published'
    prefix = verification_prefix(plan['recovery_control'])
    report = json.loads((image / ('image-amd64-published.' + prefix + '.verification.json')).read_text())
    require(document['images'] == [report] and report['sha'] == plan['sha'] and report['version'] == plan['version']
            and set(report['platforms']) == {'linux/amd64'} and supply.DIGEST.fullmatch(report['platforms']['linux/amd64'])
            and report['workflow_sha'] == plan['recovery_control']['control_sha'] and report['dry_run'] is False
            and report['release_run'] is None and report['publication_performed'] is False
            and report['verification_run'] == document['verification_run'],
            'verification 镜像报告的源码、平台或控制绑定不匹配')
    proof = json.loads((image / ('image-amd64-published.' + prefix + '.pipeline-provenance.json')).read_text())
    definition = proof['predicate']['buildDefinition']
    expected_subjects = {'container-image': {'sha256': plan['expected_digest'][7:]}}
    for path in image.iterdir():
        if path.name.endswith(('.spdx.json', '.spdx-attestation.json', '.buildkit-provenance.json')):
            expected_subjects[path.name] = {'sha256': supply.sha256(path)}
    require(proof['_type'] == supply.STATEMENT and proof['predicateType'] == supply.SLSA
            and len(proof['subject']) == len(expected_subjects)
            and {s['name']: s['digest'] for s in proof['subject']} == expected_subjects
            and definition['buildType'] == f'https://github.com/{release.REPOSITORY}/.github/workflows/{WORKFLOW}@{plan["recovery_control"]["control_sha"]}'
            and definition['externalParameters'] == {'source': f'https://github.com/{release.REPOSITORY}', 'source_sha': plan['sha'],
                'version': plan['version'], 'target': ['linux/amd64']}
            and definition['internalParameters'] == {'workflow_sha': plan['recovery_control']['control_sha'], 'ci_run_id': plan['ci_run_id'],
                'dry_run': False, 'run_attempt': plan['recovery_control']['run_attempt']}
            and proof['predicate']['runDetails']['builder']['id'] == f'https://github.com/{release.REPOSITORY}/actions'
            and proof['predicate']['runDetails']['metadata']['invocationId'] == f'https://github.com/{release.REPOSITORY}/actions/runs/{plan["recovery_control"]["run_id"]}',
            'verification pipeline provenance 没有精确绑定真实验证运行')
    return assets


def publish_local(args):
    require(release.SHA.fullmatch(args.sha) and release.PERSONAL_TAG.fullmatch(args.tag)
            and numeric(args.original_run_id) and numeric(args.ci_run_id) and numeric(args.verification_run_id)
            and supply.DIGEST.fullmatch(args.expected_digest), '本机恢复发布输入无效')
    require(args.auth == 'gh-maintainer', '本机发布必须显式选择现有 gh 维护者认证')
    credential_env = {key: value for key, value in os.environ.items()
                      if key not in ('GH_TOKEN', 'GITHUB_TOKEN', 'GH_ENTERPRISE_TOKEN', 'GITHUB_ENTERPRISE_TOKEN')}
    token = subprocess.check_output(['gh', 'auth', 'token', '--hostname', 'github.com'], text=True, env=credential_env).strip()
    require(token, '现有 gh 维护者认证不可用')
    os.environ['GH_TOKEN'] = token
    os.environ['CI_RUN_ID'], os.environ['DRY_RUN'] = args.ci_run_id, 'false'
    run, job, repository_id = verification_run(args.verification_run_id)
    control = {'control_sha': run['head_sha'], 'workflow': WORKFLOW, 'run_id': str(run['id']),
               'run_attempt': str(run['run_attempt']), 'mode': 'verify-only'}
    local_control(control['control_sha'], compare_bytes=True)
    user = release.api('user')
    repository = release.api(f'repos/{release.REPOSITORY}')
    require(re.fullmatch(r'[A-Za-z0-9-]+', user['login']) and repository['id'] == repository_id
            and repository['permissions']['push'], '现有 gh 认证不是该仓库的维护者')
    original, jobs, original_repository_id = original_run(args.original_run_id, args.sha)
    plan = {'sha': args.sha, 'tag': args.tag, 'version': args.tag[1:], 'ci_run_id': args.ci_run_id,
            'expected_digest': args.expected_digest, 'original_run_id': args.original_run_id,
            'recovery_control': control, 'original_run': original, 'jobs': jobs}
    if authorize(plan)['skip']:
        release.output({'skip': True, 'state': 'already_published', 'publisher_method': 'local-gh-maintainer-token'})
        return
    artifact = verification_artifact(run, job, repository_id)
    plan['verification'] = {'run': run, 'job': job, 'artifact': artifact}
    plan['artifacts'] = artifacts_for(original, jobs, original_repository_id)
    root = Path(args.output).resolve()
    root.mkdir(parents=True, exist_ok=False, mode=0o700)
    plan['state'] = str(root / 'state.json')
    restore_verification(zip_data(download(artifact)), root)
    data = {name: zip_data(download(metadata)) for name, metadata in plan['artifacts'].items()}
    verify_original_data(plan, data, root / 'original-data')
    summary = json.loads((root / 'evidence/summary' / (verification_prefix(control) + '.release-summary.json')).read_text())
    require(summary['recovery'] == plan['recovery'], 'verification 与本机重新核对的原成功生产者不一致')
    for directory in ('binaries', 'original-toolchain'):
        for path in (root / 'original-data/evidence' / directory).iterdir():
            require(path.read_bytes() == (root / 'evidence' / directory / path.name).read_bytes(),
                    'verification 中的原证明字节与原 artifact 不一致')
    verify_pipeline(plan, root)
    supply.save(root / 'state.json', plan)
    if local_recheck(plan)['skip']:
        release.output({'skip': True, 'state': 'already_published', 'publisher_method': 'local-gh-maintainer-token'})
        return
    supply.installed('oras')
    with tempfile.TemporaryDirectory(prefix='release-recovery-auth-') as credentials:
        registry_config = str(Path(credentials) / 'registry.json')
        def login_registry():
            subprocess.run(['oras', 'login', '--registry-config', registry_config, '--username', user['login'], '--password-stdin', 'ghcr.io'],
                           input=token + '\n', text=True, capture_output=True, check=True)
        publisher = {'method': 'local-gh-maintainer-token', 'login': user['login'], 'control_sha': control['control_sha'],
                     'started_at': datetime.now(timezone.utc).isoformat(), 'registry_config': registry_config}
        publish_verified(plan, lambda: local_recheck(plan), publisher, login_registry)


def owned_draft(draft, plan, marker):
    require(numeric(draft['id']) and draft['draft'] and draft['tag_name'] == plan['tag']
            and draft['target_commitish'] == plan['sha'] and marker in (draft.get('body') or ''),
            '已有 draft 缺少精确恢复绑定，禁止覆盖')


def existing_asset(remote, name, path):
    matches = [asset for asset in remote if asset['name'] == name]
    require(len(matches) <= 1, '既有 Release 资产重名，禁止覆盖')
    if matches:
        require(published_asset(matches[0], name, path), '既有正式资产内容不匹配，禁止覆盖')
        return matches[0]
    return None


def publication_state(plan, assets, marker):
    # tag 端点只返回 published Release，不能据其 404 推断没有 partial draft。
    # 维护者可见的分页列表才是唯一发现入口；匹配后始终按 ID 读取真实状态。
    endpoint = f'repos/{release.REPOSITORY}/releases'
    matches = [row for row in release.pages(endpoint) if row['tag_name'] == plan['tag']]
    require(len(matches) <= 1, '同 tag 存在多个 Release，禁止猜测唯一恢复对象')
    if not matches:
        return None
    selected_id = matches[0]['id']
    require(numeric(selected_id), 'Release ID 无效')
    current = release.api(endpoint + f'/{selected_id}')
    require(current['id'] == selected_id and current['tag_name'] == plan['tag'], 'Release ID/tag 绑定已变化')
    if current['draft']:
        owned_draft(current, plan, marker)
        remote = release.pages(endpoint + f'/{selected_id}/assets')
        for name, path in assets.items():
            existing_asset(remote, name, path)
    return current


def publish_verified(plan, check_binding, publisher, login_registry):
    if check_binding()['skip']:
        release.output({'skip': True, 'state': 'already_published'})
        return
    assets, summary = release_assets(plan)
    prefix = verification_prefix(plan['recovery_control'])
    publication_id = uuid.uuid4().hex
    record = {**{k: v for k, v in publisher.items() if k != 'registry_config'}, 'publication_id': publication_id, 'purpose': 'publication attempt; completion is recorded by GitHub Release',
              'source_sha': plan['sha'], 'tag': plan['tag'], 'ci_run_id': plan['ci_run_id'], 'digest': plan['expected_digest'],
              'verification': plan['verification'], 'recovery': plan['recovery'],
              'assets': [{'name': name, 'sha256': supply.sha256(path)} for name, path in assets.items()]}
    record_path = Path(plan['state']).parent / (prefix + '.local-publisher-' + publication_id + '.json')
    supply.save(record_path, record)
    assets[record_path.name] = record_path
    checksums = record_path.with_name(prefix + '.local-publication-' + publication_id + '.checksums.txt')
    checksums.write_text(''.join(f'{supply.sha256(path)}  {name}\n' for name, path in sorted(assets.items())))
    assets[checksums.name] = checksums
    marker = f'<!-- recovery-binding {plan["original_run_id"]} {plan["sha"]} {plan["ci_run_id"]} {plan["expected_digest"]} -->'
    # 首次 registry/API 写入前拒绝未知 draft、歧义对象及同名不同内容资产。
    prior = publication_state(plan, assets, marker)
    if check_binding()['skip']:
        release.output({'skip': True, 'state': 'already_published'})
        return
    require(prior is None or prior['draft'], '已有正式 Release 未通过完成授权，禁止写入')
    login_registry()
    # 只推进已经完整回读核验的既有 digest；不 push、build 或移动版本标签。
    subprocess.run(['oras', 'tag', '--registry-config', publisher['registry_config'], release.IMAGE + '@' + plan['expected_digest'], 'latest'], check=True)
    latest = subprocess.check_output(['oras', 'resolve', '--registry-config', publisher['registry_config'], release.IMAGE + ':latest'], text=True).strip()
    require(latest == plan['expected_digest'], 'latest 回读 digest 不等于已验证镜像')
    if check_binding()['skip']:
        release.output({'skip': True, 'state': 'already_published'})
        return
    section_marker = f'<!-- {prefix}.local-publisher-{publication_id} -->'
    section = (f'{section_marker}\n恢复控制及原成功生产者的完整证明：\n```json\n{summary}```\n'
               f'\n真实 publisher：`{publisher["method"]}`，维护者 `{publisher["login"]}`。\n'
               f'```json\n{json.dumps(record, ensure_ascii=False, indent=2)}\n```\n'
               f'\n固定镜像：`{release.IMAGE}@{plan["expected_digest"]}`\n')
    endpoint = f'repos/{release.REPOSITORY}/releases'
    draft = publication_state(plan, assets, marker)
    if check_binding()['skip']:
        release.output({'skip': True, 'state': 'already_published'})
        return
    require(prior is None or (draft is not None and draft['id'] == prior['id']), '受控 draft 身份已变化，禁止创建另一 Release')
    if draft is None:
        draft = release.api(endpoint, {'tag_name': plan['tag'], 'target_commitish': plan['sha'], 'draft': True,
            'prerelease': True, 'name': f'Sub2API {plan["version"]} (Simple)', 'body': marker + '\n\n' + section})
    owned_draft(draft, plan, marker)
    for name, path in assets.items():
        if check_binding()['skip']:
            release.output({'skip': True, 'state': 'already_published'})
            return
        remote = release.pages(endpoint + f'/{draft["id"]}/assets')
        current = release.api(endpoint + f'/{draft["id"]}')
        require(current['id'] == draft['id'], '受控 draft ID 已变化，禁止上传')
        owned_draft(current, plan, marker)
        if existing_asset(remote, name, path) is None:
            uploaded = upload_asset(draft['id'], name, path)
            require(published_asset(uploaded, name, path), '新 Release 资产回读 digest/size 不匹配')
    if check_binding()['skip']:
        release.output({'skip': True, 'state': 'already_published'})
        return
    current = release.api(endpoint + f'/{draft["id"]}')
    require(current['id'] == draft['id'], '受控 draft ID 已变化，禁止完成发布')
    owned_draft(current, plan, marker)
    remote = release.pages(endpoint + f'/{draft["id"]}/assets')
    for name, path in assets.items():
        require(existing_asset(remote, name, path) is not None, '正式发布前资产缺失')
    body = current['body']
    if section_marker not in body:
        body += '\n\n' + section
    # 保留既有说明与资产；唯一更新是追加本次证明并将受控 draft 置为正式。
    completed = release.api(endpoint + f'/{draft["id"]}', {'draft': False, 'body': body}, method='PATCH')
    require(not completed['draft'] and completed['tag_name'] == plan['tag']
            and completed['target_commitish'] == plan['sha'] and marker in completed['body']
            and check_binding()['skip'], 'GitHub 或最终来源/CI/镜像回读未确认恢复发布完成')
    release.output({'skip': False, 'state': 'published', 'tag': plan['tag'], 'sha': plan['sha'],
                    'digest': plan['expected_digest'], 'recovery': plan['recovery'], 'publisher': {k: v for k, v in publisher.items() if k != 'registry_config'}, 'release_url': completed['html_url']})


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    commands = parser.add_subparsers(dest='command', required=True)
    for name, function in (('prepare', prepare), ('verify', verify), ('publish-local', publish_local)):
        command = commands.add_parser(name)
        if name != 'publish-local':
            command.add_argument('--state', required=True)
        if name in ('prepare', 'publish-local'):
            for argument in ('original-run-id', 'sha', 'tag', 'ci-run-id', 'expected-digest'):
                command.add_argument('--' + argument, required=True)
        if name == 'publish-local':
            command.add_argument('--verification-run-id', required=True)
            command.add_argument('--output', required=True, help='保存本次重新核对的数据及 publisher 记录的全新目录')
            command.add_argument('--auth', choices=('gh-maintainer',), required=True)
        command.set_defaults(run=function)
    args = parser.parse_args()
    if args.command == 'publish-local':
        args.run(args)
        return
    require(os.environ.get('DRY_RUN') == 'false' and numeric(os.environ.get('CI_RUN_ID', '')),
            '恢复运行必须显式绑定正式发布及固定 CI run')
    if args.command == 'prepare':
        require(args.ci_run_id == os.environ['CI_RUN_ID'], '恢复输入 CI 与运行环境不一致')
    else:
        require(load_plan(args.state)['ci_run_id'] == os.environ['CI_RUN_ID'], '恢复状态 CI 与运行环境不一致')
    args.run(args)


if __name__ == '__main__':
    try:
        main()
    except (release.SyncError, ValueError, KeyError, OSError, subprocess.CalledProcessError, zipfile.BadZipFile) as error:
        raise SystemExit(str(error))
