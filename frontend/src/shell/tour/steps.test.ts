import { afterEach, beforeEach, describe, expect, it } from 'vitest'
import { dropCatalog, useCatalog, catalogView } from '../../base/keys/keymap.fixture'
import { FEATURES, can, type FeatureId, type Level } from '../../base/prefs/levels'
import { ARTICLES, placesOf } from '../guidetext/articles'
import { articleText } from '../guidetext/articletext'
import { FINAL, INTRO, MORE_IDS, MORE_TOAST, SKIP_TOAST, TOUR } from './steps'

// Стражи шагов тура (113-9): место, статья, уровень, длина текста и метки.
const sources = import.meta.glob('/src/**/*.svelte', { query: '?raw', import: 'default', eager: true }) as Record<string, string>
const anchorIds = (): Set<string> => {
  const out = new Set<string>()
  for (const src of Object.values(sources))
    for (const m of src.matchAll(/(?<![\w-])(?:data-guide="([^"]*)"|'data-guide': '([^']*)')/g))
      (m[1] ?? m[2])
        .split(/\s+/)
        .filter(Boolean)
        .forEach((id) => out.add(id))
  return out
}
const at = (level: Level) => (f: FeatureId) => can(f, level)
const LEVELS: Level[] = ['simple', 'full']
const FEATURE_IDS = new Set<string>(FEATURES.flatMap((f) => [f.id, ...(f.also ?? [])]))

beforeEach(() => useCatalog())
afterEach(() => dropCatalog())

describe('шаги тура: стражи', () => {
  it('место — литерал data-guide в разметке; пустое место тоже', () => {
    const have = anchorIds()
    for (const s of TOUR)
      for (const spot of [s.spot, s.emptySpot]) if (spot) expect(have.has(spot.replace(/^ui:/, '')), `${s.id}: ${spot}`).toBe(true)
  })

  // 122-4: avoid — непустые строки (что querySelector их принимает, проверяет tourfix.dom.test); якорь группы «Авто» шага 5 есть в разметке.
  it('avoid: непустые селекторы; шаг 5 обводит группу layout-auto', () => {
    for (const s of TOUR) for (const sel of s.avoid ?? []) expect(sel.trim().length, `${s.id}: пустой селектор`).toBeGreaterThan(0)
    expect(TOUR.find((s) => s.id === 'folders')?.avoid).toEqual(['[data-guide~="library"]'])
    expect(TOUR.find((s) => s.id === 'layouts')?.avoid).toEqual(['[data-current] > .spread'])
    expect(TOUR.find((s) => s.id === 'layouts')?.spot).toBe('ui:layout-auto')
    expect(anchorIds().has('layout-auto')).toBe(true)
  })

  it('место только ui:, id шагов уникальны, заголовок ≤ 32 знаков', () => {
    expect(new Set(TOUR.map((s) => s.id)).size).toBe(TOUR.length)
    for (const s of TOUR) {
      expect(s.spot.startsWith('ui:'), s.id).toBe(true)
      expect(s.title.length, s.id).toBeLessThanOrEqual(32)
    }
    for (const id of MORE_IDS)
      expect(
        TOUR.some((s) => s.id === id),
        id,
      ).toBe(true)
  })

  it('статья шага есть в справочнике', () => {
    for (const s of TOUR)
      expect(
        ARTICLES.some((a) => a.id === s.article),
        `${s.id}: ${s.article}`,
      ).toBe(true)
  })

  it('на «Простом» есть все базовые шаги: тур не зависит от уровня', () => {
    const simple = TOUR.filter((s) => can(s.feature, 'simple'))
    expect(simple.length).toBeGreaterThanOrEqual(5)
    for (const id of ['folders', 'library', 'autobuild', 'timeline', 'export'])
      expect(
        simple.some((s) => s.id === id),
        id,
      ).toBe(true)
  })

  it('текст живого места ≤ 200 знаков после articleText на «Простом» и «Полном»; пустого — ≤ 260', () => {
    for (const lv of LEVELS)
      for (const s of TOUR) {
        expect(articleText(s.ready, at(lv)).length, `${s.id}/${lv}: ready`).toBeLessThanOrEqual(200)
        expect(articleText(s.empty, at(lv)).length, `${s.id}/${lv}: empty`).toBeLessThanOrEqual(260)
      }
  })

  it('метки {key} и {can} называют известные id, без остатков разметки и {@html', () => {
    const keys = new Set(catalogView.commands.map((e) => e.id))
    const texts = [...TOUR.flatMap((s) => [s.ready, s.empty]), INTRO.text, FINAL.text, SKIP_TOAST, MORE_TOAST]
    for (const t of texts) {
      for (const m of t.matchAll(/\{key:([A-Za-z0-9]+)/g)) expect(keys.has(m[1]), `key ${m[1]}`).toBe(true)
      for (const m of t.matchAll(/\{can:([A-Za-z0-9]+)\|/g)) expect(FEATURE_IDS.has(m[1]), `can ${m[1]}`).toBe(true)
      for (const lv of LEVELS) expect(articleText(t, at(lv)).replace('{n}', '7'), t).not.toMatch(/[{}]/)
    }
    const html = Object.entries(sources).filter(([p, src]) => p.includes('/shell/tour/') && src.includes('{@html'))
    expect(html.map(([p]) => p)).toEqual([])
  })

  it('статья «Первые шаги и видео» ведёт к пунктам меню; у «Экспорта» есть место в шапке', () => {
    const a = ARTICLES.find((x) => x.id === 'first-steps')!
    expect(placesOf(a).map((p) => p.spot)).toEqual(['menu:tour', 'menu:videos'])
    expect(placesOf(ARTICLES.find((x) => x.id === 'export')!).map((p) => p.spot)).toContain('ui:topbar-export')
  })
})
