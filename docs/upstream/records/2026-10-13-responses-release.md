# Responses tps.3 发布与部署验收

2026-10-13 UTC，维护者明确授权将 personal 构建发布为新镜像、按仓库发布门禁检查并重新部署。已完成固定源码 Full → main 手动发布 → 独立验证环境 → 生产维护冻结/备份/单应用切换/开放 → 公开转发及完整聚合验收。产物只来自固定的已提交应用源码；本次没有从原工作区打包文件或混入其他未提交改动，原暂存、未暂存和未跟踪状态保留。

## 正式身份与来源

| 字段 | 实际值 / 证据 |
|---|---|
| 应用 | `personal` / `8015ce827fe5de2c9e0962f3c98bb1bd2ea0ee90` |
| 正式 tag / Release | [v0.2.15-klno.2-tps.3](https://github.com/ccisnoxx/sub2api/releases/tag/v0.2.15-klno.2-tps.3)，实际 Git tag commit 为上述应用 SHA；Release411420897，14 assets，published_at `2026-10-13T07:46:14Z`，draft=false/prerelease=true |
| 固定 Full | [38361566801](https://github.com/ccisnoxx/sub2api/actions/runs/38361566801)，attempt1、手动 personal Full、全部必要 job success，包括 personal-ready/full-ci-certified |
| 发布准备 / Release | [38363922287](https://github.com/ccisnoxx/sub2api/actions/runs/38363922287) / [38363998847](https://github.com/ccisnoxx/sub2api/actions/runs/38363998847)，均 success；后者 attempt1，prepare/frontend/linux-amd64/release 全部成功 |
| 实际 main 发布控制 / 部署工具 | `5e24b02d7aeb32f440af7a3aafbb21394fa14dd8`，Release guard 的 control-sha 与生产部署记录分别确认；后继文档提交不冒充当次控制 |
| OCI 平台 / index | linux/amd64 / `sha256:a216fd8fb138aa1889278df7fbc4a0dd24a09d974886f198cc5102eac6a1b627` |
| OCI leaf | `sha256:823ba2f40bbccf180fece07929ecb1a99c8a2e1cd0da9772c4165c7295c5b18e` |
| 固定运行镜像 | `ghcr.io/ccisnoxx/sub2api@sha256:a216fd8fb138aa1889278df7fbc4a0dd24a09d974886f198cc5102eac6a1b627` |
| KIN 来源 | 同一应用 SHA 的 [来源文件](https://github.com/ccisnoxx/sub2api/blob/8015ce827fe5de2c9e0962f3c98bb1bd2ea0ee90/deploy/personal-source.json)：`KlN-4096/sub2api` / `v0.2.15-klno.2` / `ca686de5e5ac14834e12701c156831abd5ec6b71` |
| Plus 来源及范围 | 同一应用 [NOTICE](https://github.com/ccisnoxx/sub2api/blob/8015ce827fe5de2c9e0962f3c98bb1bd2ea0ee90/NOTICE)：`v0.2.14+custom.002`，标签实际 commit `a7749f5826ec0a5c6493c47fde9474dc31525f14`，固定阅读 `90da415c62b94c9417d9ce2b72b1507ed22f0303`；计时、目录、诊断、状态的结构/概念参考，范围保持 |

Release 的 target_commitish=main 不是应用身份；实际 tag、镜像 revision 和供应链 summary 均等于8015。已回读正式 summary 与 published-image verification、核对各自 evidence checksum。证明仍为 unsigned，沿用固定 digest 与可信 Actions 运输验证，未宣称新增签名。当前入口只发布 Linux amd64 容器与证明，不上传二进制下载包。PERSONAL_RELEASE_ENABLED 初始 false，仅本次显式发布期间 true，发布成功后已恢复 false。

## 应用提交、门禁修复与部署证明

[PR50](https://github.com/ccisnoxx/sub2api/pull/50) 的三个独立提交保留：开场事件 `65ee9aa04bb1674a84d1c8874bc2235a6254200b`、软粘性分流 `97fdd5f01efafb663b828fcd15edf188f7d6c810`、失败用量计费 `d36d325f1907e1cd8cdef809a7fc353a864e7b68`。当前单账号不会产生跨账号分流；账号身份、指纹收敛与原有客户端门禁未改。

首个旧候选 Full38357512130 的 errcheck 失败后被后继运行取消；Full38358954162 仍发现两处测试断言漏检。默认同类诊断数量上限隐藏了后两处报告。已通过 [PR51](https://github.com/ccisnoxx/sub2api/pull/51) 与 [PR52](https://github.com/ccisnoxx/sub2api/pull/52) 仅修正测试显式返回值/类型断言检查，没有关闭规则或修改生产逻辑。最终默认 tags、golangci-lint2.14.0、不限诊断的三个包定向 errcheck 为0 issues，对应定向测试通过；随后固定8015执行上述完整成功 Full，不跨旧运行拼接资格。

[PR53](https://github.com/ccisnoxx/sub2api/pull/53) 在 main 增加唯一 canonical `obs-tps.2-to-resp-tps.3` 单向证明，固定实际 C08/tps.2/digest、8015锚点、完整 runtime `9f0490b6a346e885dc1e9ff4d9296b15e4f71161a4d9458072cf4ba18f170a48`、23条运行差异、完整 frontend/raw 输入与308账本。前端、schema和SQL迁移没有变化；242及251—254内容/普通文件模式保持。未知树、旧候选、伪装证明、反向或部分提交继续拒绝。工具离线120项通过，最终源重绑定后的RESP11项通过；两次独立只读复核无确认代码问题。合并main的 [部署工具CI38363364600](https://github.com/ccisnoxx/sub2api/actions/runs/38363364600) 成功。

## 独立验证环境与生产切换

既有隔离栈 `/root/sub2api-kin-s44-validation` 只更新app，使用正式不可变digest；postgres/redis/fixture容器身份及308行账本字节保持。私密证据目录 `.resp-release/tps3` 保留新compose override、record、账本和611906 bytes数据库dump，并通过pg_restore --list。新镜像管理员密码登录、管理员accounts、模型接口均200。

首个合成探针400的权威上游错误是 fixture_marker_required；读取归档fixture合同后仅将测试输入改成其既有标记，未改fixture或应用。修正后的真实SSE开场response.created为34ms、终态completed、唯一同一response ID，3 input/1 output token，用量记录与唯一计费事务均存在、实际成本0.00001375。fixture没有文本delta，因此第一段文本时间与严格首字保持NULL，不把合成结果当作真实上游性能基准。

新镜像已缓存后，可信main工具的生产只读dry-run验证C08→8015证明、OCI/source/平台、当前成功记录、依赖及完整308账本，无pull/备份/apply。随后既有REL-002站点freeze→生产预检/新备份→仅sub2api replacement→健康/账本/依赖→成功提交→绑定8015与新记录显式resume。生产部署记录为 **`20261013T075506Z-bf72a8e35e62`**，应用开始运行于 `2026-10-13T07:55:24.353781308Z`。生产PostgreSQL和Redis容器身份保持，完整账本仍为308行、filename/checksum SHA256 `a31c942e9e772bef1af9c68f930ee35afaa8c3591b5d1c22609f9bf7d30da882`。

新备份位于 `.personal-deploy/records/20261013T075506Z-bf72a8e35e62`：数据库64263194 bytes / `3c743cd8d12db5738ca997406451c49abcc72a789f933061924b7d70deaaa267`；应用数据10620670 bytes / `627873a956d3166278da07c3085b2e4762f2abe2ba0935237377041e96fb8252`，配置、私密环境与原选择同时保留。部署工具验证dump结构可读和应用归档完整性；本次未对新快照再作完整隔离还原，沿用未变迁移/schema的既有恢复边界。

本机输出摘要随后因读取远端简略结果不存在的compatibility字段报KeyError；官方部署执行已exit0、结果status=success。只读重核服务器正式记录的proof/成功/健康和回环health/models后恢复公开入口，没有重新执行apply。站点历史安装source429与83c250控制器保持；accepted_source/deployment明确更新为8015和本次记录，关闭标记已删除、phase=open。

## 公开转发、计费与聚合验收

公开 `https://api.962850.xyz` health/models均200。首个普通HTTP探针403对应原codex_cli_only、missing_engine_fingerprint，确认门禁仍生效；读取既有required x-codex信号，并使用本机已安装0.162.1的Codex协议头重测，没有调整任何账户、白名单、权限或指纹设置。这是HTTP协议探针，未宣称执行完整Codex CLI任务。

实际公开gpt-6.1-sol探针：response.created **697ms**、第一段文本 **3785ms**、包括用量核对的总时间4596ms，正常completed且开场/终态同一response ID。上游42 input/5 output token，与请求专属用量行一致；timing_version1、strict_first_token3710ms、usage_source=upstream_final、完成状态true，actual_cost **0.000134**、唯一usage_billing_dedup事务。客户端测量起点与内部计时起点不同，两种毫秒值各保留原口径。该单次结果证明开场在文本前透传，未测旧版本对照，不代表整体生成速度提升或长期延迟分布。

Chrome原管理员会话刷新后显示klno.2-tps.3，服务状态页面及配置版本2正常。本次生产沿用已有登录态，未新设密码，也未声称进行了新的生产密码登录。失败请求已观察usage的记录/计费错误路径由定向回归和Full保护，没有向生产注入流中断或计费故障。

应用启动后至08:03 UTC，实际9次aggregate success/stage=commit/scan_completed、9次cleanup committed/commit_confirmed和1次capacity success，固定ops_system_log_skip均true、无新聚合失败/告警。08:03水位已覆盖新进程之后完整五分钟以上，config/watermark版本均2，gap_code与last_error_code为空。旧source_error_at仍指03:29历史信号，未删除或伪造新值。生产持久timer在07:56—08:03的8次自动调度均current/空告警；没有以手动执行冒充定时监控。样本不足/未知继续保留原值，这只是有界上线观察，不宣称提供方SLA。

## 维护、审计与恢复边界

正式tag、镜像、Release/14 assets与旧C08/tps.2及更早产物均保留。本文与后继README/来源矩阵仅由main维护，不改personal发布SHA或镜像，也不复制原工作区未提交改动；后续追上游保留三功能独立提交。

本次委派审计 `20261013T063810Z-responses-image-release-01863edf` 已关闭并校验：3/3执行验收、2次独立只读复核；模型/档位只表示Agent TOML配置快照。补充原工具回执的follow-up没有前置guard，其实际发生记录保留，非新的实现或代码复核尝试。Bundle校验通过，额外补充回执文件提示unclassified artifact，不补造派发前验证。

当前canonical证明继续为 image_rollback_compatible=false、schema_rollback=retain。出现实际故障时用当前可信main bundle冻结并保留现场，向前修复；不会自动启动旧镜像或覆盖在线数据库。完整数据恢复须另行明确快照后写入损失与切换目标。服务器正式备份、部署和测试证据目录保留，本次临时脚本、下载和隔离本机工作树在交付时清理。
