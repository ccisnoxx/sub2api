"""精确 Compose 定位及只更新应用的命令合同；不持有部署状态。"""
import json
import os
import re
from deploy_common import *

class ComposeSite:
    def __init__(self, base, runner, config):
        self.base = Path(base)
        self.runner = runner
        self.config = config
        self.env_file = self.base / config.env_file

    def compose(self, directory, image, *arguments, sensitive=False):
        # 显式 env-file 与进程变量使用同一 digest，外部同名变量不能覆盖选择。
        env = dict(os.environ)
        env["SUB2API_IMAGE"] = image
        image_file = directory / "operation-image.env"
        atomic_private(image_file, ("SUB2API_IMAGE=" + image + "\n").encode())
        return self.runner.run(["docker", "compose", "--env-file", str(self.env_file), "--env-file", str(image_file),
                                "-p", self.config.project, "-f", self.config.compose_file, *arguments], env=env, sensitive=sensitive,
                               timeout=180)

    def configuration(self, compose, image):
        config = json.loads(compose(image, "config", "--format", "json", sensitive=True))
        services = config.get("services", {})
        require(set(services) == set(self.config.services), "E_LAYOUT", "服务集合与已验证现场不符")
        app = services[self.config.app_service]
        health_test = app.get("healthcheck", {}).get("test", [])
        require(app.get("container_name") == self.config.app_container and len(health_test) >= 2 and health_test[0] in ("CMD", "CMD-SHELL") and
                not app.get("healthcheck", {}).get("disable"), "E_LAYOUT", "应用容器名或 healthcheck 不符合合同")
        ports = app.get("ports", [])
        require(len(ports) == 1 and str(ports[0].get("published")) == str(self.config.published_port) and
                ports[0].get("target") == self.config.target_port and ports[0].get("host_ip") == self.config.host_ip,
                "E_LAYOUT", "应用端口不符合已验证现场")
        volumes = app.get("volumes", [])
        matching = [volume for volume in volumes if volume.get("target") == self.config.app_data_path and volume.get("type") == "volume"]
        require(len(matching) == 1 and config.get("volumes", {}).get(matching[0].get("source"), {}).get("name") == self.config.app_volume,
                "E_LAYOUT", "Compose 应用持久卷不符合已验证现场")
        return config

def adapted_configuration(before, app_service):
    text = before.decode("utf-8")
    # 不重新序列化 YAML；仅接受能唯一定位的普通 services/sub2api 结构。
    services = re.search(r"(?m)^services:[ \t]*(?:#[^\r\n]*)?\r?$", text)
    require(services is not None, "E_CONFIG", "无法定位 services，禁止替换生产 Compose")
    remaining = text[services.end():]
    section_end = re.search(r"(?m)^\S[^\r\n]*:", remaining)
    section = remaining[:section_end.start()] if section_end else remaining
    app = list(re.finditer(r"(?m)^  " + re.escape(app_service) + r":[ \t]*(?:#[^\r\n]*)?\r?$", section))
    require(len(app) == 1, "E_CONFIG", "无法唯一定位普通 sub2api 服务结构")
    offset = services.end() + app[0].end()
    rest = text[offset:]
    end = re.search(r"(?m)^  [^ #\r\n][^\r\n]*:|^\S[^\r\n]*:", rest)
    block = rest[:end.start()] if end else rest
    image_lines = list(re.finditer(r"(?m)^(    image:[ \t]*)([^\r\n]*)(\r?)$", block))
    require(len(image_lines) == 1, "E_CONFIG", "应用必须具有唯一且直接声明的 image 行")
    line = image_lines[0]
    value = line[2]
    comment_at = re.search(r"[ \t]+#", value)
    suffix = value[comment_at.start():] if comment_at else ""
    current = value[:comment_at.start()] if comment_at else value
    current = current.strip()
    allowed = {IMAGE_VARIABLE, '"' + IMAGE_VARIABLE + '"', "'" + IMAGE_VARIABLE + "'"}
    if current in allowed:
        return before
    require("${" not in current and re.fullmatch(r"[\"']?(?:ghcr\.io/kln-4096/sub2api|ghcr\.io/ccisnoxx/sub2api)[:@][A-Za-z0-9_.:-]+[\"']?", current),
            "E_CONFIG", "原应用 image 不符合已验证来源，禁止猜测适配")
    start = offset + line.start(2)
    stop = offset + line.end(2)
    return (text[:start] + '"' + IMAGE_VARIABLE + '"' + suffix + text[stop:]).encode()
