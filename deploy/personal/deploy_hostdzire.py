#!/usr/bin/env python3
"""个人镜像部署入口；仅使用标准库，远端 Deployment 拥有整个状态转换。"""

import argparse
import datetime
import gzip
import hashlib
import json
import os
from pathlib import Path
import re
import signal
import stat
import subprocess
import sys
import tarfile
import tempfile
import uuid


REPOSITORY = "ghcr.io/ccisnoxx/sub2api"
SOURCE = "https://github.com/ccisnoxx/sub2api"
LEGACY = {
    "image": "ghcr.io/kln-4096/sub2api@sha256:c0ec609deaf0fb6f323de660ed7d43cf2030b4c4083c6fc4bef506d28f08ad8c",
    "revision": "de08df02ae1d81668a22f798b398aa0438ac1276",
    "version": "0.2.14-klno.3",
    "source": "https://github.com/KlN-4096/sub2api",
}
BASE = Path("/root/sub2api-kin")
PROJECT = "sub2api-kin"
APP = "sub2api-kin"
VOLUME = "sub2api-kin_sub2api_data"
IMAGE_VARIABLE = "${SUB2API_IMAGE:?必须指定应用镜像}"
VERSION_RE = re.compile(r"[0-9]+\.[0-9]+\.[0-9]+-klno\.[0-9]+-tps\.[1-9][0-9]*\Z")
SHA_RE = re.compile(r"[0-9a-f]{40}\Z")
DIGEST_RE = re.compile(r"sha256:[0-9a-f]{64}\Z")
RECORD_RE = re.compile(r"[0-9]{8}T[0-9]{6}Z-[0-9a-f]{12}\Z")
# 完整后端与运行资源；部署工具、来源记录不属于被部署的应用运行配置。
RUNTIME_PATHS = ["backend", "Dockerfile", "Dockerfile.goreleaser", ".dockerignore", "deploy"]
RUNTIME_EXCLUDES = ["deploy/personal", "deploy/personal-source.json"]


class DeployError(Exception):
    def __init__(self, code, message):
        super().__init__(message)
        self.code = code


def require(condition, code, message):
    if not condition:
        raise DeployError(code, message)


def canonical(value):
    return json.dumps(value, sort_keys=True, ensure_ascii=False, separators=(",", ":")).encode()


def sha(data):
    return hashlib.sha256(data).hexdigest()


def file_sha(path):
    digest = hashlib.sha256()
    with open(path, "rb") as source:
        for chunk in iter(lambda: source.read(1024 * 1024), b""):
            digest.update(chunk)
    return digest.hexdigest()


def now():
    return datetime.datetime.now(datetime.timezone.utc).isoformat()


def private_directory(path):
    require(not path.is_symlink(), "E_PATH", "私密目录不能是符号链接")
    path.mkdir(mode=0o700, parents=True, exist_ok=True)
    os.chmod(path, 0o700)


def read_regular(path):
    require(path.is_file() and not path.is_symlink(), "E_PATH", "必要文件缺失或为符号链接")
    return path.read_bytes()


def atomic_private(path, data):
    require(not path.is_symlink(), "E_PATH", "不能覆盖符号链接")
    fd, temporary = tempfile.mkstemp(prefix=".write-", dir=path.parent)
    try:
        with os.fdopen(fd, "wb") as output:
            output.write(data)
            output.flush()
            os.fsync(output.fileno())
        os.replace(temporary, path)
        directory = os.open(path.parent, os.O_RDONLY)
        try:
            os.fsync(directory)
        finally:
            os.close(directory)
    finally:
        if os.path.exists(temporary):
            os.unlink(temporary)


