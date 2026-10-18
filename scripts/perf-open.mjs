// Замер открытия большого альбома во фронте (073 bigfront, §6–§7): сборка фронта, синтетический State big200
// (scripts/perf-open/state.mjs), статический сервер + headless Chrome по CDP; щелчок по «Последним».
// Печатает: самую длинную задачу, сумму длинных задач, DOM, img, запросы /media, время правки.
// Код выхода: 0 — пороги §7 выдержаны; 1 — провал порога; 2 — нет Chrome/Edge, сбой запуска или таймаут CDP.
// Шаг QA, в check.ps1 не входит: `node scripts/perf-open.mjs` (из корня; Chrome или Edge — или CHROME=<путь>; --keep — не удалять сборку).
import http from 'node:http'
import fs from 'node:fs'
import os from 'node:os'
import path from 'node:path'
import zlib from 'node:zlib'
import { spawn, spawnSync } from 'node:child_process'
import { fileURLToPath } from 'node:url'
import { bigState } from './perf-open/state.mjs'

// Пороги приёмки §7.
const LIMITS = { longest: 500, total: 1000, dom: 4000, img: 300 }

const here = path.dirname(fileURLToPath(import.meta.url))
const root = path.resolve(here, '..')
const front = path.join(root, 'frontend')
const keep = process.argv.includes('--keep')
const sleep = (ms) => new Promise((r) => setTimeout(r, ms))
let tmp // временная папка сборки и профиля Chrome; die убирает её, если успела появиться
const die = (msg, code = 2) => {
  console.error('perf-open: ' + msg)
  if (tmp && !keep) {
    try {
      fs.rmSync(tmp, { recursive: true, force: true })
    } catch {}
  }
  process.exit(code)
}

function findChrome() {
  const env = process.env
  const cands = [
    env.CHROME,
    'C:/Program Files/Google/Chrome/Application/chrome.exe',
    'C:/Program Files (x86)/Google/Chrome/Application/chrome.exe',
    env.LOCALAPPDATA && path.join(env.LOCALAPPDATA, 'Google/Chrome/Application/chrome.exe'),
    'C:/Program Files (x86)/Microsoft/Edge/Application/msedge.exe', // Edge — как в scripts/screenshot.mjs
    '/Applications/Google Chrome.app/Contents/MacOS/Google Chrome',
    '/usr/bin/google-chrome',
    '/usr/bin/chromium',
  ]
  return cands.find((c) => c && fs.existsSync(c))
}

// PNG-«миниатюра» 256×170: заглушка для /media/*, чтобы браузер декодировал картинку, как настоящую.
function thumb() {
  const w = 256
  const h = 170
  const raw = Buffer.alloc((w * 3 + 1) * h, 0x88)
  for (let y = 0; y < h; y++) raw[y * (w * 3 + 1)] = 0
  const crcT = Array.from({ length: 256 }, (_, n) => {
    let c = n
    for (let k = 0; k < 8; k++) c = c & 1 ? 0xedb88320 ^ (c >>> 1) : c >>> 1
    return c >>> 0
  })
  const crc = (b) => {
    let c = 0xffffffff
    for (const x of b) c = crcT[(c ^ x) & 255] ^ (c >>> 8)
    return (c ^ 0xffffffff) >>> 0
  }
  const chunk = (type, data) => {
    const len = Buffer.alloc(4)
    len.writeUInt32BE(data.length)
    const td = Buffer.concat([Buffer.from(type), data])
    const c = Buffer.alloc(4)
    c.writeUInt32BE(crc(td))
    return Buffer.concat([len, td, c])
  }
  const ihdr = Buffer.alloc(13)
  ihdr.writeUInt32BE(w, 0)
  ihdr.writeUInt32BE(h, 4)
  ihdr[8] = 8
  ihdr[9] = 2
  return Buffer.concat([
    Buffer.from([137, 80, 78, 71, 13, 10, 26, 10]),
    chunk('IHDR', ihdr),
    chunk('IDAT', zlib.deflateSync(raw)),
    chunk('IEND', Buffer.alloc(0)),
  ])
}

const chromePath = findChrome()
if (!chromePath) die('Chrome/Edge не найден. Установите Google Chrome или задайте путь в переменной CHROME.')

