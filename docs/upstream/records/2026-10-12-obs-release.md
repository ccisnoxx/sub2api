# OBS 新版本发布与本机监控验收

## 授权与当前准备状态

维护者在第8节续作后明确：没有通知渠道且不需要外部通知功能；允许另建新OBS版本并分阶段部署。真实通知送达在本次环境不适用，保留应用监控信号、容量/水位只读检查与本机执行记录，不伪装外部告警能力。跨KIN回退的已验证信息丢失、旧镜像禁令及原429发布产物保持。

应用为PR #32/#31/#34整合锚点 `423f690bec2ee9d0ebccd1672906d6b688c220c3` 的文档/控制后继，保持KIN `v0.2.15-klno.2` / `ca686de5e5ac14834e12701c156831abd5ec6b71`。新版本预期 `v0.2.15-klno.2-tps.2`，最终源码SHA在控制与文档准备完成后固定；当前记录不宣称Full、发布或部署已经成功。

现有部署证明仅支持原429完整运行树，维护resume也固定原SHA。本次增加固定OBS运行树的专属单向部署证明、原记录绑定的明确目标维护恢复；未知树、反向及跨KIN回退继续失败关闭。直接合同验证与fresh独立复核完成前不推送候选或开始Full。

## 实际预检与证据复用

生产目标仍为hostdzire `/root/sub2api-kin`，独立合成测试目标为 `/root/sub2api-kin-s44-validation`（Compose入口 `compose.yaml`，不使用生产作测试）。发布前只读快照与ARCH-002接受的现场逐字段相同：应用429、PG/Redis身份、配置摘要、308账本完整身份/applied_at摘要及维护open/无flag均保持。首次预检误按 `docker-compose.yml` 查找测试入口失败，随后只读核对实际文件名并纠正，不修改现场。

复用第8节OBS定向验证与独立复核及跨KIN负向证明；不重跑未变应用定向集。新发布必须对最终personal完整SHA执行一次必要Full，旧Full38274106520不代替新门禁。Full失败先诊断失败作业，输入未变的成功检查不重复；没有新KIN基线、安全扫描例外或迁移身份改动。

## 部署控制定向修复

中断前独立复核发现两份 `?raw` 合规文档在frontend目录外，旧OBS证明未绑定其嵌入字节。新回归使用真实Git后继，仅改变英文/中文文件内容、mode或type；原行为1顶层/6子例均因未拒绝漂移失败。修复在GitEvidence和唯一canonical OBS proof增加两条精确路径及mode/type/object摘要 `70666fc568db23059802282f36ec4d3fa25a3a2cc0fdc3bbd56c8a24a5808942`，原429与OBS423输入相同；维护、remote、运输通过共同validator消费，重算hash伪装也不能改变固定字段。

修复后直接OBS13顶层与既有维护6项通过；保留原106门禁及未变应用证据，不声称重新跑过整套106。历史KIN/累计/安全补丁常量及canonical proof原值保持。中断复核没有最终交付，不计候选通过；当前最终fresh独立复核完成后再固定发布源码。

最终fresh复核随后确认维护开放前权限元数据漏检：健康验收期间仅chmod选择/成功记录、内容不变时会开放。真实隔离回归1顶层/3子例在旧行为RED（包含同字节记录目录改symlink）；统一私密binding reader在首次和开放前复核普通父目录/文件、0600与执行者ownership。修复后该回归及直接维护/完整bundle7项、原维护6项GREEN，Git/部署/应用未变证据复用，不重跑13全套或Full。前一复核交付已接受，但候选有P2不记为发布复核通过；需要对该最小增量fresh最终复核。

最终fresh只读复核已对修复后的tree `d7b9e4f1822d1e44f0e6885ea659e9103e70685b` 完成，无确认可行动问题；源码、semantic index及binary index均与冻结值相同，操作输入9项相同。第一轮发现P2并修正，不能把交付验收通过解释为旧候选无问题。所有权分支静态核对，未真实chown；实际Linux/生产操作仍待执行。当前fresh审计 `20261013T012441Z-obs-release-resume-final-review-a252d3c7`已关闭并校验通过；此前中断Bundle配置摘要变化，原计划保持open，不重写历史或聚合为通过。复核后仅追加本段记录，不改已复核控制字节。

## 唯一一次实际 Full 与失败停止（2026-10-13 UTC）

