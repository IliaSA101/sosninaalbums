// С4 (design §4.3): ревизии протокола и клиенты через посредника — заглушка, повтор initialize, list_changed, пределы строк.
//   node c4.mjs go  <spikec>   клиент Go SDK (2026-07-28 и 2025-11-25) × сценарии live / late / restart (драйвер spikec --c4)
//   node c4.mjs raw <spikec>   сырой JSON-RPC: заглушка, версии, строка > 1 МБ, повтор initialize, выход по EOF
//   node c4.mjs insp <spikec>  MCP Inspector (--cli) через npx: программа закрыта и открыта
import fs from 'node:fs'
import os from 'node:os'
import path from 'node:path'
import { execFileSync, spawnSync } from 'node:child_process'
import { OUT, RawClient, assert, check, hardKill, ingest, isWin, record, runGo, sleep, startServer, step, finish, stopServer } from './lib.mjs'

const [mode, exeArg] = process.argv.slice(2)
const exe = path.resolve(exeArg || '')
// Сокет — в коротком пути: sun_path 104–108 байт
const sdir = path.join(os.tmpdir(), 's143-c4')
const sock = path.join(sdir, 'sock')
const env = { SPIKE_SOCK: sock, SPIKE_PROBE_MS: '200' }
const fresh = () => fs.rmSync(sdir, { recursive: true, force: true })

async function goClient() {
  for (const rev of ['2026-07-28', '2025-11-25']) {
    for (const scen of ['live', 'late', 'restart']) {
      const r = runGo(exe, ['--c4', '--rev', rev, '--scenario', scen, '--out', OUT], { timeout: 120000 })
      ingest('', r.out)
      if (r.code !== 0 || !r.out.includes('RESULT')) {
        record(`c4go-${rev}/${scen}/exit`, r.code === 0 ? 'INFO' : 'FAIL', `код ${r.code}; ${r.err.slice(0, 300)} ${r.error ?? ''}`)
      }
    }
  }
}

const bigLine = (id, n) => `{"jsonrpc":"2.0","id":${id},"method":"tools/list","params":{"pad":"${'x'.repeat(n)}"}}`

