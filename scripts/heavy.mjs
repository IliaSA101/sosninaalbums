// Очередь тяжёлых проверок: `node scripts/heavy.mjs <команда> [аргументы...]`.
// go test ./..., golangci-lint, покрытие дерева и -race каждый сами занимают все ядра.
// Когда их одновременно запускают 4–6 агентов и мутанты, каждый идёт в 5–10 раз дольше,
// а тесты с порогом времени падают (замер 2026-10-10: golangci 8 мин вместо < 1, покрытие 13 мин).
// Поэтому одновременно — не больше SA_HEAVY_SLOTS (по умолчанию 3), и новое место (кроме первого)
// даётся, только если загрузка CPU ниже SA_HEAVY_CPU % (по умолчанию 70): цель — машина занята на 60–75 %,
// а не простаивает и не стоит на 99 % (владелец 2026-10-10). Первая проверка идёт всегда — очередь не встаёт.
// Места — файлы .gotmp/heavy/slot-N с pid владельца; место умершего процесса освобождается само.
import { spawnSync } from 'node:child_process'
import fs from 'node:fs'
import os from 'node:os'
import path from 'node:path'
import { fileURLToPath } from 'node:url'
import { mainRoot, withGoCaches } from './goenv.mjs'

// Места общие для основной копии и всех worktree агентов (у проверок разные GOTMPDIR):
// .gotmp основной копии — рядом с общим .git.
const here = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..')
const main = mainRoot(here)
const dir = path.join(main, '.gotmp', 'heavy')
const slots = Math.max(1, Number(process.env.SA_HEAVY_SLOTS) || 3)
const cpuMax = Number(process.env.SA_HEAVY_CPU) || 70
const [cmd, ...args] = process.argv.slice(2)
if (!cmd) {
  console.error('usage: node scripts/heavy.mjs <command> [args...]')
  process.exit(2)
}

const alive = (pid) => {
  try {
    process.kill(pid, 0)
    return true
  } catch (e) {
    return e.code === 'EPERM'
  }
}
const sleep = (ms) => Atomics.wait(new Int32Array(new SharedArrayBuffer(4)), 0, 0, ms)

// Загрузка CPU, % за ms: os.loadavg на Windows всегда 0, поэтому — по разнице счётчиков os.cpus().
const busy = () => os.cpus().reduce((a, c) => [a[0] + c.times.idle, a[1] + Object.values(c.times).reduce((s, v) => s + v, 0)], [0, 0])
function cpuLoad(ms) {
  const [i0, t0] = busy()
  sleep(ms)
  const [i1, t1] = busy()
  return t1 > t0 ? Math.round(100 * (1 - (i1 - i0) / (t1 - t0))) : 0
}
// Места умерших процессов (убиты по таймауту, finally не сработал) убираем до подсчёта:
// иначе мёртвое место считается занятым, а при CPU ≥ порога очередь ждёт вечно (ревью 0.8.0).
const taken = () => {
  let files
  try {
    files = fs.readdirSync(dir).filter((f) => f.startsWith('slot-'))
  } catch {
    return 0
  }
  return files.filter((f) => {
    const p = path.join(dir, f)
    try {
      const owner = Number(fs.readFileSync(p, 'utf8').trim() || 0)
      if (owner && alive(owner)) return true
      fs.rmSync(p, { force: true })
    } catch {
      // место освободили между readdir и чтением
    }
    return false
  }).length
}

function acquire() {
  fs.mkdirSync(dir, { recursive: true })
  let told = false
  const start = Date.now()
  for (;;) {
    // не первое место — только при загрузке ниже порога (мерить 3 с: короткий всплеск не в счёт)
    const load = taken() > 0 ? cpuLoad(3000) : 0
    for (let i = 0; i < slots && (load < cpuMax || taken() === 0); i++) {
      const f = path.join(dir, `slot-${i}`)
      try {
        fs.writeFileSync(f, String(process.pid), { flag: 'wx' })
        if (told) console.error(`heavy: место ${i} получено через ${Math.round((Date.now() - start) / 1000)} с`)
        return f
      } catch {
        // место могли освободить между отказом wx и чтением — тогда просто следующий круг
        try {
          const owner = Number(fs.readFileSync(f, 'utf8').trim() || 0)
          if (owner && !alive(owner)) fs.rmSync(f, { force: true })
        } catch {
          continue
        }
      }
    }
    if (!told) console.error(`heavy: занято ${taken()} из ${slots}, CPU ${load} % (порог ${cpuMax}) — жду очереди…`)
    told = true
    sleep(2000)
  }
}

const slot = acquire()
const free = () => fs.rmSync(slot, { force: true })
process.on('SIGINT', () => (free(), process.exit(130)))
process.on('SIGTERM', () => (free(), process.exit(143)))
let r
try {
  r = spawnSync(cmd, args, { stdio: 'inherit', cwd: process.cwd(), env: withGoCaches(process.env, here) })
} finally {
  free()
}
if (r.error) {
  console.error(`heavy: ${r.error.message}`)
  process.exit(1)
}
process.exit(r.status ?? 1)
