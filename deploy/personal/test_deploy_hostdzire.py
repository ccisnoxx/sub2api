"""在真实临时文件/Git 与 Docker/OpenSSH 替身边界验证部署合同。"""

import copy
import io
import json
import os
from pathlib import Path
import subprocess
import sys
import tarfile
import tempfile
import unittest
from unittest import mock

import deploy_hostdzire as deploy


NEW = {
    "input": deploy.REPOSITORY + "@sha256:" + "a" * 64,
    "image": deploy.REPOSITORY + "@sha256:" + "a" * 64,
    "revision": "b" * 40,
    "version": "0.2.14-klno.3-tps.1",
    "source": deploy.SOURCE,
}
OLD_ID = "sha256:" + "c" * 64
NEW_ID = "sha256:" + "d" * 64
PG_ID = "e" * 64
REDIS_ID = "f" * 64
SECRET = b"private-backup-and-application-secret"
ORIGINAL = ("# 保留生产配置与注释\r\nservices:\r\n"
            "  sub2api:\r\n    image: ghcr.io/kln-4096/sub2api:0.2.14-klno.3 # 原版本\r\n"
            "    container_name: sub2api-kin\r\n"
            "  postgres:\r\n    image: postgres:18-alpine\r\n"
            "  redis:\r\n    image: redis:8-alpine\r\n"
            "volumes:\r\n  sub2api_data:\r\n").encode()


def image(metadata, image_id):
    return {"Id": image_id, "Os": "linux", "Architecture": "amd64", "RepoDigests": [metadata["image"]],
            "Config": {"Labels": {"org.opencontainers.image." + key: metadata[key] for key in ("revision", "version", "source")}}}


def proof(old=deploy.LEGACY, target=NEW):
    value = {"old_revision": old["revision"], "new_revision": target["revision"], "paths": deploy.RUNTIME_PATHS,
             "excludes": deploy.RUNTIME_EXCLUDES, "changed_paths": [], "compatible": True,
             "upstream": {"upstream_repository": "KlN-4096/sub2api", "upstream_tag": "v0.2.14-klno.3", "upstream_sha": deploy.LEGACY["revision"]}}
    value["sha256"] = deploy.sha(deploy.canonical(value))
    return value


def request(target=NEW, old=None, action="deploy", rollback_id=None):
    return {"schema": 1, "action": action, "target": copy.deepcopy(target),
            "expected_old": copy.deepcopy(old or dict(deploy.LEGACY, image_id=OLD_ID)),
            "compatibility": proof(old or deploy.LEGACY, target), "rollback_id": rollback_id,
            "tool_revision": "1" * 40, "tool_sha256": deploy.sha(Path(deploy.__file__).read_bytes())}


