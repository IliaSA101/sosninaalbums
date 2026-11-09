import { describe, expect, it } from 'vitest'
import type { Bin, Photo } from '../../base/core/types'
import {
  addedText,
  alreadyText,
  allBinInAlbumText,
  binAction,
  binButton,
  binButtonTip,
  binChange,
  binHost,
  noBinsText,
  binCounts,
  binIndex,
  binLabel,
  binTip,
  binsOf,
  cleanName,
  countText,
  deletedText,
  dropLabel,
  emptyBinText,
  filterBin,
  freeName,
  moreBinDrop,
  hiddenBinText,
  nameTaken,
  newText,
  removedText,
} from './bins'

const ph = (id: string): Photo => ({ id, path: id, name: id, w: 100, h: 100 })
const photos = ['a', 'b', 'c', 'd', 'e'].map(ph)
const bins: Bin[] = [
  { id: 'b1', name: 'Обязательно' },
  { id: 'b2', name: 'Запас' },
]
const marks = { a: { bins: ['b1'] }, b: { bins: ['b1', 'b2'] }, c: { rating: 3 }, d: { bins: ['b1'] }, zz: { bins: ['b1'] } }
const idx = binIndex(marks)

describe('индекс и фильтр', () => {
  it('членство по ID подборки, фото без подборок не в индексе', () => {
    expect([...(idx.get('b1') ?? [])].sort()).toEqual(['a', 'b', 'd', 'zz'])
    expect([...(idx.get('b2') ?? [])]).toEqual(['b'])
    expect(idx.has('b3')).toBe(false)
  })
  it('фильтр держит порядок ленты и не берёт фото вне ленты', () => {
    expect(filterBin(photos, 'b1', idx).map((p) => p.id)).toEqual(['a', 'b', 'd'])
  })
  it('подборка без фото и неизвестная — пустая лента', () => {
    expect(filterBin(photos, 'b9', idx)).toEqual([])
  })
})

describe('числа «в альбоме / всего»', () => {
  it('в альбоме = всего минус «Не в альбоме»: места и фон считаются, вне ленты — нет', () => {
    const c = binCounts(bins, photos, idx, new Set(['a']), new Set(['b', 'e']))
    expect(c.get('b1')).toEqual({ all: 3, inAlbum: 2 }) // a в месте, b на фоне; d свободно; zz вне ленты
    expect(c.get('b2')).toEqual({ all: 1, inAlbum: 1 })
    expect(countText(c.get('b1')!)).toBe('2 / 3')
  })
  it('совпадает со свободными фото подборки', () => {
    const used = new Set(['a', 'c'])
    const bd = new Set(['d'])
    const c = binCounts(bins, photos, idx, used, bd).get('b1')!
    const free = filterBin(photos, 'b1', idx).filter((p) => !used.has(p.id) && !bd.has(p.id))
    expect(c.all - c.inAlbum).toBe(free.length)
  })
  it('пустая подборка — нули', () => {
    expect(binCounts([{ id: 'b3', name: 'Пусто' }], photos, idx, new Set(), new Set()).get('b3')).toEqual({ all: 0, inAlbum: 0 })
  })
})

describe('«± N»', () => {
  it('все выделенные уже в подборке — убрать, иначе добавить', () => {
    expect(binAction(['a', 'b'], 'b1', idx)).toBe('remove')
    expect(binAction(['a', 'c'], 'b1', idx)).toBe('add')
    expect(binAction(['c'], 'b2', idx)).toBe('add')
  })
  it('без выделения — добавить (кнопки нет, но решение определено)', () => {
    expect(binAction([], 'b1', idx)).toBe('add')
  })
  it('сколько фото изменит действие', () => {
    expect(binChange(['a', 'c', 'e'], 'b1', idx, 'add')).toBe(2)
    expect(binChange(['a', 'c', 'e'], 'b1', idx, 'remove')).toBe(1)
    expect(binChange(['a', 'b'], 'b1', idx, 'add')).toBe(0)
  })
})

describe('имена', () => {
  it('первое свободное «Подборка N»', () => {
    expect(freeName([])).toBe('Подборка 1')
    expect(freeName([{ id: 'x', name: 'Подборка 1' }])).toBe('Подборка 2')
    expect(
      freeName([
        { id: 'x', name: 'подборка 1' },
        { id: 'y', name: 'Подборка 3' },
      ]),
    ).toBe('Подборка 2')
  })
  it('очистка как у Go: пробелы, управляющие, 40 знаков', () => {
    expect(cleanName('  Для   родителей\t')).toBe('Для родителей')
    expect(cleanName('a\u0000b')).toBe('a b')
    expect([...cleanName('я'.repeat(60))]).toHaveLength(40)
  })
  it('занято без учёта регистра; свою подборку при переименовании можно', () => {
    expect(nameTaken(bins, 'ЗАПАС')).toBe(true)
    expect(nameTaken(bins, 'Запас', 'b2')).toBe(false)
    expect(nameTaken(bins, ' Запас ', 'b1')).toBe(true)
    expect(nameTaken(bins, 'Новая')).toBe(false)
  })
  it('обрезка на кнопке', () => {
    expect(binLabel('Обязательно')).toBe('Обязательно')
    expect(binLabel('Очень длинное имя подборки')).toBe('Очень длинное и…')
    expect([...binLabel('Очень длинное имя подборки')]).toHaveLength(16)
  })
})

