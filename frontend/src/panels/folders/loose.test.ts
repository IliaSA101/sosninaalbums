import { describe, expect, it } from 'vitest'
import { pruneHidden } from './tree'
import {
  GROUP_KEY,
  addedWhen,
  batchKey,
  batchLabel,
  batchLabels,
  cachedLoose,
  fileKey,
  looseAskText,
  looseGroup,
  looseInAlbum,
  looseKeys,
  looseToast,
} from './loose'

const NOW = new Date(2026, 9, 21, 15, 0, 0)
const sec = (d: Date) => d.getTime() / 1000

describe('batchLabel', () => {
  it('сегодня — время, в этом году — «21.10, 14:05», раньше — с годом', () => {
    expect(batchLabel(sec(new Date(2026, 9, 21, 14, 5)), NOW)).toBe('14:05')
    expect(batchLabel(sec(new Date(2026, 9, 20, 9, 7)), NOW)).toBe('20.10, 09:07')
    expect(batchLabel(sec(new Date(2025, 9, 21, 14, 5)), NOW)).toBe('21.10.2025, 14:05')
    expect(batchLabel(sec(new Date(2026, 9, 21, 14, 5, 33)), NOW, true)).toBe('14:05:33')
  })

  it('одна минута у двух добавлений — у обоих секунды; остальные без', () => {
    const at = (h: number, m: number, s: number) => sec(new Date(2026, 9, 20, h, m, s))
    const l = [
      { id: 'a', at: at(14, 5, 3), files: [] },
      { id: 'b', at: at(14, 5, 33), files: [] },
      { id: 'c', at: at(18, 40, 0), files: [] },
    ]
    expect(batchLabels(l, NOW)).toEqual(['20.10, 14:05:03', '20.10, 14:05:33', '20.10, 18:40'])
  })

  it('addedWhen: подпись без даты — «сегодня в …»', () => {
    expect(addedWhen('14:05')).toBe('сегодня в 14:05')
    expect(addedWhen('14:05:33')).toBe('сегодня в 14:05:33')
    expect(addedWhen('21.10, 14:05')).toBe('21.10, 14:05')
  })
})

describe('toast parts', () => {
  const r = (o: object) => ({ batch: 'b', added: 0, inFolder: 0, folder: '', dup: 0, skipped: 0, bad: 0, over: 0, ...o })

  it('добавлено и части пропусков через « · »', () => {
    expect(looseToast(r({ added: 3 }))).toEqual({ text: 'Добавлено 3 фото', kind: 'info' })
    expect(looseToast(r({ added: 3, inFolder: 1, folder: 'E:\\Съёмки\\Март 2026' }))?.text).toBe(
      'Добавлено 3 фото · 1 уже есть в папке «Март 2026» — не добавлено',
    )
    expect(looseToast(r({ added: 2, dup: 1, skipped: 2 }))?.text).toBe('Добавлено 2 фото · уже добавлены раньше: 1 · не фото: 2')
  })

  it('ничего не добавлено: только папка — «Фото уже есть…»; иначе с заглавной; путь и предел — красный', () => {
    expect(looseToast(r({ batch: '', inFolder: 2, folder: 'D:/Март 2026' }))).toEqual({
      text: 'Фото уже есть в папке «Март 2026» — не добавлено',
      kind: 'info',
    })
    expect(looseToast(r({ batch: '', dup: 2 }))).toEqual({ text: 'Уже добавлены раньше: 2', kind: 'info' })
    expect(looseToast(r({ batch: '', bad: 1 }))).toEqual({
      text: 'Не добавлено путей: 1 — поддерживаются только локальные пути вида C:\\папка',
      kind: 'error',
    })
    expect(looseToast(r({ batch: '', over: 5 }))).toEqual({ text: 'Отдельных фото не больше 2 000 — не добавлено 5', kind: 'error' })
    expect(looseToast(r({ added: 1, over: 5 }))?.kind).toBe('info')
  })

  it('отмена системного окна — без тоста; папка неизвестна — «в папке альбома»', () => {
    expect(looseToast(r({ batch: '' }))).toBeNull()
    expect(looseToast(r({ added: 1, inFolder: 1 }))?.text).toBe('Добавлено 1 фото · 1 уже есть в папке альбома — не добавлено')
  })
})

