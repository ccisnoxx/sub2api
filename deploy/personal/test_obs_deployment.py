"""OBS 固定 Git 输入、远端失败关闭与既有维护标记的直接合同。"""
import contextlib
import copy
import io
import json
import os
from pathlib import Path
import shlex
import subprocess
import sys
import types
import unittest
from unittest import mock

import deploy_hostdzire as deploy
import test_arch002 as architecture
import test_deploy_hostdzire as fixture
import test_maintenance_hostdzire as maintenance
from deploy_trust import ToolBundle

ROOT = Path(__file__).resolve().parents[2]
OBS = dict(fixture.NEW, revision=deploy.OBS_NEW_REVISION,
           version="0.2.15-klno.2-tps.2")
OLD = dict(fixture.NEW, revision="42907bcab278f93db48d7e1b39b28e28f664569e",
           version="0.2.15-klno.2-tps.1", image=deploy.OBS_OLD_IMAGE)


class ObsGitTests(unittest.TestCase):
    setUp = fixture.KinGitTests.setUp
    git = fixture.KinGitTests.git
    commit = fixture.KinGitTests.commit

    def test_fixed_forward_and_redeploy_bind_complete_inputs_without_new_tag(self):
        proof = self.evidence.compatibility(OLD, OBS)
        self.assertEqual(proof["changed_paths"], list(deploy.OBS_CHANGED_PATHS))
        self.assertEqual(len(proof["changed_paths"]), 32)
        self.assertEqual(proof["obs_upgrade"]["new_runtime_sha256"],
                         "3d3a9765382ecc98a2a9ad95d6940377b618d1f3e1e5675a16b734b6cfccb9c0")
        self.assertEqual(proof["obs_upgrade"]["new_frontend_sha256"],
                         "47b21affa698058f84a71bcce078c4e105965017a9f34c5707886d25a3e0a367")
        self.assertEqual(len(proof["obs_upgrade"]["frontend_changed_paths"]), 13)
        self.assertEqual(self.git("diff", "--name-only", OLD["revision"], OBS["revision"],
                                  "--", "backend/migrations"), "")
        for filename, checksum in proof["obs_upgrade"]["migrations"].items():
            path = "backend/migrations/" + filename
            self.assertTrue(self.evidence.git("ls-tree", OBS["revision"], "--", path).startswith(b"100644 blob "))
            self.assertEqual(deploy.sha(self.evidence.git("show", OBS["revision"] + ":" + path).strip()), checksum)
        self.git("checkout", "-q", "--detach", OBS["revision"])
        (self.root / "README.md").write_text("文档和部署控制后继；保留固定应用输入。\n")
        (self.root / "deploy/personal/deploy_local.py").write_text("# 控制后继\n")
        successor = dict(OBS, revision=self.commit("OBS documentation and control successor"))
        for old in (OLD, OBS, successor):
            proof = self.evidence.compatibility(old, successor)
            deploy.validate_compatibility_proof(proof, old["revision"], successor["revision"],
                                               old["version"], successor["version"])
            self.assertEqual(proof["obs_upgrade"]["direction"], "upgrade" if old == OLD else "redeploy")
            self.assertIs(proof["obs_upgrade"]["image_rollback_compatible"], False)

    def test_reverse_fixed_tuple_and_unknown_inputs_are_rejected(self):
        for old, target in ((OBS, OLD), (OBS, self.old),
                            (dict(OLD, image=fixture.NEW["image"]), OBS),
                            (dict(OLD, version="0.2.15-klno.2-tps.3"), OBS),
                            (OLD, dict(OBS, version="0.2.15-klno.2-tps.1"))):
            with self.subTest(old=old["revision"], target=target["revision"]), self.assertRaises(deploy.DeployError):
                self.evidence.compatibility(old, target)
        for kind in ("runtime", "migration", "mode", "type", "source", "source_extra",
                     "frontend", "frontend_mode", "frontend_type"):
            with self.subTest(kind=kind):
                self.git("checkout", "-q", "--detach", OBS["revision"])
                if kind.startswith("source"):
                    path = self.root / "deploy/personal-source.json"
                    source = json.loads(path.read_bytes())
                    source["upstream_sha" if kind == "source" else "unexpected"] = "0" * 40
                    path.write_text(json.dumps(source))
                else:
                    relative = "frontend/src/api/client.ts" if kind.startswith("frontend") else (
                        "backend/migrations/242_drop_platform_check_constraints.sql" if kind in ("migration", "mode", "type")
                        else "backend/internal/service/user_service.go")
                    path = self.root / relative
                    if kind.endswith("mode"):
                        path.chmod(0o755)
                    elif kind.endswith("type"):
                        path.unlink(); path.symlink_to("unreviewed")
                    else:
                        path.write_bytes(path.read_bytes() + b"\n# unreviewed\n")
                unknown = dict(OBS, revision=self.commit(kind))
                for old in (OLD, unknown):
                    with self.assertRaises(deploy.DeployError):
                        self.evidence.compatibility(old, unknown)

    def test_raw_legal_build_input_drift_cannot_upgrade_or_redeploy(self):
        documents = ("docs/legal/admin-compliance.en.md", "docs/legal/admin-compliance.zh.md")
        for relative in documents:
            for kind in ("content", "mode", "type"):
                with self.subTest(document=relative, kind=kind):
                    self.git("checkout", "-q", "--detach", OBS["revision"])
                    path = self.root / relative
                    if kind == "mode":
                        path.chmod(0o755)
                    elif kind == "type":
                        path.unlink(); path.symlink_to("unreviewed")
                    else:
                        path.write_bytes(path.read_bytes() + b"\nUnreviewed embedded legal text.\n")
                    unknown = dict(OBS, revision=self.commit("raw legal " + kind))
                    # 两份 ?raw 文档在 frontend 目录之外，仍会进入发布前端。
                    self.assertEqual(self.evidence.runtime_fingerprint(unknown["revision"]), deploy.OBS_NEW_RUNTIME)
                    self.assertEqual(self.evidence.tree_fingerprint(unknown["revision"], ["frontend"]), deploy.OBS_NEW_FRONTEND)
                    for old in (OLD, OBS, unknown):
                        with self.assertRaises(deploy.DeployError):
                            self.evidence.compatibility(old, unknown)

    def test_identical_tree_without_anchor_cannot_upgrade_or_redeploy(self):
        tree = self.git("rev-parse", OBS["revision"] + "^{tree}")
        revision = subprocess.run(["git", "-C", str(self.root), "commit-tree", tree,
                                   "-p", deploy.KIN_NEW_SOURCE["upstream_sha"]],
                                  input=b"same inputs without OBS anchor\n", capture_output=True, check=True).stdout.decode().strip()
        unknown = dict(OBS, revision=revision)
        self.assertEqual(self.evidence.runtime_fingerprint(revision), deploy.OBS_NEW_RUNTIME)
        for old, target in ((OLD, unknown), (unknown, OBS), (unknown, unknown)):
            with self.subTest(old=old["revision"], target=target["revision"]), self.assertRaises(deploy.DeployError):
                self.evidence.compatibility(old, target)


