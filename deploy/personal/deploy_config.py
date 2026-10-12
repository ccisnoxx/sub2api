"""完整且无凭据的现场 schema；缺失、未知字段和错误类型均失败。"""
from dataclasses import dataclass, fields
import ipaddress
import json
from pathlib import Path
import re
from deploy_common import DeployError, require, read_regular

CONFIG_NAME = "hostdzire.json"


@dataclass(frozen=True)
class MaintenanceConfig:
    site: str
    server_name: str
    allowed_site_parents: tuple
    flag_directory: str
    record_directory: str


@dataclass(frozen=True)
class HostConfig:
    schema: int
    ssh_alias: str
    scratch_template: str
    base: str
    compose_file: str
    env_file: str
    state_directory: str
    project: str
    app_service: str
    app_container: str
    postgres_container: str
    app_volume: str
    app_data_path: str
    services: tuple
    host_ip: str
    published_port: int
    target_port: int
    health_url: str
    wait_timeout: int
    maintenance: MaintenanceConfig


def exact_object(value, model, types):
    require(type(value) is dict and set(value) == {field.name for field in fields(model)},
            "E_CONFIG_SCHEMA", "现场配置存在缺失或未知字段")
    require(all(type(value[key]) is kind for key, kind in types.items()),
            "E_CONFIG_SCHEMA", "现场配置字段类型错误")
    return value


def parse_config(value):
    names = {field.name for field in fields(HostConfig)}
    strings = names - {"schema", "services", "published_port", "target_port", "wait_timeout", "maintenance"}
    types = {key: str for key in strings}
    types.update(schema=int, services=list, published_port=int, target_port=int, wait_timeout=int, maintenance=dict)
    exact_object(value, HostConfig, types)
    m = exact_object(value["maintenance"], MaintenanceConfig,
                     {"site": str, "server_name": str, "allowed_site_parents": list, "flag_directory": str, "record_directory": str})
    require(value["schema"] == 1 and all(value[key] for key in strings), "E_CONFIG_SCHEMA", "现场配置版本或字符串为空")
    require(all(type(item) is str for item in value["services"]) and
            set(value["services"]) == {value["app_service"], "postgres", "redis"} and len(value["services"]) == 3,
            "E_CONFIG_SCHEMA", "现场配置必须完整指定应用、PostgreSQL、Redis 服务")
    for key in ("ssh_alias", "project", "app_service", "app_container", "postgres_container", "app_volume"):
        require(re.fullmatch(r"[A-Za-z0-9][A-Za-z0-9_.-]*", value[key]), "E_CONFIG_SCHEMA", "现场配置标识格式无效")
    for key in ("base", "app_data_path"):
        require(Path(value[key]).is_absolute() and ".." not in Path(value[key]).parts,
                "E_CONFIG_SCHEMA", "现场配置路径必须为明确绝对路径")
    for key in ("compose_file", "env_file", "state_directory"):
        require(Path(value[key]).name == value[key] and value[key] not in (".", ".."),
                "E_CONFIG_SCHEMA", "现场配置文件必须为直接子项")
    require(re.fullmatch(r"/tmp/[A-Za-z0-9_.-]+\.X{12}", value["scratch_template"]),
            "E_CONFIG_SCHEMA", "工具 scratch 必须是明确的 /tmp mktemp 模板")
    require(1 <= value["published_port"] <= 65535 and 1 <= value["target_port"] <= 65535 and
            1 <= value["wait_timeout"] <= 600, "E_CONFIG_SCHEMA", "现场端口或等待时间无效")
    try:
        address = ipaddress.ip_address(value["host_ip"])
    except ValueError as error:
        raise DeployError("E_CONFIG_SCHEMA", "现场回环地址无效") from error
    require(address.version == 4 and address.is_loopback and
            value["health_url"] == "http://" + value["host_ip"] + ":" + str(value["published_port"]) + "/health",
            "E_CONFIG_SCHEMA", "健康地址必须绑定配置中的 IPv4 回环端口")
    require(re.fullmatch(r"[a-z0-9]+(?:[.-][a-z0-9]+)*", m["server_name"]),
            "E_CONFIG_SCHEMA", "维护 server_name 必须明确且格式有效")
    require(all(type(item) is str and Path(item).is_absolute() for item in m["allowed_site_parents"]) and
            m["allowed_site_parents"] and len(m["allowed_site_parents"]) == len(set(m["allowed_site_parents"])),
            "E_CONFIG_SCHEMA", "维护站点父目录配置无效")
    require(all(Path(m[key]).is_absolute() and ".." not in Path(m[key]).parts for key in ("site", "flag_directory")) and
            str(Path(m["site"]).parent) in m["allowed_site_parents"], "E_CONFIG_SCHEMA", "维护路径配置无效")
    require(not Path(m["record_directory"]).is_absolute() and ".." not in Path(m["record_directory"]).parts and
            Path(m["record_directory"]).parts[:1] == (value["state_directory"],),
            "E_CONFIG_SCHEMA", "维护记录必须位于同一部署状态目录")
    return HostConfig(**dict(value, services=tuple(value["services"]),
                             maintenance=MaintenanceConfig(**dict(m, allowed_site_parents=tuple(m["allowed_site_parents"])))))


def load_config(path=None):
    path = Path(path) if path is not None else Path(__file__).with_name(CONFIG_NAME)
    try:
        return parse_config(json.loads(read_regular(path)))
    except (ValueError, UnicodeDecodeError) as error:
        raise DeployError("E_CONFIG_SCHEMA", "现场配置不是有效 JSON") from error
