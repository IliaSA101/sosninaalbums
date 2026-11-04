import { afterEach, beforeEach, describe, expect, it } from 'vitest'
import { dropCatalog, useCatalog } from '../../base/keys/keymap.fixture'
import { ALL_LEVELS, can, type Custom, type FeatureId } from '../../base/prefs/levels'
import { features } from '../../base/prefs/levelcustom'
import { buildMenu, type MenuCtx } from './menu'

beforeEach(() => useCatalog())
afterEach(() => dropCatalog())

const base: MenuCtx = {
  spreads: 0,
  hasSpread: false,
  canUndo: false,
  canRedo: false,
  zone: 'canvas',
  libraryPhotos: 0,
  anySelection: false,
  deletable: false,
  flippable: false,
  varyable: false,
  layerable: false,
  missing: 0,
  big: false,
  details: false,
  safe: false,
  rulers: false,
  snap: false,
}
const helpOf = (c: Partial<MenuCtx>) => buildMenu({ ...base, ...c }).find((m) => m.id === 'help')!.items

describe('«Справка ▸ О программе…» (101-6)', () => {
  it('последний пункт «Справки» на всех уровнях, команда about', () => {
    for (const level of ALL_LEVELS) {
      const custom: Custom = { base: 'simple', on: [], off: ['variants', 'timeline', 'captions', 'turn'] }
      const can_ = level === 'custom' ? (f: FeatureId) => features('custom', custom).has(f) : (f: FeatureId) => can(f, level)
      const help = helpOf({ level, can: can_ })
      expect(help.at(-1), level).toMatchObject({ id: 'about', label: 'О программе…', cmd: 'about' })
      expect(help.at(-1)?.disabled, level).toBeFalsy()
    }
  })
  it('без открытого альбома пункт тоже доступен; перед ним — «Что нового»', () => {
    const help = helpOf({})
    expect(help.at(-1)).toMatchObject({ id: 'about' })
    expect(help.at(-1)?.disabled).toBeFalsy()
    expect(help.at(-2)?.id).toBe('whatsNew')
  })
})

describe('«Справка ▸ Первые шаги» и «Видео о программе» (113)', () => {
  it('«Первые шаги» вторым, «Видео о программе» третьим — на всех уровнях и без функций', () => {
    for (const level of ALL_LEVELS) {
      const custom: Custom = { base: 'simple', on: [], off: ['variants', 'timeline', 'captions', 'turn'] }
      const can_ = level === 'custom' ? (f: FeatureId) => features('custom', custom).has(f) : (f: FeatureId) => can(f, level)
      for (const help of [helpOf({ level, can: can_ }), helpOf({ level, can: () => false })]) {
        expect(help[0].id, level).toBe('guide')
        expect(help[1], level).toMatchObject({ id: 'tour', label: 'Первые шаги', cmd: 'tour' })
        expect(help[2], level).toMatchObject({ id: 'videos', label: 'Видео о программе', cmd: 'videos' })
        expect(help[1].disabled, level).toBeFalsy()
      }
    }
  })
})
