import { describe, expect, it } from 'vitest'
import { COMMANDS } from './commands'
import { buildMenu, keyId, type MenuCtx, type MenuItem } from './menu'
import { MOUSE_HELP } from './mousehelp'
import { catalogView } from '../../base/keys/keymap.fixture'
import { FEATURES, type FeatureId } from '../../base/prefs/levels'

// Контракт каталога клавиш Go (internal/keymap, testdata/parity/keymap.json) и интерфейса (спека 074, design §7.4).
const handlers = COMMANDS as Record<string, unknown>
const FEATURE_IDS = new Set<string>(FEATURES.flatMap((f) => [f.id, ...(f.also ?? [])]))

const ctx: MenuCtx = {
  spreads: 3,
  hasSpread: true,
  canUndo: true,
  canRedo: true,
  zone: 'canvas',
  libraryPhotos: 10,
  anySelection: true,
  deletable: true,
  flippable: true,
  varyable: true,
  layerable: true,
  missing: 2,
  big: false,
  details: false,
  safe: false,
  rulers: true,
  snap: true,
  recent: [{ path: 'a.salbum', name: 'a', exists: true, modified: 1 }],
}
const flat = (items: MenuItem[]): MenuItem[] => items.flatMap((i) => [i, ...(i.sub ? flat(i.sub) : [])])
// Пункты обеих зон: «Выделить всё» меняет команду.
const menuItems = () => [...buildMenu(ctx), ...buildMenu({ ...ctx, zone: 'library' })].flatMap((m) => flat(m.items))

describe('каталог Go и интерфейс', () => {
  it('id каталога уникальны, название и группа заданы', () => {
    const ids = catalogView.commands.map((e) => e.id)
    expect(new Set(ids).size).toBe(ids.length)
    const groups = catalogView.groups.map((g) => g.id)
    for (const e of catalogView.commands) {
      expect(e.label.trim(), e.id).not.toBe('')
      expect(groups, e.id).toContain(e.group)
    }
  })

  it('в каждой группе есть команды (окно не покажет пустых разделов)', () => {
    for (const g of catalogView.groups)
      expect(
        catalogView.commands.some((e) => e.group === g.id),
        g.id,
      ).toBe(true)
  })

  it('каждая команда каталога есть в COMMANDS', () => {
    for (const e of catalogView.commands) expect(typeof handlers[e.id], e.id).toBe('function')
  })

  it('feature каждой команды — функция уровня интерфейса', () => {
    for (const e of catalogView.commands) if (e.feature) expect(FEATURE_IDS.has(e.feature), `${e.id}: ${e.feature}`).toBe(true)
  })

  it('каждая команда меню без параметра есть в каталоге; align — alignLeft…alignDistV', () => {
    const ids = new Set(catalogView.commands.map((e) => e.id))
    for (const it of menuItems()) {
      if (!it.cmd) continue
      const id = keyId(it.cmd, it.arg)
      if (id) expect(ids.has(id), `${it.id} → ${id}`).toBe(true)
    }
  })

  it('каждая команда каталога без клавиши по умолчанию — пункт меню (иначе её клавиша не сработает: 074-8)', () => {
    const inMenu = new Set(menuItems().map((it) => it.cmd && keyId(it.cmd, it.arg)))
    for (const e of catalogView.commands) if (e.defaults.length === 0) expect(inMenu.has(e.id), e.id).toBe(true)
  })

  it('справочные строки мыши: показ и название заданы, группа и функция известны', () => {
    const groups = catalogView.groups.map((g) => g.id)
    for (const r of MOUSE_HELP) {
      expect(r.show.trim()).not.toBe('')
      expect(r.label.trim(), r.show).not.toBe('')
      expect(groups, r.show).toContain(r.group)
      if (r.feature) expect(FEATURE_IDS.has(r.feature as FeatureId), `${r.show}: ${r.feature}`).toBe(true)
    }
  })
})

// Справочные строки мыши (MOUSE_HELP): окно клавиш их не показывает, они живут в статьях F1.
describe('справочные строки мыши', () => {
  it('справка «Ручной»: перенос места и ⇄ вместо обмена телом', () => {
    const grid = MOUSE_HELP.filter((h) => h.group === 'grid').map((h) => h.show)
    expect(grid.some((s) => s.includes('Перетащить фото или рамку'))).toBe(true)
    expect(grid.some((s) => s.includes('⇄'))).toBe(true)
  })
  it('справка: Alt+щелчок в группе «Холст» на всех уровнях', () => {
    const row = MOUSE_HELP.find((h) => h.show === 'Alt+щелчок')
    expect(row?.group).toBe('canvas')
    expect(catalogView.groups.find((g) => g.id === 'canvas')?.title).toBe('Холст')
    expect(row?.feature).toBeUndefined()
  })
  it('справка: есть строки про колесо, мышь и редактор текста', () => {
    const shows = MOUSE_HELP.map((h) => h.show).join('|')
    for (const s of ['Колесо', 'Ctrl+колесо', 'Зажатое колёсико', 'Двойной щелчок по фону', 'Ctrl+Enter', 'Shift+клик по фото'])
      expect(shows).toContain(s)
  })
})
