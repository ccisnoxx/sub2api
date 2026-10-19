"""恢复入口、原生产者运输、只读 OCI 与不可覆盖发布的可观察合同。"""

import argparse
import copy
import hashlib
import io
import json
import os
from pathlib import Path
import shutil
import stat
import subprocess
import sys
import tarfile
import tempfile
import unittest
from unittest.mock import patch
import warnings
import zipfile

import yaml
import release_recovery as recovery
import test_supply_chain as image_fixture

ROOT = Path(__file__).resolve().parents[2]
REAL_PAGES = recovery.release.pages
SHA = image_fixture.SHA
CONTROL = 'c' * 40
OLD_CONTROL = 'd' * 40
TAG = 'v9.8.7-klno.2-tps.1'
VERSION = TAG[1:]
ENV = {**image_fixture.ENV, 'GITHUB_SHA': CONTROL, 'GITHUB_RUN_ID': '9000', 'GITHUB_REF': 'refs/heads/main',
       'GITHUB_EVENT_NAME': 'workflow_dispatch', 'GITHUB_WORKFLOW_SHA': CONTROL, 'DRY_RUN': 'false',
       'GITHUB_WORKFLOW_REF': 'ccisnoxx/sub2api/.github/workflows/release-recovery.yml@refs/heads/main', 'GH_TOKEN': 'fixture'}


def encoded(value):
    return json.dumps(value, sort_keys=True).encode()


def zipped(files):
    output = io.BytesIO()
    with zipfile.ZipFile(output, 'w') as archive:
        for name, data in files.items():
            archive.writestr(name, data)
    return output.getvalue()


