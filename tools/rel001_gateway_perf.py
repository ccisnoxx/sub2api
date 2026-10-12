#!/usr/bin/env python3
"""REL-001：在隔离 Docker 环境比较两个真实 linux/amd64 网关镜像。

只使用 Python 标准库及 Docker CLI；所有镜像须预先加载到所选 daemon。
正式运行示例（new-image 必须来自 Release dry-run 的 OCI，而非本脚本构建）：
  python3 tools/rel001_gateway_perf.py --new-image sha256:<64 hex> \
      --new-sha <40 hex> --output /private/path/comparison.json
旧/旧低负载连通性检查必须显式使用 --smoke --rounds 1 --requests 4 --warmup 2。
--smoke 的 PASS 只证明工具与 HTTP 合同，不构成 REL-001 性能验收。

每个样本重建空 PG、Redis 和应用；两个版本从不共享 Redis 或运行窗口。
请求走真实 /v1/responses 的 JSON 与 SSE，核对逐请求内容、usage 和终态；
随后注入上游 400 并确认错误仍为错误及下一请求恢复。不会调用真实付费上游。
stdout/结果只输出白名单身份和匿名统计；失败日志留在权限 0700 的临时目录。
"""

from __future__ import annotations

import argparse
import collections
import concurrent.futures
import hashlib
import http.client
import http.server
import json
import math
import os
import platform
import re
import secrets
import shutil
import signal
import statistics
import subprocess
import sys
import tempfile
import threading
import time
from pathlib import Path


OLD_DIGEST = "sha256:7446ff8ebdddca5e60989f0a5b8dec670ce3a030a9c8472e2dd3c27986e21530"
OLD_IMAGE = "ghcr.io/ccisnoxx/sub2api@" + OLD_DIGEST
OLD_SHA = "3d5e1fde82707900a21f2b5538b112c7bab3c04a"
MODEL = "gpt-4.1"
LIMIT = 1024 * 1024
IMAGE_RE = re.compile(r"(?:sha256:[0-9a-f]{64}|[a-zA-Z0-9./:_-]+@sha256:[0-9a-f]{64})\Z")
SHA_RE = re.compile(r"[0-9a-f]{40}\Z")
MARKER_RE = re.compile(r"rel001-(?:warmup|measure|recovery)-[a-z0-9-]+")
IMAGE_FORMAT = ('{"id":{{json .Id}},"repo_digests":{{json .RepoDigests}},'
                '"os":{{json .Os}},"architecture":{{json .Architecture}},'
                '"revision":{{with index .Config "Labels"}}{{json (index . "org.opencontainers.image.revision")}}'
                '{{else}}null{{end}}}')


class Failure(Exception):
    """可公开的固定错误码，不包含底层响应、命令或凭据。"""


def require(condition: bool, code: str) -> None:
    if not condition:
        raise Failure(code)


