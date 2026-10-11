// @vitest-environment node
import { mkdtemp, mkdir, rm, writeFile } from 'node:fs/promises'
import { tmpdir } from 'node:os'
import { join } from 'node:path'
import { afterEach, describe, expect, it } from 'vitest'
import { build } from 'vite'
import { thirdPartyLicenses } from './thirdPartyLicenses'

const license = `MIT License

Copyright (c) 2026 Fixture authors

Permission is hereby granted, free of charge, to any person obtaining a copy
of this software and associated documentation files (the "Software"), to deal
in the Software without restriction, including without limitation the rights
to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
copies of the Software, and to permit persons to whom the Software is
furnished to do so, subject to the following conditions:

The above copyright notice and this permission notice shall be included in all
copies or substantial portions of the Software.

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
SOFTWARE.
`
const notice = 'Copyright (c) 2026 Additional contributors\r\nAdditional distribution notice.\r\n'
const roots: string[] = []

afterEach(async () => {
  await Promise.all(roots.splice(0).map((root) => rm(root, { recursive: true, force: true })))
})

async function fixture(): Promise<string> {
  const root = await mkdtemp(join(tmpdir(), 'frontend-license-fixture-'))
  roots.push(root)
  return root
}

async function addPackage(
  root: string,
  name: string,
  version: string,
  declaration: string | undefined = 'MIT',
  texts: Record<string, string> = { LICENSE: license },
): Promise<void> {
  const directory = join(root, 'node_modules', name)
  await mkdir(directory, { recursive: true })
  await writeFile(join(directory, 'package.json'), JSON.stringify({ name, version, license: declaration, type: 'module', main: 'index.js' }))
  await writeFile(join(directory, 'index.js'), `export const value = ${JSON.stringify(name)};`)
  for (const [file, text] of Object.entries(texts)) {
    await mkdir(join(directory, file, '..'), { recursive: true })
    await writeFile(join(directory, file), text)
  }
}

async function compile(root: string, plugin = thirdPartyLicenses(), manualChunks?: (id: string) => string | undefined) {
  const result = await build({
    root,
    configFile: false,
    logLevel: 'silent',
    plugins: [plugin],
    build: { write: false, minify: false, rollupOptions: { input: join(root, 'main.js'), output: { manualChunks } } },
  })
  if ('close' in result || Array.isArray(result)) throw new Error('测试构建应返回单一产物')
  return result.output
}

