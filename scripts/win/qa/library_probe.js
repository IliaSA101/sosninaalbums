// Зонд окна для win-qa (спека 144, решение 0.31-144-59): исполняется внутри WebView2 exe набора QA (тег qa),
// запускается Go (SA_QA_PROBE, internal/app/qaprobe_qa.go), записи шлёт событием qaprobe. Скрипт только
// собирает наблюдения; вердикты выносит library_exe.py (там же их тесты). Python дописывает в конец вызов
// main(<настройки>). Режимы: main (всё, что можно за один сеанс), corrupt (первый показ подменённого листа),
// refetch (докачка после удаления). Каждый шаг в try/catch: сбой шага не мешает остальным.
async function main(cfg) {
  const t0 = Date.now()
  const pace = cfg.pace == null ? 1 : cfg.pace // тесты ускоряют паузы; в прогоне всегда 1
  const sleep = (ms) => new Promise((r) => setTimeout(r, ms))
  const wait = (ms) => sleep(ms * pace)
  const send = (rec) => {
    try {
      window.runtime.EventsEmit('qaprobe', JSON.stringify(Object.assign({ t: Date.now() - t0 }, rec)))
    } catch (e) {
      // нет канала — Python увидит, что записей нет
    }
  }
  const errText = (e) => String((e && e.message) || e).slice(0, 300)
  const step = async (name, fn) => {
    try {
      const rec = await fn()
      send(Object.assign({ step: name, ok: true }, rec || {}))
      return rec
    } catch (e) {
      send({ step: name, ok: false, err: errText(e) })
      return null
    }
  }
  const App = () => window.go.app.App
  const KINDS = ['bg', 'sticker', 'frame', 'spread', 'album']

  // Контрольная сумма тела: сравнение двух ответов без SubtleCrypto (в http-контексте её может не быть)
  const fnv = (u8) => {
    let h = 0x811c9dc5
    for (let i = 0; i < u8.length; i++) h = Math.imul(h ^ u8[i], 0x01000193) >>> 0
    return h.toString(16)
  }

  // Один запрос к /media/lp/: что вернулось и что на самом деле ушло по сети (браузер схлопывает «..» и «%2e%2e»)
  const probe = async (p, init) => {
    const url = '/media/lp/' + p
    const rec = { p, resolved: '', status: 0 }
    try {
      rec.resolved = new URL(url, location.href).pathname
    } catch (e) {
      rec.resolved = '?'
    }
    try {
      const r = await fetch(url, Object.assign({ cache: 'no-store' }, init || {}))
      const buf = new Uint8Array(await r.arrayBuffer())
      rec.status = r.status
      rec.ct = r.headers.get('content-type') || ''
      rec.nosniff = r.headers.get('x-content-type-options') || ''
      rec.cc = r.headers.get('cache-control') || ''
      rec.allow = r.headers.get('allow') || ''
      rec.len = buf.length
      rec.fnv = buf.length ? fnv(buf) : ''
      if (rec.ct.startsWith('text/')) rec.body = new TextDecoder().decode(buf.slice(0, 80))
    } catch (e) {
      rec.err = errText(e)
    }
    return rec
  }

  // Плитки «Можно скачать» всех видов (DecorCatalog, без сети): {kind: [{pack, url, bytes}]}
  const tiles = async () => {
    const out = {}
    for (const k of KINDS) {
      const c = await App().DecorCatalog(k)
      out[k] = (c.get || []).map((t) => ({ pack: t.pack, url: t.url, bytes: t.bytes }))
    }
    return out
  }
  const tilePacks = (tl, kind) => [...new Set((tl[kind] || []).map((t) => t.pack))]
  const cellOf = (tl, pack) => {
    for (const k of KINDS) {
      const t = (tl[k] || []).find((x) => x.pack === pack)
      if (t) return t.url.split('?')[0].replace('/media/lp/', '')
    }
    return ''
  }

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

  // Событие library:previews: ждём один раз, с запасом по времени
  const waitPreviews = (ms) =>
    new Promise((resolve) => {
      let off = null
      const timer = setTimeout(() => {
        if (off) off()
        resolve({ timeout: true })
      }, ms)
      off = window.runtime.EventsOn('library:previews', (d) => {
        clearTimeout(timer)
        if (off) off()
        resolve({ timeout: false, data: d, at: Date.now() })
      })
    })

  if (cfg.mode === 'corrupt') {
    // Лист подменён на диске (тот же размер): плитки по Lstat ещё есть, первый показ сверяет sha256
    const before = await step('c-before', async () => {
      const tl = await tiles()
      return { n: (tl.bg || []).filter((t) => t.pack === cfg.victim).length, cell: cellOf(tl, cfg.victim) }
    })
    await step('c-cell', async () => ({ probe: await probe((before && before.cell) || cfg.victim + '/bg/0') }))
    await step('c-after', async () => {
      const tl = await tiles()
      return { n: KINDS.reduce((s, k) => s + (tl[k] || []).filter((t) => t.pack === cfg.victim).length, 0) }
    })
    send({ step: 'done', ok: true })
    return
  }

  if (cfg.mode === 'refetch') {
    // Новый сеанс после удаления листа: окно открывается — лист докачивается, плитки возвращаются
    const ev = waitPreviews(90000)
    await step('r-fetch', async () => {
      await App().FetchPackPreviews()
      const e = await ev
      return { timeout: e.timeout, ready: e.data && e.data.ready }
    })
    const tl = await step('r-tiles', async () => {
      const t = await tiles()
      return { n: KINDS.reduce((s, k) => s + (t[k] || []).filter((x) => x.pack === cfg.victim).length, 0), cell: cellOf(t, cfg.victim) }
    })
    await step('r-cell', async () => ({ probe: await probe((tl && tl.cell) || cfg.victim + '/bg/0') }))
    send({ step: 'done', ok: true })
    return
  }

  // ---- main ----
  await step('update', async () => {
    let last = null
    for (let i = 1; i <= 2; i++) {
      try {
        const r = await App().UpdateLibrary()
        return { try: i, added: r.added, updated: r.updated, available: r.available, failed: (r.failed || []).length }
      } catch (e) {
        last = e
        await wait(3000)
      }
    }
    throw last
  })
  const packs = await step('packs', async () => {
    const l = await App().LibraryPacks()
    return {
      installed: (l.packs || []).map((p) => p.id),
      available: (l.available || []).map((a) => ({ id: a.id, bytes: a.bytes, why: a.why, donor: a.donor, removed: a.removed })),
    }
  })
  await step('tiles0', async () => {
    const t = await tiles()
    return { counts: Object.fromEntries(KINDS.map((k) => [k, t[k].length])) }
  })

  // Задание образцов идёт фоном, «Скачать» набора — рядом: не мешают друг другу
  const pick = ((packs && packs.available) || [])
    .filter((a) => !a.why && !a.donor && !a.removed && a.bytes > 0)
    .sort((a, b) => a.bytes - b.bytes)[0]
  const evPrev = waitPreviews(120000)
  const tFetch = Date.now()
  await step('overlap', async () => {
    await App().FetchPackPreviews()
    if (!pick) throw new Error('нет набора для «Скачать»')
    const d0 = Date.now()
    let res = null
    let derr = ''
    try {
      res = await App().DownloadLibraryPack(pick.id)
    } catch (e) {
      derr = errText(e)
    }
    const d1 = Date.now()
    const ev = await evPrev
    return {
      pack: pick.id,
      bytes: pick.bytes,
      picked: res ? res.picked : false,
      why: res ? res.why : '',
      derr,
      downloadMs: d1 - d0,
      previewsMs: Date.now() - tFetch,
      previewsTimeout: ev.timeout,
      previewsReady: ev.data && ev.data.ready,
      overlapped: !ev.timeout && ev.at > d0, // задание образцов ещё шло, когда «Скачать» уже начался
    }
  })
  await step('tiles1', async () => {
    const t = await tiles()
    return { counts: Object.fromEntries(KINDS.map((k) => [k, t[k].length])), packs: tilePacks(t, 'bg') }
  })
  const tl = await tiles().catch(() => ({}))
  const bgPacks = tilePacks(tl, 'bg')
  const A = bgPacks[0] || ''
  const cellA = A ? cellOf(tl, A) : ''
  send({ step: 'victim', ok: !!A, victim: A, cell: cellA })

  // Хорошие ответы: тот же лист при раскодировании пути, HEAD, POST, Range/If-None-Match
  await step('good', async () => {
    const direct = await probe(cellA)
    const encoded = await probe(cellA.replace(/\//g, '%2F'))
    const head = await probe(cellA, { method: 'HEAD' })
    const post = await probe(cellA, { method: 'POST' })
    const range = await probe(cellA, { headers: { 'If-None-Match': '*', Range: 'bytes=0-9' } })
    return { cell: cellA, direct, encoded, head, post, range }
  })

  // Обходы: всё 404 и тот же ответ службы, что у заведомо неизвестного набора
  await step('paths', async () => {
    const baseline = await probe('nope-zz/bg/0')
    const items = []
    for (const p of cfg.paths.map((s) => s.split('{A}').join(A || 'nope-zz'))) items.push(await probe(p))
    const baselineSlash = await probe('nope-zz/')
    return { baseline, baselineSlash, items }
  })

  // Быстрое листание: все плитки туда-обратно, часть запросов обрывается (как у <img> вне экрана)
  const urls = [...new Set(KINDS.flatMap((k) => (tl[k] || []).map((t) => t.url.split('?')[0])))]
  const round = async (n) => {
    const rs = { n, passes: cfg.passes, urls: urls.length, requests: 0, got: 0, aborted: 0, bad: {}, errors: 0 }
    const start = Date.now()
    for (let pass = 0; pass < cfg.passes && Date.now() - start < cfg.roundMaxMs; pass++) {
      const list = pass % 2 ? urls.slice().reverse() : urls
      await Promise.all(
        list.map(async (u, i) => {
          rs.requests++
          const ctl = new AbortController()
          const cut = i % 5 < 2
          if (cut) setTimeout(() => ctl.abort(), Math.random() * 6)
          try {
            const r = await fetch(u + '?n=' + n + '-' + pass + '-' + i, { signal: ctl.signal, cache: 'no-store' })
            await r.arrayBuffer()
            if (r.status === 200) rs.got++
            else rs.bad[r.status] = (rs.bad[r.status] || 0) + 1
          } catch (e) {
            if (e && e.name === 'AbortError') rs.aborted++
            else rs.errors++
          }
        }),
      )
      rs.done = pass + 1
    }
    rs.ms = Date.now() - start
    return rs
  }
  if (urls.length) {
    await wait(2000)
    send({ step: 'mark', name: 'baseline' })
    await wait(4000) // Python снимает память сразу после записи
    for (const n of [1, 2]) {
      const rs = await step('round', () => round(n))
      await wait(3000)
      send({ step: 'mark', name: 'r' + n })
      await wait(4000)
      if (!rs) break
    }
  } else {
    send({ step: 'round', ok: false, err: 'нет плиток для листания' })
  }
  await step('final', async () => ({ probe: await probe(cellA) }))

  // Отзыв согласия: клетки отвечают 404 сразу, плиток нет; согласие потом возвращается
  await step('revoke', async () => {
    await App().SetLibraryNet(false, false)
    try {
      const t = await tiles()
      return { counts: Object.fromEntries(KINDS.map((k) => [k, t[k].length])), probe: await probe(cellA) }
    } finally {
      await App().SetLibraryNet(true, false)
    }
  })
  send({ step: 'done', ok: true })
}
