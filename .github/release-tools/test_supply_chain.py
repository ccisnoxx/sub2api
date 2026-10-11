"""验证 SBOM、源码/run 绑定及 OCI 证明失败关闭的可观察合同。"""

import argparse
import copy
import hashlib
import importlib.util
import io
import json
import os
from pathlib import Path
import tempfile
import tarfile
import unittest
from unittest.mock import patch

import yaml

ROOT = Path(__file__).resolve().parents[2]
spec = importlib.util.spec_from_file_location('supply_chain', Path(__file__).with_name('supply_chain.py'))
supply = importlib.util.module_from_spec(spec)
spec.loader.exec_module(supply)

SHA = 'a' * 40
DATE = '2026-10-11T12:00:00Z'
ENV = {'GITHUB_REPOSITORY': 'ccisnoxx/sub2api', 'GITHUB_SHA': 'b' * 40,
       'GITHUB_RUN_ID': '1234', 'CI_RUN_ID': '5678', 'DRY_RUN': 'true'}


def sbom():
    return {'spdxVersion': 'SPDX-2.3', 'SPDXID': 'SPDXRef-DOCUMENT', 'dataLicense': 'CC0-1.0',
            'packages': [{'SPDXID': 'SPDXRef-dependency', 'name': 'example', 'versionInfo': '1.0.0'}]}


