"""RESP 精确正向证明、无迁移生命周期及原维护入口恢复的合同。"""
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
import test_deploy_hostdzire as fixture
import test_maintenance_hostdzire as maintenance
import test_obs_deployment as obs
from deploy_trust import ToolBundle

ROOT = Path(__file__).resolve().parents[2]
OLD = dict(fixture.NEW, revision=deploy.RESP_OLD_REVISION, version=deploy.RESP_OLD_VERSION,
           input=deploy.RESP_OLD_IMAGE, image=deploy.RESP_OLD_IMAGE)
RESP = dict(fixture.NEW, revision=deploy.RESP_NEW_REVISION, version=deploy.RESP_NEW_VERSION)


def complete_ledger():
    return b"".join(sorted(fixture.kin_old_ledger().splitlines(keepends=True) + [fixture.KIN_LEDGER_ROW]))


def rehash(proof):
    proof["sha256"] = deploy.sha(deploy.canonical({key: value for key, value in proof.items() if key != "sha256"}))


class RespGitTests(unittest.TestCase):
    setUp = fixture.KinGitTests.setUp
    git = fixture.KinGitTests.git
    commit = fixture.KinGitTests.commit

    def test_real_forward_redeploy_and_nonapplication_successor(self):
        proof = self.evidence.compatibility(OLD, RESP)
        self.assertEqual(proof["changed_paths"], list(deploy.RESP_CHANGED_PATHS))
        self.assertEqual(len(proof["changed_paths"]), 23)
        self.assertEqual(self.evidence.runtime_fingerprint(OLD["revision"]), deploy.OBS_NEW_RUNTIME)
        self.assertEqual(proof["resp_upgrade"]["new_runtime_sha256"],
                         self.evidence.runtime_fingerprint(RESP["revision"]))
        for revision in (OLD["revision"], RESP["revision"]):
            self.assertEqual(self.evidence.source_at(revision), deploy.KIN_NEW_SOURCE)
            self.assertEqual(self.evidence.tree_fingerprint(revision, ["frontend"]), deploy.RESP_FRONTEND)
            self.assertEqual(self.evidence.tree_fingerprint(revision, deploy.RESP_RAW_PATHS), deploy.RESP_RAW_SHA256)
        self.assertEqual(self.git("diff", "--name-only", OLD["revision"], RESP["revision"],
                                  "--", "backend/migrations", "backend/ent/schema", "frontend", *deploy.RESP_RAW_PATHS), "")
        self.git("checkout", "-q", "--detach", RESP["revision"])
        (self.root / "README.md").write_text("文档后继，保留 RESP 全部应用输入。\n")
        (self.root / "deploy/personal/deploy_local.py").write_text("# 控制工具后继\n")
        successor = dict(RESP, revision=self.commit("RESP control successor"))
        for old in (OLD, RESP, successor):
            proof = self.evidence.compatibility(old, successor)
            deploy.validate_compatibility_proof(proof, old["revision"], successor["revision"],
                                               old["version"], successor["version"])
            self.assertEqual(proof["resp_upgrade"]["direction"], "upgrade" if old == OLD else "redeploy")
            self.assertIs(proof["resp_upgrade"]["image_rollback_compatible"], False)
            self.assertEqual(proof["resp_upgrade"]["schema_rollback"], "retain")

    def test_unknown_tree_source_mode_type_frontend_or_raw_input_is_rejected(self):
        cases = {
            "runtime": "backend/internal/service/user_service.go",
            "migration": "backend/migrations/242_drop_platform_check_constraints.sql",
            "mode": "backend/migrations/251_add_usage_log_timing.sql",
            "type": "deploy/config.example.yaml",
            "frontend": "frontend/src/api/client.ts",
            "frontend_mode": "frontend/src/api/client.ts",
            "raw": deploy.RESP_RAW_PATHS[0],
            "raw_type": deploy.RESP_RAW_PATHS[1],
            "source": "deploy/personal-source.json",
            "source_extra": "deploy/personal-source.json",
        }
        for kind, relative in cases.items():
            with self.subTest(kind=kind):
                self.git("checkout", "-q", "--detach", RESP["revision"])
                path = self.root / relative
                if kind.startswith("source"):
                    source = json.loads(path.read_bytes())
                    source["upstream_sha" if kind == "source" else "unexpected"] = "0" * 40
                    path.write_text(json.dumps(source))
                elif kind.endswith("mode"):
                    path.chmod(0o755)
                elif kind.endswith("type"):
                    path.unlink(); path.symlink_to("unreviewed")
                else:
                    path.write_bytes(path.read_bytes() + b"\nunreviewed\n")
                unknown = dict(RESP, revision=self.commit(kind))
                for old in (OLD, RESP, unknown):
                    with self.assertRaises(deploy.DeployError): self.evidence.compatibility(old, unknown)

    def test_identical_inputs_without_resp_anchor_are_rejected(self):
        tree = self.git("rev-parse", RESP["revision"] + "^{tree}")
        revision = subprocess.run(["git", "-C", str(self.root), "commit-tree", tree, "-p", OLD["revision"]],
                                  input=b"same inputs without RESP anchor\n", capture_output=True, check=True).stdout.decode().strip()
        unknown = dict(RESP, revision=revision)
        self.assertEqual(self.evidence.runtime_fingerprint(revision), deploy.RESP_NEW_RUNTIME)
        for old, target in ((OLD, unknown), (unknown, RESP), (unknown, unknown)):
            with self.subTest(old=old["revision"]), self.assertRaises(deploy.DeployError):
                self.evidence.compatibility(old, target)

    def test_reverse_wrong_baseline_digest_or_release_is_rejected(self):
        for old, target in ((RESP, OLD), (RESP, obs.OLD), (RESP, self.old),
                            (OLD, dict(RESP, revision="5a6c424d7ab8678bf8dc879f23c7fe468f7f714f")),
                            (dict(RESP, revision="5a6c424d7ab8678bf8dc879f23c7fe468f7f714f"), RESP),
                            (dict(OLD, image=fixture.NEW["image"]), RESP),
                            (dict(OLD, revision=deploy.OBS_NEW_REVISION), RESP),
                            (dict(OLD, version="0.2.15-klno.2-tps.1"), RESP),
                            (OLD, dict(RESP, version=deploy.OBS_NEW_VERSION)),
                            (OLD, dict(RESP, version="0.2.15-klno.2-tps.4"))):
            with self.subTest(old=old["revision"], version=target["version"]), self.assertRaises(deploy.DeployError):
                self.evidence.compatibility(old, target)

    def test_local_resume_requires_real_tag_and_all_git_inputs_before_transport(self):
        bundle = ToolBundle(self.root / "deploy/personal", "1" * 40, {})
        args = types.SimpleNamespace(maintenance="resume", target_revision=RESP["revision"],
                                     deployment_record="20261012T180000Z-123456789abc",
                                     target=None, version=None, rollback=None, dry_run=False)
        # 标签只存在于本测试临时 clone；不会创建可发布标签。
        self.git("update-ref", "-d", "refs/tags/v" + deploy.RESP_NEW_VERSION)
        with mock.patch("deploy_local.SSHTransport", side_effect=AssertionError("触及运输")):
            with self.assertRaises(deploy.DeployError): local_maintenance(args, bundle, deploy.load_config())
        self.git("tag", "v" + deploy.RESP_NEW_VERSION, RESP["revision"])
        with mock.patch("deploy_local.SSHTransport") as transport, contextlib.redirect_stdout(io.StringIO()):
            transport.return_value.__enter__.return_value.maintenance.return_value = {"phase": "open"}
            self.assertEqual(local_maintenance(args, bundle, deploy.load_config()), 0)
            transport.return_value.__enter__.return_value.maintenance.assert_called_once_with(
                "resume", RESP["revision"], args.deployment_record)
        self.git("tag", "-f", "v" + deploy.RESP_NEW_VERSION, OLD["revision"])
        with mock.patch("deploy_local.SSHTransport", side_effect=AssertionError("触及运输")):
            with self.assertRaises(deploy.DeployError): local_maintenance(args, bundle, deploy.load_config())


