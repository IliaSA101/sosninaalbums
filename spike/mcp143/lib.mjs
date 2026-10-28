// Общее для сценариев спайка 143/Т0: запись результатов в results.tsv, снимок состояния при провале,
// клиент JSON-RPC по строкам, запуск и остановка слушателя. Как CLI: `node lib.mjs record <id> <статус> [текст]`
// и `node lib.mjs ingest <префикс> <файл>` (строки RESULT из вывода spikec) — для сценария PowerShell.
import { spawn, spawnSync, execFileSync } from 'node:child_process'
import fs from 'node:fs'
import os from 'node:os'
import path from 'node:path'
import { pathToFileURL } from 'node:url'

export const OUT = path.resolve(process.env.SPIKE_OUT || path.join(os.tmpdir(), 'spike143-out'))
fs.mkdirSync(path.join(OUT, 'snapshots'), { recursive: true })
const RESULTS = path.join(OUT, 'results.tsv')
const isWin = process.platform === 'win32'
export { isWin }

export const sleep = (ms) => new Promise((r) => setTimeout(r, ms))

// record добавляет строку результата: id, статус OK | FAIL | INFO | SKIP, подробности.
let fails = 0
const clients = []
export function record(id, status, detail = '') {
  if (status === 'FAIL') fails++
  const d = String(detail).replace(/[\r\n\t]+/g, ' ').slice(0, 600)
  fs.appendFileSync(RESULTS, `${id}\t${status}\t${d}\n`, 'utf8')
  console.log(`${status.padEnd(4)} ${id}  ${d}`)
}

// ingest разбирает вывод spikec (RESULT<TAB>имя<TAB>статус<TAB>текст) в записи с префиксом.
export function ingest(prefix, text) {
  for (const line of String(text).split(/\r?\n/)) {
    const m = /^RESULT\t([^\t]*)\t([^\t]*)\t?(.*)$/.exec(line)
    if (!m) continue
    const st = { OK: 'OK', FAIL: 'FAIL', INFO: 'INFO' }[m[2]] ?? 'INFO'
    record(prefix ? `${prefix}/${m[1]}` : m[1], st, m[2] === st ? m[3] : `${m[2]}: ${m[3]}`)
  }
}

function safeList(dir, depth = 0) {
  const lines = []
  try {
    for (const e of fs.readdirSync(dir, { withFileTypes: true })) {
      const p = path.join(dir, e.name)
      let size = ''
      try {
        size = e.isFile() ? ` ${fs.statSync(p).size} B` : ''
      } catch {
        /* исчез между чтением папки и stat */
      }
      lines.push(`${'  '.repeat(depth)}${e.name}${e.isDirectory() ? '/' : size}`)
      if (e.isDirectory() && depth < 3) lines.push(...safeList(p, depth + 1))
    }
  } catch (e) {
    lines.push(`(не прочиталась ${dir}: ${e.message})`)
  }
  return lines
}

// snapshot — состояние на момент провала: папка out, процессы spike*, хвост spike.log.
export function snapshot(label) {
  const parts = [`# ${label} ${new Date().toISOString()}`, `OS ${process.platform} ${os.release()} user=${os.userInfo().username}`, '', '## out', ...safeList(OUT)]
  try {
    const ps = isWin
      ? execFileSync('tasklist', ['/FO', 'CSV', '/NH'], { encoding: 'utf8', timeout: 15000 })
      : execFileSync('ps', ['-eo', 'pid,etime,args'], { encoding: 'utf8', timeout: 15000 })
    parts.push('', '## процессы spike', ...ps.split(/\r?\n/).filter((l) => /spike/i.test(l)))
  } catch (e) {
    parts.push('', `(процессы: ${e.message})`)
  }
  try {
    const log = fs.readFileSync(path.join(OUT, 'spike.log'), 'utf8').split(/\r?\n/)
    parts.push('', '## spike.log (хвост 80)', ...log.slice(-80))
  } catch {
    parts.push('', '(spike.log нет)')
  }
  const file = path.join(OUT, 'snapshots', `${label.replace(/[^\w.-]+/g, '_')}.txt`)
  fs.writeFileSync(file, parts.join('\n'), 'utf8')
}

