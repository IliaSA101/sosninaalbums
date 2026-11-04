// Контракт биндингов Wails: методы App и поля типов из models.ts в стабильном тексте.
// Имя пространства main/app заменяется на APP, чтобы перенос App в internal/app не давал разницы.
// Без флага — печать; --write — записать testdata/bindings-contract.txt; --check — сравнить (exit 1 при разнице).
import fs from 'node:fs'
import path from 'node:path'
import { fileURLToPath } from 'node:url'

const root = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..')
const goDir = path.join(root, 'frontend', 'wailsjs', 'go')
const target = path.join(root, 'testdata', 'bindings-contract.txt')

const norm = (s) => s.replace(/\b(main|app)\./g, 'APP.')
const nsName = (n) => (n === 'main' || n === 'app' ? 'APP' : n)

function methods() {
  const out = []
  for (const d of fs.readdirSync(goDir, { withFileTypes: true })) {
    const f = path.join(goDir, d.name, 'App.d.ts')
    if (!d.isDirectory() || !fs.existsSync(f)) continue
    for (const line of fs.readFileSync(f, 'utf8').split(/\r?\n/)) {
      const m = line.match(/^export function (\w+)(\(.*\):.*);$/)
      if (m) out.push(`method ${m[1]}${norm(m[2])}`)
    }
  }
  return out
}

// Блоки типов: заголовок + поля (порядок полей сохраняем).
function types() {
  const blocks = []
  let ns = ''
  let cur = null
  for (const line of fs.readFileSync(path.join(goDir, 'models.ts'), 'utf8').split(/\r?\n/)) {
    let m
    if ((m = line.match(/^export namespace (\w+) \{/))) ns = nsName(m[1])
    else if ((m = line.match(/^\texport (?:class|interface) (\w+)/))) {
      cur = [`type ${ns}.${m[1]}`]
      blocks.push(cur)
    } else if (cur && (m = line.match(/^\t {4}(\w+\??): (.+);$/))) cur.push(`  ${m[1]}: ${norm(m[2])}`)
    else if (/^\t}/.test(line)) cur = null
  }
  return blocks
}

function build() {
  const blocks = types().sort((a, b) => (a[0] < b[0] ? -1 : a[0] > b[0] ? 1 : 0))
  return [...methods().sort(), ...blocks.flat()].join('\n') + '\n'
}

const text = build()
const mode = process.argv[2]
if (mode === '--write') {
  fs.mkdirSync(path.dirname(target), { recursive: true })
  fs.writeFileSync(target, text)
  console.log(`записано: ${path.relative(root, target)}`)
} else if (mode === '--check') {
  const want = fs.existsSync(target) ? fs.readFileSync(target, 'utf8').replace(/\r\n/g, '\n') : ''
  if (want === text) {
    console.log('контракт биндингов: совпадает')
  } else {
    const a = new Set(want.split('\n'))
    const b = new Set(text.split('\n'))
    for (const l of a) if (!b.has(l)) console.log(`- ${l}`)
    for (const l of b) if (!a.has(l)) console.log(`+ ${l}`)
    console.log('контракт биндингов изменился: node scripts/bindings-contract.mjs --write и закоммитить')
    process.exit(1)
  }
} else {
  process.stdout.write(text)
}
