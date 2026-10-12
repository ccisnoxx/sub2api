"""远端子进程边界；私密输出只进入操作诊断。"""
import os
import subprocess
from deploy_common import *

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
