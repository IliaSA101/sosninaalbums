import { describe, expect, it } from 'vitest'
import { revealLabel } from '../../base/core/platform'
import type { UseSrc } from '../../base/words/whereuse'
import { lineAt, USES_MENU_MAX } from '../../editor/photoprops/usemenu'
import { thumbMenu, type ThumbCtx } from './thumbmenu'

// Меню миниатюры ленты (106): пункты для одного, нескольких, пропавшего, не стоящего и «Простого».
const ctx = (extra: Partial<ThumbCtx> = {}): ThumbCtx => ({ photos: 1, missing: 0, used: 2, lines: null, ...extra })
const ids = (c: ThumbCtx, can?: (f: string) => boolean) => thumbMenu(c, can as never).map((n) => n.id)
const use = (i: number, what = 'место'): UseSrc => ({
  sheet: i,
  kind: 'place',
  spread: `s${i}`,
  page: `Разворот ${i}`,
  what,
  pick: `p${i}`,
})

describe('thumbMenu: состав', () => {
  it('одно стоящее фото: «Крупный просмотр», «Показать в …» (слово по ОС), «Копировать имя файла», «Где стоит ▸»', () => {
    const m = thumbMenu(ctx())
    expect(m.map((n) => n.id)).toEqual(['preview', 'explorer', 'copy', 'uses'])
    expect(m[1].label).toBe(revealLabel())
    expect(m[2].label).toBe('Копировать имя файла')
    expect(m[3].label).toBe('Где стоит')
    expect(m[3].sub).toBeTruthy()
  })

  it('«Крупный просмотр» первым, с клавишей справа из реестра, и черта под ним (128-8)', () => {
    const m = thumbMenu(ctx(), undefined, (cmd) => (cmd === 'photoPreview' ? 'Z' : ''))
    expect(m[0]).toMatchObject({ id: 'preview', label: 'Крупный просмотр', hint: 'Z' })
    expect(m[0].sep).toBeUndefined()
    expect(m[1]).toMatchObject({ id: 'explorer', sep: true })
  })

  it('клавишу сняли — пункт без подписи справа', () => {
    expect(thumbMenu(ctx())[0].hint).toBeUndefined()
  })

  it('2+ фото и пропавший файл — «Крупного просмотра» нет', () => {
    expect(ids(ctx({ photos: 2, used: 0 }))).not.toContain('preview')
    expect(ids(ctx({ missing: 1 }))).not.toContain('preview')
    expect(ids(ctx({ photos: 3, missing: 1, used: 0 }))).not.toContain('preview')
  })

  it('одно фото, которое нигде не стоит (числа нет) — без «Где стоит»; не серый, а скрыт', () => {
    expect(ids(ctx({ used: 0 }))).toEqual(['preview', 'explorer', 'copy'])
  })

  it('несколько фото: только «Копировать имена файлов»', () => {
    const m = thumbMenu(ctx({ photos: 3, used: 0 }))
    expect(m.map((n) => n.id)).toEqual(['copy'])
    expect(m[0].label).toBe('Копировать имена файлов')
  })

  it('пропавший файл: «Показать в …» и «Копировать» нет, «Где стоит» остаётся', () => {
    expect(ids(ctx({ missing: 1 }))).toEqual(['uses'])
    expect(ids(ctx({ missing: 1, used: 0 }))).toEqual([])
  })

  it('несколько фото, часть пропала: копируются найденные', () => {
    expect(ids(ctx({ photos: 3, missing: 1, used: 0 }))).toEqual(['copy'])
    expect(ids(ctx({ photos: 2, missing: 2, used: 0 }))).toEqual([])
  })

  it('«Простой» (нет photoMenu) и пустое выделение — меню нет', () => {
    expect(ids(ctx(), (f) => f !== 'photoMenu')).toEqual([])
    expect(ids(ctx({ photos: 0 }))).toEqual([])
  })
})

describe('thumbMenu: «Где стоит ▸»', () => {
  const sub = (c: ThumbCtx) => thumbMenu(c).find((n) => n.id === 'uses')?.sub ?? []

  it('ответа ещё нет — строка «…» без действия', () => {
    expect(sub(ctx())).toEqual([{ id: 'wait', label: '…', kind: 'note' }])
  })

  it('строки — «страница — что» в порядке Go; фон альбома — без тире', () => {
    const lines = [use(2), use(5, 'фон'), { sheet: -1, kind: 'albumBg', spread: '', page: 'Фон альбома', what: '' }]
    expect(sub(ctx({ lines })).map((n) => [n.id, n.arg, n.label])).toEqual([
      ['use', '0', 'Разворот 2 — место'],
      ['use', '1', 'Разворот 5 — фон'],
      ['use', '2', 'Фон альбома'],
    ])
  })

  it('больше предела — серое «и ещё N» без действия', () => {
    const lines = Array.from({ length: USES_MENU_MAX + 3 }, (_, i) => use(i + 1))
    const rows = sub(ctx({ lines }))
    expect(rows).toHaveLength(USES_MENU_MAX + 1)
    expect(rows.at(-1)).toEqual({ id: 'more', label: 'и ещё 3', kind: 'note' })
  })

  it('пустой ответ — пояснение', () => {
    expect(sub(ctx({ lines: [] }))).toEqual([{ id: 'none', label: 'Нигде не стоит', kind: 'note' }])
  })
})

describe('lineAt', () => {
  it('строка по номеру пункта; чужого номера нет — null', () => {
    const lines = [use(1), use(2)]
    expect(lineAt(lines, '1')).toMatchObject({ page: 'Разворот 2', pick: 'p2' })
    expect(lineAt(lines, '5')).toBeNull()
    expect(lineAt(lines, undefined)).toBeNull()
    expect(lineAt(null, '0')).toBeNull()
  })
})

// Проводка меню: DOM плиток в юнит-тестах не рисуется, поэтому смотрим исходники.
const files = import.meta.glob('/src/panels/library/{Thumb,Library}.svelte', { query: '?raw', import: 'default', eager: true }) as Record<
  string,
  string
>
describe('проводка', () => {
  it('плитка открывает меню правой кнопкой; прокрутка ленты закрывает; ThumbMenu стоит в ленте', () => {
    expect(files['/src/panels/library/Thumb.svelte']).toContain(
      'oncontextmenu={(e) => (peekPress(e) ? thumbmenu.peek(e, p.id) : thumbmenu.open(e, p.id))}',
    )
    const lib = files['/src/panels/library/Library.svelte']
    expect(lib).toMatch(/onscroll=\{\(\) => \{[^}]*thumbmenu\.close\(\)/)
    expect(lib).toContain('<ThumbMenu />')
  })
})
