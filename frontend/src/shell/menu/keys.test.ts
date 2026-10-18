import { afterEach, beforeEach, describe, expect, it } from 'vitest'
import {
  guardsBrowser,
  isTextField,
  isWidget,
  keyCommand,
  preventsDefault,
  TEXT_FIELDS,
  WIDGETS,
  type KeyContext,
  type KeyEvent,
} from './keys'
import { dropCatalog, useCatalog } from '../../base/keys/keymap.fixture'
import { setMacForTest } from '../../base/core/platform'
import { can } from '../../base/prefs/levels'

// Диспетчер на реестре (спека 074). Перенос 76 прежних случаев keys.test + hotkeys.test (таблица HOTKEYS ушла в Go):
// события без code (как приходили прежние случаи) идут через запасной разбор по символу (074-26), поэтому русская
// раскладка и знаки работают так же; отличия от прежнего поведения — в INTENDED_DIFFS keys.parity.test.ts.
beforeEach(() => useCatalog())
afterEach(() => {
  dropCatalog()
  setMacForTest(null)
})

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

describe('keyCommand', () => {
  it('Ctrl+Z / Ctrl+Shift+Z / Ctrl+Y, в том числе по-русски; Cmd на Mac', () => {
    expect(cmd(ev('z', ctrl))).toBe('undo')
    expect(cmd(ev('Z', { ctrlKey: true, shiftKey: true }))).toBe('redo')
    expect(cmd(ev('я', ctrl))).toBe('undo')
    expect(cmd(ev('н', ctrl))).toBe('redo')
    setMacForTest(true)
    expect(cmd(ev('y', { metaKey: true }))).toBe('redo')
    expect(cmd(ev('z', { metaKey: true }))).toBe('undo')
  })
  it('Ctrl+S и Ctrl+A', () => {
    expect(cmd(ev('s', ctrl))).toBe('save')
    expect(cmd(ev('ф', ctrl))).toBe('selectAll')
  })
  it('Ctrl+A по зоне: холст — разворот, библиотека — все фото ленты', () => {
    expect(cmd(ev('a', ctrl), ctx(canvasZone))).toBe('selectSpread')
    expect(cmd(ev('ф', ctrl), ctx(canvasZone))).toBe('selectSpread')
    expect(cmd(ev('a', ctrl), ctx({ zone: 'library' }))).toBe('selectAll')
    expect(cmd(ev('a', ctrl))).toBe('selectAll') // зоны нет — лента
    expect(cmd(ev('a', ctrl), ctx({ zone: 'canvas', inTextField: true }))).toBeNull()
    expect(preventsDefault('selectSpread')).toBe(true)
  })
  it('Del в зоне библиотеки разворот не трогает', () => {
    expect(cmd(ev('Delete'), ctx({ zone: 'library' }))).toBeNull()
    expect(cmd(ev('Delete'), ctx(canvasZone))).toBe('removeCell')
    expect(cmd(ev('Backspace'), ctx(canvasZone))).toBe('removeCell')
  })
  it('F и I — только без модификаторов и автоповтора', () => {
    expect(cmd(ev('f'))).toBe('toggleBig')
    expect(cmd(ev('ш'))).toBe('toggleDetails')
    expect(cmd(ev('f', { repeat: true }))).toBeNull()
    expect(cmd(ev('f', { altKey: true }))).toBeNull()
    expect(cmd(ev('i', ctrl))).toBeNull()
  })
  it('G / п — безопасная зона, без модификаторов и автоповтора', () => {
    expect(cmd(ev('g'))).toBe('toggleSafe')
    expect(cmd(ev('п'))).toBe('toggleSafe')
    expect(cmd(ev('G'))).toBe('toggleSafe') // CapsLock: заглавная без Shift
    expect(cmd(ev('g', { repeat: true }))).toBeNull()
    expect(cmd(ev('g', ctrl))).toBeNull()
    expect(cmd(ev('g'), ctx({ inTextField: true }))).toBeNull()
  })
  it('в поле ввода клавиши молчат', () => {
    expect(cmd(ev('z', ctrl), ctx({ inTextField: true }))).toBeNull()
    expect(cmd(ev(' '), ctx({ inTextField: true }))).toBeNull()
  })
  it('стрелки и Delete', () => {
    expect(cmd(ev('ArrowLeft'))).toBe('prevSpread')
    expect(cmd(ev('ArrowRight'))).toBe('nextSpread')
    expect(cmd(ev('Delete'), ctx(canvasZone))).toBe('removeCell')
    expect(cmd(ev('Backspace'), ctx(canvasZone))).toBe('removeCell')
  })
  it('пробел — spaceDown (раскладку листает отпускание), на флажке — нет', () => {
    expect(cmd(ev(' '))).toBe('spaceDown')
    expect(cmd(ev(' ', { shiftKey: true }))).toBe('spaceDown')
    expect(cmd(ev(' '), ctx({ onCheckbox: true }))).toBeNull()
  })
  it('Enter — только при выделении', () => {
    expect(cmd(ev('Enter'))).toBeNull()
    expect(cmd(ev('Enter'), ctx({ hasSelection: true }))).toBe('addSelected')
  })
  it('Esc: сначала галерея, потом выделение', () => {
    expect(cmd(ev('Escape'), ctx({ galleryOpen: true }))).toBe('closeGallery')
    expect(cmd(ev('Escape'))).toBe('clearSelection')
  })
  it('прочее — null; preventDefault только у нужных', () => {
    expect(cmd(ev('q'))).toBeNull()
    expect(preventsDefault('undo')).toBe(true)
    expect(preventsDefault('spaceDown')).toBe(true)
    expect(preventsDefault('prevSpread')).toBe(false)
  })
})

