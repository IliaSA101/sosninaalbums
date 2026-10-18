import { afterEach, beforeEach, describe, expect, it } from 'vitest'
import snapshot from '../../../../testdata/keys-parity.json'
import { guardsBrowser, keyCommand, preventsDefault, type KeyContext, type KeyEvent } from './keys'
import { dropCatalog, useCatalog } from '../../base/keys/keymap.fixture'
import { keymap } from '../../base/keys/keymap.svelte'
import { setMacForTest } from '../../base/core/platform'
import { can, type Base } from '../../base/prefs/levels'

// Паритет клавиш (спека 074, решение 074-24, шаг U0 → U2). testdata/keys-parity.json — снимок НЕЗАВИСИМОГО прежнего
// keyAction (таблица HOTKEYS до переноса на реестр Go), снят до правок и больше не обновляется: правка клавиш — только
// через INTENDED_DIFFS с причиной. Матрица «сочетание × контекст» прогоняется через новый диспетчер, результат сверяется
// со снимком: любое отличие обязано быть названо в INTENDED_DIFFS (и каждая запись — встретиться), иначе тест красный.

// Намеренные отличия нового диспетчера от снимка. ctx — имя контекста или '*' (любой); token — точное сочетание или
// регулярка по формату tokenOf; was — прежнее действие, null (не срабатывало) или '*' (любое); now — новое (прежнее имя
// действия), null (молчит) или '*' (любое). Правило должно встретиться хоть раз, а каждое отличие — подойти под правило.
interface Diff {
  ctx: string | '*'
  token: string | RegExp
  was: string | null
  now: string | null
  why: string // номер решения 074-N и слово
}

// Модификаторы токена: любая цепочка ctrl+/shift+/alt+ (формат tokenOf).
const WITH_MODS = String.raw`^((ctrl|shift|alt)\+)+(repeat\+)?`
export const INTENDED_DIFFS: readonly Diff[] = [
  // Модификаторы — точное совпадение, раньше Shift и Ctrl у части строк «не важны» (074-9).
  { ctx: '*', token: /^shift\+[FIGАШП0]$/, was: '*', now: null, why: '074-9: Shift+F/I/G/0 больше не режим вида и не «вписать»' },
  { ctx: '*', token: /^ctrl\+shift\+[A-ZА-Я]$/, was: '*', now: null, why: '074-9: Ctrl+Shift+буква — не Ctrl+буква (N, O, E, Y, A, R)' },
  { ctx: '*', token: /^ctrl\+shift\+-$/, was: '*', now: null, why: '074-9: Ctrl+Shift+− не уменьшает' },
  { ctx: '*', token: /^shift\+F1$/, was: 'guide', now: null, why: '074-9: Shift+F1 — другое сочетание, не справочник' },
  {
    ctx: '*',
    token: new RegExp(WITH_MODS + 'Arrow(Left|Right)$'),
    was: '*',
    now: null,
    why: '074-9: Ctrl+←, Shift+←, Alt+← не листают разворот',
  },
  {
    ctx: '*',
    token: new RegExp(WITH_MODS + '(Enter|Escape|Delete|Backspace|Space)$'),
    was: '*',
    now: null,
    why: '074-9: Enter, Esc, Del, Пробел с Ctrl/Shift/Alt — другие сочетания',
  },
  // Автоповтор — только у команд с флагом repeat (074-9): отмена и повтор, масштаб, слои, листание, сдвиг.
  {
    ctx: '*',
    token: /^repeat\+(Enter|Escape|Delete|Backspace|Space)$/,
    was: '*',
    now: null,
    why: '074-9: Del, Esc, Enter, Пробел не повторяются удержанием',
  },
  {
    ctx: '*',
    token: /^ctrl\+repeat\+(?![zяyн]$)[a-zа-я0]$/i,
    was: '*',
    now: null,
    why: '074-9: Ctrl+буква-переключатель и Ctrl+0 не повторяются (Ctrl+Z, Ctrl+Y — да)',
  },
  // Ctrl+Alt — AltGr на части раскладок Windows.
  { ctx: '*', token: /^ctrl\+alt\+/, was: '*', now: null, why: '074-23: Ctrl+Alt — AltGr, сочетанием не считается' },
  // Win-клавиша на Windows (на Mac Cmd работает: см. тест «Mac»).
  { ctx: '*', token: /meta/, was: '*', now: null, why: '074-20: Win-клавиша на Windows не наша, на Mac работает только Cmd' },
  // «?» теперь Shift+/ и Shift+7 (русская раскладка); голый символ без Shift — синтетика без code (074-26).
  { ctx: '*', token: /^shift\+[7/]$/, was: null, now: 'help', why: '074-13: «?» — Shift+/ и Shift+7 русской раскладки' },
  {
    ctx: '*',
    token: '?',
    was: 'help',
    now: null,
    why: '074-26: символ «?» без Shift (событие без code) — не Shift+/; Shift не додумывается',
  },
  {
    ctx: '*',
    token: /^(ctrl\+)?(repeat\+)?[!@)_{}]$/,
    was: null,
    now: '*',
    why: '074-26: символ с Shift без флага Shift (событие без code) разбирается по клавише, Shift не додумывается',
  },
  // Зона не задана — лента фото (073): Del и Backspace там разворот не трогают (074-4).
  {
    ctx: 'зоны нет (после запуска)',
    token: /^(Delete|Backspace)$/,
    was: 'removeCell',
    now: null,
    why: '074-4: зона не задана = лента фото, Del разворот не трогает',
  },
  // Правило окон: под вопросом/экспортом все команды молчат (074-18; отмена — только под окнами правки, 074-28).
  {
    ctx: 'под вопросом/экспортом (modal)',
    token: /.*/,
    was: '*',
    now: null,
    why: '074-18: под любым окном команды молчат, Ctrl+Z — только под окнами правки (074-28)',
  },
  // Ctrl+R без «Полного»: reload-no-op ушёл из таблицы, страж браузера гасит перезагрузку (проверка ниже).
  { ctx: '*', token: /^ctrl\+[rк]$/i, was: 'reload', now: null, why: '074-8: reload ушёл из таблицы — Ctrl+R гасит страж браузера' },
]

