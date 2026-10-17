// Покрытие дерева internal/album: все тесты дерева против всего его кода (023, решение владельца 2026-10-10).
// Общий код для precheck.mjs (при правке internal/album/**) и test-baseline.mjs (эталон), чтобы не было двух копий.
// Профиль сливаем по блокам «файл:диапазон» (блок покрыт, если покрыт в любом прогоне), операторы — по уникальным блокам.
import { spawnSync } from 'node:child_process'
import fs from 'node:fs'
import path from 'node:path'

export const TREE_ROOTS = ['internal/album']
export const TREE_KEY = 'tree:album'
export const TREE_MIN = 96.9 // % — уровень до 023 (1b61383: 3127 из 3226)
export const TREE_MAX_MISSED = 90 // непокрытых блоков, критерий спеки 023
export const MAX_DROP = 0.5 // п. п. покрытия

export const isTree = (p) => TREE_ROOTS.some((r) => p === r || p.startsWith(r + '/'))

// Разбор профиля coverprofile: общие числа и список непокрытых блоков («файл:строки») — по ним precheck
// показывает, где именно не хватает теста. Вынесен отдельно ради теста на готовом тексте.
export function parseProfile(text) {
  const blocks = new Map() // файл:диапазон -> {n, hit}
  for (const line of text.split('\n')) {
    const m = /^(.+:\d+\.\d+,\d+\.\d+) (\d+) (\d+)$/.exec(line.trim())
    if (!m) continue
    const b = blocks.get(m[1]) ?? { n: +m[2], hit: false }
    b.hit ||= +m[3] > 0
    blocks.set(m[1], b)
  }
  let stmts = 0
  let covered = 0
  const missedBlocks = []
  for (const [key, b] of blocks) {
    stmts += b.n
    if (b.hit) covered += b.n
    else missedBlocks.push(key)
  }
  return {
    stmts,
    covered,
    missed: missedBlocks.length,
    missedBlocks,
    cover: stmts ? Math.round((covered / stmts) * 1000) / 10 : 0,
  }
}

// Прогон: opts.root — корень репозитория, opts.env — окружение (GOTMPDIR), opts.heavy — через очередь heavy.mjs
// (precheck: машина общая). Ошибка прогона — исключение с хвостом вывода.
export function treeCover({ root, env, heavy = false }) {
  // На Windows go list падает целиком, если папки шаблона нет: берём только существующие корни
  const roots = TREE_ROOTS.filter((r) => fs.existsSync(path.join(root, r)))
  const list = spawnSync('go', ['list', ...roots.map((r) => `./${r}/...`)], { cwd: root, env, encoding: 'utf8' })
  if (list.error || list.status !== 0) throw new Error(`go list of album tree failed:\n${list.error ?? list.stderr}`)
  const pkgs = list.stdout
    .split('\n')
    .map((s) => s.trim())
    .filter(Boolean)
  if (!pkgs.length) throw new Error(`go list of album tree failed:\n${list.stderr}`)
  // edtest и inside — помощники тестов (до 023 жили в _test.go и в покрытие не входили): в знаменатель не берём,
  // иначе ветки отказа помощников (t.Fatal) выглядят непокрытым кодом альбома
  const measured = pkgs.filter((p) => !/\/internal\/(edtest|inside)$/.test(p))
  const gotmp = env.GOTMPDIR || path.join(root, '.gotmp')
  fs.mkdirSync(gotmp, { recursive: true })
  const prof = path.join(gotmp, `${process.pid}-tree-cover.out`)
  const goArgs = ['test', '-count=1', `-coverpkg=${measured.join(',')}`, `-coverprofile=${prof}`, ...pkgs]
  const [cmd, args] = heavy ? ['node', ['scripts/heavy.mjs', 'go', ...goArgs]] : ['go', goArgs]
  const r = spawnSync(cmd, args, { cwd: root, env, encoding: 'utf8', maxBuffer: 64 << 20 })
  if (r.status !== 0) {
    const tail = ((r.stdout || '') + (r.stderr || '')).split('\n').slice(-30).join('\n')
    throw new Error(`go test -coverpkg for album tree failed:\n${tail}`)
  }
  const text = fs.readFileSync(prof, 'utf8')
  fs.rmSync(prof, { force: true })
  return parseProfile(text)
}

// Сравнение с записью эталона tree:album (base) — список ошибок. Те же правила, что в test-baseline --check:
// падение покрытия больше MAX_DROP, уровень ниже TREE_MIN (критерий 023) и больше непокрытых блоков, чем в эталоне
// (до первого --write — TREE_MAX_MISSED).
export function compareTree(base, cur) {
  const errors = []
  if (base.cover - cur.cover > MAX_DROP + 1e-9) {
    errors.push(`${TREE_KEY}: coverage fell ${base.cover}% -> ${cur.cover}% (max drop ${MAX_DROP})`)
  }
  if (cur.cover < TREE_MIN - 1e-9) errors.push(`${TREE_KEY}: coverage ${cur.cover}% below ${TREE_MIN}% (023 criterion)`)
  const maxMissed = base.missed ?? TREE_MAX_MISSED
  if (cur.missed > maxMissed) errors.push(`${TREE_KEY}: uncovered blocks ${cur.missed} > ${maxMissed}`)
  return errors
}