已普通快进推送 main 控制提交 `54cd703ea9ac2a79f3091b53f02ddd97c5445071`、personal 同步提交 `e37a4e6ec0a880583f9fe3a18eef93af70bf4b21`；后端运行树、完整frontend、两份raw文档摘要与审定值相同。main部署工具CI [38332961520](https://github.com/ccisnoxx/sub2api/actions/runs/38332961520)成功。对当时最终personal完整SHA只dispatch一次[Full38333250481](https://github.com/ccisnoxx/sub2api/actions/runs/38333250481)，`2026-10-13T01:40:28Z`开始、约01:46结束，run attempt1，最终failure。

直接失败为：

1. release-helpers的118项release-tools测试仅 `test_python_install_requires_binary_hashes` 失败：旧工具恢复workflow在83c250新增第五处pip安装，旧测试仍断言四处。全部五处仍有 `--require-hashes` 和 `--only-binary=:all:`。已在主控制工作树准备数量4→5的最小校正；同一Python3.12.13/PyYAML6.0.3下单项RED准确复现5!=4，GREEN通过，未修改安装命令、requirements、发布权限或源码绑定。
2. golangci-lint报告OBS `service_status_aggregator_monitor_test.go:89/206` 两处直接类型断言结果未检查（errcheck），仍未修复。需要显式comma-ok及断言，并保留失败/恢复合同；不得关闭lint规则。
3. backend unit进程在60s包预算超时：`TestServiceStatusAggregatorPreflightDeadlineUsesSharedBudget`的真实45s前置deadline测试之后，`TestServiceStatusAggregatorRoundDurationIncludesConfigPreflight`又需41s，当包超时触发时后者仅运行7s。`tools/test-backend-unit.py`只分片service/handler，repository以单进程执行。需要在保持真实45/40秒行为合同和每进程60秒限制的前提下修复测试执行边界；没有重试这项超时，也未简单提高全套timeout。

binding、sync-contracts、changes、docs、shell、frontend、backend-security及frontend-security成功；ci-ready、full-ci-certified和personal-ready为上述直接失败的传播。单独成功的检查可作未变输入证据，不构成规定的同一最终SHA成功Full。根据CI-000，旧失败run不能重跑成新SHA、不能拼接部分结果认证、不能为发布跳过失败job；后续修复会改变最终personal SHA，需要先修复和定向验证，再重新固定真正的发布候选。当前按维护者“实际失败停止相关步骤”要求停止发布/部署，没有第二次Full。

安装数量最小修正收到fresh只读critical_reviewer最终交付，无确认发现，候选文件sha256 `4f78161dc6fedca4ba8f76911d5e2aeb5ba44eed3eca5a2defc55a99665972ff`。补充复核Bundle `20261013T014327Z-obs-full-helper-test-fix-d8719825`在summary/digest时因会话配置摘要129741→c167f2变化，严格校验报 `generated plan field does not match deterministic output: codex_config_evidence`；保留原plan/execution及配置漂移证据，Bundle保持open，不能聚合为审计通过或加入有效统计。前述raw/维护最终复核的已关闭Bundle及其历史Digest仍保持原验收，没有重写。

原工作区暂存/未暂存binary diff摘要保持原值；原tps.1 Release410901327正文及12个asset的id/name/size/digest逐项保持，tps.2 tag未创建，发布变量仍false。测试/生产仍为429镜像，未pull新镜像、部署、安装timer、生产新备份或注入故障；没有通知配置。旧恢复点、维护open状态及跨KIN回退禁令保持。原始日志/定向结果/配置漂移记录保留在本机私密证据目录 `20261012-obs-release`；不导出凭据或原始生产请求日志。

## 本轮验收状态

- [ ] 固定最终源码/控制revision并完成一次成功发布Full；
- [ ] 不可覆盖的新tag/镜像/Release，源码/Full/工具/digest绑定；
- [ ] 既有独立测试环境的健康、登录/权限、隐私、转发/计费/用量及完整聚合周期；
- [ ] 生产新备份、迁移影响核对、维护冻结、单向部署及回环/公开验收；
- [ ] 新producer实际结构化日志、本机水位定时检查和容量探测；
- [ ] 更新支持版本/来源/Release说明和第8节状态，同步两分支；
- [x] 外部通知按维护者明确需求不适用；没有配置路由或发送通知，不宣称送达。

## 回退边界

保留原 `v0.2.15-klno.2-tps.1` / 429 / Full38274106520 / index `sha256:fa433c2b36935717ecd7a84641998c88c02fc0d4a14b06f31009a0d525c995d3`、旧KIN稳定tag/digest及所有历史恢复点。本次部署证明保持保守单向，失败留证、维护关闭后向前修复；完整数据库恢复需另行明确快照后写入损失，先隔离恢复核对再受控切换，不在线覆盖或仅切旧镜像。实际新备份/部署身份在相应步骤成功后补入本记录。
