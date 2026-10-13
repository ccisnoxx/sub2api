"""远端唯一生命周期 owner：锁、记录、迁移前缀及选择提交/失败恢复。"""
import datetime
import json
import os
from pathlib import Path
import re
import stat
import tempfile
import uuid
from deploy_common import *
from deploy_compatibility import *
from deploy_database import *
from deploy_compose import ComposeSite, adapted_configuration
from deploy_config import load_config
from deploy_images import ImageVerifier
from deploy_health import HealthChecks
from deploy_process import Runner
from deploy_trust import verify_request_bundle

class Deployment:
    def __init__(self, base=None, runner=None, config=None, trusted_revision=None):
        self.config = config or load_config()
        self.trusted_revision = trusted_revision
        self.base = Path(base) if base is not None else Path(self.config.base)
        self.compose_file = self.base / self.config.compose_file
        self.env_file = self.base / self.config.env_file
        self.state = self.base / self.config.state_directory
        self.selection = self.state / "image.env"
        self.runner = runner or Runner(self.base)
        self.record = None
        self.directory = None
        self.config_before = None
        self.config_after = None
        self.env_before = None
        self.selection_before = None
        self.migration_before = None
        self.cumulative_baseline = None
        self.cumulative_prefix_high_water = 0
        self.started = False
        self.adapted = False
        self.images = ImageVerifier(self.runner, self.config)
        self.site = ComposeSite(self.base, self.runner, self.config)
        self.database = DatabaseOperations(self.runner, self.config)
        self.healthchecks = HealthChecks(self.runner, self.config, self.images, self.database)

    def inspect(self, reference, image=False):
        return self.images.inspect(reference, image)

    def metadata(self, image, expected=None):
        return self.images.metadata(image, expected)

    def running(self):
        return self.images.running()

    def selected(self):
        if not self.selection.exists():
            require(not self.selection.is_symlink(), "E_PATH", "镜像选择文件不能是符号链接")
            return None
        require(stat.S_IMODE(self.selection.stat().st_mode) == 0o600, "E_PERMISSION", "镜像选择文件权限必须为 600")
        text = read_regular(self.selection).decode()
        match = re.fullmatch(r"SUB2API_IMAGE=(\S+)\nSUB2API_DEPLOYMENT_ID=([^\n]+)\n", text)
        require(match and RECORD_RE.fullmatch(match[2]), "E_STATE", "镜像选择文件格式无效")
        return {"image": match[1], "record_id": match[2]}

    def read_record(self, record_id):
        require(RECORD_RE.fullmatch(record_id), "E_INPUT", "部署记录 ID 无效")
        return json.loads(read_regular(self.state / "records" / record_id / "record.json"))

    def probe(self, record_id=None):
        value = {"running": self.running(), "selection": self.selected()}
        if record_id:
            previous = self.read_record(record_id)
            value["rollback_record"] = {key: previous.get(key) for key in
                                        ("id", "action", "status", "old", "target", "compatibility", "configuration")}
        return value

    def compose(self, image, *arguments, sensitive=False):
        return self.site.compose(self.directory, image, *arguments, sensitive=sensitive)

    def configuration(self, image):
        return self.site.configuration(self.compose, image)

    def dependencies(self, image, expected=None):
        return self.healthchecks.dependencies(self.compose, image, expected)

    def save(self, status=None, **values):
        if status:
            self.record["status"] = status
        self.record.update(values)
        self.record["updated_at"] = now()
        atomic_private(self.directory / "record.json", canonical(self.record) + b"\n")

    def migration_listing(self, postgres_id):
        return self.database.migration_listing(postgres_id)

    def migration_state(self, postgres_id):
        return self.database.migration_state(postgres_id)

    def guard_configuration(self):
        require(sha(read_regular(self.compose_file)) == sha(self.config_after if self.adapted else self.config_before) and
                sha(read_regular(self.env_file)) == sha(self.env_before), "E_DRIFT", "Compose 或 .env 已被外部修改，停止覆盖和回滚")
        current = read_regular(self.selection) if self.selection.exists() else None
        require(current == self.selection_before, "E_DRIFT", "镜像选择已被外部修改，停止覆盖和回滚")

    def backup(self, old, dependencies):
        backups = self.database.backup(self.directory, self.config_before, self.env_before, self.selection_before, dependencies)
        self.save("backed_up", backups=backups)

    def adapt_configuration(self):
        self.config_after = adapted_configuration(self.config_before, self.config.app_service)

    def health(self, expected, dependencies, expected_migrations=None):
        return self.healthchecks.health(self.compose, self.guard_configuration, expected, dependencies, expected_migrations or self.record["migration_state"])

    def start(self, target):
        self.compose(target["image"], "up", "-d", "--no-deps", "--wait", "--wait-timeout", str(self.config.wait_timeout), self.config.app_service)

    def validate_proof(self, request, old, target):
        proof = request.get("compatibility", {})
        validate_compatibility_proof(proof, old["revision"], target["revision"], old["version"], target["version"])
        if proof.get("kin_upgrade", {}).get("direction") == "upgrade":
            require(old["image"] == KIN_OLD_IMAGE, "E_COMPATIBILITY", "新 KIN 必须绑定固定旧发布 digest")
        require_obs_old_image(proof, old)
        if request["action"] == "rollback":
            require_image_rollback(proof)
        require(request["action"] != "deploy" or proof.get("security_patch", {}).get("direction") != "rollback",
                "E_COMPATIBILITY", "反向安全补丁只允许绑定成功记录的显式回滚，不能用普通部署切回")
        require(request["action"] != "deploy" or proof.get("cumulative_upgrade", {}).get("direction") != "rollback",
                "E_COMPATIBILITY", "累计应用反向切回只允许绑定成功记录的显式回滚")

    def observe_cumulative_prefix(self, listing):
        # 同一部署 owner 保留已验证的最高提交前缀，覆盖健康失败及 replacement 两侧。
        prefix = cumulative_ledger_prefix(self.cumulative_baseline, listing)
        require(prefix >= self.cumulative_prefix_high_water, "E_MIGRATION",
                "固定迁移前缀不能少于本次已观察到的提交")
        self.cumulative_prefix_high_water = prefix
        return prefix

    def cumulative_recovery_state(self, request, old, dependencies):
        # Compose replacement 可能在等待失败后仍运行；只接管本次已绑定的两个镜像。
        observed = self.running()
        fields = ("image", "revision", "version", "source", "image_id")
        require(self.inspect(self.config.app_container).get("Config", {}).get("Image") == observed["image"] and
                any(all(observed.get(key) == expected.get(key) for key in fields)
                    for expected in (old, self.record["target"])),
                "E_STALE", "失败后的实际运行镜像不属于本次旧、新固定镜像，禁止接管")
        listing = self.migration_listing(dependencies["postgres"])
        if request["compatibility"]["cumulative_upgrade"]["direction"] == "upgrade":
            self.observe_cumulative_prefix(listing)
        else:
            require(listing == self.migration_before, "E_MIGRATION", "显式回滚失败后迁移账本发生变化")
        return migration_fingerprint(listing)

    def pull_target(self, target):
        return self.images.pull_target(target, lambda resolved: self.save("resolved", target=resolved))

    def rollback_binding(self, request, old, target, *, verify_env=True):
        previous = self.read_record(request["rollback_id"])
        selected = self.selected()
        require(previous.get("status") == "success" and previous.get("action") == "deploy" and
                selected == {"image": old["image"], "record_id": request["rollback_id"]}, "E_STALE_ROLLBACK", "回滚记录不是当前成功部署，禁止过期回滚")
        require(all(previous["target"].get(key) == old[key] and previous["old"].get(key) == target[key]
                    for key in ("image", "revision", "version", "source")), "E_STALE_ROLLBACK", "当前镜像或回滚目标与记录不符")
        validate_compatibility_proof(previous.get("compatibility", {}), target["revision"], old["revision"],
                                     target["version"], old["version"])
        require_image_rollback(previous["compatibility"])
        config = previous.get("configuration", {})
        require(config.get("after_sha256") == sha(self.config_before) and (not verify_env or config.get("env_sha256") == sha(self.env_before)),
                "E_STALE_ROLLBACK", "成功部署后的运行配置已变化，禁止镜像回滚")
        return previous

    def diagnostics(self):
        try:
            self.runner.run(["docker", "logs", "--tail", "100", self.config.app_container], stdout_path=self.directory / "application.log", timeout=20)
        except Exception:
            # 诊断失败可记录，但不能掩盖原始部署失败。
            self.record["diagnostic_error"] = "E_DIAGNOSTIC"
        try:
            self.record["observed_running"] = self.running()
        except Exception:
            self.record["observed_running"] = {"state": "unknown"}

    def execute(self, request):
        import fcntl
        private_directory(self.state)
        lock = os.open(self.state / "deployment.lock", os.O_CREAT | os.O_RDWR | os.O_NOFOLLOW, 0o600)
        try:
            os.fchmod(lock, 0o600)
            try:
                fcntl.flock(lock, fcntl.LOCK_EX | fcntl.LOCK_NB)
            except BlockingIOError as error:
                raise DeployError("E_LOCKED", "已有部署持有远端锁") from error
            return self._locked(request)
        finally:
            os.close(lock)

    def _locked(self, request):
        record_id = datetime.datetime.now(datetime.timezone.utc).strftime("%Y%m%dT%H%M%SZ-") + uuid.uuid4().hex[:12]
        self.directory = self.state / "records" / record_id
        private_directory(self.directory.parent)
        private_directory(self.directory)
        self.runner.log = self.directory / "commands.log"
        atomic_private(self.runner.log, b"")
        self.record = {"id": record_id, "schema": 1, "action": request.get("action"), "started_at": now(),
                       "tool_revision": request.get("tool_revision"), "tool_sha256": request.get("tool_sha256"),
                       "tool_bundle": request.get("tool_bundle"),
                       "compatibility": request.get("compatibility"), "rollback_of": request.get("rollback_id")}
        self.save("preflight")
        old = None
        dependencies = None
        proof_valid = False
        try:
            old, target, baseline, dependencies, expected_migrations, cumulative, kin, kin_baseline = self._preflight(request)
            proof_valid = True
            self.backup(old, dependencies)
            resolved = self.pull_target(target)
            self.save("pulled", target=resolved)
            self.guard_configuration()
            # 先取得旧镜像固定引用；旧标签没有不可变本地映射时禁止重建。
            self.metadata(self.inspect(old["image"], image=True), expected=old)
            if cumulative or kin or "obs_upgrade" in request["compatibility"]:
                require(self.migration_listing(dependencies["postgres"]) == self.migration_before,
                        "E_MIGRATION", "新应用启动前迁移账本已经变化")
            atomic_private(self.compose_file, self.config_after)
            self.adapted = True
            candidate = self.configuration(resolved["image"])
            require(candidate["services"][self.config.app_service]["image"] == resolved["image"], "E_CONFIG", "Compose 未选择目标 digest")
            baseline["services"][self.config.app_service]["image"] = resolved["image"]
            require(candidate == baseline, "E_CONFIG", "适配改变了应用镜像以外的 Compose 配置")
            self.guard_configuration()
            self.save("starting")
            self.started = True
            self.start(resolved)
            if cumulative and cumulative["direction"] == "upgrade":
                migrated = self.migration_listing(dependencies["postgres"])
                require(self.observe_cumulative_prefix(migrated) == len(CUMULATIVE_MIGRATIONS),
                        "E_MIGRATION", "健康终态必须完成全部固定 251–254 迁移")
                expected_migrations = migration_fingerprint(migrated)
            if kin:
                migrated = self.migration_listing(dependencies["postgres"])
                require(kin_ledger_baseline(migrated, True) == kin_baseline,
                        "E_MIGRATION", "新 KIN 健康终态必须保留原账本并完成唯一固定 242")
                expected_migrations = migration_fingerprint(migrated)
            actual = self.health(resolved, dependencies, expected_migrations)
            self.guard_configuration()
            self.save("healthy_pending_commit", health_verified=True, observed_running=actual,
                      migration_state=expected_migrations)
            atomic_private(self.selection, ("SUB2API_IMAGE=" + resolved["image"] + "\nSUB2API_DEPLOYMENT_ID=" + record_id + "\n").encode())
            self.save("success", completed_at=now(), exit_code=0)
            return self.public_result(), 0
        except Exception as error:
            code = getattr(error, "code", "E_INTERNAL")
            # replace 已生效而目录 fsync 失败时也须识别本次适配；仅接受已知的完整内容。
            if self.config_before is not None and self.config_after is not None:
                try:
                    observed_config = read_regular(self.compose_file)
                    if observed_config == self.config_after and self.config_after != self.config_before:
                        self.adapted = True
                    elif observed_config == self.config_before:
                        self.adapted = False
                except (DeployError, OSError):
                    self.record["configuration_observation_error"] = "E_PATH"
            # 一旦提交选择，不能猜测是否已持久化；保留可核对记录并停止自动覆盖。
            try:
                committed = self.selection.exists() and self.selection_before != read_regular(self.selection)
            except (DeployError, OSError):
                committed = True
                self.record["selection_observation_error"] = "E_PATH"
            if self.started and proof_valid and not committed:
                restoring = False
                try:
                    require_image_rollback(request["compatibility"])
                    self.guard_configuration()
                    self.dependencies(old["image"], dependencies)
                    cumulative = request["compatibility"].get("cumulative_upgrade")
                    recovery_migrations = (self.cumulative_recovery_state(request, old, dependencies) if cumulative
                                           else self.record["migration_state"])
                    if not cumulative:
                        require(self.migration_state(dependencies["postgres"]) == recovery_migrations,
                                "E_MIGRATION", "失败后迁移状态已变化，禁止自动镜像回滚")
                    self.save("rolling_back", error_code=code)
                    restored = self.metadata(self.inspect(old["image"], image=True), expected=old)
                    restoring = True
                    self.start(restored)
                    if cumulative:
                        # 停止新应用的 replacement 期间仍可能完成下一笔固定事务；再次核对前缀。
                        recovery_migrations = self.cumulative_recovery_state(request, old, dependencies)
                    self.health(restored, dependencies, recovery_migrations)
                    self.guard_configuration()
                    atomic_private(self.compose_file, self.config_before)
                    self.adapted = False
                    self.record["rollback_status"] = "success"
                    if cumulative:
                        self.record["recovery_migration_state"] = recovery_migrations
                except Exception as rollback_error:
                    blocked = ("forward_upgrade" in request["compatibility"] or "kin_upgrade" in request["compatibility"] or
                               "obs_upgrade" in request["compatibility"] or
                               ("cumulative_upgrade" in request["compatibility"] and not restoring))
                    self.record["rollback_status"] = "blocked" if blocked else "failed"
                    self.record["rollback_error"] = rollback_error.code if isinstance(rollback_error, DeployError) else "E_INTERNAL"
            elif self.adapted and not committed:
                try:
                    self.guard_configuration()
                    atomic_private(self.compose_file, self.config_before)
                    self.adapted = False
                except Exception as restore_error:
                    self.record["configuration_restore_error"] = restore_error.code if isinstance(restore_error, DeployError) else "E_INTERNAL"
            self.diagnostics()
            self.save("failed", error_code=code, exit_code=1, completed_at=now(), selection_committed=committed)
            return self.public_result(), 1

    def dry_run(self, request):
        """独立只读路径：现有共享锁可观察现场，临时命令文件只留在 /tmp。"""
        import fcntl
        for path in (self.state, self.state / "records"):
            require(path.is_dir() and not path.is_symlink() and
                    stat.S_IMODE(path.stat().st_mode) == 0o700 and path.stat().st_uid == os.geteuid(),
                    "E_PERMISSION", "dry-run 要求既有私密部署状态目录；不会创建或修复状态")
        lock_path = self.state / "deployment.lock"
        require(lock_path.is_file() and not lock_path.is_symlink() and
                stat.S_IMODE(lock_path.stat().st_mode) == 0o600 and lock_path.stat().st_uid == os.geteuid(),
                "E_PERMISSION", "dry-run 要求既有私密共享锁；不会创建或修复锁")
        lock = os.open(lock_path, os.O_RDONLY | os.O_NOFOLLOW)
        try:
            try:
                fcntl.flock(lock, fcntl.LOCK_SH | fcntl.LOCK_NB)
            except BlockingIOError as error:
                raise DeployError("E_LOCKED", "已有部署持有远端锁") from error
            with tempfile.TemporaryDirectory(prefix="sub2api-preflight-", dir="/tmp") as scratch:
                self.directory = Path(scratch)
                self.runner.log = None
                old, target, _, dependencies, migrations, _, _, _ = self._preflight(request, persist=False)
                # 只 inspect 现有目标；缺少缓存也失败关闭，不能偷偷 pull。
                try:
                    inspected = self.inspect(target["input"], image=True)
                except Exception as error:
                    raise DeployError("E_IMAGE_UNAVAILABLE", "dry-run 目标镜像未在本机缓存中可观察；不会拉取") from error
                resolved = self.metadata(inspected, expected=target if "image" in target else None)
                require(all(resolved[key] == target[key] for key in ("revision", "version", "source")),
                        "E_LABEL", "可观察目标镜像与 Git 发布合同不一致")
                self.healthchecks.loopback_health()
                require(self.running() == old,
                        "E_STALE", "只读预检期间运行状态发生漂移")
                current_selection = read_regular(self.selection) if self.selection.exists() else None
                require(read_regular(self.compose_file) == self.config_before and current_selection == self.selection_before,
                        "E_DRIFT", "只读预检期间 Compose 或镜像选择发生漂移")
                self.dependencies(old["image"], dependencies)
                require(self.migration_state(dependencies["postgres"]) == migrations, "E_MIGRATION", "只读预检期间迁移账本发生变化")
                return {"status": "dry-run", "action": request["action"], "will_apply": False,
                        "running": old, "target": resolved, "dependencies": dependencies,
                        "migration_state": migrations, "compatibility": request["compatibility"],
                        "configuration": {"compose_sha256": sha(self.config_before),
                                          "adapted_compose_sha256": sha(self.config_after),
                                          "private_environment_bytes": "not_read"},
                        "tool_revision": request["tool_revision"], "tool_bundle": request["tool_bundle"],
                        "message": "只读预检完成；不会拉取、备份、写操作记录/选择/Compose、启动容器或执行迁移"}, 0
        finally:
            os.close(lock)
            self.directory = None

    def _preflight(self, request, *, persist=True):
        require(request.get("schema") == 1 and request.get("action") in ("deploy", "rollback") and
                SHA_RE.fullmatch(request.get("tool_revision", "")) and verify_request_bundle(request, self.trusted_revision),
                "E_REQUEST", "部署请求或工具来源校验失败")
        target = request["target"]
        if request["action"] == "deploy":
            parsed = parse_target(target["input"], target["version"])
            require(parsed["input"] == target["input"] and parsed["version"] == target["version"] and
                    target.get("source") == SOURCE and SHA_RE.fullmatch(target.get("revision", "")),
                    "E_INPUT", "目标发布来源不完整")
            if "image" in parsed:
                require(parsed["image"] == target.get("image"), "E_INPUT", "目标 digest 不一致")
        else:
            validate_metadata(target, allow_legacy=True)
            require(target.get("input") == target["image"], "E_INPUT", "回滚必须使用记录的固定 digest")
        require(self.runner.run(["uname", "-sm"]).strip() == b"Linux x86_64", "E_PLATFORM", "服务器必须为 Linux x86_64")
        help_text = self.runner.run(["docker", "compose", "up", "--help"])
        require(b"--wait" in help_text and b"--wait-timeout" in help_text, "E_COMPOSE", "Compose 不支持所需等待功能")
        self.config_before = read_regular(self.compose_file)
        self.env_before = read_regular(self.env_file) if persist else None
        require(self.env_file.is_file() and not self.env_file.is_symlink(), "E_PATH", "必要环境文件缺失或为符号链接")
        self.selection_before = read_regular(self.selection) if self.selection.exists() else None
        old = self.running()
        require(old["health"] == "healthy" and old["running"], "E_HEALTH", "旧应用不是健康运行状态")
        require(all(old[key] == request["expected_old"].get(key) for key in ("image", "revision", "version", "source", "image_id")),
                "E_STALE", "预检后运行镜像已变化，停止部署")
        selected = self.selected()
        previous = None
        if selected:
            previous = self.read_record(selected["record_id"])
            require(selected["image"] == old["image"] and previous.get("status") == "success" and
                    all(previous.get("target", {}).get(key) == old[key] for key in ("image", "revision", "version", "source")),
                    "E_STATE", "当前镜像选择、成功记录与运行镜像不一致，需要人工恢复")
        else:
            require(all(old[key] == LEGACY[key] for key in ("image", "revision", "version", "source")),
                    "E_STATE", "个人镜像没有已提交的选择与成功记录，需要人工核对中断状态")
        self.validate_proof(request, old, target)
        if persist:
            self.save("preflight", old=old, target=target)
        if request["action"] == "rollback":
            self.rollback_binding(request, old, target, verify_env=persist)
        self.adapt_configuration()
        baseline = self.configuration(old["image"])
        dependencies = self.dependencies(old["image"])
        self.migration_before = self.migration_listing(dependencies["postgres"])
        migrations = migration_fingerprint(self.migration_before)
        require(previous is None or previous.get("migration_state") == migrations,
                "E_MIGRATION", "当前迁移状态与成功记录不一致，禁止部署或镜像回滚")
        expected_migrations = migrations
        cumulative = request["compatibility"].get("cumulative_upgrade")
        kin = request["compatibility"].get("kin_upgrade")
        if "obs_upgrade" in request["compatibility"]:
            kin_ledger_baseline(self.migration_before, True)
        if kin:
            kin_baseline = kin_ledger_baseline(self.migration_before, kin["direction"] == "redeploy")
            if persist:
                atomic_private(self.directory / "migration-ledger-before.txt", self.migration_before)
                self.save("preflight", migration_state_before=migration_fingerprint(kin_baseline))
        if cumulative:
            if cumulative["direction"] == "upgrade":
                if any(line.split(b"|", 1)[0].decode() in CUMULATIVE_MIGRATIONS
                       for line in self.migration_before.splitlines()):
                    # 只接受本工具已成功回滚保留的完整扩展账本；失败后的部分账本不能被自动接管。
                    prior_proof = previous.get("compatibility", {})
                    require(previous.get("action") == "rollback" and
                            prior_proof.get("cumulative_upgrade", {}).get("direction") == "rollback",
                            "E_MIGRATION", "旧运行账本已含新增迁移，但没有对应成功回滚记录")
                    validate_compatibility_proof(prior_proof, previous["old"]["revision"], old["revision"],
                                                 previous["old"]["version"], old["version"])
                    self.cumulative_baseline = cumulative_ledger_baseline(self.migration_before)
                    require(previous.get("migration_state_before") == migration_fingerprint(self.cumulative_baseline),
                            "E_MIGRATION", "再次升级的原账本与成功回滚记录不一致")
                else:
                    self.cumulative_baseline = self.migration_before
                self.observe_cumulative_prefix(self.migration_before)
            else:
                self.cumulative_baseline = cumulative_ledger_baseline(self.migration_before)
                require(previous.get("migration_state_before") == migration_fingerprint(self.cumulative_baseline),
                        "E_MIGRATION", "回滚账本的原有行与累计升级成功记录不一致")
            if persist:
                atomic_private(self.directory / "migration-ledger-before.txt", self.migration_before)
                self.save("preflight", migration_state_before=migration_fingerprint(self.cumulative_baseline))
        if persist:
            self.save("preflight", configuration={"before_sha256": sha(self.config_before), "after_sha256": sha(self.config_after),
                                                  "env_sha256": sha(self.env_before)}, dependencies=dependencies,
                      migration_state=migrations)
        return old, target, baseline, dependencies, expected_migrations, cumulative, kin, (kin_baseline if kin else None)

    def public_result(self):
        return {key: self.record[key] for key in ("id", "action", "status", "error_code", "exit_code", "old", "target",
                                                "observed_running", "rollback_status", "rollback_error", "configuration_restore_error",
                                                "selection_committed") if key in self.record}