class RespDeploymentTests(unittest.TestCase):
    setUp = fixture.DeploymentTests.setUp
    execute = fixture.DeploymentTests.execute
    persisted = fixture.DeploymentTests.persisted
    starts = fixture.DeploymentTests.starts

    def prepare(self):
        self.ledger = complete_ledger()
        old = dict(OLD, image_id=fixture.OLD_ID)
        self.docker.images[old["image"]] = self.docker.images[fixture.OLD_ID] = fixture.image(old, fixture.OLD_ID)
        self.docker.current = self.docker.config_image = old["image"]
        self.docker.migration_listing = self.ledger
        state = self.base / ".personal-deploy"
        previous = "20261013T032447Z-22e878072cb7"
        deploy.private_directory(state / "records" / previous)
        deploy.atomic_private(state / "records" / previous / "record.json", deploy.canonical({
            "id": previous, "status": "success", "action": "deploy", "target": old,
            "migration_state": deploy.migration_fingerprint(self.ledger)}))
        deploy.atomic_private(state / "image.env", ("SUB2API_IMAGE=" + old["image"] +
                                                    "\nSUB2API_DEPLOYMENT_ID=" + previous + "\n").encode())
        self.docker.images[RESP["image"]] = self.docker.images[fixture.NEW_ID] = fixture.image(RESP, fixture.NEW_ID)
        self.docker.failure_image = RESP["image"]
        value = fixture.request(RESP, old)
        value["compatibility"] = deploy.GitEvidence(ROOT).compatibility(old, RESP)
        return value, previous

    def test_success_redeploy_and_readonly_preflight_preserve_exact_308_ledger(self):
        value, _ = self.prepare()
        result, code = self.execute(value)
        self.assertEqual((code, result["status"]), (0, "success"))
        saved = self.persisted(result)
        self.assertEqual(saved["migration_state"], {"rows": 308,
                         "filename_checksum_sha256": "a31c942e9e772bef1af9c68f930ee35afaa8c3591b5d1c22609f9bf7d30da882"})
        self.assertIs(saved["health_verified"], True)
        commands = [args for args, _ in self.docker.commands]
        self.assertLess(next(i for i, args in enumerate(commands) if "pg_dump" in args[-1]),
                        next(i for i, args in enumerate(commands) if "--no-deps" in args))
        old = deploy.Deployment(self.base, self.docker).running()
        retry = fixture.request(RESP, old)
        retry["compatibility"] = deploy.GitEvidence(ROOT).compatibility(old, RESP)
        result, code = self.execute(retry)
        self.assertEqual((code, result["status"]), (0, "success"))
        self.assertEqual(self.persisted(result)["compatibility"]["resp_upgrade"]["direction"], "redeploy")
        self.assertEqual(self.docker.migration_listing, self.ledger)
        self.docker.commands.clear()
        before = (self.base / ".personal-deploy/image.env").read_bytes()
        result, code = deploy.Deployment(self.base, self.docker).dry_run(retry)
        self.assertEqual((code, result["status"]), (0, "dry-run"))
        self.assertEqual((self.base / ".personal-deploy/image.env").read_bytes(), before)
        self.assertFalse(any(args[:2] == ["docker", "pull"] or "--no-deps" in args or "pg_dump" in args[-1]
                             for args, _ in self.docker.commands))

    def test_rehashed_proof_disguise_and_tampering_fail_before_backup_pull_or_start(self):
        value, _ = self.prepare()
        mutations = {
            "runtime": lambda p: p["resp_upgrade"].update(new_runtime_sha256="0" * 64),
            "anchor": lambda p: p["resp_upgrade"].update(new_runtime_revision="0" * 40),
            "frontend": lambda p: p["resp_upgrade"].update(new_frontend_sha256="0" * 64),
            "raw": lambda p: p["resp_upgrade"].update(raw_sha256="0" * 64),
            "source": lambda p: p["upstream"].update(upstream_sha="0" * 40),
            "migration": lambda p: p["resp_upgrade"]["migrations"].update({"242_drop_platform_check_constraints.sql": "0" * 64}),
            "rollback": lambda p: p["resp_upgrade"].update(image_rollback_compatible=True),
            "schema": lambda p: p["resp_upgrade"].update(schema_rollback="restore"),
            "direction": lambda p: p["resp_upgrade"].update(direction="redeploy"),
            "changed": lambda p: p["changed_paths"].pop(),
            "zero": lambda p: (p.pop("resp_upgrade"), p.update(changed_paths=[])),
            "kin": lambda p: (p.pop("resp_upgrade"), p.update(changed_paths=[], kin_upgrade=deploy.approved_kin_upgrade("redeploy"))),
            "obs": lambda p: (p.pop("resp_upgrade"), p.update(changed_paths=[], obs_upgrade=deploy.approved_obs_upgrade("redeploy"))),
        }
        for kind, mutate in mutations.items():
            with self.subTest(kind=kind):
                bad = copy.deepcopy(value); mutate(bad["compatibility"]); rehash(bad["compatibility"])
                self.docker.commands.clear()
                result, code = self.execute(bad)
                self.assertEqual((code, result["error_code"]), (1, "E_COMPATIBILITY"))
                self.assertNotIn("backups", self.persisted(result))
                self.assertFalse(any(args[:2] == ["docker", "pull"] for args, _ in self.docker.commands))
                self.assertEqual(self.starts(), [])

    def test_remote_old_digest_binding_and_success_record_reject_reverse(self):
        value, _ = self.prepare()
        bad_old = dict(value["expected_old"], image=fixture.NEW["image"])
        with self.assertRaises(deploy.DeployError): deploy.Deployment(self.base, self.docker).validate_proof(value, bad_old, RESP)
        result, code = self.execute(value)
        self.assertEqual(code, 0)
        operation = deploy.Deployment(self.base, self.docker)
        running = operation.running(); rollback = fixture.request(dict(OLD, input=OLD["image"]), running, "rollback", result["id"])
        with self.assertRaises(deploy.DeployError): operation.rollback_binding(rollback, running, OLD)
        with self.assertRaises(deploy.DeployError): deploy.require_image_rollback(value["compatibility"])

    def test_start_health_or_ledger_failure_retains_scene_and_never_restores_c08(self):
        for kind in ("wait_failure", "health_failure", "ledger", "forged_prior_ledger", "selection_partial_commit"):
            with self.subTest(kind=kind):
                self.setUp(); value, previous = self.prepare()
                if kind in ("wait_failure", "health_failure"): setattr(self.docker, kind, True)
                elif kind == "ledger": self.docker.migrations_on_start = self.ledger + b"unknown.sql|" + b"0" * 64 + b"\n"
                elif kind == "forged_prior_ledger":
                    self.docker.migration_listing = self.ledger.replace(fixture.KIN_LEDGER_ROW, b"")
                    path = self.base / ".personal-deploy/records" / previous / "record.json"
                    prior = json.loads(path.read_bytes()); prior["migration_state"] = deploy.migration_fingerprint(self.docker.migration_listing)
                    deploy.atomic_private(path, deploy.canonical(prior))
                atomic = deploy_lifecycle.atomic_private
                def persist(path, data):
                    atomic(path, data)
                    if kind == "selection_partial_commit" and path.name == "image.env": raise OSError("选择替换后落盘失败")
                with mock.patch.object(deploy_lifecycle, "atomic_private", side_effect=persist): result, code = self.execute(value)
                self.assertEqual(code, 1)
                if kind == "forged_prior_ledger":
                    self.assertEqual(self.starts(), []); self.assertNotIn("backups", self.persisted(result))
                elif kind == "selection_partial_commit":
                    self.assertTrue(result["selection_committed"])
                    self.assertEqual(deploy.Deployment(self.base, self.docker).selected()["record_id"], result["id"])
                    self.assertNotIn("rollback_status", result)
                else:
                    self.assertEqual((result["rollback_status"], result["rollback_error"]), ("blocked", "E_COMPATIBILITY"))
                if kind != "forged_prior_ledger": self.assertEqual(len(self.starts()), 1)
                if kind != "selection_partial_commit":
                    self.assertEqual(deploy.Deployment(self.base, self.docker).selected()["record_id"], previous)


