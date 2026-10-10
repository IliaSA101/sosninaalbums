import { describe, expect, it, vi } from 'vitest'
import { COMMANDS } from './commands'
import { ownMarks } from '../../panels/marks/ownmarks.svelte'
import { HOTKEYS, KEY_ACTIONS } from './hotkeys'
import { buildMenu, type MenuCtx, type MenuItem } from './menu'

const full: MenuCtx = {
  spreads: 1,
  hasSpread: true,
  canUndo: true,
  canRedo: true,
  zone: 'canvas',
  libraryPhotos: 1,
  anySelection: true,
  deletable: true,
  flippable: true,
  missing: 1,
  exporting: false,
  big: false,
  details: false,
  safe: false,
  rulers: true,
  snap: true,
  recent: [{ path: 'a.salbum', name: 'a', exists: true, modified: 1 }],
}
const flat = (items: MenuItem[]): MenuItem[] => items.flatMap((i) => [i, ...(i.sub ? flat(i.sub) : [])])

describe('команды', () => {
  it('у каждого пункта меню есть команда', () => {
    // Пункты обеих зон: «Выделить всё» меняет команду.
    const items = [...buildMenu(full), ...buildMenu({ ...full, zone: 'library' })].flatMap((m) => flat(m.items))
    expect(items.length).toBeGreaterThan(20)
    for (const it of items) {
      if (it.sub) expect(it.cmd, it.id).toBeUndefined()
      else {
        expect(it.cmd, it.id).toBeDefined()
        expect(typeof COMMANDS[it.cmd!], it.id).toBe('function')
      }
    }
  })

  it('у каждого действия таблицы клавиш есть команда', () => {
    for (const a of KEY_ACTIONS) expect(typeof COMMANDS[a], a).toBe('function')
    for (const h of HOTKEYS) if (h.action) expect(COMMANDS[h.action], h.show).toBeDefined()
  })

  it('Ctrl+R ничего не делает, но команда есть (окно не перезагружается)', () => {
    expect(COMMANDS.reload()).toBeUndefined()
  })

  it('клавиши отметок собирают MarkPatch из e.key', () => {
    const set = vi.spyOn(ownMarks, 'setMarks').mockResolvedValue(true)
    const key = (k: string) => ({ key: k }) as KeyboardEvent
    COMMANDS.markRating(key('4'))
    COMMANDS.markRating(key('0'))
    COMMANDS.markStep(key(']'))
    COMMANDS.markStep(key('х'))
    COMMANDS.markStep(key('ъ'))
    COMMANDS.markLabel(key('7'))
    COMMANDS.markPick(key('p'))
    COMMANDS.markReject(key('x'))
    COMMANDS.markUnflag(key('u'))
    expect(set.mock.calls.map((c) => c[1])).toEqual([
      { rating: 4 },
      { rating: 0 },
      { step: 1 },
      { step: -1 },
      { step: 1 },
      { label: 'yellow', toggle: true },
      { flag: 1 },
      { flag: -1 },
      { flag: 0 },
    ])
    // Без события (меню) и с чужой клавишей — ничего.
    set.mockClear()
    COMMANDS.markRating()
    COMMANDS.markLabel(key('1'))
    expect(set).not.toHaveBeenCalled()
    set.mockRestore()
  })
})