describe('keyCommand: действия спеки 008', () => {
  it('Ctrl+0 — тоже вписать, и браузерный сброс масштаба гасится', () => {
    expect(cmd(ev('0', ctrl))).toBe('fitZoom')
    expect(cmd(ev('0', ctrl), ctx({ inTextField: true }))).toBeNull()
    expect(preventsDefault('fitZoom')).toBe(true)
  })
  it('«0» — вписать, только без модификаторов и автоповтора', () => {
    expect(cmd(ev('0'))).toBe('fitZoom')
    expect(cmd(ev('0', { altKey: true }))).toBeNull()
    expect(cmd(ev('0', { repeat: true }))).toBeNull()
    expect(cmd(ev('0'), ctx({ inTextField: true }))).toBeNull()
  })
  it('F1 — справочник функций, «?» — горячие клавиши (033): и Shift+/, и Shift+7 русской раскладки', () => {
    expect(cmd(ev('F1'))).toBe('guide')
    expect(cmd(ev('?', { shiftKey: true }))).toBe('help')
    expect(cmd(ev('?', { shiftKey: true, code: 'Digit7' }))).toBe('help')
    expect(cmd(ev('?', { shiftKey: true, code: 'Slash' }))).toBe('help')
    expect(cmd(ev('F1'), ctx({ inTextField: true }))).toBeNull()
    // F1 в WebView2 открыл бы справку браузера
    expect(preventsDefault('guide')).toBe(true)
  })
  it('листание раскладок — жест Пробела на keyup, а не клавиша: Пробел, Enter и стрелки их не дают', () => {
    for (const k of [' ', 'Enter', 'ArrowRight']) {
      const a = cmd(ev(k))
      expect(a === 'nextLayout' || a === 'prevLayout', k).toBe(false)
    }
    expect(preventsDefault('help')).toBe(true)
  })
})

describe('isTextField / isWidget', () => {
  it('contenteditable считается полем ввода, contenteditable=false — нет', () => {
    expect(TEXT_FIELDS).toContain("[contenteditable]:not([contenteditable='false'])")
  })
  it('спрашивает у элемента ближайшее поле по селектору TEXT_FIELDS', () => {
    const asked: string[] = []
    const el = { closest: (sel: string) => (asked.push(sel), {}) }
    expect(isTextField(el)).toBe(true)
    expect(asked).toEqual([TEXT_FIELDS])
    expect(isTextField({ closest: () => null })).toBe(false)
    expect(isTextField(null)).toBe(false)
    // document и window без closest: не падаем.
    expect(isTextField({})).toBe(false)
    expect(isWidget({})).toBe(false)
  })
  it('ползунок, флажок и кнопки не поле ввода', () => {
    for (const t of ['range', 'checkbox', 'radio', 'button', 'submit']) expect(TEXT_FIELDS).toContain(`:not([type='${t}'])`)
  })
  it('виджеты: ползунок, select, список, группа радио, меню, role=slider', () => {
    for (const w of ["input[type='range']", 'select', "[role='slider']", "[role='listbox']", "[role='radiogroup']", "[role='menu']"])
      expect(WIDGETS, w).toContain(w)
    const asked: string[] = []
    expect(isWidget({ closest: (sel: string) => (asked.push(sel), {}) })).toBe(true)
    expect(asked).toEqual([WIDGETS])
  })
  it('внутри поля клавиши молчат (в том числе Delete, Пробел, стрелки, F)', () => {
    for (const k of ['Delete', 'Backspace', ' ', 'ArrowLeft', 'f', 'i', 'Escape', 'Enter'])
      expect(cmd(ev(k), ctx({ inTextField: true, hasSelection: true }))).toBeNull()
  })
})