def mock_server() -> None:
    """同一份脚本在合成上游容器内运行，永不记录请求正文或凭据。"""
    counts: collections.Counter = collections.Counter()
    lock = threading.Lock()

    class Handler(http.server.BaseHTTPRequestHandler):
        protocol_version = "HTTP/1.1"

        def log_message(self, *_args):
            pass

        def reply(self, status, body, content_type="application/json"):
            encoded = body if isinstance(body, bytes) else json.dumps(body).encode()
            self.send_response(status)
            self.send_header("Content-Type", content_type)
            self.send_header("Content-Length", str(len(encoded)))
            self.end_headers()
            self.wfile.write(encoded)
            self.wfile.flush()

        def do_GET(self):
            if self.path == "/stats":
                with lock:
                    result = dict(counts)
                self.reply(200, result)
            elif self.path == "/health":
                self.reply(200, {"ok": True})
            else:
                self.reply(404, {"error": {"type": "invalid_request_error"}})

        def do_POST(self):
            length = int(self.headers.get("Content-Length", "0"))
            if length <= 0 or length > LIMIT:
                self.reply(400, {"error": {"type": "invalid_request_error"}})
                return
            body = json.loads(self.rfile.read(length))
            if self.path != "/v1/responses":
                with lock:
                    counts["unexpected_path"] += 1
                self.reply(404, {"error": {"type": "invalid_request_error"}})
                return
            text = json.dumps(body)
            if "rel001-controlled-error" in text:
                with lock:
                    counts["controlled_400"] += 1
                self.reply(400, {"error": {"type": "invalid_request_error",
                    "code": "rel001_invalid_request", "message": "synthetic validation error",
                    "param": "input"}})
                return
            match = MARKER_RE.search(text)
            probe = body.get("tool_choice") == "required" and not match
            if not match and not probe:
                with lock:
                    counts["unexpected_payload"] += 1
                self.reply(400, {"error": {"type": "invalid_request_error"}})
                return
            marker = match.group() if match else "probe"
            suffix = hashlib.sha256(marker.encode()).hexdigest()[:20]
            item = {"id": "msg_" + suffix, "type": "message", "status": "completed",
                    "role": "assistant", "content": [{"type": "output_text", "text": marker,
                                                         "annotations": []}]}
            if probe:
                item = {"id": "fc_" + suffix, "type": "function_call", "status": "completed",
                        "call_id": "call_" + suffix, "name": "probe_ping", "arguments": '{"ok":true}'}
            response = {"id": "resp_" + suffix, "object": "response", "created_at": 1700000000,
                        "status": "completed", "error": None, "incomplete_details": None,
                        "model": MODEL, "output": [item],
                        "usage": {"input_tokens": 12, "output_tokens": 8, "total_tokens": 20,
                                  "input_tokens_details": {"cached_tokens": 0},
                                  "output_tokens_details": {"reasoning_tokens": 0}}}
            streaming = bool(body.get("stream"))
            with lock:
                counts["probe" if probe else ("sse" if streaming else "json")] += 1
            if not streaming:
                self.reply(200, response)
                return
            initial = dict(response, status="in_progress", output=[], usage=None)
            part = {"type": "output_text", "text": "", "annotations": []}
            events = [
                {"type": "response.created", "response": initial},
                {"type": "response.in_progress", "response": initial},
                {"type": "response.output_item.added", "output_index": 0,
                 "item": dict(item, status="in_progress", content=[])},
                {"type": "response.content_part.added", "item_id": item["id"], "output_index": 0,
                 "content_index": 0, "part": part},
                {"type": "response.output_text.delta", "item_id": item["id"], "output_index": 0,
                 "content_index": 0, "delta": marker},
                {"type": "response.output_text.done", "item_id": item["id"], "output_index": 0,
                 "content_index": 0, "text": marker},
                {"type": "response.content_part.done", "item_id": item["id"], "output_index": 0,
                 "content_index": 0, "part": item["content"][0]},
                {"type": "response.output_item.done", "output_index": 0, "item": item},
                {"type": "response.completed", "response": response},
            ]
            wire = b"".join(("event: " + event["type"] + "\ndata: " +
                json.dumps(dict(event, sequence_number=i)) + "\n\n").encode()
                for i, event in enumerate(events))
            self.reply(200, wire, "text/event-stream")

    http.server.ThreadingHTTPServer(("0.0.0.0", 8090), Handler).serve_forever()


def http_request(port, method, path, payload=None, token=None, timeout=20):
    connection = http.client.HTTPConnection("127.0.0.1", port, timeout=timeout)
    try:
        headers = {"Content-Type": "application/json", "Connection": "close"}
        if token:
            headers["Authorization"] = "Bearer " + token
        data = json.dumps(payload).encode() if payload is not None else None
        connection.request(method, path, data, headers)
        response = connection.getresponse()
        raw = response.read(LIMIT + 1)
        require(len(raw) <= LIMIT, "http_response_too_large")
        return response.status, response.getheader("Content-Type", ""), raw
    finally:
        connection.close()


def api(port, path, payload=None, token=None, method="POST"):
    status, _content_type, raw = http_request(port, method, path, payload, token)
    require(status in (200, 201), "fixture_api_http_" + str(status) + "_" + path.replace("/", "_"))
    data = json.loads(raw)
    require(data.get("code") == 0 and isinstance(data.get("data"), dict), "fixture_api_contract")
    return data["data"]


def validate_response(response, marker):
    require(isinstance(response, dict) and response.get("object") == "response", "invalid_response_object")
    require(response.get("status") == "completed" and not response.get("error"), "missing_completed_status")
    require(response.get("model") == MODEL, "wrong_response_model")
    output = response.get("output", [])
    require(isinstance(output, list), "invalid_response_output")
    text = "".join(part.get("text", "") for item in output if item.get("type") == "message"
                   for part in item.get("content", []) if part.get("type") == "output_text")
    require(text == marker, "wrong_response_content")
    usage = response.get("usage", {})
    require(isinstance(usage, dict) and all(type(usage.get(key)) is int and usage[key] == value
        for key, value in (("input_tokens", 12), ("output_tokens", 8), ("total_tokens", 20))),
        "wrong_response_usage")