// check выполняет независимую проверку: возврат — подробности успеха, исключение — FAIL со снимком.
// strict=false: провал пишется как INFO (исследовательская проверка, критерий не ломает).
export async function check(id, fn, { strict = true } = {}) {
  try {
    const d = await fn()
    const na = typeof d === 'string' && d.startsWith('н/п')
    record(id, na ? 'SKIP' : 'OK', d === undefined || d === true ? '' : d)
    return true
  } catch (e) {
    record(id, strict ? 'FAIL' : 'INFO', e && e.message ? e.message : String(e))
    if (strict) snapshot(id)
    return false
  }
}

export function assert(cond, msg) {
  if (!cond) throw new Error(msg)
}

// step оборачивает весь сценарий: падение самого сценария — FAIL step/crash, а не молчание.
export async function step(name, fn) {
  try {
    await fn()
  } catch (e) {
    record(`${name}/crash`, 'FAIL', e && e.stack ? e.stack.split('\n').slice(0, 4).join(' | ') : String(e))
    snapshot(`${name}-crash`)
  }
}

// RawClient — клиент JSON-RPC по строкам поверх дочернего процесса (посредник `--mcp`).
export class RawClient {
  constructor(exe, args = ['--mcp'], env = {}) {
    this.t0 = performance.now()
    this.lines = []
    this.badLines = []
    this.stderr = ''
    this.waiters = new Map()
    this.notifs = []
    this.exited = null
    this.nextId = 1
    this.proc = spawn(exe, args, { stdio: 'pipe', env: { ...process.env, ...env } })
    clients.push(this.proc)
    this.exitP = new Promise((res) => this.proc.on('exit', (code, sig) => {
      this.exited = { code, sig, at: performance.now() }
      res(this.exited)
    }))
    this.proc.on('error', (e) => {
      this.spawnError = e
    })
    let buf = ''
    this.proc.stdout.on('data', (d) => {
      buf += d.toString('utf8')
      let i
      while ((i = buf.indexOf('\n')) >= 0) {
        const line = buf.slice(0, i).replace(/\r$/, '')
        buf = buf.slice(i + 1)
        if (line) this.onLine(line)
      }
    })
    this.proc.stderr.on('data', (d) => {
      this.stderr += d.toString('utf8')
    })
    this.proc.stdin.on('error', () => {})
  }

  onLine(line) {
    this.lines.push(line)
    let m
    try {
      m = JSON.parse(line)
    } catch {
      this.badLines.push(line.slice(0, 200))
      return
    }
    if (m.id !== undefined && m.method === undefined) {
      const w = this.waiters.get(JSON.stringify(m.id))
      if (w) {
        this.waiters.delete(JSON.stringify(m.id))
        w(m)
      }
    } else if (m.method) {
      this.notifs.push(m)
    }
  }

  write(s) {
    this.proc.stdin.write(s + '\n')
  }

  request(method, params, timeoutMs = 10000) {
    const id = this.nextId++
    return this.requestRaw(id, { jsonrpc: '2.0', id, method, params }, timeoutMs)
  }

  requestRaw(id, obj, timeoutMs = 10000) {
    return new Promise((resolve, reject) => {
      const key = JSON.stringify(id)
      const to = setTimeout(() => {
        this.waiters.delete(key)
        reject(new Error(`нет ответа на id=${key} за ${timeoutMs} мс (stderr: ${this.stderr.slice(0, 200)})`))
      }, timeoutMs)
      this.waiters.set(key, (m) => {
        clearTimeout(to)
        resolve(m)
      })
      this.write(typeof obj === 'string' ? obj : JSON.stringify(obj))
    })
  }

  notify(method, params) {
    this.write(JSON.stringify({ jsonrpc: '2.0', method, params }))
  }