type Flags = Omit<KeyContext, 'can' | 'menuOn'>
interface Ctx {
  level: Base
  flags: Partial<Flags>
}
const BASE: Flags = { inTextField: false, onCheckbox: false, galleryOpen: false, hasSelection: false }

// Контексты нынешних правил: зона (холст/лента/нет), режим фона, фокус в поле, окно вопроса/экспорта (shut),
// крупный просмотр, галерея, выделение места/стикера, уровень интерфейса. Имена — как в снимке.
const CONTEXTS: Record<string, Ctx> = {
  лента: { level: 'full', flags: { zone: 'library' } },
  'лента+выделены фото': { level: 'full', flags: { zone: 'library', hasSelection: true } },
  'лента+галерея': { level: 'full', flags: { zone: 'library', galleryOpen: true } },
  'лента+крупный просмотр': { level: 'full', flags: { zone: 'library', big: true } },
  'зоны нет (после запуска)': { level: 'full', flags: {} },
  холст: { level: 'full', flags: { zone: 'canvas' } },
  'холст+галерея': { level: 'full', flags: { zone: 'canvas', galleryOpen: true } },
  'холст+выделены фото': { level: 'full', flags: { zone: 'canvas', hasSelection: true } },
  'холст+сетка+место': {
    level: 'full',
    flags: { zone: 'canvas', gridSpread: true, placeSelected: true, layerable: true, hasSelection: true },
  },
  'холст+авто+место': { level: 'full', flags: { zone: 'canvas', gridSpread: false, placeSelected: true, layerable: false } },
  'холст+стикер': { level: 'full', flags: { zone: 'canvas', stickerSelected: true, layerable: true } },
  'холст+сетка+слои без места': { level: 'full', flags: { zone: 'canvas', gridSpread: true, layerable: true } },
  'режим фона': { level: 'full', flags: { zone: 'canvas', bgMode: true } },
  'режим фона+галерея': { level: 'full', flags: { zone: 'canvas', bgMode: true, galleryOpen: true } },
  'режим фона+место': { level: 'full', flags: { zone: 'canvas', bgMode: true, gridSpread: true, placeSelected: true, layerable: true } },
  'поле ввода': { level: 'full', flags: { zone: 'canvas', inTextField: true } },
  'флажок в фокусе': { level: 'full', flags: { zone: 'canvas', onCheckbox: true } },
  'под вопросом/экспортом (modal)': { level: 'full', flags: { zone: 'canvas', dialog: 'shut', hasSelection: true } },
  'Простой: холст+сетка+место': { level: 'simple', flags: { zone: 'canvas', gridSpread: true, placeSelected: true, layerable: true } },
  'Простой: лента': { level: 'simple', flags: { zone: 'library', hasSelection: true } },
  'Стандартный: холст+сетка+место': {
    level: 'standard',
    flags: { zone: 'canvas', gridSpread: true, placeSelected: true, layerable: true },
  },
  'Стандартный: лента': { level: 'standard', flags: { zone: 'library', hasSelection: true } },
}

