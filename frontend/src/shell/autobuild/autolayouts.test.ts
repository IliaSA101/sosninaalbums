import { describe, expect, it } from 'vitest'
import type { AutoPreview, Choice, Family } from '../../base/core/types'
import { model } from '../../../wailsjs/go/models'
import { unitOf } from '../../base/geom/sheet'
import { FAMILIES } from '../../panels/gallery/variants'
import { FAMILY_IDS, normFamilies } from '../../base/prefs/families'
import {
  autoRows,
  emptyColumn,
  FAMILY_GEN,
  fallbackNote,
  familiesHint,
  layoutsStatus,
  layoutsWaiting,
  shownFamilies,
  spreadTitle,
  toggleFamily,
  twinKey,
} from './autolayouts'

const ch = (key: string, family: Family, twin = -1): Choice => ({ key, rects: [], family, twin, formName: key, crop: 0 })
const sample = (photos: number, inOrder: Choice[], extra: Choice[] = []) =>
  model.SampleRow.createFrom({ photos, group: photos, inOrder, extra })

describe('toggleFamily (сценарий 6)', () => {
  it('«Все» сбрасывает набор', () => {
    expect(toggleFamily(['dense', 'hero'], 'all')).toEqual([])
    expect(toggleFamily([], 'all')).toEqual([])
  })
  it('из «Все» щелчок оставляет одно семейство', () => {
    expect(toggleFamily([], 'dense')).toEqual(['dense'])
  })
  it('второй щелчок добавляет, порядок — порядок сегментов', () => {
    expect(toggleFamily(['dense'], 'hero')).toEqual(['dense', 'hero'])
    expect(toggleFamily(['strip'], 'dense')).toEqual(['dense', 'strip'])
  })
  it('повторный щелчок снимает; снял последнее — «Все»', () => {
    expect(toggleFamily(['dense', 'hero'], 'hero')).toEqual(['dense'])
    expect(toggleFamily(['dense'], 'dense')).toEqual([])
  })
  it('нажаты все шесть — снова «Все»', () => {
    expect(toggleFamily(['dense', 'hero', 'air', 'bleed', 'strip'], 'figure')).toEqual([])
  })
  it('выбор не схлопывается, пока нажаты не все шесть (скрытые сегменты не теряются)', () => {
    expect(toggleFamily(['dense', 'hero', 'air', 'strip'], 'figure')).toEqual(['dense', 'hero', 'air', 'strip', 'figure'])
    expect(toggleFamily(['dense', 'hero'], 'air')).toEqual(['dense', 'hero', 'air'])
  })
  it('пустого выбора не бывает ни на каком щелчке', () => {
    for (const id of FAMILY_IDS) {
      for (const cur of [[], ['dense'], ['hero', 'strip'], [...FAMILY_IDS]] as Family[][]) {
        const r = toggleFamily(cur, id)
        expect(r.length === 0 || r.length < FAMILY_IDS.length).toBe(true)
      }
    }
  })
})

describe('normFamilies', () => {
  it('мусор, повторы и все шесть — «Все»', () => {
    expect(normFamilies(undefined)).toEqual([])
    expect(normFamilies('dense')).toEqual([])
    expect(normFamilies(['мусор', 7, null])).toEqual([])
    expect(normFamilies(['strip', 'dense', 'dense'])).toEqual(['dense', 'strip'])
    expect(normFamilies([...FAMILY_IDS].reverse())).toEqual([])
  })
})

describe('shownFamilies (сценарий 6, как familiesIn у полосы)', () => {
  const pv = { samples: [sample(3, [ch('a', 'dense'), ch('b', 'hero')], [ch('c', 'air')]), sample(4, [ch('d', 'strip')])] }
  it('семейства «Вариантов» окна в порядке сегментов', () => {
    expect(shownFamilies(pv, [])).toEqual(['dense', 'hero', 'air', 'strip'])
  })
  it('выбранное не прячется, даже если его нет в вариантах', () => {
    expect(shownFamilies(pv, ['figure'])).toEqual(['dense', 'hero', 'air', 'strip', 'figure'])
  })
  it('нет ответа (ошибка, ещё не считалось) — все шесть: снятый сегмент не пропадает', () => {
    expect(shownFamilies(null, ['bleed'])).toEqual(FAMILY_IDS)
    expect(shownFamilies(null, [])).toEqual(FAMILY_IDS)
  })
})

describe('familiesHint (сценарий 7)', () => {
  it('«Все»', () => {
    expect(familiesHint([])).toBe('Автосборка выбирает из всех раскладок')
  })
  it('перечень в порядке сегментов', () => {
    expect(familiesHint(['hero', 'dense'])).toBe('Только: Плотно, С главным фото')
  })
  it('запасные: склонение «развороте / разворотах»', () => {
    expect(fallbackNote(1)).toBe('на 1 развороте таких нет — там будет лучшая из всех')
    expect(fallbackNote(7)).toBe('на 7 разворотах таких нет — там будет лучшая из всех')
    expect(fallbackNote(21)).toContain('на 21 развороте')
    expect(fallbackNote(12)).toContain('на 12 разворотах')
  })
  it('лист виньетки: «листе / листах»', () => {
    const u = unitOf({ sheet: true })
    expect(fallbackNote(1, u)).toBe('на 1 листе таких нет — там будет лучшая из всех')
    expect(fallbackNote(3, u)).toBe('на 3 листах таких нет — там будет лучшая из всех')
    expect(fallbackNote(0, u)).toBe('')
  })
})

