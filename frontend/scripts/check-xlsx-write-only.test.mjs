import assert from 'node:assert/strict'
import { test } from 'node:test'
import { createChecker } from './check-xlsx-write-only.mjs'

const checker = createChecker()
async function check(code, filePath = 'src/views/admin/UsageView.vue') {
  const [result] = await checker.lintText(`<script setup lang="ts">\n${code}\n</script>`, { filePath })
  return result.messages
}
const declaration = "const sheet = await import('xlsx');"

test('允许现有写出 API 和 Vue/TypeScript 代码', async () => {
  assert.deepEqual(await check(`async function exportUsage() {
    ${declaration}
    const ws = sheet.utils.aoa_to_sheet([['数据']]);
    sheet.utils.sheet_add_aoa(ws, [['a']], { origin: -1 });
    const wb = sheet.utils.book_new();
    sheet.utils.book_append_sheet(wb, ws, 'Usage');
    sheet.write(wb, { bookType: 'xlsx', type: 'array' });
  }`), [])
})

for (const operation of [
  'sheet.read(upload)', 'sheet.readFile(path)', 'sheet["read"](upload)',
  'const { read } = sheet; read(upload)', 'const alias = sheet; alias.read(upload)',
  'const read = sheet.read; read(upload)', 'parseUpload(sheet, upload)',
  'sheet.utils.sheet_to_json(ws)', 'sheet.utils[method](upload)',
  'sheet.utils = other', 'return sheet',
]) {
  test(`拒绝解析或引用逃逸：${operation}`, async () => {
    const messages = await check(`async function exportUsage() { ${declaration} ${operation} }`)
    assert.ok(messages.some(message => message.ruleId === 'xlsx-policy/write-only'))
  })
}

for (const code of [
  "import { read as parse } from 'xlsx'", "import XLSX from 'xlsx'",
  "export { read } from 'xlsx'", "export * from 'xlsx'",
  "const { read } = require('xlsx')", "const sheet = await import('xlsx/xlsx.mjs')",
  "const { read } = await import('xlsx')", "import('xlsx').then(parseUpload)",
  "const moduleName = 'xlsx'; const sheet = await import(moduleName)",
  "const moduleName = 'xlsx'; const sheet = require(moduleName)",
  "XLSX.read(upload)", "XLSX['readFile'](upload)",
]) {
  test(`拒绝其他导入方式：${code}`, async () => {
    assert.ok((await check(code)).some(message => message.ruleId === 'xlsx-policy/write-only'))
  })
}

test('拒绝其他 Vue 文件中的 xlsx，即使只调用写出 API', async () => {
  assert.ok((await check(`${declaration} sheet.write(wb)`, 'src/views/user/Upload.vue')).length > 0)
})

test('解析错误导致失败', async () => {
  assert.ok((await check('const =')).some(message => message.fatal))
})

test('源码内 eslint-disable 不能关闭安全规则', async () => {
  assert.ok((await check(`/* eslint-disable xlsx-policy/write-only */\n${declaration} sheet.read(upload)`))
    .some(message => message.ruleId === 'xlsx-policy/write-only'))
})