def parse_target(value, version=None):
    if value.startswith(REPOSITORY + "@"):
        require(DIGEST_RE.fullmatch(value[len(REPOSITORY) + 1:]), "E_INPUT", "digest 格式无效")
        require(version is not None, "E_INPUT", "digest 输入必须同时指定 --version")
        version = version.removeprefix("v")
        require(VERSION_RE.fullmatch(version), "E_INPUT", "必须指定个人发布版本")
        return {"input": value, "version": version, "image": value}
    if value.startswith(REPOSITORY + ":"):
        value = value[len(REPOSITORY) + 1:]
    value = value.removeprefix("v")
    require(VERSION_RE.fullmatch(value), "E_INPUT", "仅接受个人版本或 fork 的固定 digest，禁止 latest 和其他仓库")
    require(version is None or version.removeprefix("v") == value, "E_INPUT", "版本参数不一致")
    return {"input": REPOSITORY + ":" + value, "version": value}


def validate_metadata(metadata, allow_legacy=False):
    fields = ("image", "revision", "version", "source")
    require(all(isinstance(metadata.get(key), str) for key in fields), "E_IMAGE", "镜像来源元数据不完整")
    require(SHA_RE.fullmatch(metadata["revision"]), "E_IMAGE", "镜像 revision 必须为完整 SHA")
    if allow_legacy and all(metadata[key] == LEGACY[key] for key in fields):
        return
    require(metadata["image"].startswith(REPOSITORY + "@") and DIGEST_RE.fullmatch(metadata["image"].split("@", 1)[1]),
            "E_IMAGE", "镜像仓库或 digest 不符合部署合同")
    require(metadata["source"] == SOURCE and VERSION_RE.fullmatch(metadata["version"]),
            "E_IMAGE", "镜像 source 或个人版本不符合部署合同")


class GitEvidence:
    def __init__(self, root):
        self.root = root

    def git(self, *args):
        result = subprocess.run(["git", "-C", str(self.root), *args], capture_output=True)
        require(result.returncode == 0, "E_GIT", "无法读取所需 Git 对象；请先获取个人标签与实际旧 revision")
        return result.stdout

    def source_at(self, revision):
        data = json.loads(self.git("show", revision + ":deploy/personal-source.json"))
        require(data.get("upstream_repository") == "KlN-4096/sub2api" and
                SHA_RE.fullmatch(data.get("upstream_sha", "")) and
                re.fullmatch(r"v[0-9]+\.[0-9]+\.[0-9]+-klno\.[0-9]+", data.get("upstream_tag", "")),
                "E_COMPATIBILITY", "来源记录不完整，禁止部署或镜像回滚")
        require(self.git("rev-parse", "refs/tags/" + data["upstream_tag"] + "^{commit}").decode().strip() == data["upstream_sha"],
                "E_COMPATIBILITY", "来源标签与记录 SHA 不一致")
        result = subprocess.run(["git", "-C", str(self.root), "merge-base", "--is-ancestor", data["upstream_sha"], revision], capture_output=True)
        require(result.returncode == 0, "E_COMPATIBILITY", "个人提交不包含所记录的上游基线")
        return data

    def personal(self, version, revision=None):
        require(VERSION_RE.fullmatch(version), "E_GIT", "必须使用个人发布标签")
        actual = self.git("rev-parse", "refs/tags/v" + version + "^{commit}").decode().strip()
        require(SHA_RE.fullmatch(actual) and (revision is None or actual == revision), "E_GIT", "OCI revision 与个人 Git 标签不一致")
        self.source_at(actual)
        return actual

    def compatibility(self, old, target):
        validate_metadata(old, allow_legacy=True)
        if "image" in target:
            validate_metadata(target, allow_legacy=True)
        else:
            require(target.get("source") == SOURCE and VERSION_RE.fullmatch(target.get("version", "")) and
                    SHA_RE.fullmatch(target.get("revision", "")), "E_COMPATIBILITY", "目标来源不完整，禁止猜测兼容性")
        target_source = self.source_at(target["revision"]) if target["source"] == SOURCE else {
            "upstream_repository": "KlN-4096/sub2api", "upstream_tag": "v0.2.14-klno.3", "upstream_sha": LEGACY["revision"]}
        old_source = self.source_at(old["revision"]) if old["source"] == SOURCE else {
            "upstream_repository": "KlN-4096/sub2api", "upstream_tag": "v0.2.14-klno.3", "upstream_sha": LEGACY["revision"]}
        require(old_source == target_source, "E_COMPATIBILITY", "上游基线不同，禁止部署或镜像回滚")
        paths = RUNTIME_PATHS + [":(exclude)" + path for path in RUNTIME_EXCLUDES]
        changed = self.git("diff", "--name-only", "-z", old["revision"], target["revision"], "--", *paths).decode().split("\0")
        changed = [path for path in changed if path]
        proof = {"old_revision": old["revision"], "new_revision": target["revision"],
                 "upstream": target_source, "paths": RUNTIME_PATHS, "excludes": RUNTIME_EXCLUDES,
                 "changed_paths": changed, "compatible": not changed}
        require(not changed, "E_COMPATIBILITY", "后端、迁移、镜像或运行配置有变化，禁止部署或镜像回滚")
        proof["sha256"] = sha(canonical(proof))
        return proof


