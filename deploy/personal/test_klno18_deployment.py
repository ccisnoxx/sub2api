""".18 的精确来源、固定迁移前缀、单向部署与关闭状态恢复合同。"""
import contextlib
import copy
import io
import json
from pathlib import Path
import subprocess
import types
import unittest
from unittest import mock

import deploy_hostdzire as deploy
import deploy_lifecycle
from deploy_local import local_maintenance
from deploy_trust import ToolBundle
import test_deploy_hostdzire as fixture
import test_maintenance_hostdzire as maintenance
import test_obs_deployment as obs
import test_resp_deployment as resp
import test_klno4_deployment as klno4
import test_klno17_deployment as klno17
from test_arch002 import tree_snapshot

ROOT = Path(__file__).resolve().parents[2]
OLD = dict(fixture.NEW, revision=deploy.KLNO18_OLD_REVISION, version=deploy.KLNO18_OLD_VERSION,
           input=deploy.KLNO18_OLD_IMAGE, image=deploy.KLNO18_OLD_IMAGE)
NEW = dict(fixture.NEW, revision=deploy.KLNO18_NEW_REVISION, version=deploy.KLNO18_NEW_VERSION)
# 直接引用已审定 SQL 的独立账本值，不从待测证明生成期望值。
ADDITIONS = (
    b"246_enable_gemini_image_generation_groups.sql|87267be7ab657173768989e2b0b105c5569c5930a7f1ed124f4cf81e5198297c\n",
)


def ledger(prefix=1):
    return b"".join(sorted(klno17.ledger().splitlines(keepends=True) + list(ADDITIONS[:prefix])))