class SupplyChainTest(unittest.TestCase):
    def setUp(self):
        self.temp = tempfile.TemporaryDirectory()
        self.addCleanup(self.temp.cleanup)
        self.directory = Path(self.temp.name)
        self.environment = patch.dict(os.environ, ENV)
        self.environment.start()
        self.addCleanup(self.environment.stop)

    def layout(self, mutate=None, platforms=('linux/amd64',)):
        layout = self.directory / 'oci'
        (layout / 'blobs/sha256').mkdir(parents=True, exist_ok=True)
        def put(value):
            data = json.dumps(value, sort_keys=True).encode()
            digest = hashlib.sha256(data).hexdigest()
            (layout / 'blobs/sha256' / digest).write_bytes(data)
            return {'digest': 'sha256:' + digest, 'size': len(data), 'mediaType': 'application/vnd.oci.image.manifest.v1+json'}
        descriptors = []
        for platform in platforms:
            system, arch = platform.split('/')
            config = {'os': system, 'architecture': arch, 'config': {'Labels': {
                'org.opencontainers.image.source': 'https://github.com/ccisnoxx/sub2api',
                'org.opencontainers.image.revision': SHA, 'org.opencontainers.image.version': '9.8.7',
                'org.opencontainers.image.created': DATE}}}
            if mutate:
                mutate('config', config)
            leaf = put({'schemaVersion': 2, 'config': put(config), 'layers': [put({'fixture': 'runtime'})]})
            leaf['platform'] = {'os': system, 'architecture': arch}
            descriptors.append(leaf)
            statements = [
                {'_type': supply.STATEMENT, 'subject': [{'name': 'image', 'digest': {'sha256': leaf['digest'][7:]}}],
                 'predicateType': supply.SPDX, 'predicate': sbom()},
                {'_type': supply.STATEMENT, 'subject': [{'name': 'image', 'digest': {'sha256': leaf['digest'][7:]}}],
                 'predicateType': 'https://slsa.dev/provenance/v0.2', 'predicate': {'builder': {'id': 'https://mobyproject.org/buildkit@v0.34.0'}}}]
            if mutate:
                mutate('statements', statements)
            attestation = put({'schemaVersion': 2, 'config': put({}), 'layers': [put(s) for s in statements]})
            attestation['platform'] = {'os': 'unknown', 'architecture': 'unknown'}
            attestation['annotations'] = {'vnd.docker.reference.type': 'attestation-manifest', 'vnd.docker.reference.digest': leaf['digest']}
            descriptors.append(attestation)
        index = {'schemaVersion': 2, 'manifests': descriptors}
        if mutate:
            mutate('index', index)
        root = put(index)
        supply.save(layout / 'index.json', {'schemaVersion': 2, 'manifests': [root]})
        return layout, root['digest']

    def test_real_oci_tar_digest_labels_subjects_and_full_matrix(self):
        directory, expected = self.layout(platforms=('linux/amd64', 'linux/arm64'))
        archive = self.directory / 'oci.tar'
        with tarfile.open(archive, 'w') as output:
            for path in directory.rglob('*'):
                if path.is_file():
                    output.add(path, arcname=str(path.relative_to(directory)))
        digest, platforms, proofs, date = supply.inspect_image(archive, SHA, '9.8.7', ['linux/amd64', 'linux/arm64'], DATE)
        self.assertEqual(digest, expected)
        self.assertEqual(set(platforms), {'linux/amd64', 'linux/arm64'})
        self.assertEqual(set(proofs), set(platforms.values()))
        self.assertEqual(date, DATE)

    def test_wrong_revision_source_version_created_platform_and_blob_are_rejected(self):
        for label in ('revision', 'source', 'version', 'created'):
            with self.subTest(label=label):
                def mutate(kind, value):
                    if kind == 'config':
                        value['config']['Labels']['org.opencontainers.image.' + label] = 'wrong'
                layout, _ = self.layout(mutate)
                with self.assertRaises(ValueError):
                    supply.inspect_image(layout, SHA, '9.8.7', ['linux/amd64'], DATE)
        layout, _ = self.layout()
        with self.assertRaisesRegex(ValueError, '矩阵不完整'):
            supply.inspect_image(layout, SHA, '9.8.7', ['linux/amd64', 'linux/arm64'], DATE)
        root = json.loads((layout / 'index.json').read_text())['manifests'][0]['digest'][7:]
        (layout / 'blobs/sha256' / root).write_text('tampered')
        with self.assertRaisesRegex(ValueError, 'digest/size'):
            supply.inspect_image(layout, SHA, '9.8.7', ['linux/amd64'], DATE)

    def test_missing_provenance_sbom_and_wrong_subject_fail_closed(self):
        def missing(kind, value):
            if kind == 'statements':
                value.pop()
        def empty(kind, value):
            if kind == 'statements':
                value[0]['predicate']['packages'] = []
        def wrong(kind, value):
            if kind == 'statements':
                value[0]['subject'][0]['digest']['sha256'] = 'c' * 64
        for mutate in (missing, empty, wrong):
            with self.subTest(mutate=mutate.__name__):
                layout, _ = self.layout(mutate)
                with self.assertRaises(ValueError):
                    supply.inspect_image(layout, SHA, '9.8.7', ['linux/amd64'], DATE)

    def test_recovery_uses_existing_created_and_never_rewrites_it(self):
        layout, _ = self.layout()
        actual = supply.inspect_image(layout, SHA, '9.8.7', ['linux/amd64'])[-1]
        self.assertEqual(actual, DATE)
        with self.assertRaises(ValueError):
            supply.inspect_image(layout, SHA, '9.8.7', ['linux/amd64'], '2026-10-12T12:00:00Z')

    def binary_evidence(self):
        target = {'goos': 'linux', 'goarch': 'amd64'}
        archive = self.directory / 'sub2api_9.8.7_linux_amd64.tar.gz'
        archive.write_bytes(b'actual archive')
        evidence = self.directory / 'binaries'
        document = evidence / (archive.name + '.spdx.json')
        supply.save(document, sbom())
        proof = evidence / (archive.name + '.provenance.json')
        supply.save(proof, supply.provenance([supply.subject(archive), supply.subject(document)], SHA, '9.8.7', target, []))
        args = argparse.Namespace(input=str(self.directory), evidence=str(evidence), sha=SHA, version='9.8.7', simple=True)
        return args, archive, proof

    def test_binary_archive_sbom_workflow_and_run_binding(self):
        args, archive, proof = self.binary_evidence()
        supply.verify_binaries(args)
        original = json.loads(proof.read_text())
        changes = [lambda s: s['subject'][0]['digest'].update(sha256='0' * 64),
                   lambda s: s['predicate']['buildDefinition']['externalParameters'].update(source_sha='c' * 40),
                   lambda s: s['predicate']['buildDefinition']['internalParameters'].update(workflow_sha='c' * 40),
                   lambda s: s['predicate']['runDetails']['metadata'].update(invocationId='https://github.com/other/repo/actions/runs/1234')]
        for mutate in changes:
            value = copy.deepcopy(original)
            mutate(value)
            supply.save(proof, value)
            with self.assertRaisesRegex(ValueError, '绑定不匹配'):
                supply.verify_binaries(args)
        supply.save(proof, original)
        (Path(args.evidence) / 'extra.json').write_text('{}')
        with self.assertRaisesRegex(ValueError, '额外文件'):
            supply.verify_binaries(args)
        (Path(args.evidence) / 'extra.json').unlink()
        archive.write_bytes(b'changed archive')
        with self.assertRaises(ValueError):
            supply.verify_binaries(args)

    def test_final_evidence_and_formal_ci_are_required(self):
        layout, digest = self.layout()
        dest = self.directory / 'evidence/image-amd64-built'
        args = argparse.Namespace(layout=str(layout), sha=SHA, version='9.8.7', platforms=['linux/amd64'], created=DATE, output=str(dest))
        supply.image(args)
        final = argparse.Namespace(input=str(dest.parent), sha=SHA, version='9.8.7', simple=True)
        supply.finalize(final)
        summary = json.loads((dest.parent / 'summary/release-summary.json').read_text())
        self.assertTrue(summary['dry_run'])
        self.assertEqual(summary['source_sha'], SHA)
        self.assertEqual(summary['images'][0]['digest'], digest)
        self.assertIn('unsigned', summary['signature'])
        with patch.dict(os.environ, {'DRY_RUN': 'false', 'CI_RUN_ID': ''}):
            published = dest.parent / 'image-amd64-published'
            args.output = str(published)
            supply.image(args)
            with self.assertRaisesRegex(ValueError, 'CI run'):
                supply.finalize(final)

    def test_workflow_preserves_permissions_and_generates_dry_run_materials(self):
        workflow = yaml.safe_load((ROOT / '.github/workflows/release.yml').read_text())
        self.assertEqual(workflow['permissions'], {'contents': 'read'})
        self.assertEqual(workflow['jobs']['release']['permissions'], {'contents': 'write', 'packages': 'write', 'actions': 'read', 'checks': 'read'})
        steps = workflow['jobs']['release']['steps']
        upload = next(s for s in steps if s.get('with', {}).get('name', '').startswith('release-supply-chain-'))
        self.assertNotIn('if', upload)
        self.assertNotIn('overwrite', upload['with'])
        self.assertEqual(upload['with']['if-no-files-found'], 'error')
        dry = next(s for s in steps if s.get('name') == 'Preserve dry-run OCI images with attestations')
        self.assertEqual(dry['if'], "env.DRY_RUN == 'true'")
        self.assertIn('supply_chain.py', next(s for s in steps if s.get('name') == 'Verify binary SBOM and provenance matrix')['run'])
        self.assertLess(steps.index(upload), next(i for i, s in enumerate(steps) if s.get('name') == 'Publish existing archives and release notes'))


if __name__ == '__main__':
    unittest.main()
