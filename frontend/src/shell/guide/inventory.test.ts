import { afterEach, beforeEach, describe, expect, it } from 'vitest'
import { catalogView, dropCatalog, useCatalog } from '../../base/keys/keymap.fixture'
import { ACTS, type ActId } from '../../editor/photomenu/photoacts'
import type { Command } from '../menu/commands'
import type { Menu } from '../menu/menu'
import { MOUSE_HELP } from '../menu/mousehelp'
import { ARTICLES } from '../guidetext/articles'
import { menus } from './show'
import { ACT_GROUPS, functionCount, functionsText, guideCount, inventory } from './inventory'

// Опись функций (спека 140, design §2). Само число не фиксируем: оно растёт с функциями, полноту держат стражи S1–S5.
beforeEach(() => useCatalog())
afterEach(() => dropCatalog())

const count = (menu: Menu[], extra: readonly { id: string }[] = []): number =>
  functionCount(inventory(menu, [...catalogView.commands, ...extra]))

// Меню с лишним пунктом-командой в конце первого меню.
const withItem = (cmd: string): Menu[] => {
  const [first, ...rest] = menus()
  return [{ ...first, items: [...first.items, { id: `zz-${cmd}`, label: 'Новый пункт', cmd: cmd as Command }] }, ...rest]
}

describe('inventory: что считается функцией', () => {
  it('число больше числа статей', () => {
    expect(count(menus())).toBeGreaterThan(ARTICLES.length)
  })
  it('жесты без повторов: одинаковая подпись считается один раз', () => {
    const g = inventory(menus(), []).gestures
    expect(new Set(g).size).toBe(g.length)
    expect(g.length).toBe(new Set(MOUSE_HELP.map((r) => r.show)).size)
  })
  it('команда из каталога, которой нет в меню, добавляет ровно единицу', () => {
    expect(count(menus(), [{ id: 'zzNewCmd' }])).toBe(count(menus()) + 1)
  })
  it('повтор команды в меню число не меняет, а команда только меню — добавляет', () => {
    expect(count(withItem('save'))).toBe(count(menus()))
    expect(count(withItem('zzMenuOnly'))).toBe(count(menus()) + 1)
  })
  it('команда во вложенном подменю тоже в описи', () => {
    const nested: Menu[] = [
      { id: 'file', label: 'Файл', items: [{ id: 'zzGroup', label: 'Группа', sub: [{ id: 'zzIn', label: 'Внутри', cmd: 'undo' }] }] },
    ]
    expect(inventory(nested, []).commands.has('undo')).toBe(true)
  })
  it('группы-подменю ACT_GROUPS и действия с командой в acts не входят', () => {
    const i = inventory(menus(), catalogView.commands)
    for (const g of ACT_GROUPS) expect(i.acts.has(g)).toBe(false)
    for (const [id, a] of Object.entries(ACTS)) if (a.cmd) expect(i.acts.has(id as ActId)).toBe(false)
  })
  it('действия-дубли команд (удалить, перепривязать) не считаются дважды', () => {
    const i = inventory(menus(), catalogView.commands)
    expect(i.acts.has('delete')).toBe(false)
    expect(i.acts.has('relink')).toBe(false)
    expect(i.commands.has('removeCell')).toBe(true)
    expect(i.commands.has('relink')).toBe(true)
  })
  it('жесты — все строки MOUSE_HELP', () => {
    expect(inventory(menus(), []).gestures).toEqual(MOUSE_HELP.map((r) => r.show))
  })
  it('guideCount: то же число, что у описи полного меню и каталога', () => {
    expect(guideCount()).toBe(count(menus()))
  })
})

describe('functionsText: склонение', () => {
  it('1, 2–4, 5+, 11–14, 21', () => {
    expect(functionsText(1)).toBe('1 функция')
    expect(functionsText(3)).toBe('3 функции')
    expect(functionsText(170)).toBe('170 функций')
    expect(functionsText(12)).toBe('12 функций')
    expect(functionsText(171)).toBe('171 функция')
  })
})
