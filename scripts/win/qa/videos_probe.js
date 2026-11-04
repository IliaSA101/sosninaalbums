// Зонд окна для проверки роликов справочника F1 (спека 140, Т7; образец — library_probe.js): исполняется внутри
// WebView2 (Windows) и WKWebView (macOS) программы с тегом qa, запускается Go (SA_QA_PROBE), записи шлёт событием qaprobe.
// Скрипт только собирает наблюдения; вердикты выносит videos_exe.py (там же их тесты). Python дописывает вызов main(<настройки>).
// Режимы: main (скачивание службой Go, игра роликов тем же путём, что окно: fetch -> Blob -> <video>, затем окно справочника),
// corrupt (файл ролика испорчен на диске при том же размере: «Ролик не открылся», «Скачать заново», игра после замены).
// Надписи «Скачать»/«Скачиваем» не используются: скачивание - DownloadGuideVideos, как у videos.get. Каждый шаг в try/catch.
async function main(cfg) {
  const t0 = Date.now()
  const pace = cfg.pace == null ? 1 : cfg.pace // тесты ускоряют паузы; в прогоне всегда 1
  const sleep = (ms) => new Promise((r) => setTimeout(r, ms))
  const wait = (ms) => sleep(ms * pace)
  const send = (rec) => {
    try {
      window.runtime.EventsEmit('qaprobe', JSON.stringify(Object.assign({ t: Date.now() - t0 }, rec)))
    } catch (e) {
      // нет канала - Python увидит, что записей нет
    }
  }
  const errText = (e) => String((e && e.message) || e).slice(0, 300)
  const App = () => window.go.app.App
  // Что видно в окне: для разбора провала UI-шагов (снимка экрана у WebView нет)
  const screenText = () => String((document.body && document.body.innerText) || '').replace(/\s+/g, ' ').slice(0, 400)
  const step = async (name, fn) => {
    try {
      const rec = await fn()
      send(Object.assign({ step: name, ok: true }, rec || {}))
      return rec
    } catch (e) {
      send({ step: name, ok: false, err: errText(e), screen: screenText() })
      return null
    }
  }
  const until = async (fn, ms) => {
    const end = Date.now() + ms * pace
    for (;;) {
      let v = null
      try {
        v = fn()
      } catch (e) {
        v = null
      }
      if (v) return v
      if (Date.now() > end) return null
      await sleep(100)
    }
  }

  // Наблюдение за <video>: метаданные, кадр, ход времени. Источник url ставим сами; у окна справочника src уже есть.
  const observe = async (v, url) => {
    const rec = { events: [] }
    for (const n of ['loadedmetadata', 'loadeddata', 'canplay', 'playing', 'error', 'stalled', 'waiting']) {
      v.addEventListener(n, () => rec.events.push(n))
    }
    if (url) {
      v.muted = true
      v.playsInline = true
      v.preload = 'auto'
      v.src = url
    }
    await until(() => v.readyState >= 1 || v.error, 15000)
    try {
      await v.play()
    } catch (e) {
      rec.playErr = errText(e)
    }
    await until(() => v.readyState >= 2 || v.error, 10000)
    // Время в кадре за 3 с: растёт, а не стоит
    rec.times = []
    for (let i = 0; i < 4; i++) {
      rec.times.push(Math.round(v.currentTime * 1000) / 1000)
      if (i < 3) await wait(1000)
    }
    rec.readyState = v.readyState
    rec.networkState = v.networkState
    rec.paused = v.paused
    rec.w = v.videoWidth
    rec.h = v.videoHeight
    rec.duration = Number.isFinite(v.duration) ? Math.round(v.duration * 100) / 100 : -1
    rec.error = v.error ? { code: v.error.code, message: String(v.error.message || '').slice(0, 120) } : null
    return rec
  }

  // Путь окна ролика (VideoWindow.svelte): fetch целиком, Blob с типом ответа, <video> с этим Blob
  const playBlob = async (id) => {
    const res = await fetch('/media/guide-video/' + encodeURIComponent(id), { cache: 'no-store' })
    const rec = { id, status: res.status, ct: res.headers.get('content-type') || '' }
    const body = await res.blob()
    rec.bytes = body.size
    if (!res.ok) return rec
    const url = URL.createObjectURL(new Blob([body], { type: rec.ct.split(';')[0] }))
    const v = document.createElement('video')
    v.setAttribute('style', 'position:fixed;left:0;top:0;width:320px;z-index:99999')
    document.body.appendChild(v)
    try {
      Object.assign(rec, await observe(v, url))
    } finally {
      v.remove()
      URL.revokeObjectURL(url)
    }
    return rec
  }

  // ---- окно справочника: клавиши и клики так, как их делает человек ----
  const key = (k) => document.body.dispatchEvent(new KeyboardEvent('keydown', { key: k, code: k, bubbles: true, cancelable: true }))
  const btn = (root, re) => [...root.querySelectorAll('button')].find((b) => re.test(b.textContent || ''))
  const pane = () => document.querySelector('.guide .pane')
  const closeAll = async () => {
    for (let i = 0; i < 5; i++) {
      if (!document.querySelector('.guide') && !document.querySelector('video')) return
      key('Escape')
      await sleep(250)
    }
  }
  const openArticle = async (title) => {
    await closeAll()
    key('F1')
    const q = await until(() => document.querySelector('.guide .q'), 8000)
    if (!q) throw new Error('справочник не открылся по F1')
    q.value = title
    q.dispatchEvent(new Event('input', { bubbles: true }))
    const opt = await until(() => [...document.querySelectorAll('.guide [role=option]')].find((o) => (o.textContent || '').includes(title)), 8000)
    if (!opt) throw new Error('статьи «' + title + '» нет в списке')
    opt.click()
    if (!(await until(() => pane() && (pane().textContent || '').includes('Ролик'), 8000))) throw new Error('в карточке нет строки «Ролик»')
  }
  const uiPlay = async (title) => {
    await openArticle(title)
    const watch = await until(() => pane() && btn(pane(), /Смотреть/), 20000)
    if (!watch) throw new Error('нет кнопки «Смотреть»')
    watch.click()
    const v = await until(() => document.querySelector('video'), 20000)
    if (!v) throw new Error('окно ролика не открылось')
    const rec = await observe(v, '')
    rec.inDialog = !!v.closest('[role=dialog], .modal, dialog')
    return rec
  }

  // ---- готовность: связка Go и событий есть, окно отвечает ----
  const ready = await step('ready', async () => {
    const end = Date.now() + 60000
    for (;;) {
      try {
        if (window.go && window.go.app && window.go.app.App && window.runtime && window.runtime.EventsOn) {
          await App().GuideVideos()
          return { ms: Date.now() - t0, mode: cfg.mode, ua: navigator.userAgent.slice(0, 160) }
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
  const pick = (items) => (items || []).map((i) => ({ id: i.id, version: i.version, seconds: i.seconds, size: i.size, ready: !!i.ready }))

  await step('consent', async () => ({ net: await App().SetLibraryNet(true, false) }))
  await step('update', async () => {
    let last = null
    for (let i = 1; i <= 2; i++) {
      try {
        const r = await App().UpdateLibrary()
        return { try: i, available: r.available }
      } catch (e) {
        last = e
        await wait(2000)
      }
    }
    throw last
  })

  if (cfg.mode === 'corrupt') {
    const title = cfg.titles[cfg.victim]
    await step('list-corrupt', async () => ({ items: pick(await App().GuideVideos()) }))
    // Путь окна на испорченном файле: размер тот же, Go его принял - ролик должен не открыться
    await step('play-corrupt', () => playBlob(cfg.victim))
    // Карточка: «Смотреть» -> окно закрылось само -> «Ролик не открылся…», «Показать файл», «Скачать заново»
    await step('ui-broken', async () => {
      await openArticle(title)
      const watch = await until(() => pane() && btn(pane(), /Смотреть/), 20000)
      if (!watch) throw new Error('нет кнопки «Смотреть»')
      watch.click()
      const ok = await until(() => pane() && (pane().textContent || '').includes('Ролик не открылся в окне программы'), 30000)
      const p = pane()
      return {
        broken: !!ok,
        reveal: !!(p && btn(p, /Показать файл/)),
        redo: !!(p && btn(p, /Скачать заново/)),
        watch: !!(p && btn(p, /Смотреть/)),
        windowGone: !document.querySelector('video'),
      }
    })
    // «Скачать заново»: Go сверяет sha256 и заменяет файл; строка возвращается к «Смотреть»
    await step('ui-redo', async () => {
      const p = pane()
      const redo = p && btn(p, /Скачать заново/)
      if (!redo) throw new Error('нет кнопки «Скачать заново»')
      redo.click()
      const back = await until(() => pane() && btn(pane(), /Смотреть/) && !(pane().textContent || '').includes('Ролик не открылся'), 60000)
      const items = pick(await App().GuideVideos())
      return { back: !!back, items }
    })
    await step('play-redo', () => playBlob(cfg.victim))
    await step('ui-play-redo', () => uiPlay(title))
    await closeAll()
    send({ step: 'done', ok: true })
    return
  }

  await step('list-before', async () => ({ items: pick(await App().GuideVideos()) }))
  await step('download', async () => {
    const t = Date.now()
    const items = pick(await App().DownloadGuideVideos(cfg.ids))
    return { ms: Date.now() - t, items }
  })
  await step('list-after', async () => ({ items: pick(await App().GuideVideos()) }))
  for (const id of cfg.ids) await step('play:' + id, () => playBlob(id))
  // Тот же ролик в настоящем окне справочника (Modal + <video>)
  await step('ui-play', () => uiPlay(cfg.titles[cfg.ids[0]]))
  await closeAll()
  send({ step: 'done', ok: true })
}
