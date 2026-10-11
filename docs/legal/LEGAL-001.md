# LEGAL-001：第三方归属与来源说明验收记录

核对日期：2026-10-10。任务只补充公开归属、修改关系与现有文件头保留证据；不修改应用、许可证策略或发布流程。

## 前置与固定来源

- PREP-003 已随 [PR #11](https://github.com/ccisnoxx/sub2api/pull/11) 合并，合并提交为 `24daf87641750fb78a3ad5818e0eb3eb392d638e`。来源矩阵见 [UPSTREAM.md](../../UPSTREAM.md)。
- 本任务从 `main` 的固定基准 `c1dae6357b4de7cc11d81547013b87550c4ba586` 创建 `codex/legal-001-third-party-notices`；采用独立干净工作树，维护者原工作树中已有的计划暂存与修改不进入本提交。
- 原始项目为 [Wei-Shaw/sub2api](https://github.com/Wei-Shaw/sub2api)，直接技术上游为 [KlN-4096/sub2api](https://github.com/KlN-4096/sub2api)。本次应用核对使用 KIN `v0.2.14-klno.5` / `c7aacf5d3ae383d0d5c75f471f66e61690a5701d` 和本 fork 已发布应用 `v0.2.14-klno.5-tps.2` / `3d5e1fde82707900a21f2b5538b112c7bab3c04a`，不把 `main` 的继承应用树当作个人功能实现。
- Plus 来源为 [LuckyKuang/sub2api-plus](https://github.com/LuckyKuang/sub2api-plus)，发行标签 `v0.2.14+custom.002` 的 annotated tag object 为 `4d8a25878fa7d4f99ec5bf2f8576dccb023409eb`，实际提交为 `a7749f5826ec0a5c6493c47fde9474dc31525f14`；固定阅读与适配参考提交为 `90da415c62b94c9417d9ce2b72b1507ed22f0303`。
- 本次重新查询远端标签，结果与 PREP-003 一致；阅读提交的第一父提交为标签实际提交，两棵源码树仅 `UPSTREAM.md` 不同。没有将阅读提交冒称为标签解析结果，也不因本任务升级参考版本。

## 实施与文件头核对

根目录已有 PREP-003 的最小 [NOTICE](../../NOTICE)，本任务在此补齐四个模块的借鉴方式与实质修改，不另建重复归属文件。英文、中文、日文 README 原有 NOTICE 链接均可直接访问该文件，无需重复修改 README。

| 模块 | 保留的 Plus 归属 | KIN 适配关系 |
| --- | --- | --- |
| 用量计时、完成状态 | 指标、helper、schema 的结构参考 | 旧首字与严格首字分离、可空媒体计数、终态快照；接入既有转发和计费 |
| 模型与价格目录 | opt-in 目录组织的概念参考 | KIN 分组授权、模型映射、计费探针和本地 DTO/页面 |
| 路由诊断 | 决策快照字段的结构参考 | 实际选择分支、请求/逻辑 turn、Ops 队列和管理员字段白名单 |
| 服务状态 | V3 终态、分钟事实、水位和事件的概念参考 | 独立 observation key、表与开关、管理员可见和本地健康语义 |

文件头核对针对固定应用提交 A 相对 KIN 基线 K 改动的维护源码：`backend/internal/`、`backend/ent/schema/`、`backend/migrations/` 和 `frontend/src/`，共 **169 个文件**。其中 **93 个**继承 KIN 同路径文件，**98 个**在 Plus 阅读树中有同路径文件。逐个比较来源与应用中的版权声明及 SPDX 行，检查范围内没有待恢复的声明。

为避免遗漏 Plus 的异名来源，额外扫描 P 在上述四个目录中的 **4061 个文件**；未发现 `SPDX-`、带 `(c)` / `©` / 年份的版权声明。PREP-003 指定的四份 Plus 功能文档、计时 helper 和 usage schema 也没有版权头或 SPDX 声明。代码中出现的版权违规分类、提示词与界面标签不作为作者声明处理。

本任务不修改任何源码、生成文件或现有版权头，也不凭维护 owner、文件相似程度或相同许可证编造作者、年份、版权头与 SPDX 许可标识。Plus 的结构和概念参考仍在 NOTICE 和来源矩阵中署明；本次结论仅针对这些固定来源的声明保留，不推断更早作者身份或宣称完整许可审核已完成。后续直接复制或显著改写文件时，必须保留来源已有的版权头与 SPDX 信息。

## 验证命令与结果

以下检查在本次文档候选上通过；本地未重复运行应用构建、业务测试、数据库恢复或 Release dry-run，因为没有改变这些行为或产物。仓库自动门禁的实际状态以 PR 记录为准。完整许可证副本、依赖许可和发行材料属于 LEGAL-002。

```bash
git ls-remote https://github.com/LuckyKuang/sub2api-plus.git 'refs/tags/v0.2.14+custom.002*'
git show --format='%H %P' --no-patch 90da415c62b94c9417d9ce2b72b1507ed22f0303
git diff --name-only a7749f5826ec0a5c6493c47fde9474dc31525f14 90da415c62b94c9417d9ce2b72b1507ed22f0303
git diff c1dae6357b4de7cc11d81547013b87550c4ba586 --check
```

结果：远端 tag object 和 peeled commit 一致；父提交与文档差异符合来源记录；文档差异无空白错误。本次还逐项核对 NOTICE/UPSTREAM/三份 README 的仓库、标签与完整 SHA 一致、README 的 NOTICE 入口存在，以及新增文档本地和外部链接可访问。

下面的声明保留检查可在含这些固定 Git 对象的仓库中重跑；缺失对象时先从对应来源取得完整提交，不用移动分支或猜测值代替。

```python
import re
import subprocess

K = 'c7aacf5d3ae383d0d5c75f471f66e61690a5701d'
P = '90da415c62b94c9417d9ce2b72b1507ed22f0303'
A = '3d5e1fde82707900a21f2b5538b112c7bab3c04a'
prefixes = ('backend/internal/', 'backend/ent/schema/',
            'backend/migrations/', 'frontend/src/')
marker = re.compile(r'SPDX-|copyright\s+(?:\([cC]\)|©|\d{4})', re.I)

def git(*args):
    return subprocess.check_output(['git', *args], text=True)

def source(ref, path):
    return git('show', f'{ref}:{path}')

def notices(content):
    return [line.strip() for line in content.splitlines() if marker.search(line)]

paths = {ref: set(git('ls-tree', '-r', '--name-only', ref).splitlines())
         for ref in (K, P)}
changed = [path for path in git('diff', '--name-only', K, A).splitlines()
           if path.startswith(prefixes)]
for path in changed:
    current = source(A, path)
    for ref in (K, P):
        if path in paths[ref]:
            for line in notices(source(ref, path)):
                assert line in current, f'来源声明需复核：{ref}:{path}: {line}'
plus_scope = [path for path in paths[P] if path.startswith(prefixes)]
plus_notices = [(path, line) for path in plus_scope
                for line in notices(source(P, path))]
assert not plus_notices, f'Plus 异名来源声明需人工复核：{plus_notices}'
print(f'通过：{len(changed)} 个改动文件，{len(plus_scope)} 个 Plus 来源文件')
```

结果：169 个改动文件与 4061 个 Plus 来源文件核对通过。检查用途是定位现有声明是否遗漏；没有声明不等于没有版权，也不等于无需归属。

## 风险、数据库影响与回滚

- 此记录证明固定快照的来源及声明保留；不能替代未来 KIN 重放迁移的文件头核对，不能代替 LEGAL-002 的完整许可及发行材料审核。
- 数据库影响：none；没有 SQL、迁移身份、配置、业务代码或部署操作。
- 回滚：正常回退 LEGAL-001 自身对 NOTICE、UPSTREAM.md 与本验收记录的文档提交；保留 PREP-003 已有 NOTICE 和来源矩阵、README 入口及其他任务改动。不反向依赖合并提交、不强推、不运行发布或数据库回退。
- 本任务未改变 `main` 的同步、发布、权限或审查控制规则，不触发这些规则变更的独立复核要求。

## 验收核对

- [x] 根目录可直接发现第三方归属文件 NOTICE。
- [x] 仓库名称、KIN/Plus 标签、标签实际提交和固定阅读提交与 UPSTREAM.md 一致。
- [x] 四个模块明确保留结构/概念参考及实质修改关系，未宣称完全自主实现。
- [x] 明确使用相同许可证不表示无需署名，保留原有许可与版权/SPDX 声明。
- [x] 明确不代表任何上游维护者认可、支持或背书，维护 owner 不冒充版权人。
- [x] 三份 README 已有 NOTICE 入口，检查范围内没有需要恢复的源文件声明。

提交 SHA、PR、仓库自动检查状态与交付条件由 PR 记录；上述核对不把尚未执行的 CI 或 LEGAL-002 标为完成。
