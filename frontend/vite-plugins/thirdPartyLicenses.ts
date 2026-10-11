import { readFile, readdir, realpath } from 'node:fs/promises'
import { basename, join, relative, resolve } from 'node:path'
import type { Plugin } from 'vite'

interface CopiedPackage {
  name: string
  usedIn: string[]
}

interface LicenseSupplement {
  name: string
  version: string
  files: { path: string; source: string }[]
}

interface Options {
  copiedPackages?: CopiedPackage[]
  supplements?: LicenseSupplement[]
}

interface PackageMetadata {
  name: string
  version: string
  license?: string | { type: string; url?: string }
  licenses?: { type: string; url?: string }[]
}

interface LicenseText {
  name: string
  text: string
  source?: string
}

const licenseName = /^(?:licen[cs]es?|copying|notice)(?:$|[._-])/i
const permissionName = /^(?:licen[cs]es?|copying)(?:$|[._-])/i

// 使用最后一个 node_modules 边界，兼容 pnpm、npm、作用域包及 CommonJS 虚拟模块。
function packageRoot(moduleId: string): string | undefined {
  const file = moduleId.replace(/^\0+/, '').split('?')[0].replace(/\\/g, '/')
  const boundary = file.lastIndexOf('/node_modules/')
  if (boundary < 0) return undefined
  const parts = file.slice(boundary + '/node_modules/'.length).split('/')
  const packageName = parts[0].startsWith('@') ? parts.slice(0, 2).join('/') : parts[0]
  return file.slice(0, boundary) + '/node_modules/' + packageName
}

async function packageLicenseFiles(root: string): Promise<LicenseText[]> {
  const files: LicenseText[] = []
  async function visit(directory: string, insideLicenseDirectory = false): Promise<void> {
    const entries = await readdir(directory, { withFileTypes: true })
    entries.sort((left, right) => left.name.localeCompare(right.name, 'en'))
    for (const entry of entries) {
      // 依赖自己的 node_modules 由实际产物模块单独归属，不能算作当前包的文本。
      if (entry.name === 'node_modules' || entry.name === '.git') continue
      const file = join(directory, entry.name)
      if (entry.isDirectory()) {
        await visit(file, insideLicenseDirectory || licenseName.test(entry.name))
      } else if (entry.isFile() && (insideLicenseDirectory || licenseName.test(entry.name))) {
        files.push({ name: relative(root, file).replace(/\\/g, '/'), text: await readFile(file, 'utf8') })
      }
    }
  }
  await visit(root)
  return files
}

function licenseDeclaration(metadata: PackageMetadata): string {
  const declarations = metadata.license ? [metadata.license] : metadata.licenses ?? []
  if (!declarations.length || declarations.some((license) => {
    const value = typeof license === 'string' ? license : license.type
    return !value?.trim() || /^(?:unknown|unlicensed|undefined|null|noassertion)$/i.test(value.trim())
  })) {
    throw new Error(`${metadata.name}@${metadata.version} 缺少可核实的许可声明`)
  }
  // 保留包声明的表达式及旧式声明对象，不将多许可简化为单一许可。
  return typeof metadata.license === 'string'
    ? metadata.license
    : JSON.stringify(metadata.license ?? metadata.licenses)
}

/** 只收集实际输出 chunk 中的依赖，以及显式登记的复制素材。缺口必须阻止发布。 */
export function thirdPartyLicenses(options: Options = {}): Plugin {
  let root: string
  return {
    name: 'third-party-licenses',
    apply: 'build',
    enforce: 'post',
    configResolved(config) {
      root = config.root
    },
    generateBundle: {
      // Vite 随后会删除纯 CSS 的 JS 包装 chunk；先保留其实际输出 CSS 的模块归属。
      order: 'pre',
      async handler(_outputOptions, bundle) {
        const roots = new Map<string, Set<string>>()
        async function addPackage(path: string, use: string): Promise<void> {
          const canonical = await realpath(path)
          const uses = roots.get(canonical) ?? new Set<string>()
          uses.add(use)
          roots.set(canonical, uses)
        }
        for (const output of Object.values(bundle)) {
          if (output.type !== 'chunk') continue
          for (const [moduleId, module] of Object.entries(output.modules)) {
            // 手动分包仍可能留下已被摇树优化移除的模块；CSS 原文则发往单独的 asset。
            if (module.renderedLength === 0 && !/\.(?:css|scss|sass|less|styl|stylus)(?:\?|$)/i.test(moduleId)) continue
            const dependencyRoot = packageRoot(moduleId)
            if (dependencyRoot) await addPackage(dependencyRoot, `使用分包：${output.name}`)
          }
        }
        for (const copied of options.copiedPackages ?? []) {
          await addPackage(resolve(root, 'node_modules', copied.name), `复制素材：${copied.usedIn.join('、')}`)
        }

        const sections: { identity: string; text: string }[] = []
        const issues: string[] = []
        for (const [dependencyRoot, uses] of roots) {
          const metadata: PackageMetadata = JSON.parse(await readFile(join(dependencyRoot, 'package.json'), 'utf8'))
          if (!metadata.name || !metadata.version) {
            this.error('产物中的第三方包缺少名称或精确版本，无法生成许可材料')
          }
          const identity = `${metadata.name}@${metadata.version}`
          let declaration: string
          try {
            declaration = licenseDeclaration(metadata)
          } catch (error) {
            issues.push((error as Error).message)
            continue
          }
          const texts = await packageLicenseFiles(dependencyRoot)
          const supplement = options.supplements?.find((item) => item.name === metadata.name && item.version === metadata.version)
          for (const file of supplement?.files ?? []) {
            if (!/^https:\/\//.test(file.source)) this.error(`${identity} 的补充许可缺少固定来源 URL`)
            texts.push({ name: basename(file.path), text: await readFile(resolve(root, file.path), 'utf8'), source: file.source })
          }
          const declaredFile = /^SEE LICEN[CS]E IN (.+)$/i.exec(declaration)?.[1]
          if (declaredFile && !texts.some((file) => file.name === declaredFile)) {
            texts.push({ name: declaredFile, text: await readFile(join(dependencyRoot, declaredFile), 'utf8') })
          }
          if (!texts.some((file) => (file.name.split('/').some((part) => permissionName.test(part)) || file.name === declaredFile) && file.text.trim())) {
            issues.push(`${identity} 缺少完整 LICENSE/COPYING 文本；请从该精确版本的官方来源补齐，并登记来源`)
            continue
          }
          if (texts.some((file) => !file.text.trim())) {
            issues.push(`${identity} 包含空白许可或 NOTICE 文本`)
            continue
          }
          sections.push({
            identity,
            text: [
              '='.repeat(80),
              `包：${identity}`,
              `许可声明：${declaration}`,
              ...[...uses].sort(),
              ...texts.map((file) => `\n--- ${file.name}${file.source ? `\n来源：${file.source}` : ''} ---\n${file.text}`),
            ].join('\n'),
          })
        }
        if (issues.length) this.error(`第三方许可材料不完整：\n${issues.sort().join('\n')}`)
        sections.sort((left, right) => left.identity.localeCompare(right.identity, 'en'))
        this.emitFile({
          type: 'asset',
          fileName: 'THIRD_PARTY_LICENSES.txt',
          source: [
            '前端第三方许可材料',
            '范围：本次构建所有输出 chunk 的模块（包括懒加载）及登记的复制素材。',
            '以下保留各包的实际许可声明及完整随包许可、版权和 NOTICE 原文。',
            '',
            ...sections.map((section) => section.text),
            '',
          ].join('\n'),
        })
      },
    },
  }
}