def validate_sse(raw, marker):
    completed = []
    deltas = []
    for frame in raw.decode("utf-8").replace("\r\n", "\n").split("\n\n"):
        lines = [line[5:].lstrip() for line in frame.splitlines() if line.startswith("data:")]
        if not lines:
            continue
        payload = "\n".join(lines)
        if payload == "[DONE]":
            continue
        event = json.loads(payload)
        kind = event.get("type")
        require(kind not in ("error", "response.failed", "response.incomplete"), "sse_error_event")
        if kind == "response.output_text.delta":
            deltas.append(event.get("delta", ""))
        if kind == "response.completed":
            completed.append(event.get("response"))
    require(len(completed) == 1, "missing_sse_completed_event")
    require("".join(deltas) == marker, "wrong_sse_delta_content")
    validate_response(completed[0], marker)


def gateway_request(port, key, mode, marker, timeout):
    start = time.perf_counter()
    status = None
    reason = None
    try:
        status, content_type, raw = http_request(port, "POST", "/v1/responses", {
            "model": MODEL, "input": [{"role": "user", "content": marker}],
            "instructions": "Return the synthetic input marker.", "stream": mode == "sse",
            "max_output_tokens": 32, "store": False}, key, timeout)
        require(status == 200, "http_" + str(status))
        if mode == "sse":
            require(content_type.split(";")[0] == "text/event-stream", "wrong_sse_content_type")
            validate_sse(raw, marker)
        else:
            require(content_type.split(";")[0] == "application/json", "wrong_json_content_type")
            validate_response(json.loads(raw), marker)
    except Failure as error:
        reason = str(error)
    except (OSError, http.client.HTTPException):
        reason = "transport_error"
    except (ValueError, TypeError, KeyError, AttributeError):
        reason = "malformed_response"
    return {"latency_ms": (time.perf_counter() - start) * 1000, "status": status, "error": reason}


def percentile(values, percent):
    ordered = sorted(values)
    return ordered[max(0, math.ceil(len(ordered) * percent / 100) - 1)]


def load(port, key, mode, count, concurrency, phase, timeout):
    start = time.perf_counter()
    with concurrent.futures.ThreadPoolExecutor(max_workers=concurrency) as executor:
        results = list(executor.map(lambda i: gateway_request(port, key, mode,
            "rel001-" + phase + "-" + mode + "-" + str(i), timeout), range(count)))
    elapsed = time.perf_counter() - start
    errors = collections.Counter(result["error"] for result in results if result["error"])
    latencies = [result["latency_ms"] for result in results]
    successes = count - sum(errors.values())
    return {"requests": count, "successful": successes, "unexpected_errors": count - successes,
            "error_rate": (count - successes) / count,
            "error_counts": dict(errors),
            "http_status_counts": dict(collections.Counter(str(result["status"])
                if result["status"] is not None else "transport" for result in results)),
            "elapsed_seconds": elapsed, "throughput_rps": successes / elapsed,
            "latency_ms": {"p50": statistics.median(latencies), "p95": percentile(latencies, 95),
                           "max": max(latencies)}}


class Docker:
    def __init__(self, host, directory):
        self.environment = dict(os.environ, DOCKER_HOST=host)
        # 显式 host 优先；调用者的全局 context 和 DOCKER_CONTEXT 均不被修改。
        self.environment.pop("DOCKER_CONTEXT", None)
        self.directory = directory

    def run(self, *args, timeout=30, check=True):
        try:
            result = subprocess.run(["docker", *args], env=self.environment, capture_output=True,
                                    text=True, timeout=timeout)
        except subprocess.TimeoutExpired:
            raise Failure("docker_command_timeout") from None
        except OSError:
            raise Failure("docker_cli_unavailable") from None
        if check and result.returncode:
            with (self.directory / "docker-errors.log").open("a") as log:
                log.write(result.stderr)
            raise Failure("docker_" + args[0] + "_failed")
        return result

    def image(self, reference, sha=None):
        if sha is not None:
            require(bool(IMAGE_RE.fullmatch(reference)), "application_image_must_be_immutable")
            require(bool(SHA_RE.fullmatch(sha)), "expected_sha_must_be_full_lowercase_hex")
        result = json.loads(self.run("image", "inspect", "--format", IMAGE_FORMAT, "--", reference).stdout)
        require(result["os"] == "linux", "image_must_be_linux")
        require(bool(re.fullmatch(r"sha256:[0-9a-f]{64}", result["id"])), "invalid_inspected_image_id")
        if sha is not None:
            require(result["architecture"] == "amd64", "application_image_must_be_amd64")
            require(result["revision"] == sha, "image_revision_mismatch")
            if "@" in reference:
                require(reference in (result["repo_digests"] or []), "requested_digest_not_inspected")
            else:
                require(reference == result["id"], "requested_image_id_mismatch")
        return result


