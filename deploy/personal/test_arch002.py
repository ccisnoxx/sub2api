"""ARCH-002 缺失合同：严格 schema、只读预检与导入前整套字节拒绝。"""
import copy
import json
import os
from pathlib import Path
import shlex
import shutil
import subprocess
import tempfile
import unittest
from unittest import mock

import deploy_hostdzire as deploy
import deploy_lifecycle
import deploy_compose
from deploy_config import load_config, parse_config
from deploy_trust import BUNDLE_FILES, ToolBundle, ToolTrustError, manifest, verify_manifest
import test_deploy_hostdzire as fixtures
from test_deploy_hostdzire import request, NEW, NEW_ID, SECRET

TOOLS = Path(__file__).parent


class SchemaTests(unittest.TestCase):
    def test_explicit_configuration_matches_the_verified_site(self):
        config = load_config()
        self.assertEqual((config.base, config.project, config.app_container, config.app_volume,
                          config.health_url, config.maintenance.flag_directory),
                         ("/root/sub2api-kin", "sub2api-kin", "sub2api-kin", "sub2api-kin_sub2api_data",
                          "http://127.0.0.1:10088/health", "/var/lib/sub2api-rel002-maintenance"))

    def test_every_missing_unknown_or_wrong_type_is_rejected(self):
        original = json.loads((TOOLS / "hostdzire.json").read_bytes())
        for key, value in original.items():
            with self.subTest(key=key, kind="missing"):
                bad = copy.deepcopy(original); del bad[key]
                with self.assertRaises(deploy.DeployError): parse_config(bad)
            with self.subTest(key=key, kind="type"):
                bad = copy.deepcopy(original); bad[key] = False if type(value) is int else 1
                with self.assertRaises(deploy.DeployError): parse_config(bad)
        for scope in (None, "maintenance"):
            with self.subTest(scope=scope):
                bad = copy.deepcopy(original)
                (bad if scope is None else bad[scope])["secret"] = "not-allowed"
                with self.assertRaises(deploy.DeployError): parse_config(bad)
        for key in original["maintenance"]:
            with self.subTest(maintenance=key):
                bad = copy.deepcopy(original); del bad["maintenance"][key]
                with self.assertRaises(deploy.DeployError): parse_config(bad)

    def test_invalid_paths_aliases_services_and_health_binding_are_rejected(self):
        original = json.loads((TOOLS / "hostdzire.json").read_bytes())
        for key, value in (("ssh_alias", "host;id"), ("base", "relative"), ("compose_file", "../other"),
                           ("scratch_template", "/root/XXXXXX"), ("published_port", 0),
                           ("host_ip", "0.0.0.0"), ("health_url", "http://example.invalid/health"),
                           ("services", ["sub2api", "redis", "redis"]), ("wait_timeout", True)):
            with self.subTest(key=key):
                bad = copy.deepcopy(original); bad[key] = value
                with self.assertRaises(deploy.DeployError): parse_config(bad)


def tree_snapshot(base):
    return {str(path.relative_to(base)): (path.stat().st_mode & 0o777, path.read_bytes() if path.is_file() else None)
            for path in base.rglob("*")}


