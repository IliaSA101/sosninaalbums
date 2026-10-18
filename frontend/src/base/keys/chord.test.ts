import { afterEach, describe, expect, it } from 'vitest'
import parity from '../../../../testdata/parity/keymap.json'
import { setMacForTest } from '../core/platform'
import {
  canonChord,
  codeOfChord,
  eventChord,
  isSpace,
  KEY_CODES,
  MAX_CHORD,
  modsOfChord,
  recordChord,
  stripShift,
  type ChordEvent,
} from './chord'
import { chordsText, chordText, codeText } from './chordtext'

afterEach(() => setMacForTest(null))

const ev = (o: Partial<ChordEvent>): ChordEvent => ({
  key: '',
  code: '',
  ctrlKey: false,
  shiftKey: false,
  altKey: false,
  metaKey: false,
  ...o,
})

describe('Canon: векторы Go', () => {
  it('допустимые приводятся к каноническому виду', () => {
    for (const c of parity.chords.valid) expect(canonChord(c.in), c.in).toBe(c.out)
  })
  it('недопустимые — null', () => {
    for (const s of parity.chords.invalid) expect(canonChord(s), JSON.stringify(s)).toBeNull()
  })
  it('белый список кодов совпадает с Go и без цифрового блока', () => {
    expect([...KEY_CODES]).toEqual(parity.codes)
    expect(KEY_CODES.some((c) => c.startsWith('Numpad'))).toBe(false)
  })
  it('предел длины и повтор', () => {
    expect(MAX_CHORD).toBe(32)
    expect(canonChord('Mod+Alt+Shift+BracketRight')).toBe('Mod+Alt+Shift+BracketRight')
    expect(canonChord('Shift+'.repeat(5) + 'KeyZ')).toBeNull()
  })
})

describe('eventChord: векторы событий Go', () => {
  it.each(parity.events.map((e, i) => [i, e] as const))('событие %i', (_, c) => {
    setMacForTest(!!c.mac)
    const e = ev({ key: c.key, code: c.code, ctrlKey: !!c.ctrl, shiftKey: !!c.shift, altKey: !!c.alt, metaKey: !!c.meta })
    expect(eventChord(e), JSON.stringify(c)).toBe(c.chord)
  })

  it('каждое ожидание вектора — каноническая запись', () => {
    for (const c of parity.events) if (c.chord) expect(canonChord(c.chord)).toBe(c.chord)
  })
})

describe('eventChord: раскладки', () => {
  it('русская раскладка: Ctrl+Я = Ctrl+Z, key не важен', () => {
    expect(eventChord(ev({ key: 'я', code: 'KeyZ', ctrlKey: true }))).toBe('Mod+KeyZ')
    expect(eventChord(ev({ key: 'х', code: 'BracketLeft' }))).toBe('BracketLeft')
  })
  it('латинская буква по key: AZERTY и Dvorak', () => {
    expect(eventChord(ev({ key: 'z', code: 'KeyW' }))).toBe('KeyZ')
    expect(eventChord(ev({ key: 'Q', code: 'KeyX', shiftKey: true }))).toBe('Shift+KeyQ')
  })
  it('символы верхнего ряда — по коду клавиши', () => {
    expect(eventChord(ev({ key: '&', code: 'Digit1' }))).toBe('Digit1')
    expect(eventChord(ev({ key: '?', code: 'Digit7', shiftKey: true }))).toBe('Shift+Digit7')
  })
})

describe('eventChord: цифровой блок', () => {
  it('NumLock включён: цифры и знаки приводятся к основному ряду', () => {
    expect(eventChord(ev({ key: '5', code: 'Numpad5' }))).toBe('Digit5')
    expect(eventChord(ev({ key: '0', code: 'Numpad0', ctrlKey: true }))).toBe('Mod+Digit0')
    expect(eventChord(ev({ key: '+', code: 'NumpadAdd', ctrlKey: true }))).toBe('Mod+Equal')
    expect(eventChord(ev({ key: '-', code: 'NumpadSubtract' }))).toBe('Minus')
    expect(eventChord(ev({ key: 'Enter', code: 'NumpadEnter' }))).toBe('Enter')
    expect(eventChord(ev({ key: '.', code: 'NumpadDecimal' }))).toBe('Period')
  })
  it('NumLock выключен: имя из key', () => {
    expect(eventChord(ev({ key: 'ArrowLeft', code: 'Numpad4' }))).toBe('ArrowLeft')
    expect(eventChord(ev({ key: 'Home', code: 'Numpad7' }))).toBe('Home')
    expect(eventChord(ev({ key: 'PageDown', code: 'Numpad3' }))).toBe('PageDown')
    expect(eventChord(ev({ key: 'Delete', code: 'NumpadDecimal' }))).toBe('Delete')
    expect(eventChord(ev({ key: 'Insert', code: 'Numpad0' }))).toBe('Insert')
  })
  it('без пары в основном блоке — не сочетание', () => {
    expect(eventChord(ev({ key: 'Clear', code: 'Numpad5' }))).toBeNull()
    expect(eventChord(ev({ key: '/', code: 'NumpadDivide' }))).toBeNull()
    expect(eventChord(ev({ key: '*', code: 'NumpadMultiply' }))).toBeNull()
  })
})