class Stack:
    def __init__(self, docker, identity, dependencies, args):
        self.docker, self.identity, self.dependencies, self.args = docker, identity, dependencies, args
        self.name = "rel001-perf-" + secrets.token_hex(6)
        self.network = self.name + "-net"
        self.containers = []
        self.network_created = False
        self.secret_files = []
        self.port = None

    def env_file(self, values, suffix):
        path = self.docker.directory / (self.name + suffix + ".env")
        self.secret_files.append(path)
        path.touch(mode=0o600)
        path.write_text("".join(key + "=" + value + "\n" for key, value in values.items()))
        return str(path)

    def run(self, role, image, options=(), command=()):
        name = self.name + "-" + role
        # 先登记随机资源，run 部分失败时也执行清理。
        self.containers.append(name)
        self.docker.run("run", "-d", "--pull", "never", "--name", name,
            "--network", self.network, "--network-alias", role,
            "--label", "rel001.gateway-perf=" + self.name,
            "--log-opt", "max-size=5m", "--log-opt", "max-file=1", *options, image, *command)
        return name

    def wait_command(self, name, command):
        deadline = time.monotonic() + self.args.startup_timeout
        while time.monotonic() < deadline:
            if self.docker.run("exec", name, *command, check=False, timeout=10).returncode == 0:
                return
            running = self.docker.run("inspect", "--format", "{{.State.Running}}", name).stdout.strip()
            require(running == "true", "dependency_exited_before_ready")
            time.sleep(0.5)
        raise Failure("dependency_startup_timeout")

    def start(self):
        # Docker 29 的 internal 网络不发布端口；使用独立 bridge 与 loopback 绑定。
        # 数据库、缓存和 mock 不发布端口，账号只能指向该网络内的合成上游。
        self.network_created = True
        self.docker.run("network", "create", self.network)
        password = secrets.token_hex(24)
        pg = self.run("postgres", self.dependencies["postgres"]["id"], (
            "--env-file", self.env_file({"POSTGRES_USER": "postgres", "POSTGRES_DB": "rel001",
                "POSTGRES_PASSWORD": password, "PGDATA": "/var/lib/postgresql/data", "TZ": "UTC"}, "-pg")))
        redis = self.run("redis", self.dependencies["redis"]["id"], command=(
            "redis-server", "--save", "", "--appendonly", "no"))
        mock = self.run("mock", self.dependencies["mock"]["id"], (
            "--mount", "type=bind,source=" + str(Path(__file__).resolve()) + ",target=/mock.py,readonly",
            "--read-only", "--tmpfs", "/tmp"), ("python3", "-B", "/mock.py", "--mock-server"))
        self.wait_command(pg, ("pg_isready", "-U", "postgres", "-d", "rel001"))
        self.wait_command(redis, ("redis-cli", "ping"))
        self.wait_command(mock, ("python3", "-c", "import urllib.request; urllib.request.urlopen('http://127.0.0.1:8090/health').read()"))
        pg_version = self.docker.run("exec", pg, "psql", "-U", "postgres", "-d", "rel001", "-Atqc", "SHOW server_version").stdout.strip()
        redis_info = self.docker.run("exec", redis, "redis-cli", "INFO", "server").stdout
        redis_version = re.search(r"(?m)^redis_version:([0-9.]+)\r?$", redis_info)
        require(pg_version == "18.6", "postgres_runtime_must_be_18_6")
        require(bool(redis_version) and redis_version[1].startswith("8.4."), "redis_runtime_must_be_8_4")
        self.app = self.run("app", self.identity["id"], (
            "--platform", "linux/amd64", "--publish", "127.0.0.1::8080",
            "--env-file", self.env_file({
                "AUTO_SETUP": "true", "DATABASE_HOST": "postgres", "DATABASE_PORT": "5432",
                "DATABASE_USER": "postgres", "DATABASE_PASSWORD": password,
                "DATABASE_DBNAME": "rel001", "DATABASE_SSLMODE": "disable", "REDIS_HOST": "redis",
                "REDIS_PORT": "6379", "JWT_SECRET": secrets.token_hex(32),
                "TOTP_ENCRYPTION_KEY": secrets.token_hex(32), "TZ": "UTC",
                "ADMIN_EMAIL": "rel001-admin@example.invalid", "ADMIN_PASSWORD": password,
                "SECURITY_URL_ALLOWLIST_ENABLED": "false", "SECURITY_URL_ALLOWLIST_ALLOW_INSECURE_HTTP": "true",
                "SECURITY_URL_ALLOWLIST_ALLOW_PRIVATE_HOSTS": "true"}, "-app")))
        actual = json.loads(self.docker.run("inspect", "--format",
            '{"image_id":{{json .Image}},"ports":{{json (index .NetworkSettings.Ports "8080/tcp")}}}', self.app).stdout)
        require(actual["image_id"] == self.identity["id"], "running_container_image_mismatch")
        require(isinstance(actual["ports"], list) and len(actual["ports"]) == 1 and
                actual["ports"][0]["HostIp"] == "127.0.0.1", "unsafe_or_unavailable_app_port_binding")
        self.port = int(actual["ports"][0]["HostPort"])
        deadline = time.monotonic() + self.args.startup_timeout
        while time.monotonic() < deadline:
            try:
                status, _ctype, _raw = http_request(self.port, "GET", "/health", timeout=2)
                if status == 200:
                    break
            except (OSError, http.client.HTTPException):
                pass
            require(self.docker.run("inspect", "--format", "{{.State.Running}}", self.app).stdout.strip() == "true",
                    "application_exited_before_ready")
            time.sleep(0.5)
        else:
            raise Failure("application_startup_timeout")
        version = self.docker.run("exec", self.app, "/app/sub2api", "-version")
        match = re.search(r"Sub2API ([a-zA-Z0-9._+-]+) \(commit: ([0-9a-f]{7,40}),", version.stdout + version.stderr)
        require(bool(match) and self.identity["revision"].startswith(match[2]), "runtime_binary_commit_mismatch")
        self.runtime = {"image_id": actual["image_id"], "binary_version": match[1], "binary_commit": match[2],
                        "postgres_version": pg_version, "redis_version": redis_version[1]}
        admin = api(self.port, "/api/v1/auth/login", {"email": "rel001-admin@example.invalid", "password": password})["access_token"]
        # 合成管理员通过真实确认接口满足初始化合同，不绕过管理权限中间件。
        compliance = api(self.port, "/api/v1/admin/compliance", token=admin, method="GET")
        require(type(compliance.get("required")) is bool, "fixture_compliance_contract")
        if compliance["required"]:
            require(isinstance(compliance.get("ack_phrase_zh"), str), "fixture_compliance_phrase_missing")
            acknowledged = api(self.port, "/api/v1/admin/compliance/accept", {
                "phrase": compliance["ack_phrase_zh"], "language": "zh"}, admin)
            require(acknowledged.get("required") is False, "fixture_compliance_acknowledgement_failed")
        group = api(self.port, "/api/v1/admin/groups", {"name": "rel001-synthetic", "platform": "openai",
            "rate_multiplier": 1, "subscription_type": "standard", "is_exclusive": True}, admin)
        user = api(self.port, "/api/v1/admin/users", {"email": "rel001-user@example.invalid", "password": password,
            "role": "user", "balance": 1000, "concurrency": max(8, self.args.concurrency * 2),
            "allowed_groups": [group["id"]]}, admin)
        account = api(self.port, "/api/v1/admin/accounts", {"name": "rel001-synthetic", "platform": "openai",
            "type": "apikey", "credentials": {"api_key": "synthetic-" + secrets.token_hex(16),
                "base_url": "http://mock:8090", "model_mapping": {MODEL: MODEL}},
            "concurrency": max(8, self.args.concurrency * 2), "priority": 1, "group_ids": [group["id"]]}, admin)
        token = api(self.port, "/api/v1/auth/login", {"email": "rel001-user@example.invalid", "password": password})["access_token"]
        key = api(self.port, "/api/v1/keys", {"name": "rel001-synthetic", "group_id": group["id"]}, token)
        require(all(type(obj.get("id")) is int and obj["id"] > 0 for obj in (group, user, account, key)), "fixture_missing_identity")
        self.key = key["key"]
        require(isinstance(self.key, str) and len(self.key) >= 16, "fixture_missing_api_key")

    def stats(self):
        raw = self.docker.run("exec", self.name + "-mock", "python3", "-c",
            "import urllib.request; print(urllib.request.urlopen('http://127.0.0.1:8090/stats').read().decode())").stdout
        result = json.loads(raw)
        allowed = {"probe", "json", "sse", "controlled_400", "unexpected_path", "unexpected_payload"}
        require(set(result) <= allowed and all(type(value) is int and value >= 0 for value in result.values()), "invalid_mock_statistics")
        return result

    def controlled_error(self):
        before = self.stats()
        status, _content_type, raw = http_request(self.port, "POST", "/v1/responses", {
            "model": MODEL, "input": "rel001-controlled-error", "stream": False}, self.key, self.args.request_timeout)
        parsed = json.loads(raw)
        after = self.stats()
        require(status == 400 and isinstance(parsed.get("error"), dict), "upstream_error_became_success_or_wrong_status")
        require(after.get("controlled_400", 0) - before.get("controlled_400", 0) == 1, "controlled_error_not_reached_upstream")
        recovery = gateway_request(self.port, self.key, "json", "rel001-recovery-json-0", self.args.request_timeout)
        require(not recovery["error"], "success_after_controlled_error_failed")
        return {"upstream_status": 400, "gateway_status": status, "error_envelope_present": True, "subsequent_success": True}

    def close(self, failed):
        errors = []
        for name in reversed(self.containers):
            if failed:
                try:
                    log = self.docker.run("logs", name, timeout=15, check=False)
                    path = self.docker.directory / (name + ".log")
                    path.touch(mode=0o600)
                    path.write_text(log.stdout + log.stderr)
                except (Failure, OSError):
                    pass  # 诊断不可用不能阻止资源清理。
            try:
                removed = self.docker.run("rm", "--force", "--volumes", name, timeout=30, check=False)
                if removed.returncode:
                    # run 在创建前失败时，容器可以不存在；真实残留不得忽略。
                    exists = self.docker.run("ps", "-aq", "--filter", "name=^/" + name + "$", check=False)
                    if exists.returncode or exists.stdout.strip():
                        errors.append("container_cleanup_failed")
            except Failure:
                errors.append("container_cleanup_failed")
        if self.network_created:
            try:
                removed = self.docker.run("network", "rm", self.network, check=False)
                if removed.returncode:
                    exists = self.docker.run("network", "ls", "-q", "--filter", "name=^" + self.network + "$", check=False)
                    if exists.returncode or exists.stdout.strip():
                        errors.append("network_cleanup_failed")
            except Failure:
                errors.append("network_cleanup_failed")
        for path in self.secret_files:
            try:
                path.unlink(missing_ok=True)
            except OSError:
                errors.append("credential_file_cleanup_failed")
        require(not errors, "resource_cleanup_failed")


