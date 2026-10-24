import { afterEach, beforeEach, describe, expect, it } from 'vitest'
import { keyCommand, type KeyContext, type KeyEvent } from './keys'
import { dropCatalog, useCatalog } from '../../base/keys/keymap.fixture'
import { can, type Custom, type FeatureId, type Level } from '../../base/prefs/levels'
import { features } from '../../base/prefs/levelcustom'

// Клавиши по уровням интерфейса, «Своё» и отметки фото (спеки 010 U1, 011 U2, 004 U2) на реестре (074). Остальное — keys.test.ts.
beforeEach(() => useCatalog())
afterEach(() => dropCatalog())

const ev = (key: string, o: Partial<KeyEvent> = {}): KeyEvent => ({
  key,
  code: '',
  ctrlKey: false,
  metaKey: false,
  altKey: false,
  shiftKey: false,
  repeat: false,
  ...o,
})
const ctx = (o: Partial<KeyContext> = {}): KeyContext => ({
  inTextField: false,
  onCheckbox: false,
  galleryOpen: false,
  hasSelection: false,
  ...o,
})
const cmd = (e: KeyEvent, c: KeyContext = ctx()) => keyCommand(e, c)
const ctrl = { ctrlKey: true }
const canvasZone = { zone: 'canvas' as const }

describe('клавиши по уровням (спека 010 U1)', () => {
  const at = (l: Level, o: Partial<KeyContext> = {}): KeyContext => ctx({ can: (f) => can(f, l), ...o })

  it('зум Ctrl+= / Ctrl+− молчит в «Простом», работает в «Стандартном»', () => {
    expect(cmd(ev('=', ctrl), at('simple'))).toBeNull()
    expect(cmd(ev('-', ctrl), at('simple'))).toBeNull()
    expect(cmd(ev('=', ctrl), at('standard'))).toBe('zoomIn')
    expect(cmd(ev('-', ctrl), at('full'))).toBe('zoomOut')
  })
  it('Ctrl+A на холсте — со «Стандартного»; в ленте — всегда', () => {
    expect(cmd(ev('a', ctrl), at('simple', canvasZone))).toBeNull()
    expect(cmd(ev('a', ctrl), at('standard', canvasZone))).toBe('selectSpread')
    expect(cmd(ev('a', ctrl), at('simple', { zone: 'library' }))).toBe('selectAll')
  })
  it('стрелки двигают место только на «Полном»: ниже они листают развороты', () => {
    const grid = { gridSpread: true, placeSelected: true }
    expect(cmd(ev('ArrowRight'), at('full', grid))).toBe('nudgePlace')
    expect(cmd(ev('ArrowRight'), at('standard', grid))).toBe('nextSpread')
  })
  it('стрелки двигают стикер и на «Стандартном» (048): ниже «Стандартного» — листают, место без стикера — по-прежнему «Полный»', () => {
    const sticker = { gridSpread: false, stickerSelected: true }
    expect(cmd(ev('ArrowRight'), at('standard', sticker))).toBe('nudgePlace')
    expect(cmd(ev('ArrowRight'), at('full', sticker))).toBe('nudgePlace')
    expect(cmd(ev('ArrowRight'), at('simple', sticker))).toBe('nextSpread')
    expect(cmd(ev('ArrowRight'), at('standard', { gridSpread: false }))).toBe('nextSpread')
  })
  it('стрелки — одна команда сдвига: строки для стикера в реестре нет (074-8, было две строки таблицы)', () => {
    useCatalog()
    // nudgePlace в каталоге одна; стикер и место различает условие, а не вторая строка
    expect(cmd(ev('ArrowUp'), ctx({ stickerSelected: true }))).toBe('nudgePlace')
    expect(cmd(ev('ArrowUp'), ctx({ gridSpread: true, placeSelected: true }))).toBe('nudgePlace')
  })
  it('клавиши основы живут на любом уровне', () => {
    for (const k of ['f', 'i', 'g', '0', 'F1']) expect(cmd(ev(k), at('simple')), k).not.toBeNull()
    expect(cmd(ev('z', ctrl), at('simple'))).toBe('undo')
  })
  it('без can в контексте — всё доступно (прежнее поведение)', () => {
    expect(cmd(ev('=', ctrl))).toBe('zoomIn')
  })
})

