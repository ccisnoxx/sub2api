"""迁移账本的只读验证与显式备份；不执行 SQL 迁移或数据恢复。"""
import gzip
import tarfile
from deploy_common import *
from deploy_compatibility import CUMULATIVE_MIGRATIONS, KIN_MIGRATIONS, KIN_OLD_MIGRATION_STATE

def migration_fingerprint(listing):
    return {"rows": len(listing.splitlines()), "filename_checksum_sha256": sha(listing)}


def cumulative_ledger_lines(listing):
    lines = listing.splitlines(keepends=True)
    names = []
    for line in lines:
        match = re.fullmatch(rb"([^|\r\n]+)\|[0-9a-f]{64}\n", line)
        require(match is not None, "E_MIGRATION", "累计升级要求可精确核对的迁移账本格式")
        names.append(match[1].decode())
    require(lines and len(names) == len(set(names)), "E_MIGRATION", "累计升级迁移账本为空或有重复文件名")
    return lines


def cumulative_ledger_prefix(baseline, observed):
    """原行逐字保留，不用 Python 排序冒充数据库实际 collation。"""
    approved_names = {name.encode() for name in CUMULATIVE_MIGRATIONS}
    require(not any(line.split(b"|", 1)[0] in approved_names for line in cumulative_ledger_lines(baseline)),
            "E_MIGRATION", "累计升级原账本不能包含本次迁移")
    lines = cumulative_ledger_lines(observed)
    additions = [line for line in lines if line.split(b"|", 1)[0] in approved_names]
    retained = b"".join(line for line in lines if line.split(b"|", 1)[0] not in approved_names)
    expected = [(name + "|" + checksum + "\n").encode() for name, checksum in sorted(CUMULATIVE_MIGRATIONS.items())]
    require(retained == baseline and additions == expected[:len(additions)],
            "E_MIGRATION", "迁移差异不是原账本与固定 251–254 的已提交前缀，禁止旧镜像恢复")
    return len(additions)


def cumulative_ledger_baseline(observed):
    approved_names = {name.encode() for name in CUMULATIVE_MIGRATIONS}
    baseline = b"".join(line for line in observed.splitlines(keepends=True)
                        if line.split(b"|", 1)[0] not in approved_names)
    require(cumulative_ledger_prefix(baseline, observed) == len(CUMULATIVE_MIGRATIONS),
            "E_MIGRATION", "显式回滚要求全部固定新增迁移，不能有未知差异")
    return baseline


def kin_ledger_baseline(observed, complete):
    """保留原数据库顺序；只接受固定 307 行和唯一的正确 242。"""
    lines = cumulative_ledger_lines(observed)
    additions = [line for line in lines if line.split(b"|", 1)[0].decode() in KIN_MIGRATIONS]
    baseline = b"".join(line for line in lines if line.split(b"|", 1)[0].decode() not in KIN_MIGRATIONS)
    expected = [(name + "|" + checksum + "\n").encode() for name, checksum in KIN_MIGRATIONS.items()]
    require(migration_fingerprint(baseline) == KIN_OLD_MIGRATION_STATE and
            additions == (expected if complete else []) and
            all((name + "|" + checksum + "\n").encode() in lines
                for name, checksum in CUMULATIVE_MIGRATIONS.items()),
            "E_MIGRATION", "新 KIN 账本必须保留固定原 307 行和 251–254，且只追加正确 242")
    return baseline


class DatabaseOperations:
    def __init__(self, runner, config):
        self.runner = runner
        self.config = config

    def migration_listing(self, postgres_id):
        listing = self.runner.run(["docker", "exec", postgres_id, "sh", "-ec",
                                   'psql -U "$POSTGRES_USER" -d "$POSTGRES_DB" -Atqc "SELECT filename, checksum FROM schema_migrations ORDER BY filename, checksum"'],
                                  sensitive=True)
        require(listing.strip(), "E_MIGRATION", "迁移记录为空或不可读取，禁止猜测旧版本兼容性")
        return listing

    def migration_state(self, postgres_id):
        return migration_fingerprint(self.migration_listing(postgres_id))

    def backup(self, directory, config_before, env_before, selection_before, dependencies):
        for name, content in (("docker-compose.yml", config_before), ("environment.env", env_before)):
            atomic_private(directory / name, content)
        if selection_before is not None:
            atomic_private(directory / "previous-image.env", selection_before)
        dump = directory / "postgres.dump"
        self.runner.run(["docker", "exec", dependencies["postgres"], "sh", "-ec",
                         'pg_dump -U "$POSTGRES_USER" -d "$POSTGRES_DB" --format=custom'], stdout_path=dump, timeout=900)
        require(dump.stat().st_size > 0, "E_BACKUP", "数据库备份为空")
        listing = self.runner.run(["docker", "exec", "-i", dependencies["postgres"], "pg_restore", "--list"],
                                  stdin_path=dump, timeout=900)
        require(listing.strip(), "E_BACKUP", "数据库备份未通过 pg_restore --list")
        archive = directory / "app-data.tar.gz"
        self.runner.run(["docker", "exec", self.config.app_container, "tar", "-C", self.config.app_data_path, "-czf", "-", "."], stdout_path=archive, timeout=900)
        try:
            with tarfile.open(archive, "r:gz") as data:
                data.getmembers()
            # tar 的结束块可能早于 gzip 尾部；读至 EOF 才能验证 CRC 和长度。
            with gzip.open(archive, "rb") as data:
                for _ in iter(lambda: data.read(1024 * 1024), b""):
                    pass
        except (tarfile.TarError, EOFError, OSError) as error:
            raise DeployError("E_BACKUP", "应用数据归档校验失败") from error
        return {"postgres": {"file": "postgres.dump", "sha256": file_sha(dump), "bytes": dump.stat().st_size},
                                        "app_data": {"file": "app-data.tar.gz", "sha256": file_sha(archive), "bytes": archive.stat().st_size},
                                        "configuration": "docker-compose.yml", "environment": "environment.env"}