class ReleaseRecoveryTest(unittest.TestCase):
    def setUp(self):
        self.temp = tempfile.TemporaryDirectory()
        self.addCleanup(self.temp.cleanup)
        self.root = Path(self.temp.name)
        previous = Path.cwd()
        os.chdir(self.root)
        self.addCleanup(os.chdir, previous)
        for name in ('.goreleaser.yaml', '.goreleaser.simple.yaml'):
            shutil.copyfile(ROOT / name, self.root / name)
        environment = patch.dict(os.environ, ENV)
        environment.start()
        self.addCleanup(environment.stop)
        self.run = {'id': 1000, 'workflow_id': 42, 'path': '.github/workflows/release.yml',
                    'head_sha': SHA, 'head_branch': 'personal', 'event': 'workflow_dispatch', 'status': 'completed',
                    'conclusion': 'failure', 'run_attempt': 1, 'repository': {'full_name': recovery.release.REPOSITORY, 'id': 10},
                    'head_repository': {'full_name': recovery.release.REPOSITORY, 'id': 10}}
        self.current = {**self.run, 'id': 9000, 'workflow_id': 43, 'path': '.github/workflows/release-recovery.yml',
                        'head_sha': CONTROL, 'head_branch': 'main', 'status': 'in_progress', 'conclusion': None}
        self.jobs = [{'id': i + 10, 'name': name, 'run_id': 1000, 'run_attempt': 1, 'head_sha': SHA,
                      'status': 'completed', 'conclusion': 'failure' if name == 'release' else 'success',
                      'started_at': '2026-10-11T12:00:00Z', 'completed_at': '2026-10-11T12:05:00Z'}
                     for i, name in enumerate(('prepare', 'build-frontend', 'build-binaries (linux, amd64)', 'release'))]
        self.old_guard = {'control-sha': (OLD_CONTROL + '\n').encode(),
                          'release-tools/supply_chain.py': 'raise SystemExit("旧 artifact 被执行")\n'.encode(),
                          'release-tools/personal_release.py': 'raise SystemExit("旧 guard 被执行")\n'.encode(),
                          'personal-sync/sync.py': 'raise SystemExit("旧 sync 被执行")\n'.encode(),
                          'release-tools/supply-chain-tools.json': (ROOT / '.github/release-tools/supply-chain-tools.json').read_bytes()}
        self.archive_name = recovery.matrix.archive_name(VERSION, recovery.TARGET)
        license_data = {}
        for name, path in recovery.matrix.legal_paths().items():
            path.parent.mkdir(parents=True, exist_ok=True)
            path.write_text(name + ': 许可夹具\n')
            license_data[name] = path.read_bytes()
        digests = {name: hashlib.sha256(data).hexdigest() for name, data in license_data.items()}
        files = {**license_data, 'SOURCE.json': encoded(recovery.matrix.source_metadata(SHA, VERSION, digests)), 'sub2api': b'binary fixture'}
        self.archive = self.root / self.archive_name
        with tarfile.open(self.archive, 'w:gz') as archive:
            for name, data in files.items():
                member = tarfile.TarInfo(name)
                member.size = len(data)
                archive.addfile(member, io.BytesIO(data))
        self.report = {'source_sha': SHA, 'workflow_sha': SHA, 'run_id': '1000',
                       'actual': {'python': '3.12.13'}, 'inputs_sha256': {'fixture': 'a' * 64}}
        self.proof_name = self.archive_name + '.run-1000-attempt-1.provenance.json'
        sbom_path = self.root / (self.archive_name + '.run-1000-attempt-1.spdx.json')
        recovery.supply.save(sbom_path, image_fixture.sbom())
        report_bytes = encoded(self.report)
        policy = json.loads(self.old_guard['release-tools/supply-chain-tools.json'])
        materials = [
            {'uri': 'git+https://github.com/' + recovery.release.REPOSITORY, 'digest': {'gitCommit': SHA}},
            {'uri': 'binary-toolchain.json', 'digest': {'sha256': hashlib.sha256(report_bytes).hexdigest()},
             'annotations': {'actual': {**self.report['actual'], 'syft': policy['syft']['version']}, 'inputs_sha256': self.report['inputs_sha256']}},
            {'uri': 'supply-chain-tools.json', 'digest': {'sha256': hashlib.sha256(self.old_guard['release-tools/supply-chain-tools.json']).hexdigest()}},
        ]
        with patch.dict(os.environ, {'GITHUB_SHA': SHA, 'GITHUB_RUN_ID': '1000'}):
            self.proof = recovery.supply.provenance([recovery.supply.subject(self.archive), recovery.supply.subject(sbom_path)],
                                                  SHA, VERSION, recovery.TARGET, materials)
        self.files = {
            'version-file': {'VERSION': (VERSION + '\n').encode()}, 'release-guard': self.old_guard,
            'release-binary-linux-amd64': {self.archive_name: self.archive.read_bytes(), 'manifest-linux-amd64.json': encoded({
                'sha': SHA, 'version': VERSION, 'target': recovery.TARGET, 'archive': self.archive_name,
                'sha256': recovery.supply.sha256(self.archive)})},
            'release-evidence-binary-linux-amd64': {self.proof_name: encoded(self.proof), sbom_path.name: sbom_path.read_bytes()},
            'toolchain-binary-linux-amd64': {'binary-toolchain.json': report_bytes},
        }
        self.artifacts = []
        self.downloads = {}
        self.refresh_transport()
        self.layout = self.make_layout()
        self.digest = json.loads((self.layout / 'index.json').read_text())['manifests'][0]['digest']
        self.draft = None
        self.assets = []
        self.writes = []
        self.commands = []
        self.upload_failure = None
        self.remote_main = CONTROL
        self.local_count = 0
        self.verification_artifacts = []
        self.verification_job = None
        self.active_writers = []
        self.personal_writers = []
        self.release_queries = []
        self.extra_releases = []
        self.asset_bytes = {}
        self.authorization_digest = self.digest
        for target, replacement in (
            ('urlopen', lambda *a, **k: self.fail('测试禁止真实网络')),
            ('build_opener', lambda *a: self),
            ('control_git', self.git),
        ):
            if target == 'control_git':
                patcher = patch.object(recovery.release, 'git', side_effect=replacement)
            else:
                patcher = patch.object(recovery, target, replacement)
            patcher.start()
            self.addCleanup(patcher.stop)
        for obj, name, replacement in (
            (recovery.release, 'api', self.api), (recovery.release, 'pages', self.pages),
            (recovery.release, 'publication_authorization', self.authorization),
            (recovery.release, 'output', lambda value: None),
            (recovery.subprocess, 'run', self.command), (recovery.subprocess, 'check_output', self.old_file),
            (recovery.supply, 'installed', lambda name: '1.3.4'),
            (recovery, 'upload_asset', self.upload),
        ):
            patcher = patch.object(obj, name, side_effect=replacement)
            patcher.start()
            self.addCleanup(patcher.stop)

    def make_layout(self):
        layout = self.root / 'published-oci'
        (layout / 'blobs/sha256').mkdir(parents=True)
        def put(value):
            data = encoded(value)
            digest = hashlib.sha256(data).hexdigest()
            (layout / 'blobs/sha256' / digest).write_bytes(data)
            return {'digest': 'sha256:' + digest, 'size': len(data)}
        config = {'os': 'linux', 'architecture': 'amd64', 'config': {'Labels': {
            'org.opencontainers.image.source': 'https://github.com/' + recovery.release.REPOSITORY,
            'org.opencontainers.image.revision': SHA, 'org.opencontainers.image.version': VERSION,
            'org.opencontainers.image.created': image_fixture.DATE}}}
        leaf = put({'config': put(config), 'layers': [put({'runtime': 'fixture'})]})
        leaf['platform'] = {'os': 'linux', 'architecture': 'amd64'}
        statements = [{'_type': recovery.supply.STATEMENT, 'predicateType': kind, 'predicate': predicate,
                       'subject': [{'name': 'image', 'digest': {'sha256': leaf['digest'][7:]}}]}
                      for kind, predicate in ((recovery.supply.SPDX, image_fixture.sbom()),
                                              (recovery.supply.SLSA, {'buildDefinition': {}}))]
        attestation = put({'config': put({}), 'layers': [put(s) for s in statements]})
        attestation['annotations'] = {'vnd.docker.reference.type': 'attestation-manifest', 'vnd.docker.reference.digest': leaf['digest']}
        root = put({'schemaVersion': 2, 'manifests': [leaf, attestation]})
        recovery.supply.save(layout / 'index.json', {'manifests': [root, leaf, attestation]})
        return layout

    def refresh_transport(self):
        self.artifacts.clear()
        self.downloads.clear()
        for i, (name, files) in enumerate(self.files.items()):
            data = zipped(files)
            self.downloads[i + 20] = data
            self.artifacts.append({'id': i + 20, 'name': name, 'digest': 'sha256:' + hashlib.sha256(data).hexdigest(),
                'size_in_bytes': len(data), 'expired': False, 'created_at': '2026-10-11T12:01:00Z',
                'updated_at': '2026-10-11T12:01:00Z', 'workflow_run': {'id': 1000, 'head_sha': SHA, 'head_branch': 'personal',
                    'repository_id': 10, 'head_repository_id': 10}})

    def open(self, req, timeout):
        return io.BytesIO(self.downloads[int(req.full_url.split('/')[-2])])

    def git(self, *args):
        self.assertEqual(args, ('ls-remote', '--heads', 'origin', 'refs/heads/main'))
        return self.remote_main + '\trefs/heads/main'

    def old_file(self, args, **kwargs):
        if args[0] == 'oras':
            self.assertEqual(args[:3], ['oras', 'resolve', '--registry-config'])
            self.assertEqual(args[-1], recovery.release.IMAGE + ':latest')
            return self.digest + '\n'
        if args[:3] == ['gh', 'auth', 'token']:
            self.assertEqual(args, ['gh', 'auth', 'token', '--hostname', 'github.com'])
            self.assertNotIn('GH_TOKEN', kwargs['env'])
            self.assertNotIn('GITHUB_TOKEN', kwargs['env'])
            return 'fixture-maintainer-token\n'
        if args[:2] == ['git', '-C']:
            if args[3] == 'rev-parse':
                return CONTROL + '\n'
            if args[3] == 'status':
                return ''
            if args[3] == 'ls-tree':
                return '.github/release-tools/release_recovery.py\n'
            if args[3] == 'show':
                return (ROOT / '.github/release-tools/release_recovery.py').read_bytes()
            raise AssertionError(args)
        self.assertEqual(args[:2], ['git', 'show'])
        return self.old_guard[args[2].split(':.github/')[1]]

    def command(self, args, **kwargs):
        self.commands.append(args)
        if args[:2] == ['git', 'merge-base']:
            self.assertEqual(args[2:], ['--is-ancestor', OLD_CONTROL, CONTROL])
        elif args[:3] == ['oras', 'cp', '--to-oci-layout']:
            self.assertEqual(args[3], recovery.release.IMAGE + '@' + self.digest)
            shutil.copytree(self.layout, args[4].removesuffix(':verified'))
        elif args[:2] == ['oras', 'login']:
            self.assertEqual(args[2:3], ['--registry-config'])
            self.assertEqual(args[4:], ['--username', 'maintainer', '--password-stdin', 'ghcr.io'])
            self.assertEqual(kwargs['input'], 'fixture-maintainer-token\n')
        else:
            self.assertEqual(args[:3], ['oras', 'tag', '--registry-config'])
            self.assertEqual(args[-2:], [recovery.release.IMAGE + '@' + self.digest, 'latest'])
        return subprocess.CompletedProcess(args, 0)

    def authorization(self, sha, tag, ci_run_id):
        self.assertEqual((sha, tag, ci_run_id), (SHA, TAG, ENV['CI_RUN_ID']))
        return {'skip': bool(self.draft and not self.draft['draft']), 'reuse_image': True,
                'digest': self.authorization_digest, 'ci_run_id': int(ENV['CI_RUN_ID'])}

    def api(self, path, payload=None, missing=False, method=None):
        if path == 'user':
            return {'login': 'maintainer'}
        if path == 'repos/' + recovery.release.REPOSITORY:
            return {'id': 10, 'permissions': {'push': True}}
        if '/actions/workflows/' in path:
            return {'id': 43 if path.endswith('release-recovery.yml') else 42}
        if path.endswith('/actions/runs/1000'):
            return copy.deepcopy(self.run)
        if path.endswith('/actions/runs/9000'):
            return copy.deepcopy(self.current)
        if '/releases/tags/' in path:
            # GitHub tag 端点只发现已发布 Release；partial draft 在这里是 404。
            return copy.deepcopy(self.draft) if self.draft and not self.draft['draft'] else None
        if path.endswith('/releases') and payload:
            if self.draft is not None:
                raise recovery.release.SyncError('模拟真实服务：同 tag draft 已存在，重复 POST 为 422')
            self.writes.append((path, payload, method))
            self.draft = {**payload, 'id': 50, 'html_url': 'https://github.com/ccisnoxx/sub2api/releases/tag/' + TAG}
            return copy.deepcopy(self.draft)
        if path.endswith('/releases/50'):
            if payload:
                self.writes.append((path, payload, method))
                self.assertEqual(method, 'PATCH')
                self.draft.update(payload)
            return copy.deepcopy(self.draft)
        raise AssertionError(path)

    def pages(self, path, key=None):
        if '/actions/workflows/' in path and '/runs' in path:
            self.release_queries.append(path)
            rows = self.personal_writers if '/personal-release.yml/' in path else self.active_writers
            if '?head_sha=' in path:
                sha = path.split('?head_sha=')[1]
                rows = [row for row in rows if row.get('head_sha', sha) == sha]
            return copy.deepcopy(rows)
        if '/actions/runs/9000/attempts/' in path:
            return [copy.deepcopy(self.verification_job)]
        if '/actions/runs/9000/artifacts' in path:
            return copy.deepcopy(self.verification_artifacts)
        if '/jobs?' in path:
            return copy.deepcopy(self.jobs)
        if path.endswith('/artifacts'):
            return copy.deepcopy(self.artifacts)
        if path.endswith('/releases/50/assets'):
            return copy.deepcopy(self.assets)
        if path.endswith('/releases'):
            return copy.deepcopy(([self.draft] if self.draft else []) + self.extra_releases)
        raise AssertionError(path)

    def upload(self, release_id, name, path):
        if self.upload_failure is not None and len(self.assets) == self.upload_failure:
            raise recovery.release.SyncError('模拟上传失败')
        self.assertEqual(release_id, 50)
        self.assertFalse(any(asset['name'] == name for asset in self.assets))
        asset = {'name': name, 'state': 'uploaded', 'size': path.stat().st_size, 'digest': 'sha256:' + recovery.supply.sha256(path)}
        self.assets.append(asset)
        self.asset_bytes[name] = path.read_bytes()
        return copy.deepcopy(asset)

    def args(self, directory='recovery'):
        return argparse.Namespace(state=str(self.root / directory / 'state.json'), original_run_id='1000', sha=SHA, tag=TAG,
                                  ci_run_id=ENV['CI_RUN_ID'], expected_digest=self.digest)

    def prepared(self, directory='recovery'):
        args = self.args(directory)
        recovery.prepare(args)
        return args

    def verified(self, directory='recovery'):
        args = self.prepared(directory)
        recovery.verify(args)
        return args

    def verification_transport(self, args):
        self.current.update(status='completed', conclusion='success')
        attempt = self.current['run_attempt']
        self.verification_job = {**self.jobs[-1], 'id': 100, 'run_id': 9000, 'head_sha': CONTROL,
                                 'run_attempt': attempt, 'conclusion': 'success'}
        evidence = Path(args.state).parent / 'evidence'
        data = zipped({str(path.relative_to(evidence)): path.read_bytes() for path in evidence.rglob('*') if path.is_file()})
        self.downloads[90] = data
        self.verification_artifacts = [{'id': 90, 'name': f'release-recovery-supply-chain-9000-{attempt}',
            'digest': 'sha256:' + hashlib.sha256(data).hexdigest(), 'size_in_bytes': len(data), 'expired': False,
            'created_at': '2026-10-11T12:01:00Z', 'updated_at': '2026-10-11T12:01:00Z',
            'workflow_run': {'id': 9000, 'head_sha': CONTROL, 'head_branch': 'main', 'repository_id': 10, 'head_repository_id': 10}}]

    def local_args(self):
        self.local_count += 1
        return argparse.Namespace(original_run_id='1000', sha=SHA, tag=TAG, ci_run_id=ENV['CI_RUN_ID'],
            expected_digest=self.digest, verification_run_id='9000', auth='gh-maintainer', output=str(self.root / f'local-{self.local_count}'))

    def publish(self, args):
        self.verification_transport(args)
        recovery.publish_local(self.local_args())

    def controlled_draft(self):
        return {'id': 50, 'tag_name': TAG, 'target_commitish': SHA, 'draft': True,
                'body': f'<!-- recovery-binding 1000 {SHA} {ENV["CI_RUN_ID"]} {self.digest} -->',
                'html_url': 'https://github.com/ccisnoxx/sub2api/releases/tag/' + TAG}

    def test_complete_recovery_preserves_original_producer_and_only_publishes_new_proofs(self):
        args = self.verified()
        plan = recovery.load_plan(args.state)
        original = Path(args.state).parent / 'evidence/binaries' / self.proof_name
        self.assertEqual(original.read_bytes(), encoded(self.proof))
        self.assertEqual(os.environ['GITHUB_SHA'], CONTROL)
        self.assertEqual(os.environ['GITHUB_RUN_ID'], '9000')
        assets, text = recovery.release_assets({**plan, 'state': args.state})
        summary = json.loads(text)
        self.assertEqual(summary['recovery']['original_release']['workflow_sha'], SHA)
        self.assertEqual(summary['recovery']['original_release']['control_sha'], OLD_CONTROL)
        self.assertEqual(summary['recovery']['binary_producer']['run_id'], '1000')
        self.assertIsNone(summary['release_run'])
        self.assertEqual(summary['publication'], {'method': 'verification-only', 'performed': False})
        pipeline = next(p for p in assets.values() if p.name.endswith('.pipeline-provenance.json'))
        self.assertIn('/release-recovery.yml@' + CONTROL, json.loads(pipeline.read_text())['predicate']['buildDefinition']['buildType'])
        self.publish(args)
        self.assertFalse(self.draft['draft'])
        self.assertEqual(len(self.assets), len(assets) + 2)
        record = json.loads(next(data for name, data in self.asset_bytes.items() if '.local-publisher-' in name))
        self.assertEqual(record['method'], 'local-gh-maintainer-token')
        self.assertEqual(record['login'], 'maintainer')
        self.assertEqual(record['verification']['run']['id'], 9000)
        self.assertNotIn('registry_config', record)
        self.assertTrue(all('run-9000-attempt-1' in a['name'] for a in self.assets))
        self.assertFalse(any(a['name'].endswith('.tar.gz') for a in self.assets))
        self.assertEqual([c[1] for c in self.commands if c[0] == 'oras'], ['cp', 'login', 'tag'])
        self.assertFalse(Path(next(c[3] for c in self.commands if c[:2] == ['oras', 'login'])).exists())

    def test_untrusted_current_control_and_original_runs_fail_before_data_download(self):
        for change in ({'event': 'push'}, {'head_branch': 'main'}, {'head_sha': 'e' * 40},
                       {'workflow_id': 99}, {'path': '.github/workflows/backend-ci.yml'},
                       {'conclusion': 'success'}, {'status': 'in_progress'},
                       {'repository': {'full_name': 'other/repo', 'id': 10}}):
            original = self.run
            self.run = {**original, **change}
            with self.subTest(change=change), self.assertRaises(recovery.release.SyncError):
                recovery.prepare(self.args())
            self.run = original
        for change in ({'GITHUB_REF': 'refs/heads/personal'}, {'GITHUB_EVENT_NAME': 'push'},
                       {'GITHUB_WORKFLOW_SHA': SHA}, {'GITHUB_WORKFLOW_REF': 'other/workflow'}):
            with self.subTest(change=change), patch.dict(os.environ, change), self.assertRaises(recovery.release.SyncError):
                recovery.prepare(self.args())
        self.assertFalse(Path(self.args().state).parent.exists())

    def test_failed_missing_duplicate_or_non_simple_successful_producers_are_rejected(self):
        for changed in ([j for j in self.jobs if j['name'] != 'build-frontend'], self.jobs + [self.jobs[0]],
                        [{**j, 'conclusion': 'skipped'} if j['name'] == 'prepare' else j for j in self.jobs],
                        self.jobs + [{**self.jobs[2], 'name': 'build-binaries (linux, arm64)'}]):
            with self.subTest(jobs=changed), patch.object(recovery.release, 'pages', return_value=changed), \
                 self.assertRaises(recovery.release.SyncError):
                recovery.original_run('1000', SHA)

    def test_expired_wrong_origin_missing_transport_digest_and_ambiguous_artifacts_are_rejected(self):
        for change in ({'expired': True}, {'digest': ''}, {'digest': 'sha256:invalid'},
                       {'created_at': '2026-10-11T11:59:00Z'},
                       {'workflow_run': {**self.artifacts[0]['workflow_run'], 'id': 9999}}):
            original = self.artifacts[0]
            self.artifacts[0] = {**original, **change}
            with self.subTest(change=change), self.assertRaises(recovery.release.SyncError):
                recovery.prepare(self.args())
            self.artifacts[0] = original
        self.artifacts.append(self.artifacts[0])
        with self.assertRaisesRegex(recovery.release.SyncError, '重名'):
            recovery.prepare(self.args())
        self.artifacts.pop()
        self.downloads[20] = b'changed transport'
        with self.assertRaisesRegex(recovery.release.SyncError, '运输 digest/size'):
            recovery.prepare(self.args())
        self.assertFalse(self.writes)
        self.assertFalse(self.commands)

    def test_wrong_version_original_attempt_ci_and_sbom_subject_are_rejected(self):
        changes = [lambda: self.files['version-file'].update(VERSION=b'wrong\n'),
                   lambda: self.proof['predicate']['buildDefinition']['internalParameters'].update(run_attempt='2'),
                   lambda: self.proof['predicate']['buildDefinition']['internalParameters'].update(ci_run_id='7777'),
                   lambda: self.proof['subject'][1]['digest'].update(sha256='f' * 64)]
        for i, change in enumerate(changes):
            files = copy.deepcopy(self.files)
            proof = copy.deepcopy(self.proof)
            change()
            self.files['release-evidence-binary-linux-amd64'][self.proof_name] = encoded(self.proof)
            self.refresh_transport()
            with self.subTest(i=i), self.assertRaises((ValueError, recovery.release.SyncError)):
                recovery.prepare(self.args('negative-' + str(i)))
            self.files, self.proof = files, proof
            self.refresh_transport()
        self.assertFalse(self.writes)

    def test_guard_bytes_and_toolchain_material_cannot_drift_with_valid_transport_hash(self):
        expected = copy.deepcopy(self.old_guard)
        self.files['release-guard']['release-tools/supply_chain.py'] = b'changed transported code'
        self.refresh_transport()
        with patch.object(recovery.subprocess, 'check_output', side_effect=lambda args: expected[args[2].split(':.github/')[1]]), \
             self.assertRaisesRegex(recovery.release.SyncError, 'main 控制提交'):
            recovery.prepare(self.args('wrong-guard'))
        self.files['release-guard'] = expected
        self.old_guard = expected
        self.files['toolchain-binary-linux-amd64']['binary-toolchain.json'] = encoded({**self.report, 'actual': {'python': '0.0.0'}})
        self.refresh_transport()
        with self.assertRaisesRegex(recovery.release.SyncError, '工具材料'):
            recovery.prepare(self.args('wrong-material'))
        self.assertFalse(self.writes)

    def test_binding_drift_after_prepare_never_reaches_oci_or_publication(self):
        args = self.prepared()
        for field, value in (('authorization_digest', 'sha256:' + 'e' * 64), ('remote_main', 'f' * 40)):
            original = getattr(self, field)
            setattr(self, field, value)
            with self.subTest(field=field), self.assertRaises(recovery.release.SyncError):
                recovery.verify(args)
            setattr(self, field, original)
        self.run['run_attempt'] = 2
        with self.assertRaises(recovery.release.SyncError):
            recovery.verify(args)
        self.assertFalse([c for c in self.commands if c[0] == 'oras'])
        self.assertFalse(self.writes)

    def test_partial_upload_recovery_keeps_prior_assets_and_completed_release_is_idempotent(self):
        args = self.verified()
        self.upload_failure = 1
        with self.assertRaisesRegex(recovery.release.SyncError, '模拟上传失败'):
            self.publish(args)
        old_assets = copy.deepcopy(self.assets)
        self.assertTrue(self.draft['draft'])
        self.upload_failure = None
        self.current['run_attempt'] = 2
        with patch.dict(os.environ, {'GITHUB_RUN_ATTEMPT': '2'}):
            new = self.verified('retry')
            self.publish(new)
        self.assertFalse(self.draft['draft'])
        self.assertEqual(self.assets[:1], old_assets)
        self.assertTrue(any('run-9000-attempt-2' in a['name'] for a in self.assets))
        self.assertIn('run-9000-attempt-1', self.draft['body'])
        self.assertIn('run-9000-attempt-2', self.draft['body'])
        writes = copy.deepcopy(self.writes)
        commands = copy.deepcopy(self.commands)
        for artifact in self.artifacts:
            artifact['expired'] = True
        with patch.dict(os.environ, {'GITHUB_RUN_ATTEMPT': '2'}):
            recovery.prepare(self.args('complete'))
        self.assertEqual(self.writes, writes)
        self.assertEqual(self.commands, commands)
        self.assertFalse((self.root / 'complete').exists())

    def test_existing_asset_conflict_and_unbound_draft_are_never_overwritten(self):
        args = self.verified()
        plan = {**recovery.load_plan(args.state), 'state': args.state}
        assets, _ = recovery.release_assets(plan)
        name, path = next(iter(assets.items()))
        self.draft = self.controlled_draft()
        self.assets.append({'name': name, 'state': 'uploaded', 'size': path.stat().st_size, 'digest': 'sha256:' + 'e' * 64})
        with self.assertRaisesRegex(recovery.release.SyncError, '禁止覆盖'):
            self.publish(args)
        self.assertTrue(self.draft['draft'])
        self.assertEqual(len(self.assets), 1)
        self.assertFalse(self.writes)
        self.assertFalse([command for command in self.commands if command[0] == 'oras' and command[1] in ('login', 'tag')])
        self.draft['body'] = '原有说明，没有恢复绑定'
        with self.assertRaisesRegex(recovery.release.SyncError, '禁止覆盖'):
            self.publish(args)
        self.assertEqual(self.draft['body'], '原有说明，没有恢复绑定')
        self.assertFalse(self.writes)
        self.assertFalse([command for command in self.commands if command[0] == 'oras' and command[1] in ('login', 'tag')])

    def test_tag_404_but_list_contains_partial_draft_resumes_original_id_and_assets(self):
        args = self.verified()
        self.upload_failure = 1
        with self.assertRaisesRegex(recovery.release.SyncError, '模拟上传失败'):
            self.publish(args)
        original_id = self.draft['id']
        existing_assets = copy.deepcopy(self.assets)
        self.assertIsNone(self.api(f'repos/{recovery.release.REPOSITORY}/releases/tags/{TAG}', missing=True))
        self.assertEqual(self.pages(f'repos/{recovery.release.REPOSITORY}/releases')[0]['id'], original_id)
        creations = sum(path.endswith('/releases') for path, _, _ in self.writes)
        self.upload_failure = None
        self.publish(args)
        self.assertEqual(self.draft['id'], original_id)
        self.assertEqual(self.assets[:len(existing_assets)], existing_assets)
        self.assertEqual(sum(path.endswith('/releases') for path, _, _ in self.writes), creations)
        self.assertFalse(self.draft['draft'])

    def test_multiple_same_tag_releases_fail_before_any_remote_write(self):
        args = self.verified()
        self.draft = self.controlled_draft()
        self.extra_releases = [{**self.draft, 'id': 51}]
        with self.assertRaisesRegex(recovery.release.SyncError, '多个|唯一'):
            self.publish(args)
        self.assertFalse(self.writes)
        self.assertFalse(self.assets)
        self.assertFalse([command for command in self.commands if command[0] == 'oras' and command[1] in ('login', 'tag')])

    def test_paginated_release_list_finds_draft_after_published_page(self):
        self.draft = self.controlled_draft()
        endpoint = f'repos/{recovery.release.REPOSITORY}/releases'
        requests = []
        def api_pages(path, *args, **kwargs):
            requests.append(path)
            if path == endpoint + '?per_page=100&page=1':
                return [{'id': number + 100, 'tag_name': f'v-other-{number}', 'draft': False} for number in range(100)]
            if path == endpoint + '?per_page=100&page=2':
                return [copy.deepcopy(self.draft)]
            if path == endpoint + '/50/assets?per_page=100&page=1':
                return []
            return self.api(path, *args, **kwargs)
        with patch.object(recovery.release, 'api', side_effect=api_pages), patch.object(recovery.release, 'pages', side_effect=REAL_PAGES):
            found = recovery.publication_state({'tag': TAG, 'sha': SHA}, {}, self.draft['body'])
        self.assertEqual(found['id'], 50)
        self.assertTrue(found['draft'])
        self.assertIn(endpoint + '?per_page=100&page=2', requests)
        self.assertFalse(self.writes)

    def test_evidence_mutation_stops_before_latest_or_release_write(self):
        args = self.verified()
        sbom = Path(args.state).parent / 'evidence/binaries' / (self.archive_name + '.run-1000-attempt-1.spdx.json')
        sbom.write_text('{}')
        with self.assertRaisesRegex(recovery.release.SyncError, '哈希不匹配|原证明字节'):
            self.publish(args)
        self.assertFalse([c for c in self.commands if c[:2] == ['oras', 'tag']])
        self.assertFalse(self.writes)

    def test_local_publish_requires_successful_verification_and_matching_control_without_actions_environment(self):
        args = self.verified()
        self.verification_transport(args)
        original = copy.deepcopy(self.current)
        for change in ({'event': 'push'}, {'status': 'in_progress'}, {'conclusion': 'failure'},
                       {'path': '.github/workflows/release.yml'}, {'workflow_id': 99}, {'head_branch': 'personal'},
                       {'head_repository': {'full_name': 'other/repo', 'id': 10}}, {'head_sha': 'e' * 40}):
            self.current = {**original, **change}
            with self.subTest(change=change), self.assertRaises(recovery.release.SyncError):
                recovery.publish_local(self.local_args())
        self.current = original
        self.assertFalse(self.writes)
        self.assertFalse([c for c in self.commands if c[:2] == ['oras', 'tag']])
        with patch.dict(os.environ, {'PATH': os.environ['PATH']}, clear=True):
            recovery.publish_local(self.local_args())
            self.assertFalse(any(key.startswith('GITHUB_') for key in os.environ))
        self.assertFalse(self.draft['draft'])

    def test_verification_attempt_job_and_artifact_drift_fail_before_local_writes(self):
        args = self.verified()
        self.verification_transport(args)
        job = copy.deepcopy(self.verification_job)
        for change in ({'run_attempt': 2}, {'conclusion': 'skipped'}, {'head_sha': SHA}, {'run_id': 1000}):
            self.verification_job = {**job, **change}
            with self.subTest(change=change), self.assertRaises(recovery.release.SyncError):
                recovery.publish_local(self.local_args())
        self.verification_job = job
        artifact = copy.deepcopy(self.verification_artifacts[0])
        for change in ({'expired': True}, {'digest': ''}, {'workflow_run': {**artifact['workflow_run'], 'head_sha': SHA}}):
            self.verification_artifacts[0] = {**artifact, **change}
            with self.subTest(change=change), self.assertRaises(recovery.release.SyncError):
                recovery.publish_local(self.local_args())
        self.verification_artifacts[0] = artifact
        self.downloads[90] = b'altered verification transport'
        with self.assertRaisesRegex(recovery.release.SyncError, '运输 digest/size'):
            recovery.publish_local(self.local_args())
        self.assertFalse(self.writes)
        self.assertFalse([c for c in self.commands if c[:2] == ['oras', 'tag']])

    def test_local_control_dirty_tree_is_rejected_before_publication(self):
        args = self.verified()
        self.verification_transport(args)
        def git_output(command, **kwargs):
            if command[:2] == ['git', '-C'] and command[3] == 'status':
                return ' M .github/release-tools/release_recovery.py\n'
            return self.old_file(command, **kwargs)
        with patch.object(recovery.subprocess, 'check_output', side_effect=git_output), self.assertRaisesRegex(recovery.release.SyncError, '同一已提交 main'):
            recovery.publish_local(self.local_args())
        self.assertFalse(self.writes)

    def test_local_publisher_refuses_active_actions_writers_without_cancelling_them(self):
        args = self.verified()
        self.verification_transport(args)
        for status in ('queued', 'in_progress', 'waiting'):
            self.active_writers = [{'status': status}]
            with self.subTest(status=status), self.assertRaisesRegex(recovery.release.SyncError, '并发发布'):
                recovery.publish_local(self.local_args())
        self.assertFalse(self.writes)
        self.assertFalse([c for c in self.commands if c[0] == 'oras' and c[1] in ('login', 'tag')])

    def test_old_main_personal_release_can_checkout_current_main_and_must_block_publisher(self):
        args = self.verified()
        self.verification_transport(args)
        self.assertEqual(self.remote_main, CONTROL)
        for status in ('queued', 'in_progress'):
            self.personal_writers = [{'id': 8000, 'head_sha': OLD_CONTROL, 'status': status}]
            with self.subTest(status=status), self.assertRaisesRegex(recovery.release.SyncError, '并发发布'):
                recovery.publish_local(self.local_args())
        self.assertTrue(any(path.endswith('/personal-release.yml/runs') for path in self.release_queries))
        self.assertFalse(self.writes)
        self.assertFalse([command for command in self.commands if command[0] == 'oras' and command[1] in ('login', 'tag')])