def compare(samples, args):
    result = {}
    for mode in ("json", "sse"):
        grouped = {variant: [sample["workloads"][mode] for sample in samples if sample["variant"] == variant]
                   for variant in ("old", "new")}
        summary = {variant: {"median_throughput_rps": statistics.median(row["throughput_rps"] for row in rows),
            "median_p95_latency_ms": statistics.median(row["latency_ms"]["p95"] for row in rows),
            "total_requests": sum(row["requests"] for row in rows),
            "unexpected_errors": sum(row["unexpected_errors"] for row in rows)} for variant, rows in grouped.items()}
        old, new = summary["old"], summary["new"]
        throughput_ratio = new["median_throughput_rps"] / old["median_throughput_rps"] if old["median_throughput_rps"] else 0
        latency_ratio = new["median_p95_latency_ms"] / old["median_p95_latency_ms"]
        passed = (old["unexpected_errors"] == new["unexpected_errors"] == 0 and
                  throughput_ratio >= 1 - args.max_throughput_drop and latency_ratio <= 1 + args.max_p95_increase)
        result[mode] = {"old": old, "new": new, "new_old_throughput_ratio": throughput_ratio,
                        "new_old_p95_latency_ratio": latency_ratio, "status": "PASS" if passed else "FAIL"}
    return result


