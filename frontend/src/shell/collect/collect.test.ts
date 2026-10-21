import { beforeEach, describe, expect, it } from 'vitest'
import {
  extraLine,
  folderHint,
  folderStub,
  FONTS_HINT,
  fontsRestrictedLine,
  infoStale,
  loadAll,
  missingLine,
  num,
  optionHint,
  PENDING_HINT,
  PENDING_TOTAL,
  SCANNING_LINE,
  summaryOf,
  phaseOf,
  progressLine,
  progressPercent,
  saveAll,
  sizePair,
  sizeText,
  totalLine,
} from './collect'

const GB = 1 << 30
const MB = 1 << 20

describe('sizeText (как collect.SizeText в Go)', () => {
  it('ГБ, МБ, мелочь', () => {
    expect(sizeText(6.2 * GB)).toBe('6,2 ГБ')
    expect(sizeText(4.5 * MB)).toBe('4,5 МБ')
    expect(sizeText(120 * MB)).toBe('120 МБ')
    expect(sizeText(1000)).toBe('< 0,1 МБ')
    expect(sizeText(0)).toBe('< 0,1 МБ')
  })
})

describe('sizePair', () => {
  it('одна единица по общему объёму', () => {
    expect(sizePair(2.4 * GB, 6.2 * GB)).toBe('2,4 из 6,2 ГБ')
    expect(sizePair(0, 6.2 * GB)).toBe('0,0 из 6,2 ГБ')
    expect(sizePair(40 * MB, 120 * MB)).toBe('40 из 120 МБ')
    expect(sizePair(1.5 * MB, 4.5 * MB)).toBe('1,5 из 4,5 МБ')
  })
})

describe('строки сводки', () => {
  it('число с неразрывным пробелом разрядов', () => {
    expect(num(312)).toBe('312')
    expect(num(1240)).toBe('1 240')
    expect(num(1234567)).toBe('1 234 567')
  })
  it('подсказка варианта: фото и объём; нет сводки — пусто', () => {
    expect(optionHint(312, 6.2 * GB)).toBe('(312 фото, 6,2 ГБ)')
    expect(optionHint(1240, 24.8 * GB)).toBe('(1 240 фото, 24,8 ГБ)')
    expect(optionHint(null, 0)).toBe('')
  })
  it('«Ещё: …» — только то, что есть', () => {
    expect(extraLine({ own: 3, guides: 2 })).toBe('Ещё: свои фоны и стикеры — 3, разметка типографии — 2.')
    expect(extraLine({ own: 3, guides: 0 })).toBe('Ещё: свои фоны и стикеры — 3.')
    expect(extraLine({ own: 0, guides: 2 })).toBe('Ещё: разметка типографии — 2.')
    expect(extraLine({ own: 0, guides: 0 })).toBe('')
  })
  it('пропавшие: склонения и единственное число', () => {
    expect(missingLine({ missing: 4, missingGuides: 0 })).toBe('4 фото не найдены — в папку не попадут.')
    expect(missingLine({ missing: 1, missingGuides: 0 })).toBe('1 фото не найдено — в папку не попадёт.')
    expect(missingLine({ missing: 0, missingGuides: 1 })).toBe('1 разметка типографии не найдена — в папку не попадёт.')
    expect(missingLine({ missing: 0, missingGuides: 2 })).toBe('2 разметки типографии не найдены — в папку не попадут.')
    expect(missingLine({ missing: 0, missingGuides: 5 })).toBe('5 разметок типографии не найдены — в папку не попадут.')
    expect(missingLine({ missing: 4, missingGuides: 2 })).toBe('4 фото и 2 разметки типографии не найдены — в папку не попадут.')
    expect(missingLine({ missing: 0, missingGuides: 0 })).toBe('')
  })
  it('заглушка папки до ответа Go: имя файла, черновик — «Альбом»', () => {
    expect(folderStub(true, 'Иванов')).toBe('Иванов')
    expect(folderStub(false, 'Черновик')).toBe('Альбом')
    expect(folderHint('Иванов')).toBe(
      'В выбранной папке появится папка «Иванов»: альбом, «Фото», «Свои фоны и стикеры», «Разметка». Открытый альбом не меняется.',
    )
  })
  it('итог: объём или «только файл альбома» (пустой альбом)', () => {
    expect(totalLine({ bytes: 6.2 * GB })).toBe('Скопируется 6,2 ГБ.')
    expect(totalLine({ bytes: 0 })).toBe('В папку запишется только файл альбома.')
    expect(totalLine(null)).toBe('')
  })
})

describe('ход копирования', () => {
  it('строка хода как в макете', () => {
    expect(progressLine({ doneFiles: 120, totalFiles: 312, doneBytes: 2.4 * GB, totalBytes: 6.2 * GB })).toBe(
      'Скопировано 120 из 312 · 2,4 из 6,2 ГБ',
    )
  })
  it('до первого события — «Готовим список»', () => {
    expect(progressLine({ doneFiles: 0, totalFiles: 0, doneBytes: 0, totalBytes: 0 })).toBe('Готовим список файлов…')
  })
  it('доля по байтам; нет объёма — по файлам; ничего — null', () => {
    expect(progressPercent({ doneFiles: 1, totalFiles: 4, doneBytes: 50, totalBytes: 200 })).toBe(25)
    expect(progressPercent({ doneFiles: 1, totalFiles: 4, doneBytes: 0, totalBytes: 0 })).toBe(25)
    expect(progressPercent({ doneFiles: 0, totalFiles: 0, doneBytes: 0, totalBytes: 0 })).toBeNull()
    expect(progressPercent({ doneFiles: 9, totalFiles: 4, doneBytes: 900, totalBytes: 200 })).toBe(100)
  })
})