class DockerSubstitute:
    """具有镜像/容器状态转换的替身；未知命令直接失败，避免遗漏真实边界。"""
    def __init__(self, base):
        self.base = base
        self.log = None
        self.commands = []
        self.images = {}
        for metadata, image_id in ((deploy.LEGACY, OLD_ID), (NEW, NEW_ID)):
            value = image(metadata, image_id)
            self.images[metadata["image"]] = value
            self.images[image_id] = value
            self.images[metadata["image"].split("@")[0] + ":" + metadata["version"]] = value
        self.current = deploy.LEGACY["image"]
        self.config_image = "ghcr.io/kln-4096/sub2api:0.2.14-klno.3"
        self.healthy = True
        self.pull_failure = False
        self.wait_failure = False
        self.health_failure = False
        self.backup_failure = False
        self.config_drift = False
        self.dependency_change = False
        self.migration_change = False

    def container(self, service):
        value = {"Config": {"Labels": {"com.docker.compose.project": deploy.PROJECT,
                                         "com.docker.compose.service": service}},
                 "State": {"Running": True, "Health": {"Status": "healthy"}}}
        if service == "sub2api":
            value["Image"] = self.images[self.current]["Id"]
            value["Config"]["Image"] = self.config_image
            value["State"]["Health"]["Status"] = "healthy" if self.healthy else "unhealthy"
            value["Mounts"] = [{"Type": "volume", "Name": deploy.VOLUME, "Destination": "/app/data"}]
        return value

    def configuration(self, env):
        text = (self.base / "docker-compose.yml").read_text()
        reference = env["SUB2API_IMAGE"] if deploy.IMAGE_VARIABLE in text else self.config_image
        return {"name": deploy.PROJECT,
                "services": {"sub2api": {"image": reference, "container_name": deploy.APP,
                                         "healthcheck": {"test": ["CMD", "wget", "/health"]},
                                         "ports": [{"target": 8080, "published": "10088", "host_ip": "127.0.0.1"}],
                                         "volumes": [{"target": "/app/data", "type": "volume", "source": "sub2api_data"}],
                                         "environment": {"DATABASE_PASSWORD": SECRET.decode()}},
                             "postgres": {"image": "postgres:18-alpine"}, "redis": {"image": "redis:8-alpine"}},
                "volumes": {"sub2api_data": {"name": deploy.VOLUME}}}

    def run(self, args, *, env=None, stdin=None, stdin_path=None, stdout_path=None, timeout=180, sensitive=False):
        self.commands.append((args, env))
        output = b""
        if args == ["uname", "-sm"]:
            output = b"Linux x86_64\n"
        elif args == ["docker", "compose", "up", "--help"]:
            output = b"--wait --wait-timeout"
        elif args[:3] == ["docker", "image", "inspect"]:
            output = json.dumps([self.images[args[3]]]).encode()
        elif args[:2] == ["docker", "inspect"]:
            service = "sub2api" if args[2] == deploy.APP else "postgres" if args[2] == PG_ID else "redis"
            output = json.dumps([self.container(service)]).encode()
        elif args[:2] == ["docker", "pull"]:
            if self.pull_failure:
                raise deploy.DeployError("E_COMMAND", "替身拉取失败")
        elif args[:2] == ["docker", "compose"]:
            operation = args[args.index("docker-compose.yml") + 1:]
            if operation == ["config", "--format", "json"]:
                output = json.dumps(self.configuration(env)).encode()
            elif operation[:2] == ["ps", "-q"]:
                output = (PG_ID if operation[-1] == "postgres" else REDIS_ID).encode()
                if self.dependency_change and self.current == NEW["image"]:
                    output = b"1" * 64
            elif operation == ["up", "-d", "--no-deps", "--wait", "--wait-timeout", "120", "sub2api"]:
                self.current = env["SUB2API_IMAGE"]
                self.config_image = self.current
                self.healthy = not (self.health_failure and self.current == NEW["image"])
                if self.config_drift and self.current == NEW["image"]:
                    with open(self.base / "docker-compose.yml", "ab") as output_file:
                        output_file.write(b"# external change\n")
                if self.wait_failure and self.current == NEW["image"]:
                    self.healthy = False
                    raise deploy.DeployError("E_TIMEOUT", "替身健康等待超时")
            else:
                raise AssertionError("不允许的 Compose 操作: " + repr(operation))
        elif args[:3] == ["docker", "exec", PG_ID]:
            if "pg_dump" in args[-1]:
                if self.backup_failure:
                    raise deploy.DeployError("E_COMMAND", "替身备份失败")
                output = b"PGDMP" + SECRET
            elif "SELECT 1" in args[-1]:
                output = b"1\n"
            elif "SELECT filename, checksum FROM schema_migrations" in args[-1]:
                output = b"001.sql|old-checksum\n"
                if self.migration_change and self.current == NEW["image"]:
                    output += b"002.sql|new-checksum\n"
            else:
                raise AssertionError(args)
        elif args[:3] == ["docker", "exec", "-i"]:
            assert args == ["docker", "exec", "-i", PG_ID, "pg_restore", "--list"]
            assert stdin is None and Path(stdin_path).read_bytes() == b"PGDMP" + SECRET
            output = b"; validated custom dump\n"
        elif args == ["docker", "exec", REDIS_ID, "redis-cli", "ping"]:
            output = b"PONG\n"
        elif args[:3] == ["docker", "exec", deploy.APP]:
            assert args == ["docker", "exec", deploy.APP, "tar", "-C", "/app/data", "-czf", "-", "."]
            buffer = io.BytesIO()
            with tarfile.open(fileobj=buffer, mode="w:gz") as archive:
                info = tarfile.TarInfo("./config.yaml")
                info.size = len(SECRET)
                archive.addfile(info, io.BytesIO(SECRET))
            output = buffer.getvalue()
        elif args[0] == "curl":
            output = b'{"status":"ok"}'
        elif args[:2] == ["docker", "logs"]:
            output = SECRET
        else:
            raise AssertionError("未知替身命令: " + repr(args))
        if stdout_path:
            Path(stdout_path).write_bytes(output)
            os.chmod(stdout_path, 0o600)
            return b""
        return output