describe('keyCommand: сетка без режима (спека 008)', () => {
  const arrows = ['ArrowLeft', 'ArrowRight', 'ArrowUp', 'ArrowDown']
  it('Пробел и на сетке spaceDown: панорама работает, раскладку App не листает', () => {
    expect(cmd(ev(' '), ctx({ gridSpread: true }))).toBe('spaceDown')
    expect(cmd(ev(' ', { shiftKey: true }), ctx({ gridSpread: true }))).toBe('spaceDown')
    expect(cmd(ev(' '), ctx({ gridSpread: true, placeSelected: true }))).toBe('spaceDown')
    expect(cmd(ev(' '), ctx({ gridSpread: false }))).toBe('spaceDown')
  })
  it('сетка без выделения и «Авто» (с выделением или без): ← → листают развороты', () => {
    for (const c of [ctx({ gridSpread: true }), ctx({ placeSelected: true }), ctx()]) {
      expect(cmd(ev('ArrowLeft'), c)).toBe('prevSpread')
      expect(cmd(ev('ArrowRight'), c)).toBe('nextSpread')
    }
    expect(cmd(ev('ArrowUp'), ctx({ placeSelected: true }))).toBeNull()
  })
  it('сетка + выделено: стрелки двигают место (в том числе с Shift и автоповтором), не листают', () => {
    const g = ctx({ gridSpread: true, placeSelected: true })
    for (const k of arrows) {
      expect(cmd(ev(k), g)).toBe('nudgePlace')
      expect(cmd(ev(k, { shiftKey: true }), g)).toBe('nudgePlace')
      expect(cmd(ev(k, { repeat: true }), g)).toBe('nudgePlace')
    }
    expect(cmd(ev('ArrowLeft', { ctrlKey: true }), g)).toBeNull()
  })
  it('Delete и Backspace — removeCell и на «Авто», и на сетке (что именно — решает placesel)', () => {
    for (const c of [
      ctx(canvasZone),
      ctx({ ...canvasZone, placeSelected: true }),
      ctx({ ...canvasZone, gridSpread: true, placeSelected: true }),
    ]) {
      expect(cmd(ev('Delete'), c)).toBe('removeCell')
      expect(cmd(ev('Backspace'), c)).toBe('removeCell')
    }
  })
  it('Esc: галерея, затем снять выделение', () => {
    expect(cmd(ev('Escape'), ctx({ gridSpread: true, galleryOpen: true, placeSelected: true }))).toBe('closeGallery')
    expect(cmd(ev('Escape'), ctx({ gridSpread: true, placeSelected: true }))).toBe('clearSelection')
    expect(cmd(ev('Escape'), ctx({ gridSpread: true }))).toBe('clearSelection')
  })
  it('в поле ввода (X/Y/Ш/В) молчат и на сетке', () => {
    const g = ctx({ gridSpread: true, placeSelected: true, inTextField: true })
    for (const k of ['ArrowLeft', 'Delete', 'Escape']) expect(cmd(ev(k), g)).toBeNull()
  })
  it('Ctrl+Z и режимы вида работают и на сетке', () => {
    const g = ctx({ gridSpread: true, placeSelected: true })
    expect(cmd(ev('z', ctrl), g)).toBe('undo')
    expect(cmd(ev('f'), g)).toBe('toggleBig')
  })
  it('preventDefault у сдвига места (иначе стрелки прокручивают)', () => {
    expect(preventsDefault('nudgePlace')).toBe(true)
    expect(preventsDefault('removeCell')).toBe(false)
  })
})