class Klno18GitTests(unittest.TestCase):
    setUp = fixture.KinGitTests.setUp
    git = fixture.KinGitTests.git
    commit = fixture.KinGitTests.commit

    def test_fixed_upgrade_and_control_successor_bind_all_inputs_and_only_one_new_sql(self):
        proof = self.evidence.compatibility(OLD, NEW)
        self.assertEqual((len(proof["changed_paths"]), deploy.sha(deploy.canonical(proof["changed_paths"]))),
                         (162, '37312579de49b1a29fc271d8cb17ec49fb87525c0e8a6cef016d472ea3ef33f1'))
        self.assertEqual(len(proof["klno18_upgrade"]["frontend_changed_paths"]), 42)
        self.assertEqual(self.evidence.source_at(NEW["revision"]), deploy.KLNO18_NEW_SOURCE)
        self.assertEqual(self.git("diff", "--name-status", OLD["revision"], NEW["revision"],
                                  "--", "backend/migrations/*.sql").splitlines(),
                         ["A\tbackend/migrations/" + line.split(b"|", 1)[0].decode() for line in ADDITIONS])
        for line in ADDITIONS:
            name, checksum = line.rstrip(b"\n").split(b"|")
            raw = self.evidence.git("show", NEW["revision"] + ":backend/migrations/" + name.decode())
            self.assertEqual(deploy.sha(raw.strip()), checksum.decode())
        old_sql = self.git("ls-tree", "-r", "--name-only", OLD["revision"], "--", "backend/migrations").splitlines()
        old_sql = [path for path in old_sql if path.endswith(".sql")]
        self.assertEqual(len(old_sql), 315)
        for name in old_sql:
            path = ":" + name
            self.assertEqual(self.evidence.git("show", OLD["revision"] + path),
                             self.evidence.git("show", NEW["revision"] + path))
        self.assertEqual(self.git("diff", OLD["revision"], NEW["revision"], "--",
                                  "backend/internal/repository/migrations_runner.go"), "")
        self.git("checkout", "-q", "--detach", NEW["revision"])
        (self.root / "README.md").write_text("固定 .18 应用的文档后继。\n")
        (self.root / "deploy/personal/deploy_local.py").write_text("# 部署工具后继\n")
        successor = dict(NEW, revision=self.commit(".18 nonapplication successor"))
        # detached CI 克隆不保证存在 origin/main；正式应用合并的首亲固定为旧 personal。
        self.git("update-ref", "-d", "refs/remotes/origin/main")
        merged = subprocess.run(["git", "-C", str(self.root), "commit-tree",
                                 self.git("rev-parse", successor["revision"] + "^{tree}"),
                                 "-p", OLD["revision"], "-p", successor["revision"]],
                                input=b"formal personal merge with unchanged build inputs\n",
                                capture_output=True, check=True).stdout.decode().strip()
        self.assertEqual(len(self.git("rev-list", "--parents", "-n", "1", merged).split()), 3)
        successor = dict(NEW, revision=merged)
        for old in (OLD, NEW, successor):
            approved = self.evidence.compatibility(old, successor)
            deploy.validate_compatibility_proof(approved, old["revision"], successor["revision"],
                                               old["version"], successor["version"])
            self.assertEqual(approved["klno18_upgrade"]["direction"], "upgrade" if old == OLD else "redeploy")
            with self.assertRaises(deploy.DeployError): deploy.require_image_rollback(approved)

    def test_unknown_content_modes_types_frontend_raw_source_or_missing_anchor_are_rejected(self):
        cases = {"runtime": "backend/internal/service/user_service.go",
                 "migration": "backend/migrations/246_enable_gemini_image_generation_groups.sql",
                 "old_sql_mode": "backend/migrations/251_add_usage_log_timing.sql",
                 "new_sql_type": "backend/migrations/246_enable_gemini_image_generation_groups.sql",
                 "frontend": "frontend/src/api/client.ts", "frontend_mode": "frontend/src/api/client.ts",
                 "raw": deploy.KLNO18_RAW_PATHS[0], "raw_type": deploy.KLNO18_RAW_PATHS[1],
                 "source": "deploy/personal-source.json", "source_extra": "deploy/personal-source.json"}
        for kind, relative in cases.items():
            with self.subTest(kind=kind):
                self.git("checkout", "-q", "--detach", NEW["revision"])
                path = self.root / relative
                if kind.startswith("source"):
                    source = json.loads(path.read_bytes())
                    source["upstream_sha" if kind == "source" else "unexpected"] = "0" * 40
                    path.write_text(json.dumps(source))
                elif kind.endswith("mode"): path.chmod(0o755)
                elif kind.endswith("type"): path.unlink(); path.symlink_to("unreviewed")
                else: path.write_bytes(path.read_bytes() + b"\nunreviewed\n")
                unknown = dict(NEW, revision=self.commit(kind))
                for old in (OLD, NEW, unknown):
                    with self.assertRaises(deploy.DeployError): self.evidence.compatibility(old, unknown)
        tree = self.git("rev-parse", NEW["revision"] + "^{tree}")
        revision = subprocess.run(["git", "-C", str(self.root), "commit-tree", tree,
                                   "-p", OLD["revision"], "-p", deploy.KLNO18_NEW_SOURCE["upstream_sha"]],
                                  input=b"same inputs without reviewed candidate\n", capture_output=True, check=True).stdout.decode().strip()
        self.assertEqual(self.evidence.source_at(revision), deploy.KLNO18_NEW_SOURCE)
        for old in (OLD, NEW, dict(NEW, revision=revision)):
            with self.assertRaises(deploy.DeployError): self.evidence.compatibility(old, dict(NEW, revision=revision))

    def test_reverse_wrong_baseline_image_release_or_upstream_tag_is_rejected(self):
        for old, target in ((NEW, OLD), (NEW, klno4.OLD), (NEW, resp.OLD), (NEW, obs.OLD),
                            (dict(OLD, image=fixture.NEW["image"]), NEW),
                            (dict(OLD, revision=deploy.RESP_OLD_REVISION), NEW),
                            (dict(OLD, version=deploy.RESP_OLD_VERSION), NEW),
                            (dict(OLD, source="https://github.com/KlN-4096/sub2api"), NEW),
                            (OLD, dict(NEW, version="0.2.18-klno.3-tps.2")),
                            (OLD, dict(NEW, version="0.2.18-klno.3-tps.1-rc.1"))):
            with self.subTest(old=old["revision"], version=target["version"]), self.assertRaises(deploy.DeployError):
                self.evidence.compatibility(old, target)
        self.git("tag", "-f", deploy.KLNO18_NEW_SOURCE["upstream_tag"], OLD["revision"])
        with self.assertRaises(deploy.DeployError): self.evidence.compatibility(OLD, NEW)

    def test_local_resume_requires_real_personal_tag_before_transport(self):
        bundle = ToolBundle(self.root / "deploy/personal", "1" * 40, {})
        args = types.SimpleNamespace(maintenance="resume", target_revision=NEW["revision"],
                                     deployment_record="20261013T180000Z-123456789abc",
                                     target=None, version=None, rollback=None, dry_run=False)
        self.git("update-ref", "-d", "refs/tags/v" + deploy.KLNO18_NEW_VERSION)
        with mock.patch("deploy_local.SSHTransport", side_effect=AssertionError("触及运输")):
            with self.assertRaises(deploy.DeployError): local_maintenance(args, bundle, deploy.load_config())
        self.git("tag", "v" + deploy.KLNO18_NEW_VERSION, NEW["revision"])
        with mock.patch("deploy_local.SSHTransport") as transport, contextlib.redirect_stdout(io.StringIO()):
            transport.return_value.__enter__.return_value.maintenance.return_value = {"phase": "open"}
            self.assertEqual(local_maintenance(args, bundle, deploy.load_config()), 0)
            transport.return_value.__enter__.return_value.maintenance.assert_called_once_with(
                "resume", NEW["revision"], args.deployment_record)
        self.git("tag", "-f", "v" + deploy.KLNO18_NEW_VERSION, OLD["revision"])
        with mock.patch("deploy_local.SSHTransport", side_effect=AssertionError("触及运输")):
            with self.assertRaises(deploy.DeployError): local_maintenance(args, bundle, deploy.load_config())