describe('生产构建第三方许可材料', () => {
  it('保留完整原文、精确版本和复制素材，包含懒加载且排除未分发依赖及本机路径', async () => {
    const root = await fixture()
    await addPackage(root, '@fixture/eager', '1.2.3', 'MIT', { LICENSE: license, 'docs/NOTICE.txt': notice, 'docs/COPYING': license })
    await addPackage(root, 'lazy-dep', '4.5.6', 'MIT', { 'LICENSE.txt': license })
    await addPackage(root, '@fixture/copied', '7.8.9')
    await addPackage(root, 'unused-bad', '0.0.1', undefined, {})
    await writeFile(join(root, 'main.js'), "import { value } from '@fixture/eager'; console.log(value); globalThis.loadFixture = () => import('lazy-dep');")

    const output = await compile(root, thirdPartyLicenses({ copiedPackages: [{ name: '@fixture/copied', usedIn: ['src/CopiedIcon.vue'] }] }))
    const lazyChunk = output.find((item) => item.type === 'chunk' && item.isDynamicEntry)
    expect(lazyChunk).toBeDefined()
    const asset = output.find((item) => item.type === 'asset' && item.fileName === 'THIRD_PARTY_LICENSES.txt')
    expect(asset?.type).toBe('asset')
    if (asset?.type !== 'asset') throw new Error('构建没有发出许可材料')
    const text = String(asset.source)
    expect(text).toContain('包：@fixture/eager@1.2.3\n许可声明：MIT')
    expect(text).toContain('包：lazy-dep@4.5.6')
    expect(text).toContain(`使用分包：${lazyChunk!.name}`)
    expect(text).toContain('包：@fixture/copied@7.8.9')
    expect(text).toContain('复制素材：src/CopiedIcon.vue')
    expect(text).toContain(`--- LICENSE ---\n${license}`)
    expect(text).toContain(`--- docs/NOTICE.txt ---\n${notice}`)
    expect(text).toContain(`--- docs/COPYING ---\n${license}`)
    expect(text).not.toContain('unused-bad')
    expect(text).not.toContain(root)
    expect(text).not.toContain('node_modules')
  })

  it.each([undefined, 'UNKNOWN'])('缺少可核实的许可声明（%s）时阻止构建', async (declaration) => {
    const root = await fixture()
    await addPackage(root, 'unverified-dep', '1.0.0', declaration)
    // undefined 不应触发 fixture 的默认声明。
    await writeFile(join(root, 'node_modules/unverified-dep/package.json'), JSON.stringify({ name: 'unverified-dep', version: '1.0.0', license: declaration, main: 'index.js', type: 'module' }))
    await writeFile(join(root, 'main.js'), "import { value } from 'unverified-dep'; console.log(value);")
    await expect(compile(root)).rejects.toThrow('unverified-dep@1.0.0 缺少可核实的许可声明')
  })

  it('只有 NOTICE 而没有完整许可文本时阻止构建', async () => {
    const root = await fixture()
    await addPackage(root, 'notice-only', '2.0.0', 'MIT', { NOTICE: notice })
    await writeFile(join(root, 'main.js'), "import { value } from 'notice-only'; console.log(value);")
    await expect(compile(root)).rejects.toThrow('notice-only@2.0.0 缺少完整 LICENSE/COPYING 文本')
  })

  it('手动分包留下完全移除的空 JavaScript 模块时不归为分发依赖', async () => {
    const root = await fixture()
    await addPackage(root, 'removed-dep', '0.0.1', 'UNKNOWN', {})
    await writeFile(join(root, 'main.js'), "import { value } from 'removed-dep'; console.log('entry');")
    const output = await compile(root, thirdPartyLicenses(), (id) => id.includes('/removed-dep/') ? 'removed' : undefined)
    const asset = output.find((item) => item.type === 'asset' && item.fileName === 'THIRD_PARTY_LICENSES.txt')
    if (asset?.type !== 'asset') throw new Error('构建没有发出许可材料')
    expect(String(asset.source)).not.toContain('removed-dep')
  })

  it.each(['静态导入', '动态导入'])('仅作为 CSS asset 分发的依赖仍保留许可归属（%s）', async (mode) => {
    const root = await fixture()
    await addPackage(root, 'css-dep', '1.0.0')
    await writeFile(join(root, 'node_modules/css-dep/theme.css'), '.licensed-style { color: red }')
    await writeFile(join(root, 'main.js'), mode === '静态导入'
      ? "import 'css-dep/theme.css'; console.log('entry');"
      : "globalThis.loadStyle = () => import('css-dep/theme.css');")
    const output = await compile(root)
    expect(output.some((item) => item.type === 'asset' && item.fileName.endsWith('.css'))).toBe(true)
    const asset = output.find((item) => item.type === 'asset' && item.fileName === 'THIRD_PARTY_LICENSES.txt')
    if (asset?.type !== 'asset') throw new Error('构建没有发出许可材料')
    expect(String(asset.source)).toContain('包：css-dep@1.0.0')
  })

  it('补充文本保留来源和原文，并且只适用于登记的精确版本', async () => {
    const root = await fixture()
    await addPackage(root, 'supplemented-dep', '3.0.0', 'MIT', {})
    await writeFile(join(root, 'main.js'), "import { value } from 'supplemented-dep'; console.log(value);")
    await writeFile(join(root, 'LICENSE.supplement'), license)
    const plugin = () => thirdPartyLicenses({ supplements: [{
      name: 'supplemented-dep',
      version: '3.0.0',
      files: [{ path: 'LICENSE.supplement', source: 'https://example.org/supplemented-dep/v3.0.0/LICENSE' }],
    }] })
    const output = await compile(root, plugin())
    const asset = output.find((item) => item.type === 'asset' && item.fileName === 'THIRD_PARTY_LICENSES.txt')
    if (asset?.type !== 'asset') throw new Error('构建没有发出许可材料')
    expect(String(asset.source)).toContain(`来源：https://example.org/supplemented-dep/v3.0.0/LICENSE ---\n${license}`)
    await addPackage(root, 'supplemented-dep', '3.0.1', 'MIT', {})
    await expect(compile(root, plugin())).rejects.toThrow('supplemented-dep@3.0.1 缺少完整 LICENSE/COPYING 文本')
  })
})
