import { describe, expect, it } from 'vitest'
import { ACTS } from './photoacts'
import { photoMenu, type MenuCtx, type MenuItem } from './photomenu'

// Контекст как в photomenu.test.ts: одно фото в месте, всё доступно.
const base: MenuCtx = {
  grid: true,
  places: 1,
  texts: 0,
  stickers: 0,
  photos: 1,
  missing: 0,
  borderLost: 0,
  differ: true,
  allContain: false,
  allCover: true,
  allRect: true,
  allRounded: false,
  allOval: false,
  allGray: false,
  framed: true,
  copyable: true,
  pasteable: true,
  allFlipH: false,
  allFlipV: false,
  tiltOk: true,
  scopes: [
    { scope: 'spread', label: 'Весь разворот' },
    { scope: 'left', label: 'Левая страница' },
  ],
  uses: true,
}
const flat = (m: MenuItem[]): MenuItem[] => m.flatMap((i) => [i, ...(i.sub ? flat(i.sub) : [])])
const full = (c: Partial<MenuCtx> = {}) => photoMenu({ ...base, ...c })

describe('photoMenu: значки (0.25-menu-icons)', () => {
  it('у каждого пункта первого и второго уровня есть значок', () => {
    for (const m of [full(), full({ grid: false, missing: 1 }), full({ places: 2, photos: 2 })]) {
      expect(m.length).toBeGreaterThan(8)
      for (const i of flat(m)) expect(i.icon, `${i.id}:${i.arg ?? ''}`).toBeTruthy()
    }
  })
  it('группы — значок главного действия; «Слой ▸» — значки «Расположения» правой панели', () => {
    const m = full()
    const icon = (id: string) => m.find((i) => i.id === id)?.icon
    expect(icon('turn')).toBe('turn-cw')
    expect(icon('mirror')).toBe('flip-h')
    expect(icon('layer')).toBe('layers')
    expect(m.find((i) => i.id === 'layer')?.sub?.map((i) => i.icon)).toEqual(['layer-front', 'layer-up', 'layer-down', 'layer-back'])
  })
  it('подпункты «Форма» — три разных значка', () => {
    const sub = full()
      .find((i) => i.id === 'shape')
      ?.sub?.filter((i) => ['rect', 'rounded', 'oval'].includes(i.id))
    expect(new Set(sub?.map((i) => i.icon)).size).toBe(3)
  })
  it('меню стикера и надписи — тоже со значками', () => {
    for (const c of [{ stickers: 1 }, { texts: 1 }]) {
      const items = flat(full({ places: 0, photos: 0, ...c }))
      expect(items.length).toBeGreaterThan(2)
      for (const i of items) expect(i.icon, i.id).toBeTruthy()
    }
  })
})

describe('ACTS: тот же значок — у того же действия (0.25-mainmenu-icons)', () => {
  it('«Рамочка…», «Где ещё стоит», «Заменить…», «Копировать имя файла» — свои значки, не чужие', () => {
    expect(ACTS.border.icon).not.toBe(ACTS.contain.icon)
    expect(ACTS.uses.icon).not.toBe(ACTS.preview.icon)
    expect(ACTS.replace.icon).not.toBe(ACTS.swap.icon)
    expect(ACTS.copy.icon).not.toBe(ACTS.clipCopy.icon)
  })
  it('общий значок — только у вариантов одного действия', () => {
    const byIcon = new Map<string, string[]>()
    for (const [id, a] of Object.entries(ACTS)) byIcon.set(a.icon ?? '', [...(byIcon.get(a.icon ?? '') ?? []), id])
    const shared = [...byIcon.values()].filter((ids) => ids.length > 1).map((ids) => ids.join('+'))
    expect(shared.sort()).toEqual(['caption+name', 'fit+contain'].sort())
  })
})
