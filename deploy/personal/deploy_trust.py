"""导入前的工具字节信任边界；本模块启动时仅依赖 Python 标准库。"""
from dataclasses import dataclass
import hashlib
import json
from pathlib import Path
import re
import shlex
import stat
import subprocess

BUNDLE_FILES = (
    "deploy_hostdzire.py", "deploy-hostdzire.sh", "deploy_common.py", "deploy_config.py",
    "hostdzire.json", "deploy_compatibility.py", "deploy_git.py", "deploy_process.py",
    "deploy_images.py", "deploy_database.py", "deploy_compose.py", "deploy_health.py",
    "deploy_lifecycle.py", "deploy_transport.py", "deploy_local.py", "deploy_trust.py",
    "maintenance_hostdzire.py",
)
PREFIX = "deploy/personal/"
ORIGINS = ("git@github.com:ccisnoxx/sub2api.git", "https://github.com/ccisnoxx/sub2api.git", "https://github.com/ccisnoxx/sub2api")


class ToolTrustError(Exception):
    code = "E_TOOL_TRUST"


def check(value, message):
    if not value:
        raise ToolTrustError(message)


def git(root, *args):
    result = subprocess.run(["git", "-C", str(root), *args], capture_output=True)
    check(result.returncode == 0, "无法核验可信 main 工具 Git 对象")
    return result.stdout


def regular(path):
    check(path.is_file() and not path.is_symlink(), "工具模块或配置缺失或为符号链接")
    return path.read_bytes()


def manifest(directory):
    return {name: hashlib.sha256(regular(Path(directory) / name)).hexdigest() for name in BUNDLE_FILES}


def verify_manifest(directory, expected, *, exclusive=False):
    check(type(expected) is dict and set(expected) == set(BUNDLE_FILES) and
          all(type(value) is str and re.fullmatch(r"[0-9a-f]{64}", value) for value in expected.values()),
          "工具 bundle 清单字段或摘要无效")
    check(manifest(directory) == expected, "工具模块或配置字节发生漂移")
    if exclusive:
        check({path.name for path in Path(directory).iterdir()} == set(BUNDLE_FILES), "远端工具 scratch 含未绑定文件")
    else:
        allowed = set(BUNDLE_FILES) | {"README.md", ".gitignore", "__pycache__"}
        check(all(path.name in allowed or (path.name.startswith("test_") and path.name.endswith(".py"))
                  for path in Path(directory).iterdir()), "本机工具目录含未绑定的可导入文件或目录")
    return True


@dataclass(frozen=True)
class ToolBundle:
    directory: Path
    revision: str
    files: dict

    @classmethod
    def from_git(cls, root, directory, revision):
        check(type(revision) is str and re.fullmatch(r"[0-9a-f]{40}", revision), "必须显式指定可信 main 完整 --tool-revision")
        check(git(root, "remote", "get-url", "origin").decode().strip() in ORIGINS, "可信工具 origin 必须是本 fork")
        check(git(root, "rev-parse", "refs/remotes/origin/main^{commit}").decode().strip() == revision,
              "工具 revision 必须等于已获取的本 fork origin/main")
        expected = {}
        for name in BUNDLE_FILES:
            path = PREFIX + name
            entry = git(root, "ls-tree", revision, "--", path).decode().strip().split()
            check(len(entry) == 4 and entry[0] in ("100644", "100755") and entry[1] == "blob", "可信 main 缺少普通工具文件")
            file = Path(directory) / name
            regular(file)
            check(bool(file.stat().st_mode & stat.S_IXUSR) == (entry[0] == "100755"), "本机工具文件权限漂移")
            expected[name] = hashlib.sha256(git(root, "show", revision + ":" + path)).hexdigest()
        for scope in ([], ["--cached"]):
            result = subprocess.run(["git", "-C", str(root), "diff", *scope, "--quiet", revision, "--",
                                     *[PREFIX + name for name in BUNDLE_FILES]], capture_output=True)
            check(result.returncode == 0, "本机工具存在暂存或工作树漂移")
        verify_manifest(directory, expected)
        return cls(Path(directory), revision, expected)

    def verify(self):
        return verify_manifest(self.directory, self.files)

    def bootstrap(self, mode, record_id=None):
        """SSH 命令本身携带已验证清单；远端先查字节，再添加唯一导入目录。"""
        check(mode in ("--remote-probe", "--remote-apply", "--remote-dry-run"), "远端操作无效")
        check(record_id is None or re.fullmatch(r"[0-9]{8}T[0-9]{6}Z-[0-9a-f]{12}", record_id), "部署记录 ID 无效")
        arguments = [mode]
        if record_id:
            arguments.extend(("--record", record_id))
        return self._bootstrap("deploy_hostdzire.py", arguments)

    def maintenance_bootstrap(self, action, target_revision=None, deployment_id=None):
        check(action in ("freeze", "resume"), "可信维护入口只接受 freeze 或 resume")
        if action == "resume":
            check(type(target_revision) is str and re.fullmatch(r"[0-9a-f]{40}", target_revision) and
                  type(deployment_id) is str and re.fullmatch(r"[0-9]{8}T[0-9]{6}Z-[0-9a-f]{12}", deployment_id),
                  "OBS/RESP 恢复必须显式绑定完整目标 SHA 和成功部署记录 ID")
            arguments = [action, "--target-revision", target_revision, "--deployment-record", deployment_id]
        else:
            check(target_revision is None and deployment_id is None, "freeze 不接受恢复目标参数")
            arguments = [action]
        return self._bootstrap("maintenance_hostdzire.py", arguments)

    def _bootstrap(self, entry, arguments):
        program = '''import hashlib,json,pathlib,runpy,sys
p=pathlib.Path(sys.argv[1]); a=json.loads(sys.argv[2])
try:
 if set(x.name for x in p.iterdir()) != set(a['files']): raise ValueError()
 for name,digest in a['files'].items():
  f=p/name
  if not f.is_file() or f.is_symlink() or hashlib.sha256(f.read_bytes()).hexdigest()!=digest: raise ValueError()
except Exception:
 print(json.dumps({'status':'failed','error_code':'E_TOOL_TRUST','message':'远端工具 bundle 校验失败，未导入业务模块'}));sys.exit(2)
sys.path.insert(0,str(p)); entry=sys.argv[3];sys.argv=[str(p/entry)]+sys.argv[4:]
runpy.run_path(str(p/entry),run_name='__main__',init_globals={'__tool_attestation__':a})'''
        args = ["python3", "-I", "-S", "-B", "-c", program, str(self.directory),
                json.dumps({"revision": self.revision, "files": self.files}, sort_keys=True), entry, *arguments]
        return shlex.join(args)


def verify_request_bundle(request, trusted_revision=None):
    directory = Path(__file__).parent
    check(type(request.get("tool_revision")) is str and re.fullmatch(r"[0-9a-f]{40}", request["tool_revision"]), "工具 revision 无效")
    check(trusted_revision is None or request["tool_revision"] == trusted_revision, "请求工具 revision 与导入前信任绑定不符")
    verify_manifest(directory, request.get("tool_bundle"))
    check(request.get("tool_sha256") == request["tool_bundle"]["deploy_hostdzire.py"], "请求入口摘要与完整 bundle 不符")
    return True
