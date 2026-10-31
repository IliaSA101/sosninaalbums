import { describe, expect, it } from 'vitest'
import {
  applyQuestion,
  applyToast,
  BG_NOTE,
  cardNote,
  clampSpreads,
  defaultSpreads,
  effMode,
  fitText,
  formatText,
  lookKey,
  lookRatio,
  tileRatio,
  moveStep,
  needsAsk,
  placesText,
  stepAskTitle,
  stepLabel,
  stepWhy,
  strip,
  tileStep,
} from './looks'
import { themeItems } from '../../panels/decor/decor'
import type { LookCatalog, LookInfo, LookItem } from './looktypes'

const it1 = (o: Partial<LookItem>): LookItem => ({ id: 'my:aa', name: 'A', theme: 'w', url: '', own: false, aspect: 0, w: 0, h: 0, ...o })
const info = (n: number, o: Partial<LookInfo> = {}): LookInfo => ({
  steps: Array.from({ length: n }, (_, k) => ({ name: `Ш${k + 1}`, role: '' as const, places: 2, pages: 2 as const })),
  cover: false,
  min: 1,
  max: 200,
  ...o,
})

describe('looks: плитки и подписи', () => {
  const cat: LookCatalog = {
    kind: 'album',
    themes: [],
    items: [it1({ id: 'my:a1', theme: 'w' }), it1({ id: 'my:b2', theme: 's' }), it1({ id: 'my:c3', theme: 'w' })],
    recent: ['my:c3', 'my:gone'],
    readOnly: false,
  }
  it('тема, «Все» и «Недавние» (пропавшие записи молчат)', () => {
    expect(themeItems(cat, 'w').map((i) => i.id)).toEqual(['my:a1', 'my:c3'])
    expect(themeItems(cat, 'all')).toHaveLength(3)
    expect(themeItems(cat, 'recent').map((i) => i.id)).toEqual(['my:c3'])
    expect(themeItems(null, 'all')).toEqual([])
  })
  it('подпись плитки: «от 26 фото · 7 разворотов», чего нет — того нет', () => {
    expect(cardNote({ from: 26, steps: 7 })).toBe('от 26 фото · 7 разворотов')
    expect(cardNote({ from: 18, steps: 2 })).toBe('от 18 фото · 2 разворота')
    expect(cardNote({ steps: 1 })).toBe('1 разворот')
    expect(cardNote({})).toBe('')
  })
  it('плитка показывает лицевую обложки, нет её — разворот 1', () => {
    expect(tileStep({ cover: true })).toBe(0)
    expect(tileStep({ cover: false })).toBe(1)
    expect(tileStep({})).toBe(1)
  })
  it('ключ картинки — hex после префикса записи', () => {
    expect(lookKey('my:0a1b2c3d4e5f6071')).toBe('0a1b2c3d4e5f6071')
    expect(lookKey('0a1b2c3d4e5f6071')).toBe('0a1b2c3d4e5f6071')
    for (const bad of ['0a1b', 'my:', 'my:0A1B2C3D4E5F6071', 'my:0a1b2c3d4e5f607', 'my:../../etc/passwd'])
      expect(lookKey(bad), bad).toBe('')
  })
})

describe('looks: число разворотов', () => {
  it('зажим в пределах Go; мусор — нижний предел', () => {
    const i = info(3, { min: 2, max: 20 })
    expect(clampSpreads(9, i)).toBe(9)
    expect(clampSpreads(1, i)).toBe(2)
    expect(clampSpreads(99, i)).toBe(20)
    expect(clampSpreads(NaN, i)).toBe(2)
    expect(clampSpreads(4.7, i)).toBe(4)
    expect(clampSpreads(5, null)).toBe(5)
  })
  it('по умолчанию — все шаги по разу, в пределах', () => {
    expect(defaultSpreads(info(9))).toBe(9)
    expect(defaultSpreads(info(1, { min: 3, max: 8 }))).toBe(3)
  })
})

