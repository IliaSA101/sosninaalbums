import { beforeEach, describe, expect, it } from 'vitest'
import {
  DEFAULT_PREFS,
  buildOptions,
  coverAvailable,
  estimateBytes,
  exportDoneText,
  exportWarnings,
  limitWarn,
  formatSize,
  hasFix,
  activePlan,
  printHint,
  printQuality,
  togglePrint,
  canExport,
  planStale,
  loadPrefs,
  memoLine,
  sampleName,
  savePrefs,
  sizeSummary,
} from './exportopts'
import type { PrintPlan } from '../../base/core/types'

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
  const p = { quality: 80 as const, dpi: 150, pages: true, cover: true, coverOnly: false, print: '' as const }
  it('«Все» — пустой диапазон', () => {
    expect(buildOptions('all', 3, '1-2', p)).toEqual({
      range: '',
      quality: 80,
      dpi: 150,
      pages: true,
      cover: false,
      coverOnly: false,
      print: '',
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
      print: '',
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
    expect(o).toEqual({ range: '', quality: 80, dpi: 150, pages: true, cover: true, coverOnly: true, print: '' })
  })
  it('«Текущий» на обложке — только обложка, а не разворот с номером до неё', () => {
    const o = buildOptions('current', 4, '', p, true, true)
    expect(o).toEqual({ range: '', quality: 80, dpi: 150, pages: true, cover: true, coverOnly: true, print: '' })
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
    expect(loadPrefs()).toEqual({ quality: 95, dpi: 0, pages: false, cover: true, coverOnly: false, print: '' })
  })
  it('сохранённое читается', () => {
    savePrefs({ quality: 100, dpi: 150, pages: true, cover: false, coverOnly: true, print: 'fabrika' })
    expect(loadPrefs()).toEqual({ quality: 100, dpi: 150, pages: true, cover: false, coverOnly: true, print: 'fabrika' })
  })
  it('порченое и чужое отбрасывается', () => {
    localStorage.setItem(
      'exportOpts',
      JSON.stringify({ quality: 50, dpi: 5, pages: 'да', cover: 1, coverOnly: 'да', print: 'xerox', fsfera: 1 }),
    )
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

describe('предел разворотов в окне (085)', () => {
  const over = {
    pageLimit: {
      who: 'Нетпринт',
      range: 'до 30 разворотов',
      count: 31,
      min: 8,
      max: 30,
      warn: 'Разворотов 31 — у Нетпринта («Royal») не больше 30',
    },
  }
  it('обычная выгрузка — строка предупреждения Go', () => {
    expect(limitWarn(over, false)).toBe('Разворотов 31 — у Нетпринта («Royal») не больше 30')
  })
  it('для Фотосферы Pro — нет: список плана от Go', () => {
    expect(limitWarn(over, true)).toBe('')
  })
  it('в пределах или без типографии — пусто', () => {
    expect(limitWarn({ pageLimit: { ...over.pageLimit, count: 20, warn: '' } }, false)).toBe('')
    expect(limitWarn({}, false)).toBe('')
  })
  it('«Выгрузить» не зависит от предела: при непустом warn допуск остаётся', () => {
    expect(limitWarn(over, false)).not.toBe('')
    expect(canExport({ busy: false, bad: false, printPref: '', plans: null })).toBe(true)
    expect(exportWarnings({ emptyPlaces: 0, lowDPI: 0 }, 0)).toBe('') // строка «Перед выгрузкой» тоже не меняется
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

const plan = (o: Partial<PrintPlan> = {}): PrintPlan => ({
  kind: 'fsfera',
  title: 'Для Фотосферы Pro',
  format: '30×30',
  pages: 'Тонкие',
  spreads: 20,
  spine: 14,
  min: 0,
  max: 40,
  folder: 'Иванов',
  coverName: '00.jpg',
  quality: 0,
  block: [],
  coverFix: false,
  fixAt: -1,
  warn: [],
  memo: [],
  done: 'Готово для Фотосферы',
  site: '',
  siteButton: 'Открыть Фотосферу',
  ...o,
})
const fab = plan({ kind: 'fabrika', title: 'Для Фабрики Фотокниги', quality: 100, coverName: '1-00.jpg' })

describe('для типографии: Фотосфера Pro (047), Фабрика Фотокниги (081)', () => {
  const p = { quality: 80 as const, dpi: 150, pages: true, cover: false, coverOnly: true, print: 'fsfera' as const }
  it('buildOptions: прочие выборы игнорируются, обложка — если она есть', () => {
    const want = { range: '', quality: 80, dpi: 300, pages: false, coverOnly: false, print: 'fsfera' }
    expect(buildOptions('range', 2, '3-5', p, true, true, plan())).toEqual({ ...want, cover: true })
    expect(buildOptions('range', 2, '3-5', p, false, false, plan())).toEqual({ ...want, cover: false })
  })
  it('buildOptions fabrika: качество плана 100 вместо выбора окна', () => {
    expect(buildOptions('all', 1, '', p, true, false, fab)).toEqual({
      range: '',
      quality: 100,
      dpi: 300,
      pages: false,
      cover: true,
      coverOnly: false,
      print: 'fabrika',
    })
  })
  it('без плана — print пусто, как обычная выгрузка', () => {
    expect(buildOptions('all', 1, '', p, true).print).toBe('')
  })
  it('sizeSummary: все развороты и обложка при 300 dpi, как ни выбрано в окне', () => {
    const one = estimateBytes(album, 300, 95, false)
    const w = { scope: 'current' as const, spreads: 4, units: null, cover: true, only: true, print: true }
    expect(sizeSummary(album, 150, 95, true, 'разворот', w)).toBe(`${formatSize(one)} на разворот, всего ${formatSize(5 * one)}`)
  })
  it('printQuality: у плана со своим качеством оно, иначе выбор окна', () => {
    expect(printQuality(fab, 80)).toBe(100)
    expect(printQuality(plan(), 80)).toBe(80)
    expect(printQuality(null, 95)).toBe(95)
  })
  it('printHint: папка, 300 dpi, качество плана и обложка', () => {
    expect(printHint(plan(), true)).toBe('(папка «Иванов», 300 dpi, с обложкой)')
    expect(printHint(fab, false)).toBe('(папка «Иванов», 300 dpi, качество 100)')
    expect(printHint(fab, true)).toBe('(папка «Иванов», 300 dpi, качество 100, с обложкой)')
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
  it('«Исправить» — у строки с номером fixAt, не по тексту', () => {
    const fix = { coverFix: true, fixAt: 1 }
    expect([0, 1, 2].map((i) => hasFix(fix, i))).toEqual([false, true, false])
    expect(hasFix({ coverFix: true, fixAt: -1 }, 0)).toBe(false)
    expect(hasFix({ coverFix: false, fixAt: 0 }, 0)).toBe(false)
  })
  it('activePlan: берёт план выбранной типографии, без формата или не выбрана — нет режима', () => {
    expect(activePlan([plan(), fab], 'fabrika')).toBe(fab)
    expect(activePlan([plan(), fab], 'fsfera')).toEqual(plan())
    expect(activePlan([plan(), fab], '')).toBeNull()
    expect(activePlan([plan()], 'fabrika')).toBeNull() // у страницы Фабрики нет: флажка нет, режим выключен
    expect(activePlan([plan({ format: '' })], 'fsfera')).toBeNull()
    expect(activePlan(null, 'fsfera')).toBeNull()
  })
  it('two plans one checked: включён не больше одного, повторный щелчок выключает', () => {
    let pref: '' | 'fsfera' | 'fabrika' = ''
    pref = togglePrint(pref, 'fsfera', true)
    expect(pref).toBe('fsfera')
    pref = togglePrint(pref, 'fabrika', true) // второй флажок переключает
    expect(pref).toBe('fabrika')
    expect(activePlan([plan(), fab], pref)).toBe(fab)
    pref = togglePrint(pref, 'fsfera', false) // снятие чужого флажка выбор не меняет
    expect(pref).toBe('fabrika')
    pref = togglePrint(pref, 'fabrika', false)
    expect(pref).toBe('')
  })
  it('допуск «Выгрузить»: занято, диапазон, блок, планы не пришли', () => {
    const ok = plan()
    const blocked = plan({ block: ['Разворотов 60'] })
    const base = { busy: false, bad: false, printPref: '' as '' | 'fsfera' | 'fabrika', plans: null as PrintPlan[] | null }
    expect(canExport(base)).toBe(true)
    expect(canExport({ ...base, busy: true })).toBe(false)
    expect(canExport({ ...base, bad: true })).toBe(false)
    // без флажка блок плана не мешает обычному экспорту
    expect(canExport({ ...base, plans: [blocked] })).toBe(true)
    expect(canExport({ ...base, printPref: 'fsfera', plans: [ok] })).toBe(true)
    expect(canExport({ ...base, printPref: 'fsfera', plans: [blocked] })).toBe(false)
    // блок Фабрики не мешает, пока включена Фотосфера, и наоборот
    expect(canExport({ ...base, printPref: 'fsfera', plans: [ok, plan({ kind: 'fabrika', block: ['x'] })] })).toBe(true)
  })
  it('canExport print pref without plan: типография выбрана, а планов ещё нет — закрыто; страница не её — открыто', () => {
    const base = { busy: false, bad: false, printPref: 'fabrika' as '' | 'fsfera' | 'fabrika' }
    expect(canExport({ ...base, plans: null })).toBe(false) // только открыли окно
    expect(canExport({ ...base, plans: [] })).toBe(true) // страница без типографии: режим выключен
    expect(canExport({ ...base, plans: [plan()] })).toBe(true) // у страницы только Фотосфера
    expect(canExport({ ...base, plans: [plan({ kind: 'fabrika', format: '' })] })).toBe(true)
    expect(canExport({ ...base, bad: true, plans: [] })).toBe(false) // неверный диапазон мешает обычной выгрузке
  })
  it('запоздавший ответ плана отсекается по номеру запроса', () => {
    expect(planStale(2, 2)).toBe(false)
    expect(planStale(1, 2)).toBe(true)
  })
})

describe('prefs печати', () => {
  beforeEach(() => useStorage())
  it('loadPrefs migrates fsfera: прежний флажок fsfera: true становится print «fsfera»', () => {
    localStorage.setItem('exportOpts', JSON.stringify({ quality: 80, fsfera: true }))
    expect(loadPrefs()).toMatchObject({ quality: 80, print: 'fsfera' })
    localStorage.setItem('exportOpts', JSON.stringify({ fsfera: false }))
    expect(loadPrefs().print).toBe('')
  })
  it('print главнее старого fsfera; неизвестное значение — выключено', () => {
    localStorage.setItem('exportOpts', JSON.stringify({ print: 'fabrika', fsfera: true }))
    expect(loadPrefs().print).toBe('fabrika')
    localStorage.setItem('exportOpts', JSON.stringify({ print: 'xerox', fsfera: true }))
    expect(loadPrefs().print).toBe('')
  })
  it('в сохранённое старое поле fsfera не пишется', () => {
    savePrefs({ ...DEFAULT_PREFS, print: 'fsfera' })
    expect(JSON.parse(localStorage.getItem('exportOpts')!)).not.toHaveProperty('fsfera')
  })
})

describe('строка предела в режиме печати', () => {
  const v = { pageLimit: { warn: 'Разворотов 21 — у Фабрики Фотокниги («Лайфлат, картон») не больше 20' } } as never
  it('limitWarn hidden in print mode: любая типография прячет строку 085', () => {
    expect(limitWarn(v, true)).toBe('')
    expect(limitWarn(v, false)).toContain('Фабрики Фотокниги')
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
