"""保护 REL-001 对外验收阈值和不可变旧镜像来源合同。"""
import importlib.util
import json
from pathlib import Path
from types import SimpleNamespace
import unittest
from unittest.mock import patch

spec = importlib.util.spec_from_file_location('gateway', Path(__file__).with_name('rel001_gateway_perf.py'))
gateway = importlib.util.module_from_spec(spec)
spec.loader.exec_module(gateway)


class GatewayAcceptanceTests(unittest.TestCase):
    def arguments(self, *extra):
        return gateway.arguments(['--new-image', 'sha256:' + 'b' * 64,
                                  '--new-sha', 'b' * 40, *extra])

    def test_formal_run_cannot_weaken_either_acceptance_threshold(self):
        for extra in (('--max-throughput-drop', '0.1501'), ('--max-p95-increase', '0.2001')):
            with self.subTest(extra=extra), patch.object(gateway, 'Docker') as docker:
                with self.assertRaisesRegex(gateway.Failure, 'formal_thresholds_cannot_exceed_rel001_limits'):
                    gateway.execute(self.arguments(*extra), {}, Path('/unused'))
                docker.assert_not_called()

    def test_canonical_digest_is_accepted_when_local_image_id_is_config_digest(self):
        class DependenciesReached(Exception):
            pass

        class Docker:
            def __init__(self, *_args):
                pass

            def run(self, *_args):
                return SimpleNamespace(stdout=json.dumps({'architecture': 'amd64'}))

            def image(self, reference, sha=None):
                if sha is None:
                    raise DependenciesReached()
                return {'id': 'sha256:' + ('a' if sha == gateway.OLD_SHA else 'b') * 64,
                        'repo_digests': [gateway.OLD_IMAGE] if sha == gateway.OLD_SHA else []}

        with patch.object(gateway, 'Docker', Docker):
            # 身份预检必须通过；第三次依赖查询的哨兵保证未运行 Docker/Stack。
            with self.assertRaises(DependenciesReached):
                gateway.execute(self.arguments(), {}, Path('/unused'))

    def test_image_inspection_rejects_digest_not_present_in_repo_digests(self):
        docker = gateway.Docker('unix:///unused', Path('/unused'))
        inspected = {'id': 'sha256:' + 'a' * 64, 'repo_digests': [], 'os': 'linux',
                     'architecture': 'amd64', 'revision': gateway.OLD_SHA}
        with patch.object(docker, 'run', return_value=SimpleNamespace(stdout=json.dumps(inspected))):
            with self.assertRaisesRegex(gateway.Failure, 'requested_digest_not_inspected'):
                docker.image(gateway.OLD_IMAGE, gateway.OLD_SHA)


if __name__ == '__main__':
    unittest.main()
