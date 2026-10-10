// Снимок окна для README: docs/screenshot.png (1600×900). Запуск: node scripts/screenshot.mjs [--spread N]
// Нужны Node ≥ 22 (встроенный WebSocket), Edge и запущенный `wails dev` (http://localhost:34115).
// Альбом не правится: меняется только уровень интерфейса («Полный»), по окончании он возвращается.
import { spawn } from 'node:child_process'
import { mkdirSync, mkdtempSync, rmSync, writeFileSync } from 'node:fs'
import { createServer } from 'node:net'
import { dirname, join, resolve } from 'node:path'
import { fileURLToPath } from 'node:url'

const root = resolve(dirname(fileURLToPath(import.meta.url)), '..')
const EDGE = 'C:/Program Files (x86)/Microsoft/Edge/Application/msedge.exe'
const URL = 'http://localhost:34115'
const sleep = (ms) => new Promise((r) => setTimeout(r, ms))

const argAt = process.argv.indexOf('--spread')
const wanted = argAt > 0 ? Number(process.argv[argAt + 1]) : 0 // номер с 1; 0 — самый наполненный

const freePort = () =>
  new Promise((res, rej) => {
    const s = createServer()
    s.listen(0, '127.0.0.1', () => {
      const { port } = s.address()
      s.close(() => res(port))
    })
    s.on('error', rej)
  })

mkdirSync(join(root, '.gotmp'), { recursive: true })
const profile = mkdtempSync(join(root, '.gotmp', 'shot-'))
const port = await freePort()
const edge = spawn(
  EDGE,
  [
    '--headless=new',
    `--remote-debugging-port=${port}`,
    `--user-data-dir=${profile}`,
    '--window-size=1600,900',
    '--hide-scrollbars',
    'about:blank',
  ],
  { stdio: 'ignore' },
)

let ws
let restore = null // { level } для возврата
let call = null
try {
  // Ждём страницу в списке целей DevTools.
  let target
  for (let i = 0; i < 60 && !target; i++) {
    try {
      const list = await (await fetch(`http://127.0.0.1:${port}/json`)).json()
      target = list.find((t) => t.type === 'page')
    } catch {
      /* Edge ещё стартует */
    }
    if (!target) await sleep(250)
  }
  if (!target) throw new Error('Edge не запустился')

  ws = new WebSocket(target.webSocketDebuggerUrl)
  await new Promise((res, rej) => {
    ws.onopen = res
    ws.onerror = () => rej(new Error('нет соединения с DevTools'))
  })
  let id = 0
  const pending = new Map()
  ws.onmessage = (m) => {
    const d = JSON.parse(m.data)
    const p = pending.get(d.id)
    if (!p) return
    pending.delete(d.id)
    if (d.error) p.rej(new Error(d.error.message))
    else p.res(d.result)
  }
  call = (method, params = {}) =>
    new Promise((res, rej) => {
      pending.set(++id, { res, rej })
      ws.send(JSON.stringify({ id, method, params }))
    })
  const evalJs = async (expr) => {
    const r = await call('Runtime.evaluate', { expression: expr, awaitPromise: true, returnByValue: true })
    if (r.exceptionDetails) throw new Error(r.exceptionDetails.exception?.description ?? r.exceptionDetails.text)
    return r.result.value
  }

  await call('Emulation.setDeviceMetricsOverride', { width: 1600, height: 900, deviceScaleFactor: 1, mobile: false })
  await call('Page.navigate', { url: URL })
  // Ждём, пока загрузится интерфейс и Go отдаст состояние.
  let st
  for (let i = 0; i < 60 && !st; i++) {
    await sleep(500)
    st = await evalJs(
      `(async()=>{try{const s=await window.go.app.App.GetState();return {level:s.prefs?.level,n:s.view.spreads.length,counts:s.view.spreads.map(x=>x.photos.length)}}catch(e){return null}})()`,
    ).catch(() => null)
  }
  if (!st) throw new Error('GetState недоступен')

  restore = { level: st.level }
  if (st.level !== 'full') await evalJs(`window.go.app.App.SetLevel('full')`)
  // Уровень применяется при загрузке настроек: перезагрузить страницу, чтобы фронт его прочитал.
  await call('Page.reload')
  await sleep(2500)

  // Закрыть стартовое окно (Esc, если оно есть).
  await evalJs(`document.dispatchEvent(new KeyboardEvent('keydown',{key:'Escape',bubbles:true}))`)
  await sleep(500)

  let idx = wanted > 0 ? wanted - 1 : st.counts.indexOf(Math.max(...st.counts))
  if (idx < 0 || idx >= st.n) throw new Error(`нет разворота ${idx + 1} (всего ${st.n})`)
  const spreadId = await evalJs(`window.go.app.App.GetState().then(s=>s.view.spreads[${idx}].id)`)
  await evalJs(
    `(()=>{const el=document.querySelector('[data-drop="mini"][data-spread="${spreadId}"]');if(!el)throw new Error('нет миниатюры разворота');el.scrollIntoView({inline:'center'});el.click()})()`,
  )
  await sleep(1500)

  const shot = await call('Page.captureScreenshot', { format: 'png', captureBeyondViewport: false })
  const out = join(root, 'docs', 'screenshot.png')
  writeFileSync(out, Buffer.from(shot.data, 'base64'))
  console.log(`разворот ${idx + 1} из ${st.n} (фото: ${st.counts[idx]}) -> ${out}`)
} finally {
  try {
    if (restore && restore.level !== 'full') {
      await call?.('Runtime.evaluate', {
        expression: `window.go.app.App.SetLevel(${JSON.stringify(restore.level)})`,
        awaitPromise: true,
      })
      console.log(`уровень возвращён: ${restore.level}`)
    }
  } catch (e) {
    console.error('не удалось вернуть уровень:', e.message)
  }
  try {
    ws?.close()
  } catch {
    /* уже закрыт */
  }
  edge.kill()
  await sleep(1000)
  try {
    rmSync(profile, { recursive: true, force: true, maxRetries: 5, retryDelay: 300 })
  } catch {
    /* профиль занят — останется в .gotmp */
  }
}
