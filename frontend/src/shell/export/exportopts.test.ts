import { beforeEach, describe, expect, it } from 'vitest'
import {
  DEFAULT_PREFS,
  buildOptions,
  coverAvailable,
  estimateBytes,
  exportDoneText,
  exportWarnings,
  formatSize,
  hasFix,
  doneFolder,
  canExport,
  planStale,
  loadPrefs,
  memoLine,
  sampleName,
  savePrefs,
  sizeSummary,
} from './exportopts'

const album = { pageW: 300, pageH: 300, dpi: 300 }
const MB = 1024 * 1024

describe('estimateBytes', () => {
  it('разворот 2×300 мм, 300 dpi, 95 — пиксели × 0,45', () => {
    // 3543 px страница: 7086 × 3543
    expect(estimateBytes(album, 300, 95, false)).toBeCloseTo(7086 * 3543 * 0.45, 0)
  })
  it('страница вдвое меньше разворота', () => {
    expect(estimateBytes(album, 300, 95, true)).toBeCloseTo(estimateBytes(album, 300, 95, false) / 2, -3)
  })
  it('dpi 0 берёт из альбома', () => {
    expect(estimateBytes(album, 0, 80, false)).toBe(estimateBytes(album, 300, 80, false))
    expect(estimateBytes({ ...album, dpi: 150 }, 0, 80, false)).toBe(estimateBytes(album, 150, 80, false))
  })
  it('качество монотонно, dpi квадратично', () => {
    const q = ([100, 95, 80] as const).map((v) => estimateBytes(album, 300, v, false))
    expect(q[0]).toBeGreaterThan(q[1])
    expect(q[1]).toBeGreaterThan(q[2])
    expect(estimateBytes(album, 150, 95, false) / estimateBytes(album, 300, 95, false)).toBeCloseTo(0.25, 2)
  })
})

describe('formatSize', () => {
  it('десятые до 10 МБ, потом целые', () => {
    expect(formatSize(2.5 * MB)).toBe('≈ 2,5 МБ')
    expect(formatSize(24.6 * MB)).toBe('≈ 25 МБ')
    expect(formatSize(1000)).toBe('< 0,1 МБ')
  })
})

describe('coverAvailable', () => {
  it('нужна обложка и не лист-виньетка', () => {
    const v = (o: object) => o as Parameters<typeof coverAvailable>[0]
    expect(coverAvailable(v({ settings: {} }))).toBe(false)
    expect(coverAvailable(v({ cover: {}, settings: {} }))).toBe(true)
    expect(coverAvailable(v({ cover: {}, settings: { sheet: true } }))).toBe(false)
  })
})

describe('buildOptions', () => {
  const p = { quality: 80 as const, dpi: 150, pages: true, cover: true, coverOnly: false, fsfera: false }
  it('«Все» — пустой диапазон', () => {
    expect(buildOptions('all', 3, '1-2', p)).toEqual({
      range: '',
      quality: 80,
      dpi: 150,
      pages: true,
      cover: false,
      coverOnly: false,
      fsfera: false,
    })
  })
  it('«Текущий» — номер с 1', () => {
    expect(buildOptions('current', 4, '', DEFAULT_PREFS)).toEqual({
      range: '4',
      quality: 95,
      dpi: 0,
      pages: false,
      cover: false,
      coverOnly: false,
      fsfera: false,
    })
  })
  it('«Диапазон» — текст без пробелов по краям', () => {
    expect(buildOptions('range', 1, '  3-5, 7п ', p).range).toBe('3-5, 7п')
  })
  it('с обложкой — cover по выбору, без обложки флаги всегда false', () => {
    expect(buildOptions('all', 1, '', p, true)).toMatchObject({ cover: true, coverOnly: false })
    expect(buildOptions('all', 1, '', { ...p, cover: false }, true)).toMatchObject({ cover: false, coverOnly: false })
    expect(buildOptions('all', 1, '', { ...p, cover: true, coverOnly: true }, false)).toMatchObject({ cover: false, coverOnly: false })
  })
  it('«только обложка» включает обложку сам и гасит диапазон', () => {
    const o = buildOptions('range', 2, '3-5', { ...p, cover: false, coverOnly: true }, true)
    expect(o).toEqual({ range: '', quality: 80, dpi: 150, pages: true, cover: true, coverOnly: true, fsfera: false })
  })
  it('«Текущий» на обложке — только обложка, а не разворот с номером до неё', () => {
    const o = buildOptions('current', 4, '', p, true, true)
    expect(o).toEqual({ range: '', quality: 80, dpi: 150, pages: true, cover: true, coverOnly: true, fsfera: false })
    expect(buildOptions('current', 4, '', p, true, false).range).toBe('4')
    expect(buildOptions('all', 4, '', p, true, true).coverOnly).toBe(false)
  })
})

