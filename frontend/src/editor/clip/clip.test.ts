import { beforeEach, describe, expect, it } from 'vitest'
import type { SpreadView } from '../../base/core/types'
import {
  NO_CLIP,
  REPEAT_MS,
  canPaste,
  clipCount,
  clipSel,
  clipToast,
  clipWords,
  fresh,
  nativeSel,
  pastedRefs,
  resetStamps,
  stamp,
  type ClipInfo,
} from './clip'

// Чистая часть буфера (099): что уходит в Go, слова тоста, страж повтора, чужое выделение страницы.
const info = (o: Partial<ClipInfo> = {}): ClipInfo => ({ ...NO_CLIP, ...o })

const grid = (): SpreadView =>
  ({
    id: 's',
    grid: true,
    photos: ['a', 'b'],
    rects: [],
    texts: [{ id: 't1' }, { id: 't2' }],
    stickers: [{ id: 'k1' }],
    places: [
      { id: 'p0', photo: 'a', x: 0, y: 0, w: 10, h: 10, fit: '', rot: 0 },
      { id: 'p1', photo: '', x: 20, y: 0, w: 10, h: 10, fit: '', rot: 0 },
    ],
  }) as unknown as SpreadView
const auto = (): SpreadView =>
  ({
    id: 'a',
    grid: false,
    photos: ['a', 'b'],
    rects: [
      { x: 0, y: 0, w: 5, h: 5 },
      { x: 6, y: 0, w: 5, h: 5 },
    ],
    texts: [],
    places: [],
  }) as unknown as SpreadView

describe('clipSel', () => {
  it('«Ручная»: места по ID (занятое по фото тоже), надписи и стикеры — по ID', () => {
    expect(clipSel(grid(), ['a', 'p1'], ['t1'], ['k1'], null)).toEqual({ refs: ['p0', 'p1'], texts: ['t1'], stickers: ['k1'] })
  })
  it('«Авто»: ссылка — ID фото', () => {
    expect(clipSel(auto(), ['b'], [], [], null)).toEqual({ refs: ['b'], texts: [], stickers: [] })
  })
  it('одиночная надпись (edit.sel) входит без повтора', () => {
    expect(clipSel(grid(), [], ['t2'], [], 't2')?.texts).toEqual(['t2'])
    expect(clipSel(grid(), [], [], [], 't1')).toEqual({ refs: [], texts: ['t1'], stickers: [] })
  })
  it('чужое и исчезнувшее отбрасывается; ничего не осталось — null', () => {
    expect(clipSel(grid(), ['zzz'], ['tx'], ['kx'], 'ty')).toBeNull()
    expect(clipSel(grid(), ['p0', 'zzz'], ['tx'], [], null)).toEqual({ refs: ['p0'], texts: [], stickers: [] })
  })
  it('нет листа или пустое выделение — null', () => {
    expect(clipSel(undefined, ['a'], [], [], null)).toBeNull()
    expect(clipSel(grid(), [], [], [], null)).toBeNull()
  })
  it('фото и его место в выделении — один элемент', () => {
    expect(clipSel(grid(), ['a', 'p0'], [], [], null)?.refs).toEqual(['p0'])
  })
})

describe('pastedRefs', () => {
  it('занятое место выделяется по фото, пустое — по ID, «Авто» — по фото', () => {
    expect(pastedRefs(grid(), ['p0', 'p1'])).toEqual(['a', 'p1'])
    expect(pastedRefs(auto(), ['b'])).toEqual(['b'])
  })
  it('нет такого места — выпадает', () => {
    expect(pastedRefs(grid(), ['nope'])).toEqual([])
    expect(pastedRefs(undefined, ['a'])).toEqual([])
  })
})

describe('слова тоста', () => {
  it('«2 фото, надпись»: единица без числа, склонение по числу', () => {
    expect(clipWords(info({ photos: 2, texts: 1 }))).toBe('2 фото, надпись')
    expect(clipWords(info({ photos: 1 }))).toBe('фото')
    expect(clipWords(info({ places: 1, texts: 3, stickers: 5 }))).toBe('пустое место, 3 надписи, 5 стикеров')
    expect(clipWords(info({ places: 2, stickers: 2 }))).toBe('2 пустых места, 2 стикера')
    expect(clipWords(info({ places: 11, texts: 21 }))).toBe('11 пустых мест, 21 надпись')
  })
  it('Ctrl+C: «Скопировано: … Вставить — Ctrl+V на любом листе»', () => {
    expect(clipToast('Скопировано', info({ photos: 2, texts: 1 }), 'Ctrl+V')).toBe(
      'Скопировано: 2 фото, надпись. Вставить — Ctrl+V на любом листе',
    )
  })
  it('Ctrl+X: «Вырезано: … Вставить — Ctrl+V»', () => {
    expect(clipToast('Вырезано', info({ stickers: 1 }), 'Ctrl+V')).toBe('Вырезано: стикер. Вставить — Ctrl+V')
  })
  it('клавиша вставки снята — хвоста нет', () => {
    expect(clipToast('Скопировано', info({ texts: 1 }), '')).toBe('Скопировано: надпись.')
  })
})

describe('сводка буфера', () => {
  it('пуст — вставлять нечего; любое число — можно', () => {
    expect(clipCount(NO_CLIP)).toBe(0)
    expect(canPaste(NO_CLIP)).toBe(false)
    for (const k of ['photos', 'places', 'texts', 'stickers'] as const) expect(canPaste(info({ [k]: 1 })), k).toBe(true)
  })
})

describe('страж повтора', () => {
  beforeEach(() => resetStamps())
  it('до первой команды — свежо; в течение 300 мс после неё — нет; позже — да', () => {
    expect(fresh('pasteItems', 1000)).toBe(true)
    stamp('pasteItems', 1000)
    expect(fresh('pasteItems', 1000 + REPEAT_MS)).toBe(false)
    expect(fresh('pasteItems', 1000 + REPEAT_MS + 1)).toBe(true)
  })
  it('команды независимы: копирование не гасит вставку', () => {
    stamp('copyItems', 5000)
    expect(fresh('pasteItems', 5001)).toBe(true)
    expect(fresh('copyItems', 5001)).toBe(false)
  })
})

describe('nativeSel: выделен текст страницы вне холста', () => {
  const el = (inStage: boolean) => ({ closest: (s: string) => (inStage && s === '.stage' ? {} : null) })
  const sel = (o: { collapsed?: boolean; text?: string; anchor?: object | null }) => ({
    isCollapsed: !!o.collapsed,
    toString: () => o.text ?? 'слово',
    anchorNode: (o.anchor === undefined ? el(false) : o.anchor) as Node | null,
  })
  it('справочник, «О снимке» (вне .stage) — да', () => {
    expect(nativeSel(sel({}))).toBe(true)
  })
  it('нет выделения, свёрнутое, пустая строка — нет', () => {
    expect(nativeSel(null)).toBe(false)
    expect(nativeSel(sel({ collapsed: true }))).toBe(false)
    expect(nativeSel(sel({ text: '' }))).toBe(false)
  })
  it('внутри холста — нет: Ctrl+C копирует выделенное на листе', () => {
    expect(nativeSel(sel({ anchor: el(true) }))).toBe(false)
  })
  it('якорь — текстовый узел: смотрим его элемент', () => {
    expect(nativeSel(sel({ anchor: { parentElement: el(true) } }))).toBe(false)
    expect(nativeSel(sel({ anchor: { parentElement: el(false) } }))).toBe(true)
  })
  it('якоря нет — считаем страницей', () => {
    expect(nativeSel(sel({ anchor: null }))).toBe(true)
  })
})
