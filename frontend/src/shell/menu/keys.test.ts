import { describe, expect, it } from 'vitest'
import { isTextField, keyAction, preventsDefault, TEXT_FIELDS, type KeyContext, type KeyEvent } from './keys'

const ev = (key: string, o: Partial<KeyEvent> = {}): KeyEvent => ({
  key,
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
const ctrl = { ctrlKey: true }

describe('keyAction', () => {
  it('Ctrl+Z / Ctrl+Shift+Z / Ctrl+Y, в том числе по-русски и с Cmd', () => {
    expect(keyAction(ev('z', ctrl), ctx())).toBe('undo')
    expect(keyAction(ev('Z', { ctrlKey: true, shiftKey: true }), ctx())).toBe('redo')
    expect(keyAction(ev('я', ctrl), ctx())).toBe('undo')
    expect(keyAction(ev('н', ctrl), ctx())).toBe('redo')
    expect(keyAction(ev('y', { metaKey: true }), ctx())).toBe('redo')
  })
  it('Ctrl+S и Ctrl+A', () => {
    expect(keyAction(ev('s', ctrl), ctx())).toBe('save')
    expect(keyAction(ev('ф', ctrl), ctx())).toBe('selectAll')
  })
  it('Ctrl+A по зоне: холст — разворот, библиотека (и без зоны) — все фото ленты', () => {
    expect(keyAction(ev('a', ctrl), ctx({ zone: 'canvas' }))).toBe('selectSpread')
    expect(keyAction(ev('ф', ctrl), ctx({ zone: 'canvas' }))).toBe('selectSpread')
    expect(keyAction(ev('a', ctrl), ctx({ zone: 'library' }))).toBe('selectAll')
    expect(keyAction(ev('a', ctrl), ctx())).toBe('selectAll')
    expect(keyAction(ev('a', ctrl), ctx({ zone: 'canvas', inTextField: true }))).toBeNull()
    expect(preventsDefault('selectSpread')).toBe(true)
  })
  it('Del в зоне библиотеки разворот не трогает', () => {
    expect(keyAction(ev('Delete'), ctx({ zone: 'library' }))).toBeNull()
    expect(keyAction(ev('Delete'), ctx({ zone: 'canvas' }))).toBe('removeCell')
    expect(keyAction(ev('Backspace'), ctx())).toBe('removeCell')
  })
  it('F и I — только без модификаторов и автоповтора', () => {
    expect(keyAction(ev('f'), ctx())).toBe('toggleBig')
    expect(keyAction(ev('ш'), ctx())).toBe('toggleDetails')
    expect(keyAction(ev('f', { repeat: true }), ctx())).toBeNull()
    expect(keyAction(ev('f', { altKey: true }), ctx())).toBeNull()
    expect(keyAction(ev('i', ctrl), ctx())).toBeNull()
  })
  it('G / п — безопасная зона, без модификаторов и автоповтора', () => {
    expect(keyAction(ev('g'), ctx())).toBe('toggleSafe')
    expect(keyAction(ev('п'), ctx())).toBe('toggleSafe')
    expect(keyAction(ev('G'), ctx())).toBe('toggleSafe')
    expect(keyAction(ev('g', { repeat: true }), ctx())).toBeNull()
    expect(keyAction(ev('g', ctrl), ctx())).toBeNull()
    expect(keyAction(ev('g'), ctx({ inTextField: true }))).toBeNull()
  })
  it('в поле ввода хоткеи молчат', () => {
    expect(keyAction(ev('z', ctrl), ctx({ inTextField: true }))).toBeNull()
    expect(keyAction(ev(' '), ctx({ inTextField: true }))).toBeNull()
  })
  it('стрелки и Delete', () => {
    expect(keyAction(ev('ArrowLeft'), ctx())).toBe('prevSpread')
    expect(keyAction(ev('ArrowRight'), ctx())).toBe('nextSpread')
    expect(keyAction(ev('Delete'), ctx())).toBe('removeCell')
    expect(keyAction(ev('Backspace'), ctx())).toBe('removeCell')
  })
  it('пробел — spaceDown (раскладку листает отпускание), на флажке — нет', () => {
    expect(keyAction(ev(' '), ctx())).toBe('spaceDown')
    expect(keyAction(ev(' ', { shiftKey: true }), ctx())).toBe('spaceDown')
    expect(keyAction(ev(' ', { repeat: true }), ctx())).toBe('spaceDown') // автоповтор гасим, но не отдаём странице
    expect(keyAction(ev(' '), ctx({ onCheckbox: true }))).toBeNull()
  })
  it('Enter — только при выделении', () => {
    expect(keyAction(ev('Enter'), ctx())).toBeNull()
    expect(keyAction(ev('Enter'), ctx({ hasSelection: true }))).toBe('addSelected')
  })
  it('Esc: сначала галерея, потом выделение', () => {
    expect(keyAction(ev('Escape'), ctx({ galleryOpen: true }))).toBe('closeGallery')
    expect(keyAction(ev('Escape'), ctx())).toBe('clearSelection')
  })
  it('прочее — null; preventDefault только у нужных', () => {
    expect(keyAction(ev('q'), ctx())).toBeNull()
    expect(preventsDefault('undo')).toBe(true)
    expect(preventsDefault('spaceDown')).toBe(true)
    expect(preventsDefault('prevSpread')).toBe(false)
  })
})

describe('keyAction: действия спеки 008', () => {
  it('Ctrl+0 — тоже вписать, и браузерный сброс масштаба гасится', () => {
    expect(keyAction(ev('0', ctrl), ctx())).toBe('fitZoom')
    expect(keyAction(ev('0', ctrl), ctx({ inTextField: true }))).toBeNull()
    expect(preventsDefault('fitZoom')).toBe(true)
  })
  it('«0» — вписать, только без модификаторов и автоповтора', () => {
    expect(keyAction(ev('0'), ctx())).toBe('fitZoom')
    expect(keyAction(ev('0', { altKey: true }), ctx())).toBeNull()
    expect(keyAction(ev('0', { repeat: true }), ctx())).toBeNull()
    expect(keyAction(ev('0'), ctx({ inTextField: true }))).toBeNull()
  })
  it('F1 — справочник функций, «?» — горячие клавиши (033)', () => {
    expect(keyAction(ev('F1'), ctx())).toBe('guide')
    expect(keyAction(ev('?', { shiftKey: true }), ctx())).toBe('help')
    expect(keyAction(ev('F1'), ctx({ inTextField: true }))).toBeNull()
    // F1 в WebView2 открыл бы справку браузера
    expect(preventsDefault('guide')).toBe(true)
  })
  it('nextLayout/prevLayout — только строки справки: клавишей их не получить', () => {
    for (const k of [' ', 'Enter', 'ArrowRight']) {
      const a = keyAction(ev(k), ctx())
      expect(a === 'nextLayout' || a === 'prevLayout', k).toBe(false)
    }
    expect(preventsDefault('help')).toBe(true)
  })
})

describe('isTextField', () => {
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
  })
  it('ползунок, флажок и кнопки не поле ввода', () => {
    for (const t of ['range', 'checkbox', 'radio', 'button', 'submit']) expect(TEXT_FIELDS).toContain(`:not([type='${t}'])`)
  })
  it('внутри поля хоткеи молчат (в том числе Delete, Пробел, стрелки, F)', () => {
    for (const k of ['Delete', 'Backspace', ' ', 'ArrowLeft', 'f', 'i', 'Escape', 'Enter'])
      expect(keyAction(ev(k), ctx({ inTextField: true, hasSelection: true }))).toBeNull()
  })
})

