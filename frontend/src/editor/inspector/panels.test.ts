import { describe, expect, it } from 'vitest'
import {
  COL_W,
  groupHeight,
  INSP_ZONE,
  inspectorMaxH,
  keyHint,
  labelWidth,
  M,
  panelMode,
  RAIL_GROUPS,
  RAIL_MAX,
  railColumns,
  railCols,
  railLabeled,
  railWidth,
  sideReserve,
  type PanelCtx,
  groupTitle,
  cardTitle,
} from './panels'

const ctx = (p: Partial<PanelCtx> = {}): PanelCtx => ({
  spread: 's1',
  gallery: false,
  textSpread: null,
  refs: 0,
  group: false,
  photo: false,
  photoMissing: false,
  ...p,
})

describe('panelMode', () => {
  it('нет разворота или открыта галерея — панелей нет', () => {
    expect(panelMode(ctx({ spread: null }))).toBe(null)
    expect(panelMode(ctx({ gallery: true, refs: 2, textSpread: 's1' }))).toBe(null)
  })
  it('ничего не выделено — idle', () => expect(panelMode(ctx())).toBe('idle'))
  it('приоритет: текст → фото → места', () => {
    const all = { textSpread: 's1', refs: 1, photo: true }
    expect(panelMode(ctx(all))).toBe('text')
    expect(panelMode(ctx({ ...all, textSpread: null }))).toBe('photo')
    expect(panelMode(ctx({ ...all, textSpread: null, photo: false }))).toBe('places')
  })
  it('несколько выделенных — места, даже если первое фото', () => {
    expect(panelMode(ctx({ refs: 2, photo: false }))).toBe('places')
  })
  it('группа фото и текста — places, не photo', () => {
    expect(panelMode(ctx({ refs: 1, photo: true, group: true }))).toBe('places')
  })
  it('два текста без мест — places', () => expect(panelMode(ctx({ refs: 0, group: true }))).toBe('places'))
  it('одно пустое место — места', () => expect(panelMode(ctx({ refs: 1, photo: false }))).toBe('places'))
  it('чужой разворот и фото без файла не в счёт', () => {
    expect(panelMode(ctx({ textSpread: 's2' }))).toBe('idle')
    expect(panelMode(ctx({ refs: 1, photo: true, photoMissing: true }))).toBe('idle')
  })
})

describe('лента', () => {
  it('две группы, пять кнопок', () => {
    expect(RAIL_GROUPS).toEqual([2, 3])
    expect(RAIL_GROUPS.reduce((a, b) => a + b, 0)).toBe(RAIL_MAX)
    expect(RAIL_MAX).toBe(5)
  })
  it('высота группы: подпись, кнопки 32 через 4', () => {
    expect(groupHeight(2)).toBe(84)
    expect(groupHeight(3)).toBe(120)
  })
  it('влезло — один столбец, обе группы (1280×760: окно холста 268)', () => {
    expect(railColumns(RAIL_GROUPS, 268)).toEqual([[0, 1]])
    expect(railCols(268)).toBe(1)
  })
  it('граница: группы подряд через черту и рамку', () => {
    const need = groupHeight(2) + 9 + groupHeight(3) + 2 * M + 2
    expect(railCols(need)).toBe(1)
    expect(railCols(need - 1)).toBe(2)
  })
  it('не влезло — группа целиком в новом столбце, не меньше одного', () => {
    expect(railColumns(RAIL_GROUPS, 150)).toEqual([[0], [1]])
    expect(railColumns(RAIL_GROUPS, 0)).toEqual([[0], [1]])
    expect(railCols(NaN)).toBe(2)
    expect(railColumns([], 268)).toEqual([])
  })
  it('ширина: столбец 60, между столбцами 4, рамка 2', () => {
    expect(railWidth(1)).toBe(COL_W + 2)
    expect(railWidth(2)).toBe(126)
    expect(railWidth(0)).toBe(62)
  })
})

