import { describe, expect, it, vi } from 'vitest'

const show = vi.hoisted(() => vi.fn())
vi.mock('../../shell/guide/guidestate.svelte', () => ({ guide: { show } }))

const { foldLink, foldNote, foldNoteShown } = await import('./foldnote')

describe('foldNote — строка «Ещё N раскладок с фото на сгибе»', () => {
  it('нет поля и 0 — строки нет', () => {
    expect(foldNote(undefined)).toBeNull()
    expect(foldNote(0)).toBeNull()
  })
  it.each([
    [1, 'Ещё 1 раскладка'],
    [2, 'Ещё 2 раскладки'],
    [5, 'Ещё 5 раскладок'],
    [8, 'Ещё 8 раскладок'],
  ])('%i — склонение', (n, head) => {
    expect(foldNote(n)).toEqual({ text: `${head} с фото на сгибе — `, link: 'Настройки альбома ▸ Любые фото' })
  })
})

describe('foldNoteShown — под каким семейством строка видна', () => {
  it('«Все» и «Фигуры» — да; остальные — нет', () => {
    expect(foldNoteShown('all')).toBe(true)
    expect(foldNoteShown('figure')).toBe(true)
    expect(foldNoteShown('dense')).toBe(false)
    expect(foldNoteShown('air')).toBe(false)
  })
})

describe('foldLink', () => {
  it('ведёт тем же ходом, что «Показать» в F1: статья «fold», место ui:fold-mode', () => {
    foldLink.open()
    expect(show).toHaveBeenCalledWith('fold', 'ui:fold-mode')
  })
})
