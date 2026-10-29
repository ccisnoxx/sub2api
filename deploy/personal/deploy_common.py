"""部署协议的错误、镜像输入和私密文件原语；不持有主机或生命周期状态。"""
import datetime
import hashlib
import json
import os
from pathlib import Path
import re
import tempfile


REPOSITORY = "ghcr.io/ccisnoxx/sub2api"

SOURCE = "https://github.com/ccisnoxx/sub2api"

# 退出个人应用补丁的单次固定切换，不开放任意上游版本或镜像仓库。
UPSTREAM_TARGET = {
    "image": "ghcr.io/kln-4096/sub2api@sha256:28076fa6f15b5a74dc5b6ce6a716f3909955b60a74de2d09801a1f4b8fc7d86a",
    "revision": "bdff1129ffbd22947612f83cd09d23bf3fa47f5d",
    "version": "0.2.19-klno.1",
    "source": "https://github.com/KlN-4096/sub2api",
}
UPSTREAM_OLD = {
    "image": "ghcr.io/ccisnoxx/sub2api@sha256:998adb232164ca58da43046ea2de24336f9b2e653d10f34e3a2739f52ed824d1",
    "revision": "6acab6c3a68eca9e7511bc6f34f9d0fd3e69276b",
    "version": "0.2.18-klno.3-tps.1",
    "source": SOURCE,
}
UPSTREAM_OLD_TREE = "c61e3dcc76de1b567d25bb8ad2863dc653b2cfb1"
UPSTREAM_TARGET_TREE = "8ef1bab1a6308d698d342c6639b7ae166797cbaa"
UPSTREAM_CHANGED_SHA256 = "646ecfdfc5b9e48897f248acea6821f810ad66953b6bc0df4d581765773a0b90"
UPSTREAM_OLD_LEDGER = {"rows": 316, "filename_checksum_sha256": "0f31040b9ad784825165747d29cc73ead2cd24e209156731eab4fa0fce180687"}
UPSTREAM_MIGRATIONS = {
    "247_scoped_admin_api_keys.sql": "03445774e2dfad32da9e23279193d1bca02953896bc3d3b8a87ba0b36b6ed577",
    "248_group_image_count_billing.sql": "a872dde339b19f311feb5fa6fd85c8ff7b11d69794c35a18e2086d9846b3499c",
    "254_add_usage_log_tps_inputs.sql": "4171933399bdae34c818f4638b2c4576c0208eab75f2d6c39f19c6c085cf768c",
}

LEGACY = {
    "image": "ghcr.io/kln-4096/sub2api@sha256:c0ec609deaf0fb6f323de660ed7d43cf2030b4c4083c6fc4bef506d28f08ad8c",
    "revision": "de08df02ae1d81668a22f798b398aa0438ac1276",
    "version": "0.2.14-klno.3",
    "source": "https://github.com/KlN-4096/sub2api",
}

IMAGE_VARIABLE = "${SUB2API_IMAGE:?必须指定应用镜像}"

VERSION_RE = re.compile(r"[0-9]+\.[0-9]+\.[0-9]+-klno\.[0-9]+-tps\.[1-9][0-9]*\Z")

SHA_RE = re.compile(r"[0-9a-f]{40}\Z")

DIGEST_RE = re.compile(r"sha256:[0-9a-f]{64}\Z")

RECORD_RE = re.compile(r"[0-9]{8}T[0-9]{6}Z-[0-9a-f]{12}\Z")

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
    if value == UPSTREAM_TARGET["image"]:
        require(version is not None and version.removeprefix("v") == UPSTREAM_TARGET["version"],
                "E_INPUT", "上游切换必须绑定固定 digest 与正式版本")
        return dict(UPSTREAM_TARGET, input=value)
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
    if all(metadata[key] == UPSTREAM_TARGET[key] for key in fields):
        return
    if allow_legacy and all(metadata[key] == LEGACY[key] for key in fields):
        return
    require(metadata["image"].startswith(REPOSITORY + "@") and DIGEST_RE.fullmatch(metadata["image"].split("@", 1)[1]),
            "E_IMAGE", "镜像仓库或 digest 不符合部署合同")
    require(metadata["source"] == SOURCE and VERSION_RE.fullmatch(metadata["version"]),
            "E_IMAGE", "镜像 source 或个人版本不符合部署合同")
