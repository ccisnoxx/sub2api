#!/usr/bin/env bash
set -euo pipefail
task_dir="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
mkdir -p "$task_dir/tls"
if [[ -e "$task_dir/tls/fixture.key" || -e "$task_dir/tls/fixture.crt" ]]; then
  echo '已有测试证书；不覆盖。需要新证书时先单独归档旧文件。' >&2
  exit 2
fi
umask 077
openssl req -x509 -newkey rsa:2048 -nodes -days 7 \
  -keyout "$task_dir/tls/fixture.key" -out "$task_dir/tls/fixture.crt" \
  -subj '/CN=chatgpt.com' \
  -addext 'subjectAltName=DNS:chatgpt.com,DNS:s44-ws-fixture,IP:127.0.0.1' \
  -addext 'basicConstraints=critical,CA:TRUE' \
  > /dev/null 2>&1
# 容器以非 root 运行；只有此一次性隔离测试目录中的 fake TLS 私钥放开容器读取。
chmod 644 "$task_dir/tls/fixture.key" "$task_dir/tls/fixture.crt"