async function raw() {
  const bridge = () => new RawClient(exe, ['--mcp'], env)
  fresh()
  // Программы нет: заглушка
  await check('c4raw/stub-versions', async () => {
    const c = bridge()
    const got = []
    for (const [asked, want] of [['2026-07-28', '2025-11-25'], ['2025-11-25', '2025-11-25'], ['2025-06-18', '2025-06-18'], ['1999-01-01', '2025-11-25']]) {
      const r = await c.request('initialize', { protocolVersion: asked, capabilities: {}, clientInfo: { name: 'raw', version: '1' } })
      assert(r.result?.protocolVersion === want, `запрошена ${asked}, заглушка ответила ${r.result?.protocolVersion}`)
      got.push(`${asked}->${want}`)
    }
    await c.end()
    return got.join(' ')
  })
  await check('c4raw/stub-methods', async () => {
    const c = bridge()
    await c.init()
    const ping = await c.request('ping', {})
    assert(ping.result && !ping.error, 'ping')
    const disc = await c.request('server/discover', {})
    assert(disc.error, 'server/discover заглушка должна отвечать ошибкой, чтобы клиент 2026-07-28 откатился на initialize')
    const unk = await c.request('resources/list', {})
    assert(unk.error?.code === -32000, `прочее: ${JSON.stringify(unk).slice(0, 150)}`)
    const call = await c.request('tools/call', { name: 'nope', arguments: {} })
    assert(call.error, 'неизвестный инструмент без ошибки')
    await c.end()
    return `discover -> ${disc.error.code}, прочее -> ${unk.error.code}`
  })
  await check('c4raw/stub-oversize-line', async () => {
    const c = bridge()
    await c.init()
    const r = await c.requestRaw(7, bigLine(7, 1_200_000), 15000)
    assert(r.error?.code === -32600, `строка 1,2 МБ: ${JSON.stringify(r).slice(0, 150)}`)
    const p = await c.request('ping', {})
    assert(p.result, 'после длинной строки сессия не отвечает')
    const e = await c.end()
    return `ответ-ошибка на id 7, сессия жива, выход ${Math.round(e.ms)} мс`
  })
  await check('c4raw/stub-eof-exit', async () => {
    const c = bridge()
    await c.init()
    const e = await c.end(3000)
    assert(e.code === 0 && e.ms <= 1000, `код ${e.code}, ${Math.round(e.ms)} мс`)
    return `${Math.round(e.ms)} мс`
  })

  // Программа есть с самого начала
  let srv
  await check('c4raw/live-server', async () => {
    srv = await startServer(exe, { sock, tag: 'c4raw' })
    return `pid ${srv.pid}`
  })
  if (srv) {
    await check('c4raw/live-oversize-line', async () => {
      const c = bridge()
      await c.init()
      const r = await c.requestRaw(8, bigLine(8, 1_200_000), 15000)
      assert(r.error?.code === -32600, `строка 1,2 МБ при живой программе: ${JSON.stringify(r).slice(0, 150)}`)
      const t = await c.request('tools/list', {})
      assert(t.result?.tools?.length === 2, 'после длинной строки tools/list не работает')
      await c.end()
    })
    await check('c4raw/live-big-frame-ok', async () => {
      const c = bridge()
      await c.init()
      const r = await c.requestRaw(9, bigLine(9, 900_000), 15000)
      assert(r.result || r.error, 'на строку 0,9 МБ нет ответа')
      await c.end()
      return `ответ получен: ${r.result ? 'result' : 'error ' + r.error.code}`
    })
    await check('c4raw/live-passthrough', async () => {
      const c = bridge()
      const i = await c.init('2025-11-25')
      assert(i.result?.serverInfo?.version === 'spike', `ответ настоящей программы: ${JSON.stringify(i).slice(0, 150)}`)
      const t = await c.request('tools/call', { name: 'album_info', arguments: {} })
      assert(t.result?.content?.[0]?.text === 'spreads=12', `album_info: ${JSON.stringify(t).slice(0, 150)}`)
      await c.end()
    })
    stopServer(srv)
    await sleep(500)
  }

  // Заглушка, потом программа открылась: повтор initialize клиента и list_changed — на двух ревизиях
  for (const rev of ['2025-11-25', '2026-07-28']) {
    await check(`c4raw/replay-${rev}`, async () => {
      fresh()
      const c = bridge()
      const i = await c.init(rev)
      const want = rev === '2026-07-28' ? '2025-11-25' : rev // initialize потолок SDK — 2025-11-25
      assert(i.result?.protocolVersion === want, `заглушка: ${i.result?.protocolVersion}`)
      const t1 = await c.request('tools/list', {})
      assert(t1.result?.tools?.length === 1, 'в заглушке не один инструмент')
      const s = await startServer(exe, { sock, tag: `c4rep-${rev}` })
      try {
        assert(await c.waitNotif('notifications/tools/list_changed', 10000), 'list_changed не пришёл за 10 с')
        const t2 = await c.request('tools/list', {})
        assert(t2.result?.tools?.length === 2, `после повтора initialize инструментов ${t2.result?.tools?.length} (${JSON.stringify(t2).slice(0, 150)})`)
        const call = await c.request('tools/call', { name: 'program_status', arguments: {} })
        assert(/running pid=/.test(call.result?.content?.[0]?.text ?? ''), `вызов: ${JSON.stringify(call).slice(0, 150)}`)
      } finally {
        stopServer(s)
      }
      const e = await c.end()
      assert(c.badLines.length === 0, `в stdout не JSON: ${c.badLines.join('|')}`)
      return `ревизия ${rev}: заглушка → повтор initialize → list_changed → 2 инструмента; выход ${Math.round(e.ms)} мс`
    })
    await sleep(300)
  }

  // Программа убита посреди сессии
  await check('c4raw/server-killed-mid-session', async () => {
    fresh()
    const s = await startServer(exe, { sock, tag: 'c4kill' })
    const c = bridge()
    await c.init()
    const t = await c.request('tools/list', {})
    assert(t.result?.tools?.length === 2, 'до убийства')
    hardKill(s.pid)
    assert(await c.waitNotif('notifications/tools/list_changed', 8000), 'после обрыва list_changed не пришёл')
    const t2 = await c.request('tools/list', {})
    assert(t2.result?.tools?.length === 1, `после обрыва инструментов ${t2.result?.tools?.length} (ждали заглушку)`)
    const e = await c.end()
    return `обрыв → заглушка, выход ${Math.round(e.ms)} мс`
  })
  fresh()
}

