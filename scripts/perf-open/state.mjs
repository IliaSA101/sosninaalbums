// Синтетический State «big200» для scripts/perf-open.mjs: 200 разворотов по 16 фото (сетка 4×4), как
// `qa-bigalbum.go -spreads 200 -per 16`, но без файлов и личных путей. Форма — frontend/src/base/types
// (State, View, SpreadView, FrameView): геометрию в мм обычно считает Go, здесь она ровная сетка.
const PAGE_W = 210
const PAGE_H = 297
const MARGIN = 10
const GAP = 3

export function bigState(spreads = 200, per = 16) {
  const photos = []
  const views = []
  const cols = Math.ceil(Math.sqrt(per))
  const rows = Math.ceil(per / cols)
  const W = PAGE_W * 2
  const cw = (W - 2 * MARGIN - (cols - 1) * GAP) / cols
  const ch = (PAGE_H - 2 * MARGIN - (rows - 1) * GAP) / rows
  for (let s = 0; s < spreads; s++) {
    const ids = []
    const rects = []
    const frames = []
    for (let k = 0; k < per; k++) {
      const n = photos.length + 1
      const num = String(n).padStart(5, '0')
      const id = `p${num}`
      const land = n % 3 !== 0 // часть фото вертикальные
      photos.push({
        id,
        path: `/perf/album/IMG_${num}.jpg`,
        name: `IMG_${num}.jpg`,
        w: land ? 6000 : 4000,
        h: land ? 4000 : 6000,
      })
      ids.push(id)
      const r = {
        x: MARGIN + (k % cols) * (cw + GAP),
        y: MARGIN + Math.floor(k / cols) * (ch + GAP),
        w: cw,
        h: ch,
      }
      rects.push(r)
      frames.push({
        s: 1,
        x: 0,
        y: 0,
        r: 0,
        img: { ...r },
        turn: 0,
        tilt: 0,
        dpi: 300,
        flipH: false,
        flipV: false,
        gray: false,
        bright: 0,
        contrast: 0,
        filter: '',
        cellRot: 0,
        z: k,
      })
    }
    views.push({
      id: `s${s + 1}`,
      name: '',
      photos: ids,
      w: W,
      h: PAGE_H,
      mid: PAGE_W,
      rects,
      frames,
      texts: [],
      captions: [],
      bg: ['#ffffff', '#ffffff'],
      bgSet: ['', ''],
      layout: 'auto',
      variant: 0,
      variants: 1,
      more: false,
      rearranged: false,
      pages: null,
      grid: false,
      places: null,
      backdrops: null,
      lowDPI: 0,
    })
  }
  const view = {
    spreads: views,
    settings: {
      pageW: PAGE_W,
      pageH: PAGE_H,
      margin: MARGIN,
      gap: GAP,
      layflat: false,
      foldAny: false,
      dpi: 300,
      bg: '',
      safe: 0,
    },
    folders: ['/perf/album'],
    path: '/perf/big200.salbum',
    dirty: false,
    canUndo: false,
    canRedo: false,
    rev: 1,
    maxSpreadPhotos: 64,
    minDPI: 200,
    lowDPI: 0,
    emptyPlaces: 0,
    textMaxW: 400,
    backdropUsed: null,
    marksRev: 0,
  }
  return {
    view,
    photos,
    scan: {
      active: false,
      walking: false,
      found: photos.length,
      read: photos.length,
    },
    missing: [],
    version: '0.0.0-perf',
    fonts: [],
    prefs: { level: 'full', showStart: true, first: false, custom: null },
    recent: [{ path: view.path, name: 'big200', exists: true, modified: Date.now() }],
  }
}
