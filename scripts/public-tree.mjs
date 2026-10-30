// Публичное дерево: дерево коммита без путей scripts/publish-exclude.txt — то, что уйдёт в main на GitHub и в ветку winqa/*.
// Общий модуль publish-main.mjs и winqa-push.mjs (раньше фильтр был скопирован в обоих) и script-tests.mjs --public.
// CLI: `node scripts/public-tree.mjs [--ref <коммит>] --out <папка> [--repo <папка>]` — раскладывает дерево в <папка>
// (только внутри <repo>/.gotmp; папка пересоздаётся). Без --ref берётся рабочая копия (tracked + новые, без игнорируемых):
// проверки идут до коммита. Рабочую копию и индекс проекта не трогает.
import { execFileSync } from 'node:child_process'
import fs from 'node:fs'
import os from 'node:os'
import path from 'node:path'
import { fileURLToPath } from 'node:url'

/** Строки publish-exclude.txt без пустых и комментариев. */
export function parseExclude(text) {
  return text
    .split('\n')
    .map((l) => l.trim())
    .filter((l) => l && !l.startsWith('#'))
}

/** Путь исключён: совпал с файлом или лежит в исключённой папке (запись с «/» на конце). */
export function isExcluded(exclude, p) {
  return exclude.some((e) => (e.endsWith('/') ? p.startsWith(e) : p === e))
}

/**
 * Читает ref в индекс gi и убирает исключённые пути. gi(args) — git с этим индексом, возвращает stdout.
 * Возвращает {files, drop}: оставшиеся и убранные пути. Если исключённое осталось — исключение.
 */
export function filterIndex(gi, ref, exclude) {
  gi(['read-tree', ref])
  const lines = () => gi(['ls-files']).split('\n').filter(Boolean)
  const drop = lines().filter((p) => isExcluded(exclude, p))
  for (let i = 0; i < drop.length; i += 200) gi(['update-index', '--force-remove', '--', ...drop.slice(i, i + 200)])
  const files = lines()
  if (files.some((p) => isExcluded(exclude, p))) throw new Error('в дереве остались исключённые пути')
  return { files, drop }
}

/**
 * sha дерева ref без исключённых путей (временный индекс, удаляется всегда). git(args, env) — git в репозитории,
 * excludeText — содержимое publish-exclude.txt.
 */
export function publicTree(git, ref, excludeText) {
  const index = path.join(os.tmpdir(), `public-tree-index-${process.pid}-${Math.random().toString(36).slice(2)}`)
  try {
    const gi = (a) => git(a, { GIT_INDEX_FILE: index })
    const { drop } = filterIndex(gi, ref, parseExclude(excludeText))
    return { tree: gi(['write-tree']), dropped: drop.length }
  } finally {
    fs.rmSync(index, { force: true })
  }
}

/** Куда можно класть копию: строго внутри <repo>/.gotmp (пустое значение и «сам .gotmp» — отказ). */
export function checkOut(repo, out) {
  if (!out || !String(out).trim()) throw new Error('--out: пустой путь')
  const base = path.resolve(repo, '.gotmp')
  const full = path.resolve(repo, out)
  const rel = path.relative(base, full)
  if (!rel || rel.startsWith('..') || path.isAbsolute(rel)) throw new Error(`--out должен быть внутри ${base}: ${full}`)
  return full
}

/**
 * Пути с режимом 100755 из вывода `git ls-tree -r -z` / `git ls-files -s -z` (запись: «режим ... \t путь»).
 * Для ls-files -s (stage=true) учитываются только записи стадии 0.
 */
export function execPaths(out, sep = '\t', stage = false) {
  const res = []
  for (const rec of out.split('\0').filter(Boolean)) {
    const i = rec.indexOf(sep)
    if (i < 0) continue
    const meta = rec.slice(0, i).split(' ')
    if (meta[0] === '100755' && (!stage || meta[2] === '0')) res.push(rec.slice(i + 1))
  }
  return res
}

