# 已发布个人版本来源记录

核对日期：2026-10-12。只读 GitHub Releases API 返回以下三条已发布记录，均 draft=false、prerelease=true。tag commit 由远端 ref 核对；KIN 来源由同一个应用 SHA 的 deploy/personal-source.json 重建。Release 的 target_commitish=main 不作为应用身份。下面的 digest/run/平台来自当时 Release notes，本轮没有重拉 registry、下载旧归档或重跑历史 CI，不把这些记录当作当前 I 的发布资格。

| 实际个人 tag / Release notes | published_at UTC | 完整个人应用 SHA | KIN tag / SHA | Plus 四模块参考 |
|---|---|---|---|---|
| [v0.2.14-klno.3-tps.1](https://github.com/ccisnoxx/sub2api/releases/tag/v0.2.14-klno.3-tps.1) | 2026-10-08T18:14:36Z | `896de21b4be7f4ec4b4236f4df663b47371665b0` | `v0.2.14-klno.3` / `de08df02ae1d81668a22f798b398aa0438ac1276` | 本表四模块尚未引入，不适用；不据此断言整棵树没有其他 Plus 来源 |
| [v0.2.14-klno.5-tps.1](https://github.com/ccisnoxx/sub2api/releases/tag/v0.2.14-klno.5-tps.1) | 2026-10-09T03:52:47Z | `9397eb8afb621aef483f2ec0bf4b2dd6247c7b92` | `v0.2.14-klno.5` / `c7aacf5d3ae383d0d5c75f471f66e61690a5701d` | 本表四模块尚未引入，不适用 |
| [v0.2.14-klno.5-tps.2](https://github.com/ccisnoxx/sub2api/releases/tag/v0.2.14-klno.5-tps.2) | 2026-10-10T12:03:11Z | `3d5e1fde82707900a21f2b5538b112c7bab3c04a` | `v0.2.14-klno.5` / `c7aacf5d3ae383d0d5c75f471f66e61690a5701d` | `v0.2.14+custom.002`，标签 commit `a7749f5826ec0a5c6493c47fde9474dc31525f14`，固定阅读 `90da415c62b94c9417d9ce2b72b1507ed22f0303`；仅计时、目录、诊断、状态四模块的结构/概念参考 |

KIN 仓库三条均为 `KlN-4096/sub2api`。历史 Notes 未显式列出完整 KIN/Plus 字段，本表注明上述源码重建来源，不改写既有 Release。四模块引入边界分别是计时 `12499e1a78fd3fee5d2afd61559d73bd8e770136`、目录 `49c209a77059d2927f78bbd31c1dd479b509dc6f`、诊断 `ab4a3f5ce058b28cc3139e5e60297ab4c264ffb1`、状态 `35962a802fdc499639c9c861072f25774ccba50b`；前两版均不是这些提交的后代，tps.2 是四者后代。归属/本地适配见 [固定 main 的 PREP-003 来源矩阵](https://github.com/ccisnoxx/sub2api/blob/ab86413aa27848239d0374adeff9f7aeaa41609e/UPSTREAM.md)。

## 当时产物与运行身份

历史 Notes 的 workflow_sha 与应用 SHA 相同，表示当时使用应用分支 workflow 的旧流程；本表未取得独立 main 控制 SHA，不用当前 C 补造历史字段。当前发布另按 main 手动控制与固定 Full 执行。

| 版本 | Notes workflow_sha | CI / Release run | 平台 / Notes image digest |
|---|---|---|---|
| klno.3-tps.1 | `896de21b4be7f4ec4b4236f4df663b47371665b0` | [37819628097](https://github.com/ccisnoxx/sub2api/actions/runs/37819628097) / [37822041580](https://github.com/ccisnoxx/sub2api/actions/runs/37822041580) | linux/amd64 / `sha256:f4a979fdeef6c79b982d16d77bc3a6c7b528164bd6ce5b1deb34a8f4981a3d76` |
| klno.5-tps.1 | `9397eb8afb621aef483f2ec0bf4b2dd6247c7b92` | [37879317375](https://github.com/ccisnoxx/sub2api/actions/runs/37879317375) / [37880741133](https://github.com/ccisnoxx/sub2api/actions/runs/37880741133) | linux/amd64 / `sha256:a6a2a017622158b8813c9355db816d9cca0b37654553c5bacf8aa304b51f06e6` |
| klno.5-tps.2 | `3d5e1fde82707900a21f2b5538b112c7bab3c04a` | [38049101726](https://github.com/ccisnoxx/sub2api/actions/runs/38049101726) / [38050231176](https://github.com/ccisnoxx/sub2api/actions/runs/38050231176) | linux/amd64 / `sha256:7446ff8ebdddca5e60989f0a5b8dec670ce3a030a9c8472e2dd3c27986e21530` |

对应 image 均为 `ghcr.io/ccisnoxx/sub2api@` 加表中完整 digest。源代码、SOURCE.json/许可证与 OCI 必须在实际发布/使用时按原材料核验；此记录没有新产物验收结论。旧稳定回退选择 tps.2，但应用兼容、当时数据库恢复点与部署许可仍需按 [迁移记录](https://github.com/ccisnoxx/sub2api/blob/c54b9497d4bee04215384deb494c5467b917a7db/docs/upstream/records/2026-10-11-kin-v0.2.15-klno.2.md)及部署合同核对。

2026-10-12 当前 I 为 `bea1db12d5f4853c0b85b44840e15de6a74309b1`，源码 VERSION `0.2.15-klno.2-tps.1-rc.1`。上述 API 查询没有这个 RC 或新 KIN personal 的正式 Release；本表不生成新 tag/digest/run，也不表示 tps.2 是 I 的产物。

## 2026-10-13 追加：新KIN个人版本与旧429归档

上文三次API/Notes核对是其2026-10-12时点，不用于新版本发布资格。另有原 [v0.2.15-klno.2-tps.1](https://github.com/ccisnoxx/sub2api/releases/tag/v0.2.15-klno.2-tps.1) / `42907bcab278f93db48d7e1b39b28e28f664569e` / Release410901327 / Full38274106520 / index `sha256:fa433c2b36935717ecd7a84641998c88c02fc0d4a14b06f31009a0d525c995d3`，原12个assets与正文保留；其恢复/生产验收见REL-002/003，现归档。

当时 [v0.2.15-klno.2-tps.2](https://github.com/ccisnoxx/sub2api/releases/tag/v0.2.15-klno.2-tps.2) 为 `c08fba12eb4cd30e65c4177f3acbe20567ecd252`，published_at 2026-10-13T02:37:06Z，draft=false/prerelease=true，Release411195117/14 assets；[Full38335510831](https://github.com/ccisnoxx/sub2api/actions/runs/38335510831)和[Release38337243568](https://github.com/ccisnoxx/sub2api/actions/runs/38337243568)均attempt1 success，实际控制/部署工具 `90aea7ec626aeab41434d72739573aa51d6227ad`。Linux/amd64 index `sha256:c4ffab7a920f05564eafe055bdb3171772b145ca5e2ccda690fe8b554bf6fa25`，leaf `sha256:7e2d731fb3fd0421cc6c51e8f522f40e70306492f0a604a248483538f5e741e8`；KIN N、Plus P/四模块参考与NOTICE字段保持，没有升级基线。完整来源、未签名证明、实际迁移账本/备份/部署及本机监控见[OBS验收](2026-10-12-obs-release.md)。两个旧个人tag、镜像和恢复点继续保留，不能据归档说明执行被禁止的旧镜像恢复。后继文档HEAD不是上述两个发布源码。

## 2026-10-13 追加：Responses tps.3

当前 [v0.2.15-klno.2-tps.3](https://github.com/ccisnoxx/sub2api/releases/tag/v0.2.15-klno.2-tps.3) 的正式tag解析为 `8015ce827fe5de2c9e0962f3c98bb1bd2ea0ee90`，published_at `2026-10-13T07:46:14Z`，Release411420897、draft=false/prerelease=true、14 assets。[Full38361566801](https://github.com/ccisnoxx/sub2api/actions/runs/38361566801)与[Release38363998847](https://github.com/ccisnoxx/sub2api/actions/runs/38363998847)均attempt1 success；实际发布/部署控制 `5e24b02d7aeb32f440af7a3aafbb21394fa14dd8`。Linux/amd64 index为 `sha256:a216fd8fb138aa1889278df7fbc4a0dd24a09d974886f198cc5102eac6a1b627`，leaf为 `sha256:823ba2f40bbccf180fece07929ecb1a99c8a2e1cd0da9772c4165c7295c5b18e`。KIN N、Plus P/四模块参考与对应应用NOTICE保持。源码、控制、未签名证明、独立验证环境及实际生产备份/308账本/SSE/计费/聚合见[Responses验收](2026-10-13-responses-release.md)。C08/tps.2及更旧产物与恢复点继续归档保留；后继文档HEAD不冒充本次发布源码。