describe('railLabeled', () => {
  it('один столбец и запас слева позволяет — подписи', () => {
    expect(railLabeled(268, false, INSP_ZONE, 8)).toBe(true) // «Отразить»
    expect(railLabeled(268, false, INSP_ZONE, 14)).toBe(true)
  })
  it('два столбца (низкое окно) — значки', () => expect(railLabeled(150, false, INSP_ZONE, 8)).toBe(false))
  it('карточка в панели деталей — значки', () => expect(railLabeled(268, true, 134, 8)).toBe(false))
  it('подпись не влезла в запас', () => {
    expect(railLabeled(268, false, 100, 8)).toBe(false)
    expect(railLabeled(268, false, 2 * M + 2 + labelWidth(8), 8)).toBe(true)
    expect(railLabeled(268, false, 2 * M + 2 + labelWidth(8) - 1, 8)).toBe(false)
  })
  it('столбец не уже подписи группы', () => expect(labelWidth(1)).toBe(COL_W))
  it('нет подписей или запаса', () => {
    expect(railLabeled(268, false, INSP_ZONE, 0)).toBe(false)
    expect(railLabeled(268, false, 0, 8)).toBe(false)
  })
})

describe('sideReserve', () => {
  it('карточка на холсте — запас под неё', () => {
    expect(sideReserve(110, 24, false, 2)).toBe(INSP_ZONE)
    expect(sideReserve(56, 24, false, 2)).toBe(INSP_ZONE)
  })
  it('карточка в панели деталей — только лента или края соседей', () => {
    expect(sideReserve(56, 24, true, 1)).toBe(80) // края соседей шире ленты (78)
    expect(sideReserve(40, 24, true, 1)).toBe(M + 62 + M)
    expect(sideReserve(40, 24, true, 2)).toBe(M + 126 + M)
  })
  it('не меньше краёв соседей', () => expect(sideReserve(300, 24, false, 1)).toBe(324))
})

describe('inspectorMaxH / keyHint', () => {
  it('под карточкой место значку масштаба', () => {
    expect(inspectorMaxH(268)).toBe(268 - 16 - 32)
    expect(inspectorMaxH(10)).toBe(0)
  })
  it('клавиша без пояснения', () => {
    expect(keyHint('nudgePlace')).toBe('← ↑ ↓ →')
    expect(keyHint('removeCell')).toBe('Delete / Backspace')
  })
})

describe('groupTitle', () => {
  it('состав группы', () => {
    expect(groupTitle(3, 0)).toBe('3 места')
    expect(groupTitle(2, 1)).toBe('2 места, 1 текст')
    expect(groupTitle(0, 2)).toBe('2 текста')
    expect(groupTitle(5, 5)).toBe('5 мест, 5 текстов')
  })
})

describe('cardTitle', () => {
  const c = { spreadNo: 18, caption: false, group: false, places: 1, texts: 0 }
  it('заголовок по режиму', () => {
    expect(cardTitle('idle', c)).toBe('Разворот 18')
    expect(cardTitle(null, c)).toBe('Разворот 18')
    expect(cardTitle('photo', c)).toBe('Фото')
    expect(cardTitle('text', c)).toBe('Текст')
    expect(cardTitle('text', { ...c, caption: true })).toBe('Подпись')
    expect(cardTitle('places', c)).toBe('Место')
    expect(cardTitle('places', { ...c, group: true, places: 2, texts: 1 })).toBe('2 места, 1 текст')
  })
  it('в листе (031) — «Лист N», обложка и прочее не меняются', () => {
    expect(cardTitle('idle', { ...c, unit: 'Лист' })).toBe('Лист 18')
    expect(cardTitle('idle', { ...c, unit: 'Разворот' })).toBe('Разворот 18')
    expect(cardTitle('idle', { ...c, unit: 'Лист', cover: true })).toBe('Обложка')
    expect(cardTitle('photo', { ...c, unit: 'Лист' })).toBe('Фото')
  })
  it('на обложке без выделения — «Обложка», с выделением — как у разворота', () => {
    expect(cardTitle('idle', { ...c, cover: true })).toBe('Обложка')
    expect(cardTitle('text', { ...c, cover: true })).toBe('Текст')
  })
})