class DryRunTests(unittest.TestCase):
    setUp = fixtures.DeploymentTests.setUp

    def ready(self):
        state = self.base / ".personal-deploy"
        deploy.private_directory(state)
        deploy.private_directory(state / "records")
        deploy.atomic_private(state / "deployment.lock", b"")
        return deploy.Deployment(self.base, self.docker)

    def test_preflight_does_not_apply_write_state_or_read_private_environment(self):
        operation = self.ready()
        before = tree_snapshot(self.base)
        regular = deploy_lifecycle.read_regular
        def read(path):
            if path == self.base / ".env": raise AssertionError("dry-run 读取了私密 .env 内容")
            return regular(path)
        with mock.patch.object(operation, "execute", side_effect=AssertionError("dry-run 调用了 apply")), \
             mock.patch.object(deploy_lifecycle, "read_regular", side_effect=read):
            result, code = operation.dry_run(request())
        self.assertEqual((code, result["status"], result["will_apply"]), (0, "dry-run", False))
        self.assertEqual(tree_snapshot(self.base), before)
        self.assertNotIn(SECRET, json.dumps(result).encode())
        self.assertEqual(operation.directory, None)
        self.assertTrue(any(args[0] == "curl" for args, _ in self.docker.commands))
        for args, _ in self.docker.commands:
            self.assertFalse(args[:2] == ["docker", "pull"] or "--no-deps" in args or
                             "pg_dump" in args[-1] or "pg_restore" in args or "tar" in args)

    def test_tmpdir_cannot_place_scratch_or_transient_image_file_in_persistent_state(self):
        operation = self.ready()
        created = []
        writes = []
        mkdtemp = tempfile.mkdtemp
        atomic = deploy_compose.atomic_private
        def observe_creation(*args, **kwargs):
            directory = Path(mkdtemp(*args, **kwargs))
            if directory.name.startswith("sub2api-preflight-"):
                created.append((directory.resolve(), directory.is_dir()))
            return str(directory)
        def observe_write(path, data):
            atomic(path, data)
            if path.name == "operation-image.env":
                writes.append((path.resolve(), path.is_file()))
        overrides = {name: str(operation.state) for name in ("TMPDIR", "TEMP", "TMP")}
        with mock.patch.dict(os.environ, overrides), mock.patch.object(tempfile, "tempdir", None), \
             mock.patch.object(tempfile, "mkdtemp", side_effect=observe_creation), \
             mock.patch.object(deploy_compose, "atomic_private", side_effect=observe_write):
            result, code = operation.dry_run(request())
        self.assertEqual((code, result["status"]), (0, "dry-run"))
        self.assertTrue(created)
        self.assertTrue(writes)
        state = operation.state.resolve()
        for path, existed_during_operation in created + writes:
            self.assertTrue(existed_during_operation)
            self.assertFalse(state == path or state in path.parents,
                             "dry-run 瞬时在持久状态目录创建 scratch 或 operation-image.env: " + str(path))
        self.assertTrue(all(path.parent == Path("/tmp").resolve() for path, _ in created))
        self.assertTrue(all(not path.exists() for path, _ in created + writes))

    def test_missing_state_or_busy_lock_is_rejected_without_repair(self):
        before = tree_snapshot(self.base)
        with self.assertRaises(deploy.DeployError): deploy.Deployment(self.base, self.docker).dry_run(request())
        self.assertEqual(tree_snapshot(self.base), before)
        operation = self.ready()
        import fcntl
        with (operation.state / "deployment.lock").open("rb") as lock:
            fcntl.flock(lock.fileno(), fcntl.LOCK_EX | fcntl.LOCK_NB)
            with self.assertRaises(deploy.DeployError) as raised: operation.dry_run(request())
        self.assertEqual(raised.exception.code, "E_LOCKED")
        self.assertEqual(self.docker.commands, [])

    def test_uncached_target_and_bad_metadata_are_explicit_failures_without_pull(self):
        operation = self.ready()
        del self.docker.images[NEW["image"]]
        before = tree_snapshot(self.base)
        with self.assertRaises(deploy.DeployError) as raised: operation.dry_run(request())
        self.assertEqual(raised.exception.code, "E_IMAGE_UNAVAILABLE")
        self.assertEqual(tree_snapshot(self.base), before)
        self.assertFalse(any(args[:2] == ["docker", "pull"] for args, _ in self.docker.commands))
        self.docker.images[NEW["image"]] = self.docker.images[NEW_ID]
        self.docker.images[NEW_ID]["Architecture"] = "arm64"
        with self.assertRaises(deploy.DeployError) as raised: operation.dry_run(request())
        self.assertEqual(raised.exception.code, "E_PLATFORM")
        self.assertEqual(tree_snapshot(self.base), before)
        self.assertFalse(any(args[:2] == ["docker", "pull"] for args, _ in self.docker.commands))

    def test_bundle_or_configuration_drift_is_rejected_before_docker(self):
        operation = self.ready()
        for name in ("deploy_images.py", "hostdzire.json", "deploy_hostdzire.py"):
            with self.subTest(name=name):
                bad = request(); bad["tool_bundle"][name] = "0" * 64
                with self.assertRaises(ToolTrustError): operation.dry_run(bad)
                self.assertEqual(self.docker.commands, [])


