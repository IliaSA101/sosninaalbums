import { describe, expect, it } from 'vitest'
import { GROUPS, HOTKEYS, KEY_ACTIONS, findAction, visibleHotkeys, type KeyContext, type KeyEvent } from './hotkeys'
import { preventsDefault } from './keys'
import { can, type Custom, type Level } from '../../base/prefs/levels'
import { features } from '../../base/prefs/levelcustom'

describe('таблица клавиш', () => {
  it('каждое действие есть в таблице', () => {
    const used = new Set(HOTKEYS.map((h) => h.action))
    for (const a of KEY_ACTIONS) expect(used.has(a), a).toBe(true)
  })
  it('действия таблицы известны, KEY_ACTIONS без повторов', () => {
    for (const h of HOTKEYS) if (h.action) expect(KEY_ACTIONS).toContain(h.action)
    expect(new Set(KEY_ACTIONS).size).toBe(KEY_ACTIONS.length)
  })
  it('у каждой строки непустые show и label и известная группа', () => {
    const ids = GROUPS.map((g) => g.id)
    for (const h of HOTKEYS) {
      expect(h.show.trim(), h.label).not.toBe('')
      expect(h.label.trim(), h.show).not.toBe('')
      expect(ids).toContain(h.group)
    }
  })
  it('в каждой группе есть строки (справка не покажет пустых разделов)', () => {
    for (const g of GROUPS)
      expect(
        HOTKEYS.some((h) => h.group === g.id),
        g.id,
      ).toBe(true)
  })
  it('латинская буква в combos — вместе с русской на той же клавише', () => {
    const pairs: Record<string, string> = { z: 'я', y: 'н', s: 'ы', a: 'ф', f: 'а', i: 'ш', g: 'п', n: 'т', o: 'щ', e: 'у', r: 'к' }
    const back = Object.fromEntries(Object.entries(pairs).map(([l, r]) => [r, l]))
    for (const h of HOTKEYS) {
      for (const cb of h.combos ?? []) {
        for (const k of cb.keys) {
          if (k in pairs) expect(cb.keys, `${h.show}: ${k}`).toContain(pairs[k])
          if (k in back) expect(cb.keys, `${h.show}: ${k}`).toContain(back[k])
        }
      }
    }
  })
  it('клавиши в combos — в нижнем регистре', () => {
    for (const h of HOTKEYS) for (const cb of h.combos ?? []) for (const k of cb.keys) expect(k).toBe(k.toLowerCase())
  })
  it('строка с combos имеет action (иначе она не сработает)', () => {
    for (const h of HOTKEYS) if (h.combos) expect(h.action, h.show).toBeDefined()
  })
  it('справка «Ручной»: перенос места и ⇄ вместо обмена телом', () => {
    const grid = HOTKEYS.filter((h) => h.group === 'grid').map((h) => h.show)
    expect(grid.some((s) => s.includes('Перетащить фото или рамку'))).toBe(true)
    expect(grid.some((s) => s.includes('⇄'))).toBe(true)
  })
  it('справка: Alt+щелчок в группе «Холст»', () => {
    const row = HOTKEYS.find((h) => h.show === 'Alt+щелчок')
    expect(row?.group).toBe('canvas')
    expect(GROUPS.find((g) => g.id === 'canvas')?.title).toBe('Холст')
    // Способ выделения, а не инструмент: на всех уровнях.
    expect(row?.feature).toBeUndefined()
  })
  it('справка: есть строки про колесо, мышь и редактор текста', () => {
    const shows = HOTKEYS.map((h) => h.show).join('|')
    for (const s of ['Колесо', 'Ctrl+колесо', 'Зажатое колёсико', 'Двойной щелчок по фону', 'Ctrl+Enter', 'Shift+клик по фото']) {
      expect(shows).toContain(s)
    }
  })
})

const ev = (key: string, o: Partial<KeyEvent> = {}): KeyEvent => ({
  key,
  ctrlKey: false,
  metaKey: false,
  altKey: false,
  shiftKey: false,
  repeat: false,
  ...o,
})
const ctx: KeyContext = { inTextField: false, onCheckbox: false, galleryOpen: false, hasSelection: false }
const ctrl = { ctrlKey: true }

