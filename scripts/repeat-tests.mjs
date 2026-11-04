// Повтор добавленных Go-тестов с горутинами и временем (разбор процесса 0.35, С3): `go test -short` один раз не ловит
// нестабильность — TestDecodeFailCachedOnce (0.35/155) прошёл precheck и упал в CI. Здесь только чистые части:
// отбор тестов, расчёт числа повторов, разбор времени; запуск go test — в precheck.mjs.
// Не ловит data race (без -race): её ловит go-race в CI.

// Признаки: горутина, пакет sync, время, параллельный тест.
export const RISK = [/\bgo\s+func\b/, /\bsync\./, /\btime\./, /\bt\.Parallel\(/]
export const PER_TEST_SEC = 3 // бюджет на один тест
export const CAP_SEC = 60 // общий потолок повторов
export const MAX_COUNT = 500 // тест в доли миллисекунды не гоняем бесконечно
const MIN_DUR = 0.005 // go test округляет до 0.01s: ноль считаем за 5 мс

const TEST_RE = /^func (Test\w*)\(/

const noComments = (s) => s.replace(/\/\/.*$/gm, '')
export const hasRisk = (src) => RISK.some((re) => re.test(noComments(src)))

/** Тела тестов файла: {имя → текст}. Тело — до строки «}» в первой колонке (gofmt). */
export function testBodies(src) {
  const res = {}
  let name = ''
  let buf = []
  for (const line of src.split('\n')) {
    if (!name) {
      const m = TEST_RE.exec(line)
      if (!m) continue
      name = m[1]
      buf = [line]
    } else {
      buf.push(line)
    }
    if (name && line.startsWith('}')) {
      res[name] = buf.join('\n')
      name = ''
    }
  }
  return res
}

/**
 * Добавленные тесты с признаками риска: {dir → [имена]}.
 * files — изменённые *_test.go (пути от корня через «/»); read(путь) — текст или null; baseTests(dir) — имена тестов
 * папки в базе (перенос теста между файлами папки не считается добавлением). Признак — в теле теста или в файле,
 * который тестируют (x_test.go → x.go).
 */
export function selectRiskyTests(files, read, baseTests) {
  const byDir = {}
  for (const f of files) {
    if (!f.endsWith('_test.go')) continue
    const src = read(f)
    if (src == null) continue
    const dir = f.includes('/') ? f.slice(0, f.lastIndexOf('/')) : '.'
    const entry = (byDir[dir] ??= { base: baseTests(dir), names: new Set() })
    const tested = read(f.replace(/_test\.go$/, '.go'))
    const testedRisky = tested != null && hasRisk(tested)
    for (const [name, body] of Object.entries(testBodies(src))) {
      if (entry.base.has(name)) continue
      if (testedRisky || hasRisk(body)) entry.names.add(name)
    }
  }
  const res = {}
  for (const [dir, v] of Object.entries(byDir)) if (v.names.size) res[dir] = [...v.names].sort()
  return res
}

/** Шаблон -run для набора имён. */
export const runPattern = (names) => `^(${names.join('|')})$`

/** Время верхнеуровневых тестов из `go test -v`: {имя → секунды}. */
export function parseDurations(out) {
  const res = {}
  for (const m of out.matchAll(/^--- (?:PASS|FAIL): (Test\w*) \(([\d.]+)s\)/gm)) res[m[1]] = Number(m[2])
  return res
}

/**
 * Число повторов по папкам: {dir → count}. groups — {dir → [имена]}, seconds — {dir → сумма времени одного прогона}.
 * Бюджет — 3 с на тест, не больше CAP_SEC на все папки; делится по числу тестов. Минимум 2 повтора.
 */
export function planRepeats(groups, seconds, perTest = PER_TEST_SEC, cap = CAP_SEC) {
  const k = Object.values(groups).reduce((n, g) => n + g.length, 0)
  const budget = Math.min(cap, perTest * k)
  const res = {}
  for (const [dir, names] of Object.entries(groups)) {
    const one = Math.max(seconds[dir] || 0, MIN_DUR * names.length)
    res[dir] = Math.max(2, Math.min(MAX_COUNT, Math.floor((budget * (names.length / k)) / one)))
  }
  return res
}

/** Строка итога: «повтор новых тестов: K тестов, N раз, Xs». */
export function repeatLine(k, counts, sec) {
  const v = Object.values(counts)
  const lo = Math.min(...v)
  const hi = Math.max(...v)
  return `повтор новых тестов: ${k} тестов, ${lo === hi ? lo : `${lo}–${hi}`} раз, ${sec.toFixed(1)}s`
}
