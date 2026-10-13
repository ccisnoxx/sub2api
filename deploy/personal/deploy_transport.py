"""现有 OpenSSH 别名运输完整可信 bundle；scratch 与部署状态完全分开。"""
import json
from pathlib import Path
import re
import shlex
import subprocess
import sys
from deploy_common import *
from deploy_trust import ToolBundle


class SSHTransport:
    def __init__(self, bundle, config):
        require(isinstance(bundle, ToolBundle), "E_TOOL_TRUST", "运输必须提供已验证的完整工具 bundle")
        self.bundle = bundle
        self.config = config
        self.directory = None

    def command(self, args, data=None):
        result = subprocess.run(args, input=data, capture_output=True)
        require(result.returncode == 0, "E_SSH", "OpenSSH 操作失败；未改变 SSH 配置，远端状态须重新核对")
        return result.stdout

    def __enter__(self):
        self.bundle.verify()
        template = self.config.scratch_template
        directory = self.command(["ssh", self.config.ssh_alias, "umask 077; mktemp -d " + shlex.quote(template)]).decode().strip()
        pattern = re.escape(template[:-12]) + r"[A-Za-z0-9]{12}"
        require(re.fullmatch(pattern, directory), "E_SSH", "远端临时目录格式无效")
        self.directory = directory
        try:
            self.command(["scp", *[str(self.bundle.directory / name) for name in self.bundle.files],
                          self.config.ssh_alias + ":" + directory + "/"])
        except Exception:
            self.__exit__(None, None, None)
            raise
        return self

    def __exit__(self, *_):
        if self.directory:
            try:
                self.command(["ssh", self.config.ssh_alias, "rm -rf -- " + shlex.quote(self.directory)])
            except DeployError:
                print("远端临时工具目录清理失败；部署结果仍以服务器记录为准。", file=sys.stderr)

    def remote_command(self, mode, record_id=None):
        self.bundle.verify()
        remote = ToolBundle(Path(self.directory), self.bundle.revision, self.bundle.files)
        return ["ssh", self.config.ssh_alias, remote.bootstrap(mode, record_id)]

    def probe(self, record_id=None):
        return json.loads(self.command(self.remote_command("--remote-probe", record_id)))

    def maintenance(self, action, target_revision=None, deployment_id=None):
        self.bundle.verify()
        remote = ToolBundle(Path(self.directory), self.bundle.revision, self.bundle.files)
        result = subprocess.run(["ssh", self.config.ssh_alias,
                                remote.maintenance_bootstrap(action, target_revision, deployment_id)], capture_output=True)
        try:
            response = json.loads(result.stdout)
        except (ValueError, UnicodeDecodeError) as error:
            raise DeployError("E_SSH", "未收到有效维护结果；请核对持久标记与维护记录，不自动重试") from error
        require(result.returncode == 0 and response.get("action") == action and
                response.get("phase") == ("closed" if action == "freeze" else "open") and
                response.get("maintenance_flag_present") is (action == "freeze") and
                (action != "resume" or (response.get("accepted_source_sha") == target_revision and
                                        response.get("accepted_deployment_id") == deployment_id)),
                "E_MAINTENANCE", "维护未确认完成；请核对持久标记与私密记录")
        return response

    def execute(self, request, *, dry_run=False):
        mode = "--remote-dry-run" if dry_run else "--remote-apply"
        result = subprocess.run(self.remote_command(mode), input=canonical(request), capture_output=True)
        try:
            response = json.loads(result.stdout)
        except (ValueError, UnicodeDecodeError) as error:
            raise DeployError("E_SSH", "未收到有效远端结果；不要重试写入，请先核对部署记录与运行镜像") from error
        successes = ("dry-run",) if dry_run else ("success",)
        require(result.returncode in (0, 1, 2) and response.get("status") in (*successes, "failed"), "E_SSH", "远端结果不完整")
        require((response["status"] in successes) == (result.returncode == 0), "E_SSH", "远端结果与退出码不一致")
        return response, result.returncode