class Klno18LedgerTests(unittest.TestCase):
    def test_each_committed_prefix_retains_original_database_order_and_complete_requires_single_sql(self):
        before = ledger(0)
        self.assertEqual(deploy.migration_fingerprint(before), deploy.KLNO18_OLD_MIGRATION_STATE)
        for prefix in range(2):
            self.assertEqual(deploy.klno18_ledger_prefix(before, ledger(prefix)), prefix)
            if prefix in (0, 1):
                self.assertEqual(deploy.klno18_ledger_baseline(ledger(prefix), prefix == 1), before)
        with self.assertRaises(deploy.DeployError): deploy.klno18_ledger_baseline(before, True)
        operation = deploy.Deployment()
        operation.klno18_baseline = before
        operation.observe_klno18_prefix(ledger(1))
        with self.assertRaises(deploy.DeployError): operation.observe_klno18_prefix(ledger(0))

    def test_unknown_duplicate_missing_wrong_checksum_old_row_or_order_changes_are_rejected(self):
        before, complete = ledger(0), ledger(1)
        cases = [complete + b"unknown.sql|" + b"0" * 64 + b"\n", complete + ADDITIONS[0],
                 complete.replace(ADDITIONS[0], ADDITIONS[0][:-2] + b"0\n"),
                 complete.replace(fixture.KIN_LEDGER_ROW, b""),
                 b"".join(reversed(complete.splitlines(keepends=True))),
                 complete.replace(fixture.CUMULATIVE_LEDGER_ROWS[0], fixture.CUMULATIVE_LEDGER_ROWS[0][:-2] + b"0\n")]
        for observed in cases:
            with self.subTest(observed=deploy.migration_fingerprint(observed)), self.assertRaises(deploy.DeployError):
                deploy.klno18_ledger_prefix(before, observed)