// Сборка фронта во временную папку. keep-dist в vite.config пишет dist/gitkeep, поэтому dist должен существовать.
tmp = fs.mkdtempSync(path.join(fs.existsSync(path.join(root, '.gotmp')) ? path.join(root, '.gotmp') : os.tmpdir(), 'sa-perf-open-'))
const dist = path.join(tmp, 'dist')
fs.mkdirSync(path.join(front, 'dist'), { recursive: true })
const vite = path.join(front, 'node_modules', 'vite', 'bin', 'vite.js')
if (!fs.existsSync(vite)) die('нет frontend/node_modules: выполните npm ci в frontend/.')
console.log('perf-open: сборка фронта…')
const b = spawnSync(process.execPath, [vite, 'build', '--outDir', dist, '--emptyOutDir', '--minify', 'false', '--logLevel', 'warn'], {
  cwd: front,
  encoding: 'utf8',
})
if (b.status !== 0) die('сборка фронта не удалась:\n' + (b.stdout || '') + (b.stderr || ''))

const state = Buffer.from(JSON.stringify(bigState()))
const mock = fs.readFileSync(path.join(here, 'perf-open', 'mock.js'))
const png = thumb()
const media = []
const types = {
  '.js': 'text/javascript',
  '.css': 'text/css',
  '.html': 'text/html',
  '.png': 'image/png',
  '.woff2': 'font/woff2',
  '.svg': 'image/svg+xml',
}
const srv = http.createServer((req, res) => {
  const u = new URL(req.url, 'http://x')
  if (u.pathname.startsWith('/media/')) {
    media.push(u.pathname)
    res.writeHead(200, {
      'content-type': 'image/png',
      'cache-control': 'no-store',
    })
    return res.end(png)
  }
  if (u.pathname === '/state.json') return res.end(state)
  if (u.pathname === '/mock.js') {
    res.writeHead(200, { 'content-type': 'text/javascript' })
    return res.end(mock)
  }
  const f = path.join(dist, u.pathname === '/' ? 'index.html' : u.pathname)
  if (!f.startsWith(dist) || !fs.existsSync(f) || fs.statSync(f).isDirectory()) {
    res.writeHead(404)
    return res.end()
  }
  let body = fs.readFileSync(f)
  if (f.endsWith('index.html'))
    body = Buffer.from(String(body).replace('<script type="module"', '<script src="/mock.js"></script><script type="module"'))
  res.writeHead(200, {
    'content-type': types[path.extname(f)] || 'application/octet-stream',
  })
  res.end(body)
})
await new Promise((r) => srv.listen(0, '127.0.0.1', r))
const port = srv.address().port