describe('клавиши меню (спека 010 U0)', () => {
  it('Ctrl+N / O / E / Shift+S — в обеих раскладках', () => {
    for (const [k, ru, act] of [
      ['n', 'т', 'new'],
      ['o', 'щ', 'open'],
      ['e', 'у', 'export'],
    ] as const) {
      expect(findAction(ev(k, ctrl), ctx), k).toBe(act)
      expect(findAction(ev(ru, ctrl), ctx), ru).toBe(act)
    }
    expect(findAction(ev('S', { ctrlKey: true, shiftKey: true }), ctx)).toBe('saveAs')
    expect(findAction(ev('Ы', { ctrlKey: true, shiftKey: true }), ctx)).toBe('saveAs')
    expect(findAction(ev('s', ctrl), ctx)).toBe('save')
  })
  it('Ctrl+R (и Ctrl+К) без «Полного» перехватывается: окно не перезагружается', () => {
    for (const k of ['r', 'к']) {
      const a = findAction(ev(k, ctrl), { ...ctx, can: (f) => can(f, 'standard') })
      expect(a, k).toBe('reload')
      expect(preventsDefault(a!)).toBe(true)
    }
    // Служебная строка в справку не попадает.
    expect(HOTKEYS.find((h) => h.action === 'reload')?.hidden).toBe(true)
  })
  it('Ctrl+R (и Ctrl+К) на «Полном» — линейки, окно тоже не перезагружается', () => {
    for (const k of ['r', 'к']) {
      const a = findAction(ev(k, ctrl), { ...ctx, can: (f) => can(f, 'full') })
      expect(a, k).toBe('toggleRulers')
      expect(preventsDefault(a!)).toBe(true)
    }
    expect(HOTKEYS.find((h) => h.action === 'toggleRulers')?.hidden).toBeFalsy()
  })
  it('Ctrl+= / Ctrl++ / Ctrl+− — зум; без Ctrl не срабатывают', () => {
    expect(findAction(ev('=', ctrl), ctx)).toBe('zoomIn')
    expect(findAction(ev('+', { ctrlKey: true, shiftKey: true }), ctx)).toBe('zoomIn')
    expect(findAction(ev('-', ctrl), ctx)).toBe('zoomOut')
    expect(findAction(ev('-'), ctx)).toBeNull()
  })
  it('Ctrl+N / O / E под открытым окном молчат', () => {
    for (const k of ['n', 'o', 'e']) expect(findAction(ev(k, ctrl), { ...ctx, modal: true }), k).toBeNull()
    expect(findAction(ev('s', ctrl), { ...ctx, modal: true })).toBe('save')
  })
  it('новые действия гасят действие браузера', () => {
    for (const a of ['new', 'open', 'export', 'saveAs', 'zoomIn', 'zoomOut'] as const) expect(preventsDefault(a), a).toBe(true)
  })
})

