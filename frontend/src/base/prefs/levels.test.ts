import { describe, expect, it } from 'vitest'
import {
  ALL_LEVELS,
  FEATURES,
  LEVELS,
  LEVEL_HINT,
  LEVEL_POINTS,
  LEVEL_TITLE,
  customPoints,
  blockedText,
  featureGroups,
  blocker,
  can,
  minLevel,
  normLevel,
  type FeatureId,
} from './levels'

const ALL: FeatureId[] = FEATURES.flatMap((f) => [f.id, ...(f.also ?? [])])

// Страж: уровень каждого id до деления строк (спека 011 §2.1) — ~30 гейтов prefs.can на них завязаны.
const LEGACY_MIN: Record<FeatureId, 'simple' | 'standard' | 'full'> = {
  base: 'simple',
  variants: 'simple',
  timeline: 'simple',
  captions: 'simple',
  bgColor: 'simple',
  menu: 'simple',
  turn: 'simple',
  text: 'standard',
  photoMenu: 'standard',
  frame: 'standard',
  flip: 'standard',
  tilt: 'standard',
  zoom: 'standard',
  names: 'standard',
  allSpreads: 'standard',
  groupSelect: 'standard',
  bgPhoto: 'standard',
  manual: 'full',
  rotate: 'full',
  groupXf: 'full',
  layers: 'full',
  align: 'full',
  rulers: 'full',
  templates: 'full',
  textRotate: 'full',
  panels: 'full',
  marks: 'simple', // спека 004
  vignette: 'standard', // 031: на «Простом» пунктов виньетки нет (решение 80)
  decor: 'standard', // 048: галерея фонов и стикеров; опора — фото на фоне
}

describe('featureGroups', () => {
  it('собирает все функции по группам, ничего не теряя', () => {
    const g = featureGroups()
    expect(g.map((x) => x.title)).toEqual(['Основа', 'Фото и текст', 'Холст', 'Ручная раскладка'])
    expect(g.flatMap((x) => x.rows)).toHaveLength(FEATURES.length)
  })
})