class RecoveryBoundaryTest(unittest.TestCase):
    def test_zip_traversal_symlink_and_duplicate_files_are_rejected(self):
        for name, mode, duplicate in (('../guard.py', stat.S_IFREG, False), ('/guard.py', stat.S_IFREG, False),
                                       ('guard.py', stat.S_IFLNK, False), ('guard.py', stat.S_IFREG, True)):
            output = io.BytesIO()
            with zipfile.ZipFile(output, 'w') as archive:
                info = zipfile.ZipInfo(name)
                info.external_attr = mode << 16
                archive.writestr(info, b'data')
                if duplicate:
                    with warnings.catch_warnings():
                        warnings.simplefilter('ignore', UserWarning)
                        archive.writestr(copy.copy(info), b'changed')
            with self.subTest(name=name, mode=mode, duplicate=duplicate), self.assertRaises(recovery.release.SyncError):
                recovery.zip_data(output.getvalue())

    def test_artifact_redirect_removes_github_credential(self):
        req = recovery.Request('https://api.github.com/artifact', headers={'Authorization': 'Bearer fixture'})
        redirect = recovery.ArtifactRedirect().redirect_request(req, None, 302, 'Found', {}, 'https://blob.example.invalid/object')
        self.assertIsNone(redirect.get_header('Authorization'))
        with self.assertRaises(recovery.release.SyncError):
            recovery.ArtifactRedirect().redirect_request(req, None, 302, 'Found', {}, 'http://blob.example.invalid/object')

    def test_workflow_has_only_fixed_main_control_and_no_rebuild_or_ci_dispatch(self):
        flow = yaml.load((ROOT / '.github/workflows/release-recovery.yml').read_text(), Loader=yaml.BaseLoader)
        self.assertEqual(set(flow['on']), {'workflow_dispatch'})
        self.assertEqual(set(flow['jobs']), {'release'})
        self.assertEqual(set(flow['on']['workflow_dispatch']['inputs']),
                         {'original_run_id', 'source_sha', 'tag', 'ci_run_id', 'expected_digest'})
        steps = flow['jobs']['release']['steps']
        self.assertEqual(flow['jobs']['release']['permissions'], {'contents': 'read', 'packages': 'read', 'actions': 'read', 'pull-requests': 'read'})
        checkouts = [s['with'] for s in steps if s.get('uses', '').startswith('actions/checkout@')]
        self.assertEqual(checkouts[0]['ref'], '${{ github.sha }}')
        self.assertEqual(checkouts[0]['path'], 'control')
        self.assertEqual(checkouts[1]['path'], 'source')
        self.assertFalse(any(s.get('uses', '').startswith('actions/download-artifact@') for s in steps))
        text = (ROOT / '.github/workflows/release-recovery.yml').read_text()
        for forbidden in ('setup-go', 'setup-node', 'pnpm', 'goreleaser', 'buildx', 'workflow run', 'rerun', 'publish-local', 'login-action', 'secrets.'):
            self.assertNotIn(forbidden, text)
        for s in steps:
            if 'release_recovery.py' in s.get('run', ''):
                self.assertIn('python -I "$RUNNER_TEMP/release-tools/release_recovery.py"', s['run'])

    def test_isolated_entrypoint_does_not_import_application_modules(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            shutil.copytree(ROOT / '.github/release-tools', root / 'trusted/release-tools')
            shutil.copytree(ROOT / '.github/personal-sync', root / 'trusted/personal-sync')
            for name in ('json.py', 'yaml.py', 'supply_chain.py', 'personal_release.py', 'sitecustomize.py'):
                (root / name).write_text('raise SystemExit("应用模块被执行")\n')
            result = subprocess.run([sys.executable, '-I', str(root / 'trusted/release-tools/release_recovery.py'), '--help'],
                                    cwd=root, env={**os.environ, 'PYTHONPATH': str(root)}, text=True, capture_output=True)
            self.assertEqual(result.returncode, 0, result.stderr)
            self.assertIn('prepare', result.stdout)


if __name__ == '__main__':
    unittest.main()