// Клавиши: e.key как приходит от браузера. Латиница и русские двойники, цифры, знаки, служебные.
const LATIN = ['z', 'y', 's', 'n', 'o', 'e', 'r', 'a', 'f', 'i', 'g', 'p', 'x', 'u', 'b']
const RUSSIAN = ['я', 'н', 'ы', 'т', 'щ', 'у', 'к', 'ф', 'а', 'ш', 'п', 'з', 'ч', 'г', 'й']
const CAPS = ['Z', 'Я', 'S', 'Ы', 'F', 'А'] // CapsLock: заглавная буква без Shift
const DIGITS = ['0', '1', '2', '3', '4', '5', '6', '7', '8', '9']
const SYMBOLS = ['=', '+', '-', '_', '[', ']', '{', '}', 'х', 'ъ', 'Х', 'Ъ', '?', ',', '/', '!', '@', ')']
const SPECIAL = [
  'ArrowLeft',
  'ArrowRight',
  'ArrowUp',
  'ArrowDown',
  'Delete',
  'Backspace',
  'Escape',
  'Enter',
  ' ',
  'Tab',
  'F1',
  'F2',
  'Home',
  'PageUp',
]

interface Mods {
  ctrl?: boolean
  alt?: boolean
  shift?: boolean
  meta?: boolean
  repeat?: boolean
}
// Без Mods — просто клавиша; Ctrl+Alt — AltGr на части раскладок Windows; Meta — Cmd на Mac.
const MODS: Mods[] = [
  {},
  { ctrl: true },
  { shift: true },
  { ctrl: true, shift: true },
  { alt: true },
  { meta: true },
  { meta: true, shift: true },
  { ctrl: true, alt: true },
  { repeat: true },
  { ctrl: true, repeat: true },
  { shift: true, repeat: true },
]

const isLetter = (k: string) => k.length === 1 && k.toLowerCase() !== k.toUpperCase()
const keyName = (k: string) => (k === ' ' ? 'Space' : k)
export const tokenOf = (key: string, m: Mods): string =>
  [m.ctrl && 'ctrl', m.alt && 'alt', m.shift && 'shift', m.meta && 'meta', m.repeat && 'repeat', keyName(key)].filter(Boolean).join('+')

// Прежняя матрица шла по e.key без code: запасной разбор по символу (074-26) её и читает.
const eventOf = (key: string, m: Mods): KeyEvent => ({
  key,
  code: '',
  ctrlKey: !!m.ctrl,
  altKey: !!m.alt,
  shiftKey: !!m.shift,
  metaKey: !!m.meta,
  repeat: !!m.repeat,
})
const ctxOf = (c: Ctx): KeyContext => ({ ...BASE, ...c.flags, can: (f) => can(f, c.level) })