describe('keyCommand: режим фона', () => {
  const m = ctx({ bgMode: true, gridSpread: true, placeSelected: true, zone: 'canvas' })
  it('Esc — выход из режима (галерея открыта — сначала она)', () => {
    expect(cmd(ev('Escape'), m)).toBe('bgExit')
    expect(cmd(ev('Escape'), { ...m, galleryOpen: true })).toBe('closeGallery')
  })
  it('стрелки — рамка фона (Shift, автоповтор), не место и не развороты; preventDefault', () => {
    for (const k of ['ArrowLeft', 'ArrowRight', 'ArrowUp', 'ArrowDown']) {
      expect(cmd(ev(k), m)).toBe('bgNudge')
      expect(cmd(ev(k, { shiftKey: true, repeat: true }), m)).toBe('bgNudge')
    }
    expect(preventsDefault('bgNudge')).toBe(true)
  })
  it('Del / Backspace — убрать фото фона, в том числе после броска из библиотеки', () => {
    expect(cmd(ev('Delete'), m)).toBe('bgRemove')
    expect(cmd(ev('Backspace'), { ...m, zone: 'library' })).toBe('bgRemove')
  })
  it('Ctrl+Z работает; в поле ввода молчат', () => {
    expect(cmd(ev('z', ctrl), m)).toBe('undo')
    expect(cmd(ev('Escape'), { ...m, inTextField: true })).toBeNull()
  })
})

describe('keyCommand: слои (010 U6)', () => {
  const g = ctx({ gridSpread: true, layerable: true })
  const k = (key: string, o: Partial<KeyEvent> = {}) => cmd(ev(key, { ctrlKey: true, ...o }), g)
  it('Ctrl+] / Ctrl+[ — на слой, с Shift — на край', () => {
    expect(k(']')).toBe('layerUp')
    expect(k('[')).toBe('layerDown')
    expect(k('}', { shiftKey: true })).toBe('layerFront')
    expect(k('{', { shiftKey: true })).toBe('layerBack')
  })
  it('русская раскладка: ъ и х', () => {
    expect(k('ъ')).toBe('layerUp')
    expect(k('х')).toBe('layerDown')
    expect(k('ъ', { shiftKey: true })).toBe('layerFront')
    expect(k('х', { shiftKey: true })).toBe('layerBack')
  })
  it('молчат без выделения, в «Авто», в режиме фона и в поле ввода', () => {
    expect(cmd(ev(']', ctrl), ctx({ gridSpread: true }))).toBeNull()
    expect(cmd(ev(']', ctrl), ctx({ layerable: false }))).toBeNull()
    expect(cmd(ev(']', ctrl), { ...g, bgMode: true })).toBeNull()
    expect(cmd(ev(']', ctrl), { ...g, inTextField: true })).toBeNull()
  })
  it('«Авто», выделен стикер (layerable из canLayer): Ctrl+] / Ctrl+[ работают, без layerable — молчат', () => {
    const a = ctx({ gridSpread: false, stickerSelected: true, layerable: true })
    expect(cmd(ev(']', ctrl), a)).toBe('layerUp')
    expect(cmd(ev('[', ctrl), a)).toBe('layerDown')
    expect(cmd(ev(']', ctrl), { ...a, layerable: false })).toBeNull()
  })
  it('под открытым окном молчат (слои не переставляются за окном)', () => {
    expect(cmd(ev(']', ctrl), { ...g, dialog: 'shut' })).toBeNull()
    expect(cmd(ev('х', { ctrlKey: true, shiftKey: true }), { ...g, dialog: 'shut' })).toBeNull()
  })
  // Без Ctrl ] — уже не слои, а шаг звёзд (004).
  it('без Ctrl не слои', () => expect(cmd(ev(']'), g)).toBe('markUp'))
  it('чужой уровень: клавиша молчит', () => expect(cmd(ev(']', ctrl), { ...g, can: () => false })).toBeNull())
})

