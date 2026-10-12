#!/usr/bin/env python3
"""REL-001 一次性原生复测：只运行既有 OCI 和固定候选工具，不构建或发布。"""
import hashlib
import json
import os
from pathlib import Path
import platform
import re
import subprocess
import sys
import threading
import time

REPO = "ccisnoxx/sub2api"
SHA = "3ded82bcde22dad28ff6c08939369c44d57ff0b1"
RUN = 38210844700
ARTIFACT = 11729451134
ARTIFACT_DIGEST = "sha256:1922376221cbb1a0d9d042b1c4cc4065d89eb601aaf39588f577ea6d8c82b19d"
OCI_TAR_SHA = "c25369794d147d89bc8966a9f3e68e8cea76cbe8dcea98b28f3fe23692553fa3"
OCI_INDEX = "sha256:9397e21caf01ea1080c84a93809d5a586a9d6864e6923872a58589817b1850e3"
OCI_MANIFEST = "sha256:9cd36d4fa42c6c0b890449f71125652917e406006afbd3618b787d98a42babb1"
OCI_CONFIG = "sha256:346d4e648967693a1255e0c634874a25336701a446d838f493ba1526e0d5517c"
HARNESS_SHA = "2a6078d692933f323bd5bea2ae4f3710985f159fb413859f4a624406a79238f5"
LOCAL_TAG = "ghcr.io/ccisnoxx/sub2api:0.2.15-klno.2-tps.1-rc.1-amd64"


def command(*args, timeout=120):
    return subprocess.check_output(args, text=True, timeout=timeout).strip()


def require(condition, code):
    if not condition:
        raise RuntimeError(code)


def source():
    run = json.loads(command("gh", "api", f"repos/{REPO}/actions/runs/{RUN}"))
    require(run["head_sha"] == SHA and run["conclusion"] == "success" and
            run["status"] == "completed" and run["event"] == "workflow_dispatch" and
            run["path"] == ".github/workflows/release.yml" and
            run["repository"]["full_name"] == REPO and
            run["head_repository"]["full_name"] == REPO, "source_run_mismatch")
    artifact = json.loads(command("gh", "api", f"repos/{REPO}/actions/artifacts/{ARTIFACT}"))
    require(artifact["id"] == ARTIFACT and not artifact["expired"] and
            artifact["digest"] == ARTIFACT_DIGEST and
            artifact["name"] == f"release-dry-run-oci-{RUN}-1" and
            artifact["workflow_run"]["id"] == RUN and
            artifact["workflow_run"]["head_sha"] == SHA and
            artifact["workflow_run"]["repository_id"] == 1410056045 and
            artifact["workflow_run"]["head_repository_id"] == 1410056045,
            "source_artifact_mismatch")
    print("固定源码运行与 OCI artifact 身份匹配。")


def sha256(path):
    digest = hashlib.sha256()
    with path.open("rb") as handle:
        for chunk in iter(lambda: handle.read(1024 * 1024), b""):
            digest.update(chunk)
    return digest.hexdigest()


def execute_harness(args, timeout=900, grace=60, cwd=None, env=None, captured=None):
    # SIGTERM 允许固定工具执行 finally、删除合成凭据并落盘失败 JSON。
    with subprocess.Popen(args, cwd=cwd, env=env, stdout=subprocess.PIPE, stderr=subprocess.PIPE, text=True) as process:
        try:
            stdout, _ = process.communicate(timeout=timeout)
        except subprocess.TimeoutExpired:
            process.terminate()
            try:
                process.communicate(timeout=grace)
            except subprocess.TimeoutExpired:
                process.kill()
                process.communicate(timeout=10)
                raise RuntimeError("harness_hard_timeout") from None
            raise RuntimeError("harness_timeout") from None
        if captured is not None:
            captured.append(stdout)
        return process.returncode


def database_result(stdout, exit_code):
    """只接受实际 Go JSON 的固定测试判定，不运输原始输出或数据库内容。"""
    root = "TestUPSTREAM007UpgradePaths"
    expected = {root, *(root + "/" + name for name in ("empty", "old-kin", "already-personal"))}
    passed = {}
    for line in stdout.splitlines():
        row = json.loads(line)
        name = row.get("Test")
        if name in expected and row.get("Action") == "pass":
            passed[name] = row.get("Elapsed")
        require(row.get("Action") not in ("fail", "skip"), "database_failed_or_skipped")
    require(exit_code == 0 and set(passed) == expected, "database_upgrade_gate_failed")
    return {"status": "PASS", "exit_code": exit_code, "tests": passed}