interface Case {
  token: string
  key: string
  m: Mods
  ev: KeyEvent
}
function matrix(): Case[] {
  const out: Case[] = []
  const plain = [...LATIN, ...RUSSIAN, ...DIGITS, ...SYMBOLS, ...SPECIAL]
  for (const m of MODS) {
    for (const k of plain) {
      // Shift делает букву заглавной, как в браузере.
      const key = m.shift && isLetter(k) ? k.toUpperCase() : k
      out.push({ token: tokenOf(key, m), key, m, ev: eventOf(key, m) })
    }
    for (const k of CAPS) out.push({ token: tokenOf(k, m), key: k, m, ev: eventOf(k, m) })
  }
  return out
}

// id реестра -> прежнее имя действия (id разбиты по 074-7: одна команда на клавишу вместо e.key в обработчике).
const LEGACY: Record<string, string> = {
  markRating0: 'markRating',
  markRating1: 'markRating',
  markRating2: 'markRating',
  markRating3: 'markRating',
  markRating4: 'markRating',
  markRating5: 'markRating',
  markUp: 'markStep',
  markDown: 'markStep',
  markRed: 'markLabel',
  markYellow: 'markLabel',
  markGreen: 'markLabel',
  markBlue: 'markLabel',
}
const legacy = (id: string | null): string | null => (id === null ? null : (LEGACY[id] ?? id))

type Actions = Record<string, string[]>
interface SnapCtx {
  level: Base
  flags: Record<string, unknown>
  actions: Actions
}
const snap = snapshot as unknown as {
  cases: number
  matched: number
  prevent: string[]
  hints: { action: string; show: string; menu?: string }[]
  app: Record<string, unknown>
  contexts: Record<string, SnapCtx>
}

// Прежнее действие по сочетанию в контексте (инвертированный снимок).
function wasOf(): Map<string, Map<string, string>> {
  const out = new Map<string, Map<string, string>>()
  for (const [name, c] of Object.entries(snap.contexts)) {
    const m = new Map<string, string>()
    for (const [act, tokens] of Object.entries(c.actions)) for (const t of tokens) m.set(t, act)
    out.set(name, m)
  }
  return out
}

interface Found {
  ctx: string
  token: string
  was: string | null
  now: string | null
}

// Все расхождения нового диспетчера со снимком.
function diffs(cases: Case[]): Found[] {
  const was = wasOf()
  const out: Found[] = []
  for (const [name, c] of Object.entries(CONTEXTS)) {
    const ctx = ctxOf(c)
    const w = was.get(name)!
    for (const { token, ev } of cases) {
      const now = legacy(keyCommand(ev, ctx))
      const prev = w.get(token) ?? null
      if (now !== prev) out.push({ ctx: name, token, was: prev, now })
    }
  }
  return out
}

const ruleMatches = (r: Diff, f: Found): boolean =>
  (r.ctx === '*' || r.ctx === f.ctx) &&
  (typeof r.token === 'string' ? r.token === f.token : r.token.test(f.token)) &&
  (r.was === '*' ? f.was !== null : r.was === f.was) &&
  (r.now === '*' ? f.now !== null : r.now === f.now)

beforeEach(() => useCatalog())
afterEach(() => {
  dropCatalog()
  setMacForTest(null)
})