describe('ask texts', () => {
  const at = sec(new Date(2026, 9, 20, 14, 5))
  const g = looseGroup(
    [
      {
        id: 'b1',
        at,
        files: ['a', 'b', 'c', 'd', 'e'].map((n) => ({ path: `D:/x/${n}.jpg`, photo: n })),
      },
      { id: 'b2', at: at + 3600, files: [{ path: 'C:/Загрузки/bouquet.jpg', photo: 'q' }] },
    ],
    () => true,
    false,
    NOW,
  )!
  const [b1, b2] = g.kids

  it('добавление: «2 из 5 фото, добавленных 20.10, 14:05, стоят в альбоме»', () => {
    const used = new Set(['a', 'c', 'zzz'])
    expect(looseInAlbum(b1, used)).toBe(2)
    expect(looseAskText(b1, 2)).toBe(
      '2 из 5 фото, добавленных 20.10, 14:05, стоят в альбоме. На листах они останутся, а из ленты пропадут.',
    )
    expect(looseAskText(b1, 1)).toBe(
      '1 из 5 фото, добавленных 20.10, 14:05, стоит в альбоме. На листе оно останется, а из ленты пропадут все 5 фото.',
    )
  })

  it('файл и добавление из одного файла — «Фото «bouquet.jpg» стоит в альбоме»', () => {
    const text = 'Фото «bouquet.jpg» стоит в альбоме. На листе оно останется, а из ленты пропадёт.'
    expect(looseAskText(b2.kids[0], 1)).toBe(text)
    expect(looseAskText(b2, 1)).toBe(text)
    expect(looseInAlbum(b2.kids[0], new Set(['q']))).toBe(1)
    expect(looseInAlbum(b2.kids[0], new Set())).toBe(0)
  })
})

describe('ключи и память', () => {
  const L = [{ id: 'b1', at: 0, files: [{ path: 'D:\\x\\1.JPG', photo: 'p1' }] }]

  it('looseKeys: ветка, добавление, файл; prune забывает убранное добавление', () => {
    expect([...looseKeys(L)].sort()).toEqual([GROUP_KEY, batchKey('b1'), fileKey('d:/x/1.jpg')].sort())
    expect(looseKeys([]).size).toBe(0)
    const hidden = new Set([fileKey('D:/x/1.jpg'), batchKey('b1'), batchKey('old'), GROUP_KEY, fileKey('D:/x/2.jpg')])
    pruneHidden(hidden, [], looseKeys(L))
    expect([...hidden].sort()).toEqual([GROUP_KEY, batchKey('b1'), fileKey('D:/x/1.jpg')].sort())
    pruneHidden(hidden, [], looseKeys([]))
    expect(hidden.size).toBe(0)
  })

  it('повтор ключа (не бывает при инварианте Go) — одна строка; пустое добавление — нет строки', () => {
    const g = looseGroup(
      [
        { id: 'b1', at: 0, files: [{ path: 'D:/x/1.jpg', photo: 'p1' }] },
        { id: 'b2', at: 0, files: [{ path: 'd:\\X\\1.jpg', photo: 'p1' }] },
        { id: 'b1', at: 0, files: [{ path: 'D:/y.jpg', photo: 'p2' }] },
      ],
      () => true,
      false,
      NOW,
    )!
    expect(g.kids.map((b) => [b.key, b.kids.length])).toEqual([[batchKey('b1'), 1]])
  })

  it('cachedLoose: те же добавления, лента и чтение — та же ветка; иначе новая', () => {
    const c = cachedLoose()
    const byId = new Map([['p1', {}]])
    const a = c(L, byId, false, NOW)
    expect(c(L, byId, false, NOW)).toBe(a)
    expect(c(L, byId, true, NOW)).not.toBe(a)
    expect(c(L, new Map(), true, NOW)?.kids[0].kids[0].own).toBe(0)
    expect(c([], new Map(), true, NOW)).toBeNull()
  })
})