describe('клавиши «Своего» (спека 011 U2)', () => {
  const custom = (c: Custom) => features('custom', c)
  const ctxOf = (c: Custom, o: Partial<KeyContext> = {}): KeyContext => ctx({ can: (f: FeatureId) => custom(c).has(f), ...o })
  const full: Custom = { base: 'full', on: [], off: [] }

  it('F (крупный просмотр) молчит без «Разреза и склейки», I и G остаются', () => {
    const c: Custom = { base: 'simple', on: [], off: ['timeline'] }
    expect(cmd(ev('f'), ctxOf(c))).toBeNull()
    expect(cmd(ev('f'), ctxOf({ base: 'simple', on: [], off: [] }))).toBe('toggleBig')
    for (const k of ['i', 'g']) expect(cmd(ev(k), ctxOf(c)), k).not.toBeNull()
  })
  it('слои без «Ручной»: снять ручную — слои гаснут вместе с ней', () => {
    const c: Custom = { ...full, off: ['manual', 'layers'] }
    const layered = { gridSpread: true, layerable: true, placeSelected: true }
    expect(cmd(ev(']', ctrl), ctxOf(c, layered))).not.toBe('layerUp')
    expect(cmd(ev(']', ctrl), ctxOf(full, layered))).toBe('layerUp')
  })
  it('справка без «Вариантов»: Esc не закрывает галерею, которой нет, — снимает выделение', () => {
    const bare: Custom = { base: 'simple', on: [], off: ['variants'] }
    expect(cmd(ev('Escape'), ctxOf(bare))).toBe('clearSelection')
  })
})

