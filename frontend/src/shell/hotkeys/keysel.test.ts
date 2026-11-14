import { beforeEach, describe, expect, it } from 'vitest'
import { buildIndex } from '../../base/keys/keyindex'
import { catalogView, catalogWith } from '../../base/keys/keymap.fixture'
import type { KeymapEntry } from '../../base/types/keys'
import { chordOf, cmdsAt, curId, dimOthers, keyCell, modsOf, NO_MODS, reasonOf, shadowsOf, type SelCtx } from './keysel'

let view = catalogView
let ctx: SelCtx
const setup = (v = catalogView, visible: (e: KeymapEntry) => boolean = () => true) => {
  view = v
  ctx = { index: buildIndex(v), locked: v.locked, visible }
}
beforeEach(() => setup())

describe('слой и сочетание', () => {
  it('chordOf: порядок Mod, Alt, Shift; modsOf — обратно', () => {
    expect(chordOf('KeyD', { mod: true, alt: false, shift: true })).toBe('Mod+Shift+KeyD')
    expect(chordOf('KeyD', NO_MODS)).toBe('KeyD')
    expect(modsOf('Mod+Alt+KeyD')).toEqual({ mod: true, alt: true, shift: false })
  })
})

describe('служебные клавиши (замок)', () => {
  it('Esc, Tab, Пробел, F1 — с любыми модификаторами; стрелки без Ctrl/Alt; Ctrl+Alt+что угодно', () => {
    for (const c of ['Escape', 'Mod+Escape', 'Shift+Tab', 'Mod+Shift+Space', 'F1', 'Alt+F1'])
      expect(reasonOf(c, ctx.locked), c).not.toBe('')
    for (const c of ['ArrowLeft', 'Shift+ArrowUp']) expect(reasonOf(c, ctx.locked), c).not.toBe('')
    expect(reasonOf('Mod+ArrowLeft', ctx.locked)).toBe('')
    expect(reasonOf('Mod+Alt+KeyE', ctx.locked)).toMatch(/Ctrl\+Alt|AltGr/)
    expect(reasonOf('Alt+F4', ctx.locked)).toMatch(/Windows/)
  })
  it('обычные клавиши свободны', () => {
    for (const c of ['KeyD', 'Mod+KeyD', 'Mod+Shift+KeyD', 'Digit0', 'Delete']) expect(reasonOf(c, ctx.locked), c).toBe('')
  })
  it('порядок проверки как в Go: системные Windows раньше правил «с любыми модификаторами»', () => {
    expect(reasonOf('Mod+Escape', ctx.locked)).toBe('системная клавиша Windows')
    expect(reasonOf('Shift+Escape', ctx.locked)).toMatch(/^Esc/)
  })
})

describe('команды на клавише', () => {
  it('Digit0: в ленте — «Звёзды: убрать», рядом «Вписать»; старшая область первой', () => {
    const l = cmdsAt(ctx, 'Digit0').map((e) => e.id)
    expect(l).toEqual(expect.arrayContaining(['markRating0', 'fitZoom']))
    expect(cmdsAt(ctx, 'KeyD')).toEqual([])
  })
  it('скрытая на уровне команда в списке не видна, но с all — видна', () => {
    setup(catalogView, (e) => e.id !== 'fitZoom')
    expect(cmdsAt(ctx, 'Digit0').map((e) => e.id)).not.toContain('fitZoom')
    expect(cmdsAt(ctx, 'Digit0', true).map((e) => e.id)).toContain('fitZoom')
  })
})

