import { describe, expect, it } from 'vitest'
import type { BorderView } from '../../base/core/types'
import { LIB_MISSING_TEXT } from '../../panels/library/missing'
import { STICKER_NEWER } from '../sticker/sticker'
import { BORDER_LOST_FILE, BORDER_NEWER, borderNeedsRelink, borderState, borderTip } from './borderstate'

// Пропавшие и неизвестные рамочки (079 U5, сц. 11): состояние, подсказка, путь к файлу.
const bv = (over: Partial<BorderView> = {}): BorderView => ({
  kind: 'line',
  photo: '',
  lib: '',
  preset: 'sa:fr-white-line',
  name: 'Белая линия',
  inks: ['#ffffff'],
  width: 1.5,
  scale: 0,
  ins: [1.5, 1.5, 1.5, 1.5],
  url: '/u',
  maskUrl: '',
  pic: false,
  open: false,
  known: true,
  ...over,
})
const pic = (over: Partial<BorderView> = {}) =>
  bv({ kind: '', photo: 'f1', lib: 'my:0123456789abcdef', preset: '', name: 'Резная', inks: [], width: 0, scale: 1, pic: true, ...over })

describe('borderState', () => {
  it('встроенная и целая картинка — ok', () => {
    expect(borderState(bv(), [])).toBe('ok')
    expect(borderState(pic(), ['другое'])).toBe('ok')
  })
  it('файл картинки в списке пропавших — lost; чужой ID — нет', () => {
    expect(borderState(pic(), ['f1'])).toBe('lost')
    expect(borderState(pic(), ['f2'])).toBe('ok')
  })
  it('вид новее программы — newer, раньше пропажи файла', () => {
    expect(borderState(bv({ known: false }), [])).toBe('newer')
    expect(borderState(pic({ known: false }), ['f1'])).toBe('newer')
  })
  it('встроенная с чужим «photo» в missing не пропадает', () => {
    expect(borderState(bv({ photo: 'f1' }), ['f1'])).toBe('ok')
  })
})

describe('borderTip', () => {
  it('целая — подсказки нет', () => {
    expect(borderTip(bv(), [])).toBeUndefined()
    expect(borderTip(pic(), [])).toBeUndefined()
  })
  it('вид новее — тот же текст, что у стикера', () => {
    expect(borderTip(bv({ known: false }), [])).toBe(BORDER_NEWER)
    expect(BORDER_NEWER).toBe(STICKER_NEWER)
  })
  it('своя из библиотеки без файла — «загрузите тот же файл», как у фона и стикера', () => {
    expect(borderTip(pic(), ['f1'])).toBe(LIB_MISSING_TEXT)
  })
  it('без записи в библиотеке — путь «Перепривязать…», с именем файла, если Go его назвал', () => {
    expect(borderTip(pic({ lib: '' }), ['f1'])).toBe(BORDER_LOST_FILE)
    expect(borderTip(pic({ lib: '' }), ['f1'], { f1: 'vine.png' })).toBe(`${BORDER_LOST_FILE}: vine.png`)
    expect(BORDER_LOST_FILE).toContain('Перепривязать')
  })
})

describe('borderNeedsRelink', () => {
  it('только пропавшая картинка без lib: папкой ищется она, свои грузятся', () => {
    expect(borderNeedsRelink(pic({ lib: '' }), ['f1'])).toBe(true)
    expect(borderNeedsRelink(pic(), ['f1'])).toBe(false)
    expect(borderNeedsRelink(pic({ lib: '' }), [])).toBe(false)
    expect(borderNeedsRelink(bv({ known: false }), [])).toBe(false)
    expect(borderNeedsRelink(undefined, ['f1'])).toBe(false)
  })
})