class DeploymentTests(unittest.TestCase):
    def setUp(self):
        self.temporary = tempfile.TemporaryDirectory()
        self.addCleanup(self.temporary.cleanup)
        self.base = Path(self.temporary.name)
        (self.base / "docker-compose.yml").write_bytes(ORIGINAL)
        (self.base / ".env").write_bytes(b"POSTGRES_PASSWORD=" + SECRET + b"\nSUB2API_IMAGE=malicious:latest\n")
        self.docker = DockerSubstitute(self.base)

    def execute(self, value=None):
        return deploy.Deployment(self.base, self.docker).execute(value or request())

    def persisted(self, result):
        return json.loads((self.base / ".personal-deploy" / "records" / result["id"] / "record.json").read_bytes())

    def starts(self):
        return [args for args, _ in self.docker.commands if "--no-deps" in args]

    def test_success_only_recreates_app_and_commits_same_digest(self):
        result, code = self.execute()
        self.assertEqual(code, 0)
        self.assertEqual(result["status"], "success")
        self.assertEqual(result["target"]["image"], NEW["image"])
        self.assertEqual(result["observed_running"]["image_id"], NEW_ID)
        self.assertEqual(len(self.starts()), 1)
        self.assertEqual(self.starts()[0][-7:], ["up", "-d", "--no-deps", "--wait", "--wait-timeout", "120", "sub2api"])
        self.assertEqual((self.base / "docker-compose.yml").read_bytes(), ORIGINAL.replace(
            b"ghcr.io/kln-4096/sub2api:0.2.14-klno.3", ('"' + deploy.IMAGE_VARIABLE + '"').encode()))
        selection = deploy.Deployment(self.base, self.docker).selected()
        self.assertEqual(selection, {"image": NEW["image"], "record_id": result["id"]})
        self.assertTrue(self.persisted(result)["health_verified"])
        for args, env in self.docker.commands:
            if args[:2] == ["docker", "compose"] and "docker-compose.yml" in args:
                self.assertIn(env["SUB2API_IMAGE"], (NEW["image"], deploy.LEGACY["image"]))
                self.assertIn("--env-file", args)
            self.assertFalse(any(command in args for command in ("down", "prune", "rm")))
        self.assertNotIn(SECRET, json.dumps(result).encode())
        self.assertEqual((self.base / ".env").read_bytes(), b"POSTGRES_PASSWORD=" + SECRET + b"\nSUB2API_IMAGE=malicious:latest\n")
        state = self.base / ".personal-deploy"
        for path in state.rglob("*"):
            self.assertEqual(path.stat().st_mode & 0o777, 0o700 if path.is_dir() else 0o600, str(path))

    def test_version_resolves_then_uses_digest_everywhere(self):
        target = dict(NEW, input=deploy.REPOSITORY + ":" + NEW["version"])
        del target["image"]
        result, code = self.execute(request(target))
        self.assertEqual(code, 0)
        pulls = [args[-1] for args, _ in self.docker.commands if args[:2] == ["docker", "pull"]]
        self.assertEqual(pulls, [target["input"], NEW["image"]])
        self.assertEqual(result["target"]["image"], NEW["image"])
        self.assertEqual(self.docker.current, NEW["image"])

    def test_pull_failure_preserves_configuration_running_and_selection(self):
        self.docker.pull_failure = True
        result, code = self.execute()
        self.assertNotEqual(code, 0)
        self.assertEqual(result["status"], "failed")
        self.assertEqual(self.docker.current, deploy.LEGACY["image"])
        self.assertEqual((self.base / "docker-compose.yml").read_bytes(), ORIGINAL)
        self.assertFalse((self.base / ".personal-deploy" / "image.env").exists())
        self.assertEqual(self.starts(), [])
        self.assertIn("backups", self.persisted(result))
        self.assertNotIn(SECRET, json.dumps(result).encode())

    def test_wait_timeout_rolls_back_but_keeps_failure_exit_and_record(self):
        self.docker.wait_failure = True
        result, code = self.execute()
        self.assertEqual(code, 1)
        self.assertEqual(result["error_code"], "E_TIMEOUT")
        self.assertEqual(result["rollback_status"], "success")
        self.assertEqual(self.docker.current, deploy.LEGACY["image"])
        self.assertEqual((self.base / "docker-compose.yml").read_bytes(), ORIGINAL)
        self.assertEqual(len(self.starts()), 2)
        self.assertFalse((self.base / ".personal-deploy" / "image.env").exists())
        self.assertEqual(self.persisted(result)["status"], "failed")

    def test_actual_migration_drift_prohibits_automatic_rollback(self):
        self.docker.migration_change = True
        result, code = self.execute()
        self.assertEqual(code, 1)
        self.assertEqual(result["error_code"], "E_MIGRATION")
        self.assertEqual(result["rollback_error"], "E_MIGRATION")
        self.assertEqual(len(self.starts()), 1)
        self.assertEqual(self.docker.current, NEW["image"])
        self.assertFalse((self.base / ".personal-deploy" / "image.env").exists())
        self.assertEqual(self.persisted(result)["status"], "failed")

    def test_migration_drift_after_success_blocks_explicit_rollback(self):
        deployed, code = self.execute()
        self.assertEqual(code, 0)
        self.docker.migration_change = True
        old = deploy.Deployment(self.base, self.docker).running()
        target = dict(deploy.LEGACY, input=deploy.LEGACY["image"])
        result, code = self.execute(request(target, old, "rollback", deployed["id"]))
        self.assertEqual((code, result["error_code"]), (1, "E_MIGRATION"))
        self.assertEqual(len(self.starts()), 1)

    def test_container_health_failure_rolls_back(self):
        self.docker.health_failure = True
        result, code = self.execute()
        self.assertEqual((code, result["error_code"], result["rollback_status"]), (1, "E_HEALTH", "success"))

    def test_backup_failure_has_no_pull_or_configuration_mutation(self):
        self.docker.backup_failure = True
        result, code = self.execute()
        self.assertEqual(code, 1)
        self.assertEqual(self.starts(), [])
        self.assertFalse(any(args[:2] == ["docker", "pull"] for args, _ in self.docker.commands))
        self.assertEqual((self.base / "docker-compose.yml").read_bytes(), ORIGINAL)

    def test_corrupt_gzip_footer_stops_before_pull_or_recreate(self):
        run = self.docker.run
        def corrupt_archive(args, **options):
            result = run(args, **options)
            path = options.get("stdout_path")
            if path and Path(path).name == "app-data.tar.gz":
                data = bytearray(Path(path).read_bytes())
                data[-8] ^= 0xff
                Path(path).write_bytes(data)
            return result
        self.docker.run = corrupt_archive
        result, code = self.execute()
        self.assertEqual((code, result.get("error_code")), (1, "E_BACKUP"))
        self.assertEqual(self.starts(), [])
        self.assertFalse(any(args[:2] == ["docker", "pull"] for args, _ in self.docker.commands))
        self.assertEqual((self.base / "docker-compose.yml").read_bytes(), ORIGINAL)
        self.assertFalse((self.base / ".personal-deploy" / "image.env").exists())

    def test_disabled_compose_healthcheck_stops_before_image_pull(self):
        configuration = self.docker.configuration
        def disabled(env):
            value = configuration(env)
            value["services"]["sub2api"]["healthcheck"]["test"] = ["NONE"]
            return value
        self.docker.configuration = disabled
        result, code = self.execute()
        self.assertEqual((code, result["error_code"]), (1, "E_LAYOUT"))
        self.assertEqual(self.starts(), [])
        self.assertFalse(any(args[:2] == ["docker", "pull"] for args, _ in self.docker.commands))

    def test_explicit_rollback_binds_current_success_and_rejects_replay(self):
        deployed, code = self.execute()
        self.assertEqual(code, 0)
        old = deploy.Deployment(self.base, self.docker).running()
        target = dict(deploy.LEGACY, input=deploy.LEGACY["image"])
        rolled, code = self.execute(request(target, old, "rollback", deployed["id"]))
        self.assertEqual(code, 0)
        self.assertEqual(rolled["target"]["image"], deploy.LEGACY["image"])
        self.assertEqual(self.persisted(rolled)["rollback_of"], deployed["id"])
        self.assertEqual(deploy.Deployment(self.base, self.docker).selected()["record_id"], rolled["id"])
        starts = len(self.starts())
        replay_old = deploy.Deployment(self.base, self.docker).running()
        result, code = self.execute(request(target, replay_old, "rollback", deployed["id"]))
        self.assertEqual(code, 1)
        self.assertEqual(result["error_code"], "E_STALE_ROLLBACK")
        self.assertEqual(len(self.starts()), starts)

    def test_lock_competition_stops_before_docker_commands(self):
        import fcntl
        state = self.base / ".personal-deploy"
        deploy.private_directory(state)
        with open(state / "deployment.lock", "w") as lock:
            fcntl.flock(lock.fileno(), fcntl.LOCK_EX | fcntl.LOCK_NB)
            with self.assertRaises(deploy.DeployError) as raised:
                self.execute()
        self.assertEqual(raised.exception.code, "E_LOCKED")
        self.assertEqual(self.docker.commands, [])

    def test_runtime_change_proof_prohibits_deploy_and_rollback(self):
        bad = request()
        bad["compatibility"]["changed_paths"] = ["backend/migrations/999_change.sql"]
        bad["compatibility"]["compatible"] = False
        bad["compatibility"]["sha256"] = deploy.sha(deploy.canonical({key: value for key, value in bad["compatibility"].items() if key != "sha256"}))
        result, code = self.execute(bad)
        self.assertEqual((code, result["error_code"]), (1, "E_COMPATIBILITY"))
        self.assertEqual(self.starts(), [])
        deployed, code = self.execute()
        self.assertEqual(code, 0)
        old = deploy.Deployment(self.base, self.docker).running()
        rollback = request(dict(deploy.LEGACY, input=deploy.LEGACY["image"]), old, "rollback", deployed["id"])
        rollback["compatibility"]["compatible"] = False
        result, code = self.execute(rollback)
        self.assertEqual((code, result["error_code"]), (1, "E_COMPATIBILITY"))
        self.assertEqual(len(self.starts()), 1)

    def test_bad_platform_labels_source_and_digest_never_start(self):
        for mutation, expected_code in (
            (lambda value: value.update(Architecture="arm64"), "E_PLATFORM"),
            (lambda value: value["Config"]["Labels"].update({"org.opencontainers.image.revision": "0" * 40}), "E_LABEL"),
            (lambda value: value["Config"]["Labels"].update({"org.opencontainers.image.version": "0.2.14-klno.3-tps.2"}), "E_LABEL"),
            (lambda value: value["Config"]["Labels"].update({"org.opencontainers.image.source": "https://github.com/unknown/sub2api"}), "E_IMAGE"),
            (lambda value: value.update(RepoDigests=[deploy.REPOSITORY + "@sha256:" + "0" * 64]), "E_DIGEST"),
        ):
            with self.subTest(expected_code=expected_code):
                self.docker = DockerSubstitute(self.base)
                mutation(self.docker.images[NEW["image"]])
                result, code = self.execute()
                self.assertEqual((code, result["error_code"]), (1, expected_code))
                self.assertEqual(self.starts(), [])
                self.assertEqual((self.base / "docker-compose.yml").read_bytes(), ORIGINAL)

    def test_config_drift_is_preserved_and_prohibits_automatic_rollback(self):
        self.docker.wait_failure = self.docker.config_drift = True
        result, code = self.execute()
        self.assertEqual(code, 1)
        self.assertEqual(result["rollback_status"], "failed")
        self.assertEqual(result["rollback_error"], "E_DRIFT")
        self.assertTrue((self.base / "docker-compose.yml").read_bytes().endswith(b"# external change\n"))
        self.assertEqual(len(self.starts()), 1)

    def test_stale_running_revision_is_rejected_before_backups(self):
        value = request()
        value["expected_old"]["revision"] = "0" * 40
        result, code = self.execute(value)
        self.assertEqual((code, result["error_code"]), (1, "E_STALE"))
        self.assertNotIn("backups", self.persisted(result))
        self.assertEqual(self.starts(), [])

    def test_dependency_change_is_failure_and_is_not_hidden(self):
        self.docker.dependency_change = True
        result, code = self.execute()
        self.assertEqual(code, 1)
        self.assertEqual(result["error_code"], "E_DEPENDENCY")
        self.assertEqual(result["rollback_status"], "failed")
        self.assertEqual(result["rollback_error"], "E_DEPENDENCY")
        self.assertEqual(len(self.starts()), 1)

    def test_selection_write_failure_restores_old_runtime_without_success(self):
        atomic = deploy.atomic_private
        def fail_selection(path, data):
            if path.name == "image.env":
                raise OSError("simulated disk error")
            atomic(path, data)
        with mock.patch.object(deploy, "atomic_private", side_effect=fail_selection):
            result, code = self.execute()
        self.assertEqual((code, result["status"], result["rollback_status"]), (1, "failed", "success"))
        self.assertEqual(self.docker.current, deploy.LEGACY["image"])

    def test_configuration_write_committed_before_error_is_restored(self):
        atomic = deploy.atomic_private
        failed = False
        def fail_after_replace(path, data):
            nonlocal failed
            atomic(path, data)
            if path == self.base / "docker-compose.yml" and data != ORIGINAL and not failed:
                failed = True
                raise OSError("directory fsync failed after replace")
        with mock.patch.object(deploy, "atomic_private", side_effect=fail_after_replace):
            result, code = self.execute()
        self.assertEqual((code, result["status"]), (1, "failed"))
        self.assertEqual(self.starts(), [])
        self.assertEqual((self.base / "docker-compose.yml").read_bytes(), ORIGINAL)
        self.assertFalse((self.base / ".personal-deploy" / "image.env").exists())

    def test_selection_committed_before_error_is_preserved_and_blocks_retry(self):
        atomic = deploy.atomic_private
        def fail_after_replace(path, data):
            atomic(path, data)
            if path.name == "image.env":
                raise OSError("directory fsync failed after selection replace")
        with mock.patch.object(deploy, "atomic_private", side_effect=fail_after_replace):
            result, code = self.execute()
        self.assertEqual((code, result["status"], result["selection_committed"]), (1, "failed", True))
        self.assertEqual(self.docker.current, NEW["image"])
        self.assertEqual(len(self.starts()), 1)
        snapshot = deploy.Deployment(self.base, self.docker).running()
        retry, code = self.execute(request(old=snapshot))
        self.assertEqual((code, retry["error_code"]), (1, "E_STATE"))
        self.assertEqual(len(self.starts()), 1)

    def test_personal_running_without_committed_selection_is_not_adopted(self):
        self.docker.current = self.docker.config_image = NEW["image"]
        snapshot = deploy.Deployment(self.base, self.docker).running()
        result, code = self.execute(request(old=snapshot))
        self.assertEqual((code, result["error_code"]), (1, "E_STATE"))
        self.assertEqual(self.starts(), [])