describe('единый выбор: клавиша = строка списка (макет Г, замечание владельца)', () => {
  it('выбрана строка — это она; выбрана клавиша — переключатель или старшая команда', () => {
    expect(curId(ctx, { kind: 'cmd', id: 'undo' })).toBe('undo')
    const top = cmdsAt(ctx, 'Digit0')[0].id
    expect(curId(ctx, { kind: 'key', chord: 'Digit0', pick: null })).toBe(top)
    const other = cmdsAt(ctx, 'Digit0')[1].id
    expect(curId(ctx, { kind: 'key', chord: 'Digit0', pick: other })).toBe(other)
    expect(curId(ctx, { kind: 'key', chord: 'Digit0', pick: 'нет такой' })).toBe(top)
  })
  it('свободная и служебная клавиши команду не выбирают; ничего не выбрано — нет', () => {
    expect(curId(ctx, { kind: 'key', chord: 'KeyD', pick: null })).toBeNull()
    expect(curId(ctx, { kind: 'key', chord: 'Escape', pick: null })).toBeNull()
    expect(curId(ctx, null)).toBeNull()
  })
  it('остальные клавиши приглушены: при выборе клавиши всегда, при выборе строки — если её клавиша на этом слое', () => {
    expect(dimOthers(view, null, NO_MODS)).toBe(false)
    expect(dimOthers(view, { kind: 'key', chord: 'KeyD', pick: null }, NO_MODS)).toBe(true)
    expect(dimOthers(view, { kind: 'cmd', id: 'toggleBig' }, NO_MODS)).toBe(true) // F — без модификаторов
    expect(dimOthers(view, { kind: 'cmd', id: 'toggleBig' }, { ...NO_MODS, mod: true })).toBe(false) // слой Ctrl: клавиши F там нет
    expect(dimOthers(view, { kind: 'cmd', id: 'нет' }, NO_MODS)).toBe(false)
  })
  it('клавиша карты: выбрана рамкой, клавиша выбранной команды — подсветкой', () => {
    const sel = { kind: 'cmd', id: 'toggleBig' } as const
    const k = keyCell(ctx, 'KeyF', NO_MODS, sel, curId(ctx, sel), view)
    expect(k).toMatchObject({ chord: 'KeyF', selected: false, hl: true, why: '' })
    expect(k.cmds.map((e) => e.id)).toContain('toggleBig')
    const keySel = { kind: 'key', chord: 'KeyF', pick: null } as const
    expect(keyCell(ctx, 'KeyF', NO_MODS, keySel, curId(ctx, keySel), view)).toMatchObject({ selected: true, hl: true })
    expect(keyCell(ctx, 'KeyG', NO_MODS, keySel, curId(ctx, keySel), view)).toMatchObject({ selected: false, hl: false })
    expect(keyCell(ctx, 'Escape', NO_MODS, null, null, view).why).not.toBe('')
  })
  it('своя клавиша: команда на новом месте, на старом — пусто', () => {
    setup(catalogWith({ toggleBig: ['KeyQ'] })) // KeyB занят подборками (083)
    expect(cmdsAt(ctx, 'KeyQ').map((e) => e.id)).toEqual(['toggleBig'])
    expect(cmdsAt(ctx, 'KeyF')).toEqual([])
  })
})

describe('перекрытия областей', () => {
  const e = (id: string) => view.commands.find((x) => x.id === id)!
  it('Digit0: у «Звёзды: убрать» (лента) перекрытие с «Вписать» (везде); одинаковый ранг холст/лента — не перекрытие', () => {
    const o = shadowsOf(view, ctx.index, e('markRating0'), 'Digit0').map((x) => x.id)
    expect(o).toContain('fitZoom')
    expect(shadowsOf(view, ctx.index, e('markRating0'), 'Digit0').every((x) => x.scope !== 'library')).toBe(true)
  })
  it('Delete: режим фона старше холста', () => {
    expect(shadowsOf(view, ctx.index, e('removeCell'), 'Delete').map((x) => x.id)).toContain('bgRemove')
    expect(shadowsOf(view, ctx.index, e('bgRemove'), 'Delete').map((x) => x.id)).toContain('removeCell')
  })
  it('клавиша без других команд — пусто', () => {
    expect(shadowsOf(view, ctx.index, e('toggleBig'), 'KeyF')).toEqual([])
  })
})
