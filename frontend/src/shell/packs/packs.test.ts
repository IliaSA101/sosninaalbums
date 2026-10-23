import { describe, expect, it } from 'vitest'
import {
  addedText,
  cancelled,
  contentsLabel,
  creditLine,
  metaLine,
  notAddedText,
  onlyBuiltin,
  removeNote,
  removeQuestion,
  replaceQuestion,
  skippedLabel,
  sortPacks,
  type PackAdd,
  type PackInfo,
} from './packs'

const pack = (o: Partial<PackInfo> = {}): PackInfo => ({
  id: 'ivory',
  title: 'Айвори',
  about: '',
  author: 'Марина',
  license: 'CC0-1.0',
  source: 'example.org/ivory',
  origin: 'file',
  version: 3,
  bg: 12,
  sticker: 30,
  spread: 8,
  bytes: 41 * 1048576,
  removable: true,
  ...o,
})
const add = (o: Partial<PackAdd> = {}): PackAdd => ({ pack: pack(), ask: '', fromVersion: 0, token: '', skipped: [], ...o })

describe('строки списка', () => {
  it('состав — только то, что есть, со склонением', () => {
    expect(contentsLabel(pack())).toBe('12 фонов, 30 стикеров, 8 разворотов')
    expect(contentsLabel(pack({ bg: 1, sticker: 0, spread: 22 }))).toBe('1 фон, 22 разворота')
    expect(contentsLabel(pack({ bg: 0, sticker: 0, spread: 0 }))).toBe('')
  })

  it('метаданные: версия, состав, размер; у встроенного версии и размера нет', () => {
    expect(metaLine(pack())).toBe('версия 3 · 12 фонов · 30 стикеров · 8 разворотов · 41,0 МБ')
    const builtin = pack({ origin: 'builtin', version: 1, bytes: 0, bg: 30, sticker: 45, spread: 12 })
    expect(metaLine(builtin)).toBe('30 фонов · 45 стикеров · 12 разворотов')
  })

  it('автор, лицензия, источник и «откуда» — одной строкой; пустое пропускается', () => {
    expect(creditLine(pack())).toBe('Автор: Марина · Лицензия: CC0-1.0 · Источник: example.org/ivory · Из файла')
    expect(creditLine(pack({ author: '', license: '', source: '', origin: 'net' }))).toBe('Из сети')
    expect(creditLine(pack({ author: 'Sosnina Albums', license: '', source: '', origin: 'builtin' }))).toBe(
      'Автор: Sosnina Albums · Рисует программа',
    )
  })

  it('разметка в источнике остаётся текстом', () => {
    expect(creditLine(pack({ source: '<a href="x">y</a>' }))).toContain('Источник: <a href="x">y</a>')
  })

  it('встроенные первыми, остальные в порядке Go', () => {
    const list = [pack({ id: 'a' }), pack({ id: 'b' }), pack({ id: 'builtin', origin: 'builtin', removable: false })]
    expect(sortPacks(list).map((p) => p.id)).toEqual(['builtin', 'a', 'b'])
    expect(sortPacks([])).toEqual([])
  })

  it('«только встроенные» — пустое состояние', () => {
    expect(onlyBuiltin([])).toBe(true)
    expect(onlyBuiltin([pack({ origin: 'builtin' })])).toBe(true)
    expect(onlyBuiltin([pack({ origin: 'builtin' }), pack()])).toBe(false)
  })
})

describe('итоги «Добавить из файла»', () => {
  it('добавлен: название и состав', () => {
    expect(addedText(add({ pack: pack({ sticker: 0 }) }), false)).toBe('Набор «Айвори» добавлен: 12 фонов, 8 разворотов')
  })

  it('обновлён: версия в тексте', () => {
    expect(addedText(add(), true)).toBe('Набор «Айвори» обновлён до версии 3: 12 фонов, 30 стикеров, 8 разворотов')
  })

  it('пропущенное — в конце, не больше трёх имён', () => {
    const skipped = ['a', 'b', 'c', 'd', 'e'].map((n) => ({ name: n, why: 'плохая картинка' }))
    expect(skippedLabel(skipped)).toBe('пропущено: 5 (a — плохая картинка; b — плохая картинка; c — плохая картинка и ещё 2)')
    expect(addedText(add({ skipped: [{ name: 'x.png', why: 'SVG не поддерживается' }] }), false)).toMatch(
      /разворотов; пропущено: 1 \(x\.png — SVG не поддерживается\)$/,
    )
    expect(skippedLabel(null)).toBe('')
    expect(skippedLabel([])).toBe('')
  })

  it('тот же набор и набор старее — тосты без установки', () => {
    expect(notAddedText(add({ ask: 'same' }))).toBe('Набор «Айвори» уже есть')
    expect(notAddedText(add({ ask: 'older', fromVersion: 5 }))).toBe('Набор «Айвори»: уже стоит версия новее (5)')
    expect(notAddedText(add({ ask: 'older', fromVersion: 0 }))).toBe('Набор «Айвори»: уже стоит версия новее')
    expect(notAddedText(add())).toBe('')
    expect(notAddedText(add({ ask: 'update' }))).toBe('')
  })

  it('диалог закрыт без выбора — пустой итог Go', () => {
    expect(cancelled(add({ pack: pack({ id: '' }) }))).toBe(true)
    expect(cancelled(null)).toBe(true)
    expect(cancelled(add())).toBe(false)
    expect(cancelled(add({ pack: pack({ id: '' }), ask: 'update' }))).toBe(false)
  })

  it('вопрос «Заменить?»', () => {
    expect(replaceQuestion(add({ ask: 'update', fromVersion: 2 }))).toBe('Заменить набор «Айвори» версии 2 версией 3?')
  })
})

describe('вопрос «Удалить набор?»', () => {
  it('что пропадёт — по составу набора, что останется — в альбомах', () => {
    expect(removeQuestion(pack())).toBe(
      'Удалить набор «Айвори»? Его фоны, стикеры и развороты пропадут из окон. ' +
        'В открытом альбоме всё останется; в других альбомах — станет пропавшим и вернётся при повторной загрузке набора.',
    )
    expect(removeQuestion(pack({ sticker: 0, spread: 0 }))).toContain('Его фоны пропадут')
    expect(removeQuestion(pack({ sticker: 0 }))).toContain('Его фоны и развороты пропадут')
    expect(removeQuestion(pack({ bg: 0, sticker: 0, spread: 0 }))).toContain('Его содержимое пропадёт')
  })

  it('числа Go: сколько освободится и сколько файлов остаётся', () => {
    expect(removeNote({ files: 40, kept: 0, bytes: 58 * 1048576 })).toBe('Освободится 58,0 МБ (40 файлов).')
    expect(removeNote({ files: 21, kept: 3, bytes: 2048 })).toBe(
      'Освободится 2 КБ (21 файл). 3 файла останутся: нужны открытому альбому, вашим фонам или другим наборам.',
    )
    expect(removeNote({ files: 5, kept: 1, bytes: 100 })).toContain('1 файл останется')
    expect(removeNote({ files: 0, kept: 5, bytes: 0 })).toBe(
      'Место не освободится: все файлы набора нужны ещё где-то. 5 файлов останутся: нужны открытому альбому, вашим фонам или другим наборам.',
    )
  })
})