class GitAndInputTests(unittest.TestCase):
    def setUp(self):
        self.temporary = tempfile.TemporaryDirectory()
        self.addCleanup(self.temporary.cleanup)
        self.root = Path(self.temporary.name)
        self.git("init", "-q")
        self.git("config", "user.email", "fixture@example.invalid")
        self.git("config", "user.name", "Deployment Fixture")
        (self.root / "backend" / "migrations").mkdir(parents=True)
        (self.root / "backend" / "migrations" / "001.sql").write_text("select 1;\n")
        self.baseline = self.commit("baseline")
        self.git("tag", "v1.0.0-klno.1")
        (self.root / "deploy").mkdir()
        source = {"upstream_repository": "KlN-4096/sub2api", "upstream_tag": "v1.0.0-klno.1", "upstream_sha": self.baseline}
        (self.root / "deploy" / "personal-source.json").write_text(json.dumps(source))
        old_revision = self.commit("old personal")
        self.git("tag", "v1.0.0-klno.1-tps.1")
        self.old = dict(NEW, image=deploy.REPOSITORY + "@sha256:" + "1" * 64, revision=old_revision, version="1.0.0-klno.1-tps.1")
        self.evidence = deploy.GitEvidence(self.root)

    def git(self, *args):
        result = subprocess.run(["git", "-C", str(self.root), *args], capture_output=True, check=True)
        return result.stdout.decode().strip()

    def commit(self, message):
        self.git("add", ".")
        self.git("commit", "-qm", message)
        return self.git("rev-parse", "HEAD")

    def test_real_git_diff_accepts_frontend_and_tools_but_blocks_migration(self):
        (self.root / "frontend").mkdir()
        (self.root / "frontend" / "tps.vue").write_text("TPS\n")
        (self.root / "deploy" / "personal").mkdir()
        (self.root / "deploy" / "personal" / "tool.py").write_text("# 运行工具\n")
        revision = self.commit("frontend and deployment tool")
        self.git("tag", "v1.0.0-klno.1-tps.2")
        target = dict(NEW, revision=revision, version="1.0.0-klno.1-tps.2")
        self.assertEqual(self.evidence.personal(target["version"]), revision)
        self.assertEqual(self.evidence.compatibility(self.old, target)["changed_paths"], [])
        (self.root / "backend" / "migrations" / "002.sql").write_text("alter table accounts add column new_value text;\n")
        target["revision"] = self.commit("migration")
        with self.assertRaises(deploy.DeployError) as raised:
            self.evidence.compatibility(self.old, target)
        self.assertEqual(raised.exception.code, "E_COMPATIBILITY")
        with self.assertRaises(deploy.DeployError):
            self.evidence.compatibility(target, self.old)

    def test_unknown_git_revision_and_tag_label_mismatch_stop(self):
        with self.assertRaises(deploy.DeployError):
            self.evidence.personal(self.old["version"], "0" * 40)
        target = dict(NEW, revision="0" * 40)
        with self.assertRaises(deploy.DeployError):
            self.evidence.compatibility(self.old, target)

    def test_inputs_reject_latest_untrusted_repository_and_shell_syntax(self):
        for value in ("latest", deploy.REPOSITORY + ":latest", "ghcr.io/kln-4096/sub2api:0.2.14-klno.3", "0.2.14-klno.3-tps.1;id",
                      "$(id)", "0.2.14-klno.3-tps.1\nwhoami", NEW["image"].upper()):
            with self.subTest(value=value), self.assertRaises(deploy.DeployError):
                deploy.parse_target(value)
        self.assertEqual(deploy.parse_target("v" + NEW["version"])["input"], deploy.REPOSITORY + ":" + NEW["version"])
        self.assertEqual(deploy.parse_target(NEW["image"], NEW["version"])["image"], NEW["image"])
        with self.assertRaises(deploy.DeployError):
            deploy.parse_target(NEW["image"])

    def test_actual_published_target_has_no_runtime_change(self):
        root = Path(deploy.__file__).resolve().parents[2]
        evidence = deploy.GitEvidence(root)
        version = "0.2.14-klno.3-tps.1"
        revision = evidence.personal(version)
        self.assertEqual(revision, "896de21b4be7f4ec4b4236f4df663b47371665b0")
        target = dict(NEW, revision=revision, image=deploy.REPOSITORY + "@sha256:f4a979fdeef6c79b982d16d77bc3a6c7b528164bd6ce5b1deb34a8f4981a3d76")
        self.assertTrue(evidence.compatibility(deploy.LEGACY, target)["compatible"])