describe('eventChord: пустой code (074-26)', () => {
  it('empty code fallback: по символу, ЙЦУКЕН и QWERTY', () => {
    const by = (key: string, o: Partial<ChordEvent> = {}) => eventChord(ev({ key, ...o }))
    expect(by('я')).toBe('KeyZ')
    expect(by('Я', { shiftKey: true })).toBe('Shift+KeyZ')
    expect(by('х')).toBe('BracketLeft')
    expect(by('ъ')).toBe('BracketRight')
    expect(by('ж')).toBe('Semicolon')
    expect(by('э')).toBe('Quote')
    expect(by('б')).toBe('Comma')
    expect(by('ю')).toBe('Period')
    expect(by('ё')).toBe('Backquote')
    expect(by('й')).toBe('KeyQ')
    expect(by('ь')).toBe('KeyM')
    expect(by('z', { ctrlKey: true })).toBe('Mod+KeyZ')
    expect(by('1')).toBe('Digit1')
    expect(by('!', { shiftKey: true })).toBe('Shift+Digit1')
    expect(by('?', { shiftKey: true })).toBe('Shift+Slash')
    expect(by('[')).toBe('BracketLeft')
    expect(by('-')).toBe('Minus')
    expect(by(' ')).toBe('Space')
    expect(by('Delete')).toBe('Delete')
    expect(by('ArrowUp')).toBe('ArrowUp')
    expect(by('F5')).toBe('F5')
  })
  it('неизвестный символ без code — не сочетание', () => {
    expect(eventChord(ev({ key: 'Ω' }))).toBeNull()
    expect(eventChord(ev({ key: 'Unidentified' }))).toBeNull()
    expect(eventChord(ev({ key: '' }))).toBeNull()
  })
  it('неизвестный code — не сочетание', () => {
    expect(eventChord(ev({ key: 'AudioVolumeUp', code: 'AudioVolumeUp' }))).toBeNull()
    expect(eventChord(ev({ key: 'x', code: 'F13' }))).toBe('KeyX') // латиница по key
    expect(eventChord(ev({ key: 'F13', code: 'F13' }))).toBeNull()
  })
})

describe('eventChord: модификаторы и Mac', () => {
  it('одни модификаторы — не сочетание (в том числе смена раскладки)', () => {
    for (const key of ['Control', 'Shift', 'Alt', 'Meta', 'AltGraph', 'CapsLock']) {
      expect(eventChord(ev({ key, code: key + 'Left', ctrlKey: true, shiftKey: true })), key).toBeNull()
    }
  })
  it('Windows: Ctrl = Mod, Win-клавиша не наша', () => {
    setMacForTest(false)
    expect(eventChord(ev({ key: 'z', code: 'KeyZ', ctrlKey: true }))).toBe('Mod+KeyZ')
    expect(eventChord(ev({ key: 'z', code: 'KeyZ', metaKey: true }))).toBeNull()
    expect(eventChord(ev({ key: 'z', code: 'KeyZ', ctrlKey: true, metaKey: true }))).toBeNull()
  })
  it('Mac: Cmd = Mod, Ctrl не наш', () => {
    setMacForTest(true)
    expect(eventChord(ev({ key: 'z', code: 'KeyZ', metaKey: true }))).toBe('Mod+KeyZ')
    expect(eventChord(ev({ key: 'z', code: 'KeyZ', ctrlKey: true }))).toBeNull()
    expect(eventChord(ev({ key: 'z', code: 'KeyZ', ctrlKey: true, metaKey: true }))).toBeNull()
    expect(eventChord(ev({ key: 'Ω', code: 'KeyZ', altKey: true }))).toBe('Alt+KeyZ') // Option
  })
  it('Ctrl+Alt (AltGr) — не сочетание; запись окна его оставляет для отказа Go', () => {
    const e = ev({ key: 'z', code: 'KeyZ', ctrlKey: true, altKey: true })
    expect(eventChord(e)).toBeNull()
    expect(recordChord(e)).toBe('Mod+Alt+KeyZ')
    setMacForTest(true)
    const m = ev({ key: 'z', code: 'KeyZ', metaKey: true, altKey: true })
    expect(eventChord(m)).toBeNull()
    expect(recordChord(m)).toBe('Mod+Alt+KeyZ')
  })
  it('recordChord = eventChord без Ctrl+Alt, модификаторы сами — null', () => {
    expect(recordChord(ev({ key: 'Control', code: 'ControlLeft', ctrlKey: true }))).toBeNull()
    expect(recordChord(ev({ key: 'я', code: 'KeyZ', ctrlKey: true, shiftKey: true }))).toBe('Mod+Shift+KeyZ')
  })
  it('порядок модификаторов канонический', () => {
    expect(eventChord(ev({ key: '1', code: 'Digit1', altKey: true, shiftKey: true }))).toBe('Alt+Shift+Digit1')
  })
})

