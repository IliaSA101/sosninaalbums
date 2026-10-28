// Приём ветки агента одной командой: `node scripts/accept.mjs <worktree> <ветка>` (П4, владелец 24.10).
// worktree — путь или имя под .claude/worktrees/. Порядок: копия чистая → rebase на dev (конфликт только в
// docs/specs/*/decisions.md сводится «сначала dev, затем ветка») → precheck по диффу ветки → ff-merge в основной dev →
// снять ссылку frontend/node_modules → удалить worktree и ветку. Перед precheck ссылку node_modules ставим сами (агенты
// снимают её перед отчётом, без неё precheck фронта падает). Итог — одна строка («принято …» или «СТОП: …»).
// Ничего не пушит. Без оболочки: только execFileSync с массивом аргументов (Windows-пути тоже).
// ACCEPT_PRECHECK — путь к .mjs вместо scripts/precheck.mjs копии (подмена в тестах).
import { execFileSync } from 'node:child_process'
import fs from 'node:fs'
import path from 'node:path'
import { fileURLToPath } from 'node:url'

export class Stop extends Error {}

const DECISIONS = /^docs\/specs\/[^/]+\/decisions\.md$/
const MAX_STEPS = 100

/** Запуск без оболочки: {code, out} (out = stdout+stderr). */
function sh(cmd, args, cwd, env = process.env) {
  try {
    const out = execFileSync(cmd, args, { cwd, env, encoding: 'utf8', maxBuffer: 1 << 28, stdio: ['ignore', 'pipe', 'pipe'] })
    return { code: 0, out }
  } catch (e) {
    return { code: typeof e.status === 'number' ? e.status : 1, out: `${e.stdout || ''}${e.stderr || ''}${e.code === 'ENOENT' ? `${cmd}: не найден` : ''}` }
  }
}

const gitIn = (cwd, args, env) => sh('git', ['-c', 'core.quotepath=off', ...args], cwd, env)
const lines = (s) => s.split('\n').map((l) => l.trim()).filter(Boolean)

/**
 * Сводит конфликтные блоки «обе стороны»: dev-часть, пустая строка, часть ветки (diff3-основание отбрасывается).
 * Возвращает null, если маркеров нет или блок не закрыт.
 */
export function mergeBothSides(text) {
  const eol = text.includes('\r\n') ? '\r\n' : '\n'
  const src = text.split(/\r?\n/)
  const out = []
  let n = 0
  let state = 0 // 0 — вне блока, 1 — dev, 2 — основание, 3 — ветка
  let dev = []
  let br = []
  for (const l of src) {
    if (state === 0) {
      if (/^<{7}( |$)/.test(l)) {
        state = 1
        dev = []
        br = []
      } else out.push(l)
    } else if (state === 1 && /^\|{7}( |$)/.test(l)) state = 2
    else if (state <= 2 && /^={7}$/.test(l)) state = 3
    else if (state === 3 && /^>{7}( |$)/.test(l)) {
      out.push(...dev)
      if (dev.length && br.length) out.push('')
      out.push(...br)
      state = 0
      n++
    } else if (state === 1) dev.push(l)
    else if (state === 3) br.push(l)
  }
  if (state !== 0 || n === 0) return null
  return out.join(eol)
}

function ensureClean(cwd, what, untracked) {
  const r = gitIn(cwd, ['status', '--porcelain', untracked ? '--untracked-files=normal' : '--untracked-files=no'])
  if (r.code !== 0) throw new Stop(`${what}: git status: ${r.out.trim()}`)
  const l = lines(r.out)
  if (l.length) throw new Stop(`${what}: есть незакоммиченное (${l.length}): ${l.slice(0, 5).join('; ')}`)
}

/** Корень основной копии — родитель общего .git (из любого worktree тоже). */
export function mainRoot(cwd) {
  const r = gitIn(cwd, ['rev-parse', '--path-format=absolute', '--git-common-dir'])
  if (r.code !== 0) throw new Stop(`не git-репозиторий: ${cwd}`)
  return path.dirname(r.out.trim())
}

function resolveWorktree(root, arg) {
  const cands = [path.resolve(arg), path.join(root, '.claude', 'worktrees', arg)]
  const found = cands.find((c) => fs.existsSync(path.join(c, '.git')))
  if (!found) throw new Stop(`worktree не найден: ${arg}`)
  return fs.realpathSync(found)
}

/** Сводка rebase: конфликты только в decisions.md сводятся, прочие — стоп (rebase отменяется, копия цела). */
function rebaseOnDev(wt, env) {
  const e = { ...env, GIT_EDITOR: 'true', GIT_SEQUENCE_EDITOR: 'true' }
  let r = gitIn(wt, ['rebase', '-q', 'dev'], e)
  for (let i = 0; r.code !== 0; i++) {
    const un = lines(gitIn(wt, ['diff', '--name-only', '--diff-filter=U'], e).out)
    if (!un.length || i >= MAX_STEPS) {
      gitIn(wt, ['rebase', '--abort'], e)
      throw new Stop(`rebase не удался: ${r.out.trim().split('\n').slice(-3).join(' | ')}`)
    }
    const bad = un.filter((f) => !DECISIONS.test(f))
    if (bad.length) {
      gitIn(wt, ['rebase', '--abort'], e)
      throw new Stop(`rebase: конфликт в ${bad.join(', ')} — свести вручную (копия возвращена к исходной)`)
    }
    for (const f of un) {
      const file = path.join(wt, f)
      const merged = fs.existsSync(file) ? mergeBothSides(fs.readFileSync(file, 'utf8')) : null
      if (merged === null) {
        gitIn(wt, ['rebase', '--abort'], e)
        throw new Stop(`rebase: ${f} — конфликт без текстовых маркеров, свести вручную (копия возвращена)`)
      }
      fs.writeFileSync(file, merged)
      gitIn(wt, ['add', '--', f], e)
    }
    r = gitIn(wt, ['rebase', '--continue'], e)
    // коммит после сведения стал пустым — пропускаем его
    if (r.code !== 0 && !lines(gitIn(wt, ['diff', '--name-only', '--diff-filter=U'], e).out).length && !gitIn(wt, ['diff', '--cached', '--quiet'], e).code) {
      r = gitIn(wt, ['rebase', '--skip'], e)
    }
  }
}

