import { describe, expect, it } from 'vitest'

// Страж текстов (К2 ревизии тестов 24.10): надписи ловят только QA и владелец, а они повторяются одними и теми же видами.
// 1) Тост и ошибка начинаются с заглавной буквы: ошибки Go пишутся со строчной — перед показом их поднимает capFirst.
// 2) Число + существительное — только через plural() (base/words/plural.ts): «${n} файлов» вручную врёт при 1, 2 и 21.
// Исключение строки — маркер в комментарии на этой же или предыдущей строке: `caps-ok: причина` / `plural-ok: причина`
// (как lang-ok у комментариев). Словаря «запрещённых» синонимов здесь нет: это сверка с владельцем.
// Сканер — по образцу shell/menu/keys.guard.test.ts (import.meta.glob по исходникам).
const files = import.meta.glob('/src/**/*.{ts,svelte}', { query: '?raw', import: 'default', eager: true }) as Record<string, string>
const sources: [string, string][] = Object.entries(files).filter(([p]) => !/\.test\.ts$/.test(p))

// ---------- общие помощники ----------

// Код без комментариев; длина и переводы строк сохраняются, чтобы смещения совпадали с исходником.
// `//` — комментарий, если перед ним чётное число кавычек (адреса https:// в строках остаются).
function code(src: string): string {
  const blank = (m: string) => m.replace(/[^\n]/g, ' ')
  const noBlocks = src.replace(/\/\*[\s\S]*?\*\//g, blank).replace(/<!--[\s\S]*?-->/g, blank)
  return noBlocks
    .split('\n')
    .map((line) => {
      for (let i = line.indexOf('//'); i >= 0; i = line.indexOf('//', i + 2)) {
        const quotes = (line.slice(0, i).match(/['"`]/g) ?? []).length
        if (quotes % 2 === 0 && (i === 0 || /\s|[;,{}()]/.test(line[i - 1]))) return line.slice(0, i) + ' '.repeat(line.length - i)
      }
      return line
    })
    .join('\n')
}
const lineOf = (src: string, at: number) => src.slice(0, at).split('\n').length

// Строка помечена исключением: маркер в комментарии на ней или на предыдущей.
const waived = (raw: string, line: number, marker: string): boolean => {
  const lines = raw.split('\n')
  return [lines[line - 1], lines[line - 2]].some((l) => l !== undefined && new RegExp(`(?://|/\\*|<!--)[^\\n]*${marker}`).test(l))
}

// ---------- обход кода с учётом строк и скобок ----------

const OPEN = '([{'
const CLOSE = ')]}'
const isQuote = (c: string) => c === "'" || c === '"' || c === '`'

// Конец строкового литерала, открытого кавычкой в позиции i: индекс закрывающей кавычки (шаблон с ${…} учтён).
function stringEnd(s: string, i: number): number {
  const q = s[i]
  for (let j = i + 1; j < s.length; j++) {
    if (s[j] === '\\') j++
    else if (s[j] === q) return j
    else if (q === '`' && s[j] === '$' && s[j + 1] === '{') j = walk(s, j + 2, (c, _i, d) => c === '}' && d === 0)
  }
  return s.length
}

// Идёт от from по коду, пропуская строки, и возвращает позицию, где stop(символ, позиция, глубина скобок) стал истиной.
function walk(s: string, from: number, stop: (c: string, i: number, depth: number) => boolean): number {
  let depth = 0
  for (let i = from; i < s.length; i++) {
    const c = s[i]
    if (isQuote(c)) i = stringEnd(s, i)
    else if (stop(c, i, depth)) return i
    else if (OPEN.includes(c)) depth++
    else if (CLOSE.includes(c)) depth--
  }
  return s.length
}

// Конец выражения: первая запятая или закрывающая скобка на нулевой глубине.
const exprEnd = (s: string, from: number): number => walk(s, from, (c, _i, d) => d === 0 && (c === ',' || CLOSE.includes(c)))

// Конец блока, открытого скобкой в позиции open.
const bodyEnd = (s: string, open: number): number => walk(s, open, (c, _i, d) => c === '}' && d === 1)

// Конец оператора `const x = …`: перевод строки на нулевой глубине, если выражение на нём не продолжается.
function stmtEnd(s: string, from: number): number {
  return walk(s, from, (c, i, d) => {
    if (c !== '\n' || d > 0) return false
    const head = s.slice(from, i).trimEnd()
    const next = s.slice(i).trimStart()[0] ?? ''
    return !/(?:=>|[=?:,+&|(])$/.test(head) && !/[?:.+&|]/.test(next)
  })
}

// ---------- 1. заглавная буква в тостах и ошибках ----------

const LOWER = /^\p{Ll}/u

// С литерала начинается текст, если перед ним ничего нет, `?`, `||`, `??`, `=>`, `return` или (вторая ветка тернарного)
// двоеточие после другого литерала. «kind: 'info'» и «x: 'step'» — не текст.
function startsText(before: string): boolean {
  const prev = before.trimEnd()
  if (prev === '' || /(?:\?|\|\||\?\?|=>|\breturn)$/.test(prev)) return true
  return prev.endsWith(':') && prev.includes('?') && isQuote(prev.slice(0, -1).trimEnd().slice(-1))
}

// Позиции первого символа текстовых литералов выражения (внутри кавычки).
function textStarts(expr: string): number[] {
  const res: number[] = []
  for (let i = 0; i < expr.length; i++) {
    if (!isQuote(expr[i])) continue
    if (startsText(expr.slice(0, i))) res.push(i + 1)
    i = stringEnd(expr, i)
  }
  return res
}

interface Finding {
  where: string
  text: string
}

// Выражения, которыми определение отдаёт текст: после `return`, после `=>` без блока или правая часть константы целиком.
// Промежуточные `const what = …` внутри тела не берём: из них собирают фразу, и в середине она вправе быть строчной.
function results(text: string): { off: number; expr: string }[] {
  const out: { off: number; expr: string }[] = []
  for (const m of text.matchAll(/\breturn\b/g)) {
    const off = m.index + m[0].length
    out.push({ off, expr: text.slice(off, stmtEnd(text, off)) })
  }
  const arrow = /=>\s*(?!\{)/.exec(text)
  if (arrow && out.length === 0) out.push({ off: arrow.index + arrow[0].length, expr: text.slice(arrow.index + arrow[0].length) })
  else if (!/^\s*(?:async\s*)?\(.*\)\s*(?::[^=]*)?=>/s.test(text) && !text.trimStart().startsWith('{')) out.push({ off: 0, expr: text })
  return out
}

interface Def {
  p: string
  raw: string
  base: number
  text: string
  exported: boolean
}

// Определения (const, let, function) файла: имя → правая часть константы или тело функции.
function definitionsOf(p: string, raw: string, src: string, into: Map<string, Def[]>): void {
  for (const m of src.matchAll(/\b(export\s+)?(?:(const|let)|function)\s+([A-Za-z_]\w{3,})\b/g)) {
    const from = m.index + m[0].length
    const isConst = !!m[2]
    const base = isConst ? src.indexOf('=', from) + 1 : src.indexOf('{', src.indexOf(')', from))
    if (base <= 0) continue
    const arrow = isConst ? /^\s*(?:async\s*)?\([^)]*\)\s*(?::\s*[\w<>[\]| ]+)?\s*=>\s*\{/.exec(src.slice(base)) : null
    const end = isConst && !arrow ? stmtEnd(src, base) : bodyEnd(src, arrow ? base + arrow[0].length - 1 : base)
    const list = into.get(m[3]) ?? []
    list.push({ p, raw, base, text: src.slice(base, end), exported: !!m[1] })
    into.set(m[3], list)
  }
}

// Тосты: первый аргумент notify()/undoToast(). Литерал — проверяем сразу; вызов сборщика текста — по его определению
// (в этом же файле или экспортируемому откуда-то ещё); константы с текстом тоста — по имени (*_TEXT, *Toast…).
function toastFindings(list: [string, string][]): Finding[] {
  const out: Finding[] = []
  const stripped = list.map(([p, raw]) => [p, raw, code(raw)] as const)
  const add = (p: string, raw: string, at: number, text: string) => {
    const line = lineOf(raw, at)
    if (!waived(raw, line, 'caps-ok')) out.push({ where: `${p}:${line}`, text: text.trim().slice(0, 40) })
  }
  const defs = new Map<string, Def[]>()
  for (const [p, raw, src] of stripped) definitionsOf(p, raw, src, defs)

  const checkDefs = (name: string, from: string) => {
    const all = defs.get(name) ?? []
    const own = all.filter((d) => d.p === from)
    for (const d of own.length > 0 ? own : all.filter((x) => x.exported)) {
      for (const c of results(d.text)) {
        for (const at of textStarts(c.expr)) {
          if (LOWER.test(c.expr[at] ?? '')) add(d.p, d.raw, d.base + c.off + at, `${name}: ${c.expr.slice(at, at + 30)}`)
        }
      }
    }
  }
  const checkCall = (p: string, raw: string, src: string, open: number) => {
    const arg = src.slice(open, exprEnd(src, open))
    if (/^\s*\w+\??:\s*[A-Za-z]/.test(arg)) return // объявление метода `notify(text: string…`, не вызов
    for (const at of textStarts(arg)) if (LOWER.test(arg[at] ?? '')) add(p, raw, open + at, arg.slice(at, at + 30))
    const id = /^\s*(?:\w+\.)?([A-Za-z_]\w*)\s*(?:\(|$)/.exec(arg)
    if (id) checkDefs(id[1], p)
  }

  for (const [p, raw, src] of stripped) {
    for (const m of src.matchAll(/(?<![\w$])(?:\w+\.)*(?:notify|undoToast)\s*\(/g)) checkCall(p, raw, src, m.index + m[0].length)
    // Константы с текстом тоста по имени: *_TEXT, *_TOAST, *Toast, *Text со строковым значением.
    for (const m of src.matchAll(/\bconst\s+([A-Za-z_]\w*(?:_TEXT|_TOAST|Toast|Text))\s*(?::\s*string\s*)?=\s*(['"`])/g)) {
      const at = m.index + m[0].length
      if (LOWER.test(src[at] ?? '')) add(p, raw, at, `${m[1]}: ${src.slice(at, at + 30)}`)
    }
  }
  return out
}

// Ошибки Go (errText) перед показом поднимает capFirst; notify(…, 'error') делает это сам (Toasts.notify).
function errorFindings(list: [string, string][]): Finding[] {
  const out: Finding[] = []
  for (const [p, raw] of list) {
    const src = code(raw)
    for (const m of src.matchAll(/\berrText\(/g)) {
      const before = src.slice(0, m.index)
      const line = lineOf(raw, m.index)
      const lineText = src.split('\n')[line - 1]
      if (/capFirst\(\s*$/.test(before)) continue
      if (/function\s+errText/.test(lineText)) continue
      // тост красного вида: поднимет Toasts.notify
      if (/\bnotify\(/.test(lineText) && /'error'/.test(lineText)) continue
      if (waived(raw, line, 'caps-ok')) continue
      out.push({ where: `${p}:${line}`, text: lineText.trim().slice(0, 60) })
    }
  }
  return out
}

describe('тексты: заглавная буква', () => {
  it('самопроверка: тост со строчной ловится, с заглавной и «Вернуть» нет', () => {
    expect(toastFindings([['/a.ts', "store.notify('ошибка')"]])).toHaveLength(1)
    expect(toastFindings([['/a.ts', 'store.notify(ok ? `готово` : "Готово")']])).toHaveLength(1)
    expect(toastFindings([['/a.ts', "store.notify('Готово')\nstore.notify(`${n} фото`)\nstore.notify('«Да»')"]])).toEqual([])
    expect(toastFindings([['/a.ts', "const GONE_TEXT = 'пропало'\nstore.notify(GONE_TEXT)"]])).toHaveLength(2)
    expect(toastFindings([['/a.ts', "const flagText = (n: number) => (n ? 'нет' : 'Есть')\nnotify(flagText(1))"]])).toHaveLength(1)
    expect(toastFindings([['/a.ts', "// caps-ok: продолжение фразы\nstore.notify('и ещё')"]])).toEqual([])
    expect(toastFindings([['/a.ts', "notify(text: string, kind = 'info')"]])).toEqual([])
  })

  it('самопроверка: errText без capFirst ловится, в тосте error и под маркером нет', () => {
    expect(errorFindings([['/a.ts', 'this.error = errText(e)']])).toHaveLength(1)
    expect(errorFindings([['/a.ts', 'this.error = capFirst(errText(e))']])).toEqual([])
    expect(errorFindings([['/a.ts', "store.notify(errText(e), 'error')"]])).toEqual([])
    expect(errorFindings([['/a.ts', '// caps-ok: идёт в середину фразы\nconst why = errText(e)']])).toEqual([])
    expect(errorFindings([['/a.ts', '// errText(e) в комментарии не считается']])).toEqual([])
  })

  it('страж видит исходники', () => {
    expect(sources.length).toBeGreaterThan(300)
    expect(sources.some(([, s]) => /\bnotify\(/.test(s))).toBe(true)
  })

  it('тосты начинаются с заглавной буквы', () => {
    expect(toastFindings(sources), 'тост: заглавная; продолжение фразы — маркер caps-ok: причина').toEqual([])
  })

  it('текст ошибки Go перед показом проходит capFirst', () => {
    expect(errorFindings(sources), 'обернуть в capFirst(errText(e)) или маркер caps-ok: причина').toEqual([])
  })
})

// ---------- 2. число + существительное — через plural() ----------

// Склоняемые существительные предметной области. «фото» не склоняется («5 фото») и сюда не входит.
const NOUN =
  '(?:файл|разворот|страниц|лист|кадр|папк|снимк|шаблон|подборк|копи[яйию]|шрифт|набор|подпис|надпис|имен|имён|мест[оае]?|слов|слой|слоёв|слоев|объект|элемент|раскладк|виньетк|групп|альбом|строк|ученик|учител|колонтитул|символ|цвет|человек|минут|секунд|рамк|стикер|вариант|абзац|букв|пункт)'
const WORD = `${NOUN}[а-яё]*(?![а-яё])`
// Выражение похоже на число: счётчик по имени, длина массива, Math.*. Прочее («${who} страницы») — не счёт.
const NUMERIC =
  /(?:^|[\s.(!])(?:n|k|f|g|cnt|count|total|num|len|length|max|min|done|left|rest|extra|named|seats|missing|added|sum|size)(?:$|[\s.)])|\.length|\bMath\./i

function pluralFindings(list: [string, string][]): Finding[] {
  const out: Finding[] = []
  for (const [p, raw] of list) {
    const src = code(raw)
    const add = (at: number, text: string) => {
      const line = lineOf(raw, at)
      if (!waived(raw, line, 'plural-ok')) out.push({ where: `${p}:${line}`, text: text.slice(0, 60) })
    }
    const counted = (re: RegExp) => {
      for (const m of src.matchAll(re)) if (NUMERIC.test(m[1])) add(m.index, m[0])
    }
    // `${n} файлов` в шаблонной строке
    counted(new RegExp(`\\$\\{([^{}]+)\\}[ \\u00a0]+${WORD}`, 'gi'))
    // `{n} файлов` в разметке .svelte (не `{#if}`, не `{@const}`)
    if (p.endsWith('.svelte')) counted(new RegExp(`(?<!\\$)\\{(?![#/:@])([^{}]+)\\}[ \\u00a0]+${WORD}`, 'gi'))
    // `n + ' файлов'`
    counted(new RegExp(`([\\w.)\\]]+)\\s*\\+\\s*['"\`]\\s+${WORD}`, 'gi'))
    // «фото(в)», «файл(ов)»: скобочные окончания
    for (const m of src.matchAll(/['"`][^'"`\n]*[а-яё]\((?:в|ов|ев|ей|а|ы|и|ий|ок)\)/gi)) add(m.index, m[0])
    // `n === 1 ? 'файл' : 'файлов'`: ручной выбор формы слова по числу
    for (const m of src.matchAll(
      new RegExp(`===?\\s*1\\s*\\)?\\s*\\?\\s*(['"\`])\\s*${WORD}\\s*\\1\\s*:\\s*(['"\`])\\s*${WORD}\\s*\\2`, 'gi'),
    ))
      add(m.index, m[0])
  }
  return out
}

describe('тексты: число + существительное через plural()', () => {
  it('самопроверка: ручные формы ловятся, plural(), «фото» и маркер нет', () => {
    expect(pluralFindings([['/a.ts', 'const t = `${n} файлов`']])).toHaveLength(1)
    expect(pluralFindings([['/a.ts', 'const t = `Выгружено ${a.count} разворота`']])).toHaveLength(1)
    expect(pluralFindings([['/a.svelte', '<p>{count} подписей</p>']])).toHaveLength(1)
    expect(pluralFindings([['/a.ts', "const t = n + ' шаблонов'"]])).toHaveLength(1)
    expect(pluralFindings([['/a.ts', "const t = 'Выбрано фото(в)'"]])).toHaveLength(1)
    expect(pluralFindings([['/a.ts', "const t = n === 1 ? 'файл' : 'файлов'"]])).toHaveLength(1)
    expect(pluralFindings([['/a.ts', "const t = `${n} ${plural(n, ['файл', 'файла', 'файлов'])}`"]])).toEqual([])
    expect(pluralFindings([['/a.ts', 'const t = `${n} фото, ${m} мм`']])).toEqual([])
    expect(pluralFindings([['/a.svelte', '{#if x} файлов нет{/if}']])).toEqual([])
    expect(pluralFindings([['/a.svelte', '<p>{who} страницы</p>']])).toEqual([])
    expect(pluralFindings([['/a.ts', '// plural-ok: n всегда 64\nconst t = `${n} мест`']])).toEqual([])
    expect(pluralFindings([['/a.ts', '// `${n} файлов` в комментарии не считается']])).toEqual([])
  })

  it('в интерфейсе нет ручных «N файлов»', () => {
    expect(
      pluralFindings(sources),
      'число + слово — через plural(n, [один, два, пять]) из base/words/plural; исключение — plural-ok: причина',
    ).toEqual([])
  })
})
