# UPSTREAM-010：KIN v0.2.18-klno.3 升级

日期：2026-10-25（Asia/Shanghai）。用户授权追更上游并升级 hostdzire。当前为候选记录，正式发布与生产验收尚待完成。

- 上游标签对象 `9d4396d87459df5aec630b19a1a79879fffbb241`，peeled commit `35aae52f4218b6410f959ac46a13de1f05a1e272`。
- personal 基础 `8e064ba4436fab2dbccb1ee132dc0a69166152ae`；线上旧发布 `ceb2147b933035c5b50345c6e779b12112c097ac / 0.2.17-klno.1-tps.1 / sha256:e1d169160facdcf348a9341b6336730666f2f33dfdc71792a69ab7fb6150e3d5`。
- 上游再次改写历史，真实 merge 以旧来源 `cf0bf0526704bb615745fda76009cb20d85df613` 为逻辑三方基线，保留旧 personal 与目标 KIN 祖先。8处文本冲突中，3份工作流保留可信 main，VERSION保留个人RC，Wire重新生成，SSE新增queued/metadata但keepalive仅属于metadata，双语同时注册个人serviceStatus与新codexClientProfile。
- 应用锚点 `742952d420dd7d92ff13ff119a05658dd48f006a`：只追加246，旧315份SQL身份/字节和runner保持。迁移会启用存量Gemini/Antigravity分组生图，不影响composite/openai；重复启动不覆盖管理员后来关闭的值。历史迁移夹具明确排除新SQL，新增315→316路径。
- 部署合同绑定162条完整运行路径差异、42条前端路径差异、来源/锚点/全部构建输入与唯一新增SQL。完整315行旧账本现场读取摘要与成功记录相符。只替换应用，保留生产PG/Redis身份，单向升级且禁止旧镜像恢复。

已执行：后端定向192+398项通过；前端4文件58项及typecheck通过；新部署合同14项通过。初次pnpm使用环境11.25与项目锁定版本不符，改为corepack遵从9.15.9后frozen-lockfile成功，锁文件未变。本机Docker不可用，未把真实迁移本地命令当作通过。定向race（service与repository）、完整部署回归通过。应用独立复核742952d4与部署合同独立复核均无剩余确认阻断；迁移夹具问题已修正并复核。手动Full固定run40159916483，实际镜像隔离验证仍在进行。

线上凭据、数据库/应用备份和私密验收细节只保留服务器，不进入Git/Actions。
