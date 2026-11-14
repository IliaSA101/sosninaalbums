// Зонд окна для win-qa «диск снова доступен» (спека 157): исполняется внутри WebView2 exe набора QA (тег qa),
// запускается Go (SA_QA_PROBE, internal/app/qaprobe_qa.go), записи шлёт событием qaprobe. Скрипт только собирает
// наблюдения со временем (t - мс от старта зонда, w - часы машины); вердикты выносит volume_checks.py.
// Python дописывает в конец вызов main(<настройки>). Режимы:
//   sleepy  открыть альбом, фото которого на томе с задержкой первой пробы (шов SA_QA_VOLDELAY): ждать missing -> drive:back;
//   cut     открыть альбом, попросить Python отключить том (запись cut-now), CheckFolders(true) один раз, затем
//           CheckFolders(cfg.full) раз в cfg.periodMs, пока полоса не исчезнет или не выйдет срок.
// Что видно в окне, пишется переходами: полоса недостающих, тост, миниатюры и кадры холста с naturalWidth > 0.
async function main(cfg) {
  const t0 = Date.now()
  const pace = cfg.pace == null ? 1 : cfg.pace // тесты ускоряют паузы; в прогоне всегда 1
  const sleep = (ms) => new Promise((r) => setTimeout(r, ms))
  const wait = (ms) => sleep(ms * pace)
  const send = (rec) => {
    try {
      window.runtime.EventsEmit('qaprobe', JSON.stringify(Object.assign({ t: Date.now() - t0, w: Date.now() }, rec)))
    } catch (e) {
      // нет канала — Python увидит, что записей нет
    }
  }
  const errText = (e) => String((e && e.message) || e).slice(0, 300)
  const step = async (name, fn) => {
    try {
      const rec = await fn()
      send(Object.assign({ step: name, ok: true }, rec || {}))
      return rec || {}
    } catch (e) {
      send({ step: name, ok: false, err: errText(e) })
      return null
    }
  }
  const App = () => window.go.app.App

  // ---- готовность: связка Go и событий есть, окно отвечает ----
  const ready = await step('ready', async () => {
    const end = Date.now() + 60000
    for (;;) {
      try {
        if (window.go && window.go.app && window.go.app.App && window.runtime && window.runtime.EventsOn) {
          await App().LibraryPacks()
          return { ms: Date.now() - t0, mode: cfg.mode }
        }
      } catch (e) {
        // окно ещё поднимается
      }
      if (Date.now() > end) throw new Error('связка Go не появилась за 60 с')
      await sleep(250)
    }
  })
  if (!ready) {
    send({ step: 'done', ok: false })
    return
  }

  // ---- события Go: порядок и время приходов; состояние для выхода из ожиданий ----
  const seen = { missing: 0, lastN: -1, failAt: 0, cleared: false, backs: 0, cutAt: 0 }
  window.runtime.EventsOn('missing', (ids) => {
    const n = Array.isArray(ids) ? ids.length : 0
    seen.missing++
    seen.lastN = n
    if (n > 0 && seen.cutAt && !seen.failAt) seen.failAt = Date.now()
    if (n === 0 && seen.failAt) seen.cleared = true
    send({ step: 'ev', name: 'missing', n })
  })
  window.runtime.EventsOn('drive:back', (d) => {
    seen.backs++
    send({ step: 'ev', name: 'drive:back', drive: d && d.drive, photos: d && d.photos })
  })

  // ---- окно: переходы полосы, тоста, миниатюр и холста (опрос 100 мс) ----
  const dom = { strip: false, toast: '', thumbs: '', canvas: '' }
  const loaded = (sel) => Array.from(document.querySelectorAll(sel)).filter((i) => i.complete && i.naturalWidth > 0).length
  const pollDom = () => {
    if (typeof document === 'undefined') return
    try {
      const strip = !!document.querySelector('[data-guide="missing-bar"]')
      if (strip !== dom.strip) {
        dom.strip = strip
        send({ step: 'strip', on: strip })
      }
      const el = document.querySelector('.toast')
      const text = el ? String(el.textContent || '').trim().slice(0, 160) : ''
      if (text !== dom.toast) {
        dom.toast = text
        send({ step: 'toast', text })
      }
      const th = { n: loaded('[data-thumb] img'), total: document.querySelectorAll('[data-thumb]').length }
      if (th.n + '/' + th.total !== dom.thumbs) {
        dom.thumbs = th.n + '/' + th.total
        send(Object.assign({ step: 'thumbs' }, th))
      }
      const cv = { imgs: loaded('[data-spread] img'), spreads: document.querySelectorAll('[data-spread]').length }
      if (cv.imgs + '/' + cv.spreads !== dom.canvas) {
        dom.canvas = cv.imgs + '/' + cv.spreads
        send(Object.assign({ step: 'canvas' }, cv))
      }
    } catch (e) {
      // DOM в этот момент перестраивается — следующий опрос
    }
  }
  const poller = setInterval(pollDom, 100)
  const finish = async (ok) => {
    pollDom()
    clearInterval(poller)
    send({ step: 'done', ok })
  }
  const until = async (cond, ms) => {
    const end = Date.now() + ms * pace
    while (Date.now() < end) {
      if (cond()) return true
      await sleep(30)
    }
    return cond()
  }

  // ---- открытие альбома: запись opening - точка отсчёта времён ----
  send({ step: 'opening' })
  const opened = await step('opened', async () => {
    await App().OpenRecent(cfg.album)
    return {}
  })
  if (!opened) {
    await finish(false)
    return
  }

  if (cfg.mode === 'sleepy') {
    // Диск «спит»: missing с недостающими, затем пустой список и drive:back; хвост - миниатюры и лишний тост
    await until(() => seen.backs > 0 && seen.cleared, cfg.capMs)
    await wait(cfg.tailMs)
    await finish(true)
    return
  }

  // cut: дождаться итога открытия, попросить Python убрать том, дальше только сверка папок
  await until(() => seen.missing > 0, 8000)
  await wait(500)
  seen.cutAt = Date.now()
  send({ step: 'cut-now' })
  await wait(cfg.cutAfterMs)
  await step('check', async () => {
    await App().CheckFolders(true)
    return { full: true }
  })
  for (;;) {
    if (seen.cleared && seen.backs > 0) break
    if (seen.failAt && Date.now() - seen.failAt > cfg.limitMs * pace) break
    if (!seen.failAt && Date.now() - seen.cutAt > cfg.noFailMs * pace) break
    await wait(cfg.periodMs)
    await step('check', async () => {
      await App().CheckFolders(!!cfg.full)
      return { full: !!cfg.full }
    })
  }
  await wait(cfg.tailMs)
  await finish(true)
}