describe('тексты', () => {
  it('кнопка: число подборок или показанная с числами', () => {
    expect(binButton(0, null)).toBe('Подборки')
    expect(binButton(3, null)).toBe('Подборки 3')
    expect(binButton(3, { name: 'Обязательно', n: { all: 30, inAlbum: 24 } })).toBe('Обязательно 24 / 30')
    expect(binButtonTip({ name: 'Очень длинное имя подборки', n: { all: 30, inAlbum: 24 } })).toContain('«Очень длинное имя подборки»')
    expect(binButtonTip(null)).toContain('Подборки')
  })
  it('тосты и подсказки', () => {
    expect(addedText('Обязательно', 8)).toBe('В «Обязательно»: +8')
    expect(removedText('Обязательно', 3)).toBe('Из «Обязательно»: −3')
    expect(alreadyText('Запас', 'add')).toBe('Эти фото уже в «Запас»')
    expect(alreadyText('Запас', 'remove')).toBe('Этих фото нет в «Запас»')
    expect(deletedText('Запас')).toBe('Подборка «Запас» удалена')
    expect(binTip(['Обязательно', 'Для родителей'])).toBe('Подборки: Обязательно, Для родителей')
    expect(binTip([])).toBe('')
    expect(allBinInAlbumText('Запас')).toBe('Все фото подборки «Запас» уже в альбоме')
  })
  it('пустая лента: с клавишей и без неё', () => {
    expect(emptyBinText('Запас', 'B')).toBe(
      'В подборке «Запас» пока нет фото. Выделите фото в «Все фото» и нажмите B или перетащите их на кнопку «Подборки»',
    )
    expect(emptyBinText('Запас', '')).toContain('кнопкой «+» в окне «Подборки»')
  })
  it('до Go поля bins нет — пустой список', () => {
    expect(binsOf({})).toEqual([])
    expect(binsOf({ bins })).toBe(bins)
  })
})

describe('перенос и строка «Новая» (083 U2)', () => {
  it('подпись кнопки во время переноса: целевая названа (длинное имя обрезано), иначе окно выбора', () => {
    expect(dropLabel('Обязательно')).toBe('+ в «Обязательно»')
    expect(dropLabel('Очень длинное название подборки')).toBe('+ в «Очень длинное н…»')
    expect(dropLabel('')).toBe('+ в подборку…')
  })
  it('«Новая»: без выделения — просто, с выделением и с брошенными — свои слова', () => {
    expect(newText(0, false)).toBe('Новая подборка')
    expect(newText(8, false)).toBe('Новая из выделенных (8)')
    expect(newText(3, true)).toBe('Новая из перенесённых (3)')
  })
})

describe('окно подборок и «⋯» (083, ревью)', () => {
  it('кнопка в полосе — окно у неё; в «⋯» — открыть «⋯»; негде (нет группы, лента свёрнута) — сбросить флаг', () => {
    expect(binHost(true, false, false)).toBe('bar')
    expect(binHost(true, true, false)).toBe('more')
    expect(binHost(false, false, false)).toBe('none')
    expect(binHost(true, true, true)).toBe('none')
    expect(binHost(true, false, true)).toBe('none')
  })
  it('все фото подборки в скрытых папках — свой текст, а не «пока нет фото»', () => {
    expect(hiddenBinText('Запас')).toBe('Фото подборки «Запас» в скрытых папках')
    // подборка с фото в ленте «Все фото», но без показанных: filterBin по полному списку непуста
    const idx = binIndex({ a: { bins: ['b2'] } })
    expect(filterBin(photos, 'b2', idx).length).toBe(1)
    expect(filterBin(photos, 'b9', idx)).toEqual([])
  })
})

describe('бросок на «⋯» (083-29)', () => {
  it('кнопка в «⋯» и тащат фото — «⋯» цель; подпись только под курсором, как у кнопки', () => {
    expect(moreBinDrop('more', true, false, 'Запас')).toEqual({ drop: true, text: '' })
    expect(moreBinDrop('more', true, true, 'Запас')).toEqual({ drop: true, text: dropLabel('Запас') })
    expect(moreBinDrop('more', true, true, '').text).toBe('+ в подборку…') // целевой нет — окно «Новая из перенесённых»
  })
  it('кнопка в полосе, нет кнопки или не тащат — «⋯» не цель', () => {
    for (const host of ['bar', 'none'] as const) expect(moreBinDrop(host, true, true, 'Запас')).toEqual({ drop: false, text: '' })
    expect(moreBinDrop('more', false, true, 'Запас')).toEqual({ drop: false, text: '' })
  })
})

describe('noBinsText', () => {
  it('в полосе — «кнопка «Подборки»», в «⋯» — путь через «⋯»', () => {
    expect(noBinsText('bar')).toBe('Сначала создайте подборку: кнопка «Подборки»')
    expect(noBinsText('more')).toBe('Сначала создайте подборку: кнопка «⋯» → «Подборки»')
  })
})