async function insp() {
  fresh()
  const npx = isWin ? 'npx.cmd' : 'npx'
  // Окружение в Inspector (TS SDK) передаётся ребёнку только флагом -e: остальное режется до безопасного набора
  const run = (args, tmo = 240000) => {
    const r = spawnSync(npx, ['-y', '@modelcontextprotocol/inspector', '--cli', '-e', `SPIKE_SOCK=${sock}`, '-e', `SPIKE_PROBE_MS=200`, exe, '--mcp', ...args], { encoding: 'utf8', timeout: tmo, shell: isWin })
    return r
  }
  try {
    const v = execFileSync(isWin ? 'npm.cmd' : 'npm', ['view', '@modelcontextprotocol/inspector', 'version'], { encoding: 'utf8', timeout: 120000, shell: isWin }).trim()
    record('c4insp/version', 'INFO', `@modelcontextprotocol/inspector ${v}`)
  } catch (e) {
    record('c4insp/version', 'SKIP', `npm view не ответил: ${e.message.slice(0, 120)} — Inspector пропущен (нет сети?)`)
    return
  }
  await check('c4insp/stub-tools-list', async () => {
    const r = run(['--method', 'tools/list'])
    assert(r.status === 0, `код ${r.status}: ${(r.stderr || '').slice(0, 300)}`)
    const names = (JSON.parse(r.stdout).tools ?? []).map((t) => t.name)
    assert(names.join() === 'program_status', `инструменты заглушки: ${names}`)
    return names.join()
  })
  await check('c4insp/stub-call', async () => {
    const r = run(['--method', 'tools/call', '--tool-name', 'program_status'])
    assert(r.status === 0, `код ${r.status}: ${(r.stderr || '').slice(0, 300)}`)
    const t = JSON.parse(r.stdout).content?.[0]?.text ?? ''
    assert(t.includes('не запущена'), `ответ: ${t.slice(0, 120)}`)
    return t.slice(0, 60)
  })
  let srv
  await check('c4insp/server-start', async () => {
    srv = await startServer(exe, { sock, tag: 'c4insp' })
    return `pid ${srv.pid}`
  })
  if (srv) {
    await check('c4insp/live-tools-list', async () => {
      const r = run(['--method', 'tools/list'])
      assert(r.status === 0, `код ${r.status}: ${(r.stderr || '').slice(0, 300)}`)
      const names = (JSON.parse(r.stdout).tools ?? []).map((t) => t.name).sort()
      assert(names.join() === 'album_info,program_status', `инструменты: ${names}`)
      return names.join()
    })
    await check('c4insp/live-call', async () => {
      const r = run(['--method', 'tools/call', '--tool-name', 'program_status'])
      assert(r.status === 0, `код ${r.status}: ${(r.stderr || '').slice(0, 300)}`)
      const t = JSON.parse(r.stdout).content?.[0]?.text ?? ''
      assert(/running pid=/.test(t), `ответ: ${t.slice(0, 120)}`)
      return t
    })
    stopServer(srv)
  }
  fresh()
}

await step(`c4${mode}`, async () => {
  assert(fs.existsSync(exe), `нет exe: ${exe}`)
  if (mode === 'go') await goClient()
  else if (mode === 'raw') await raw()
  else if (mode === 'insp') await insp()
  else throw new Error('режим: go | raw | insp')
})
finish()