class Runner:
    """进程边界保存私密诊断，异常只暴露固定错误码与说明。"""
    def __init__(self, base):
        self.base = base
        self.log = None

    def run(self, args, *, env=None, stdin=None, stdin_path=None, stdout_path=None, timeout=180, sensitive=False):
        output = open(stdout_path, "wb") if stdout_path else subprocess.PIPE
        input_file = open(stdin_path, "rb") if stdin_path else None
        if stdout_path:
            os.chmod(stdout_path, 0o600)
        try:
            result = subprocess.run(args, cwd=self.base, env=env, input=stdin, stdin=input_file, stdout=output,
                                    stderr=subprocess.PIPE, timeout=timeout)
        except subprocess.TimeoutExpired as error:
            if self.log:
                with open(self.log, "ab") as log:
                    log.write(canonical({"command": args, "timeout": timeout}) + b"\n")
            raise DeployError("E_TIMEOUT", "远端命令超时，详情保存在私密诊断") from error
        finally:
            if stdout_path:
                output.close()
            if input_file:
                input_file.close()
        if self.log and not sensitive:
            with open(self.log, "ab") as log:
                log.write(canonical({"command": args, "returncode": result.returncode}) + b"\n")
                if result.stdout:
                    log.write(result.stdout + b"\n")
                log.write(result.stderr + b"\n")
        require(result.returncode == 0, "E_COMMAND", "远端命令失败，详情保存在私密诊断")
        return result.stdout or b""


