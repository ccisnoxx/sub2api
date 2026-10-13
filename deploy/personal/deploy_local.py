"""本机输入与 Git 证明编排；运输只接收已验证的 main bundle。"""
import json
from pathlib import Path
from deploy_common import *
from deploy_compatibility import *
from deploy_git import GitEvidence
from deploy_transport import SSHTransport

def local_maintenance(args, bundle, config):
    require(args.target is None and args.version is None and args.rollback is None and not args.dry_run,
            "E_INPUT", "维护操作不能与部署、回滚或 dry-run 参数混用")
    if args.maintenance == "resume":
        require(SHA_RE.fullmatch(args.target_revision or "") and RECORD_RE.fullmatch(args.deployment_record or ""),
                "E_INPUT", "OBS/RESP 恢复必须显式指定完整 --target-revision 与 --deployment-record")
        git = GitEvidence(bundle.directory.parents[1])
        runtime = git.runtime_fingerprint(args.target_revision)
        if runtime == OBS_NEW_RUNTIME:
            git.personal(OBS_NEW_VERSION, args.target_revision)
            git.obs_runtime(args.target_revision)
        elif runtime == RESP_NEW_RUNTIME:
            git.personal(RESP_NEW_VERSION, args.target_revision)
            git.resp_runtime(args.target_revision)
        else:
            raise DeployError("E_COMPATIBILITY", "维护恢复只接受审定的 OBS 或 RESP 完整运行树")
    else:
        require(args.target_revision is None and args.deployment_record is None,
                "E_INPUT", "freeze 不接受恢复目标参数")
    with SSHTransport(bundle, config) as remote:
        print(json.dumps(remote.maintenance(args.maintenance, args.target_revision, args.deployment_record),
                         ensure_ascii=False, indent=2))
    return 0

def local_main(args, bundle, config):
    root = bundle.directory.parents[1]
    git = GitEvidence(root)
    target = parse_target(args.target, args.version) if not args.rollback else None
    require(args.rollback is None or (args.target is None and args.version is None and RECORD_RE.fullmatch(args.rollback)),
            "E_INPUT", "--rollback 只接受部署记录 ID，不与目标版本混用")
    origin = git.git("remote", "get-url", "origin").decode().strip()
    require(origin in ("git@github.com:ccisnoxx/sub2api.git", "https://github.com/ccisnoxx/sub2api.git", "https://github.com/ccisnoxx/sub2api"),
            "E_GIT", "本机 origin 必须是 ccisnoxx/sub2api")
    if target:
        target.update(revision=git.personal(target["version"]), source=SOURCE)
    with SSHTransport(bundle, config) as remote:
        snapshot = remote.probe(args.rollback)
        old = snapshot["running"]
        validate_metadata(old, allow_legacy=True)
        if old["source"] == SOURCE:
            git.personal(old["version"], old["revision"])
        if args.rollback:
            previous = snapshot["rollback_record"]
            require(previous.get("status") == "success" and previous.get("action") == "deploy" and
                    snapshot.get("selection") == {"image": old["image"], "record_id": args.rollback},
                    "E_STALE_ROLLBACK", "回滚记录不是当前成功部署")
            validate_compatibility_proof(previous.get("compatibility", {}), previous["old"]["revision"], old["revision"],
                                         previous["old"]["version"], old["version"])
            require_image_rollback(previous["compatibility"])
            target = dict(previous["old"])
            validate_metadata(target, allow_legacy=True)
            if target["source"] == SOURCE:
                git.personal(target["version"], target["revision"])
            target["input"] = target["image"]
        proof = git.compatibility(old, target)
        request = {"schema": 1, "action": "rollback" if args.rollback else "deploy", "expected_old": old, "target": target,
                   "compatibility": proof, "tool_revision": bundle.revision, "tool_bundle": bundle.files,
                   "tool_sha256": bundle.files["deploy_hostdzire.py"], "rollback_id": args.rollback}
        response, code = remote.execute(request, dry_run=args.dry_run)
        print(json.dumps(response, ensure_ascii=False, indent=2))
        return code