def database(loaded):
    env = dict(os.environ, CI="true", GOTOOLCHAIN="local", TESTCONTAINERS_RYUK_DISABLED="false",
               UPSTREAM007_CANDIDATE_IMAGE=loaded["id"], UPSTREAM007_CANDIDATE_SHA=SHA)
    output = []
    try:
        code = execute_harness(["go", "test", "-json", "-tags=integration,upstream007",
            "-run", "^TestUPSTREAM007UpgradePaths$", "-count=1", "-timeout=30m", "./internal/repository"],
            timeout=2100, cwd="application/backend", env=env, captured=output)
        return database_result(output[0], code)
    finally:
        # TestMain 的 os.Exit 不运行 defer；专属一次性 VM 上按 Go Testcontainers 标签清理。
        for kind, listing, removal in (
            ("container", ("ps", "-aq"), ("rm", "-fv")),
            ("network", ("network", "ls", "-q"), ("network", "rm")),
        ):
            owned = command("docker", *listing, "--filter", "label=org.testcontainers.lang=go")
            for identity in owned.splitlines():
                require(bool(re.fullmatch(r"[0-9a-f]{12,64}", identity)), "unexpected_database_resource_id")
                removal_result = subprocess.run(["docker", *removal, identity], capture_output=True, timeout=120)
                if removal_result.returncode:
                    # Ryuk 可能已并发删除该对象；仅确认对象消失后接受这个非零结果。
                    require(not command("docker", *listing, "--filter", "id=" + identity),
                            "database_resource_removal_failed")
            require(not command("docker", *listing, "--filter", "label=org.testcontainers.lang=go"),
                    kind + "_database_cleanup_incomplete")


def cleanup(report, output):
    # 隔离 runner 上仅清理本工具带固定名称/标签的资源，匿名卷随容器删除。
    for log in output.glob("rel001-perf-*/*-app.log"):
        report["runtime_oom_lines"].extend(line for line in log.read_text(errors="replace").splitlines()
            if re.fullmatch(r"runtime: out of memory: cannot allocate [0-9]+-byte block \([0-9]+ in use\)", line))
    names = command("docker", "ps", "-a", "--filter", "label=rel001.gateway-perf", "--format", "{{.Names}}")
    report["leftovers_before_cleanup"] = names.splitlines()
    for name in names.splitlines():
        require(bool(re.fullmatch(r"rel001-perf-[a-f0-9]+-(app|postgres|redis|mock)", name)), "unexpected_owned_container_name")
        if name.endswith("-app"):
            try:
                logs = subprocess.run(["docker", "logs", name], capture_output=True, text=True, timeout=15)
                report["runtime_oom_lines"].extend(line for line in (logs.stdout + logs.stderr).splitlines()
                    if re.fullmatch(r"runtime: out of memory: cannot allocate [0-9]+-byte block \([0-9]+ in use\)", line))
                if logs.returncode:
                    report["diagnostics_failure_code"] = "docker_logs_failed"
            except (subprocess.SubprocessError, OSError):
                report["diagnostics_failure_code"] = "docker_logs_unavailable"
        command("docker", "rm", "-fv", name)
    networks = command("docker", "network", "ls", "--format", "{{.Name}}").splitlines()
    for name in networks:
        if re.fullmatch(r"rel001-perf-[a-f0-9]+-net", name):
            command("docker", "network", "rm", name)
    for path in output.glob("rel001-perf-*/*.env"):
        path.unlink()
    report["remaining_resources"] = command("docker", "ps", "-a", "--filter", "label=rel001.gateway-perf", "--format", "{{.Names}}")
    require(not report["remaining_resources"], "container_cleanup_incomplete")
    report["remaining_networks"] = [name for name in command("docker", "network", "ls", "--format", "{{.Name}}").splitlines()
        if re.fullmatch(r"rel001-perf-[a-f0-9]+-net", name)]
    require(not report["remaining_networks"], "network_cleanup_incomplete")


def monitor(stop, states):
    """只记录应用内存与退出事实，不读取容器环境或完整日志。"""
    while not stop.wait(0.2):
        try:
            names = command("docker", "ps", "-a", "--format", "{{.Names}}", timeout=10).splitlines()
            for name in names:
                if not re.fullmatch(r"rel001-perf-[a-f0-9]+-app", name):
                    continue
                data = json.loads(command("docker", "inspect", "--format",
                    '{"state":{{json .State}},"memory_limit":{{json .HostConfig.Memory}}}', name, timeout=10))
                state = data["state"]
                row = states.setdefault(name, {"observations": 0, "max_memory_bytes": 0,
                    "oom_killed": False, "exit_codes": [], "memory_limit": data["memory_limit"]})
                row["observations"] += 1
                row["oom_killed"] |= state["OOMKilled"]
                if not state["Running"] and state["ExitCode"] not in row["exit_codes"]:
                    row["exit_codes"].append(state["ExitCode"])
                if state["Running"]:
                    stats = json.loads(command("docker", "stats", "--no-stream", "--format",
                                               '{{json .MemUsage}}', name, timeout=10))
                    usage = stats.split(" / ")[0]
                    match = re.fullmatch(r"([0-9.]+)([KMGT]?i?B)", usage)
                    if match:
                        units = {"B": 1, "kB": 1000, "KB": 1000, "MB": 10**6, "GB": 10**9,
                                 "KiB": 1024, "MiB": 1024**2, "GiB": 1024**3, "TiB": 1024**4}
                        row["max_memory_bytes"] = max(row["max_memory_bytes"],
                            int(float(match[1]) * units[match[2]]))
        except (subprocess.SubprocessError, ValueError, KeyError):
            # 容器退出并清理会与只读采样交错；采样缺口不冒充 OOM 结论。
            continue


