"""本机输入与 Git 证明编排；运输只接收已验证的 main bundle。"""
import json
from pathlib import Path
from deploy_common import *
from deploy_compatibility import *
from deploy_git import GitEvidence
from deploy_transport import SSHTransport

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