def arguments(argv):
    parser = argparse.ArgumentParser(description=__doc__, formatter_class=argparse.RawDescriptionHelpFormatter)
    parser.add_argument("--old-image", default=OLD_IMAGE)
    parser.add_argument("--old-sha", default=OLD_SHA)
    parser.add_argument("--new-image", required=True)
    parser.add_argument("--new-sha", required=True)
    parser.add_argument("--docker-host", default=os.environ.get("DOCKER_HOST", "unix:///var/run/docker.sock"))
    parser.add_argument("--postgres-image", default="postgres:18.6-alpine")
    parser.add_argument("--redis-image", default="redis:8.4-alpine")
    parser.add_argument("--mock-image", default="python:3.12-alpine")
    parser.add_argument("--rounds", type=int, default=5)
    parser.add_argument("--requests", type=int, default=200, help="每轮、每镜像、每种响应格式的请求数")
    parser.add_argument("--warmup", type=int, default=10, help="每个样本、每种格式的预热请求数")
    parser.add_argument("--concurrency", type=int, default=4)
    parser.add_argument("--startup-timeout", type=float, default=120)
    parser.add_argument("--request-timeout", type=float, default=20)
    parser.add_argument("--max-throughput-drop", type=float, default=0.15)
    parser.add_argument("--max-p95-increase", type=float, default=0.20)
    parser.add_argument("--smoke", action="store_true", help="允许相同镜像及少于 5 轮；结果永不构成 REL 验收")
    parser.add_argument("--output", type=Path, help="保存同 stdout 的脱敏 JSON")
    parser.add_argument("--diagnostics-parent", type=Path, help="私密失败日志临时目录的父目录；不会删除父目录")
    return parser.parse_args(argv)