class KinDryRunTests(unittest.TestCase):
    setUp = fixtures.DeploymentTests.setUp
    prepare = fixtures.KinDeploymentTests.prepare

    @classmethod
    def setUpClass(cls): fixtures.KinDeploymentTests.setUpClass.__func__(cls)

    def test_actual_extended_ledger_record_is_read_without_migration_or_selection_changes(self):
        value, _ = self.prepare()
        operation = deploy.Deployment(self.base, self.docker)
        deployed, code = operation.execute(value)
        self.assertEqual(code, 0)
        old = operation.running()
        value = request(value["target"], old)
        value["compatibility"] = deploy.GitEvidence(TOOLS.resolve().parents[1]).compatibility(old, value["target"])
        self.docker.commands.clear()
        before = tree_snapshot(self.base)
        result, code = deploy.Deployment(self.base, self.docker).dry_run(value)
        self.assertEqual((code, result["migration_state"]["rows"]), (0, 308))
        self.assertEqual(result["migration_state"]["filename_checksum_sha256"],
                         "a31c942e9e772bef1af9c68f930ee35afaa8c3591b5d1c22609f9bf7d30da882")
        self.assertIs(result["compatibility"]["kin_upgrade"]["image_rollback_compatible"], False)
        self.assertEqual(tree_snapshot(self.base), before)
        self.assertFalse(any(args[:2] == ["docker", "pull"] or "--no-deps" in args for args, _ in self.docker.commands))