  async waitNotif(method, timeoutMs = 10000) {
    const end = Date.now() + timeoutMs
    while (Date.now() < end) {
      if (this.notifs.some((n) => n.method === method)) return true
      await sleep(40)
    }
    return false
  }

  // initialize + notifications/initialized
  async init(version = '2025-11-25') {
    const r = await this.request('initialize', { protocolVersion: version, capabilities: {}, clientInfo: { name: 'spike-raw', version: '1' } })
    this.notify('notifications/initialized', {})
    return r
  }

  // end закрывает stdin и ждёт выхода: возвращает {code, ms}.
  async end(timeoutMs = 5000) {
    const t = performance.now()
    this.proc.stdin.end()
    const r = await Promise.race([this.exitP, sleep(timeoutMs).then(() => null)])
    if (!r) {
      this.proc.kill()
      return { code: null, ms: timeoutMs, killed: true }
    }
    return { code: r.code, ms: r.at - t }
  }
}

// finish завершает сценарий: оставшиеся посредники убираются (только свои), код 1 при любом FAIL.
export function finish() {
  for (const p of clients) killTree(p)
  process.exit(fails > 0 ? 1 : 0)
}

export function killTree(proc) {
  try {
    proc.kill()
  } catch {
    /* уже вышел */
  }
}

// startServer запускает слушатель (spikec/spike143 --serve) отдельным процессом и ждёт файл готовности.
export async function startServer(exe, { sock, tag, extra = [], noClean = false } = {}) {
  const ready = path.join(OUT, `ready-${tag}`)
  for (const f of [ready, ready + '.fail']) fs.rmSync(f, { force: true })
  const args = ['--serve', '--ttl', '150', '--ready', ready, '--out', OUT, ...extra]
  if (sock) args.push('--sock', sock)
  if (noClean) args.push('--no-clean')
  const proc = spawn(exe, args, { detached: true, stdio: 'ignore', windowsHide: true })
  let exited = null
  proc.on('exit', (code) => {
    exited = code
  })
  proc.unref()
  for (let i = 0; i < 200; i++) {
    if (fs.existsSync(ready)) {
      const [pid, , rep] = fs.readFileSync(ready, 'utf8').trim().split(' ')
      return { proc, pid: Number(pid), replaced: rep === 'replaced=true', ready }
    }
    if (fs.existsSync(ready + '.fail')) throw new Error(`слушатель не занял сокет: ${fs.readFileSync(ready + '.fail', 'utf8')}`)
    if (exited !== null) throw new Error(`слушатель вышел с кодом ${exited}`)
    await sleep(50)
  }
  killTree(proc)
  throw new Error('слушатель не стал готов за 10 с')
}

// hardKill — taskkill /F /PID на Windows, SIGKILL иначе (чужие процессы не трогаем: только свой pid).
export function hardKill(pid) {
  if (isWin) spawnSync('taskkill', ['/F', '/PID', String(pid)], { encoding: 'utf8' })
  else process.kill(pid, 'SIGKILL')
}

export function stopServer(srv) {
  try {
    if (isWin) spawnSync('taskkill', ['/F', '/PID', String(srv.pid)], { encoding: 'utf8' })
    else process.kill(srv.pid, 'SIGTERM')
  } catch {
    /* уже вышел */
  }
}

export function runGo(exe, args, { timeout = 90000, env = {} } = {}) {
  const r = spawnSync(exe, args, { encoding: 'utf8', timeout, env: { ...process.env, ...env } })
  return { code: r.status, out: r.stdout || '', err: r.stderr || '', error: r.error }
}

if (process.argv[1] && import.meta.url === pathToFileURL(process.argv[1]).href) {
  const [cmd, a, b, c] = process.argv.slice(2)
  if (cmd === 'record') record(a, b, c ?? '')
  else if (cmd === 'ingest') ingest(a, fs.readFileSync(b, 'utf8'))
  else if (cmd === 'snapshot') snapshot(a)
  else {
    console.error('lib.mjs record <id> <статус> [текст] | ingest <префикс> <файл> | snapshot <метка>')
    process.exit(2)
  }
}