describe('клавиши меню (спека 010 U0)', () => {
  it('Ctrl+N / O / E / Shift+S — в обеих раскладках', () => {
    for (const [k, ru, act] of [
      ['n', 'т', 'new'],
      ['o', 'щ', 'open'],
      ['e', 'у', 'export'],
    ] as const) {
      expect(cmd(ev(k, ctrl)), k).toBe(act)
      expect(cmd(ev(ru, ctrl)), ru).toBe(act)
    }
    expect(cmd(ev('S', { ctrlKey: true, shiftKey: true }))).toBe('saveAs')
    expect(cmd(ev('Ы', { ctrlKey: true, shiftKey: true }))).toBe('saveAs')
    expect(cmd(ev('s', ctrl))).toBe('save')
  })
  it('Ctrl+R (и Ctrl+К) на «Полном» — линейки, окно тоже не перезагружается', () => {
    for (const k of ['r', 'к']) {
      const a = cmd(ev(k, ctrl), ctx({ can: (f) => can(f, 'full') }))
      expect(a, k).toBe('toggleRulers')
      expect(preventsDefault(a!, ev(k, ctrl))).toBe(true)
    }
  })
  it('Ctrl+R (и Ctrl+К) без «Полного» ничего не запускает, но страж браузера гасит перезагрузку', () => {
    for (const k of ['r', 'к']) {
      expect(cmd(ev(k, ctrl), ctx({ can: (f) => can(f, 'standard') })), k).toBeNull()
      expect(guardsBrowser(ev(k, ctrl)), k).toBe(true)
    }
    expect(guardsBrowser(ev('R', { ctrlKey: true, shiftKey: true }))).toBe(true)
    expect(guardsBrowser(ev('F5', { code: 'F5' }))).toBe(true)
    expect(guardsBrowser(ev('ArrowLeft', { altKey: true }))).toBe(true)
    expect(guardsBrowser(ev('ArrowRight', { altKey: true }))).toBe(true)
    // Обычные клавиши страж не трогает.
    for (const k of ['r', 'f', 'ArrowLeft', 'F1']) expect(guardsBrowser(ev(k)), k).toBe(false)
  })
  it('Ctrl+= / Ctrl++ / Ctrl+− — зум; без Ctrl не срабатывают', () => {
    expect(cmd(ev('=', ctrl))).toBe('zoomIn')
    expect(cmd(ev('+', { ctrlKey: true, shiftKey: true }))).toBe('zoomIn')
    expect(cmd(ev('-', ctrl))).toBe('zoomOut')
    expect(cmd(ev('-'))).toBeNull()
  })
  it('Ctrl+N / O / E под окном молчат; Ctrl+S — тоже (074-18)', () => {
    for (const k of ['n', 'o', 'e', 's']) expect(cmd(ev(k, ctrl), ctx({ dialog: 'shut' })), k).toBeNull()
  })
  it('новые действия гасят действие браузера', () => {
    for (const a of ['new', 'open', 'export', 'saveAs', 'zoomIn', 'zoomOut']) expect(preventsDefault(a), a).toBe(true)
  })
  it('любое совпавшее сочетание с Ctrl/Cmd или Alt гасит браузер (свои Ctrl+P/F/U), без них — только список', () => {
    useCatalog({ toggleSafe: ['Mod+KeyP'], toggleDetails: ['Alt+KeyD'], toggleMarkup: ['KeyM'] })
    expect(preventsDefault('toggleSafe', ev('p', ctrl))).toBe(true)
    expect(preventsDefault('toggleDetails', ev('d', { altKey: true }))).toBe(true)
    expect(preventsDefault('toggleMarkup', ev('m'))).toBe(false)
  })
})

describe('русская раскладка: буква на той же клавише, что латинская (074-2)', () => {
  const pairs: Record<string, string> = {
    z: 'я',
    y: 'н',
    s: 'ы',
    a: 'ф',
    f: 'а',
    i: 'ш',
    g: 'п',
    n: 'т',
    o: 'щ',
    e: 'у',
    r: 'к',
    p: 'з',
    x: 'ч',
    u: 'г',
  }
  it('Ctrl+буква и буква — те же команды, что с латиницей; в любом из регистров', () => {
    const c = ctx({ can: (f) => can(f, 'full'), zone: 'library', hasSelection: true })
    for (const [lat, ru] of Object.entries(pairs)) {
      for (const mods of [{}, ctrl, { ctrlKey: true, shiftKey: true }]) {
        expect(cmd(ev(ru, mods), c), `${ru} ${JSON.stringify(mods)}`).toBe(cmd(ev(lat, mods), c))
      }
    }
  })
  it('настоящее событие русской раскладки: key — кириллица, code — физическая клавиша', () => {
    expect(cmd(ev('я', { code: 'KeyZ', ctrlKey: true }))).toBe('undo')
    expect(cmd(ev('а', { code: 'KeyF' }))).toBe('toggleBig')
    expect(cmd(ev('з', { code: 'KeyP' }), ctx({ hasSelection: true }))).toBe('markPick')
    // Dvorak: Ctrl+Z набирается другой клавишей, но буква z — та же
    expect(cmd(ev('z', { code: 'KeyW', ctrlKey: true }))).toBe('undo')
  })
})

