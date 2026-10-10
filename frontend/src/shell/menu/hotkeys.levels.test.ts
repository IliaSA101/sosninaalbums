import { describe, expect, it } from 'vitest'
import { FEATURES, can, type FeatureId } from '../../base/prefs/levels'
import { HOTKEYS, visibleHotkeys } from './hotkeys'

const ALL: FeatureId[] = FEATURES.flatMap((f) => [f.id, ...(f.also ?? [])])

describe('клавиши и справка по уровням', () => {
  const canAt = (l: 'simple' | 'standard' | 'full') => (f: FeatureId) => can(f, l)
  it('справка «Простого» без зума, текста и ручной раскладки', () => {
    const shows = visibleHotkeys(canAt('simple')).map((h) => h.show)
    for (const s of ['Ctrl+=', '0 / Ctrl+0', 'Пробел + мышь', 'Двойной щелчок по тексту', 'Shift+клик по фото', '← ↑ ↓ → (ручная)'])
      expect(shows, s).not.toContain(s)
    expect(shows).toContain('Ctrl+Z')
    expect(shows).toContain('F')
  })
  it('справка «Полного» — вся таблица без служебных строк', () => {
    expect(visibleHotkeys(canAt('full'))).toEqual(HOTKEYS.filter((h) => !h.hidden))
    expect(visibleHotkeys()).toEqual(HOTKEYS.filter((h) => !h.hidden))
  })
  it('каждая feature строки таблицы клавиш есть в таблице уровней', () => {
    for (const h of HOTKEYS) if (h.feature) expect(ALL, h.show).toContain(h.feature)
  })
})