describe('looks: лента шагов', () => {
  it('обложка (если есть) и шаги; i совпадает с параметром картинки', () => {
    const i = info(3, { cover: true })
    i.steps[1].role = 'repeat'
    expect(strip(i)).toEqual([
      { i: 0, label: 'Обложка', num: 0, role: '' },
      { i: 1, label: 'Ш1', num: 1, role: '' },
      { i: 2, label: 'Ш2', num: 2, role: 'repeat' },
      { i: 3, label: 'Ш3', num: 3, role: '' },
    ])
    expect(strip(info(2)).map((c) => c.i)).toEqual([1, 2])
    expect(strip(null)).toEqual([])
  })
  it('‹ › идут по ленте без заворота; чужой шаг — первый', () => {
    const c = strip(info(3, { cover: true }))
    expect(moveStep(c, 0, -1)).toBe(0)
    expect(moveStep(c, 0, 1)).toBe(1)
    expect(moveStep(c, 3, 1)).toBe(3)
    expect(moveStep(c, 99, 1)).toBe(0)
    expect(moveStep([], 5, 1)).toBe(5)
  })
})

describe('looks: режим и вопрос', () => {
  it('«Что сделать» действует только в собранном альбоме', () => {
    expect(effMode({ full: false }, 'bg')).toBe('series')
    expect(effMode(null, 'add')).toBe('series')
    expect(effMode({ full: true }, 'bg')).toBe('bg')
    expect(effMode({ full: true }, 'add')).toBe('add')
  })
  it('вопрос — только если заменится своё или фото уйдут в ленту', () => {
    expect(needsAsk({ touched: 0, replaced: 0, left: 0 })).toBe(false)
    expect(needsAsk({ touched: 2, replaced: 0, left: 0 })).toBe(true)
    expect(needsAsk({ touched: 0, replaced: 3, left: 0 })).toBe(true)
    expect(needsAsk({ touched: 0, replaced: 0, left: 4 })).toBe(true) // фото уйдут в ленту
  })
})

describe('looks: подписи формата и подбора', () => {
  it('формат без «см», мест и подбор со склонением', () => {
    expect(formatText({ pageW: 300, pageH: 300, sheet: false })).toBe('30×30')
    expect(formatText({ pageW: 205, pageH: 290, sheet: false })).toBe('20,5×29')
    expect(placesText(46)).toBe('мест 46')
    expect(placesText(null)).toBe('')
    expect(fitText(64)).toBe('Подобрать под 64 выбранных фото')
    expect(fitText(1)).toBe('Подобрать под 1 выбранное фото')
    expect(fitText(64, true)).toBe('Подобрать под 64 выбранных фото')
    expect(fitText(64, false)).toBe('Подобрать под 64 свободных фото')
    expect(fitText(1, false)).toBe('Подобрать под 1 свободное фото')
    expect(BG_NOTE).toContain('Поставить на разворот')
  })
  it('пропорция запроса картинки: страница, разворот, обложка; в пределах Go', () => {
    const st = { pageW: 300, pageH: 300, sheet: false }
    expect(lookRatio(st, 2)).toBe(2)
    expect(lookRatio(st, 1)).toBe(1)
    expect(lookRatio({ ...st, sheet: true }, 2)).toBe(1)
    expect(lookRatio(st, 2, { w: 640, h: 320 })).toBe(2)
    expect(lookRatio({ pageW: 10, pageH: 300, sheet: true }, 2)).toBe(0.2)
    expect(lookRatio({ pageW: 3000, pageH: 100, sheet: false }, 2)).toBe(5)
  })
  it('пропорция плитки карточки: обложка — лицевая сторона (половина листа), разворот — лист; в пределах Go', () => {
    const st = { pageW: 300, pageH: 300, sheet: false }
    expect(tileRatio(st, { cover: true })).toBe(1)
    expect(tileRatio(st, { cover: true }, { w: 640, h: 320 })).toBe(1)
    expect(tileRatio(st, { cover: false })).toBe(2)
    expect(tileRatio(st, {})).toBe(2)
    expect(tileRatio({ pageW: 10, pageH: 300, sheet: true }, { cover: true })).toBe(0.2)
    expect(tileRatio({ pageW: 3000, pageH: 100, sheet: false }, { cover: true })).toBe(2.5)
  })
})

