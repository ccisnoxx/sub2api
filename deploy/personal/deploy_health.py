"""依赖身份与部署健康验收；由唯一生命周期提供配置防漂移边界。"""
import json
import re
from deploy_common import *

class HealthChecks:
    def __init__(self, runner, config, images, database):
        self.runner = runner
        self.config = config
        self.images = images
        self.database = database

    def dependencies(self, compose, image, expected=None):
        ids = {}
        for service in ("postgres", "redis"):
            cid = compose(image, "ps", "-q", service).decode().strip()
            require(re.fullmatch(r"[0-9a-f]{12,64}", cid), "E_DEPENDENCY", "数据库或 Redis 容器不存在或不唯一")
            container = self.images.inspect(cid)
            labels = container.get("Config", {}).get("Labels") or {}
            require(labels.get("com.docker.compose.project") == self.config.project and labels.get("com.docker.compose.service") == service and
                    container.get("State", {}).get("Running") is True and
                    container.get("State", {}).get("Health", {}).get("Status") == "healthy",
                    "E_DEPENDENCY", "数据库或 Redis 未健康运行")
            ids[service] = cid
        require(expected is None or expected == ids, "E_DEPENDENCY", "操作期间数据库或 Redis 容器已变化")
        value = self.runner.run(["docker", "exec", ids["postgres"], "sh", "-ec",
                                 'psql -U "$POSTGRES_USER" -d "$POSTGRES_DB" -Atqc "SELECT 1"'])
        require(value.strip() == b"1", "E_DEPENDENCY", "PostgreSQL SELECT 1 未通过")
        require(self.runner.run(["docker", "exec", ids["redis"], "redis-cli", "ping"]).strip() == b"PONG",
                "E_DEPENDENCY", "Redis PING 未通过")
        return ids

    def health(self, compose, guard_configuration, expected, dependencies, expected_migrations):
        guard_configuration()
        container = self.images.inspect(self.config.app_container)
        actual = self.images.running()
        require(actual["running"] and actual["health"] == "healthy", "E_HEALTH", "应用容器没有通过 healthcheck")
        require(container.get("Config", {}).get("Image") == expected["image"] and
                actual["image_id"] == expected["image_id"] and all(actual[key] == expected[key] for key in ("image", "revision", "version", "source")),
                "E_HEALTH", "实际运行镜像与不可变目标不一致")
        self.loopback_health()
        self.dependencies(compose, expected["image"], dependencies)
        require(self.database.migration_state(dependencies["postgres"]) == expected_migrations,
                "E_MIGRATION", "应用更新后迁移状态已变化，禁止自动镜像回滚")
        return actual


    def loopback_health(self):
        health = json.loads(self.runner.run(["curl", "--fail", "--silent", "--show-error", "--max-time", "10",
                                           self.config.health_url], sensitive=True))
        require(health.get("status") == "ok", "E_HEALTH", "主机 /health 未返回 ok")
        return health