class Klno18DeploymentTests(unittest.TestCase):
    setUp = fixture.DeploymentTests.setUp
    execute = fixture.DeploymentTests.execute
    persisted = fixture.DeploymentTests.persisted
    starts = fixture.DeploymentTests.starts

    def prepare(self):
        self.before, self.after = ledger(0), ledger(1)
        old = dict(OLD, image_id=fixture.OLD_ID)
        self.docker.images[old["image"]] = self.docker.images[fixture.OLD_ID] = fixture.image(old, fixture.OLD_ID)
        self.docker.current = self.docker.config_image = old["image"]
        self.docker.migration_listing = self.before
        self.docker.migrations_on_start = self.after
        state = self.base / ".personal-deploy"
        previous = "20261013T101751Z-4b2b5c9d445b"
        deploy.private_directory(state)
        deploy.private_directory(state / "records")
        deploy.private_directory(state / "records" / previous)
        deploy.atomic_private(state / "records" / previous / "record.json", deploy.canonical({
            "id": previous, "status": "success", "action": "deploy", "target": old,
            "migration_state": deploy.migration_fingerprint(self.before)}))
        deploy.atomic_private(state / "image.env", ("SUB2API_IMAGE=" + old["image"] +
                                                    "\nSUB2API_DEPLOYMENT_ID=" + previous + "\n").encode())
        self.docker.images[NEW["image"]] = self.docker.images[fixture.NEW_ID] = fixture.image(NEW, fixture.NEW_ID)
        self.docker.failure_image = NEW["image"]
        value = fixture.request(NEW, old)
        value["compatibility"] = deploy.GitEvidence(ROOT).compatibility(old, NEW)
        return value, previous

    def test_readonly_upgrade_then_success_and_redeploy_require_complete_316_ledger(self):
        value, _ = self.prepare()
        deploy.atomic_private(self.base / ".personal-deploy/deployment.lock", b"")
        before = tree_snapshot(self.base)
        result, code = deploy.Deployment(self.base, self.docker).dry_run(value)
        self.assertEqual((code, result["migration_state"]["rows"]), (0, 315))
        self.assertEqual(tree_snapshot(self.base), before)
        self.assertEqual(self.starts(), [])
        result, code = self.execute(value)
        self.assertEqual((code, result["status"]), (0, "success"))
        saved = self.persisted(result)
        self.assertEqual(saved["migration_state"], deploy.migration_fingerprint(self.after))
        self.assertEqual(saved["migration_state"]["rows"], 316)
        self.assertEqual(saved["migration_committed_prefix"], 1)
        self.assertEqual(saved["migration_state_before"], deploy.KLNO18_OLD_MIGRATION_STATE)
        self.assertEqual((self.base / ".personal-deploy/records" / result["id"] / "migration-ledger-before.txt").read_bytes(), self.before)
        commands = [args for args, _ in self.docker.commands]
        self.assertLess(next(i for i, args in enumerate(commands) if "pg_dump" in args[-1]),
                        next(i for i, args in enumerate(commands) if "--no-deps" in args))
        old = deploy.Deployment(self.base, self.docker).running()
        retry = fixture.request(NEW, old)
        retry["compatibility"] = deploy.GitEvidence(ROOT).compatibility(old, NEW)
        result, code = self.execute(retry)
        self.assertEqual((code, result["status"]), (0, "success"))
        self.assertEqual(self.persisted(result)["compatibility"]["klno18_upgrade"]["direction"], "redeploy")
        self.assertEqual(self.docker.migration_listing, self.after)

    def test_rehashed_proof_disguise_or_tampering_fails_before_backup_pull_start(self):
        value, _ = self.prepare()
        mutations = {
            "runtime": lambda p: p["klno18_upgrade"].update(new_runtime_sha256="0" * 64),
            "anchor": lambda p: p["klno18_upgrade"].update(new_runtime_revision="0" * 40),
            "frontend": lambda p: p["klno18_upgrade"].update(new_frontend_sha256="0" * 64),
            "frontend_paths": lambda p: p["klno18_upgrade"]["frontend_changed_paths"].pop(),
            "raw": lambda p: p["klno18_upgrade"].update(raw_sha256="0" * 64),
            "source": lambda p: p["upstream"].update(upstream_sha="0" * 40),
            "migration": lambda p: p["klno18_upgrade"]["migrations"].update({"246_enable_gemini_image_generation_groups.sql": "0" * 64}),
            "rollback": lambda p: p["klno18_upgrade"].update(image_rollback_compatible=True),
            "schema": lambda p: p["klno18_upgrade"].update(schema_rollback="restore"),
            "direction": lambda p: p["klno18_upgrade"].update(direction="redeploy"),
            "changed": lambda p: p["changed_paths"].pop(),
            "zero": lambda p: (p.pop("klno18_upgrade"), p.update(changed_paths=[])),
            "resp": lambda p: (p.pop("klno18_upgrade"), p.update(changed_paths=[], resp_upgrade=deploy.approved_resp_upgrade("redeploy"))),
            "klno4": lambda p: (p.pop("klno18_upgrade"), p.update(changed_paths=[], klno4_upgrade=deploy.approved_klno4_upgrade("redeploy"))),
        }
        for kind, mutate in mutations.items():
            with self.subTest(kind=kind):
                bad = copy.deepcopy(value); mutate(bad["compatibility"]); resp.rehash(bad["compatibility"])
                self.docker.commands.clear()
                result, code = self.execute(bad)
                self.assertEqual((code, result["error_code"]), (1, "E_COMPATIBILITY"))
                self.assertNotIn("backups", self.persisted(result))
                self.assertFalse(any(args[:2] == ["docker", "pull"] for args, _ in self.docker.commands))
                self.assertEqual(self.starts(), [])
        for field, wrong in (("image", NEW["image"]), ("revision", deploy.KLNO4_NEW_REVISION),
                             ("version", deploy.RESP_NEW_VERSION), ("source", "https://github.com/KlN-4096/sub2api")):
            with self.subTest(field=field), self.assertRaises(deploy.DeployError):
                deploy.Deployment(self.base, self.docker).validate_proof(value, dict(OLD, **{field: wrong}), NEW)

    def test_partial_transactions_cancel_health_or_unknown_ledger_never_restore_old_application(self):
        for kind, prefix in (("wait", 0), ("wait", 1), ("cancel", 1), ("health", 1),
                             ("incomplete", 0), ("unknown", 1)):
            with self.subTest(kind=kind, prefix=prefix):
                self.setUp(); value, previous = self.prepare()
                self.docker.migrations_on_start = ledger(prefix)
                if kind == "wait": self.docker.wait_failure = True
                elif kind == "health": self.docker.health_failure = True
                elif kind == "unknown": self.docker.migrations_on_start += b"unknown.sql|" + b"0" * 64 + b"\n"
                start = deploy_lifecycle.Deployment.start
                def interrupted(operation, target):
                    start(operation, target)
                    if kind == "cancel": raise deploy.DeployError("E_INTERRUPTED", "替身取消")
                with mock.patch.object(deploy_lifecycle.Deployment, "start", interrupted): result, code = self.execute(value)
                self.assertEqual((code, result["rollback_status"], result["rollback_error"]), (1, "blocked", "E_COMPATIBILITY"))
                self.assertEqual(len(self.starts()), 1)
                self.assertEqual(self.docker.current, NEW["image"])
                self.assertEqual(deploy.Deployment(self.base, self.docker).selected()["record_id"], previous)
                saved = self.persisted(result)
                if kind == "unknown": self.assertEqual(saved["migration_observation_error"], "E_MIGRATION")
                else: self.assertEqual(saved["migration_committed_prefix"], prefix)
                # 部分迁移不能借旧成功记录自动重试接管。
                if prefix > 0:
                    self.docker.healthy = True
                    retry = copy.deepcopy(value)
                    retry["expected_old"] = deploy.Deployment(self.base, self.docker).running()
                    result, code = self.execute(retry)
                    self.assertEqual((code, result["error_code"]), (1, "E_STATE"))
                    self.assertEqual(len(self.starts()), 1)

    def test_bad_initial_ledger_or_new_redeploy_missing_migration_is_rejected(self):
        for kind in ("old_row", "preapplied"):
            with self.subTest(kind=kind):
                self.setUp(); value, previous = self.prepare()
                self.docker.migration_listing = ledger(1) if kind == "preapplied" else self.before.replace(fixture.KIN_LEDGER_ROW, b"")
                path = self.base / ".personal-deploy/records" / previous / "record.json"
                prior = json.loads(path.read_bytes()); prior["migration_state"] = deploy.migration_fingerprint(self.docker.migration_listing)
                deploy.atomic_private(path, deploy.canonical(prior))
                result, code = self.execute(value)
                self.assertEqual((code, result["error_code"]), (1, "E_MIGRATION"))
                self.assertNotIn("backups", self.persisted(result)); self.assertEqual(self.starts(), [])
        self.setUp(); value, _ = self.prepare(); result, code = self.execute(value); self.assertEqual(code, 0)
        running = deploy.Deployment(self.base, self.docker).running()
        self.docker.migration_listing = ledger(0)
        path = self.base / ".personal-deploy/records" / result["id"] / "record.json"
        saved = json.loads(path.read_bytes()); saved["migration_state"] = deploy.migration_fingerprint(ledger(0))
        deploy.atomic_private(path, deploy.canonical(saved))
        retry = fixture.request(NEW, running); retry["compatibility"] = deploy.GitEvidence(ROOT).compatibility(running, NEW)
        failed, code = self.execute(retry)
        self.assertEqual((code, failed["error_code"]), (1, "E_MIGRATION")); self.assertEqual(len(self.starts()), 1)
        with self.assertRaises(deploy.DeployError):
            deploy.Deployment(self.base, self.docker).rollback_binding(
                fixture.request(OLD, running, "rollback", result["id"]), running, OLD)

    def test_ledger_drift_after_backup_stops_before_start(self):
        value, previous = self.prepare(); pull = deploy_lifecycle.Deployment.pull_target
        def drift(operation, target):
            resolved = pull(operation, target)
            self.docker.migration_listing = ledger(1)
            return resolved
        with mock.patch.object(deploy_lifecycle.Deployment, "pull_target", drift): result, code = self.execute(value)
        self.assertEqual((code, result["error_code"]), (1, "E_MIGRATION"))
        self.assertEqual(self.starts(), []); self.assertEqual(self.docker.current, OLD["image"])
        self.assertEqual((self.base / "docker-compose.yml").read_bytes(), fixture.ORIGINAL)
        self.assertEqual(deploy.Deployment(self.base, self.docker).selected()["record_id"], previous)

    def test_selection_failure_before_or_after_replace_retains_target_and_no_restore(self):
        for replaced in (False, True):
            with self.subTest(replaced=replaced):
                self.setUp(); value, previous = self.prepare(); atomic = deploy_lifecycle.atomic_private
                def fail(path, data):
                    if path.name == "image.env" and not replaced: raise OSError("选择替换前写入失败")
                    atomic(path, data)
                    if path.name == "image.env": raise OSError("选择替换后落盘失败")
                with mock.patch.object(deploy_lifecycle, "atomic_private", side_effect=fail): result, code = self.execute(value)
                self.assertEqual((code, result["selection_committed"]), (1, replaced))
                if replaced: self.assertNotIn("rollback_status", result)
                else: self.assertEqual((result["rollback_status"], result["rollback_error"]), ("blocked", "E_COMPATIBILITY"))
                self.assertEqual(len(self.starts()), 1); self.assertEqual(self.docker.current, NEW["image"])
                self.assertEqual(deploy.Deployment(self.base, self.docker).selected()["record_id"], result["id"] if replaced else previous)