class TransportTests(unittest.TestCase):
    def test_native_ssh_scp_transport_uses_alias_and_sends_json_via_stdin(self):
        calls = []
        response = {"status": "success", "id": "20261008T180000Z-123456789abc"}
        def process(args, **kwargs):
            calls.append((args, kwargs))
            if "mktemp" in args[-1]:
                output = b"/tmp/sub2api-personal.123456789abc\n"
            elif "--remote-probe" in args[-1]:
                output = json.dumps({"running": deploy.LEGACY, "selection": None}).encode()
            elif "--remote-apply" in args[-1]:
                output = json.dumps(response).encode()
            else:
                output = b""
            return subprocess.CompletedProcess(args, 0, output, b"")
        with mock.patch.object(deploy.subprocess, "run", side_effect=process):
            with deploy.SSHTransport(Path(deploy.__file__)) as remote:
                self.assertEqual(remote.probe()["running"], deploy.LEGACY)
                self.assertEqual(remote.execute(request()), (response, 0))
        self.assertEqual([args[0] for args, _ in calls], ["ssh", "scp", "ssh", "ssh", "ssh"])
        for args, _ in calls:
            if args[0] == "ssh":
                self.assertEqual(args[1], "hostdzire")
            self.assertFalse(any("StrictHostKeyChecking" in part or "UserKnownHostsFile" in part for part in args))
        write = calls[3]
        self.assertEqual(json.loads(write[1]["input"])["target"]["image"], NEW["image"])
        self.assertNotIn(NEW["image"], write[0][-1])

    def test_remote_disconnection_does_not_report_success_or_retry(self):
        with mock.patch.object(deploy.subprocess, "run", return_value=subprocess.CompletedProcess([], 255, b"", b"private connection error")) as run:
            remote = deploy.SSHTransport(Path(deploy.__file__))
            remote.directory = "/tmp/sub2api-personal.123456789abc"
            with self.assertRaises(deploy.DeployError) as raised:
                remote.execute(request())
        self.assertEqual(raised.exception.code, "E_SSH")
        self.assertEqual(run.call_count, 1)
        self.assertNotIn("private connection error", str(raised.exception))

    def test_real_process_boundary_streams_files_and_keeps_errors_private(self):
        with tempfile.TemporaryDirectory() as temporary:
            base = Path(temporary)
            runner = deploy.Runner(base)
            runner.log = base / "commands.log"
            deploy.atomic_private(runner.log, b"")
            source = base / "source.dump"
            source.write_bytes(SECRET)
            target = base / "output.dump"
            command = [sys.executable, "-c", "import sys; sys.stdout.buffer.write(sys.stdin.buffer.read())"]
            runner.run(command, stdin_path=source, stdout_path=target)
            self.assertEqual(target.read_bytes(), SECRET)
            self.assertEqual(target.stat().st_mode & 0o777, 0o600)
            with self.assertRaises(deploy.DeployError) as raised:
                runner.run([sys.executable, "-c", "import sys; sys.stderr.write('private failure'); sys.exit(7)"])
            self.assertEqual(raised.exception.code, "E_COMMAND")
            self.assertNotIn("private failure", str(raised.exception))
            self.assertIn(b"private failure", runner.log.read_bytes())
            runner.run([sys.executable, "-c", "print('sensitive compose configuration')"], sensitive=True)
            self.assertNotIn(b"sensitive compose configuration", runner.log.read_bytes())


if __name__ == "__main__":
    unittest.main()
