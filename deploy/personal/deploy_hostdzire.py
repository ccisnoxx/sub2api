#!/usr/bin/env python3
"""参数解析与编排；业务导入前先闭合 main 工具字节信任边界。"""
import argparse
import json
from pathlib import Path
import signal
import re
import subprocess
import sys
import types


def trusted_runtime(args):
    attestation = globals().get("__tool_attestation__")
    directory = Path(__file__).resolve().parent
    if attestation is not None:
        sys.pycache_prefix = "/dev/null"
        from deploy_trust import ToolBundle
        return ToolBundle(directory, attestation["revision"], attestation["files"])
    if args.remote_probe or args.remote_apply or args.remote_dry_run:
        raise RuntimeError("远端入口必须经完整工具 bootstrap 验证")
    if not sys.flags.isolated or not args.tool_revision:
        raise RuntimeError("请使用 shell 入口并显式指定可信 main 完整 --tool-revision")
    root = directory.parents[1]
    if re.fullmatch(r"[0-9a-f]{40}", args.tool_revision) is None:
        raise RuntimeError("工具 revision 必须为完整 SHA")
    origin = subprocess.run(["git", "-C", str(root), "remote", "get-url", "origin"], capture_output=True)
    main = subprocess.run(["git", "-C", str(root), "rev-parse", "refs/remotes/origin/main^{commit}"], capture_output=True)
    if origin.returncode or main.returncode or origin.stdout.decode().strip() not in (
            "git@github.com:ccisnoxx/sub2api.git", "https://github.com/ccisnoxx/sub2api.git", "https://github.com/ccisnoxx/sub2api") or main.stdout.decode().strip() != args.tool_revision:
        raise RuntimeError("导入前信任锚必须是本 fork 的 origin/main 完整 SHA")
    # trust 源码来自固定 Git blob；不能先导入工作树中的未验证模块。
    result = subprocess.run(["git", "-C", str(root), "show", args.tool_revision + ":deploy/personal/deploy_trust.py"], capture_output=True)
    if result.returncode != 0:
        raise RuntimeError("可信 main 不含完整工具信任模块")
    module = types.ModuleType("deploy_trust")
    module.__file__ = str(directory / "deploy_trust.py")
    sys.modules[module.__name__] = module
    exec(compile(result.stdout, module.__file__, "exec"), module.__dict__)
    bundle = module.ToolBundle.from_git(root, directory, args.tool_revision)
    # -B 禁止生成 pyc；前缀指向非目录同时禁止载入工作树中的既有 pyc。
    sys.pycache_prefix = "/dev/null"
    sys.path.insert(0, str(directory))
    return bundle


def main():
    parser = argparse.ArgumentParser(description="通过配置中的现有 SSH 别名部署个人版本；digest 需 --version。")
    parser.add_argument("target", nargs="?", help="个人版本、fork 版本地址或 fork@sha256:digest")
    parser.add_argument("--version", help="固定 digest 所对应的个人版本")
    parser.add_argument("--rollback", metavar="记录ID", help="只回滚当前成功部署记录")
    parser.add_argument("--tool-revision", help="已经获取的本 fork origin/main 完整可信工具 SHA")
    parser.add_argument("--dry-run", action="store_true", help="只读核验现场、目标和兼容；不会应用部署")
    parser.add_argument("--maintenance", choices=("freeze", "resume"), help="通过可信完整 bundle 冻结流量或恢复已成功部署的审定应用")
    parser.add_argument("--target-revision", help="维护恢复所绑定的完整发布源码 SHA")
    parser.add_argument("--deployment-record", help="维护恢复所绑定的当前成功部署记录 ID")
    remote = parser.add_mutually_exclusive_group()
    for name in ("--remote-probe", "--remote-apply", "--remote-dry-run"):
        remote.add_argument(name, action="store_true", help=argparse.SUPPRESS)
    parser.add_argument("--record", help=argparse.SUPPRESS)
    args = parser.parse_args()
    try:
        try:
            bundle = trusted_runtime(args)
        except Exception as error:
            print(json.dumps({"status": "failed", "error_code": "E_TOOL_TRUST", "message": str(error)}, ensure_ascii=False))
            return 2
        from deploy_config import load_config
        from deploy_common import DeployError, require
        from deploy_lifecycle import Deployment
        from deploy_local import local_main, local_maintenance
        config = load_config()
        require(not (args.remote_probe or args.remote_apply or args.remote_dry_run) or
                (args.maintenance is None and args.target_revision is None and args.deployment_record is None),
                "E_INPUT", "远端部署入口不能混用维护参数")
        if args.remote_probe:
            print(json.dumps(Deployment(config=config, trusted_revision=bundle.revision).probe(args.record), ensure_ascii=False))
            return 0
        if args.remote_apply or args.remote_dry_run:
            def interrupted(_signum, _frame):
                raise DeployError("E_INTERRUPTED", "操作被中断，保存失败并尝试适用的回滚")
            signal.signal(signal.SIGTERM, interrupted)
            signal.signal(signal.SIGINT, interrupted)
            deployment = Deployment(config=config, trusted_revision=bundle.revision)
            request = json.load(sys.stdin)
            response, code = deployment.dry_run(request) if args.remote_dry_run else deployment.execute(request)
            print(json.dumps(response, ensure_ascii=False))
            return code
        if args.maintenance is not None:
            return local_maintenance(args, bundle, config)
        require(args.target_revision is None and args.deployment_record is None, "E_INPUT", "恢复目标参数只能用于维护操作")
        require(args.target is not None or args.rollback is not None, "E_INPUT", "必须指定个人版本、digest 或 --rollback 记录ID")
        return local_main(args, bundle, config)
    except Exception as error:
        code = getattr(error, "code", "E_INTERNAL")
        message = str(error) if code != "E_INTERNAL" else "输入、环境或记录异常；未确认完成，请核对私密部署记录"
        print(json.dumps({"status": "failed", "error_code": code, "message": message}, ensure_ascii=False))
        return 2


# 原部署测试与已有只读调用者的私有 import seam；执行 CLI 不使用此路径。
if __name__ != "__main__":
    from deploy_common import *
    from deploy_compatibility import *
    from deploy_database import *
    from deploy_git import GitEvidence
    from deploy_lifecycle import Deployment
    from deploy_process import Runner
    from deploy_transport import SSHTransport
    from deploy_config import load_config
    _config = load_config()
    BASE, PROJECT, APP, VOLUME = Path(_config.base), _config.project, _config.app_container, _config.app_volume

if __name__ == "__main__":
    sys.exit(main())
