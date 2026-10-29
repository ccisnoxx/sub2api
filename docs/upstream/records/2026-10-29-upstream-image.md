# 2026-10-29：退出个人应用补丁并切换KIN正式镜像

用户授权先用生产备份副本隔离验证，通过后备份并切换hostdzire，保留数据和配置，不再维护个人应用补丁。

## 固定输入

- 原应用6acab6c3a68eca9e7511bc6f34f9d0fd3e69276b / 0.2.18-klno.3-tps.1，原镜像 `ghcr.io/ccisnoxx/sub2api@sha256:998adb232164ca58da43046ea2de24336f9b2e653d10f34e3a2739f52ed824d1`。
- 上游v0.2.19-klno.1，tag object3a76dd83e16edddd1f217f29605b2e06c4f4bc3b，peeled commit bdff1129ffbd22947612f83cd09d23bf3fa47f5d。
- 官方镜像 `ghcr.io/kln-4096/sub2api@sha256:28076fa6f15b5a74dc5b6ce6a716f3909955b60a74de2d09801a1f4b8fc7d86a`；实际linux/amd64、OCI revision/version/source均一致。不自行构建。
- 当前成功记录20261025T044327Z-c82a22085b08，316行账本SHA256 `0f31040b9ad784825165747d29cc73ead2cd24e209156731eab4fa0fce180687`，运行/selection/记录一致。

## 候选验证

源树同名SQL checksum全部一致，4份个人扩展保留，新增247_scoped_admin_api_keys、248_group_image_count_billing、254_add_usage_log_tps_inputs；完整filename为身份，不因254前缀相同冲突。新工具只支持本次固定来源、完整树、镜像及316→319账本；失败禁止旧镜像恢复。

定向6项及完整部署工具168项通过，独立只读复核未确认阻断。复核另行指出既有维护恢复入口不重查PG/Redis ID，实际开放流量前须与成功记录再次比较。

私密生产快照位于hostdzire:/root/sub2api-upstream019-validation。数据库97468352 bytes/SHA256 cfcd890481e0118ca5091f6b41ac5a7863391bb4fae6a42d907aa8bb85647405；应用数据9990046 bytes/SHA256 5c949c41c61b80751e80fe556b7e54de34ca63544e1aae752291e460bc9e1299。dump结构与归档完整性检查通过。独立PG/Redis/应用数据完整恢复，原316行保持；官方应用健康后319行，原用户5、账号4、Key11、分组6、用量227711、计费去重234171、Ops错误1232及个人状态数据的记录数保持。网络internal限制外部访问。

验证设施错误已诊断：服务器Python3.11无tar filter参数，改为普通文件/目录及路径边界验证；PGDATA挂载层级残留空data目录，改用任务独立干净PGDATA；Docker internal网络不发布主机端口，改为容器内HTTP验收。环境文件ADMIN_PASSWORD为空，不能宣称既有密码登录通过；原Chrome管理员会话可正常访问生产。后续隔离登录使用仅副本中的测试夹具，生产凭据不变。

## 生产状态

尚未切换，等待隔离验收闭合及可信main工具固定。个人发布开关已从true关闭为false，定时个人补丁同步将移除；不运行个人应用Full或重新发行镜像。