describe('вспомогательные', () => {
  it('разбор записи', () => {
    expect(codeOfChord('Mod+Shift+KeyZ')).toBe('KeyZ')
    expect(codeOfChord('KeyZ')).toBe('KeyZ')
    expect(modsOfChord('Mod+Shift+KeyZ')).toEqual(['Mod', 'Shift'])
    expect(modsOfChord('KeyZ')).toEqual([])
  })
  it('stripShift убирает только Shift', () => {
    expect(stripShift('Shift+ArrowLeft')).toBe('ArrowLeft')
    expect(stripShift('Mod+Shift+KeyZ')).toBe('Mod+KeyZ')
    expect(stripShift('Mod+KeyZ')).toBe('Mod+KeyZ')
  })
  it('isSpace: по code и по пустому code', () => {
    expect(isSpace({ key: ' ', code: 'Space' })).toBe(true)
    expect(isSpace({ key: ' ', code: '' })).toBe(true)
    expect(isSpace({ key: 'a', code: 'KeyA' })).toBe(false)
  })
})

describe('chordText', () => {
  it('Windows: Ctrl+Alt+Shift+клавиша', () => {
    expect(chordText('Mod+KeyZ', false)).toBe('Ctrl+Z')
    expect(chordText('Mod+Shift+KeyZ', false)).toBe('Ctrl+Shift+Z')
    expect(chordText('Mod+Alt+Shift+Digit1', false)).toBe('Ctrl+Alt+Shift+1')
    expect(chordText('Mod+Minus', false)).toBe('Ctrl+−')
    expect(chordText('Mod+BracketRight', false)).toBe('Ctrl+]')
    expect(chordText('PageUp', false)).toBe('PgUp')
  })
  it('Mac: ⌥⇧⌘ без разделителя', () => {
    expect(chordText('Mod+KeyZ', true)).toBe('⌘Z')
    expect(chordText('Mod+Alt+Shift+KeyZ', true)).toBe('⌥⇧⌘Z')
    expect(chordText('Mod+Shift+KeyS', true)).toBe('⇧⌘S')
  })
  it('имена клавиш', () => {
    expect(chordText('Delete', false)).toBe('Del')
    expect(chordText('Space', false)).toBe('Пробел')
    expect(chordText('Escape', false)).toBe('Esc')
    expect(chordText('Shift+ArrowLeft', false)).toBe('Shift+←')
    expect(chordText('F5', false)).toBe('F5')
    expect(chordText('Backspace', false)).toBe('Backspace')
    expect(chordText('Enter', false)).toBe('Enter')
  })
  it('Shift+/ — «?», на обеих системах', () => {
    expect(chordText('Shift+Slash', false)).toBe('?')
    expect(chordText('Shift+Slash', true)).toBe('?')
    expect(chordText('Mod+Shift+Slash', false)).toBe('Ctrl+Shift+/')
    expect(chordText('Shift+Digit7', false)).toBe('Shift+7')
  })
  it('не сочетание — как есть; у каждого кода списка есть имя', () => {
    expect(chordText('мусор', false)).toBe('мусор')
    for (const code of KEY_CODES) expect(codeText(code), code).not.toBe('')
    const same = new Set(['Tab', 'Enter', 'Home', 'End', 'Backspace'])
    for (const code of KEY_CODES.filter((c) => !/^(Key|Digit|F\d)/.test(c) && !same.has(c))) expect(codeText(code), code).not.toBe(code)
  })
})

describe('chordsText', () => {
  it('несколько сочетаний через « / »; стрелки со Shift не повторяются', () => {
    expect(chordsText(['Mod+KeyY', 'Mod+Shift+KeyZ'], false)).toBe('Ctrl+Y / Ctrl+Shift+Z')
    expect(chordsText(['Mod+KeyY', 'Mod+Shift+KeyZ'], true)).toBe('⌘Y / ⇧⌘Z')
    expect(chordsText(['ArrowLeft', 'ArrowRight', 'Shift+ArrowLeft', 'Shift+ArrowRight'], false)).toBe('← / →')
    // Shift+стрелка без простой стрелки остаётся
    expect(chordsText(['Shift+ArrowUp'], false)).toBe('Shift+↑')
    expect(chordsText([], false)).toBe('')
  })
})