const prof = path.join(tmp, 'chrome')
const ch = spawn(
  chromePath,
  [
    '--headless=new',
    '--remote-debugging-port=0',
    `--user-data-dir=${prof}`,
    '--window-size=1280,760',
    '--no-first-run',
    '--disable-extensions',
    'about:blank',
  ],
  { stdio: 'ignore' },
)
let ws
let code = 0
const cleanup = async () => {
  try {
    ws?.close()
  } catch {}
  const exited = new Promise((r) => ch.once('exit', r))
  ch.kill()
  srv.close()
  await Promise.race([exited, sleep(3000)])
  // Chrome мог ещё держать файлы профиля: не удалилось — не ошибка замера, папка во временной.
  if (!keep) {
    try {
      fs.rmSync(tmp, {
        recursive: true,
        force: true,
        maxRetries: 5,
        retryDelay: 300,
      })
    } catch {}
  }
}
// Общий предел на весь блок CDP: оборвавшийся сокет иначе вешает скрипт (cdp() ждёт ответ вечно).
const CDP_LIMIT_MS = 120000
const guard = setTimeout(async () => {
  console.error(`perf-open: таймаут ${CDP_LIMIT_MS / 1000} с: Chrome не ответил или сокет CDP закрылся`)
  await cleanup()
  process.exit(2)
}, CDP_LIMIT_MS)
try {
  // Порт отладки Chrome пишет в DevToolsActivePort профиля.
  let dbg
  for (let i = 0; i < 150 && !dbg; i++) {
    await sleep(200)
    try {
      dbg = fs.readFileSync(path.join(prof, 'DevToolsActivePort'), 'utf8').split('\n')[0]
    } catch {}
  }
  if (!dbg) throw new Error('Chrome не отдал порт отладки')
  let wsUrl
  for (let i = 0; i < 50 && !wsUrl; i++) {
    try {
      const l = await (await fetch(`http://127.0.0.1:${dbg}/json/list`)).json()
      wsUrl = l.find((x) => x.type === 'page')?.webSocketDebuggerUrl
    } catch {}
    if (!wsUrl) await sleep(200)
  }
  if (!wsUrl) throw new Error('нет страницы Chrome')
  ws = new WebSocket(wsUrl)
  await new Promise((r, j) => ((ws.onopen = r), (ws.onerror = () => j(new Error('CDP: сокет')))))
  let id = 0
  const pend = new Map()
  ws.onmessage = (m) => {
    const d = JSON.parse(m.data)
    if (d.id && pend.has(d.id)) (pend.get(d.id)(d), pend.delete(d.id))
  }
  const cdp = (method, params = {}) =>
    new Promise((r) => {
      const i = ++id
      pend.set(i, r)
      ws.send(JSON.stringify({ id: i, method, params }))
    })
  const ev = async (expr) =>
    (
      await cdp('Runtime.evaluate', {
        expression: expr,
        returnByValue: true,
        awaitPromise: true,
      })
    ).result?.result?.value
  await cdp('Page.enable')
  await cdp('Runtime.enable')
  await cdp('Emulation.setDeviceMetricsOverride', {
    width: 1280,
    height: 760,
    deviceScaleFactor: 1,
    mobile: false,
  })

  // Тихо: длинных задач нет последние quietMs.
  async function waitQuiet(t0, quietMs = 1500, maxMs = 90000) {
    const start = Date.now()
    while (Date.now() - start < maxMs) {
      await sleep(300)
      const r = await ev(
        `(()=>{const lt=window.__lt||[];const last=lt.length?Math.max(...lt.map(x=>x.s+x.d)):0;return {now:performance.now(),last}})()`,
      )
      if (r && r.now - Math.max(r.last, t0) > quietMs) return
    }
  }

  await cdp('Page.navigate', { url: `http://127.0.0.1:${port}/` })
  await sleep(1500)
  await waitQuiet(0)
  if (!(await ev(`!!document.querySelector('button.row')`)))
    throw new Error('на стартовом экране нет «Последних» (button.row): сценарий устарел')
  const m0 = media.length
  const t0 = await ev('performance.now()')
  await ev(`document.querySelector('button.row').click()`)
  await waitQuiet(t0)
  const r = await ev(
    `({lt:(window.__lt||[]).filter(x=>x.s>=${t0}-5), img:document.images.length, dom:document.getElementsByTagName('*').length, spreads:document.querySelectorAll('.spread').length, err:window.__err.slice(-5)})`,
  )
  const opened = media.length - m0
  const edit = await ev('window.__mock.timeEdit(6)')
  if (r.err.length) console.log('perf-open: ошибки страницы:', r.err.join(' | '))

  const longest = Math.max(0, ...r.lt.map((x) => x.d))
  const total = r.lt.reduce((s, x) => s + x.d, 0)
  const editMax = Math.max(...edit)
  const rows = [
    ['Самая длинная задача', longest, 'мс', LIMITS.longest],
    ['Сумма длинных задач', total, 'мс', LIMITS.total],
    ['DOM (узлов)', r.dom, '', LIMITS.dom],
    ['img', r.img, '', LIMITS.img],
  ]
  console.log(`\nОткрытие big200 из «Последних» (разворотов в DOM: ${r.spreads}, длинных задач: ${r.lt.length})`)
  let fail = 0
  for (const [name, v, unit, lim] of rows) {
    const ok = v <= lim
    if (!ok) fail++
    console.log(`  ${ok ? 'OK  ' : 'FAIL'} ${name.padEnd(22)} ${String(v).padStart(7)} ${unit.padEnd(2)} (порог ≤ ${lim})`)
  }
  console.log(`  --   Запросы /media         ${String(opened).padStart(7)}    (на открытие; всего ${media.length})`)
  console.log(`  --   Правка до кадра        ${String(editMax).padStart(7)} мс (макс. из ${edit.join(', ')}; цель ≤ 60)`)
  code = fail ? 1 : 0
  console.log(fail ? `\nperf-open: ПРОВАЛ, порогов нарушено: ${fail}` : '\nperf-open: OK')
} catch (e) {
  console.error('perf-open: ' + (e?.message || e))
  code = 2
}
clearTimeout(guard)
await cleanup()
process.exit(code)
