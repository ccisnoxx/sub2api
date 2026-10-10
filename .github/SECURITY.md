# Security Policy

[English](#english) | [中文](#中文)

## English

### Reporting a Vulnerability

**Please do not report security vulnerabilities through public GitHub issues, pull requests, or discussions.**

Report vulnerabilities affecting this fork privately to the maintainers of **`ccisnoxx/sub2api`** using GitHub Private Vulnerability Reporting:

**[Report a vulnerability](https://github.com/ccisnoxx/sub2api/security/advisories/new)**

(This repository → **Security** → **Report a vulnerability**.) Sign in to GitHub to use the form. Unpublished reports are accessible to the reporter and people granted access to the advisory, including repository security maintainers and invited collaborators.

This fork has no verified fallback security email. If the form is unavailable after signing in, open an issue only to request restoration of the private reporting channel; do not include vulnerability details, a PoC, logs, or sensitive data in that public issue. Keep the report private until the channel is restored.

A useful report includes:

- The affected component, this fork's release tag or full commit SHA, and image digest if relevant
- Impact: what an attacker can do and which privileges they need
- Minimal reproduction steps or a proof of concept using test data
- Any non-default configuration required to trigger the issue
- A suggested fix, if you have one

Redact passwords, API keys, tokens, cookies, private keys, connection strings, and user request contents from attachments and logs. Do not test against someone else's deployment without authorization.

### Supported Versions

Application code and personal features live on `personal`; `main` contains synchronization, release, and deployment control tools. Support refers to this fork, not to an upstream release with a similar version name.

| Version or branch | Security fix policy |
| ----------------- | ------------------- |
| Latest stable application release published by [this fork](https://github.com/ccisnoxx/sub2api/releases) | Supported; fixes ship in a new fork release based on `personal` |
| Latest `personal` commit | Application fix baseline; also the temporary support baseline if this fork has no stable release yet |
| Latest `main` commit | Supported for synchronization, release, and deployment control tools |
| Older releases or commits | No routine backports; upgrade to the fixed release or commit. Maintainers may explicitly announce an exceptional backport |

If practical, check the latest supported version, but do not delay a report because you cannot upgrade or reproduce it there. Include the version you actually tested. Application reports should identify the `personal` code or release; control-tool reports should identify the `main` commit.

### Scope

In scope are this fork's backend, frontend, published binaries and container images, and repository deployment files, including:

- Personal features such as usage timing, the model catalog, service status, and routing diagnostics
- Authentication, authorization, billing, and exposure of diagnostic or status data
- Synchronization and release tools, GitHub workflows, and deployment scripts
- Inherited upstream code and third-party dependencies when they have a demonstrated impact on this fork

Report fork-specific issues here. If an inherited issue also affects `KlN-4096/sub2api`, `Wei-Shaw/sub2api`, or referenced `LuckyKuang/sub2api-plus` code, still report its impact on this fork here; you are not required to contact upstream instead. This fork's maintainers will assess the affected code and coordinate private upstream reporting with the reporter as needed. Upstream maintainers are not responsible for this fork's response or support.

Provider-only or dependency-only issues with no impact on this fork belong with the relevant provider or dependency maintainer. Scanner findings should explain their impact. Possessing an ordinary user or administrator account does not by itself exclude an authorization bypass, privilege escalation, or unintended data exposure from scope; describe the additional access gained.

### Response Roles and Disclosure

The repository owner, **`ccisnoxx`**, coordinates private intake, triage, and disclosure. Maintainers granted access to the advisory handle reproduction and fixes; the maintainers responsible for release and deployment tools verify and publish the fixed fork version. Upstream maintainers are consulted only where inherited code requires coordination.

1. We acknowledge the report in the private advisory and assess affected versions, impact, and severity.
2. We develop and verify a fix privately, coordinate inherited issues when needed, and may ask the reporter to verify the fix.
3. We publish the fixed release or control-tool commit and any upgrade or mitigation instructions before publishing the advisory. We credit the reporter unless they prefer anonymity, and consider a CVE when applicable.

Please coordinate disclosure through the private advisory and keep details private until publication. This is a personally maintained fork; response and release times are best effort, with priority given to demonstrated impact and severity.

## 中文

### 报告漏洞

**请不要通过公开的 Issue、Pull Request 或 Discussion 报告安全漏洞。**

影响本 fork 的漏洞，请通过 GitHub Private Vulnerability Reporting 私密提交给 **`ccisnoxx/sub2api`** 的维护者：

**[提交漏洞报告](https://github.com/ccisnoxx/sub2api/security/advisories/new)**

（本仓库 → **Security** → **Report a vulnerability**。）使用表单需要登录 GitHub。未公开的报告对报告者以及获准访问该安全公告的人员可见，包括仓库安全维护者和受邀协作者。

本 fork 没有经过验证的备用安全邮箱。登录后仍无法使用表单时，可以创建 Issue 仅请求恢复私密报告渠道；该公开 Issue 不得包含漏洞细节、PoC、日志或敏感数据。在渠道恢复前请保留报告的私密性。

报告中建议包含：

- 受影响的组件、本 fork 的 Release tag 或完整 commit SHA，以及相关镜像 digest
- 影响：攻击者能做什么、需要什么权限
- 使用测试数据的最小复现步骤或 PoC
- 触发问题所需的非默认配置
- 修复建议（如有）

请对附件和日志中的密码、API Key、Token、Cookie、私钥、连接串和用户请求内容脱敏。未经授权，不得在他人的部署上验证漏洞。

### 支持的版本

应用代码和个人功能位于 `personal`；`main` 包含同步、发布和部署控制工具。以下支持策略仅适用于本 fork，不代表支持版本名称相似的上游发行版。

| 版本或分支 | 安全修复策略 |
| ---------- | ------------ |
| [本 fork 发布](https://github.com/ccisnoxx/sub2api/releases)的最新稳定应用版本 | 支持；修复随基于 `personal` 的新版本发布 |
| `personal` 最新提交 | 应用修复基线；本 fork 尚无稳定 Release 时，也是临时支持基线 |
| `main` 最新提交 | 支持同步、发布和部署控制工具的安全修复 |
| 更早的版本或提交 | 不常规回移；应升级到修复版本或提交。特殊回移由维护者明确公告 |

条件允许时，请检查最新受支持版本，但无法升级或在新版本复现不应阻止报告。请提供实际测试版本：应用问题对应 `personal` 代码或发行版，控制工具问题对应 `main` 提交。

### 范围

安全范围覆盖本 fork 的后端、前端、发布的二进制和容器镜像，以及仓库部署文件，具体包括：

- 用量计时、模型目录、服务状态、路由诊断等个人功能
- 身份认证、授权、计费，以及诊断或状态数据泄露
- 同步与发布工具、GitHub 工作流、部署脚本
- 对本 fork 有明确影响的上游继承代码和第三方依赖漏洞

本 fork 特有问题应向本仓库报告。继承问题即使也影响 `KlN-4096/sub2api`、`Wei-Shaw/sub2api` 或参考的 `LuckyKuang/sub2api-plus` 代码，也请在此报告其对本 fork 的影响，无须改向上游报告。本 fork 维护者负责评估受影响代码，并按需与报告者协调向上游私密报告。上游维护者不承担本 fork 的响应或支持责任。

仅影响服务商或依赖自身、对本 fork 无影响的问题，应向相应服务商或依赖维护者报告。扫描结果应说明实际影响。拥有普通用户或管理员账号本身不会排除越权、权限提升或非预期数据泄露问题；请说明攻击者额外获得的权限或数据。

### 响应角色与披露流程

仓库所有者 **`ccisnoxx`** 负责协调私密报告接收、分级和披露。获准访问安全公告的维护者负责复现和修复；负责发布与部署工具的维护者验证并发布本 fork 的修复版本。仅在继承代码需要协调时联系上游维护者。

1. 在私密安全公告中确认收到报告，评估受影响版本、实际影响和严重程度。
2. 私下开发并验证修复，按需协调上游继承问题，并可能请报告者协助验证。
3. 先发布修复版本或控制工具提交以及升级、缓解措施，再公开安全公告。除非报告者希望匿名，否则予以致谢；适用时考虑申请 CVE。

请通过私密安全公告协调披露，在公告发布前保密。本 fork 由个人维护，响应和发版时间尽力而为，按实际影响和严重程度优先处理。
