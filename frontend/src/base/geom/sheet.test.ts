import { describe, expect, it } from 'vitest'
import { bdLines } from './bdbox'
import {
  BORDER_NOTE,
  SHEET_PRESETS,
  hasBorders,
  hasSeam,
  isCover,
  isSheet,
  newSheetSize,
  pageAt,
  pageSize,
  sheetIn,
  sheetSize,
  sheetsOf,
  unitOf,
} from './sheet'
import { snapLines, snapThreshold } from './snap'

describe('sheet', () => {
  const spread = { w: 560, h: 280, mid: 280 }
  const vignette = { w: 204, h: 304, mid: 0 }

  it('sheetSize берёт w/h/mid листа как есть', () => {
    expect(sheetSize(spread)).toEqual({ w: 560, h: 280, mid: 280 })
    expect(sheetSize(vignette)).toEqual({ w: 204, h: 304, mid: 0 })
  })
  it('шов есть только при mid > 0', () => {
    expect(hasSeam(spread)).toBe(true)
    expect(hasSeam(vignette)).toBe(false)
  })
  it('newSheetSize: разворот — две страницы, лист — одна', () => {
    expect(newSheetSize({ pageW: 280, pageH: 284 })).toEqual({ w: 560, h: 284, mid: 280 })
    expect(newSheetSize({ pageW: 204, pageH: 304, sheet: true })).toEqual({ w: 204, h: 304, mid: 0 })
  })
  it('pageSize — формат страницы альбома', () => {
    expect(pageSize({ pageW: 281, pageH: 284 })).toEqual({ w: 281, h: 284 })
  })
  it('pageAt: левая и правая по шву; без шва страница одна', () => {
    expect(pageAt(spread, 10)).toBe(0)
    expect(pageAt(spread, 280)).toBe(1)
    expect(pageAt(spread, 559)).toBe(1)
    expect(pageAt(vignette, 190)).toBe(0)
  })
})

describe('режим листа', () => {
  it('isSheet читает settings.sheet', () => {
    expect(isSheet({ sheet: true })).toBe(true)
    expect(isSheet({})).toBe(false)
    expect(isSheet(undefined)).toBe(false)
  })
  it('слова интерфейса: лист вместо разворота', () => {
    expect(unitOf({ sheet: true })).toMatchObject({ cap: 'Лист', new: 'Новый лист', all: 'Весь лист', every: 'Все листы' })
    expect(unitOf({})).toMatchObject({ cap: 'Разворот', new: 'Новый разворот', all: 'Весь разворот', every: 'Все развороты' })
    expect(unitOf({ sheet: true })).toMatchObject({ many: 'листы', of: 'листа' })
    expect(unitOf({})).toMatchObject({ many: 'развороты', of: 'разворота' })
  })
  it('пресеты листа — 20×30 и 30×40 см', () => {
    expect(SHEET_PRESETS.map((p) => [p.w, p.h])).toEqual([
      [204, 304],
      [304, 404],
    ])
  })
})

describe('линии прилипания без шва', () => {
  it('snapLines разворота — как раньше (шов и центры страниц)', () => {
    const l = snapLines({ w: 560, h: 280, mid: 280, margin: 12, rects: [] })
    expect(l.v).toEqual([0, 12, 140, 268, 280, 292, 420, 548, 560])
  })
  it('snapLines листа виньетки: нет шва и центров страниц', () => {
    const l = snapLines({ w: 200, h: 300, mid: 0, margin: 10, rects: [] })
    expect(l.v).toEqual([0, 10, 100, 190, 200])
  })
  it('bdLines: шов только при mid > 0', () => {
    const area = { x: 0, y: 0, w: 200, h: 300 }
    expect(bdLines(area, 0).v).toEqual([0, 100, 200])
    expect(bdLines({ ...area, w: 560 }, 280).v).toEqual([0, 280, 560])
  })
  it('snapThreshold по ширине листа в мм', () => {
    expect(snapThreshold(1000, 500)).toBeCloseTo(3)
  })
})