class RespMaintenanceTests(unittest.TestCase):
    setUp = maintenance.GateTests.setUp
    phase = maintenance.GateTests.phase
    persist = obs.ObsMaintenanceTests.persist
    action = obs.ObsMaintenanceTests.action

    def prepare(self):
        self.ledger = complete_ledger()
        target = dict(RESP, image_id=fixture.NEW_ID)
        self.deployed = {"schema": 1, "id": self.id, "action": "deploy", "status": "success", "exit_code": 0,
                         "health_verified": True, "old": copy.deepcopy(OLD), "target": target,
                         "compatibility": deploy.GitEvidence(ROOT).compatibility(OLD, RESP),
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

    def test_canonical_upgrade_and_redeploy_resume_preserve_original_site_and_marker(self):
        self.prepare(); self.action("resume", RESP["revision"], self.id)
        record = json.loads((self.g.DIRECTORY / "record.json").read_bytes())
        self.assertEqual(record["source_sha"], deploy.KIN_NEW_REVISION)
        self.assertEqual((record["accepted_source_sha"], record["accepted_deployment_id"]), (RESP["revision"], self.id))
        self.assertEqual(self.phase(), "open"); self.assertFalse(self.g.FLAG.exists())
        self.deployed["old"] = copy.deepcopy(self.deployed["target"])
        self.deployed["compatibility"] = deploy.GitEvidence(ROOT).compatibility(RESP, RESP)
        self.persist(); self.action("freeze")
        self.assertEqual(self.g.FLAG.read_bytes(), b"REL-002 42907bcab278\n")
        self.action("resume", RESP["revision"], self.id)
        self.assertEqual(self.phase(), "open")

    def test_running_digest_alone_wrong_record_oci_proof_or_ledger_never_opens(self):
        for kind in ("no_explicit", "sha", "id", "failed", "health_verified", "selection", "old_digest", "proof", "kin",
                     "oci_version", "oci_source", "image_id", "ledger", "forged_ledger", "flag", "site"):
            with self.subTest(kind=kind):
                self.setUp(); self.prepare(); revision, record_id = RESP["revision"], self.id
                if kind == "no_explicit": revision = record_id = None
                elif kind == "sha": revision = "0" * 40
                elif kind == "id": self.deployed["id"] = "20261012T180000Z-000000000000"
                elif kind == "failed": self.deployed["status"] = "failed"
                elif kind == "health_verified": self.deployed["health_verified"] = False
                elif kind == "selection": self.g.atomic(self.g.BASE / ".personal-deploy/image.env", b"SUB2API_IMAGE=unknown\nSUB2API_DEPLOYMENT_ID=" + self.id.encode() + b"\n")
                elif kind == "old_digest": self.deployed["old"]["image"] = fixture.NEW["image"]
                elif kind == "proof": self.deployed["compatibility"]["resp_upgrade"]["new_runtime_sha256"] = "0" * 64
                elif kind == "kin":
                    proof = self.deployed["compatibility"]; proof.pop("resp_upgrade")
                    proof.update(changed_paths=[], kin_upgrade=deploy.approved_kin_upgrade("redeploy"))
                elif kind.startswith("oci_"): self.app["Config"]["Labels"]["org.opencontainers.image." + kind[4:]] = "unknown"
                elif kind == "image_id": self.app["Image"] = fixture.OLD_ID
                elif kind == "ledger": self.ledger += b"unknown.sql|" + b"0" * 64 + b"\n"
                elif kind == "forged_ledger":
                    self.ledger = self.ledger.replace(fixture.KIN_LEDGER_ROW, b"242_drop_platform_check_constraints.sql|" + b"0" * 64 + b"\n")
                    self.deployed["migration_state"] = deploy.migration_fingerprint(self.ledger)
                elif kind == "flag": self.g.FLAG.write_bytes(b"unknown maintenance\n")
                elif kind == "site": self.g.SITE.write_bytes(b"unrelated site\n")
                rehash(self.deployed["compatibility"]); self.persist()
                with self.assertRaises((RuntimeError, deploy.DeployError)):
                    self.action("resume", revision, record_id)
                self.assertTrue(self.g.FLAG.exists()); self.assertEqual(self.phase(), "closed")


if __name__ == "__main__":
    unittest.main()