def run():
    output = Path(os.environ["PROOF_DIRECTORY"])
    output.mkdir(mode=0o700, parents=True, exist_ok=True)
    report = {"schema": "rel001.native-proof.v1", "source_sha": SHA, "source_run": RUN,
        "source_artifact": ARTIFACT, "oci_index": OCI_INDEX, "oci_manifest": OCI_MANIFEST,
        "oci_config": OCI_CONFIG, "oci_tar_sha256": OCI_TAR_SHA, "status": "FAIL",
        "control_sha": os.environ.get("GITHUB_SHA"), "harness_sha256": HARNESS_SHA,
        "application_states": {}, "runtime_oom_lines": []}
    stop = threading.Event()
    watcher = None
    try:
        require(platform.machine() == "x86_64", "host_must_be_native_amd64")
        info = json.loads(command("docker", "info", "--format",
            '{"architecture":{{json .Architecture}},"cpus":{{json .NCPU}},"memory_bytes":{{json .MemTotal}},"version":{{json .ServerVersion}}}'))
        require(info["architecture"] in ("x86_64", "amd64"), "daemon_must_be_native_amd64")
        report["environment"] = {"host": platform.machine(), "kernel": platform.release(), "docker": info}
        require(command("git", "-C", "application", "rev-parse", "HEAD") == SHA, "application_checkout_mismatch")
        harness = Path("application/tools/rel001_gateway_perf.py")
        require(sha256(harness) == HARNESS_SHA, "harness_source_mismatch")
        archive = Path(os.environ["OCI_DIRECTORY"]) / "sub2api-amd64.oci.tar"
        require(sha256(archive) == OCI_TAR_SHA, "oci_archive_bytes_mismatch")
        command("docker", "load", "--input", str(archive), timeout=180)
        loaded = json.loads(command("docker", "image", "inspect", "--format",
            '{"id":{{json .Id}},"architecture":{{json .Architecture}},"revision":{{json (index .Config.Labels "org.opencontainers.image.revision")}}}', LOCAL_TAG))
        require(loaded["id"] in (OCI_INDEX, OCI_CONFIG) and loaded["architecture"] == "amd64" and
                loaded["revision"] == SHA, "loaded_image_mismatch")
        report["loaded_image"] = loaded
        for image in ("ghcr.io/ccisnoxx/sub2api@sha256:7446ff8ebdddca5e60989f0a5b8dec670ce3a030a9c8472e2dd3c27986e21530",
                      "postgres:18.6-alpine", "redis:8.4-alpine", "python:3.12-alpine"):
            command("docker", "pull", "--platform", "linux/amd64", image, timeout=180)
        watcher = threading.Thread(target=monitor, args=(stop, report["application_states"]), daemon=True)
        watcher.start()
        exit_code = execute_harness([sys.executable, "-B", str(harness), "--new-image", loaded["id"],
            "--new-sha", SHA, "--rounds", "5", "--requests", "200", "--concurrency", "4",
            "--warmup", "10", "--output", str(output / "gateway-performance.json"),
            "--diagnostics-parent", str(output)])
        stop.set()
        watcher.join(timeout=15)
        performance = json.loads((output / "gateway-performance.json").read_text())
        report["performance_exit_code"] = exit_code
        report["performance_status"] = performance["status"]
        require(exit_code == 0 and performance["rel001_acceptance"] is True and
                performance["environment"]["emulated"] is False and len(performance["samples"]) == 10,
                "native_performance_gate_failed")
        require(not any(row["oom_killed"] for row in report["application_states"].values()), "native_kernel_oom_seen")
        report["database_upgrade"] = database(loaded)
        report["status"] = "PASS"
    except (RuntimeError, subprocess.SubprocessError, OSError, ValueError, KeyError) as error:
        report["failure_code"] = str(error) if isinstance(error, RuntimeError) else type(error).__name__
    finally:
        stop.set()
        if watcher:
            watcher.join(timeout=15)
        try:
            cleanup(report, output)
        except (RuntimeError, subprocess.SubprocessError, OSError) as error:
            report["status"] = "FAIL"
            report["cleanup_failure_code"] = str(error) if isinstance(error, RuntimeError) else type(error).__name__
        finally:
            # 即使 daemon 不可用，也删除本次生成的本地合成凭据。
            for path in output.glob("rel001-perf-*/*.env"):
                path.unlink(missing_ok=True)
        (output / "native-proof.json").write_text(json.dumps(report, ensure_ascii=False, indent=2) + "\n")
    print(json.dumps({"status": report["status"], "failure_code": report.get("failure_code")}, ensure_ascii=False))
    return 0 if report["status"] == "PASS" else 1


if __name__ == "__main__":
    if sys.argv[1:] == ["source"]:
        source()
    elif sys.argv[1:] == ["run"]:
        sys.exit(run())
    else:
        sys.exit("expected source or run")
