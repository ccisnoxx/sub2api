# LEGAL-002：许可材料与对应源码审查

## 状态与范围（2026-10-11 UTC）

**未完成，任务台账保持未勾选。** 已实现许可材料生成、归档校验与镜像打包候选。此前实际分发的两个 Airwallex 包缺少可核实的完整许可与版权文本；用户随后明确授权禁用 Airwallex 支付并移除前端 SDK 与支付入口。当前候选按此授权移除分发依赖，保留严格许可门禁，仍须取得真实归档、容器及 `personal` 应用候选的验收证据后才能用于正式发布。

开始前的原分支为 `codex/doc-002-install-sources`，HEAD 为 `e0f1643daef7847d8ff2c144de82b0ad758964ca`，工作树仅有暂存及未暂存的 `REMEDIATION_PLAN.md`。本候选在独立 `codex/legal-002-license-audit` 分支执行，基于 LEGAL-001 提交 `dbb4cb099f96218e7cb53040fadcd17467936541`；其 [PR #16](https://github.com/ccisnoxx/sub2api/pull/16) 尚未合并。本次续作前候选 HEAD 为 `2b17e595471b0d01da7a9fb2764e41d11ed7adaf`、工作树干净。此次只增加已授权的 Airwallex 禁用、前端依赖及入口移除，以及 dry-run 镜像留存；未升级其他依赖，未修改数据库、生产部署或已发布标签/镜像。

## 许可策略与文件头

根目录 `LICENSE` 为 GNU LGPLv3，README 的既有许可策略为 LGPLv3 或后续版本。其 Git blob `153d416dc8d2d60076698ec3cbfce34d91436a03` 在本候选及 LEGAL-001 固定的 KIN、Plus 和 personal 来源相同；本任务保持字节不变，SHA256 为 `a5681bf9b05db14d86776930017c647ad9e6e56ff6bbcfdf21e5848288dfaf1b`。

LGPLv3 文本引用并纳入 GPLv3 条款；新增 `LICENSE-GPL-3.0.txt` 保留 [GNU 官方完整 GPLv3 文本](https://www.gnu.org/licenses/gpl-3.0.txt)，SHA256 为 `3972dc9744f6499f0f9b2dbf76696f2ae7ad8af9b23dde66d6af86c9dfb36986`。新增副本没有改变项目的既有 LGPL 策略。

复用 [LEGAL-001 的固定来源及文件头审计](./LEGAL-001.md)：169 个维护中的应用改动文件及 4061 个 Plus 来源文件已核对，未发现需要恢复的版权/SPDX 声明。本次支付修改保留维护中文件的已有版权声明；删除的 Airwallex 专属页面及测试没有版权/SPDX 文件头，未编造声明。Dockerfile 的既有 Wei-Shaw maintainer 归属保留，OCI source 改为实际分发仓库。源码 Docker 开发入口默认未传 COMMIT/VERSION，仍不能作为正式来源证明；正式入口为受 SHA/CI 门禁约束的个人 Release 工作流，源码构建时须显式提供真实参数。

## 候选行为

- Vite 从所有产物 chunk（含动态加载）的实际模块收集精确依赖版本、许可声明、LICENSE/COPYING/NOTICE 原文；复制自 LobeHub 的图标另行登记。未分发的安装依赖不列入。文本缺失、声明未知及空材料会明确阻止构建。
- `release_matrix.py materials` 核对所选源码 HEAD，生成 `SOURCE.json`，包含完整 SHA、版本、对应源码树和源码归档链接，以及项目/第三方许可材料的 SHA256。文件是构建产物，不写入源码提交。
- tar.gz/zip 必须包含所选源码的 `LICENSE`、`LICENSE-GPL-3.0.txt`、`NOTICE`、生成的 `THIRD_PARTY_LICENSES.txt`、`SOURCE.json` 和对应平台二进制。收集及传输校验拒绝缺失、重复、非普通文件、来源不匹配及原许可字节变化。
- 容器上下文从已验证归档提取这些材料，保存于 `/usr/share/licenses/sub2api/`。正式工作流已有 revision/source/version 标签参数；GoReleaser 配置和 Release notes 使用完整 SHA，并给出源码树及下载链接。
- Airwallex 新建订单在配置读取及订单事务前拒绝；所选实例的 provider 再次校验，避免通过其他 method 绕过。公开方法目录及启用类型不再提供 Airwallex。前端删除两个 SDK、专属页面和路由、新支付及新增服务商入口，并禁止旧订单快照重新打开付款窗口；保留历史类型、快照、订单查询、原实例配置、查询/取消/退款/通知签名验证与历史履约，不迁移或删除数据。
- 历史订单的用户支付方法可能是自定义别名，不能据此判断 SDK 归属。认证订单响应额外提供已持久化的 `provider_key`，以订单 snapshot 为先、列值为后，不推测未知服务商；不暴露 snapshot 或商户信息。恢复带 SDK 密钥的旧快照时先核实订单绑定，再由状态面板和 Stripe 页面拒绝真实 Airwallex provider；旧 localStorage 原文字节及匿名查询响应保持原样。
- dry-run 将实际 OCI 归档保留为短期 Actions artifact，供检查镜像配置中的 labels 及最终层中的许可材料；仅 dry-run 留存，不改变正式发布或 registry 推送行为。
- 老源码缺少材料时必须停止发布，不从较新工作流提交借用材料。解除阻塞后还须将 LEGAL-001/002 必需的源码材料和构建配置纳入 `personal` 应用候选并重新验证；当前 main 候选不会自动改变历史二进制或镜像。

## 已验证与缺口

- pnpm 9 冻结依赖安装成功；前端定向 Vite 测试 8 项通过，覆盖真实编译、完整原文、动态加载、复制素材、未分发依赖排除、路径不泄漏、未知/缺失文本失败及精确版本补充限制。
- 历史提交 `2b17e595471b0d01da7a9fb2764e41d11ed7adaf` 的真实前端构建和 [Release dry-run 38113665007](https://github.com/ccisnoxx/sub2api/actions/runs/38113665007) 均因两个 Airwallex 包缺完整许可而失败。精确 npm tarball、GitLab TLS 失败及 CDN 缺材料的复核未取得官方原文，没有将同厂商其他项目许可或通用模板套用于它们。来源证据与本次移除范围见 [前端许可记录](../../frontend/licenses/README.md)。历史 [CI 38113651458](https://github.com/ccisnoxx/sub2api/actions/runs/38113651458) 成功不能替代失败的 Release 验收。
- 本次后端定向服务测试 36 项、provider 测试 9 项通过，覆盖新支付拒绝且没有订单、审计或外部 HTTP 副作用，以及历史查询、退款和通知签名。新增拒绝/目录回归在修改前按预期失败、修改后通过。前端实现定向 11 文件 118 个不同用例、类型检查和 ESLint 通过。独立复核发现并修复历史别名订单误入 Stripe 的边界后，重新运行直接受影响的 4 文件 67 用例、handler 身份/匿名查询和 service 历史 snapshot 定向测试通过；别名快照、订单 DTO 和 Stripe 启动回归先按预期失败。合法 Stripe 支付宝订单仍可启动。
- pnpm 9.15.9 冻结 lockfile 校验和真实生产脚本通过（i18n 3 用例、`vue-tsc -b`、Vite 5.4.21）。实际许可文件含 28 个包、85,857 字节，SHA-256 `6c2468b5b42456ee43bfd60daa0de8631b073186692779cc57625861d6905800`；133 个 JS chunk、133 个 sourcemap 的 618 个来源均无上述两个 SDK，专属页面 chunk 为0，Stripe chunk 保留。lockfile 仅删除两包共15行，其余锁定内容逐字不变。
- 发布工具测试 29 项通过，其中 release matrix 14 项，覆盖 tar.gz/zip、源码/校验和绑定、许可负向检查、容器材料运输、dry-run 不推送及既有个人发布最后门禁。
- GoReleaser v2.18.3（官方下载 SHA256 已校验）：simple 配置检查通过；full 配置格式有效，但严格 `check` 因已有 `archives.format` / `format_overrides.format` 弃用字段返回非零。未扩大本任务去迁移整个发布配置。
- `bash -n .github/release-tools/release-images.sh` 与 `deploy/tests/docker-runtime-resources-test.sh` 通过。
- 本次 dry-run 留存步骤的 YAML 及 guard/name/path 校验通过，上传参数按官方 `actions/upload-artifact@v7` schema 核对。最小 Linux amd64 Release dry-run 绑定最终候选 SHA，其最终结果与实际归档、OCI 的下载核验记录于 PR 和任务台账；没有成功前不得将实际产物验收列为通过。未修改数据库或恢复工具，本任务不重复数据库恢复演练；已有恢复基线属于 PREP-001。

## 风险、回滚与后续

Airwallex 不再接受新付款，历史未付款订单也不能从本应用重新启动支付；这是用户明确授权的行为变化。历史订单、实例配置、回调和退款保留。两个 SDK 的官方完整许可仍未取得；本次只因停止分发它们而解除相应构建阻塞，不能声称已补齐其许可。当前草稿仍须通过实际产物核验并将必需材料纳入 `personal` 应用候选；不能将 main 候选证据套用于旧应用源码或历史产物。

无需数据迁移。若撤销本次禁用提交，将恢复 Airwallex 支付及缺失许可的 SDK，严格门禁会再次阻止前端发行；只有取得适用于精确版本的官方材料并重新验收后才能恢复发布。若撤销全部候选，关闭草稿 PR 或仅 revert 本任务提交，保留 LEGAL-001 归属材料与原 LGPL LICENSE。不要重写历史标签或覆盖已有镜像版本。

## 验收

- [ ] 每个正式发布产物均能定位到精确源码提交：候选工具已实现绑定，真实产物及 personal 候选尚未通过。
- [ ] Release archive 包含许可证和第三方声明：定向归档测试通过，实际产物核验见 PR 和台账，尚需 personal 应用候选证据。
- [ ] 容器 OCI labels 包含 revision、source 和 version：配置已补齐，实际产物核验见 PR 和台账，尚需 personal 应用候选证据。
- [x] 不删除上游已有版权或许可证信息：根 LICENSE 字节不变，来源材料和 maintainer 归属保留。
