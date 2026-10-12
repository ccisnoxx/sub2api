# CI-000：真实触发与证据复用验收

本记录只覆盖控制上线与真实 Actions 验收，不包含发布或部署。十个后续独立任务的长期观察保持待完成，不能用本次验收事件充数。

## 固定来源

- main [PR #37](https://github.com/ccisnoxx/sub2api/pull/37) 已合并：cd5afc67fc83c021c72aa056c9f29277949ae4ff。
- personal 控制同步 [PR #38](https://github.com/ccisnoxx/sub2api/pull/38) 已普通 merge：当前 personal 为 7ebb60b2f6bf43e5f73c38e37b61de6cb5f063e6；第一父提交/基础 fb3be6334d97cc1c5b3a1418ecfcf9f560530cfb，第二父提交/候选 b07ff16dcf7ed1a1a9773970daef9024c1bc6c70，merge tree 与候选逐字节一致。
- 同步必要 .github 控制闭包、CI 文档和执行协议；仅额外补齐 CI-002 的 deploy/Dockerfile 构建 pins/版本日志/GOTOOLCHAIN 和 frontend/package.json packageManager。保留 personal 应用源码、依赖、锁文件、Makefile、上游来源、许可材料和审计例外，没有整体合并 main 应用树。

## 验证记录

| 事件/检查 | 运行 | 结果与覆盖 |
|---|---|---|
| 专用候选 opened | [38202375583](https://github.com/ccisnoxx/sub2api/actions/runs/38202375583) | 预期拒绝，62秒；binding成功，缺少Full使certified/ready失败；应用和安全job未重复执行 |
| Full期间 Ready | [38202519258](https://github.com/ccisnoxx/sub2api/actions/runs/38202519258) | 预期拒绝，66秒；显式Full未被取消 |
| 唯一候选 Full | [38202385227](https://github.com/ccisnoxx/sub2api/actions/runs/38202385227) | success，18分10秒；15个jobs全部成功，含Go/前端安全扫描、unit/integration/recording Race、关键前端/TPS/build与certified/ready |
| Full后 full-ci 标签 | [38203673542](https://github.com/ccisnoxx/sub2api/actions/runs/38203673542) | success，65秒；check_only Quick 引用上述Full；应用/扫描/同步job跳过 |
| personal merge push | [38203810810](https://github.com/ccisnoxx/sub2api/actions/runs/38203810810) | success，2分17秒；Quick引用同一Full，后端全量测试、全库lint与安全扫描跳过 |
| 文档 synchronize | [38203961302](https://github.com/ccisnoxx/sub2api/actions/runs/38203961302) | success，2分02秒；应用/辅助/扫描job全部跳过 |
| 文档 Ready | [38204006676](https://github.com/ccisnoxx/sub2api/actions/runs/38204006676) | success，1分34秒；仍为Quick，不要求Full，两类扫描关闭 |
| 旧文档 opened | [38203920822](https://github.com/ccisnoxx/sub2api/actions/runs/38203920822) | cancelled，墙钟71秒；下一synchronize运行创建后34秒结束 |
| 最终SHA手动 Quick | [38204082448](https://github.com/ccisnoxx/sub2api/actions/runs/38204082448) | success，70秒；无Full marker；指定为发布证据时拒绝 |

本地发布合同90项、同步/分类40项、工具合同10项、actionlint v1.7.12、Bash语法、文档本地链接与diff通过。未执行ShellCheck/pyflakes。Docker pins/packageManager 缺失在同步前导致合同失败，补齐后先重跑失败检查，再验证完整控制合同；没有删测试或跳过失败。

独立 fresh critical_reviewer 对冻结同步差异无确认阻断，另执行16个聚合失败、4个专用事件、10个必要Full job跳过反例，均拒绝不完整证据。真实扫描工具元数据绑定候选 `b07ff16dcf7ed1a1a9773970daef9024c1bc6c70`，实际 Go 1.27.2、govulncheck v1.9.0、Python 3.12.13。

同步调度器使用真实 API 返回运行中 Full 38202385227，未重复 dispatch。传输适配器禁止再次调用 workflow run，用于将误调度直接转为验收失败。

## 只读发布门禁

调用来自可信 main cd5afc67 快照的 personal_release.gate，API 适配器仅允许 GET、拒绝 payload。没有调用 prepare/authorize，也没有分配版本、创建标签、发布镜像或部署。

旧个人源码 fb3be6334d97cc1c5b3a1418ecfcf9f560530cfb 的历史成功 push 38135489900 被新政策拒绝；无 request/full 证明的旧成功不能替代首次新Full。最终 SHA `7ebb60b2f6bf43e5f73c38e37b61de6cb5f063e6` 的 push 38203810810 已被 gate 接受；实际计划引用根Full 38202385227，保护规则与同一check suite核对通过。手动Quick 38204082448成功后，指定Quick被拒绝；留空自动查找仍返回38203810810并实时重验根Full，较新Quick没有掩盖有效复用证据。

临时文档 [PR #39](https://github.com/ccisnoxx/sub2api/pull/39) 已关闭且未合并，验收分支删除。synchronize 与 Ready 是两个明确触发事件，均成功Quick；仅旧SHA的opened运行取消，不把取消或跳过记为通过。

PERSONAL_RELEASE_ENABLED=false 保持；personal 的管理员约束、严格保护及 GitHub Actions app 15368 的 personal-ready required check 保持。周安全扫描入口启用，真实本次扫描通过不能冒称未来定时扫描已执行。

## 数据库、回滚与后续

两次自动 Personal release（38203968842、38204164045）均 skipped；前后个人标签及最近Release清单一致。没有执行发布/部署或镜像构建dry-run，也没有人为删除/重跑失败远端根Full。精确的“binding后、certify前Full刚完成”时间窗口、失效根拒绝及同最终SHA真实Full复用仍以本地合同/独立反例覆盖；上述真实merge复用不冒称这些情形的现场演练。

数据库影响 none。回滚采用普通 revert 控制同步与对应 main 控制提交，生产者/消费者接口一并恢复；不强推、移动正式标签、放宽审计或重新部署。上线后至少十个独立任务观察仍待完成。