describe('keyAction: сетка без режима (спека 008)', () => {
  const arrows = ['ArrowLeft', 'ArrowRight', 'ArrowUp', 'ArrowDown']
  it('Пробел и на сетке spaceDown: панорама работает, раскладку App не листает', () => {
    expect(keyAction(ev(' '), ctx({ gridSpread: true }))).toBe('spaceDown')
    expect(keyAction(ev(' ', { shiftKey: true }), ctx({ gridSpread: true }))).toBe('spaceDown')
    expect(keyAction(ev(' '), ctx({ gridSpread: true, placeSelected: true }))).toBe('spaceDown')
    expect(keyAction(ev(' '), ctx({ gridSpread: false }))).toBe('spaceDown')
  })
  it('сетка без выделения и «Авто» (с выделением или без): ← → листают развороты', () => {
    for (const c of [ctx({ gridSpread: true }), ctx({ placeSelected: true }), ctx()]) {
      expect(keyAction(ev('ArrowLeft'), c)).toBe('prevSpread')
      expect(keyAction(ev('ArrowRight'), c)).toBe('nextSpread')
    }
    expect(keyAction(ev('ArrowUp'), ctx({ placeSelected: true }))).toBeNull()
  })
  it('сетка + выделено: стрелки двигают место (в том числе с Shift и автоповтором), не листают', () => {
    const g = ctx({ gridSpread: true, placeSelected: true })
    for (const k of arrows) {
      expect(keyAction(ev(k), g)).toBe('nudgePlace')
      expect(keyAction(ev(k, { shiftKey: true }), g)).toBe('nudgePlace')
      expect(keyAction(ev(k, { repeat: true }), g)).toBe('nudgePlace')
    }
    expect(keyAction(ev('ArrowLeft', { ctrlKey: true }), g)).toBeNull()
  })
  it('Delete и Backspace — removeCell и на «Авто», и на сетке (что именно — решает placesel)', () => {
    for (const c of [ctx(), ctx({ placeSelected: true }), ctx({ gridSpread: true, placeSelected: true })]) {
      expect(keyAction(ev('Delete'), c)).toBe('removeCell')
      expect(keyAction(ev('Backspace'), c)).toBe('removeCell')
    }
  })
  it('Esc: галерея, затем снять выделение', () => {
    expect(keyAction(ev('Escape'), ctx({ gridSpread: true, galleryOpen: true, placeSelected: true }))).toBe('closeGallery')
    expect(keyAction(ev('Escape'), ctx({ gridSpread: true, placeSelected: true }))).toBe('clearSelection')
    expect(keyAction(ev('Escape'), ctx({ gridSpread: true }))).toBe('clearSelection')
  })
  it('в поле ввода (X/Y/Ш/В) молчат и на сетке', () => {
    const g = ctx({ gridSpread: true, placeSelected: true, inTextField: true })
    for (const k of ['ArrowLeft', 'Delete', 'Escape']) expect(keyAction(ev(k), g)).toBeNull()
  })
  it('Ctrl+Z и режимы вида работают и на сетке', () => {
    const g = ctx({ gridSpread: true, placeSelected: true })
    expect(keyAction(ev('z', ctrl), g)).toBe('undo')
    expect(keyAction(ev('f'), g)).toBe('toggleBig')
  })
  it('preventDefault у сдвига места (иначе стрелки прокручивают)', () => {
    expect(preventsDefault('nudgePlace')).toBe(true)
    expect(preventsDefault('removeCell')).toBe(false)
  })
})

