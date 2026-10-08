#!/bin/sh
set -eu

# 本机只负责读取 Git 合同与调用现有 OpenSSH；部署状态由远端生命周期管理。
SCRIPT_DIR=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
exec python3 "$SCRIPT_DIR/deploy_hostdzire.py" "$@"