def execute(args, report, directory):
    require(args.rounds >= (1 if args.smoke else 5), "formal_comparison_requires_at_least_five_rounds")
    require(args.requests >= 1 and args.warmup >= 1 and args.concurrency >= 1, "load_parameters_must_be_positive")
    require(args.requests >= args.concurrency, "requests_must_cover_all_workers")
    require(args.startup_timeout > 0 and args.request_timeout > 0, "timeouts_must_be_positive")
    require(0 <= args.max_throughput_drop < 1 and math.isfinite(args.max_p95_increase) and args.max_p95_increase >= 0,
            "invalid_comparison_thresholds")
    if not args.smoke:
        require(args.max_throughput_drop <= 0.15 and args.max_p95_increase <= 0.20,
                "formal_thresholds_cannot_exceed_rel001_limits")
    require(args.docker_host.startswith("unix:///"), "only_local_unix_docker_daemon_allowed")
    docker = Docker(args.docker_host, directory)
    info = json.loads(docker.run("info", "--format",
        '{"architecture":{{json .Architecture}},"cpus":{{json .NCPU}},"memory_bytes":{{json .MemTotal}},'
        '"docker_version":{{json .ServerVersion}}}').stdout)
    require(info["architecture"] in ("aarch64", "arm64", "amd64", "x86_64"), "unsupported_docker_architecture")
    report["environment"] = {"host_architecture": platform.machine(), "client_python_version": platform.python_version(),
        "docker": info, "application_platform": "linux/amd64",
        "emulated": info["architecture"] in ("aarch64", "arm64"),
        "network": "fresh Docker bridge network per sample; only application port published, bound to loopback",
        "state": "fresh database, Redis and API fixtures per sample; samples run sequentially",
        "limitations": ["合成固定响应只覆盖基础 HTTP 网关、鉴权、调度和协议合同；不代表付费模型推理或生产容量。",
                        "ARM daemon 上的 amd64 仿真及本机调度噪声限制结果外推；需在同一空闲环境串行运行。"]}
    identities = {"old": docker.image(args.old_image, args.old_sha), "new": docker.image(args.new_image, args.new_sha)}
    report["images"] = identities
    if not args.smoke:
        require(args.old_image == OLD_IMAGE and args.old_sha == OLD_SHA, "formal_old_image_must_match_fixed_stable_baseline")
        require(identities["new"]["id"] != identities["old"]["id"] and args.new_sha != args.old_sha,
                "same_image_or_revision_requires_smoke")
    dependencies = {name: docker.image(reference) for name, reference in (
        ("postgres", args.postgres_image), ("redis", args.redis_image), ("mock", args.mock_image))}
    report["dependencies"] = dependencies
    report["load"] = {"endpoint": "/v1/responses", "model": MODEL, "formats": ["json", "sse"],
        "rounds": args.rounds, "requests_per_format_per_sample": args.requests, "concurrency": args.concurrency,
        "warmup_per_format_per_sample": args.warmup, "request_timeout_seconds": args.request_timeout,
        "latency_boundary": "request start to fully consumed, content/usage/terminal-validated response",
        "throughput_boundary": "validated successful requests / wall-clock duration, including client scheduling"}
    report["thresholds"] = {"max_median_throughput_drop": args.max_throughput_drop,
        "max_median_p95_latency_increase": args.max_p95_increase, "unexpected_success_traffic_errors": 0,
        "aggregation": "each format compared separately using median per-round throughput and median per-round p95"}
    samples = report["samples"] = []
    source_digest = hashlib.sha256(Path(__file__).read_bytes()).hexdigest()
    report["harness_sha256"] = source_digest
    for round_index in range(args.rounds):
        order = ("old", "new") if round_index % 2 == 0 else ("new", "old")
        for position, variant in enumerate(order):
            require(hashlib.sha256(Path(__file__).read_bytes()).hexdigest() == source_digest, "harness_source_changed_during_comparison")
            sample = {"round": round_index + 1, "position": position + 1, "order": list(order), "variant": variant,
                      "warmup": {}, "workloads": {}}
            samples.append(sample)
            stack = Stack(docker, identities[variant], dependencies, args)
            failed = True
            try:
                stack.start()
                sample["runtime"] = stack.runtime
                modes = ("json", "sse") if round_index % 2 == 0 else ("sse", "json")
                for mode in modes:
                    warmup = load(stack.port, stack.key, mode, args.warmup, min(args.concurrency, args.warmup),
                                  "warmup", args.request_timeout)
                    sample["warmup"][mode] = {"requests": warmup["requests"], "unexpected_errors": warmup["unexpected_errors"],
                                              "error_counts": warmup["error_counts"]}
                    require(warmup["unexpected_errors"] == 0, "warmup_response_contract_failed")
                    before = stack.stats()
                    measured = load(stack.port, stack.key, mode, args.requests, args.concurrency, "measure", args.request_timeout)
                    after = stack.stats()
                    measured["mock_upstream_requests"] = after.get(mode, 0) - before.get(mode, 0)
                    sample["workloads"][mode] = measured
                    require(measured["mock_upstream_requests"] == args.requests, "measurement_not_fully_reached_mock_upstream")
                sample["controlled_error"] = stack.controlled_error()
                sample["mock_statistics"] = stack.stats()
                require(not any(sample["mock_statistics"].get(key, 0) for key in ("unexpected_path", "unexpected_payload")),
                        "unexpected_mock_upstream_requests")
                failed = False
            finally:
                previous_handler = signal.signal(signal.SIGTERM, signal.SIG_IGN)
                previous_interrupt = signal.signal(signal.SIGINT, signal.SIG_IGN)
                try:
                    stack.close(failed)
                    sample["resources_cleaned"] = True
                finally:
                    signal.signal(signal.SIGTERM, previous_handler)
                    signal.signal(signal.SIGINT, previous_interrupt)
    report["comparison"] = compare(samples, args)
    passed = all(result["status"] == "PASS" for result in report["comparison"].values())
    # smoke 的性能比值受小样本噪声影响，仅 HTTP/身份/清理合同决定 smoke 状态。
    smoke_ok = all(row["unexpected_errors"] == 0 for sample in samples for row in sample["workloads"].values())
    report["status"] = "PASS" if (smoke_ok if args.smoke else passed) else "FAIL"
    report["rel001_acceptance"] = not args.smoke and passed