// Минимальное хранилище вместо localStorage (среда тестов — node).
function useStorage(broken = false) {
  const m = new Map<string, string>()
  const guard = () => {
    if (broken) throw new Error('нет хранилища')
  }
  const s = {
    getItem: (k: string) => (guard(), m.get(k) ?? null),
    setItem: (k: string, v: string) => {
      guard()
      m.set(k, v)
    },
  }
  Object.defineProperty(globalThis, 'localStorage', { value: s, configurable: true, writable: true })
}

describe('prefs в localStorage', () => {
  beforeEach(() => useStorage())
  it('по умолчанию 95, из альбома, разворотами', () => {
    expect(loadPrefs()).toEqual({ quality: 95, dpi: 0, pages: false, cover: true, coverOnly: false, fsfera: false })
  })
  it('сохранённое читается', () => {
    savePrefs({ quality: 100, dpi: 150, pages: true, cover: false, coverOnly: true, fsfera: true })
    expect(loadPrefs()).toEqual({ quality: 100, dpi: 150, pages: true, cover: false, coverOnly: true, fsfera: true })
  })
  it('порченое и чужое отбрасывается', () => {
    localStorage.setItem('exportOpts', JSON.stringify({ quality: 50, dpi: 5, pages: 'да', cover: 1, coverOnly: 'да', fsfera: 1 }))
    expect(loadPrefs()).toEqual(DEFAULT_PREFS)
    localStorage.setItem('exportOpts', '{не json')
    expect(loadPrefs()).toEqual(DEFAULT_PREFS)
  })
  it('без хранилища не падает', () => {
    useStorage(true)
    expect(() => savePrefs(DEFAULT_PREFS)).not.toThrow()
    expect(loadPrefs()).toEqual(DEFAULT_PREFS)
  })
})

describe('sampleName', () => {
  it('две цифры, как у Go', () => {
    expect(sampleName(7, 12, false)).toBe('(07.jpg)')
    expect(sampleName(3, 12, true)).toBe('(03-1.jpg, 03-2.jpg)')
  })
  it('при 100+ разворотах три цифры', () => {
    expect(sampleName(7, 120, false)).toBe('(007.jpg)')
    expect(sampleName(120, 120, true)).toBe('(120-1.jpg, 120-2.jpg)')
  })
  it('пустой альбом — номер 01', () => {
    expect(sampleName(0, 0, false)).toBe('(01.jpg)')
  })
})

describe('сводка перед выгрузкой', () => {
  it('нечего сказать — строки нет', () => {
    expect(exportWarnings({ emptyPlaces: 0, lowDPI: 0 }, 0)).toBe('')
  })
  it('только то, что есть', () => {
    expect(exportWarnings({ emptyPlaces: 2, lowDPI: 0 }, 0)).toBe('Перед выгрузкой: пустые места: 2')
    expect(exportWarnings({ emptyPlaces: 0, lowDPI: 3 }, 1)).toBe('Перед выгрузкой: мало разрешения: 3 фото, не найдено фото: 1')
    expect(exportWarnings({ emptyPlaces: 1, lowDPI: 4 }, 5)).toBe(
      'Перед выгрузкой: пустые места: 1, мало разрешения: 4 фото, не найдено фото: 5',
    )
  })
})

describe('итог по объёму', () => {
  const one = estimateBytes(album, 300, 95, false)
  const base = { scope: 'all' as const, spreads: 4, units: null, cover: false, only: false }
  it('«Все» — на разворот и всего', () => {
    expect(sizeSummary(album, 300, 95, false, 'разворот', base)).toBe(`${formatSize(one)} на разворот, всего ${formatSize(4 * one)}`)
  })
  it('обложка — как один разворот', () => {
    expect(sizeSummary(album, 300, 95, false, 'разворот', { ...base, cover: true })).toContain(`всего ${formatSize(5 * one)}`)
    expect(sizeSummary(album, 300, 95, false, 'разворот', { ...base, cover: true, only: true })).toBe(`${formatSize(one)} на разворот`)
  })
  it('«Текущий» без обложки — один файл, без «всего»', () => {
    expect(sizeSummary(album, 300, 95, false, 'разворот', { ...base, scope: 'current' })).toBe(`${formatSize(one)} на разворот`)
  })
  it('диапазон: разворот целиком и страница', () => {
    const units: [number, 0 | 1 | 2][] = [
      [3, 0],
      [7, 2],
    ]
    const r = sizeSummary(album, 300, 95, false, 'разворот', { ...base, scope: 'range', units })
    expect(r).toContain(`всего ${formatSize(1.5 * one)}`)
  })
  it('неверный диапазон — без итога', () => {
    expect(sizeSummary(album, 300, 95, false, 'разворот', { ...base, scope: 'range' })).toBe(`${formatSize(one)} на разворот`)
  })
  it('по страницам — размер страницы', () => {
    expect(sizeSummary(album, 300, 95, true, 'страницу', base)).toBe(`${formatSize(one / 2)} на страницу, всего ${formatSize(4 * one)}`)
  })
})