class ObsDeploymentTests(unittest.TestCase):
    setUp = fixture.DeploymentTests.setUp
    execute = fixture.DeploymentTests.execute
    persisted = fixture.DeploymentTests.persisted
    starts = fixture.DeploymentTests.starts

    def prepare(self):
        self.ledger = b"".join(sorted(fixture.kin_old_ledger().splitlines(keepends=True) + [fixture.KIN_LEDGER_ROW]))
        old = dict(OLD, image_id=fixture.OLD_ID)
        self.docker.images[old["image"]] = self.docker.images[fixture.OLD_ID] = fixture.image(old, fixture.OLD_ID)
        self.docker.current = self.docker.config_image = old["image"]
        self.docker.migration_listing = self.ledger
        state = self.base / ".personal-deploy"
        previous = "20261012T180000Z-123456789abc"
        deploy.private_directory(state / "records" / previous)
        deploy.atomic_private(state / "records" / previous / "record.json", deploy.canonical({
            "id": previous, "status": "success", "action": "deploy", "target": old,
            "migration_state": deploy.migration_fingerprint(self.ledger)}))
        deploy.atomic_private(state / "image.env", ("SUB2API_IMAGE=" + old["image"] +
                                                    "\nSUB2API_DEPLOYMENT_ID=" + previous + "\n").encode())
        self.docker.images[OBS["image"]] = self.docker.images[fixture.NEW_ID] = fixture.image(OBS, fixture.NEW_ID)
        self.docker.failure_image = OBS["image"]
        value = fixture.request(OBS, old)
        value["compatibility"] = deploy.GitEvidence(ROOT).compatibility(old, OBS)
        return value, previous

    def test_success_and_redeploy_keep_ledger_and_backup_before_app_only_start(self):
        value, previous = self.prepare()
        result, code = self.execute(value)
        self.assertEqual((code, result["status"]), (0, "success"))
        self.assertEqual(self.persisted(result)["migration_state"], deploy.migration_fingerprint(self.ledger))
        commands = [args for args, _ in self.docker.commands]
        self.assertLess(next(i for i, args in enumerate(commands) if "pg_dump" in args[-1]),
                        next(i for i, args in enumerate(commands) if "--no-deps" in args))
        old = deploy.Deployment(self.base, self.docker).running()
        retry = fixture.request(OBS, old)
        retry["compatibility"] = deploy.GitEvidence(ROOT).compatibility(old, OBS)
        result, code = self.execute(retry)
        self.assertEqual((code, result["status"]), (0, "success"))
        self.assertEqual(self.persisted(result)["compatibility"]["obs_upgrade"]["direction"], "redeploy")
        self.assertEqual(self.docker.migration_listing, self.ledger)
        self.assertEqual(len(self.starts()), 2)

    def test_rehashed_proof_disguise_or_tampering_stops_before_backup_pull_start(self):
        value, _ = self.prepare()
        mutations = {
            "checksum": lambda p: p["obs_upgrade"]["migrations"].update({"242_drop_platform_check_constraints.sql": "0" * 64}),
            "runtime": lambda p: p["obs_upgrade"].update(new_runtime_sha256="0" * 64),
            "frontend": lambda p: p["obs_upgrade"].update(new_frontend_sha256="0" * 64),
            "frontend_paths": lambda p: p["obs_upgrade"]["frontend_changed_paths"].pop(),
            "raw": lambda p: p["obs_upgrade"].update(raw_sha256="0" * 64),
            "raw_paths": lambda p: p["obs_upgrade"]["raw_paths"].pop(),
            "source": lambda p: p["upstream"].update(upstream_sha="0" * 40),
            "anchor": lambda p: p["obs_upgrade"].update(new_runtime_revision="0" * 40),
            "direction": lambda p: p["obs_upgrade"].update(direction="redeploy"),
            "rollback": lambda p: p["obs_upgrade"].update(image_rollback_compatible=True),
            "revision": lambda p: p.update(new_revision="0" * 40),
            "changed": lambda p: p["changed_paths"].pop(),
            "zero": lambda p: (p.pop("obs_upgrade"), p.update(changed_paths=[])),
            "kin": lambda p: (p.pop("obs_upgrade"), p.update(changed_paths=[], kin_upgrade=deploy.approved_kin_upgrade("redeploy"))),
        }
        for kind, mutate in mutations.items():
            with self.subTest(kind=kind):
                bad = copy.deepcopy(value); mutate(bad["compatibility"])
                bad["compatibility"]["sha256"] = deploy.sha(deploy.canonical({k: v for k, v in bad["compatibility"].items() if k != "sha256"}))
                self.docker.commands.clear()
                result, code = self.execute(bad)
                self.assertEqual((code, result["error_code"]), (1, "E_COMPATIBILITY"))
                self.assertNotIn("backups", self.persisted(result))
                self.assertFalse(any(args[:2] == ["docker", "pull"] for args, _ in self.docker.commands))
                self.assertEqual(self.starts(), [])

    def test_health_or_ledger_failure_never_restores_429(self):
        for kind in ("health", "ledger", "forged_prior_ledger"):
            with self.subTest(kind=kind):
                self.setUp(); value, previous = self.prepare()
                if kind == "health":
                    self.docker.health_failure = True
                elif kind == "ledger":
                    self.docker.migrations_on_start = self.ledger + b"unknown.sql|" + b"0" * 64 + b"\n"
                else:
                    self.docker.migration_listing = self.ledger.replace(fixture.KIN_LEDGER_ROW, b"")
                    path = self.base / ".personal-deploy/records" / previous / "record.json"
                    prior = json.loads(path.read_bytes()); prior["migration_state"] = deploy.migration_fingerprint(self.docker.migration_listing)
                    deploy.atomic_private(path, deploy.canonical(prior))
                result, code = self.execute(value)
                self.assertEqual(code, 1)
                self.assertEqual(deploy.Deployment(self.base, self.docker).selected()["record_id"], previous)
                if kind == "forged_prior_ledger":
                    self.assertEqual(self.starts(), [])
                    self.assertNotIn("backups", self.persisted(result))
                else:
                    self.assertEqual((result["rollback_status"], result["rollback_error"]), ("blocked", "E_COMPATIBILITY"))
                    self.assertEqual(len(self.starts()), 1)


