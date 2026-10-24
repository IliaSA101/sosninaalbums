import { describe, expect, it } from 'vitest'
import { COMMANDS } from './commands'
import { MOUSE_HELP } from './mousehelp'
import { catalogView } from '../../base/keys/keymap.fixture'
import { keyMarks } from '../../base/keys/keytext'
import { ARTICLES } from '../guide/articles'

// Страж 074: командные клавиши живут в одном месте — в реестре (каталог Go + свои), а не в обработчиках и текстах.
// Иначе после переназначения подсказка врёт, а второй обработчик ловит ту же клавишу. По образцу platform.guard.test.ts.
const files = import.meta.glob('/src/**/*.{ts,svelte}', { query: '?raw', import: 'default', eager: true }) as Record<string, string>
const isTest = (p: string) => /\.test\.ts$/.test(p)
const sources = Object.entries(files).filter(([p]) => !isTest(p))
const inKeys = (p: string) => p.startsWith('/src/base/keys/')

// Код без комментариев: тексты в комментариях и документации ловить незачем. `//` считается комментарием, если перед ним
// чётное число кавычек (адреса вида https:// в строках остаются).
function code(src: string): string {
  const noBlocks = src
    .replace(/\/\*[\s\S]*?\*\//g, (m) => m.replace(/[^\n]/g, ' '))
    .replace(/<!--[\s\S]*?-->/g, (m) => m.replace(/[^\n]/g, ' '))
  return noBlocks
    .split('\n')
    .map((line) => {
      for (let i = line.indexOf('//'); i >= 0; i = line.indexOf('//', i + 2)) {
        const before = line.slice(0, i)
        const quotes = (before.match(/['"`]/g) ?? []).length
        if (quotes % 2 === 0 && (i === 0 || /\s|[;,{}()]/.test(line[i - 1]))) return before
      }
      return line
    })
    .join('\n')
}
const lineOf = (src: string, at: number) => src.slice(0, at).split('\n').length

describe('обработчики клавиш мимо реестра', () => {
  // Окна и жесты, которым нужны свои клавиши фокуса: Esc отменяет жест или закрывает меню и окно, Enter подтверждает,
  // стрелки — навигация внутри списка или меню, Ctrl+Enter — «Вставить» в окне имён. Командных клавиш (буквы, цифры,
  // Del, Ctrl+буква) среди них нет и быть не должно: команда — в каталог internal/keymap.
  const ALLOWED: Record<string, string> = {
    '/src/App.svelte': 'диспетчер: единственное место командных клавиш',
    '/src/base/modal/Modal.svelte': 'окно: Esc закрывает, фокус не уходит под окно',
    '/src/base/ui/Popover.svelte': 'Esc закрывает всплывающее окно',
    '/src/base/color/ColorPicker.svelte': 'Esc закрывает палитру и отменяет пипетку',
    '/src/base/ui/MenuList.svelte': 'навигация и Esc общего списка меню (меню фото, «⋯» панели)',
    '/src/base/drag/drag.svelte.ts': 'Esc отменяет перенос',
    '/src/editor/backdrop/BackdropEditor.svelte': 'Esc отменяет жест рамки фона',
    '/src/editor/canvas/Marquee.svelte': 'Esc отменяет рамку выделения',
    '/src/editor/canvas/pickunder.svelte.ts': 'Alt/Esc жеста «выбрать нижний объект»',
    '/src/editor/captions/FieldMenu.svelte': 'навигация и Esc меню',
    '/src/editor/eyedrop/EyedropLayer.svelte': 'Esc выходит из пипетки',
    '/src/editor/photo/FrameControls.svelte': 'Esc жеста кадра',
    '/src/editor/photo/PhotoPreview.svelte': 'Esc закрывает крупный просмотр; остальные клавиши страницу не трогают',
    '/src/editor/places/MiniPanel.svelte': 'Esc наклона и модификатор Alt/Shift панели (owner-ux §2)',
    '/src/editor/places/PlaceAddMenu.svelte': 'навигация и Esc меню',
    '/src/editor/places/placegesture.ts': 'Esc отменяет жест места',
    '/src/editor/text/FontPicker.svelte': 'навигация по списку шрифтов',
    '/src/editor/text/textgesture.svelte.ts': 'Esc отменяет жест текста',
    '/src/editor/tilt/StraightenLayer.svelte': 'Esc режима «Выпрямить»',
    '/src/editor/tilt/TiltDial.svelte': '←/→ и Shift у дуги наклона (owner-ux §2.4)',
    '/src/editor/zoom/ZoomBadge.svelte': 'Esc закрывает бегунок масштаба',
    '/src/panels/layouts/MirrorMenu.svelte': 'навигация и Esc меню',
    '/src/panels/names/NamesDialog.svelte': 'Ctrl+Enter «Вставить» в окне имён',
    '/src/shell/guide/spotstate.svelte.ts': 'Esc гасит подсветку места',
    '/src/shell/hotkeys/KeysDialog.svelte':
      'окно «Горячие клавиши»: нажатие выбирает клавишу на карте, при записи его забирает запись (074)',
    '/src/shell/menu/LevelBadge.svelte': 'навигация и Esc меню уровня',
    '/src/shell/menu/MenuBar.svelte': 'навигация по меню',
  }
  const HANDLER = /addEventListener\(\s*['"]keydown['"]|<svelte:(?:window|document|body)\b[\s\S]*?\/>/g

  const hasHandler = (src: string): boolean =>
    [...code(src).matchAll(HANDLER)].some((m) => m[0].startsWith('addEventListener') || /\bonkeydown(?:capture)?\s*=/.test(m[0]))
  const handlers = (): string[] =>
    sources
      .filter(([, src]) => hasHandler(src))
      .map(([p]) => p)
      .sort()

  it('самопроверка: страж видит обработчики на window и document, но не на элементе', () => {
    expect(hasHandler('<svelte:window onresize={a} onkeydowncapture={(e) => b(e)} />')).toBe(true)
    expect(hasHandler('<svelte:window\n  onkeydown={key}\n/>')).toBe(true)
    expect(hasHandler("document.addEventListener('keydown', f)")).toBe(true)
    expect(hasHandler('<svelte:window onresize={a} />')).toBe(false)
    expect(hasHandler('<div onkeydown={key}></div>')).toBe(false)
    expect(hasHandler("// addEventListener('keydown', f)")).toBe(false)
  })
  it('обработчик keydown на window/document — только в списке с причиной; новая команда — в каталог internal/keymap', () => {
    const extra = handlers().filter((p) => !(p in ALLOWED))
    expect(extra, 'команда — в каталог internal/keymap (клавиши меняет пользователь); местная клавиша — в ALLOWED с причиной').toEqual([])
  })
  it('в списке нет лишнего: удалённый обработчик убирается из ALLOWED', () => {
    const have = new Set(handlers())
    expect(Object.keys(ALLOWED).filter((p) => !have.has(p))).toEqual([])
  })
})

describe('клавиши в обработчиках: только навигация', () => {
  // Литералы, с которыми разрешено сравнивать event.key вне base/keys: Esc, Enter, Tab, стрелки, Home/End/PageUp/PageDown, Пробел.
  const NAV = new Set([
    'Escape',
    'Enter',
    'Tab',
    ' ',
    'ArrowLeft',
    'ArrowRight',
    'ArrowUp',
    'ArrowDown',
    'Home',
    'End',
    'PageUp',
    'PageDown',
  ])

  function offenders(src: string): string[] {
    const c = code(src)
    const out: string[] = []
    const add = (lit: string, at: number) => {
      if (!NAV.has(lit)) out.push(`${lineOf(c, at)}: '${lit}'`)
    }
    for (const m of c.matchAll(/\.key\s*(?:===|!==|==|!=)\s*(['"])(.*?)\1/g)) add(m[2], m.index!)
    for (const m of c.matchAll(/(['"])(.*?)\1\s*(?:===|!==|==|!=)\s*[\w.]+\.key\b/g)) add(m[2], m.index!)
    if (/\.key\.toLowerCase\(\)/.test(c)) out.push(`${lineOf(c, c.search(/\.key\.toLowerCase\(\)/))}: key.toLowerCase()`)
    // switch (e.key) { case 'x': … }
    for (const sw of c.matchAll(/switch\s*\(([^)]*\.key[^)]*)\)\s*\{/g)) {
      let depth = 1
      let i = sw.index! + sw[0].length
      const start = i
      for (; i < c.length && depth > 0; i++) depth += c[i] === '{' ? 1 : c[i] === '}' ? -1 : 0
      for (const cs of c.slice(start, i).matchAll(/case\s+(['"])(.*?)\1\s*:/g)) add(cs[2], start + cs.index!)
    }
    // ['x', 'y'].includes(e.key)
    for (const m of c.matchAll(/\[([^\]]*)\]\.includes\(\s*[\w.]+\.key\s*\)/g))
      for (const l of m[1].matchAll(/(['"])(.*?)\1/g)) add(l[2], m.index!)
    return out
  }

  it('event.key сравнивается только с Esc, Enter, Tab, стрелками, Home/End/PageUp/PageDown и Пробелом', () => {
    const bad: string[] = []
    for (const [path, src] of sources) if (!inKeys(path)) for (const o of offenders(src)) bad.push(`${path}:${o}`)
    expect(bad, 'командная клавиша (буква, цифра, Del, знак) — только через реестр: каталог internal/keymap + commands.ts').toEqual([])
  })
  it('сам страж ловит командную клавишу (самопроверка)', () => {
    expect(offenders("if (e.key === 'z') go()")).toHaveLength(1)
    expect(offenders("if (e.key === 'Delete') go()")).toHaveLength(1)
    expect(offenders("if (e.key.toLowerCase() === 'f') go()")).toHaveLength(1)
    expect(offenders("switch (e.key) { case 'Enter': a(); break; case 'f': b() }")).toHaveLength(1)
    expect(offenders("if (['a', 'ArrowUp'].includes(e.key)) go()")).toHaveLength(1)
    expect(offenders("if (e.key === 'Enter' || e.key !== 'Escape') go() // e.key === 'z'")).toHaveLength(0)
  })

  it('event.code читает только base/keys/chord.ts', () => {
    const bad: string[] = []
    for (const [path, src] of sources) {
      if (path === '/src/base/keys/chord.ts') continue
      const c = code(src)
      for (const m of c.matchAll(/\b(?:e|ev|evt|event|ke)\.code\b/g)) bad.push(`${path}:${lineOf(c, m.index!)}`)
    }
    expect(bad, 'код клавиши — eventChord из base/keys/chord.ts').toEqual([])
  })

  it('копий набора клавиш виджета (Set со стрелками и PageUp) нет: это правило диспетчера (074-19)', () => {
    const bad: string[] = []
    for (const [path, src] of sources) {
      if (path === '/src/shell/menu/keys.ts' || inKeys(path)) continue
      for (const m of code(src).matchAll(/new Set\(\s*\[[^\]]*'PageUp'[^\]]*\]\s*\)/g)) bad.push(`${path}:${m.index}`)
    }
    expect(bad, 'стрелки ползунка и списка оставляет виджету диспетчер (WIDGETS в shell/menu/keys.ts)').toEqual([])
  })
})

describe('клавиши в текстах', () => {
  // Текст с клавишей команды берёт её из реестра (keymap.hint / tip / метка {key:id}). Жесты мыши, местные клавиши полей
  // и чужие программы названы в исключениях с причиной.
  // Запись сочетания (Mod+Shift+KeyR, Alt+ArrowLeft) — не показ: исключена по имени кода.
  const KEY_TEXT = /\b(?:Ctrl|Cmd|Alt|Shift)\+(?!Key[A-Z]|Digit|Arrow|Bracket|Slash)[A-Za-z0-9[\]=−+]/g
  const IN_PARENS =
    /\((?:(?:Ctrl|Cmd|Alt|Shift)\+(?:[A-Za-z0-9[\]=−+][^)]*|[Пп]робел)|Del|Delete|Backspace|F|I|G|P|X|U|[Пп]робел|←|→|↑|↓)\)/g
  const EXCEPT: Record<string, { text: RegExp; why: string }> = {
    '/src/panels/names/NamesDialog.svelte': { text: /Ctrl\+Enter/, why: 'местная клавиша окна «Имена» (Ctrl+Enter в тексте)' },
    '/src/panels/folders/FolderMenu.svelte': { text: /Ctrl\+S/, why: 'Ctrl+S в Lightroom (чужая программа)' },
    '/src/base/prefs/levels.ts': {
      text: /Ctrl\+[AC]/,
      why: 'названия функций «Выделение рамкой, Ctrl+A, удаление группы» и «Копировать и вставить (Ctrl+C, X, V, D)» в таблице уровней (099)',
    },
  }
  const textOffenders = (path: string, src: string): string[] => {
    if (inKeys(path) || path === '/src/shell/menu/mousehelp.ts') return []
    // Справочные строки мыши и жестов (mousehelp.ts) — готовые тексты, их show в коде не считается
    let c = code(src)
    for (const r of MOUSE_HELP) c = c.split(r.show).join(' ')
    const ex = EXCEPT[path]
    const out: string[] = []
    for (const re of [KEY_TEXT, IN_PARENS]) {
      for (const m of c.matchAll(re)) {
        const line = c.split('\n')[lineOf(c, m.index!) - 1]
        if (ex && ex.text.test(line)) continue
        out.push(`${lineOf(c, m.index!)}: ${m[0]}`)
      }
    }
    return out
  }

  it('нет «Ctrl+Z», «Shift+…», «(Del)», «(F)», «(Shift+пробел)» руками: клавиша — из реестра', () => {
    const bad: string[] = []
    for (const [path, src] of sources) for (const o of textOffenders(path, src)) bad.push(`${path}:${o}`)
    expect(bad, 'keymap.hint(id) / keymap.tip(подпись, id) / метка {key:id} в статьях').toEqual([])
  })
  it('самопроверка: страж видит запрещённые тексты и пропускает жесты мыши', () => {
    expect(textOffenders('/src/x.ts', "title = 'Отменить (Ctrl+Z)'")).toHaveLength(2)
    expect(textOffenders('/src/x.ts', "title = 'Убрать (Del)'")).toHaveLength(1)
    expect(textOffenders('/src/x.ts', "title = 'Раскладка (Shift+пробел)'")).toHaveLength(1)
    expect(textOffenders('/src/x.ts', "title = 'Крупный просмотр (F)'")).toHaveLength(1)
    expect(textOffenders('/src/x.ts', "title = 'Ctrl+колесо — масштаб'")).toHaveLength(0)
    expect(textOffenders('/src/x.ts', "title = 'Закрыть (Esc)'")).toHaveLength(0)
    expect(textOffenders('/src/x.ts', '// Ctrl+Z в комментарии\nconst a = 1 /* Ctrl+Z (Del) */')).toHaveLength(0)
    expect(textOffenders('/src/base/keys/chordtext.ts', "'Ctrl+Z'")).toHaveLength(0)
  })
})

describe('id клавиш в статьях справочника и меню', () => {
  const ids = new Set(catalogView.commands.map((e) => e.id))
  const mouse = new Set(MOUSE_HELP.map((r) => r.show))

  it('keys статьи F1 — id каталога или справочная строка мыши', () => {
    const bad: string[] = []
    for (const a of ARTICLES) for (const k of a.keys ?? []) if (!ids.has(k) && !mouse.has(k)) bad.push(`${a.id}: ${k}`)
    expect(bad).toEqual([])
  })
  it('метки {key:id} в статьях — id каталога', () => {
    const bad: string[] = []
    for (const a of ARTICLES) {
      const text = [a.what, a.where, a.how, a.miss?.hint].filter(Boolean).join(' ')
      for (const id of keyMarks(text)) if (!ids.has(id)) bad.push(`${a.id}: ${id}`)
    }
    expect(bad).toEqual([])
  })
  it('метки {key:id} в исходниках — id каталога', () => {
    const bad: string[] = []
    for (const [path, src] of sources) for (const id of keyMarks(src)) if (!ids.has(id) && !inKeys(path)) bad.push(`${path}: ${id}`)
    expect(bad).toEqual([])
  })
  it('каждая команда каталога есть в COMMANDS (команда без обработчика молча не сработает)', () => {
    for (const e of catalogView.commands) expect(typeof (COMMANDS as Record<string, unknown>)[e.id], e.id).toBe('function')
  })
})

describe('окна: отмену пускают только окна правки (074-28)', () => {
  // Modal с undo — под окном работают «Отменить» и «Повторить». Остальные окна (вопросы, сохранение, экспорт, сбор,
  // открытие, автосборка, уровни…) клавиш не пускают. Список закрытый: новое окно правки — осознанное решение.
  const UNDO_WINDOWS = [
    '/src/panels/decor/DecorDialog.svelte',
    '/src/panels/names/NamesDialog.svelte',
    '/src/panels/vignette/VigDialog.svelte',
  ]
  it('undo задан у «Фонов» (верхний уровень), «Имён» и «Виньетки», больше ни у кого', () => {
    const have = sources.filter(([, src]) => /<Modal\b[^>]*\sundo\b/.test(src)).map(([p]) => p)
    expect(have.sort()).toEqual(UNDO_WINDOWS)
    const decor = files['/src/panels/decor/DecorDialog.svelte']
    expect(decor.match(/<Modal\b/g)).toHaveLength(5) // вопросы поверх «Фонов» (имя, убрать, разворот, фон на набор) — без undo
    expect(decor.match(/<Modal\b[^>]*\sundo\b/g)).toHaveLength(1)
  })
})

describe('страж видит файлы', () => {
  it('исходники и тест-вектора на месте', () => {
    expect(files['/src/shell/menu/keys.ts']).toContain('keyCommand')
    expect(files['/src/base/keys/chord.ts']).toContain('eventChord')
    expect(Object.keys(files).length).toBeGreaterThan(100)
    expect(catalogView.commands.length).toBeGreaterThan(80)
  })
})