describe('для Фотосферы Pro (047)', () => {
  const p = { quality: 80 as const, dpi: 150, pages: true, cover: false, coverOnly: true, fsfera: true }
  it('buildOptions: прочие выборы игнорируются, обложка — если она есть', () => {
    const want = { range: '', quality: 80, dpi: 300, pages: false, coverOnly: false, fsfera: true }
    expect(buildOptions('range', 2, '3-5', p, true, true, true)).toEqual({ ...want, cover: true })
    expect(buildOptions('range', 2, '3-5', p, false, false, true)).toEqual({ ...want, cover: false })
  })
  it('без флага — fsfera false, как раньше', () => {
    expect(buildOptions('all', 1, '', p, true).fsfera).toBe(false)
  })
  it('sizeSummary: все развороты и обложка при 300 dpi, как ни выбрано в окне', () => {
    const one = estimateBytes(album, 300, 95, false)
    const w = { scope: 'current' as const, spreads: 4, units: null, cover: true, only: true, fsfera: true }
    expect(sizeSummary(album, 150, 95, true, 'разворот', w)).toBe(`${formatSize(one)} на разворот, всего ${formatSize(5 * one)}`)
  })
  it('памятка: первые три строки', () => {
    const memo = [
      'Фотокнига 30×30, обложка «Фото»',
      'Страницы: Тонкие',
      'Разворотов: 20 (корешок 14 мм)',
      'Папка: Иванов — Петрова — 00.jpg и 01…20.jpg',
    ]
    expect(memoLine(memo)).toBe('Фотокнига 30×30, обложка «Фото» · Страницы: Тонкие · Разворотов: 20 (корешок 14 мм)')
  })
  it('имя папки в «Готово» — из плана, « — » в имени не режется', () => {
    expect(doneFolder('Иванов — Петрова')).toBe('Иванов — Петрова')
    expect(doneFolder('')).toBe('книги')
  })
  it('«Исправить» — у строки с номером fixAt, не по тексту', () => {
    const plan = { coverFix: true, fixAt: 1 }
    expect([0, 1, 2].map((i) => hasFix(plan, i))).toEqual([false, true, false])
    expect(hasFix({ coverFix: true, fixAt: -1 }, 0)).toBe(false)
    expect(hasFix({ coverFix: false, fixAt: 0 }, 0)).toBe(false)
  })
  it('допуск «Выгрузить»: занято, диапазон, блок, план не пришёл', () => {
    const ok = { format: '30×30', block: [] as string[] }
    const blocked = { format: '30×30', block: ['Разворотов 60'] }
    const base = { busy: false, bad: false, fsPref: false, plan: null }
    expect(canExport(base)).toBe(true)
    expect(canExport({ ...base, busy: true })).toBe(false)
    expect(canExport({ ...base, bad: true })).toBe(false)
    // без флажка блок плана не мешает обычному экспорту
    expect(canExport({ ...base, plan: blocked })).toBe(true)
    expect(canExport({ ...base, fsPref: true, plan: ok })).toBe(true)
    expect(canExport({ ...base, fsPref: true, plan: blocked })).toBe(false)
    // Фотосфера выбрана, плана ещё нет (только что открыли окно) — закрыто
    expect(canExport({ ...base, fsPref: true, plan: null })).toBe(false)
    // формат не Фотосферы (флажка нет): режим не включён, план не мешает
    expect(canExport({ ...base, fsPref: true, plan: { format: '', block: ['x'] } })).toBe(true)
  })
  it('запоздавший ответ плана отсекается по номеру запроса', () => {
    expect(planStale(2, 2)).toBe(false)
    expect(planStale(1, 2)).toBe(true)
  })
})

describe('тост выгрузки', () => {
  it('число файлов из последнего прогресса', () => {
    expect(exportDoneText(12)).toBe('Выгружено файлов: 12')
  })
  it('нет числа — «Выгрузка готова»', () => {
    expect(exportDoneText(undefined)).toBe('Выгрузка готова')
    expect(exportDoneText(0)).toBe('Выгрузка готова')
  })
  it('сводка цвета Go — через точку', () => {
    expect(exportDoneText(3, 'Пересчитано из Adobe RGB в sRGB: 2 фото')).toBe(
      'Выгружено файлов: 3. Пересчитано из Adobe RGB в sRGB: 2 фото',
    )
  })
})