def main(argv=None):
    args = arguments(argv)
    report = {"schema": "rel001.gateway-performance.v1", "mode": "smoke" if args.smoke else "comparison",
              "status": "FAIL", "rel001_acceptance": False,
              "started_at_utc": time.strftime("%Y-%m-%dT%H:%M:%SZ", time.gmtime())}
    directory = None

    def interrupted(_signum, _frame):
        raise Failure("execution_interrupted")

    signal.signal(signal.SIGTERM, interrupted)
    try:
        directory = Path(tempfile.mkdtemp(prefix="rel001-perf-", dir=args.diagnostics_parent))
        directory.chmod(0o700)
        execute(args, report, directory)
    except Failure as error:
        report["failure_code"] = str(error)
    except KeyboardInterrupt:
        report["failure_code"] = "execution_interrupted"
    except Exception as error:
        # 异常文本可能带 URL、响应或凭据；只公开固定类别名，不写 traceback。
        report["failure_code"] = "unhandled_" + type(error).__name__
    finally:
        report["finished_at_utc"] = time.strftime("%Y-%m-%dT%H:%M:%SZ", time.gmtime())
        keep = report["status"] == "FAIL"
        report["private_diagnostics_retained"] = keep and directory is not None
        if directory is not None and not keep:
            shutil.rmtree(directory)
        elif directory is not None:
            print("私密诊断目录：" + str(directory), file=sys.stderr)
    encoded = json.dumps(report, ensure_ascii=False, indent=2, allow_nan=False) + "\n"
    if args.output:
        try:
            args.output.write_text(encoded)
        except OSError:
            report.update(status="FAIL", rel001_acceptance=False, failure_code="output_file_write_failed")
            encoded = json.dumps(report, ensure_ascii=False, indent=2, allow_nan=False) + "\n"
    sys.stdout.write(encoded)
    return 0 if report["status"] == "PASS" else 1


if __name__ == "__main__":
    if sys.argv[1:] == ["--mock-server"]:
        mock_server()
    else:
        sys.exit(main())