describe('правило окон (074-18, 074-28)', () => {
  const lib = (o: Partial<KeyContext> = {}) => ctx({ ...canvasZone, hasSelection: true, ...o })
  it('any modal: only undo/redo — под окном правки работают отмена и повтор, остальное молчит', () => {
    const w = lib({ dialog: 'undo' })
    expect(cmd(ev('z', ctrl), w)).toBe('undo')
    expect(cmd(ev('y', ctrl), w)).toBe('redo')
    expect(cmd(ev('Z', { ctrlKey: true, shiftKey: true }), w)).toBe('redo')
    for (const e of [ev('Delete'), ev('ArrowLeft'), ev('f'), ev('s', ctrl), ev('Escape'), ev('1'), ev('Enter'), ev('0')])
      expect(cmd(e, w), e.key).toBeNull()
  })
  it('вопрос и прочие окна: ничего, в том числе Ctrl+Z (jobs window: nothing)', () => {
    const w = lib({ dialog: 'shut' })
    for (const e of [ev('z', ctrl), ev('y', ctrl), ev('Delete'), ev('ArrowLeft'), ev('f'), ev('s', ctrl), ev('1'), ev('=', ctrl)])
      expect(cmd(e, w), e.key).toBeNull()
  })
  it('вопрос поверх «Фонов»: решает верхнее окно — отмена молчит', () => {
    expect(cmd(ev('z', ctrl), lib({ dialog: 'shut' }))).toBeNull()
    expect(cmd(ev('z', ctrl), lib({ dialog: 'undo' }))).toBe('undo')
  })
  it('автоповтор отмены под окном правки работает', () => {
    expect(cmd(ev('z', { ctrlKey: true, repeat: true }), lib({ dialog: 'undo' }))).toBe('undo')
  })
  it('в поле ввода окна клавиши молчат и без правила окон', () => {
    expect(cmd(ev('z', ctrl), lib({ dialog: 'undo', inTextField: true }))).toBeNull()
  })
  it('нет окна — обычная работа', () => {
    expect(cmd(ev('Delete'), lib())).toBe('removeCell')
  })
})

describe('фокус на виджете (074-19)', () => {
  const w = (o: Partial<KeyContext> = {}) => ctx({ widget: true, ...canvasZone, ...o })
  it('slider arrows belong to widget: стрелки, PageUp/PageDown, Home/End остаются ему', () => {
    for (const k of ['ArrowLeft', 'ArrowRight', 'ArrowUp', 'ArrowDown', 'PageUp', 'PageDown', 'Home', 'End']) {
      expect(cmd(ev(k), w()), k).toBeNull()
      expect(cmd(ev(k, { shiftKey: true }), w()), 'Shift+' + k).toBeNull()
    }
    expect(cmd(ev('ArrowLeft'), w({ bgMode: true }))).toBeNull()
    expect(cmd(ev('ArrowRight'), w({ gridSpread: true, placeSelected: true }))).toBeNull()
  })
  it('Ctrl/Alt со стрелкой и прочие клавиши виджету не отдаются', () => {
    expect(cmd(ev('Delete'), w())).toBe('removeCell')
    expect(cmd(ev('z', ctrl), w())).toBe('undo')
    expect(cmd(ev('f'), w())).toBe('toggleBig')
    expect(cmd(ev('Home', { code: 'Home', ctrlKey: true }), w())).toBeNull() // нет такой команды
  })
  it('вне виджета стрелки листают', () => {
    expect(cmd(ev('ArrowLeft'), ctx(canvasZone))).toBe('prevSpread')
  })
})

describe('автоповтор: повторяют только команды с флагом (074-9)', () => {
  it('repeat ignored for toggles: F, I, G, P, Ctrl+R, Del', () => {
    const c = ctx({ ...canvasZone, can: (f) => can(f, 'full') })
    for (const [k, o] of [
      ['f', {}],
      ['i', {}],
      ['g', {}],
      ['r', ctrl],
      ['Delete', {}],
      ['s', ctrl],
      ['Escape', {}],
    ] as const)
      expect(cmd(ev(k, { ...o, repeat: true }), c), k).toBeNull()
  })
  it('повторяют: отмена/повтор, зум, слои, листание, сдвиг', () => {
    const c = ctx({ ...canvasZone, gridSpread: true, layerable: true })
    for (const [k, o, id] of [
      ['z', ctrl, 'undo'],
      ['y', ctrl, 'redo'],
      ['=', ctrl, 'zoomIn'],
      ['-', ctrl, 'zoomOut'],
      [']', ctrl, 'layerUp'],
      ['[', ctrl, 'layerDown'],
      ['ArrowLeft', {}, 'prevSpread'],
      ['ArrowRight', {}, 'nextSpread'],
    ] as const)
      expect(cmd(ev(k, { ...o, repeat: true }), c), k).toBe(id)
  })
})