describe('keyAction: режим фона', () => {
  const m = ctx({ bgMode: true, gridSpread: true, placeSelected: true, zone: 'canvas' })
  it('Esc — выход из режима (галерея открыта — сначала она)', () => {
    expect(keyAction(ev('Escape'), m)).toBe('bgExit')
    expect(keyAction(ev('Escape'), { ...m, galleryOpen: true })).toBe('closeGallery')
  })
  it('стрелки — рамка фона (Shift, автоповтор), не место и не развороты; preventDefault', () => {
    for (const k of ['ArrowLeft', 'ArrowRight', 'ArrowUp', 'ArrowDown']) {
      expect(keyAction(ev(k), m)).toBe('bgNudge')
      expect(keyAction(ev(k, { shiftKey: true, repeat: true }), m)).toBe('bgNudge')
    }
    expect(preventsDefault('bgNudge')).toBe(true)
  })
  it('Del / Backspace — убрать фото фона, в том числе после броска из библиотеки', () => {
    expect(keyAction(ev('Delete'), m)).toBe('bgRemove')
    expect(keyAction(ev('Backspace'), { ...m, zone: 'library' })).toBe('bgRemove')
  })
  it('Ctrl+Z работает; в поле ввода молчат', () => {
    expect(keyAction(ev('z', ctrl), m)).toBe('undo')
    expect(keyAction(ev('Escape'), { ...m, inTextField: true })).toBeNull()
  })
})

describe('keyAction: слои (010 U6)', () => {
  const g = ctx({ gridSpread: true, layerable: true })
  const k = (key: string, o: Partial<KeyEvent> = {}) => keyAction(ev(key, { ctrlKey: true, ...o }), g)
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
    expect(keyAction(ev(']', ctrl), ctx({ gridSpread: true }))).toBeNull()
    expect(keyAction(ev(']', ctrl), ctx({ layerable: false }))).toBeNull()
    expect(keyAction(ev(']', ctrl), { ...g, bgMode: true })).toBeNull()
    expect(keyAction(ev(']', ctrl), { ...g, inTextField: true })).toBeNull()
  })
  it('под открытым диалогом молчат (слои не переставляются за окном)', () => {
    expect(keyAction(ev(']', ctrl), { ...g, modal: true })).toBeNull()
    expect(keyAction(ev('х', { ctrlKey: true, shiftKey: true }), { ...g, modal: true })).toBeNull()
  })
  // Без Ctrl ] — уже не слои, а шаг звёзд (004).
  it('без Ctrl не слои', () => expect(keyAction(ev(']'), g)).toBe('markStep'))
  it('чужой уровень: клавиша молчит', () => expect(keyAction(ev(']', ctrl), { ...g, can: () => false })).toBeNull())
})
