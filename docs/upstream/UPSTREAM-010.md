# UPSTREAM-010：KIN v0.2.18-klno.3 升级

日期：2026-10-25（Asia/Shanghai）。用户授权追更上游并升级 hostdzire。正式个人版本已发布并完成hostdzire生产验收；最终身份及实际结果见[发布验收](records/2026-10-25-kin-0.2.18-release.md)。下列为候选准备过程记录，不替代最终结果。

- 上游标签对象 `9d4396d87459df5aec630b19a1a79879fffbb241`，peeled commit `35aae52f4218b6410f959ac46a13de1f05a1e272`。
- personal 基础 `8e064ba4436fab2dbccb1ee132dc0a69166152ae`；线上旧发布 `ceb2147b933035c5b50345c6e779b12112c097ac / 0.2.17-klno.1-tps.1 / sha256:e1d169160facdcf348a9341b6336730666f2f33dfdc71792a69ab7fb6150e3d5`。
- 上游再次改写历史，真实 merge 以旧来源 `cf0bf0526704bb615745fda76009cb20d85df613` 为逻辑三方基线，保留旧 personal 与目标 KIN 祖先。8处文本冲突中，3份工作流保留可信 main，VERSION保留个人RC，Wire重新生成，SSE新增queued/metadata但keepalive仅属于metadata，双语同时注册个人serviceStatus与新codexClientProfile。
- 应用锚点 `7a6fef9514a8cf68ba69950aa1e4a6f5a335c2d5`：只追加246，旧315份SQL身份/字节和runner保持。迁移会启用存量Gemini/Antigravity分组生图，不影响composite/openai；重复启动不覆盖管理员后来关闭的值。历史迁移夹具明确排除新SQL，新增315→316路径。
- 部署合同绑定163条完整运行路径差异、42条前端路径差异、来源/锚点/全部构建输入与唯一新增SQL。完整315行旧账本现场读取摘要与成功记录相符。只替换应用，保留生产PG/Redis身份，单向升级且禁止旧镜像恢复。

已执行：后端定向192+398项通过；前端4文件58项及typecheck通过；新部署合同14项通过。初次pnpm使用环境11.25与项目锁定版本不符，改为corepack遵从9.15.9后frozen-lockfile成功，锁文件未变。本机Docker不可用，未把真实迁移本地命令当作通过。定向race（service与repository）、完整部署回归通过。应用独立复核742952d4与部署合同独立复核均无剩余确认阻断；迁移夹具问题已修正并复核。最终手动Full固定run40160265206，实际镜像隔离验证仍在进行。最终锚点与部署指纹增量已独立复核，新14项部署合同再次通过；原162项完整回归未重复。

线上凭据、数据库/应用备份和私密验收细节只保留服务器，不进入Git/Actions。

首次Full40159916483的release-helpers被deploy/Dockerfile旧Go1.27.2固定镜像阻断，已对齐官方Go1.27.3 index digest并通过10项工具链定向检查；应用仅该构建输入与journal增量，专属部署锚点/指纹随最终输入更新，旧Full不得复用。

隔离项目sub2api-kin018-validation已启动固定旧版，独立PG/Redis/网络/卷，应用只映射127.0.0.1:10468；315行账本、真实登录/SSE、用量3/1及唯一计费去重通过。合成上游初始HTTP被旧应用拒绝，已改用隔离测试TLS及显式CA信任，未关闭TLS验证；验收关联采用响应返回的ClientRequestID。生产应用和依赖未更换。
