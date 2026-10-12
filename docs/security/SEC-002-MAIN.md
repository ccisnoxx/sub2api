# SEC-002：main 例外清理与定向验收续作

核对日期：2026-10-12（America/Los_Angeles）。固定 main 输入 `b4ecd4794e464b334a82cce63dc114ac93dcda75`，参照 personal `0dc428e8ad9156302660efcac994d0c986773504` 的既有 SEC-002 门禁。仅处理依赖审计例外、其缓解措施的执行入口和计划记录，不发布或部署。

## 实际依赖与例外范围

两分支锁文件只差 main 保留的 `@airwallex/components-sdk@1.30.2` 与 `@airwallex/airtracker@3.2.0`；没有用“锁文件相同”替代核对。使用 pnpm 9.15.9 分别离线冻结安装（`--ignore-scripts`），保持 manifest 的依赖区、overrides 和 lockfile 字节。main 的实际生产路径包含 axios 1.20.0、lodash/lodash-es 4.18.1、nanoid 3.3.20，以及直接 xlsx 0.18.5。生产 nanoid 来自 Vue compiler-sfc → postcss 8.5.29；开发 postcss 8.5.23 使用 nanoid 3.3.18。两份已安装 postcss 的 input.js 均调用 non-secure `nanoid(6)`，没有据此新增漏洞例外。

对两个实际安装分别向 npm 官方 registry 执行 `pnpm audit --prod --audit-level=high --json`，有效报告均为 **high 2 / critical 0 / moderate 13 / low 4**。两个 high 都是 `xlsx@0.18.5`、路径 `. > xlsx@0.18.5`，精确 GHSA 为 `GHSA-4r6h-8v6p-xvw6` 与 `GHSA-5pgg-2g8v-p4x9`。原 lodash、lodash-es、axios、nanoid 条目没有对应发现，删除四条例外，未升级依赖或延长例外。审计命令因两个 high 返回 1；后续精确例外校验通过，不将原审计退出码写成零漏洞。

有效 main 报告 SHA-256：`95d6173ac4de538f403cfae75c9c60fa24ed1b3b8a739c6189a3aec2a1d96113`；有效 personal 报告：`1826ec8179c46ca20a800c6a21e838180a12f9b47761d59f266e8cf801ad8070`。personal 首次未安装依赖时报告的 finding 路径为空，未计有效例外验收；冻结安装后重新取得完整路径并通过原严格校验器，没有放宽路径匹配。报告是本次时间点证据，不能承诺未来 registry/advisory 结果不变。

main 仅保留上述两项 XLSX 例外，owner 沿用 CODEOWNERS 中的 `@ccisnoxx`，到期日沿用 personal 的 **2026-11-09**，缩短原 main 的 2027-01-06 期限；限定实际版本和完整依赖路径。main 的 evidence 字段指向本记录，其余例外字段与 personal 一致。personal 的例外文件不变。

## 必要门禁与验证

main 的原校验器没有强制 owner、精确版本/路径或有效报告结构；仅改 YAML 无法落实所声明的范围。复用 personal 已验证的校验器、14 项既有政策测试、XLSX 写出规则及 33 项既有规则测试，四份实现/测试文件逐字相同，未新建另一套策略。保留 Python 原 CRLF。

main 的 `exportToExcel` 仍仅从管理员 API 字段数组构造工作表，调用 `aoa_to_sheet`、`sheet_add_aoa`、`book_new`、`book_append_sheet`、`write`；两分支当前源码扫描均通过。静态规则限制导入位置、解析 API、模块引用逃逸和 Vue/TypeScript 变体，不把后台字段值称为完全可信文件输入。

两分支的 Security Scan 原已执行审计校验，但缺少历史报告所述的静态规则/政策回归入口。本次仅在原 Check audit exceptions 步骤补入三条命令，保持触发器、权限、工具版本、扫描等级、审计失败传播和发布流程：

```bash
python tools/test_check_pnpm_audit_exceptions.py
pnpm --dir frontend run test:xlsx-write-only
pnpm --dir frontend run check:xlsx-write-only
```

实际定向验证：

- main 原校验器执行上述既有 14 项政策测试，出现 19 个失败断言（含子测试），确认报告错误、占位 owner、版本/路径及漏项等缺口；同步后 14 项全部通过。
- main XLSX 规则回归 33 项通过；main 与 personal 全部前端源码的 XLSX 写出边界扫描通过。
- 两份有效实际生产审计报告分别由各分支严格校验器匹配通过。
- actionlint v1.7.12 对修改的 Security Scan 通过，未运行 ShellCheck/pyflakes；两分支工作流字节相同。新增脚本由已有 ESLint/Vue/TypeScript 依赖运行，不新增依赖。
- `git -c core.whitespace=cr-at-eol diff --check` 通过。安装环境 Node v24.16.0、Python 3.12.13；未将此本地检查写成 CI Node 20.20.2 的云端验收。

没有运行后端回归、全量前端测试/构建、云端 Quick/Full、安全工作流 dispatch、Release dry-run、镜像构建或数据库演练。未增加或修改应用业务代码、SQL、迁移身份、源 manifest、发布 tag 或镜像。当前任务的固定候选与独立复核结果随后登记在 REMEDIATION_PLAN.md；此记录不替代尚未取得的复核。

## 风险与回退

XLSX 包中漏洞仍存在；本次收紧的是两个已知 advisory 的有限写出路径例外，并未消除包内漏洞。到期、新版本、新路径、新 advisory、缺失/错误报告均继续失败。moderate/low 未扩大为本轮整改范围。静态检查不证明外部运行时注入安全，也不替代未来正式发布的完整门禁。

数据库影响：none。必要回退使用普通 revert 撤销任务控制增量，并同步两分支的 Security Scan 命令/消费者；优先保留已清理例外与严格校验器。不能恢复占位/过期配置后声称安全验收仍通过，不强推、不改发布版本或生产数据。