class BundleTests(unittest.TestCase):
    def setUp(self):
        self.tmp = tempfile.TemporaryDirectory(); self.addCleanup(self.tmp.cleanup)
        self.root = Path(self.tmp.name) / "repository"
        self.directory = self.root / "deploy/personal"; self.directory.mkdir(parents=True)
        for name in BUNDLE_FILES: shutil.copy2(TOOLS / name, self.directory / name)
        self.git("init", "-q")
        self.git("config", "user.email", "fixture@example.invalid"); self.git("config", "user.name", "Tool Fixture")
        self.git("remote", "add", "origin", "https://github.com/ccisnoxx/sub2api.git")
        self.git("add", "deploy/personal"); self.git("commit", "-qm", "trusted control fixture")
        self.revision = self.git("rev-parse", "HEAD").decode().strip()
        self.git("update-ref", "refs/remotes/origin/main", self.revision)
        self.bundle = ToolBundle.from_git(self.root, self.directory, self.revision)

    def git(self, *args): return subprocess.run(["git", "-C", str(self.root), *args], capture_output=True, check=True).stdout

    def test_complete_exact_bundle_is_bound_to_fork_main(self):
        self.assertEqual(set(self.bundle.files), set(BUNDLE_FILES))
        self.assertTrue(verify_manifest(self.directory, self.bundle.files, exclusive=True))
        with self.assertRaises(ToolTrustError): ToolBundle.from_git(self.root, self.directory, "0" * 40)
        self.git("remote", "set-url", "origin", "https://github.com/unknown/repository")
        with self.assertRaises(ToolTrustError): ToolBundle.from_git(self.root, self.directory, self.revision)

    def test_dirty_module_config_symlink_mode_and_staged_drift_fail_before_ssh(self):
        for name in ("deploy_common.py", "hostdzire.json"):
            with self.subTest(name=name):
                original = (self.directory / name).read_bytes()
                (self.directory / name).write_bytes(original + b"\n")
                with mock.patch("deploy_transport.subprocess.run", side_effect=AssertionError("触及了 SSH")):
                    with self.assertRaises(ToolTrustError): self.bundle.verify()
                with self.assertRaises(ToolTrustError): ToolBundle.from_git(self.root, self.directory, self.revision)
                (self.directory / name).write_bytes(original)
        path = self.directory / "deploy_common.py"; original = path.read_bytes()
        path.unlink(); path.symlink_to(self.directory / "deploy_images.py")
        with self.assertRaises(ToolTrustError): self.bundle.verify()
        path.unlink(); path.write_bytes(original); path.chmod(0o755)
        with self.assertRaises(ToolTrustError): ToolBundle.from_git(self.root, self.directory, self.revision)
        path.chmod(0o644); path.write_bytes(original + b"\n"); self.git("add", "deploy/personal/deploy_common.py")
        path.write_bytes(original)
        with self.assertRaises(ToolTrustError): ToolBundle.from_git(self.root, self.directory, self.revision)

    def test_remote_bootstrap_rejects_drift_and_extra_files_before_any_business_import(self):
        marker = self.root / "untrusted-import-ran"
        original = (self.directory / "deploy_common.py").read_bytes()
        (self.directory / "deploy_common.py").write_text("from pathlib import Path\nPath(" + repr(str(marker)) + ").write_text('executed')\n")
        result = subprocess.run(shlex.split(self.bundle.bootstrap("--remote-probe")), capture_output=True)
        self.assertEqual((result.returncode, json.loads(result.stdout)["error_code"]), (2, "E_TOOL_TRUST"))
        self.assertFalse(marker.exists())
        (self.directory / "deploy_common.py").write_bytes(original)
        for name in ("hostdzire.json", "unbound.py"):
            with self.subTest(name=name):
                path = self.directory / name
                previous = path.read_bytes() if path.exists() else None
                path.write_bytes(b"drift")
                result = subprocess.run(shlex.split(self.bundle.bootstrap("--remote-dry-run")), capture_output=True)
                self.assertEqual((result.returncode, json.loads(result.stdout)["error_code"]), (2, "E_TOOL_TRUST"))
                if previous is None: path.unlink()
                else: path.write_bytes(previous)

    def test_verified_remote_bootstrap_imports_the_complete_runtime_without_writing_state(self):
        path = self.directory / "hostdzire.json"
        config = json.loads(path.read_bytes())
        config["base"] = str(self.root / "empty-runtime")
        path.write_text(json.dumps(config))
        self.git("add", "deploy/personal/hostdzire.json")
        self.git("commit", "-qm", "isolated read-only target")
        revision = self.git("rev-parse", "HEAD").decode().strip()
        self.git("update-ref", "refs/remotes/origin/main", revision)
        bundle = ToolBundle.from_git(self.root, self.directory, revision)
        result = subprocess.run(shlex.split(bundle.bootstrap("--remote-dry-run")), input=b"{}", capture_output=True)
        self.assertEqual((result.returncode, json.loads(result.stdout)["error_code"]), (2, "E_PERMISSION"))
        self.assertFalse(Path(config["base"]).exists())

    def test_isolated_entry_uses_git_bytes_and_stops_dirty_bundle_before_connection(self):
        common = self.directory / "deploy_common.py"
        common.write_bytes(common.read_bytes() + b"\nraise RuntimeError('untrusted module imported')\n")
        result = subprocess.run(["sh", str(self.directory / "deploy-hostdzire.sh"), "latest", "--tool-revision", self.revision, "--dry-run"], capture_output=True)
        self.assertEqual((result.returncode, json.loads(result.stdout)["error_code"]), (2, "E_TOOL_TRUST"))
        self.assertNotIn(b"untrusted module imported", result.stdout)


if __name__ == "__main__": unittest.main()