describe('состояния окна', () => {
  it('закрыто → настройки → копирование', () => {
    expect(phaseOf(false, false)).toBe('closed')
    expect(phaseOf(true, false)).toBe('setup')
    expect(phaseOf(true, true)).toBe('running')
  })
  it('ход без окна не показывается', () => {
    expect(phaseOf(false, true)).toBe('closed')
  })
  it('старый ответ сводки отсекается номером запроса', () => {
    expect(infoStale(1, 2)).toBe(true)
    expect(infoStale(2, 2)).toBe(false)
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

describe('выбор запоминается', () => {
  beforeEach(() => useStorage())
  it('по умолчанию — только фото альбома', () => {
    expect(loadAll()).toBe(false)
  })
  it('сохранённый выбор читается', () => {
    saveAll(true)
    expect(loadAll()).toBe(true)
    saveAll(false)
    expect(loadAll()).toBe(false)
  })
  it('без хранилища не падает', () => {
    useStorage(true)
    expect(() => saveAll(true)).not.toThrow()
    expect(loadAll()).toBe(false)
  })
})

describe('сводка до ответа Go: места заняты', () => {
  const i = {
    photos: 312,
    extra: 928,
    own: 0,
    guides: 0,
    missing: 0,
    missingGuides: 0,
    bytes: 6 * GB,
    scanning: false,
    folderName: 'Иванов',
  }
  it('до ответа — заглушки чисел и строка «Скопируется …»', () => {
    const s = summaryOf(null, null, null)
    expect(s.albumHint).toBe(PENDING_HINT)
    expect(s.fullHint).toBe(PENDING_HINT)
    expect(s.total).toBe(PENDING_TOTAL)
    expect(s.pending).toBe(true)
  })
  it('после ответа — настоящие числа, заглушек нет', () => {
    const s = summaryOf(i, i, i)
    expect(s.albumHint).toBe('(312 фото, 6,0 ГБ)')
    expect(s.fullHint).toBe('(' + num(1240) + ' фото, 6,0 ГБ)')
    expect(s.total).toBe('Скопируется 6,0 ГБ.')
    expect(s.pending).toBe(false)
  })
  it('идущее чтение папок дописывается к итогу', () => {
    expect(summaryOf(i, i, { ...i, scanning: true }).total).toContain(SCANNING_LINE)
  })
})

describe('свои шрифты в сборе (082)', () => {
  it('extraLine: шрифты — между фонами и разметкой; одни шрифты; без шрифтов как раньше', () => {
    expect(extraLine({ own: 3, guides: 0, fonts: 2 })).toBe('Ещё: свои фоны и стикеры — 3, шрифты — 2.')
    expect(extraLine({ own: 3, guides: 2, fonts: 2 })).toBe('Ещё: свои фоны и стикеры — 3, шрифты — 2, разметка типографии — 2.')
    expect(extraLine({ own: 0, guides: 0, fonts: 1 })).toBe('Ещё: шрифты — 1.')
    expect(extraLine({ own: 0, guides: 0, fonts: 0 })).toBe('')
    expect(extraLine({ own: 0, guides: 0 })).toBe('')
    expect(FONTS_HINT).toContain('лицензию')
  })
  it('fontsRestrictedLine: 1 имя, 2 имени, пусто', () => {
    expect(fontsRestrictedLine(['Lato'])).toBe('Шрифт „Lato“ не переносится по лицензии — загрузите его на другом компьютере сами')
    expect(fontsRestrictedLine(['Lato', 'Bodoni Moda'])).toBe(
      'Шрифты „Lato“, „Bodoni Moda“ не переносятся по лицензии — загрузите их на другом компьютере сами',
    )
    expect(fontsRestrictedLine([])).toBe('')
  })
  it('missingLine: шрифты — отдельной фразой, склонение и согласование', () => {
    const base = { missing: 0, missingGuides: 0 }
    expect(missingLine({ ...base, missingFonts: 1 })).toBe('1 шрифта нет на этом компьютере — в папку не попадёт.')
    expect(missingLine({ ...base, missingFonts: 2 })).toBe('2 шрифта нет на этом компьютере — в папку не попадут.')
    expect(missingLine({ ...base, missingFonts: 5 })).toBe('5 шрифтов нет на этом компьютере — в папку не попадут.')
    expect(missingLine({ missing: 4, missingGuides: 0, missingFonts: 2 })).toBe(
      '4 фото не найдены — в папку не попадут. 2 шрифта нет на этом компьютере — в папку не попадут.',
    )
    expect(missingLine({ ...base, missingFonts: 0 })).toBe('')
    expect(missingLine(base)).toBe('')
  })
})