describe('клавиши по уровням (спека 010 U1)', () => {
  const at = (l: Level): KeyContext => ({ ...ctx, can: (f) => can(f, l) })

  it('зум Ctrl+= / Ctrl+− молчит в «Простом», работает в «Стандартном»', () => {
    expect(findAction(ev('=', ctrl), at('simple'))).toBeNull()
    expect(findAction(ev('-', ctrl), at('simple'))).toBeNull()
    expect(findAction(ev('=', ctrl), at('standard'))).toBe('zoomIn')
    expect(findAction(ev('-', ctrl), at('full'))).toBe('zoomOut')
  })
  it('Ctrl+A на холсте — со «Стандартного»; в ленте — всегда', () => {
    const canvas = { zone: 'canvas' as const }
    expect(findAction(ev('a', ctrl), { ...at('simple'), ...canvas })).toBeNull()
    expect(findAction(ev('a', ctrl), { ...at('standard'), ...canvas })).toBe('selectSpread')
    expect(findAction(ev('a', ctrl), { ...at('simple'), zone: 'library' })).toBe('selectAll')
  })
  it('стрелки двигают место только на «Полном»: ниже они листают развороты', () => {
    const grid = { gridSpread: true, placeSelected: true }
    expect(findAction(ev('ArrowRight'), { ...at('full'), ...grid })).toBe('nudgePlace')
    expect(findAction(ev('ArrowRight'), { ...at('standard'), ...grid })).toBe('nextSpread')
  })
  it('стрелки двигают стикер и на «Стандартном» (048): ниже «Стандартного» — листают, место без стикера — по-прежнему «Полный»', () => {
    const sticker = { gridSpread: false, stickerSelected: true }
    expect(findAction(ev('ArrowRight'), { ...at('standard'), ...sticker })).toBe('nudgePlace')
    expect(findAction(ev('ArrowRight'), { ...at('full'), ...sticker })).toBe('nudgePlace')
    expect(findAction(ev('ArrowRight'), { ...at('simple'), ...sticker })).toBe('nextSpread')
    expect(findAction(ev('ArrowRight'), { ...at('standard'), gridSpread: false })).toBe('nextSpread')
  })
  it('вторая строка стрелок для стикера скрыта из справки: «Двигать выделенное» одна', () => {
    const rows = HOTKEYS.filter((h) => h.action === 'nudgePlace')
    expect(rows.length).toBeGreaterThan(1)
    expect(rows.filter((h) => !h.hidden)).toHaveLength(1)
  })
  it('клавиши основы живут на любом уровне', () => {
    for (const k of ['f', 'i', 'g', '0', 'f1']) expect(findAction(ev(k), at('simple')), k).not.toBeNull()
    expect(findAction(ev('z', ctrl), at('simple'))).toBe('undo')
  })
  it('без can в контексте — всё доступно (прежнее поведение)', () => {
    expect(findAction(ev('=', ctrl), ctx)).toBe('zoomIn')
  })
})

describe('клавиши и справка «Своего» (спека 011 U2)', () => {
  const custom = (c: Custom) => features('custom', c)
  const ctxOf = (c: Custom): KeyContext => ({ ...ctx, can: (f) => custom(c).has(f) })
  const full: Custom = { base: 'full', on: [], off: [] }

  it('F (крупный просмотр) молчит без «Разреза и склейки», I и G остаются', () => {
    const c: Custom = { base: 'simple', on: [], off: ['timeline'] }
    expect(findAction(ev('f'), ctxOf(c))).toBeNull()
    expect(findAction(ev('f'), ctxOf({ base: 'simple', on: [], off: [] }))).toBe('toggleBig')
    for (const k of ['i', 'g']) expect(findAction(ev(k), ctxOf(c)), k).not.toBeNull()
  })
  it('справка без слоёв не показывает клавиши слоёв', () => {
    const c: Custom = { ...full, off: ['layers'] }
    const shows = visibleHotkeys((f) => custom(c).has(f)).map((h) => h.show)
    for (const s of ['Ctrl+]', 'Ctrl+[', 'Ctrl+Shift+]', 'Ctrl+Shift+[']) expect(shows, s).not.toContain(s)
    expect(visibleHotkeys((f) => custom(full).has(f)).map((h) => h.show)).toContain('Ctrl+]')
  })
  it('слои без «Ручной»: снять ручную — слои гаснут вместе с ней', () => {
    const c: Custom = { ...full, off: ['manual', 'layers'] }
    const layered = { ...ctxOf(c), gridSpread: true, layerable: true, placeSelected: true }
    expect(findAction(ev(']', ctrl), layered)).not.toBe('layerUp')
    expect(findAction(ev(']', ctrl), { ...ctxOf(full), gridSpread: true, layerable: true, placeSelected: true })).toBe('layerUp')
  })
  it('справка без «Вариантов» не называет Пробел раскладкой', () => {
    const c: Custom = { base: 'simple', on: [], off: ['variants'] }
    const labels = visibleHotkeys((f) => custom(c).has(f)).map((h) => h.label)
    expect(labels).not.toContain('Следующая раскладка разворота')
    expect(labels).not.toContain('Предыдущая раскладка разворота')
    expect(visibleHotkeys((f) => can(f, 'simple')).map((h) => h.label)).toContain('Следующая раскладка разворота')
  })
  it('справка без зума и галереи не называет увеличение и «Esc — закрыть галерею»', () => {
    const labels = (c: Custom) => visibleHotkeys((f) => custom(c).has(f)).map((h) => h.label)
    const shows = (c: Custom) => visibleHotkeys((f) => custom(c).has(f)).map((h) => h.show)
    const bare: Custom = { base: 'simple', on: [], off: ['variants'] }
    for (const s of ['0 / Ctrl+0', 'Пробел + мышь', 'Колесо, Shift+колесо (в увеличении)']) expect(shows(bare), s).not.toContain(s)
    expect(labels(bare)).not.toContain('Закрыть галерею раскладок')
    // «Простой» с вариантами: галерея есть, зума нет.
    const simple = visibleHotkeys((f) => can(f, 'simple'))
    expect(simple.map((h) => h.label)).toContain('Закрыть галерею раскладок')
    expect(simple.map((h) => h.show)).not.toContain('0 / Ctrl+0')
    // Зум включён — строки на месте; шаблоны без вариантов тоже держат галерею.
    expect(shows({ base: 'simple', on: ['zoom'], off: [] })).toContain('0 / Ctrl+0')
    expect(labels({ base: 'full', on: [], off: ['variants'] })).toContain('Закрыть галерею раскладок')
  })
})

