# 前端许可材料维护

正常 `pnpm build` 使用 `vite-plugins/thirdPartyLicenses.ts`，在后端嵌入的前端产物目录生成 `THIRD_PARTY_LICENSES.txt`。范围由所有输出 chunk 的 `modules` 决定，包括动态加载的 chunk。完全移除的空 JavaScript 模块不列为分发依赖，模块中的 CSS 因输出到独立 asset 而保留归属。许可收集在 Vite 删除纯 CSS 的空 JavaScript 包装 chunk 之前进行，避免遗漏动态加载样式。依赖包自己的 `node_modules` 不作为当前包的许可文本扫描范围，未进入产物的安装依赖不列入材料。

插件保留包的名称、精确版本和 `package.json` 中的实际许可声明，递归收集包内的 LICENSE、LICENCE、COPYING 和 NOTICE 文件。声明未知、完整许可文本缺失或文本为空会阻止构建。不得用通用许可模板猜测第三方版权归属，也不得将一个包的文本套用到另一个包。

`ModelIcon.vue` 已注明 SVG 路径摘自 `@lobehub/icons`，`ProviderIcon.vue` 又复制了其中的路径。两者在 Vite 配置中显式登记为复制素材，因此即便没有 JavaScript 包导入，产物仍包含当前锁定 `@lobehub/icons` 包的 MIT 许可和 LobeHub 版权原文。登记不引入该包的其他 React 依赖。

## 已核实的补充材料

`@vue/devtools-api@6.6.4` 的 npm 包声明 MIT，但未携带 LICENSE。补充的 [原始 LICENSE](./vue-devtools-api-6.6.4/LICENSE) 保持官方文本不变，来源和精确版本关联如下：

- [该版本 npm 元数据](https://registry.npmjs.org/@vue%2fdevtools-api/6.6.4) 的 `gitHead` 是 `df6ab6bb7791a7a525a97990de73b3ea5e9a1941`；该提交的源码包版本仍是 `6.6.3`，因此补充材料采用官方 `v6.6.4` 发布 tag。
- [官方 tag 引用](https://api.github.com/repos/vuejs/devtools-v6/git/ref/tags/v6.6.4) 指向 tag 对象 `5b3bd4b362556fc58ef948b459ca7b52a207d3b3`；[该 tag 对象](https://api.github.com/repos/vuejs/devtools-v6/git/tags/5b3bd4b362556fc58ef948b459ca7b52a207d3b3) 指向提交 `dd2ab5d4275187cbd81952a2a3d53c335643c1f4`。
- [固定发布提交的包元数据](https://raw.githubusercontent.com/vuejs/devtools-v6/dd2ab5d4275187cbd81952a2a3d53c335643c1f4/packages/api/package.json) 确认名称 `@vue/devtools-api`、版本 `6.6.4`、许可 `MIT`。
- [固定发布提交的完整 LICENSE](https://raw.githubusercontent.com/vuejs/devtools-v6/dd2ab5d4275187cbd81952a2a3d53c335643c1f4/LICENSE) 包含 `Copyright (c) 2014-present Evan You`，SHA-256 是 `050bbca6960784db52ff387271bf2ecc5cbed7cf8581b415d528a6ecb6585015`。
- 补充文件仅对配置登记的 `6.6.4` 生效，升级后必须重新核实，不会自动沿用旧版材料。

## 尚待解决的上游材料缺口

2026-10-10 的真实生产构建确认以下包进入分发 chunk；两个 npm 包均未携带 LICENSE/COPYING/NOTICE，公开源码和完整版权文本尚未核实。构建会明确失败，不能将此候选当作可发布产物：

| 精确包版本 | 实际 npm 许可声明 | 已核实来源 | 缺口 |
| --- | --- | --- | --- |
| `@airwallex/components-sdk@1.30.2` | `MIT` | [精确 npm 元数据](https://registry.npmjs.org/@airwallex%2fcomponents-sdk/1.30.2)、[精确 npm tarball](https://registry.npmjs.org/@airwallex/components-sdk/-/components-sdk-1.30.2.tgz) | 完整 MIT 许可及版权通知；元数据未给出源码仓库、作者或 gitHead |
| `@airwallex/airtracker@3.2.0` | `ISC` | [精确 npm 元数据](https://registry.npmjs.org/@airwallex%2fairtracker/3.2.0)、[精确 npm tarball](https://registry.npmjs.org/@airwallex/airtracker/-/airtracker-3.2.0.tgz) | 完整 ISC 许可及版权通知；元数据给出 gitHead `d7cc7ca00bd15a6c67b2de25e73814c475b18778`，未给出可核实的公开源码仓库 |

解除阻塞需要取得对应版本的官方完整许可及版权材料，保留原文字节，放入本目录，并在 `vite.config.ts` 的 `supplements` 中登记名称、精确版本、文件路径和固定来源 URL；然后运行定向测试和真实 `pnpm build`。只知道 MIT/ISC 名称，或找到同厂商其他项目的 LICENSE，均不能补齐这些包的文本。

定向验证命令：`pnpm exec vitest run vite-plugins/thirdPartyLicenses.spec.ts`。它通过真实 Vite 编译的临时包验证许可产物、懒加载、未分发依赖排除、复制素材、文本保留、路径不泄露、缺失声明/全文失败和补充材料的精确版本限制。