class ObsMaintenanceTests(unittest.TestCase):
    setUp = maintenance.GateTests.setUp
    phase = maintenance.GateTests.phase

    def prepare(self):
        self.ledger = b"".join(sorted(fixture.kin_old_ledger().splitlines(keepends=True) + [fixture.KIN_LEDGER_ROW]))
        target = dict(OBS, image_id=fixture.NEW_ID)
        self.deployed = {"schema": 1, "id": self.id, "action": "deploy", "status": "success", "exit_code": 0,
                         "health_verified": True, "old": OLD, "target": target,
                         "compatibility": deploy.GitEvidence(ROOT).compatibility(OLD, OBS),
                         "observed_running": dict(target, health="healthy", running=True),
                         "migration_state": deploy.migration_fingerprint(self.ledger)}
        self.persist()
        self.image = target["image"]
        self.g.atomic(self.g.BASE / ".personal-deploy/image.env",
                      ("SUB2API_IMAGE=" + self.image + "\nSUB2API_DEPLOYMENT_ID=" + self.id + "\n").encode())
        self.app["Image"] = fixture.NEW_ID
        self.app["Config"]["Image"] = self.image
        self.app["Config"]["Labels"] = {"org.opencontainers.image." + key: target[key] for key in ("revision", "version", "source")}
        self.app["Config"]["Labels"].update({"com.docker.compose.project": self.g.CONFIG.project,
                                             "com.docker.compose.service": self.g.CONFIG.app_service})
        self.action("freeze")

    def persist(self):
        self.g.atomic(self.g.BASE / ".personal-deploy/records" / self.id / "record.json", deploy.canonical(self.deployed))

    def action(self, name, revision=None, record=None):
        with contextlib.redirect_stdout(io.StringIO()):
            self.g.main(name, revision, record)

    def test_explicit_successful_obs_resume_preserves_old_site_record_and_marker_identity(self):
        self.prepare(); self.action("resume", OBS["revision"], self.id)
        record = json.loads((self.g.DIRECTORY / "record.json").read_bytes())
        self.assertEqual(record["source_sha"], "42907bcab278f93db48d7e1b39b28e28f664569e")
        self.assertEqual((record["accepted_source_sha"], record["accepted_deployment_id"]), (OBS["revision"], self.id))
        self.assertEqual(self.phase(), "open"); self.assertFalse(self.g.FLAG.exists())
        self.action("freeze"); self.assertEqual(self.g.FLAG.read_bytes(), b"REL-002 42907bcab278\n")

    def test_wrong_sha_record_proof_selection_health_or_ledger_keeps_flag(self):
        for kind in ("sha", "record", "failed", "id", "proof", "selection", "oci", "image_id", "healthy", "ledger",
                     "selection_mode", "record_mode", "record_directory", "flag_mode", "no_explicit_target"):
            with self.subTest(kind=kind):
                self.setUp(); self.prepare(); revision, record = OBS["revision"], self.id
                if kind == "sha": revision = "0" * 40
                elif kind == "record": record = "20261012T180000Z-000000000000"
                elif kind == "failed": self.deployed["status"] = "failed"
                elif kind == "id": self.deployed["id"] = "20261012T180000Z-000000000000"
                elif kind == "proof": self.deployed["compatibility"]["obs_upgrade"]["image_rollback_compatible"] = True
                elif kind == "selection":
                    self.g.atomic(self.g.BASE / ".personal-deploy/image.env", b"SUB2API_IMAGE=unknown\nSUB2API_DEPLOYMENT_ID=" + self.id.encode() + b"\n")
                elif kind == "oci": self.app["Config"]["Labels"]["org.opencontainers.image.revision"] = "0" * 40
                elif kind == "image_id": self.app["Image"] = fixture.OLD_ID
                elif kind == "healthy": self.app["State"]["Health"]["Status"] = "unhealthy"
                elif kind == "ledger": self.ledger += b"unknown.sql|" + b"0" * 64 + b"\n"
                elif kind == "no_explicit_target": revision = record = None
                self.persist()
                if kind == "selection_mode": (self.g.BASE / ".personal-deploy/image.env").chmod(0o644)
                elif kind == "record_mode": (self.g.BASE / ".personal-deploy/records" / self.id / "record.json").chmod(0o644)
                elif kind == "flag_mode": self.g.FLAG.chmod(0o600)
                elif kind == "record_directory":
                    directory = self.g.BASE / ".personal-deploy/records" / self.id
                    moved = directory.with_name("unbound-record"); directory.rename(moved); directory.symlink_to(moved, target_is_directory=True)
                with self.assertRaises((RuntimeError, deploy.DeployError)):
                    self.action("resume", revision, record)
                self.assertEqual(self.g.FLAG.read_bytes(), b"REL-002 42907bcab278\n")
                self.assertEqual(self.phase(), "closed")

    def test_drift_during_health_is_not_overwritten_or_opened(self):
        for kind in ("flag", "site", "selection", "record"):
            with self.subTest(kind=kind):
                self.setUp(); self.prepare(); run = self.g.run
                def drift(args, **options):
                    output = run(args, **options)
                    if args[0] == "curl":
                        path = {"flag": self.g.FLAG, "site": self.g.SITE,
                                "selection": self.g.BASE / ".personal-deploy/image.env",
                                "record": self.g.BASE / ".personal-deploy/records" / self.id / "record.json"}[kind]
                        path.write_bytes(b"external drift\n")
                    return output
                self.g.run = drift
                with self.assertRaises(RuntimeError): self.action("resume", OBS["revision"], self.id)
                self.assertTrue(self.g.FLAG.exists()); self.assertEqual(self.phase(), "closed")
                if kind == "flag": self.assertEqual(self.g.FLAG.read_bytes(), b"external drift\n")


    def test_metadata_drift_during_health_keeps_private_binding_closed(self):
        for kind in ("selection_mode", "record_mode", "record_directory"):
            with self.subTest(kind=kind):
                self.setUp(); self.prepare(); run = self.g.run
                def drift(args, **options):
                    output = run(args, **options)
                    if args[0] == "curl":
                        path = self.g.BASE / ".personal-deploy/image.env" if kind == "selection_mode" else (
                            self.g.BASE / ".personal-deploy/records" / self.id / "record.json")
                        if kind == "record_directory":
                            directory = path.parent
                            moved = directory.with_name("metadata-drift-record")
                            directory.rename(moved); directory.symlink_to(moved, target_is_directory=True)
                        else:
                            path.chmod(0o666)
                    return output
                self.g.run = drift
                with self.assertRaises(RuntimeError): self.action("resume", OBS["revision"], self.id)
                self.assertTrue(self.g.FLAG.exists()); self.assertEqual(self.phase(), "closed")