describe('таблица уровней', () => {
  it('страж: поведение трёх уровней для прежних id не изменилось', () => {
    expect(Object.keys(LEGACY_MIN).sort()).toEqual([...ALL].sort())
    for (const id of ALL) {
      expect(minLevel(id), id).toBe(LEGACY_MIN[id])
      for (const l of LEVELS) expect(can(id, l), `${id}@${l}`).toBe(LEVELS.indexOf(l) >= LEVELS.indexOf(LEGACY_MIN[id]))
    }
  })
  it('26 строк, у каждой краткое имя без повторов', () => {
    expect(FEATURES).toHaveLength(26)
    const shorts = FEATURES.map((f) => f.short)
    expect(new Set(shorts).size).toBe(shorts.length)
    for (const s of shorts) expect(s.trim(), s).not.toBe('')
  })
  it('одна строка «Масштаб и поворот группы» (спека 018): id прежний, уровни — с «Полного»', () => {
    const f = FEATURES.find((x) => x.id === 'groupXf')!
    expect(f.title).toBe('Масштаб и поворот группы')
    expect(f.short).toBe('Масштаб и поворот группы')
    expect(can('groupXf', 'full')).toBe(true)
    expect(can('groupXf', 'simple')).toBe(false)
  })
  it('уровни замкнуты по опорам: опоры функции есть на том же уровне', () => {
    for (const f of FEATURES)
      for (const n of f.needs ?? []) {
        expect(
          FEATURES.some((x) => x.id === n),
          `${f.id} → ${n}`,
        ).toBe(true)
        for (const l of LEVELS) if (can(f.id, l)) expect(can(n, l), `${f.id}@${l} нужна ${n}`).toBe(true)
      }
  })
  it('опоры — существующие строки без циклов; у основы опор нет, на неё никто не ссылается', () => {
    const dep = new Map(FEATURES.map((f) => [f.id, f.needs ?? []]))
    const visit = (id: FeatureId, path: FeatureId[]) => {
      expect(path, `цикл через ${id}`).not.toContain(id)
      for (const n of dep.get(id) ?? []) visit(n, [...path, id])
    }
    for (const f of FEATURES) visit(f.id, [])
    for (const id of ['base', 'menu'] as const) {
      expect(dep.get(id), id).toEqual([])
      for (const f of FEATURES) expect(f.needs ?? [], `${f.id} → ${id}`).not.toContain(id)
    }
  })
  it('на строки «Простого» никто не опирается', () => {
    const simple = FEATURES.filter((f) => f.min === 'simple').map((f) => f.id)
    for (const f of FEATURES) for (const n of f.needs ?? []) expect(simple, `${f.id} → ${n}`).not.toContain(n)
  })
  it('пункты карточек ≤ 26 знаков', () => {
    for (const l of ALL_LEVELS) for (const p of LEVEL_POINTS[l]) expect(p.length, p).toBeLessThanOrEqual(26)
    for (const b of LEVELS) for (const p of customPoints(b)) expect(p.length, p).toBeLessThanOrEqual(26)
    expect(customPoints('standard').at(-1)).toBe('Основа сейчас: Стандартный')
  })
  it('обложка и виньетка названы в карточках, подсказке и строке «Основа» (035 U10)', () => {
    expect(LEVEL_POINTS.simple).toContain('Обложка')
    expect(LEVEL_POINTS.simple).not.toContain('Поворот фото на 90°')
    expect(LEVEL_POINTS.standard).toContain('Виньетка в альбоме')
    expect(LEVEL_POINTS.standard).not.toContain('Зум и группа фото')
    expect(LEVEL_HINT.simple).toContain('обложка')
    expect(FEATURES.find((f) => f.id === 'base')?.title).toContain('обложка')
  })
  it('у каждой функции один главный id и нет повторов среди всех', () => {
    expect(new Set(ALL).size).toBe(ALL.length)
  })
  it('уровни вложены: что доступно ниже, доступно и выше', () => {
    for (const id of ALL) {
      if (can(id, 'simple')) expect(can(id, 'standard'), id).toBe(true)
      if (can(id, 'standard')) expect(can(id, 'full'), id).toBe(true)
      expect(can(id, 'full'), id).toBe(true)
    }
  })
  it('«Простой» — основа, подписи, цвет фона, меню, ↻ 90° и отметки фото (вопрос 2; 004)', () => {
    for (const id of ['base', 'variants', 'timeline', 'captions', 'bgColor', 'menu', 'turn', 'marks'] as const)
      expect(can(id, 'simple'), id).toBe(true)
  })
  it('«Стандартный» добавляет текст, кадр, зеркало, зум, группу, фото фона и меню правой кнопки', () => {
    for (const id of [
      'text',
      'photoMenu',
      'frame',
      'flip',
      'tilt',
      'zoom',
      'names',
      'allSpreads',
      'groupSelect',
      'bgPhoto',
      'vignette',
      'decor',
    ] as const) {
      expect(can(id, 'simple'), id).toBe(false)
      expect(can(id, 'standard'), id).toBe(true)
    }
  })
  it('«Полный» добавляет ручную раскладку, слои, выравнивание, линейки, шаблоны и плавающие панели', () => {
    for (const id of ['manual', 'rotate', 'groupXf', 'layers', 'align', 'rulers', 'templates', 'textRotate', 'panels'] as const) {
      expect(can(id, 'standard'), id).toBe(false)
      expect(can(id, 'full'), id).toBe(true)
    }
  })
  it('minLevel совпадает с can на границах', () => {
    for (const id of ALL) {
      const min = minLevel(id)
      for (const l of LEVELS) expect(can(id, l), `${id}@${l}`).toBe(LEVELS.indexOf(l) >= LEVELS.indexOf(min))
    }
  })
  it('у уровней есть названия по-русски', () => {
    expect(LEVELS.map((l) => LEVEL_TITLE[l])).toEqual(['Простой', 'Стандартный', 'Полный'])
    expect(ALL_LEVELS.map((l) => LEVEL_TITLE[l])).toEqual(['Простой', 'Стандартный', 'Полный', 'Свой'])
  })
})

describe('normLevel', () => {
  it('неизвестное — «Простой»', () => {
    expect(normLevel('full')).toBe('full')
    expect(normLevel('standard')).toBe('standard')
    expect(normLevel('')).toBe('simple')
    expect(normLevel(undefined)).toBe('simple')
    expect(normLevel('expert')).toBe('simple')
    expect(normLevel('custom')).toBe('custom')
  })
})

describe('blocker (вопрос 5)', () => {
  it('текст в «Простом» требует «Стандартный»', () => {
    expect(blocker('text', 'simple')).toBe('standard')
    expect(blocker('text', 'standard')).toBeNull()
    expect(blocker('text', 'full')).toBeNull()
  })
  it('место «Ручной» ниже «Полного» требует «Полный»', () => {
    expect(blocker('place', 'simple')).toBe('full')
    expect(blocker('place', 'standard')).toBe('full')
    expect(blocker('place', 'full')).toBeNull()
  })
  it('стикер (048) — как текст: ниже «Стандартного» требует «Стандартный»', () => {
    expect(blocker('sticker', 'simple')).toBe('standard')
    expect(blocker('sticker', 'standard')).toBeNull()
    expect(blocker('sticker', 'full')).toBeNull()
  })
  it('готовая подпись правится на всех готовых уровнях', () => {
    for (const l of LEVELS) expect(blocker('caption', l)).toBeNull()
  })
  it('текст тоста называет уровень', () => {
    expect(blockedText('standard')).toBe('Править — в уровне «Стандартный»')
    expect(blockedText('full')).toBe('Править — в уровне «Полный»')
  })
})