describe('свои клавиши (074-5)', () => {
  it('своя клавиша срабатывает, стандартная после переназначения молчит', () => {
    useCatalog({ toggleBig: ['KeyB'] })
    expect(cmd(ev('b'))).toBe('toggleBig')
    expect(cmd(ev('и'))).toBe('toggleBig') // русская «и» на той же клавише
    expect(cmd(ev('f'))).toBeNull()
  })
  it('сочетание со вторым модификатором: точное совпадение, Shift не «не важен» (074-9)', () => {
    expect(cmd(ev('F', { shiftKey: true }))).toBeNull()
    expect(cmd(ev('s', { ctrlKey: true, shiftKey: true }))).toBe('saveAs')
    expect(cmd(ev('ArrowLeft', { ctrlKey: true }))).toBeNull()
    expect(cmd(ev('z', { ctrlKey: true, altKey: true }))).toBeNull() // Ctrl+Alt — AltGr
    expect(cmd(ev('-', { ctrlKey: true, shiftKey: true }))).toBeNull()
  })
  it('две команды на одной клавише в разных областях: старшая область главнее, фон — над холстом', () => {
    useCatalog({ toggleDetails: ['Delete'] })
    // Del на холсте: removeCell (canvas) выше «везде»
    expect(cmd(ev('Delete'), ctx(canvasZone))).toBe('removeCell')
    // в ленте фото Del у removeCell молчит, остаётся «Панель деталей» (область «везде»)
    expect(cmd(ev('Delete'), ctx({ zone: 'library' }))).toBe('toggleDetails')
  })
  it('команда меню без клавиши по умолчанию: срабатывает, пока пункт меню доступен (menuOn)', () => {
    useCatalog({ toggleMarkup: ['KeyM'] })
    expect(cmd(ev('m'))).toBe('toggleMarkup') // menuOn не задан — пункт доступен
    expect(cmd(ev('m'), ctx({ menuOn: () => false }))).toBeNull()
    expect(cmd(ev('m'), ctx({ menuOn: (id) => id === 'toggleMarkup' }))).toBe('toggleMarkup')
  })
  it('команда с клавишей по умолчанию меню не спрашивает (undo при серой кнопке всё равно идёт в историю)', () => {
    expect(cmd(ev('z', ctrl), ctx({ menuOn: () => false }))).toBe('undo')
  })
  it('уровень гасит команду меню со своей клавишей: Ctrl+R у «Линеек» молчит ниже «Полного»', () => {
    useCatalog({ toggleRulers: ['KeyR'] })
    expect(cmd(ev('r'), ctx({ can: (f) => can(f, 'standard') }))).toBeNull()
    expect(cmd(ev('r'), ctx({ can: (f) => can(f, 'full') }))).toBe('toggleRulers')
  })
  it('команда без клавиши (убрана) не срабатывает ни от какой', () => {
    useCatalog({ undo: [] })
    expect(cmd(ev('z', ctrl))).toBeNull()
  })
})

describe('Mac: модификатор только Cmd (074-20)', () => {
  it('⌘Z отменяет, Ctrl+Z на Mac — не наше сочетание; на Windows — наоборот', () => {
    setMacForTest(true)
    expect(cmd(ev('z', { metaKey: true }))).toBe('undo')
    expect(cmd(ev('z', ctrl))).toBeNull()
    expect(cmd(ev('я', { code: 'KeyZ', metaKey: true }))).toBe('undo')
    setMacForTest(false)
    expect(cmd(ev('z', ctrl))).toBe('undo')
    expect(cmd(ev('z', { metaKey: true }))).toBeNull()
  })
  it('на Mac показ клавиш — ⌘, Ctrl не мешает листанию и одиночным клавишам', () => {
    setMacForTest(true)
    expect(cmd(ev('f'))).toBe('toggleBig')
    expect(cmd(ev('f', ctrl))).toBeNull()
  })
})
