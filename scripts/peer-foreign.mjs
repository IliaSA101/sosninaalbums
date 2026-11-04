// Шаг CI «чужая учётная запись» (148, design §5): `node scripts/peer-foreign.mjs '<шаблон -run>' <Тест>… [--pkg=<пакет>]… [--allow-skip]`.
// Гоняет `go test -count=1 -v -run <шаблон> <пакет, по умолчанию ./internal/peercred>` и разбирает вывод: пропуск (`--- SKIP`) — провал
// (иначе без sudo/пользователя тест тихо «проходит»), каждый названный тест обязан дать `--- PASS: <Тест>`
// (при «no tests to run» go test выходит с 0). Все причины провала печатаются разом, а не по одной на круг.
import { spawn } from 'node:child_process'
import path from 'node:path'
import { fileURLToPath } from 'node:url'

// analyze: вывод go test, код выхода, обязательные тесты -> список причин провала (пустой — всё хорошо).
export function analyze(output, code, need, allowSkip = false) {
  const why = []
  if (code !== 0) why.push(`go test вышел с кодом ${code}`)
  for (const m of output.matchAll(/^\s*--- FAIL: (\S+)/gm)) why.push(`провал теста ${m[1]}`)
  if (!allowSkip) {
    for (const m of output.matchAll(/^\s*--- SKIP: (\S+)/gm)) why.push(`пропущен ${m[1]}: в CI пропуск — провал (нет sudo, пользователя или переменной?)`)
  }
  for (const name of need) {
    const pass = new RegExp(`^--- PASS: ${name.replace(/[.*+?^${}()|[\]\\]/g, '\\$&')} `, 'm')
    if (!pass.test(output)) why.push(`нет «--- PASS: ${name}»`)
  }
  return why
}

// parseArgs: аргументы командной строки -> шаблон, обязательные тесты, пакеты, allowSkip.
export function parseArgs(args) {
  const allowSkip = args.includes('--allow-skip')
  const pkgs = args.filter((a) => a.startsWith('--pkg=')).map((a) => a.slice('--pkg='.length))
  const [pattern, ...need] = args.filter((a) => a !== '--allow-skip' && !a.startsWith('--pkg='))
  return { pattern, need, pkgs: pkgs.length > 0 ? pkgs : ['./internal/peercred'], allowSkip }
}

function main() {
  const { pattern, need, pkgs, allowSkip } = parseArgs(process.argv.slice(2))
  if (!pattern || need.length === 0 || pkgs.some((p) => p === '')) {
    console.error("usage: node scripts/peer-foreign.mjs '<шаблон -run>' <Тест>… [--pkg=<пакет>]… [--allow-skip]")
    process.exit(2)
  }
  const root = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..')
  const child = spawn('go', ['test', '-count=1', '-v', '-run', pattern, ...pkgs], { cwd: root })
  let output = ''
  for (const [stream, sink] of [
    [child.stdout, process.stdout],
    [child.stderr, process.stderr],
  ]) {
    stream.on('data', (d) => {
      output += d
      sink.write(d)
    })
  }
  child.on('error', (e) => {
    console.error(`не запустился go: ${e.message}`)
    process.exit(1)
  })
  child.on('close', (code) => {
    const why = analyze(output, code ?? 1, need, allowSkip)
    if (allowSkip) for (const m of output.matchAll(/^\s*--- SKIP: (\S+)/gm)) console.log(`peer-foreign: пропущен ${m[1]} (разрешено --allow-skip; причина выше в журнале)`)
    if (why.length === 0) {
      console.log(`peer-foreign: OK (${need.join(', ')})`)
      return
    }
    for (const w of why) console.error(`peer-foreign: ${w}`)
    process.exit(1)
  })
}

if (process.argv[1] && path.resolve(process.argv[1]) === fileURLToPath(import.meta.url)) main()
