// Подмена Wails (window.go, window.runtime) для scripts/perf-open.mjs: фронт без Go. Сценарий — пустой альбом
// со «Последними», щелчок по строке открывает big200 (ответ через 400 мс, как холодное открытие).
;(function () {
  const x = new XMLHttpRequest()
  x.open('GET', '/state.json', false)
  x.send()
  const big = JSON.parse(x.responseText)
  const copy = (o) => JSON.parse(JSON.stringify(o))
  const emptyView = copy(big.view)
  Object.assign(emptyView, {
    spreads: [],
    cover: null,
    folders: [],
    path: '',
    rev: 1,
  })
  const empty = {
    ...big,
    view: emptyView,
    photos: [],
    missing: [],
    scan: { active: false, walking: false, found: 0, read: 0 },
  }
  const subs = {}
  window.__lt = []
  window.__err = []
  try {
    new PerformanceObserver((l) => {
      for (const e of l.getEntries())
        window.__lt.push({
          s: Math.round(e.startTime),
          d: Math.round(e.duration),
        })
    }).observe({ type: 'longtask', buffered: true })
  } catch (e) {}
  window.addEventListener('error', (e) => window.__err.push(String(e.message)))
  window.addEventListener('unhandledrejection', (e) => window.__err.push('rej ' + String(e.reason && (e.reason.stack || e.reason))))
  const emit = (name, ...args) => (subs[name] || []).slice().forEach((cb) => cb(...args))
  window.runtime = new Proxy(
    {
      EventsOnMultiple(name, cb) {
        ;(subs[name] = subs[name] || []).push(cb)
        return () => (subs[name] = (subs[name] || []).filter((f) => f !== cb))
      },
      EventsOff() {},
      EventsOffAll() {},
      Environment: () =>
        Promise.resolve({
          platform: 'windows',
          arch: 'amd64',
          buildType: 'production',
        }),
      OnFileDrop() {},
      OnFileDropOff() {},
      WindowGetSize: () => Promise.resolve({ w: 1280, h: 760 }),
    },
    { get: (t, k) => (k in t ? t[k] : () => Promise.resolve(undefined)) },
  )
  const later = (ms, f) => new Promise((r) => setTimeout(() => r(f()), ms))
  let cur = empty
  const api = {
    GetState: () => later(5, () => copy(cur)),
    Recent: () => Promise.resolve(cur.recent),
    OpenRecent: () => {
      setTimeout(() => {
        emit('library', copy(big.photos))
        emit('scan', big.scan)
      }, 30)
      return later(400, () => {
        cur = big
        // как emitState в exe: событие view, затем ответ того же вызова
        emit('view', copy(big.view))
        return copy(big)
      })
    },
    MissingNames: () => Promise.resolve({}),
    Templates: () => Promise.resolve([]),
  }
  window.go = {
    app: {
      App: new Proxy(api, {
        get: (t, k) => (k in t ? t[k] : () => Promise.resolve(null)),
      }),
    },
  }
  window.__mock = {
    // Время правки до кадра: применение вида + раскладка + отрисовка (rAF, затем задача), мс.
    timeEdit(n = 6) {
      const one = (k) =>
        new Promise((res) => {
          const v = copy(big.view)
          v.rev = big.view.rev + 10 + k
          v.spreads[k % v.spreads.length].name = 'Правка ' + k
          const t = performance.now()
          emit('view', v)
          requestAnimationFrame(() => setTimeout(() => res(performance.now() - t), 0))
        })
      return (async () => {
        const out = []
        for (let k = 0; k < n; k++) out.push(Math.round(await one(k)))
        return out
      })()
    },
  }
})()