// Сканер (как TestPageWReaders в Go): settings.pageW читает только sheet.ts и места, где его правят как число
// альбома. Остальной код берёт размер из SpreadView (w/h/mid), иначе обложка и лист виньетки поедут.
describe('чтения pageW', () => {
  const ALLOWED = [
    '/src/base/geom/sheet.ts',
    '/src/base/types/app.ts', // описание Settings
    '/src/base/core/nav.svelte.ts', // значения по умолчанию до первого ответа Go (emptyView)
    '/src/panels/details/SettingsForm.svelte', // форма правки настроек альбома
    '/src/panels/details/presets.ts', // пресеты форматов — поля той же формы
    '/src/panels/details/settingsnorm.ts', // нормализация полей той же формы (вынесена из SettingsForm, 066)
  ]
  const files = import.meta.glob('/src/**/*.{ts,svelte}', { query: '?raw', import: 'default', eager: true }) as Record<string, string>

  it('вне разрешённых файлов pageW не читается', () => {
    const bad: string[] = []
    for (const [path, src] of Object.entries(files)) {
      if (/\.test\.ts$/.test(path) || ALLOWED.includes(path)) continue
      src.split('\n').forEach((line, i) => {
        const code = line.replace(/\/\/.*$/, '').replace(/<!--.*?-->/g, '')
        if (/\bpageW\b/.test(code)) bad.push(`${path}:${i + 1}: ${line.trim()}`)
      })
    }
    expect(bad).toEqual([])
  })
  it('сканер видит файлы', () => {
    expect(Object.keys(files).length).toBeGreaterThan(100)
    expect(ALLOWED.every((p) => p in files)).toBe(true)
  })
})

describe('лист обложки (032)', () => {
  const sp = (id: string) => ({ id }) as never
  const view = { spreads: [sp('a'), sp('b')], cover: { sheet: sp('cover') } }
  it('isCover узнаёт лист по id', () => {
    expect(isCover(sp('cover'))).toBe(true)
    expect(isCover(sp('a'))).toBe(false)
    expect(isCover(undefined)).toBe(false)
  })
  it('sheetIn ищет и обложку, и развороты; sheetsOf ставит обложку первой', () => {
    expect(sheetIn(view, 'cover')?.id).toBe('cover')
    expect(sheetIn(view, 'b')?.id).toBe('b')
    expect(sheetIn(view, 'x')).toBeUndefined()
    expect(sheetsOf(view).map((s) => s.id)).toEqual(['cover', 'a', 'b'])
  })
  it('без обложки sheetIn(cover) пуст, sheetsOf — только развороты', () => {
    const none = { spreads: [sp('a')], cover: null }
    expect(sheetIn(none, 'cover')).toBeUndefined()
    expect(sheetsOf(none).map((s) => s.id)).toEqual(['a'])
    expect(sheetIn({ spreads: [sp('a')] }, 'cover')).toBeUndefined()
  })
})

describe('hasBorders (079: рамочки в шаблон не входят)', () => {
  const b = { kind: 'sa:fr-1' }
  const pl = (x: number, border?: object) => ({ x, y: 0, w: 100, h: 100, ...(border ? { border } : {}) })
  const sv = (places: ReturnType<typeof pl>[] | null, mid = 280) => ({ mid, places }) as never

  it('нет листа, «Авто», нет мест или рамочек — false', () => {
    expect(hasBorders(undefined)).toBe(false)
    expect(hasBorders(sv(null))).toBe(false)
    expect(hasBorders(sv([]))).toBe(false)
    expect(hasBorders(sv([pl(10), pl(300)]))).toBe(false)
  })
  it('одно место с рамочкой — true (один элемент)', () => {
    expect(hasBorders(sv([pl(300, b)]))).toBe(true)
  })
  it('страница: левая и правая смотрят только свои места', () => {
    const s = sv([pl(10), pl(300, b)])
    expect(hasBorders(s, 'left')).toBe(false)
    expect(hasBorders(s, 'right')).toBe(true)
    expect(hasBorders(s, 'spread')).toBe(true)
    expect(hasBorders(sv([pl(10, b)]), 'left')).toBe(true)
    expect(hasBorders(sv([pl(10, b)]), 'right')).toBe(false)
  })
  it('лист без шва (виньетка): область не режет', () => {
    expect(hasBorders(sv([pl(10, b)], 0), 'right')).toBe(true)
  })
  it('текст пояснения', () => {
    expect(BORDER_NOTE).toBe('Рамочки в шаблон не входят')
  })
})