/**
 * Ссылка <копия>/frontend/node_modules на node_modules основной копии, если в копии её нет (агент снял перед отчётом).
 * Тип 'junction' — на Windows не требует прав на симлинки (на Linux/macOS игнорируется, как в CLAUDE.md, «Worktree»).
 * Возвращает true, если ссылку поставили. Нет frontend/ в копии или node_modules в основной — пропуск (precheck скажет сам).
 */
export function ensureNodeModules(root, wt) {
  const link = path.join(wt, 'frontend', 'node_modules')
  const target = path.join(root, 'frontend', 'node_modules')
  if (fs.lstatSync(link, { throwIfNoEntry: false })) return false
  if (!fs.statSync(path.join(wt, 'frontend'), { throwIfNoEntry: false })?.isDirectory()) return false
  if (!fs.statSync(target, { throwIfNoEntry: false })?.isDirectory()) return false
  fs.symlinkSync(target, link, 'junction')
  return true
}

const plural = (n) => {
  const m = n % 100
  const d = n % 10
  return m >= 11 && m <= 14 ? 'коммитов' : d === 1 ? 'коммит' : d >= 2 && d <= 4 ? 'коммита' : 'коммитов'
}

/** Приём ветки; возвращает итоговую строку, при остановке бросает Stop. */
export function accept(wtArg, branch, { cwd = process.cwd(), env = process.env, log = () => {} } = {}) {
  if (!wtArg || !branch) throw new Stop('использование: node scripts/accept.mjs <worktree> <ветка>')
  if (branch === 'dev') throw new Stop('ветка dev не принимается')
  const root = mainRoot(cwd)
  const wt = resolveWorktree(root, wtArg)
  if (path.resolve(wt) === path.resolve(root)) throw new Stop('worktree — это основная копия')

  const cur = gitIn(root, ['rev-parse', '--abbrev-ref', 'HEAD']).out.trim()
  if (cur !== 'dev') throw new Stop(`основная копия на «${cur}», а не на dev`)
  ensureClean(root, 'основная копия', false)
  const wb = gitIn(wt, ['rev-parse', '--abbrev-ref', 'HEAD']).out.trim()
  if (wb !== branch) throw new Stop(`в копии ветка «${wb}», а не «${branch}»`)
  ensureClean(wt, 'копия', true)

  rebaseOnDev(wt, env)

  const devHash = gitIn(root, ['rev-parse', 'dev']).out.trim()
  const pre = env.ACCEPT_PRECHECK || path.join(wt, 'scripts', 'precheck.mjs')
  if (ensureNodeModules(root, wt)) log('frontend/node_modules: поставлена ссылка на основную копию')
  log('precheck…')
  const p = sh(process.execPath, [pre, '--base', devHash], wt, env)
  if (p.code !== 0) {
    const tail = p.out.trim().split('\n').slice(-40).join('\n')
    throw new Stop(`precheck провален (код ${p.code}), ветка не влита; копия ${wt}\n${tail}`)
  }
  // precheck мог оставить мусор, но не правки: копия снова должна быть чистой
  ensureClean(wt, 'копия после precheck', true)

  const n = Number(gitIn(root, ['rev-list', '--count', `dev..${branch}`]).out.trim()) || 0
  const m = gitIn(root, ['merge', '--ff-only', '-q', branch])
  if (m.code !== 0) throw new Stop(`ff-merge не прошёл (dev ушёл вперёд?): ${m.out.trim()}; повторить приём`)

  // frontend/node_modules в копии — ссылка на основное дерево: убрать саму ссылку, не цель
  const link = path.join(wt, 'frontend', 'node_modules')
  if (fs.lstatSync(link, { throwIfNoEntry: false })?.isSymbolicLink()) {
    if (process.platform === 'win32') fs.rmdirSync(link)
    else fs.unlinkSync(link)
  }
  const rm = gitIn(root, ['worktree', 'remove', wt])
  if (rm.code !== 0) throw new Stop(`влито, но worktree не удалён: ${rm.out.trim()}`)
  const bd = gitIn(root, ['branch', '-d', branch])
  if (bd.code !== 0) throw new Stop(`влито, но ветка не удалена: ${bd.out.trim()}`)
  const last = gitIn(root, ['log', '--oneline', '-1']).out.trim()
  return `принято ${branch}: ${n} ${plural(n)}, ${last}`
}

function main() {
  try {
    const [wt, branch] = process.argv.slice(2)
    console.log(accept(wt, branch, { log: (s) => console.error(s) }))
  } catch (e) {
    if (!(e instanceof Stop)) throw e
    console.log(`СТОП: ${e.message}`)
    process.exit(1)
  }
}

if (process.argv[1] && path.resolve(process.argv[1]) === fileURLToPath(import.meta.url)) main()