describe('клавиши отметок (спека 004 U2)', () => {
  const lib: KeyContext = { ...ctx, hasSelection: true, zone: 'library' }
  const act = (key: string, c: KeyContext = lib, o: Partial<KeyEvent> = {}) => findAction(ev(key, o), c)

  it('1–5 и 0 — звёзды, в том числе цифровой блок (те же e.key)', () => {
    for (const k of ['1', '2', '3', '4', '5', '0']) expect(act(k), k).toBe('markRating')
  })
  it('0 без выделения и на холсте — «вписать», с выделением в ленте — звёзды', () => {
    expect(act('0', { ...lib, hasSelection: false })).toBe('fitZoom')
    expect(act('0', { ...lib, zone: 'canvas' })).toBe('fitZoom')
    expect(act('0', { ...lib, zone: undefined })).toBe('markRating')
    expect(act('0', lib, { ctrlKey: true })).toBe('fitZoom')
  })
  it('без выделения клавиши доходят до обработчика (тост)', () => {
    const none = { ...lib, hasSelection: false }
    expect(act('3', none)).toBe('markRating')
    expect(act('p', none)).toBe('markPick')
  })
  it('] [ и русские ъ х — шаг; с Ctrl это слои, не оценка', () => {
    for (const k of [']', '[', 'ъ', 'х']) expect(act(k), k).toBe('markStep')
    const layered = { ...lib, gridSpread: true, layerable: true, placeSelected: true }
    expect(act(']', layered, { ctrlKey: true })).toBe('layerUp')
    expect(act('х', layered, { ctrlKey: true, shiftKey: true })).toBe('layerBack')
  })
  it('6–9 — метки; P/X/U и з/ч/г — флажки', () => {
    for (const k of ['6', '7', '8', '9']) expect(act(k), k).toBe('markLabel')
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
      expect(act(k, { ...lib, modal: true }), `modal ${k}`).toBeNull()
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
    expect(act('4', { ...lib, can: () => true })).toBe('markRating')
  })
  it('в справке: группа «Фото», шесть действий, служебная строка 0 скрыта', () => {
    const rows = HOTKEYS.filter((h) => h.action?.startsWith('mark'))
    expect(new Set(rows.map((h) => h.action)).size).toBe(6)
    for (const h of rows) expect(h.group).toBe('photos')
    expect(visibleHotkeys().filter((h) => h.action?.startsWith('mark'))).toHaveLength(6)
  })
})