class Klno18MaintenanceTests(unittest.TestCase):
    setUp = maintenance.GateTests.setUp
    phase = maintenance.GateTests.phase
    persist = obs.ObsMaintenanceTests.persist
    action = obs.ObsMaintenanceTests.action

    def prepare(self):
        self.ledger = ledger(1)
        target = dict(NEW, image_id=fixture.NEW_ID)
        self.deployed = {"schema": 1, "id": self.id, "action": "deploy", "status": "success", "exit_code": 0,
                         "health_verified": True, "old": copy.deepcopy(OLD), "target": target,
                         "compatibility": deploy.GitEvidence(ROOT).compatibility(OLD, NEW),
                         "observed_running": dict(target, health="healthy", running=True),
                         "migration_state": deploy.migration_fingerprint(self.ledger)}
        self.persist()
        self.g.atomic(self.g.BASE / ".personal-deploy/image.env",
                      ("SUB2API_IMAGE=" + target["image"] + "\nSUB2API_DEPLOYMENT_ID=" + self.id + "\n").encode())
        self.app["Image"] = fixture.NEW_ID; self.app["Config"]["Image"] = target["image"]
        self.app["Config"]["Labels"] = {"org.opencontainers.image." + key: target[key] for key in ("revision", "version", "source")}
        self.app["Config"]["Labels"].update({"com.docker.compose.project": self.g.CONFIG.project,
                                             "com.docker.compose.service": self.g.CONFIG.app_service})
        self.action("freeze")

    def test_upgrade_and_redeploy_resume_reuse_original_site_identity_and_marker(self):
        self.prepare(); self.action("resume", NEW["revision"], self.id)
        record = json.loads((self.g.DIRECTORY / "record.json").read_bytes())
        self.assertEqual(record["source_sha"], deploy.KIN_NEW_REVISION)
        self.assertEqual((record["accepted_source_sha"], record["accepted_deployment_id"]), (NEW["revision"], self.id))
        self.deployed["old"] = copy.deepcopy(self.deployed["target"])
        self.deployed["compatibility"] = deploy.GitEvidence(ROOT).compatibility(NEW, NEW)
        self.persist(); self.action("freeze")
        self.assertEqual(self.g.FLAG.read_bytes(), b"REL-002 42907bcab278\n")
        self.action("resume", NEW["revision"], self.id)
        self.assertEqual(self.phase(), "open")

    def test_wrong_proof_identity_partial_or_forged_complete_ledger_remains_closed(self):
        for kind in ("no_explicit", "failed", "old_digest", "proof", "resp", "oci_version", "image_id", "ledger", "forged_ledger"):
            with self.subTest(kind=kind):
                self.setUp(); self.prepare(); revision, record = NEW["revision"], self.id
                if kind == "no_explicit": revision = record = None
                elif kind == "failed": self.deployed["status"] = "failed"
                elif kind == "old_digest": self.deployed["old"]["image"] = fixture.NEW["image"]
                elif kind == "proof": self.deployed["compatibility"]["klno18_upgrade"]["image_rollback_compatible"] = True
                elif kind == "resp":
                    proof = self.deployed["compatibility"]; proof.pop("klno18_upgrade")
                    proof.update(changed_paths=[], resp_upgrade=deploy.approved_resp_upgrade("redeploy"))
                elif kind == "oci_version": self.app["Config"]["Labels"]["org.opencontainers.image.version"] = "unknown"
                elif kind == "image_id": self.app["Image"] = fixture.OLD_ID
                elif kind == "ledger": self.ledger = ledger(0)
                elif kind == "forged_ledger":
                    self.ledger = self.ledger.replace(fixture.KIN_LEDGER_ROW, fixture.KIN_LEDGER_ROW[:-2] + b"0\n")
                    self.deployed["migration_state"] = deploy.migration_fingerprint(self.ledger)
                resp.rehash(self.deployed["compatibility"]); self.persist()
                with self.assertRaises((RuntimeError, deploy.DeployError)): self.action("resume", revision, record)
                self.assertTrue(self.g.FLAG.exists()); self.assertEqual(self.phase(), "closed")


if __name__ == "__main__": unittest.main()
