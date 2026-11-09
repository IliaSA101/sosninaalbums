import { describe, expect, it } from 'vitest'
import { NOT_LISTED_TEXT, availAtLabel, availEmpty, availHint, availMeta, availNote, availWhy, downloadAllLabel } from './packavail'
import { EMPTY_TEXT, metaLine, type PackAvail, type PackInfo } from './packs'

const MB = 1048576
const avail = (o: Partial<PackAvail> = {}): PackAvail => ({
  id: 'school',
  title: 'Школа',
  about: '',
  license: '',
  version: 1,
  bg: 12,
  sticker: 30,
  spread: 0,
  frame: 0,
  print: 0,
  bytes: 41 * MB,
  addonTitles: [],
  donor: '',
  why: '',
  removed: false,
  ...o,
})
const info = (o: Partial<PackInfo> = {}): PackInfo => ({
  id: 'b',
  title: 'Встроенные',
  about: '',
  author: '',
  license: '',
  source: '',
  origin: 'builtin',
  version: 1,
  bg: 1,
  sticker: 0,
  spread: 0,
  frame: 0,
  print: 0,
  bytes: 0,
  removable: false,
  ...o,
})

describe('packavail: строка «Можно скачать»', () => {
  it('availMeta: версия, состав и размер вместе с дополнениями — как у стоящих', () => {
    expect(availMeta(avail())).toBe('версия 1 · 12 фонов · 30 стикеров · 41,0 МБ')
    expect(availMeta(avail({ bg: 0, sticker: 0, version: 0, bytes: 0 }))).toBe('')
  })

  it('размер: стоящий набор и строка «Можно скачать» подписаны одним правилом (sizeLabel), одинаковые байты — одинаковая подпись', () => {
    for (const bytes of [463 * 1024, 463 * 1024 + 511, 464 * 1024 - 1, 5 * MB]) {
      expect(availMeta(avail({ bg: 1, sticker: 0, bytes }))).toBe(metaLine(info({ origin: 'net', bytes })))
    }
    // разные байты (zip каталога против файлов на диске, 137 QA) дают разные подписи — расхождение приходит от Go, не от округления
    expect(metaLine(info({ bytes: 463 * 1024 }))).toContain('463 КБ')
    expect(availMeta(avail({ bytes: 464 * 1024 }))).toContain('464 КБ')
  })

  it('availNote: лицензия, дополнение к, удалён вами, вместе с — через « · », пустое пропущено', () => {
    expect(availNote(avail())).toBe('')
    expect(availNote(avail({ license: 'CC0-1.0' }))).toBe('Лицензия: CC0-1.0')
    expect(availNote(avail({ donor: 'Свадьба' }))).toBe('дополнение к «Свадьба»')
    expect(availNote(avail({ removed: true }))).toBe('удалён вами')
    expect(availNote(avail({ addonTitles: ['Свадьба: макеты', 'Школа: рамки'] }))).toBe('вместе с: «Свадьба: макеты», «Школа: рамки»')
    expect(availNote(avail({ license: 'CC0-1.0', removed: true, addonTitles: ['A'] }))).toBe(
      'Лицензия: CC0-1.0 · удалён вами · вместе с: «A»',
    )
  })

  it('availNote: старый Go без addonTitles (null) не падает', () => {
    expect(availNote(avail({ addonTitles: null }))).toBe('')
  })

  it('availHint: обычная — «Скачать набор «…» (размер)»; серая — причина Go с заглавной', () => {
    expect(availHint(avail())).toBe('Скачать набор «Школа» (41,0 МБ)')
    expect(availHint(avail({ why: 'нужна версия Sosnina Albums 0.30.0 или новее' }))).toBe('Нужна версия Sosnina Albums 0.30.0 или новее')
    expect(availWhy(avail())).toBe('')
  })

  it('разметка в названии и описании — просто текст', () => {
    expect(availHint(avail({ title: '<b>X</b>' }))).toBe('Скачать набор «<b>X</b>» (41,0 МБ)')
  })

  it('availHint: удалённый вами и дополнение — обычная подсказка (кнопка активна)', () => {
    expect(availHint(avail({ removed: true }))).toBe('Скачать набор «Школа» (41,0 МБ)')
    expect(availHint(avail({ donor: 'Свадьба' }))).toBe('Скачать набор «Школа» (41,0 МБ)')
  })
})

describe('packavail: «Скачать все» и дата', () => {
  it('downloadAllLabel: N, размер; при N = 0 ссылки нет; при N = 1 без особого слова', () => {
    expect(downloadAllLabel({ count: 5, bytes: 380 * MB })).toBe('Скачать все (5, 380,0 МБ)')
    expect(downloadAllLabel({ count: 1, bytes: 41 * MB })).toBe('Скачать все (1, 41,0 МБ)')
    expect(downloadAllLabel({ count: 0, bytes: 0 })).toBe('')
    expect(downloadAllLabel(null)).toBe('')
    expect(downloadAllLabel(undefined)).toBe('')
  })

  it('availAtLabel: «по списку от ДД.ММ» по местному времени; 0 — пусто', () => {
    expect(availAtLabel(new Date(2026, 9, 27, 15, 0).getTime())).toBe('по списку от 27.10')
    expect(availAtLabel(new Date(2026, 0, 5, 1, 0).getTime())).toBe('по списку от 05.01')
    expect(availAtLabel(0)).toBe('')
    expect(availAtLabel(undefined)).toBe('')
  })
})

describe('packavail: вместо раздела', () => {
  it('до первой проверки — приглашение; список есть и пуст при одних встроенных — «Других наборов нет»', () => {
    expect(availEmpty([info()], 0, 0)).toBe(NOT_LISTED_TEXT)
    expect(availEmpty([info()], 0, 3)).toBe(NOT_LISTED_TEXT)
    expect(availEmpty([info()], 5, 0)).toBe(EMPTY_TEXT)
    expect(availEmpty([info(), info({ id: 'n', origin: 'net' })], 5, 0)).toBe('')
    expect(availEmpty([info()], 5, 2)).toBe('')
    expect(NOT_LISTED_TEXT).toBe('Наборы из интернета появятся здесь после «Обновить библиотеку»')
  })
})
