import { afterEach, beforeEach, describe, expect, it } from 'vitest'
import { dropCatalog, useCatalog } from '../../base/keys/keymap.fixture'
import type { IconName } from '../../base/ui/Icon.svelte'
import { ALIGN_OPS } from '../../panels/layouts/align'
import { buildMenu, hasIcons, hasMarks, type Menu, type MenuCtx, type MenuItem } from './menu'

// Значки главного меню (владелец 23.10): только у пунктов «Правки», у которых есть кнопка-двойник со значком;
// значок — тот же, что у кнопки. «Файл», «Вид», «Справка» — без значков.
beforeEach(() => useCatalog())
afterEach(() => dropCatalog())

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
  sticker: true,
  varyable: true,
  vigable: true,
  vigSetup: true,
  decorable: true,
  copyable: true,
  pasteable: true,
  layerable: true,
  alignCount: 3,
  missing: 2,
  big: false,
  details: false,
  safe: false,
  rulers: true,
  snap: true,
}

const flat = (items: MenuItem[]): MenuItem[] => items.flatMap((i) => [i, ...(i.sub ? flat(i.sub) : [])])
const items = (menus: Menu[], id: string) => flat(menus.find((m) => m.id === id)!.items)
const iconOf = (menus: Menu[], id: string) => items(menus, 'edit').find((i) => i.id === id)?.icon

// Исходники кнопок-двойников: значок у них записан литералом в разметке.
const src = import.meta.glob('/src/**/*.{ts,svelte}', { query: '?raw', import: 'default', eager: true }) as Record<string, string>
const text = (path: string): string => src[path]

// Пункт → значок двойника (где двойник — в комментарии).
const TWINS: [string, IconName][] = [
  ['undo', 'undo'], // верхняя полоса
  ['redo', 'redo'], // верхняя полоса
  ['copyItems', 'copy'], // меню фото
  ['pasteItems', 'paste'], // меню фото
  ['removeCell', 'trash'], // мини-панель
  ['addText', 'text'], // лента «Добавить»
  ['addSticker', 'sticker'], // лента «Добавить»
  ['makeVignette', 'vignette'], // карточка разворота
  ['layer', 'layers'], // «⋯ ▸ Слой» и меню фото
  ['layerFront', 'layer-front'], // «Расположение» в карточке
  ['layerUp', 'layer-up'],
  ['layerDown', 'layer-down'],
  ['layerBack', 'layer-back'],
  ['turn', 'turn-cw'], // мини-панель: кнопка поворота
  ['turnCw', 'turn-cw'],
  ['turnCcw', 'turn-ccw'], // меню фото
  ['straighten', 'level'], // меню фото
  ['flip', 'flip-h'], // мини-панель: кнопка зеркала
  ['flipH', 'flip-h'],
  ['flipV', 'flip-v'],
  ['gray', 'gray'], // меню фото
  ...ALIGN_OPS.map((a): [string, IconName] => [`align-${a.op}`, a.icon]), // полоса «Выровнять»
]

describe('значки главного меню', () => {
  it('у пунктов «Правки» с двойником значок равен значку двойника', () => {
    const m = buildMenu(ctx)
    for (const [id, icon] of TWINS) expect(iconOf(m, id), id).toBe(icon)
  })

  it('«Сделать цветным» держит тот же значок, что «Сделать ч/б»', () => {
    expect(iconOf(buildMenu({ ...ctx, allGray: true }), 'gray')).toBe('gray')
  })

  it('пункты без двойника остаются без значка', () => {
    const edit = items(buildMenu(ctx), 'edit')
    const twin = new Set(TWINS.map(([id]) => id))
    // Значок есть ровно у пунктов таблицы: лишнего не придумано.
    expect(edit.filter((i) => i.icon).map((i) => i.id)).toEqual(edit.filter((i) => twin.has(i.id)).map((i) => i.id))
    // «Выровнять ▸» — полоса без одной кнопки с таким действием; опоры — текстовые кнопки.
    const bare = ['cutItems', 'duplicateItems', 'selectAll', 'clearSelection', 'bgGallery', 'borderGallery', 'copyDecor', 'spreadTpl']
    for (const id of [...bare, 'align', 'alignTo-sel']) expect(edit.find((i) => i.id === id)?.icon, id).toBeUndefined()
  })

  it('«Файл», «Вид» и «Справка» — без значков совсем', () => {
    const m = buildMenu(ctx)
    for (const id of ['file', 'view', 'help']) {
      const list = items(m, id)
      expect(list.length, id).toBeGreaterThan(3)
      expect(
        list.filter((i) => i.icon),
        id,
      ).toEqual([])
    }
  })

  it('столбцы: значки без отметок — один столбец; отметки без значков — как раньше; оба — в «Выровнять»', () => {
    const m = buildMenu(ctx)
    const top = m.find((x) => x.id === 'edit')!.items
    expect([hasIcons(top), hasMarks(top)]).toEqual([true, false])
    const file = m.find((x) => x.id === 'file')!.items
    expect([hasIcons(file), hasMarks(file)]).toEqual([false, true])
    const align = top.find((i) => i.id === 'align')!.sub!
    expect([hasIcons(align), hasMarks(align)]).toEqual([true, true])
  })

  it('двойники в разметке: значок записан у кнопки', () => {
    const rail = text('/src/editor/inspector/EditRail.svelte')
    expect(rail).toContain("icon: 'text'")
    expect(rail).toContain("icon: 'sticker'")
    const bar = text('/src/shell/menu/TopBar.svelte')
    expect(bar).toContain('name="undo"')
    expect(bar).toContain('name="redo"')
    expect(text('/src/editor/inspector/InspectorSpread.svelte')).toContain('name="vignette"')
    const layer = text('/src/editor/inspector/LayerRow.svelte')
    for (const i of ['layer-front', 'layer-up', 'layer-down', 'layer-back']) expect(layer).toContain(`icon: '${i}'`)
  })
})