describe('keyCommand: отметки (спека 004 U2)', () => {
  const lib = ctx({ hasSelection: true, zone: 'library' })
  const act = (key: string, c: KeyContext = lib, o: Partial<KeyEvent> = {}) => cmd(ev(key, o), c)

  it('1–5 и 0 — звёзды, в том числе цифровой блок', () => {
    for (const k of ['1', '2', '3', '4', '5', '0']) expect(act(k), k).toBe('markRating' + k)
    // Цифровой блок: key — цифра, code — Numpad
    expect(act('4', lib, { code: 'Numpad4' })).toBe('markRating4')
    expect(act('1', lib, { code: 'Digit1' })).toBe('markRating1')
    // NumLock выключен: Numpad1 даёт End — не звезда
    expect(act('End', lib, { code: 'Numpad1' })).toBeNull()
  })
  it('0 без выделения и на холсте — «вписать», с выделением в ленте — звёзды', () => {
    expect(act('0', { ...lib, hasSelection: false })).toBe('fitZoom')
    expect(act('0', { ...lib, zone: 'canvas' })).toBe('fitZoom')
    expect(act('0', { ...lib, zone: undefined })).toBe('markRating0')
    expect(act('0', lib, { ctrlKey: true })).toBe('fitZoom')
  })
  it('без выделения клавиши доходят до обработчика (тост)', () => {
    const none = { ...lib, hasSelection: false }
    expect(act('3', none)).toBe('markRating3')
    expect(act('p', none)).toBe('markPick')
  })
  it('] [ и русские ъ х — шаг; с Ctrl это слои, не оценка', () => {
    for (const [k, id] of [
      [']', 'markUp'],
      ['ъ', 'markUp'],
      ['[', 'markDown'],
      ['х', 'markDown'],
    ])
      expect(act(k), k).toBe(id)
    const layered = { ...lib, gridSpread: true, layerable: true, placeSelected: true }
    expect(act(']', layered, { ctrlKey: true })).toBe('layerUp')
    expect(act('х', layered, { ctrlKey: true, shiftKey: true })).toBe('layerBack')
  })
  it('6–9 — метки; P/X/U и з/ч/г — флажки', () => {
    expect([6, 7, 8, 9].map((k) => act(String(k)))).toEqual(['markRed', 'markYellow', 'markGreen', 'markBlue'])
    expect(act('p')).toBe('markPick')
    expect(act('з')).toBe('markPick')
    expect(act('x')).toBe('markReject')
    expect(act('ч')).toBe('markReject')
    expect(act('u')).toBe('markUnflag')
    expect(act('г')).toBe('markUnflag')
  })
  it('молчат с Ctrl/Alt/Shift, при автоповторе, на холсте, под окном, в крупном просмотре, при галерее', () => {
    for (const k of ['4', ']', '6', 'p', 'x', 'u']) {
      expect(String(act(k, lib, { ctrlKey: true })), `Ctrl+${k}`).not.toMatch(/^mark/)
      expect(act(k, lib, { altKey: true }), `Alt+${k}`).toBeNull()
      expect(act(k, lib, { shiftKey: true }), `Shift+${k}`).toBeNull()
      expect(act(k, lib, { repeat: true }), `repeat ${k}`).toBeNull()
      expect(act(k, { ...lib, zone: 'canvas' }), `canvas ${k}`).toBeNull()
      expect(act(k, { ...lib, dialog: 'shut' }), `dialog ${k}`).toBeNull()
      expect(act(k, { ...lib, big: true }), `big ${k}`).toBeNull()
      expect(act(k, { ...lib, galleryOpen: true }), `gallery ${k}`).toBeNull()
    }
  })
  it('не задевают существующие клавиши', () => {
    for (const [k, a] of [
      ['f', 'toggleBig'],
      ['i', 'toggleDetails'],
      ['g', 'toggleSafe'],
      ['Enter', 'addSelected'],
      ['ArrowLeft', 'prevSpread'],
      ['Escape', 'clearSelection'],
    ] as const)
      expect(act(k), k).toBe(a)
  })
  it('уровень: без функции «marks» клавиши молчат, 0 снова «вписать»', () => {
    const off: KeyContext = { ...lib, can: (f) => (f as string) !== 'marks' }
    expect(act('4', off)).toBeNull()
    expect(act('p', off)).toBeNull()
    expect(act('0', off)).toBe('fitZoom')
    expect(act('4', { ...lib, can: () => true })).toBe('markRating4')
  })
})

describe('копировать и вставить по уровням (099)', () => {
  const at = (l: Level) => ctx({ ...canvasZone, copyable: true, pasteable: true, can: (f) => can(f, l) })
  it('на «Простом» Ctrl+C, X, V, D молчат; со «Стандартного» работают', () => {
    for (const k of ['c', 'x', 'v', 'd']) {
      expect(cmd(ev(k, ctrl), at('simple')), `simple ${k}`).toBeNull()
      expect(cmd(ev(k, ctrl), at('standard')), `standard ${k}`).not.toBeNull()
      expect(cmd(ev(k, ctrl), at('full')), `full ${k}`).not.toBeNull()
    }
  })
  it('«Своё»: строка clipboard снята — клавиши молчат, включена на «Простом» — работают', () => {
    const run = (c: Custom) =>
      cmd(ev('c', ctrl), ctx({ ...canvasZone, copyable: true, can: (f: FeatureId) => features('custom', c).has(f) }))
    expect(run({ base: 'full', on: [], off: ['clipboard'] })).toBeNull()
    expect(run({ base: 'simple', on: ['clipboard'], off: [] })).toBe('copyItems')
    expect(run({ base: 'standard', on: [], off: [] })).toBe('copyItems')
  })
})
