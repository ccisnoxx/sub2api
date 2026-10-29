# 2026-10-29：退出个人应用补丁，KIN正式镜像已部署

维护者授权先用生产备份副本隔离验证，通过后为生产制作新备份并切换hostdzire，保留数据与配置。已完成隔离验证、可信工具合并/CI、维护冻结、新备份、仅应用替换、显式恢复及公开验收。不再重放个人应用补丁或发行tps版本。

## 正式身份

| 项目 | 固定值 |
|---|---|
| 上游发行 | [v0.2.19-klno.1](https://github.com/KlN-4096/sub2api/releases/tag/v0.2.19-klno.1) |
| tag object / peeled commit | 3a76dd83e16edddd1f217f29605b2e06c4f4bc3b / bdff1129ffbd22947612f83cd09d23bf3fa47f5d |
| 实际镜像 | ghcr.io/kln-4096/sub2api@sha256:28076fa6f15b5a74dc5b6ce6a716f3909955b60a74de2d09801a1f4b8fc7d86a |
| 平台及OCI | linux/amd64；version=0.2.19-klno.1、revision=bdff1129…、source=https://github.com/KlN-4096/sub2api，实际运行/镜像标签一致 |
| 旧应用 | 6acab6c3a68eca9e7511bc6f34f9d0fd3e69276b / 0.2.18-klno.3-tps.1 / digest998adb232164ca58da43046ea2de24336f9b2e653d10f34e3a2739f52ed824d1 |
| 可信main工具 | cdf8384a412ad2a49f97dfe498bfa48b66ed4ce7；[PR63](https://github.com/ccisnoxx/sub2api/pull/63)，运行实现096825673203ab0f88f6520d55836fe407b6961e |
| 工具CI | [40826616115](https://github.com/ccisnoxx/sub2api/actions/runs/40826616115)，success；本地新增6项及完整168项通过 |
| 生产成功记录 | 20261029T060321Z-510245ba9010 |
| 应用实际启动 | 2026-10-29T06:03:44.149011229Z，即北京时间14:03:44 |

上游[Release运行40812543398](https://github.com/KlN-4096/sub2api/actions/runs/40812543398)为同一bdff1129…且success。直接采用上游正式镜像，没有在本fork构建、发行新应用或运行个人应用Full。main新增的只是本次固定旧、新来源及316→319账本的单向部署合同，保留可信bundle、锁内启动前备份、漂移检查、失败关闭和原子selection。独立只读复核未确认阻断；恢复流量前补做PG/Redis身份与成功记录的现场比对，未扩大既有维护工具。

## 生产备份副本隔离验证

私密目录 `hostdzire:/root/sub2api-upstream019-validation` 保留生产快照、脚本及验收证据，不上传Git或Actions。初始数据库dump97468352 bytes / SHA256 cfcd890481e0118ca5091f6b41ac5a7863391bb4fae6a42d907aa8bb85647405；应用归档9990046 bytes / SHA256 5c949c41c61b80751e80fe556b7e54de34ca63544e1aae752291e460bc9e1299。

- dump结构读取、应用tar/gzip与EOF/CRC检查通过；真实pg_restore --exit-on-error恢复至独立PG，使用独立Redis及应用数据。原316行账本完整一致，正式镜像健康后为319行，原行全部保持。
- 升级前后原用户5、账号4、API Key11、分组6、用量227711、计费去重234171、Ops错误1232、个人状态facts19523和事件45的记录数保持。随后才在副本加入测试夹具。
- Docker internal网络阻止外部访问，避免副本后台任务和复制凭据作用于真实上游。管理员/历史用量API、正式镜像重启与JWT保持通过。
- 环境文件不含管理员密码。隔离密码登录使用副本中临时测试口令，保留原管理员身份与既有合规确认，测试后恢复原口令哈希；生产凭据、权限和确认状态未改。新建夹具管理员返回423，证明合规确认门禁仍生效，没有自动接受协议。
- 内部合成上游的真实HTTP/SSE转发通过：3 input/1 output、actual_cost=0.00000105、billing dedup恰好1。该结果只证明隔离转发/结算链路，不代表真实提供商性能。
- 设施错误已诊断后修复：Python3.11不支持tar filter参数；PGDATA挂载层级；internal网络不发布主机端口，HTTP改从容器内验收。均未改生产。原始脚本和私密结果保留。

## 本次生产备份、切换与保留

可信main完整bundle先freeze既有REL-002站点，公开健康返回503；没有重新setup或更改域名、证书、私钥。随后执行器在同一部署锁内制作并验证本次新备份，才替换应用：

| 备份 | bytes | SHA256 |
|---|---:|---|
| postgres.dump | 97486904 | 4fea01edcac3cde259ca63a3adfa3b45efc9c9a98e7c54641709b9e60318d3f0 |
| app-data.tar.gz | 9995882 | 866ec52e6245371f468367a270a0b71aacd09a43d189d558875caffc3d06c0f4 |

备份、Compose、私密环境、原镜像选择及审计位于 `hostdzire:/root/sub2api-kin/.personal-deploy/records/20261029T060321Z-510245ba9010/`。本次生产新dump通过结构检查，未将这份新dump再做完整隔离还原；实际全量恢复验证使用此前新取得的生产副本。两者边界区分，不覆盖生产数据库，不重复制作执行器已完成的备份。

Compose字节、.env字节、运行环境变量键值、应用持久卷均保持。环境变量数组顺序随镜像改变，按键值比较一致。生产PG ID仍为c98be88c92274954da5270fc801f16f8dc266299bdff0892427b19eb3b60a0ea，Redis ID仍为21ee6e287dbc786215a00d7265508bf644c792ca1132e6692d8d18c37a5857b9；没有重建依赖、删除生产卷/网络或并行运行共享Redis的新旧应用。

原316行账本SHA256为0f31040b9ad784825165747d29cc73ead2cd24e209156731eab4fa0fce180687；最终319行为b654b3d8663b144e13be37e38d87b46513e90659ee086d1d89198495be761b6e。只追加247_scoped_admin_api_keys、248_group_image_count_billing、254_add_usage_log_tps_inputs；同数字前缀不是同一filename，个人251–254原行/数据保持。agy分组生图开关仍为false。

个人服务状态专用timer已disable并停止，其service也停止，文件与历史数据保留。操作前后状态写入personal-monitor-retirement.json。使用同一可信bundle显式绑定bdff1129…和本次成功记录resume，运行digest/Image ID/OCI/健康/完整账本/selection以及依赖ID全部核对后开放。

## 公开验收与维护收尾

- 公开health与授权models均200，模型列表20项。
- 本机已安装Codex CLI0.164.0通过公开入口调用gpt-6.1-sol，ephemeral/read-only、不调用工具，exit0返回OK。对应usage234191：input21240/output5、actual_cost=0.17012、duration1834ms、首帧1011ms、首答1730ms、request_type=stream，唯一billing dedup。客户端与数据库Token一致，不据单次请求宣称性能改善或SLA。
- 首个CLI尝试在本机配置解析阶段因MCP配置覆盖形态失败，尚未发出模型请求；改用本次进程的空MCP表后通过。全局Codex配置未修改。CLI还输出本地状态库/实验功能警告，未影响请求完成，没有清理共享状态库。
- 原Chrome管理员会话刷新后正常显示v0.2.19 / klno.1，控制台和历史统计可读；未重设生产密码，也不声称新做了生产密码登录。个人服务状态导航随官方应用退役，站点品牌配置仍保留。
- 原渠道监控设置保持enabled=true、mode=v1，目前0个配置探针；Ops保持开启，模型目录原开关false保持。没有为此次切换另启V2或新增探针，不宣称完成不存在的监控周期或提供方全面健康。
- PERSONAL_RELEASE_ENABLED已从true改false，PERSONAL_SYNC_SCHEDULE_ENABLED从true改false，仓库移除个人补丁同步cron。personal、旧标签、镜像、许可和迁移历史归档保留。
- 本次4个隔离容器、专属internal网络及约2GB恢复副本目录已精确清理，生产快照/归档/私密配置/验收记录保留。原镜像与共享缓存不删除；本地临时生产API Key文件已移除。

本次保持image_rollback_compatible=false和schema_rollback=retain。异常时保留实际选择与现场并向前修复，不能自动启动旧个人镜像或覆盖生产数据库。覆盖生产数据及可能丢失快照后写入的恢复仍需另行明确授权。
