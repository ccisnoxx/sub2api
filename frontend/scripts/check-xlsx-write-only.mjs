import path from 'node:path'
import { fileURLToPath } from 'node:url'
import eslint from 'eslint'

const { ESLint } = eslint
const frontendRoot = fileURLToPath(new URL('../', import.meta.url))
const writerPath = path.join(frontendRoot, 'src/views/admin/UsageView.vue')
const allowedCalls = new Set([
  'utils.aoa_to_sheet', 'utils.sheet_add_aoa', 'utils.book_new',
  'utils.book_append_sheet', 'write',
])

// 按作用域检查整个模块的引用，禁止解析 API 或把模块转交给其他代码。
export const writeOnlyRule = {
  meta: { type: 'problem', schema: [], messages: {
    forbidden: 'xlsx 仅允许管理员用量导出直接调用已核实的写出 API；禁止读入或传递模块引用。',
    dynamic: '安全检查要求 import/require 使用静态模块名，才能验证 xlsx 的写出边界。',
  } },
  create(context) {
    const bindings = []
    const report = node => context.report({ node, messageId: 'forbidden' })
    const isXlsx = source => typeof source?.value === 'string' && /(^xlsx(?:\/|$)|sheetjs|xlsx\.(?:mjs|js))/.test(source.value)
    const rejectImport = node => { if (isXlsx(node.source)) report(node) }
    const scriptVisitor = {
      ImportDeclaration: rejectImport,
      ExportNamedDeclaration: rejectImport,
      ExportAllDeclaration: rejectImport,
      TSImportEqualsDeclaration(node) {
        if (isXlsx(node.moduleReference.expression)) report(node)
      },
      CallExpression(node) {
        if (node.callee.name !== 'require') return
        if (typeof node.arguments[0]?.value !== 'string') context.report({ node, messageId: 'dynamic' })
        else if (isXlsx(node.arguments[0])) report(node)
      },
      MemberExpression(node) {
        if (node.object.type === 'Identifier' && node.object.name === 'XLSX'
          && ['read', 'readFile', 'readFileSync'].includes(node.computed ? node.property.value : node.property.name)) report(node)
      },
      ImportExpression(node) {
        if (typeof node.source.value !== 'string') {
          context.report({ node, messageId: 'dynamic' })
          return
        }
        if (!isXlsx(node.source)) return
        const awaited = node.parent
        const declaration = awaited.parent
        if (path.resolve(context.getFilename()) !== writerPath || node.source.value !== 'xlsx'
          || awaited.type !== 'AwaitExpression' || declaration.type !== 'VariableDeclarator'
          || declaration.init !== awaited || declaration.id.type !== 'Identifier'
          || declaration.parent.kind !== 'const') {
          report(node)
          return
        }
        bindings.push(declaration)
      },
      'Program:exit'() {
        for (const declaration of bindings) {
          const [variable] = context.sourceCode.getDeclaredVariables(declaration)
          for (const reference of variable.references) {
            if (reference.identifier === declaration.id) continue
            let node = reference.identifier
            const properties = []
            while (node.parent.type === 'MemberExpression' && node.parent.object === node
              && !node.parent.computed && node.parent.property.type === 'Identifier') {
              node = node.parent
              properties.push(node.property.name)
            }
            if (!allowedCalls.has(properties.join('.')) || node.parent.type !== 'CallExpression'
              || node.parent.callee !== node) report(reference.identifier)
          }
        }
      },
    }
    return context.sourceCode.parserServices.defineTemplateBodyVisitor({
      // 模板引用不属于脚本作用域 references；禁止模块通过事件或 props 逃逸。
      VExpressionContainer(node) {
        const names = new Set(bindings.map(binding => binding.id.name))
        for (const reference of node.references) {
          if (!reference.variable && names.has(reference.id.name)) report(reference.id)
        }
      },
      ImportExpression(node) {
        if (typeof node.source.value !== 'string') context.report({ node, messageId: 'dynamic' })
        else if (isXlsx(node.source)) report(node)
      },
      CallExpression: scriptVisitor.CallExpression,
      MemberExpression: scriptVisitor.MemberExpression,
    }, scriptVisitor)
  },
}

export function createChecker() {
  return new ESLint({
    cwd: frontendRoot,
    allowInlineConfig: false,
    useEslintrc: false,
    ignore: false,
    plugins: { 'xlsx-policy': { rules: { 'write-only': writeOnlyRule } } },
    baseConfig: {
      parser: 'vue-eslint-parser',
      parserOptions: { parser: '@typescript-eslint/parser', ecmaVersion: 'latest', sourceType: 'module' },
      plugins: ['xlsx-policy'],
      rules: { 'xlsx-policy/write-only': 'error' },
    },
  })
}

if (process.argv[1] && path.resolve(process.argv[1]) === fileURLToPath(import.meta.url)) {
  const checker = createChecker()
  const results = await checker.lintFiles(['src/**/*.{vue,js,jsx,cjs,mjs,ts,tsx,cts,mts}'])
  const formatter = await checker.loadFormatter('stylish')
  const output = formatter.format(results)
  if (output) console.error(output)
  if (results.some(result => result.errorCount > 0)) process.exitCode = 1
  else console.log('XLSX 写出边界检查通过。')
}
