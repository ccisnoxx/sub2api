"""本机真实 Git 来源、祖先与完整运行树证据。"""
import json
import subprocess
from deploy_common import *
from deploy_compatibility import *

class GitEvidence:
    def __init__(self, root):
        self.root = root

    def git(self, *args):
        result = subprocess.run(["git", "-C", str(self.root), *args], capture_output=True)
        require(result.returncode == 0, "E_GIT", "无法读取所需 Git 对象；请先获取个人标签与实际旧 revision")
        return result.stdout

    def source_at(self, revision):
        data = json.loads(self.git("show", revision + ":deploy/personal-source.json"))
        require(data.get("upstream_repository") == "KlN-4096/sub2api" and
                SHA_RE.fullmatch(data.get("upstream_sha", "")) and
                re.fullmatch(r"v[0-9]+\.[0-9]+\.[0-9]+-klno\.[0-9]+", data.get("upstream_tag", "")),
                "E_COMPATIBILITY", "来源记录不完整，禁止部署或镜像回滚")
        require(self.git("rev-parse", "refs/tags/" + data["upstream_tag"] + "^{commit}").decode().strip() == data["upstream_sha"],
                "E_COMPATIBILITY", "来源标签与记录 SHA 不一致")
        result = subprocess.run(["git", "-C", str(self.root), "merge-base", "--is-ancestor", data["upstream_sha"], revision], capture_output=True)
        require(result.returncode == 0, "E_COMPATIBILITY", "个人提交不包含所记录的上游基线")
        return data

    def personal(self, version, revision=None):
        require(VERSION_RE.fullmatch(version), "E_GIT", "必须使用个人发布标签")
        actual = self.git("rev-parse", "refs/tags/v" + version + "^{commit}").decode().strip()
        require(SHA_RE.fullmatch(actual) and (revision is None or actual == revision), "E_GIT", "OCI revision 与个人 Git 标签不一致")
        self.source_at(actual)
        return actual

    def security_patch(self, old_revision, new_revision, changed):
        require(changed == sorted(SECURITY_PATCH_FILES), "E_COMPATIBILITY", "运行差异超出已审定的安全补丁")
        files = {}
        for path in SECURITY_PATCH_FILES:
            for revision in (old_revision, new_revision):
                require(self.git("ls-tree", revision, "--", path).startswith(b"100644 blob "),
                        "E_COMPATIBILITY", "安全补丁不能改变文件类型或权限")
            files[path] = {"old_sha256": sha(self.git("show", old_revision + ":" + path)),
                           "new_sha256": sha(self.git("show", new_revision + ":" + path))}
        for direction in ("upgrade", "rollback"):
            patch = approved_security_patch(direction)
            if files == patch["files"]:
                return patch
        raise DeployError("E_COMPATIBILITY", "安全补丁文件与已审定的完整字节不一致，禁止部署或镜像回滚")

    def runtime_fingerprint(self, revision):
        return self.tree_fingerprint(revision, RUNTIME_PATHS, RUNTIME_EXCLUDES)

    def tree_fingerprint(self, revision, paths, excludes=()):
        entries = []
        for raw in self.git("ls-tree", "-r", "-z", revision, "--", *paths).split(b"\0"):
            if not raw:
                continue
            info, path = raw.split(b"\t", 1)
            path = path.decode()
            if any(path == excluded or path.startswith(excluded + "/") for excluded in excludes):
                continue
            mode, kind, object_id = info.decode().split()
            entries.append({"path": path, "mode": mode, "type": kind, "object": object_id})
        return sha(canonical(sorted(entries, key=lambda entry: entry["path"])))

    def cumulative_upgrade(self, old_revision, new_revision, changed):
        direction = "upgrade" if old_revision == CUMULATIVE_OLD_REVISION else "rollback"
        approved = approved_cumulative_upgrade(direction)
        require((old_revision if direction == "upgrade" else new_revision) == CUMULATIVE_OLD_REVISION and
                changed == list(CUMULATIVE_CHANGED_PATHS) and
                self.runtime_fingerprint(old_revision) == approved["old_runtime_sha256"] and
                self.runtime_fingerprint(new_revision) == approved["new_runtime_sha256"],
                "E_COMPATIBILITY", "累计升级仅允许固定旧应用与已审定的新完整运行树")
        application_revision = new_revision if direction == "upgrade" else old_revision
        ancestry = subprocess.run(["git", "-C", str(self.root), "merge-base", "--is-ancestor",
                                   CUMULATIVE_NEW_REVISION, application_revision], capture_output=True)
        require(ancestry.returncode == 0, "E_COMPATIBILITY", "个人合并 revision 不包含已审定的累计应用候选")
        for filename, checksum in CUMULATIVE_MIGRATIONS.items():
            path = "backend/migrations/" + filename
            require(self.git("ls-tree", application_revision, "--", path).startswith(b"100644 blob ") and
                    sha(self.git("show", application_revision + ":" + path).strip()) == checksum,
                    "E_COMPATIBILITY", "累计应用新增迁移不是已审定的普通文件与 checksum")
        return approved

    def kin_upgrade(self, old, target, changed, direction):
        approved = approved_kin_upgrade(direction)
        require(sha(canonical(changed)) == approved["changed_paths_sha256"] and
                self.runtime_fingerprint(old["revision"]) == approved["old_runtime_sha256"] and
                self.runtime_fingerprint(target["revision"]) == KIN_NEW_RUNTIME,
                "E_COMPATIBILITY", "新 KIN 仅允许固定旧应用与审定的新完整运行树")
        if direction == "upgrade":
            require(old["revision"] == KIN_OLD_REVISION and old["version"] == KIN_OLD_VERSION and
                    old["image"] == KIN_OLD_IMAGE,
                    "E_COMPATIBILITY", "新 KIN 升级必须从固定已发布旧镜像开始")
        for revision in ((target["revision"],) if direction == "upgrade" else (old["revision"], target["revision"])):
            ancestry = subprocess.run(["git", "-C", str(self.root), "merge-base", "--is-ancestor",
                                       KIN_NEW_REVISION, revision], capture_output=True)
            require(ancestry.returncode == 0, "E_COMPATIBILITY", "新 KIN revision 不包含审定运行候选")
        for filename, checksum in {**CUMULATIVE_MIGRATIONS, **KIN_MIGRATIONS}.items():
            path = "backend/migrations/" + filename
            require(self.git("ls-tree", target["revision"], "--", path).startswith(b"100644 blob ") and
                    sha(self.git("show", target["revision"] + ":" + path).strip()) == checksum,
                    "E_COMPATIBILITY", "新 KIN 迁移不是固定普通文件与 checksum")
        return approved

    def obs_runtime(self, revision):
        require(self.source_at(revision) == KIN_NEW_SOURCE and
                self.runtime_fingerprint(revision) == OBS_NEW_RUNTIME and
                self.tree_fingerprint(revision, ["frontend"]) == OBS_NEW_FRONTEND and
                self.tree_fingerprint(revision, OBS_RAW_PATHS) == OBS_RAW_SHA256,
                "E_COMPATIBILITY", "OBS 必须保留固定来源三字段和完整运行树")
        ancestry = subprocess.run(["git", "-C", str(self.root), "merge-base", "--is-ancestor",
                                   OBS_NEW_REVISION, revision], capture_output=True)
        require(ancestry.returncode == 0, "E_COMPATIBILITY", "OBS revision 不包含审定应用锚点")
        for filename, checksum in {**CUMULATIVE_MIGRATIONS, **KIN_MIGRATIONS}.items():
            path = "backend/migrations/" + filename
            require(self.git("ls-tree", revision, "--", path).startswith(b"100644 blob ") and
                    sha(self.git("show", revision + ":" + path).strip()) == checksum,
                    "E_COMPATIBILITY", "OBS 必须保留固定迁移内容、checksum 与普通文件模式")

    def obs_upgrade(self, old, target, changed):
        self.obs_runtime(target["revision"])
        direction = "upgrade" if self.runtime_fingerprint(old["revision"]) == KIN_NEW_RUNTIME else "redeploy"
        if direction == "upgrade":
            require(old["revision"] == OBS_OLD_REVISION and old["version"] == OBS_OLD_VERSION and
                    old["image"] == OBS_OLD_IMAGE and changed == list(OBS_CHANGED_PATHS) and
                    self.tree_fingerprint(old["revision"], ["frontend"]) == OBS_OLD_FRONTEND and
                    self.tree_fingerprint(old["revision"], OBS_RAW_PATHS) == OBS_RAW_SHA256,
                    "E_COMPATIBILITY", "OBS 升级只接受固定已发布 429 镜像及精确运行差异")
        else:
            self.obs_runtime(old["revision"])
            require(changed == [], "E_COMPATIBILITY", "OBS 重新部署必须保留相同完整运行树")
        frontend_changed = sorted(path for path in self.git("diff", "--no-renames", "--name-only", "-z",
                                  old["revision"], target["revision"], "--", "frontend").decode().split("\0") if path)
        require(frontend_changed == ([] if direction == "redeploy" else list(OBS_FRONTEND_CHANGED_PATHS)),
                "E_COMPATIBILITY", "OBS 前端构建输入差异不是审定的精确路径")
        return approved_obs_upgrade(direction)

    def compatibility(self, old, target):
        validate_metadata(old, allow_legacy=True)
        if "image" in target:
            validate_metadata(target, allow_legacy=True)
        else:
            require(target.get("source") == SOURCE and VERSION_RE.fullmatch(target.get("version", "")) and
                    SHA_RE.fullmatch(target.get("revision", "")), "E_COMPATIBILITY", "目标来源不完整，禁止猜测兼容性")
        target_source = self.source_at(target["revision"]) if target["source"] == SOURCE else {
            "upstream_repository": "KlN-4096/sub2api", "upstream_tag": "v0.2.14-klno.3", "upstream_sha": LEGACY["revision"]}
        old_source = self.source_at(old["revision"]) if old["source"] == SOURCE else {
            "upstream_repository": "KlN-4096/sub2api", "upstream_tag": "v0.2.14-klno.3", "upstream_sha": LEGACY["revision"]}
        paths = RUNTIME_PATHS + [":(exclude)" + path for path in RUNTIME_EXCLUDES]
        changed = self.git("diff", "--no-renames", "--name-only", "-z", old["revision"], target["revision"], "--", *paths).decode().split("\0")
        changed = sorted(path for path in changed if path)
        proof = {"old_revision": old["revision"], "new_revision": target["revision"],
                 "upstream": target_source, "paths": RUNTIME_PATHS, "excludes": RUNTIME_EXCLUDES,
                 "changed_paths": changed, "compatible": True}
        if target_source.get("upstream_tag") == KIN_NEW_SOURCE["upstream_tag"]:
            require(target_source == KIN_NEW_SOURCE, "E_COMPATIBILITY", "新 KIN 来源必须与固定三字段完全一致")
            if self.runtime_fingerprint(target["revision"]) == OBS_NEW_RUNTIME:
                require(old_source == KIN_NEW_SOURCE, "E_COMPATIBILITY", "OBS 只接受固定新 KIN 来源")
                proof["obs_upgrade"] = self.obs_upgrade(old, target, changed)
            else:
                direction = "upgrade" if old_source == FORWARD_NEW_SOURCE else "redeploy"
                require(old_source == (FORWARD_NEW_SOURCE if direction == "upgrade" else KIN_NEW_SOURCE),
                        "E_COMPATIBILITY", "新 KIN 只允许固定旧来源升级或同一审定新运行树重新部署")
                proof["kin_upgrade"] = self.kin_upgrade(old, target, changed, direction)
        elif old_source != target_source:
            require(old_source == FORWARD_OLD_SOURCE and target_source == FORWARD_NEW_SOURCE and
                    changed == list(FORWARD_CHANGED_PATHS) and
                    self.runtime_fingerprint(old["revision"]) == FORWARD_OLD_RUNTIME and
                    self.runtime_fingerprint(target["revision"]) == FORWARD_NEW_RUNTIME,
                    "E_COMPATIBILITY", "跨基线仅允许已审定的 .3 → .5 完整运行树；禁止镜像回退")
            proof["forward_upgrade"] = approved_forward_upgrade()
        elif changed:
            if CUMULATIVE_OLD_REVISION in (old["revision"], target["revision"]):
                require(old_source == FORWARD_NEW_SOURCE, "E_COMPATIBILITY", "累计应用必须保留固定 .5 来源")
                proof["cumulative_upgrade"] = self.cumulative_upgrade(old["revision"], target["revision"], changed)
            else:
                proof["security_patch"] = self.security_patch(old["revision"], target["revision"], changed)
        proof["sha256"] = sha(canonical(proof))
        if any(name in proof for name in ("forward_upgrade", "cumulative_upgrade", "kin_upgrade", "obs_upgrade")):
            validate_compatibility_proof(proof, old["revision"], target["revision"], old["version"], target["version"])
        return proof