class Deployment:
    def __init__(self, base=BASE, runner=None):
        self.base = Path(base)
        self.compose_file = self.base / "docker-compose.yml"
        self.env_file = self.base / ".env"
        self.state = self.base / ".personal-deploy"
        self.selection = self.state / "image.env"
        self.runner = runner or Runner(self.base)
        self.record = None
        self.directory = None
        self.config_before = None
        self.config_after = None
        self.env_before = None
        self.selection_before = None
        self.started = False
        self.adapted = False

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
        container = self.inspect(APP)
        labels = container.get("Config", {}).get("Labels") or {}
        require(labels.get("com.docker.compose.project") == PROJECT and labels.get("com.docker.compose.service") == "sub2api",
                "E_LAYOUT", "应用容器不属于预期 Compose 项目和服务")
        require(any(mount.get("Type") == "volume" and mount.get("Name") == VOLUME and mount.get("Destination") == "/app/data"
                    for mount in container.get("Mounts", [])), "E_LAYOUT", "应用持久卷不符合已验证现场")
        metadata = self.metadata(self.inspect(container["Image"], image=True))
        metadata["health"] = container.get("State", {}).get("Health", {}).get("Status")
        metadata["running"] = container.get("State", {}).get("Running") is True
        return metadata

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
        # 显式 env-file 与进程变量使用同一 digest，外部同名变量不能覆盖选择。
        env = dict(os.environ)
        env["SUB2API_IMAGE"] = image
        image_file = self.directory / "operation-image.env"
        atomic_private(image_file, ("SUB2API_IMAGE=" + image + "\n").encode())
        return self.runner.run(["docker", "compose", "--env-file", str(self.env_file), "--env-file", str(image_file),
                                "-p", PROJECT, "-f", "docker-compose.yml", *arguments], env=env, sensitive=sensitive,
                               timeout=180)

    def configuration(self, image):
        config = json.loads(self.compose(image, "config", "--format", "json", sensitive=True))
        services = config.get("services", {})
        require(set(services) == {"sub2api", "postgres", "redis"}, "E_LAYOUT", "服务集合与已验证现场不符")
        app = services["sub2api"]
        health_test = app.get("healthcheck", {}).get("test", [])
        require(app.get("container_name") == APP and len(health_test) >= 2 and health_test[0] in ("CMD", "CMD-SHELL") and
                not app.get("healthcheck", {}).get("disable"), "E_LAYOUT", "应用容器名或 healthcheck 不符合合同")
        ports = app.get("ports", [])
        require(len(ports) == 1 and str(ports[0].get("published")) == "10088" and
                ports[0].get("target") == 8080 and ports[0].get("host_ip") == "127.0.0.1",
                "E_LAYOUT", "应用端口不符合已验证现场")
        volumes = app.get("volumes", [])
        matching = [volume for volume in volumes if volume.get("target") == "/app/data" and volume.get("type") == "volume"]
        require(len(matching) == 1 and config.get("volumes", {}).get(matching[0].get("source"), {}).get("name") == VOLUME,
                "E_LAYOUT", "Compose 应用持久卷不符合已验证现场")
        return config

    def dependencies(self, image, expected=None):
        ids = {}
        for service in ("postgres", "redis"):
            cid = self.compose(image, "ps", "-q", service).decode().strip()
            require(re.fullmatch(r"[0-9a-f]{12,64}", cid), "E_DEPENDENCY", "数据库或 Redis 容器不存在或不唯一")
            container = self.inspect(cid)
            labels = container.get("Config", {}).get("Labels") or {}
            require(labels.get("com.docker.compose.project") == PROJECT and labels.get("com.docker.compose.service") == service and
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

    def save(self, status=None, **values):
        if status:
            self.record["status"] = status
        self.record.update(values)
        self.record["updated_at"] = now()
        atomic_private(self.directory / "record.json", canonical(self.record) + b"\n")

    def migration_state(self, postgres_id):
        listing = self.runner.run(["docker", "exec", postgres_id, "sh", "-ec",
                                   'psql -U "$POSTGRES_USER" -d "$POSTGRES_DB" -Atqc "SELECT filename, checksum FROM schema_migrations ORDER BY filename, checksum"'],
                                  sensitive=True)
        require(listing.strip(), "E_MIGRATION", "迁移记录为空或不可读取，禁止猜测旧版本兼容性")
        return {"rows": len(listing.splitlines()), "filename_checksum_sha256": sha(listing)}

    def guard_configuration(self):
        require(sha(read_regular(self.compose_file)) == sha(self.config_after if self.adapted else self.config_before) and
                sha(read_regular(self.env_file)) == sha(self.env_before), "E_DRIFT", "Compose 或 .env 已被外部修改，停止覆盖和回滚")
        current = read_regular(self.selection) if self.selection.exists() else None
        require(current == self.selection_before, "E_DRIFT", "镜像选择已被外部修改，停止覆盖和回滚")

    def backup(self, old, dependencies):
        for name, content in (("docker-compose.yml", self.config_before), ("environment.env", self.env_before)):
            atomic_private(self.directory / name, content)
        if self.selection_before is not None:
            atomic_private(self.directory / "previous-image.env", self.selection_before)
        dump = self.directory / "postgres.dump"
        self.runner.run(["docker", "exec", dependencies["postgres"], "sh", "-ec",
                         'pg_dump -U "$POSTGRES_USER" -d "$POSTGRES_DB" --format=custom'], stdout_path=dump, timeout=900)
        require(dump.stat().st_size > 0, "E_BACKUP", "数据库备份为空")
        listing = self.runner.run(["docker", "exec", "-i", dependencies["postgres"], "pg_restore", "--list"],
                                  stdin_path=dump, timeout=900)
        require(listing.strip(), "E_BACKUP", "数据库备份未通过 pg_restore --list")
        archive = self.directory / "app-data.tar.gz"
        self.runner.run(["docker", "exec", APP, "tar", "-C", "/app/data", "-czf", "-", "."], stdout_path=archive, timeout=900)
        try:
            with tarfile.open(archive, "r:gz") as data:
                data.getmembers()
            # tar 的结束块可能早于 gzip 尾部；读至 EOF 才能验证 CRC 和长度。
            with gzip.open(archive, "rb") as data:
                for _ in iter(lambda: data.read(1024 * 1024), b""):
                    pass
        except (tarfile.TarError, EOFError, OSError) as error:
            raise DeployError("E_BACKUP", "应用数据归档校验失败") from error
        self.save("backed_up", backups={"postgres": {"file": "postgres.dump", "sha256": file_sha(dump), "bytes": dump.stat().st_size},
                                        "app_data": {"file": "app-data.tar.gz", "sha256": file_sha(archive), "bytes": archive.stat().st_size},
                                        "configuration": "docker-compose.yml", "environment": "environment.env"})

    def adapt_configuration(self):
        text = self.config_before.decode("utf-8")
        # 不重新序列化 YAML；仅接受能唯一定位的普通 services/sub2api 结构。
        services = re.search(r"(?m)^services:[ \t]*(?:#[^\r\n]*)?\r?$", text)
        require(services is not None, "E_CONFIG", "无法定位 services，禁止替换生产 Compose")
        remaining = text[services.end():]
        section_end = re.search(r"(?m)^\S[^\r\n]*:", remaining)
        section = remaining[:section_end.start()] if section_end else remaining
        app = list(re.finditer(r"(?m)^  sub2api:[ \t]*(?:#[^\r\n]*)?\r?$", section))
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
            self.config_after = self.config_before
            return
        require("${" not in current and re.fullmatch(r"[\"']?(?:ghcr\.io/kln-4096/sub2api|ghcr\.io/ccisnoxx/sub2api)[:@][A-Za-z0-9_.:-]+[\"']?", current),
                "E_CONFIG", "原应用 image 不符合已验证来源，禁止猜测适配")
        start = offset + line.start(2)
        stop = offset + line.end(2)
        self.config_after = (text[:start] + '"' + IMAGE_VARIABLE + '"' + suffix + text[stop:]).encode()

    def health(self, expected, dependencies):
        self.guard_configuration()
        container = self.inspect(APP)
        actual = self.running()
        require(actual["running"] and actual["health"] == "healthy", "E_HEALTH", "应用容器没有通过 healthcheck")
        require(container.get("Config", {}).get("Image") == expected["image"] and
                actual["image_id"] == expected["image_id"] and all(actual[key] == expected[key] for key in ("image", "revision", "version", "source")),
                "E_HEALTH", "实际运行镜像与不可变目标不一致")
        health = json.loads(self.runner.run(["curl", "--fail", "--silent", "--show-error", "--max-time", "10",
                                           "http://127.0.0.1:10088/health"], sensitive=True))
        require(health.get("status") == "ok", "E_HEALTH", "主机 /health 未返回 ok")
        self.dependencies(expected["image"], dependencies)
        require(self.migration_state(dependencies["postgres"]) == self.record["migration_state"],
                "E_MIGRATION", "应用更新后迁移状态已变化，禁止自动镜像回滚")
        return actual

    def start(self, target):
        self.compose(target["image"], "up", "-d", "--no-deps", "--wait", "--wait-timeout", "120", "sub2api")

    def validate_proof(self, request, old, target):
        proof = request.get("compatibility", {})
        content = {key: value for key, value in proof.items() if key != "sha256"}
        require(proof.get("sha256") == sha(canonical(content)) and proof.get("compatible") is True and
                proof.get("old_revision") == old["revision"] and proof.get("new_revision") == target["revision"] and
                proof.get("paths") == RUNTIME_PATHS and proof.get("excludes") == RUNTIME_EXCLUDES and proof.get("changed_paths") == [],
                "E_COMPATIBILITY", "兼容证据无效或没有绑定实际旧、新 revision")

    def pull_target(self, target):
        value = target["input"]
        if "@" not in value:
            self.runner.run(["docker", "pull", "--platform", "linux/amd64", value], timeout=600)
            resolved = self.metadata(self.inspect(value, image=True))
            require(all(resolved[key] == target[key] for key in ("version", "revision", "source")), "E_LABEL", "版本镜像标签与 Git 发布合同不一致")
            target = dict(target, image=resolved["image"])
            self.save("resolved", target=target)
        self.runner.run(["docker", "pull", "--platform", "linux/amd64", target["image"]], timeout=600)
        return self.metadata(self.inspect(target["image"], image=True), expected=target)

    def rollback_binding(self, request, old, target):
        previous = self.read_record(request["rollback_id"])
        selected = self.selected()
        require(previous.get("status") == "success" and previous.get("action") == "deploy" and
                selected == {"image": old["image"], "record_id": request["rollback_id"]}, "E_STALE_ROLLBACK", "回滚记录不是当前成功部署，禁止过期回滚")
        require(all(previous["target"].get(key) == old[key] and previous["old"].get(key) == target[key]
                    for key in ("image", "revision", "version", "source")), "E_STALE_ROLLBACK", "当前镜像或回滚目标与记录不符")
        proof = previous.get("compatibility", {})
        require(proof.get("compatible") is True and proof.get("old_revision") == target["revision"] and
                proof.get("new_revision") == old["revision"] and proof.get("changed_paths") == [] and
                proof.get("sha256") == sha(canonical({key: value for key, value in proof.items() if key != "sha256"})),
                "E_COMPATIBILITY", "原部署没有可验证的兼容回滚证据")
        config = previous.get("configuration", {})
        require(config.get("after_sha256") == sha(self.config_before) and config.get("env_sha256") == sha(self.env_before),
                "E_STALE_ROLLBACK", "成功部署后的运行配置已变化，禁止镜像回滚")
        return previous

    def diagnostics(self):
        try:
            self.runner.run(["docker", "logs", "--tail", "100", APP], stdout_path=self.directory / "application.log", timeout=20)
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
                       "compatibility": request.get("compatibility"), "rollback_of": request.get("rollback_id")}
        self.save("preflight")
        old = None
        dependencies = None
        proof_valid = False
        try:
            require(request.get("schema") == 1 and request.get("action") in ("deploy", "rollback") and
                    SHA_RE.fullmatch(request.get("tool_revision", "")) and request.get("tool_sha256") == sha(Path(__file__).read_bytes()),
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
            self.env_before = read_regular(self.env_file)
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
            proof_valid = True
            self.save("preflight", old=old, target=target)
            if request["action"] == "rollback":
                self.rollback_binding(request, old, target)
            self.adapt_configuration()
            baseline = self.configuration(old["image"])
            dependencies = self.dependencies(old["image"])
            migrations = self.migration_state(dependencies["postgres"])
            require(previous is None or previous.get("migration_state") == migrations,
                    "E_MIGRATION", "当前迁移状态与成功记录不一致，禁止部署或镜像回滚")
            self.save("preflight", configuration={"before_sha256": sha(self.config_before), "after_sha256": sha(self.config_after),
                                                  "env_sha256": sha(self.env_before)}, dependencies=dependencies,
                      migration_state=migrations)
            self.backup(old, dependencies)
            resolved = self.pull_target(target)
            self.save("pulled", target=resolved)
            self.guard_configuration()
            # 先取得旧镜像固定引用；旧标签没有不可变本地映射时禁止重建。
            self.metadata(self.inspect(old["image"], image=True), expected=old)
            atomic_private(self.compose_file, self.config_after)
            self.adapted = True
            candidate = self.configuration(resolved["image"])
            require(candidate["services"]["sub2api"]["image"] == resolved["image"], "E_CONFIG", "Compose 未选择目标 digest")
            baseline["services"]["sub2api"]["image"] = resolved["image"]
            require(candidate == baseline, "E_CONFIG", "适配改变了应用镜像以外的 Compose 配置")
            self.guard_configuration()
            self.save("starting")
            self.started = True
            self.start(resolved)
            actual = self.health(resolved, dependencies)
            self.guard_configuration()
            self.save("healthy_pending_commit", health_verified=True, observed_running=actual)
            atomic_private(self.selection, ("SUB2API_IMAGE=" + resolved["image"] + "\nSUB2API_DEPLOYMENT_ID=" + record_id + "\n").encode())
            self.save("success", completed_at=now(), exit_code=0)
            return self.public_result(), 0
        except Exception as error:
            code = error.code if isinstance(error, DeployError) else "E_INTERNAL"
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
                try:
                    self.guard_configuration()
                    self.dependencies(old["image"], dependencies)
                    require(self.migration_state(dependencies["postgres"]) == self.record["migration_state"],
                            "E_MIGRATION", "失败后迁移状态已变化，禁止自动镜像回滚")
                    self.save("rolling_back", error_code=code)
                    restored = self.metadata(self.inspect(old["image"], image=True), expected=old)
                    self.start(restored)
                    self.health(restored, dependencies)
                    self.guard_configuration()
                    atomic_private(self.compose_file, self.config_before)
                    self.adapted = False
                    self.record["rollback_status"] = "success"
                except Exception as rollback_error:
                    self.record["rollback_status"] = "failed"
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

    def public_result(self):
        return {key: self.record[key] for key in ("id", "action", "status", "error_code", "exit_code", "old", "target",
                                                "observed_running", "rollback_status", "rollback_error", "configuration_restore_error",
                                                "selection_committed") if key in self.record}


class SSHTransport:
    """只使用现有 hostdzire 别名；上传的工具目录在操作后清理，记录留在部署目录。"""
    def __init__(self, source):
        self.source = source
        self.directory = None

    def command(self, args, data=None):
        result = subprocess.run(args, input=data, capture_output=True)
        require(result.returncode == 0, "E_SSH", "OpenSSH 操作失败；未改变 SSH 配置，远端状态须重新核对")
        return result.stdout

    def __enter__(self):
        directory = self.command(["ssh", "hostdzire", "umask 077; mktemp -d /tmp/sub2api-personal.XXXXXXXXXXXX"]).decode().strip()
        require(re.fullmatch(r"/tmp/sub2api-personal\.[A-Za-z0-9]{12}", directory), "E_SSH", "远端临时目录格式无效")
        self.directory = directory
        try:
            self.command(["scp", str(self.source), "hostdzire:" + directory + "/deploy_hostdzire.py"])
        except Exception:
            self.__exit__(None, None, None)
            raise
        return self

    def __exit__(self, *_):
        if self.directory:
            try:
                self.command(["ssh", "hostdzire", "rm -rf -- " + self.directory])
            except DeployError:
                # 临时工具清理不参与已核实的部署提交；不覆盖原始结果或异常。
                print("远端临时工具目录清理失败；部署结果仍以服务器记录为准。", file=sys.stderr)

    def probe(self, record_id=None):
        args = ["ssh", "hostdzire", "python3 " + self.directory + "/deploy_hostdzire.py --remote-probe"]
        if record_id:
            require(RECORD_RE.fullmatch(record_id), "E_INPUT", "部署记录 ID 无效")
            args[-1] += " --record " + record_id
        return json.loads(self.command(args))

    def execute(self, request):
        result = subprocess.run(["ssh", "hostdzire", "python3 " + self.directory + "/deploy_hostdzire.py --remote-apply"],
                                input=canonical(request), capture_output=True)
        try:
            response = json.loads(result.stdout)
        except (ValueError, UnicodeDecodeError) as error:
            raise DeployError("E_SSH", "未收到有效远端结果；不要重试写入，请先核对部署记录与运行镜像") from error
        require(result.returncode in (0, 1, 2) and response.get("status") in ("success", "failed"), "E_SSH", "远端结果不完整")
        require((response["status"] == "success") == (result.returncode == 0), "E_SSH", "远端结果与退出码不一致")
        return response, result.returncode


def local_main(args):
    root = Path(__file__).resolve().parents[2]
    git = GitEvidence(root)
    target = parse_target(args.target, args.version) if not args.rollback else None
    require(args.rollback is None or (args.target is None and args.version is None and RECORD_RE.fullmatch(args.rollback)),
            "E_INPUT", "--rollback 只接受部署记录 ID，不与目标版本混用")
    origin = git.git("remote", "get-url", "origin").decode().strip()
    require(origin in ("git@github.com:ccisnoxx/sub2api.git", "https://github.com/ccisnoxx/sub2api.git", "https://github.com/ccisnoxx/sub2api"),
            "E_GIT", "本机 origin 必须是 ccisnoxx/sub2api")
    if target:
        target.update(revision=git.personal(target["version"]), source=SOURCE)
    with SSHTransport(Path(__file__).resolve()) as remote:
        snapshot = remote.probe(args.rollback)
        old = snapshot["running"]
        validate_metadata(old, allow_legacy=True)
        if old["source"] == SOURCE:
            git.personal(old["version"], old["revision"])
        if args.rollback:
            previous = snapshot["rollback_record"]
            require(previous.get("status") == "success" and previous.get("action") == "deploy" and
                    snapshot.get("selection") == {"image": old["image"], "record_id": args.rollback},
                    "E_STALE_ROLLBACK", "回滚记录不是当前成功部署")
            target = dict(previous["old"])
            validate_metadata(target, allow_legacy=True)
            if target["source"] == SOURCE:
                git.personal(target["version"], target["revision"])
            target["input"] = target["image"]
        proof = git.compatibility(old, target)
        request = {"schema": 1, "action": "rollback" if args.rollback else "deploy", "expected_old": old, "target": target,
                   "compatibility": proof, "tool_revision": git.git("rev-parse", "HEAD").decode().strip(),
                   "tool_sha256": sha(Path(__file__).read_bytes()), "rollback_id": args.rollback}
        response, code = remote.execute(request)
        print(json.dumps(response, ensure_ascii=False, indent=2))
        return code


def main():
    parser = argparse.ArgumentParser(description="使用现有 hostdzire SSH 别名部署个人版本；digest 需同时提供 --version。")
    parser.add_argument("target", nargs="?", help="个人版本、fork 版本地址或 fork@sha256:digest")
    parser.add_argument("--version", help="固定 digest 所对应的个人版本")
    parser.add_argument("--rollback", metavar="记录ID", help="只回滚当前成功部署记录")
    parser.add_argument("--remote-probe", action="store_true", help=argparse.SUPPRESS)
    parser.add_argument("--remote-apply", action="store_true", help=argparse.SUPPRESS)
    parser.add_argument("--record", help=argparse.SUPPRESS)
    args = parser.parse_args()
    try:
        if args.remote_probe:
            print(json.dumps(Deployment().probe(args.record), ensure_ascii=False))
            return 0
        if args.remote_apply:
            def interrupted(_signum, _frame):
                raise DeployError("E_INTERRUPTED", "操作被中断，保存失败并尝试适用的回滚")
            signal.signal(signal.SIGTERM, interrupted)
            signal.signal(signal.SIGINT, interrupted)
            response, code = Deployment().execute(json.load(sys.stdin))
            print(json.dumps(response, ensure_ascii=False))
            return code
        require(args.target is not None or args.rollback is not None, "E_INPUT", "必须指定个人版本、digest 或 --rollback 记录ID")
        return local_main(args)
    except DeployError as error:
        print(json.dumps({"status": "failed", "error_code": error.code, "message": str(error)}, ensure_ascii=False))
        return 2
    except Exception:
        print(json.dumps({"status": "failed", "error_code": "E_INTERNAL", "message": "输入、环境或记录异常；未确认完成，请核对私密部署记录"}, ensure_ascii=False))
        return 2


if __name__ == "__main__":
    sys.exit(main())