describe('looks: вопрос «Применить» и тосты (U2)', () => {
  const q = (
    o: Partial<Record<'touched' | 'replaced' | 'left' | 'anchors', number>> & { coverTouched?: boolean; coverReplaced?: boolean } = {},
  ) => applyQuestion({ touched: 0, replaced: 0, left: 0, anchors: 0, coverTouched: false, coverReplaced: false, ...o })

  it('нечего заменять — вопроса нет', () => {
    expect(q({})).toBeNull()
    expect(q({ anchors: 4 })).toBeNull() // якоря сами по себе вопроса не требуют
  })
  it('свой фон: слова спеки и про «Весь альбом заново»', () => {
    expect(q({ replaced: 3, anchors: 3 })).toBe(
      'У 3 разворотов заменится свой фон. 3 разворота станут оформленными — «Весь альбом заново» их не пересоберёт. Применить?',
    )
    expect(q({ replaced: 1 })).toBe('У 1 разворота заменится свой фон. Применить?')
    expect(q({ replaced: 1, anchors: 1 })).toBe(
      'У 1 разворота заменится свой фон. 1 разворот станет оформленным — «Весь альбом заново» его не пересоберёт. Применить?',
    )
    expect(q({ replaced: 5, anchors: 5 })).toContain('5 разворотов станут оформленными — «Весь альбом заново» их не пересоберёт')
  })
  it('тронутые развороты: число и падеж', () => {
    expect(q({ touched: 2 })).toBe('Ваши правки на 2 разворотах заменятся. Применить?')
    expect(q({ touched: 1 })).toBe('Ваши правки на 1 развороте заменятся. Применить?')
    expect(q({ touched: 21 })).toBe('Ваши правки на 21 развороте заменятся. Применить?')
  })
  it('обложка в тронутых: «N разворотах и обложке», одна обложка — «на обложке» (0.28-131-174)', () => {
    expect(q({ touched: 3, coverTouched: true })).toBe('Ваши правки на 2 разворотах и обложке заменятся. Применить?')
    expect(q({ touched: 2, coverTouched: true })).toBe('Ваши правки на 1 развороте и обложке заменятся. Применить?')
    expect(q({ touched: 1, coverTouched: true })).toBe('Ваши правки на обложке заменятся. Применить?')
    expect(q({ touched: 2, left: 3, coverTouched: true })).toBe(
      'Ваши правки на 1 развороте и обложке заменятся. 3 фото вернутся в ленту. Применить?',
    )
  })
  it('обложка не тронута (оформление поставлено на чистую): прежние слова про развороты', () => {
    expect(q({ touched: 3, coverTouched: false })).toBe('Ваши правки на 3 разворотах заменятся. Применить?')
  })
  it('свой фон у обложки: «У N разворотов и обложки», одна обложка — «У обложки»', () => {
    expect(q({ replaced: 3, coverReplaced: true })).toBe('У 2 разворотов и обложки заменится свой фон. Применить?')
    expect(q({ replaced: 2, coverReplaced: true })).toBe('У 1 разворота и обложки заменится свой фон. Применить?')
    expect(q({ replaced: 1, coverReplaced: true })).toBe('У обложки заменится свой фон. Применить?')
    expect(q({ replaced: 3, coverReplaced: false })).toBe('У 3 разворотов заменится свой фон. Применить?')
  })
  it('тронутые и свой фон вместе, обе доли у обложки', () => {
    expect(q({ touched: 2, replaced: 2, coverTouched: true, coverReplaced: true })).toBe(
      'Ваши правки на 1 развороте и обложке заменятся. У 1 разворота и обложки заменится свой фон. Применить?',
    )
  })
  it('фото уходят в ленту — как 067; несколько причин через точку', () => {
    expect(q({ left: 1 })).toBe('1 фото вернётся в ленту. Применить?')
    expect(q({ left: 3 })).toBe('3 фото вернутся в ленту. Применить?')
    expect(q({ touched: 2, left: 3 })).toBe('Ваши правки на 2 разворотах заменятся. 3 фото вернутся в ленту. Применить?')
  })
  it('вопрос есть ровно там, где needsAsk', () => {
    for (const o of [{}, { touched: 1 }, { replaced: 1 }, { left: 1 }, { touched: 1, left: 2 }])
      expect(q(o) !== null).toBe(needsAsk({ touched: 0, replaced: 0, left: 0, ...o }))
  })

  const plan = { mode: 'series' as const, spreads: 9, places: 46, missing: 0 }
  it('тост серии: «Макет «…»: 9 разворотов, 46 мест», с «Вернуть»', () => {
    expect(applyToast('Светлая', plan, true)).toEqual({ text: 'Макет «Светлая»: 9 разворотов, 46 мест', undo: true })
    expect(applyToast('Светлая', { ...plan, spreads: 1, places: 1 }, true).text).toBe('Макет «Светлая»: 1 разворот, 1 место')
  })
  it('тост «Фоны» и «Добавить» называют своё; нет в библиотеке — в конце', () => {
    expect(applyToast('С', { ...plan, mode: 'bg' }, true).text).toBe('Макет «С»: фоны у 9 разворотов')
    expect(applyToast('С', { ...plan, mode: 'add', spreads: 2, places: 8 }, true).text).toBe('Макет «С»: добавлено 2 разворота, 8 мест')
    expect(applyToast('С', { ...plan, missing: 2 }, true).text).toBe('Макет «С»: 9 разворотов, 46 мест · Нет в библиотеке: 2')
  })
  it('повтор того же — холостой: без «Вернуть»', () => {
    expect(applyToast('С', plan, false)).toEqual({ text: 'Макет «С» уже стоит', undo: false })
  })

  it('«Поставить на…»: подпись по листу, слово листа в альбоме листов', () => {
    expect(stepLabel(false, 5)).toBe('Поставить на разворот 5')
    expect(stepLabel(false, 2, 'лист')).toBe('Поставить на лист 2')
    expect(stepLabel(true, 5)).toBe('Поставить на обложку')
  })
  it('«Добавить»: в тосте число из запроса, а не plan.spreads Go (разворотов макета после постановки)', () => {
    const p = { mode: 'add' as const, spreads: 11, places: 8, missing: 0 }
    expect(applyToast('С', p, true, 2).text).toBe('Макет «С»: добавлено 2 разворота, 8 мест')
    expect(applyToast('С', { ...p, mode: 'series' }, true, 2).text).toBe('Макет «С»: 11 разворотов, 8 мест') // серия — число Go
  })
  it('заголовок вопроса «Поставить на…»: у обложки — «Поставить обложку?»', () => {
    expect(stepAskTitle(false)).toBe('Поставить разворот?')
    expect(stepAskTitle(true)).toBe('Поставить обложку?')
  })
  it('почему «Поставить на…» серая: четыре случая, и пусто, когда шаг подходит листу', () => {
    // stepWhy(шаг, на обложке, в альбоме есть обложка, у макета есть обложка)
    expect(stepWhy(1, false, true, true)).toBe('')
    expect(stepWhy(0, true, true, true)).toBe('')
    expect(stepWhy(0, false, true, true)).toBe('Оформление обложки ставится только на обложку: откройте её')
    expect(stepWhy(0, false, false, true)).toBe('В альбоме нет обложки: её оформление встанет при «Создать обложку»')
    expect(stepWhy(2, true, true, true)).toBe('На обложку встаёт шаг «Обложка»: выберите его в ленте шагов')
    expect(stepWhy(2, true, true, false)).toBe('У этого макета нет оформления обложки')
  })
})
