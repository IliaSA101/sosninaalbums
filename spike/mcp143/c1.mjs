// С1 (design §4.3): stdio у exe с подсистемой GUI. node spawn(exe, ["--mcp"], {stdio: "pipe"}) →
// initialize, tools/list, tools/call program_status; ответы ≤ 2 с, окна нет, «окно программы» не открылось,
// выход по EOF stdin ≤ 1 с, stdout только JSON, stderr пуст. Usage: node c1.mjs <exe с подсистемой GUI>
import fs from 'node:fs'
import path from 'node:path'
import { spawnSync } from 'node:child_process'
import { OUT, RawClient, assert, check, isWin, record, startServer, step, finish, stopServer } from './lib.mjs'

const exe = path.resolve(process.argv[2] || '')

// Один сеанс агента: возвращает замеры для проверок. Ошибки внутри проверок не прерывают соседние.
async function session(tag) {
  const c = new RawClient(exe, ['--mcp'])
  const m = {}
  await check(`c1/${tag}/spawn`, async () => {
    await new Promise((res, rej) => {
      c.proc.once('spawn', res)
      c.proc.once('error', rej)
    })
    return `pid=${c.proc.pid}`
  })
  if (c.spawnError || !c.proc.pid) return null
  await check(`c1/${tag}/initialize`, async () => {
    const r = await c.request('initialize', { protocolVersion: '2025-11-25', capabilities: {}, clientInfo: { name: 'spike-c1', version: '1' } }, 8000)
    m.initMs = performance.now() - c.t0
    assert(r.result && r.result.serverInfo, `нет result: ${JSON.stringify(r).slice(0, 200)}`)
    assert(m.initMs <= 2000, `initialize с запуска ${Math.round(m.initMs)} мс > 2000`)
    return `${Math.round(m.initMs)} мс, ревизия ${r.result.protocolVersion}, сервер ${r.result.serverInfo.name}`
  })
  c.notify('notifications/initialized', {})
  await check(`c1/${tag}/tools-list`, async () => {
    const r = await c.request('tools/list', {}, 5000)
    const n = (r.result?.tools ?? []).map((t) => t.name)
    assert(n.includes('program_status') && n.includes('album_info'), `инструменты: ${n.join(',')}`)
    return n.join(',')
  })
  await check(`c1/${tag}/tools-call`, async () => {
    const r = await c.request('tools/call', { name: 'program_status', arguments: {} }, 5000)
    const t = r.result?.content?.[0]?.text ?? ''
    assert(/running pid=\d+/.test(t), `ответ: ${JSON.stringify(r).slice(0, 200)}`)
    m.total = performance.now() - c.t0
    assert(m.total <= 2000, `весь обмен с запуска ${Math.round(m.total)} мс > 2000`)
    return `${t}; весь обмен ${Math.round(m.total)} мс`
  })
  await check(`c1/${tag}/no-window`, async () => {
    if (!isWin) return 'н/п: окна подсистемы GUI только в Windows'
    const r = spawnSync('powershell', ['-NoProfile', '-Command', `$p = Get-Process -Id ${c.proc.pid}; "$($p.MainWindowHandle)|$($p.MainWindowTitle)"`], { encoding: 'utf8', timeout: 20000 })
    const [h, title] = r.stdout.trim().split('|')
    assert(h === '0' && !title, `у посредника есть окно: handle=${h} title=${title} (${r.stderr})`)
    return 'MainWindowHandle=0'
  })
  await check(`c1/${tag}/eof-exit`, async () => {
    const e = await c.end(5000)
    assert(!e.killed, 'не вышел за 5 с после закрытия stdin')
    assert(e.code === 0, `код выхода ${e.code}`)
    assert(e.ms <= 1000, `выход после EOF ${Math.round(e.ms)} мс > 1000`)
    return `${Math.round(e.ms)} мс, код 0`
  })
  await check(`c1/${tag}/stdout-clean`, async () => {
    assert(c.badLines.length === 0, `в stdout не JSON: ${c.badLines.join(' | ')}`)
    return `${c.lines.length} строк JSON`
  })
  await check(`c1/${tag}/stderr-empty`, async () => {
    assert(c.stderr === '', `stderr: ${c.stderr.slice(0, 200)}`)
  })
  return m
}

await step('c1', async () => {
  assert(fs.existsSync(exe), `нет exe: ${exe}`)
  let srv
  await check('c1/server-start', async () => {
    srv = await startServer(exe, { tag: 'c1' })
    return `программа слушает, pid ${srv.pid}`
  })
  if (srv) {
    await session('cold')
    await session('warm')
    // Контроль приёма: запуск без --mcp «открывает окно» и оставляет метку, значит её отсутствие после --mcp что-то значит
    await check('c1/window-marker-control', async () => {
      spawnSync(exe, [], { timeout: 15000, env: { ...process.env, SPIKE_OUT: OUT } })
      const marks = fs.readdirSync(OUT).filter((f) => f.startsWith('window-opened-'))
      assert(marks.length === 1, `меток окна после запуска без --mcp: ${marks.length}`)
      fs.rmSync(path.join(OUT, marks[0]))
    })
    await check('c1/no-second-window', async () => {
      const marks = fs.readdirSync(OUT).filter((f) => f.startsWith('window-opened-'))
      assert(marks.length === 0, `посредник открыл «окно»: ${marks.join(',')}`)
    })
    await check('c1/server-saw-sessions', async () => {
      const log = fs.readFileSync(path.join(OUT, 'spike.log'), 'utf8')
      const n = (log.match(/сессия закрыта/g) || []).length
      assert(n >= 2, `программа видела закрытых сессий: ${n}`)
      return `${n} сессий`
    })
    stopServer(srv)
  }
  record('c1/note', 'INFO', 'Kaspersky/SmartScreen и Claude Desktop — на ноутбуке (С3), в CI не проверяются')
})
finish()