describe('паритет клавиш: новый диспетчер против снимка прежнего keyAction (074-24)', () => {
  const cases = matrix()

  it('снимок цел: матрица та же, что при съёмке', () => {
    expect(cases.length * Object.keys(CONTEXTS).length).toBe(snap.cases)
    expect(Object.keys(CONTEXTS).sort()).toEqual(Object.keys(snap.contexts).sort())
    for (const [name, c] of Object.entries(snap.contexts)) expect(c.level, name).toBe(CONTEXTS[name].level)
    const matched = Object.values(snap.contexts).reduce((n, c) => n + Object.values(c.actions).reduce((m, l) => m + l.length, 0), 0)
    expect(matched).toBe(snap.matched)
  })

  it('каждое отличие от снимка названо в INTENDED_DIFFS, с причиной', () => {
    const unexplained = diffs(cases).filter((f) => !INTENDED_DIFFS.some((r) => ruleMatches(r, f)))
    // Показываем не больше 40 строк: остальные тот же класс.
    expect(unexplained.slice(0, 40).map((f) => `${f.ctx} | ${f.token}: ${f.was} → ${f.now}`)).toEqual([])
  })

  it('каждая запись INTENDED_DIFFS встречается (нет мёртвых правил) и имеет причину с номером решения', () => {
    const found = diffs(cases)
    for (const r of INTENDED_DIFFS) {
      expect(r.why, String(r.token)).toMatch(/^074-\d+/)
      expect(
        found.some((f) => ruleMatches(r, f)),
        `${r.ctx} | ${String(r.token)}: ${r.was} → ${r.now}`,
      ).toBe(true)
    }
  })

  it('имена контекстов INTENDED_DIFFS существуют', () => {
    for (const r of INTENDED_DIFFS) if (r.ctx !== '*') expect(Object.keys(CONTEXTS), r.why).toContain(r.ctx)
  })

  it('опорные случаи (страховка от слепого расширения INTENDED_DIFFS)', () => {
    const at = (name: string, token: string) => legacy(keyCommand(matrix().find((c) => c.token === token)!.ev, ctxOf(CONTEXTS[name])))
    const same = (name: string, token: string, act: string | null) => expect(at(name, token), `${name} | ${token}`).toBe(act)
    // Русская раскладка — двойниками букв.
    same('холст', 'ctrl+я', 'undo')
    same('холст', 'ctrl+shift+Я', 'redo')
    same('холст', 'ctrl+н', 'redo')
    same('холст', 'ctrl+shift+Ы', 'saveAs')
    same('холст', 'а', 'toggleBig')
    // Повтор: переключатели молчат, сдвиг стрелкой — нет.
    same('холст', 'repeat+f', null)
    same('холст+сетка+место', 'repeat+ArrowRight', 'nudgePlace')
    // Del в ленте не трогает разворот; на холсте убирает.
    same('лента', 'Delete', null)
    same('холст', 'Delete', 'removeCell')
    // Режим фона забирает Esc, стрелки и Del.
    same('режим фона', 'Escape', 'bgExit')
    same('режим фона', 'ArrowLeft', 'bgNudge')
    same('режим фона', 'Backspace', 'bgRemove')
    same('режим фона+галерея', 'Escape', 'closeGallery')
    // Фокус в поле: всё молчит.
    for (const c of cases) expect(keyCommand(c.ev, ctxOf(CONTEXTS['поле ввода'])), c.token).toBeNull()
    // Отметки — только в ленте.
    same('лента', '3', 'markRating')
    same('холст', '3', null)
    same('лента+крупный просмотр', '3', null)
    // Уровни: на «Простом» зум и сдвиг места молчат.
    same('Простой: холст+сетка+место', 'ctrl+=', null)
    same('Простой: холст+сетка+место', 'ArrowRight', 'nextSpread')
    same('Стандартный: холст+сетка+место', 'ctrl+=', 'zoomIn')
    // «?» — справка, F1 — справочник.
    same('холст', '?', null) // «?» без Shift — не сочетание справки (на клавиатуре это Shift+/)
    same('холст', 'shift+?', 'help')
    same('холст', 'F1', 'guide')
  })

  it('Mac: Cmd вместо Ctrl — то, что Windows даёт на Ctrl, Mac даёт на Cmd (074-20)', () => {
    const ctx = (name: string) => ctxOf(CONTEXTS[name])
    const win = new Map<string, string | null>()
    for (const name of Object.keys(CONTEXTS)) for (const c of cases) win.set(`${name}|${c.token}`, keyCommand(c.ev, ctx(name)))
    setMacForTest(true)
    let checked = 0
    for (const name of Object.keys(CONTEXTS)) {
      for (const c of cases) {
        if (!c.token.includes('meta') || c.token.includes('ctrl')) continue
        const as = tokenOf(c.key, { ...c.m, meta: false, ctrl: true })
        expect(keyCommand(c.ev, ctx(name)), `${name} | ${c.token}`).toBe(win.get(`${name}|${as}`))
        checked++
      }
      // Ctrl на Mac — не наш модификатор: никакой Ctrl-случай не срабатывает.
      for (const c of cases) if (c.token.includes('ctrl')) expect(keyCommand(c.ev, ctx(name)), `${name} | ${c.token}`).toBeNull()
    }
    expect(checked).toBeGreaterThan(1000)
  })

  it('глушение браузера: всё, что гасилось, гасится (reload — стражем), лишнего не гасится', () => {
    const NEW_OF: Record<string, string[]> = {
      markRating: ['markRating0', 'markRating1'],
      markStep: ['markUp', 'markDown'],
      markLabel: ['markRed', 'markBlue'],
    }
    const oldNames = new Set(snap.prevent)
    for (const old of snap.prevent) {
      if (old === 'reload') continue
      for (const id of NEW_OF[old] ?? [old]) expect(preventsDefault(id), id).toBe(true)
    }
    // Ctrl+R: прежняя команда reload → страж браузера
    expect(oldNames.has('reload')).toBe(true)
    expect(guardsBrowser(eventOf('r', { ctrl: true }))).toBe(true)
    // Не гасилось раньше — не гасится и теперь (кроме любых совпавших сочетаний с Ctrl/Alt: проверка по событию)
    for (const id of [
      'toggleBig',
      'toggleDetails',
      'toggleSafe',
      'prevSpread',
      'nextSpread',
      'removeCell',
      'addSelected',
      'closeGallery',
      'clearSelection',
      'bgExit',
      'bgRemove',
      'markUp',
    ])
      expect(preventsDefault(id), id).toBe(false)
  })

  it('подписи клавиш в меню: те же, что показывала таблица (кроме названных отличий)', () => {
    // Составные прежние подписи (несколько клавиш в одной строке) сверяются отдельно в keys.test / menu.test.
    const COMPOSITE = new Set(['markRating', 'markStep', 'markLabel', 'nudgePlace', 'bgNudge', 'bgRemove', 'spaceDown', 'reload'])
    // Первая клавиша Go-каталога у «Вписать» — 0, а не Ctrl+0 (порядок Defaults, G1): подпись пункта «Вид ▸ Вписать».
    const CHANGED: Record<string, string> = { fitZoom: '0' }
    let checked = 0
    for (const h of snap.hints) {
      if (COMPOSITE.has(h.action)) continue
      const was = h.menu ?? h.show
      if (/[ /(]/.test(was)) continue // «Ctrl+A (на развороте)» без menu — справочный показ
      expect(keymap.hint(h.action), h.action).toBe(CHANGED[h.action] ?? was)
      checked++
    }
    expect(checked).toBeGreaterThan(20)
  })
})

describe('правила окон App.svelte после переноса на реестр (было: список underDialog и modal)', () => {
  const files = import.meta.glob('/src/App.svelte', { query: '?raw', import: 'default', eager: true }) as Record<string, string>
  const app = (files['/src/App.svelte'] ?? '').replace(/\s+/g, ' ')
  const at = (s: string) => app.indexOf(s)

  it('порядок в keys(): F1, диспетчер, глушение браузера, команда', () => {
    const order = ['if (f1(e)) return', 'keyCommand(e, {', 'guardsBrowser(e)', 'preventsDefault(id, e)', 'runKey(id, e)']
    const pos = order.map(at)
    expect(
      pos.every((p) => p >= 0),
      order.join(' < '),
    ).toBe(true)
    expect([...pos].sort((a, b) => a - b)).toEqual(pos)
  })
  it('списки окон ушли: окно определяет стек Modal (modalStack), а не перечисление флагов', () => {
    expect(app).not.toContain('underDialog')
    expect(app).not.toContain('modal:')
    expect(app).toContain('dialog: dialogState()')
    expect(app).toContain('modalStack.topUndo()')
  })
  it('F1 открыт отдельно (поверх стартового окна), как и раньше', () => {
    expect(app).toContain("eventChord(e) !== 'F1'")
  })
  it('прежние перечни окон в снимке были: список окон снимка не пуст', () => {
    expect((snap.app.underDialog as string[]).length).toBeGreaterThan(3)
  })
})
