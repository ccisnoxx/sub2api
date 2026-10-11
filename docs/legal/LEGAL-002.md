# LEGAL-002：许可材料与对应源码审查

## 状态与范围（2026-10-10）

**未完成，禁止将此候选用于正式发布。** 已实现许可材料生成、归档校验与镜像打包候选，但实际分发的两个 Airwallex 包缺少可核实的完整许可与版权文本，前端构建必须失败。任务台账保持未勾选；真实发布归档和容器尚未获得成功 dry-run 证据。

开始前的原分支为 `codex/doc-002-install-sources`，HEAD 为 `e0f1643daef7847d8ff2c144de82b0ad758964ca`，工作树仅有暂存及未暂存的 `REMEDIATION_PLAN.md`。本候选在独立 `codex/legal-002-license-audit` 分支执行，基于 LEGAL-001 提交 `dbb4cb099f96218e7cb53040fadcd17467936541`；其 [PR #16](https://github.com/ccisnoxx/sub2api/pull/16) 尚未合并。未修改受保护分支、业务组件、依赖版本、lockfile、数据库、生产部署或已发布标签/镜像。

## 许可策略与文件头

根目录 `LICENSE` 为 GNU LGPLv3，README 的既有许可策略为 LGPLv3 或后续版本。其 Git blob `153d416dc8d2d60076698ec3cbfce34d91436a03` 在本候选及 LEGAL-001 固定的 KIN、Plus 和 personal 来源相同；本任务保持字节不变，SHA256 为 `a5681bf9b05db14d86776930017c647ad9e6e56ff6bbcfdf21e5848288dfaf1b`。

LGPLv3 文本引用并纳入 GPLv3 条款；新增 `LICENSE-GPL-3.0.txt` 保留 [GNU 官方完整 GPLv3 文本](https://www.gnu.org/licenses/gpl-3.0.txt)，SHA256 为 `3972dc9744f6499f0f9b2dbf76696f2ae7ad8af9b23dde66d6af86c9dfb36986`。新增副本没有改变项目的既有 LGPL 策略。

复用 [LEGAL-001 的固定来源及文件头审计](./LEGAL-001.md)：169 个维护中的应用改动文件及 4061 个 Plus 来源文件已核对，未发现需要恢复的版权/SPDX 声明。本任务未修改其中的业务源码，也未编造版权文件头。Dockerfile 的既有 Wei-Shaw maintainer 归属保留，OCI source 改为实际分发仓库。源码 Docker 开发入口默认未传 COMMIT/VERSION，仍不能作为正式来源证明；正式入口为受 SHA/CI 门禁约束的个人 Release 工作流，源码构建时须显式提供真实参数。

## 候选行为

- Vite 从所有产物 chunk（含动态加载）的实际模块收集精确依赖版本、许可声明、LICENSE/COPYING/NOTICE 原文；复制自 LobeHub 的图标另行登记。未分发的安装依赖不列入。文本缺失、声明未知及空材料会明确阻止构建。
- `release_matrix.py materials` 核对所选源码 HEAD，生成 `SOURCE.json`，包含完整 SHA、版本、对应源码树和源码归档链接，以及项目/第三方许可材料的 SHA256。文件是构建产物，不写入源码提交。
- tar.gz/zip 必须包含所选源码的 `LICENSE`、`LICENSE-GPL-3.0.txt`、`NOTICE`、生成的 `THIRD_PARTY_LICENSES.txt`、`SOURCE.json` 和对应平台二进制。收集及传输校验拒绝缺失、重复、非普通文件、来源不匹配及原许可字节变化。
- 容器上下文从已验证归档提取这些材料，保存于 `/usr/share/licenses/sub2api/`。正式工作流已有 revision/source/version 标签参数；GoReleaser 配置和 Release notes 使用完整 SHA，并给出源码树及下载链接。
- 老源码缺少材料时必须停止发布，不从较新工作流提交借用材料。解除阻塞后还须将 LEGAL-001/002 必需的源码材料和构建配置纳入 `personal` 应用候选并重新验证；当前 main 候选不会自动改变历史二进制或镜像。

## 已验证与缺口

- pnpm 9 冻结依赖安装成功；前端定向 Vite 测试 8 项通过，覆盖真实编译、完整原文、动态加载、复制素材、未分发依赖排除、路径不泄漏、未知/缺失文本失败及精确版本补充限制。
- 真实 `pnpm build` 完成 i18n、类型检查和模块转换，但在许可门禁失败。阻塞包为 `@airwallex/components-sdk@1.30.2`（npm 声明 MIT）及 `@airwallex/airtracker@3.2.0`（npm 声明 ISC）；未将同厂商其他项目许可或通用模板套用于它们。来源证据、Vue 官方精确版本补充材料和解除条件见 [前端许可记录](../../frontend/licenses/README.md)。
- 发布工具测试 29 项通过，其中 release matrix 14 项，覆盖 tar.gz/zip、源码/校验和绑定、许可负向检查、容器材料运输、dry-run 不推送及既有个人发布最后门禁。
- GoReleaser v2.18.3（官方下载 SHA256 已校验）：simple 配置检查通过；full 配置格式有效，但严格 `check` 因已有 `archives.format` / `format_overrides.format` 弃用字段返回非零。未扩大本任务去迁移整个发布配置。
- `bash -n .github/release-tools/release-images.sh` 与 `deploy/tests/docker-runtime-resources-test.sh` 通过。
- 完整 Release dry-run 的最终结果记录于 PR；没有成功前不得将 archive/OCI 的实际产物验收列为通过。未修改数据库或恢复工具，本任务不重复数据库恢复演练；已有恢复基线属于 PREP-001。

## 风险、回滚与后续

此候选的严格许可门禁会让当前生产前端构建失败，因此只供草稿审查，不可合并为可发布候选。需要取得上述两个精确版本的官方完整许可及版权材料，在前端配置中登记固定来源，再重新构建、验证实际归档/容器和对应源码链接。若选择替换 SDK，则需要另行明确产品行为范围，本任务不会自行删除支付功能。

无需数据迁移。若撤销本候选，关闭草稿 PR 或仅 revert 本任务提交；保留 LEGAL-001 归属材料与原 LGPL LICENSE。撤销会恢复缺少发行许可材料的旧行为，不能据此声称旧发布已经合规。不要重写历史标签或覆盖已有镜像版本。

## 验收

- [ ] 每个正式发布产物均能定位到精确源码提交：候选工具已实现绑定，真实产物及 personal 候选尚未通过。
- [ ] Release archive 包含许可证和第三方声明：定向归档测试通过，真实构建因第三方许可缺口受阻。
- [ ] 容器 OCI labels 包含 revision、source 和 version：配置已补齐，真实容器尚未验证。
- [x] 不删除上游已有版权或许可证信息：根 LICENSE 字节不变，来源材料和 maintainer 归属保留。