describe('layoutsWaiting', () => {
  it('бледнеем при плане, предпросмотре и «от» > «до»', () => {
    expect(layoutsWaiting({ pending: false, previewBusy: false, inverted: false })).toBe(false)
    expect(layoutsWaiting({ pending: true, previewBusy: false, inverted: false })).toBe(true)
    expect(layoutsWaiting({ pending: false, previewBusy: true, inverted: false })).toBe(true)
    expect(layoutsWaiting({ pending: false, previewBusy: false, inverted: true })).toBe(true)
  })
})

describe('layoutsStatus', () => {
  it('ошибка главнее, потом нет фото, потом ожидание; иначе пусто (покажут familiesHint)', () => {
    expect(layoutsStatus({ error: true, empty: true, waiting: true })).toBe('Не удалось показать раскладки')
    expect(layoutsStatus({ error: false, empty: true, waiting: true })).toBe('Нет фото для сборки')
    expect(layoutsStatus({ error: false, empty: false, waiting: true })).toBe('Считаю…')
    expect(layoutsStatus({ error: false, empty: false, waiting: false })).toBe('')
  })
})

describe('autoRows («Варианты»)', () => {
  const pv: Pick<AutoPreview, 'samples'> = {
    samples: [sample(3, [ch('a', 'dense'), ch('b', 'hero'), ch('c', 'hero', 1)], [ch('d', 'air')]), sample(4, [ch('e', 'strip')])],
  }
  it('части по числу фото; «Авто», за чертой «другие»; числа — плитки после фильтра', () => {
    const r = autoRows(pv, [], [])
    expect(r.parts.map((p) => p.photos)).toEqual([3, 4])
    expect(r.count).toBe(5)
    const p3 = r.parts[0].row.parts
    expect(p3.map((p) => [p.line, p.sep])).toEqual([
      ['В порядке ленты', ''],
      ['Другие варианты', 'другие'],
    ])
    // c — близнец b: свёрнут в «Похожие: 1»
    expect(p3[0].shown.map((c) => c.key)).toEqual(['a', 'b'])
    expect(p3[0].hidden).toBe(1)
  })
  it('набор семейств прячет чужие; числа учитывают фильтр', () => {
    const r = autoRows(pv, ['dense', 'air'], [])
    expect(r.count).toBe(2)
    expect(r.parts[0].row.parts.flatMap((p) => p.shown.map((c) => c.key))).toEqual(['a', 'd'])
    expect(r.parts[1].row.parts).toEqual([]) // на 4 фото таких нет — там будет лучшая
  })
  it('«Похожие» раскрываются по ключу части', () => {
    const r = autoRows(pv, [], [twinKey(3, 'В порядке ленты')])
    expect(r.parts[0].row.parts[0].shown.map((c) => c.key)).toEqual(['a', 'b', 'c'])
    // раскрытие одной части не трогает другие
    expect(autoRows(pv, [], [twinKey(4, 'В порядке ленты')]).parts[0].row.parts[0].shown.map((c) => c.key)).toEqual(['a', 'b'])
  })
  it('нет образцов — пусто', () => {
    expect(autoRows({ samples: [] }, ['dense'], [])).toEqual({ parts: [], count: 0 })
  })
})

describe('spreadTitle', () => {
  it('форма; у запасной — почему она здесь', () => {
    expect(spreadTitle({ formName: 'Лента', fallback: false })).toBe('Лента')
    expect(spreadTitle({ formName: '', fallback: false })).toBeUndefined()
    expect(spreadTitle({ formName: 'Плотно', fallback: true })).toBe('Плотно — нет раскладок выбранных семейств, взята лучшая')
  })
  it('подрезка (104): «подрезка до N %» после формы; 0 — без хвоста; у запасной — перед причиной', () => {
    expect(spreadTitle({ formName: 'Плотно', crop: 2, fallback: false })).toBe('Плотно · подрезка до 2 %')
    expect(spreadTitle({ formName: 'Плотно', crop: 0, fallback: false })).toBe('Плотно')
    expect(spreadTitle({ formName: 'Плотно', crop: 3, fallback: true })).toBe(
      'Плотно · подрезка до 3 % — нет раскладок выбранных семейств, взята лучшая',
    )
  })
})

describe('emptyColumn: пустая колонка при фильтре', () => {
  it('таблица слов есть для каждого семейства', () => {
    expect(Object.keys(FAMILY_GEN).sort()).toEqual(FAMILIES.map((f) => f.id).sort())
    expect(FAMILY_GEN.dense).toBe('Плотных')
    expect(FAMILY_GEN.hero).toBe('С главным фото')
    expect(FAMILY_GEN.air).toBe('С воздухом')
    expect(FAMILY_GEN.strip).toBe('Лент')
    expect(FAMILY_GEN.figure).toBe('Фигур')
  })
  it('одно семейство — его имя', () => {
    const e = emptyColumn(['dense'], 4)
    expect(e.main).toBe('Плотных на 4 фото нет')
    expect(e.sub).toBe('возьмём лучшую из всех')
    expect(e.title).toBe(
      'В выбранных раскладках нет вариантов на 4 фото — на таких разворотах автосборка поставит лучшую раскладку из всех',
    )
  })
  it('несколько семейств — «Таких раскладок»', () => {
    expect(emptyColumn(['dense', 'air'], 2).main).toBe('Таких раскладок на 2 фото нет')
  })
})
