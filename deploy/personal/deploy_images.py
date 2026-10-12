"""只验证镜像/容器身份；拉取由生命周期明确调用。"""
import json
import re
from deploy_common import *

class ImageVerifier:
    def __init__(self, runner, config):
        self.runner = runner
        self.config = config

    def inspect(self, reference, image=False):
        args = ["docker", "image", "inspect", reference] if image else ["docker", "inspect", reference]
        values = json.loads(self.runner.run(args, sensitive=True))
        require(isinstance(values, list) and len(values) == 1, "E_INSPECT", "无法唯一识别容器或镜像")
        return values[0]

    def metadata(self, image, expected=None):
        require(image.get("Os") == "linux" and image.get("Architecture") == "amd64", "E_PLATFORM", "镜像平台必须为 linux/amd64")
        labels = image.get("Config", {}).get("Labels") or {}
        digests = image.get("RepoDigests") or []
        if expected:
            require(expected["image"] in digests, "E_DIGEST", "镜像 RepoDigest 不匹配固定目标")
            reference = expected["image"]
        else:
            accepted = [value for value in digests if value.startswith(REPOSITORY + "@") or value == LEGACY["image"]]
            require(len(accepted) == 1, "E_DIGEST", "运行镜像没有唯一的允许仓库 digest")
            reference = accepted[0]
        metadata = {"image": reference, "revision": labels.get("org.opencontainers.image.revision"),
                    "version": labels.get("org.opencontainers.image.version"), "source": labels.get("org.opencontainers.image.source")}
        validate_metadata(metadata, allow_legacy=True)
        if expected:
            require(all(metadata.get(key) == expected.get(key) for key in ("image", "revision", "version", "source")),
                    "E_LABEL", "镜像 OCI 标签与固定发布合同不一致")
        require(re.fullmatch(r"sha256:[0-9a-f]{64}", image.get("Id", "")), "E_IMAGE", "镜像 ID 无效")
        metadata["image_id"] = image["Id"]
        return metadata

    def running(self):
        container = self.inspect(self.config.app_container)
        labels = container.get("Config", {}).get("Labels") or {}
        require(labels.get("com.docker.compose.project") == self.config.project and labels.get("com.docker.compose.service") == self.config.app_service,
                "E_LAYOUT", "应用容器不属于预期 Compose 项目和服务")
        require(any(mount.get("Type") == "volume" and mount.get("Name") == self.config.app_volume and mount.get("Destination") == self.config.app_data_path
                    for mount in container.get("Mounts", [])), "E_LAYOUT", "应用持久卷不符合已验证现场")
        metadata = self.metadata(self.inspect(container["Image"], image=True))
        metadata["health"] = container.get("State", {}).get("Health", {}).get("Status")
        metadata["running"] = container.get("State", {}).get("Running") is True
        return metadata

    def pull_target(self, target, resolved_callback):
        value = target["input"]
        if "@" not in value:
            self.runner.run(["docker", "pull", "--platform", "linux/amd64", value], timeout=600)
            resolved = self.metadata(self.inspect(value, image=True))
            require(all(resolved[key] == target[key] for key in ("version", "revision", "source")), "E_LABEL", "版本镜像标签与 Git 发布合同不一致")
            target = dict(target, image=resolved["image"])
            resolved_callback(target)
        self.runner.run(["docker", "pull", "--platform", "linux/amd64", target["image"]], timeout=600)
        return self.metadata(self.inspect(target["image"], image=True), expected=target)