/** Разложить публичное дерево в папку out (внутри .gotmp). ref пуст — рабочая копия. Возвращает число файлов. */
export function exportPublic(repo, out, ref) {
  const dir = checkOut(repo, out)
  const git = (a, env = {}, opts = {}) =>
    execFileSync('git', a, { cwd: repo, env: { ...process.env, ...env }, maxBuffer: 1 << 28, ...opts })
  const excludeText = ref ? git(['show', `${ref}:scripts/publish-exclude.txt`], {}, { encoding: 'utf8' }) : fs.readFileSync(path.join(repo, 'scripts', 'publish-exclude.txt'), 'utf8')
  fs.rmSync(dir, { recursive: true, force: true })
  fs.mkdirSync(dir, { recursive: true })
  let execFiles = [] // пути с режимом 100755 по исходному дереву/индексу, а не по файловой системе (на Windows её биты +x не читаются)
  if (ref) {
    const { tree } = publicTree((a, env) => git(a, env, { encoding: 'utf8' }).trim(), ref, excludeText)
    // git archive кладёт режимы (+x) и симлинки как есть
    const tar = execFileSync('git', ['archive', '--format=tar', tree], { cwd: repo, maxBuffer: 1 << 30 })
    execFileSync('tar', ['-x', '-C', dir], { input: tar, maxBuffer: 1 << 28 })
    execFiles = execPaths(git(['ls-tree', '-r', '-z', tree], {}, { encoding: 'utf8' }), '\t')
  } else {
    const exclude = parseExclude(excludeText)
    // tracked — режим из индекса; новым файлам остаётся бит файловой системы (на Windows его нет — они без +x)
    execFiles = execPaths(git(['ls-files', '-s', '-z'], {}, { encoding: 'utf8' }), '\t', true)
    const list = git(['ls-files', '-z', '--cached', '--others', '--exclude-standard'], {}, { encoding: 'utf8' })
      .split('\0')
      .filter(Boolean)
    for (const rel of new Set(list)) {
      if (isExcluded(exclude, rel)) continue
      const src = path.join(repo, rel)
      let st
      try {
        st = fs.lstatSync(src)
      } catch {
        continue // удалён в рабочей копии, но ещё в индексе
      }
      const dst = path.join(dir, rel)
      fs.mkdirSync(path.dirname(dst), { recursive: true })
      if (st.isSymbolicLink()) fs.symlinkSync(fs.readlinkSync(src), dst)
      else if (st.isFile()) {
        fs.copyFileSync(src, dst)
        if (process.platform !== 'win32' && st.mode & 0o111) execFiles.push(rel)
      }
    }
    execFiles = [...new Set(execFiles)]
  }
  // Часть проверок (release-check) читает режимы из индекса git: копии нужен свой репозиторий
  execFileSync('git', ['init', '-q'], { cwd: dir })
  execFileSync('git', ['add', '-A'], { cwd: dir })
  // Режим +x ставим в индексе явно: при core.fileMode=false (новый репозиторий на Windows) add записал бы 100644
  const inIndex = new Set(execFileSync('git', ['ls-files', '-z'], { cwd: dir, encoding: 'utf8' }).split('\0').filter(Boolean))
  const chmod = execFiles.filter((p) => inIndex.has(p))
  for (let i = 0; i < chmod.length; i += 200) {
    execFileSync('git', ['update-index', '--chmod=+x', '--', ...chmod.slice(i, i + 200)], { cwd: dir })
  }
  return execFileSync('git', ['ls-files'], { cwd: dir, encoding: 'utf8' }).split('\n').filter(Boolean).length
}

if (process.argv[1] && path.resolve(process.argv[1]) === fileURLToPath(import.meta.url)) {
  const args = process.argv.slice(2)
  const val = (f) => (args.includes(f) ? (args[args.indexOf(f) + 1] ?? '') : undefined)
  const repo = val('--repo') ? path.resolve(val('--repo')) : path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..')
  try {
    const out = val('--out')
    if (out === undefined) throw new Error('usage: node scripts/public-tree.mjs [--ref <коммит>] --out <папка в .gotmp> [--repo <папка>]')
    const n = exportPublic(repo, out, val('--ref'))
    console.log(`публичное дерево: ${n} файлов -> ${checkOut(repo, out)}`)
  } catch (e) {
    console.error(`public-tree: ${e.message}`)
    process.exit(2)
  }
}