class ObsBundleTests(unittest.TestCase):
    setUp = architecture.BundleTests.setUp
    git = architecture.BundleTests.git

    def test_maintenance_uses_complete_bootstrap_and_rejects_drift_before_import(self):
        marker = self.root / "untrusted-import"
        (self.directory / "maintenance_hostdzire.py").write_text("from pathlib import Path\nPath(" + repr(str(marker)) + ").touch()\n")
        result = subprocess.run(shlex.split(self.bundle.maintenance_bootstrap("freeze")), capture_output=True)
        self.assertEqual((result.returncode, json.loads(result.stdout)["error_code"]), (2, "E_TOOL_TRUST"))
        self.assertFalse(marker.exists())

    def test_maintenance_argument_binding_and_no_ssh_for_bad_input(self):
        from deploy_local import local_maintenance
        for action, revision, record in (("resume", None, None), ("freeze", OBS["revision"], "20261012T180000Z-123456789abc")):
            args = types.SimpleNamespace(maintenance=action, target_revision=revision, deployment_record=record,
                                         target=None, version=None, rollback=None, dry_run=False)
            with mock.patch("deploy_transport.subprocess.run", side_effect=AssertionError("触及 SSH")):
                with self.assertRaises(deploy.DeployError): local_maintenance(args, self.bundle, deploy.load_config())

    def test_verified_bootstrap_freezes_and_resumes_existing_record_with_explicit_target(self):
        gate = ObsMaintenanceTests("test_explicit_successful_obs_resume_preserves_old_site_record_and_marker_identity")
        gate.setUp(); self.addCleanup(gate.doCleanups); gate.prepare()
        config_path = self.directory / "hostdzire.json"
        config = json.loads(config_path.read_bytes())
        config["base"] = str(gate.g.BASE)
        config["maintenance"].update(site=str(gate.g.SITE), allowed_site_parents=[str(gate.g.SITE.parent)],
                                     flag_directory=str(gate.g.FLAG_DIRECTORY),
                                     record_directory=str(gate.g.DIRECTORY.relative_to(gate.g.BASE)))
        config_path.write_text(json.dumps(config))
        self.git("add", "deploy/personal/hostdzire.json"); self.git("commit", "-qm", "isolated maintenance fixture")
        revision = self.git("rev-parse", "HEAD").decode().strip()
        self.git("update-ref", "refs/remotes/origin/main", revision)
        bundle = ToolBundle.from_git(self.root, self.directory, revision)
        app_file, ledger_file = self.root / "app.json", self.root / "ledger.txt"
        app_file.write_text(json.dumps([gate.app])); ledger_file.write_bytes(gate.ledger)
        commands = self.root / "commands"; commands.mkdir()
        for command in ("docker", "curl"):
            script = commands / command
            script.write_text("#!" + sys.executable + "\nimport pathlib,sys\n" +
                              ("if sys.argv[1]=='inspect':sys.stdout.buffer.write(pathlib.Path(" + repr(str(app_file)) + ").read_bytes())\n"
                               "elif sys.argv[1]=='exec':sys.stdout.buffer.write(pathlib.Path(" + repr(str(ledger_file)) + ").read_bytes())\n"
                               "else:sys.exit(9)\n" if command == "docker" else "print('{\"status\":\"ok\"}')\n"))
            script.chmod(0o755)
        environment = dict(os.environ, PATH=str(commands) + os.pathsep + os.environ["PATH"])
        for action in ("freeze", "resume"):
            command = bundle.maintenance_bootstrap(action, OBS["revision"] if action == "resume" else None,
                                                   gate.id if action == "resume" else None)
            result = subprocess.run(shlex.split(command), capture_output=True, env=environment)
            self.assertEqual(result.returncode, 0, result.stdout + result.stderr)
            response = json.loads(result.stdout)
            self.assertEqual(response["phase"], "closed" if action == "freeze" else "open")
        self.assertFalse(gate.g.FLAG.exists())
        self.assertEqual(json.loads((gate.g.DIRECTORY / "record.json").read_bytes())["accepted_source_sha"], OBS["revision"])


if __name__ == "__main__":
    unittest.main()
