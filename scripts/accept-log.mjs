// Журнал приёмов и проверок Windows (разбор процесса 0.35, С1; исполнение решения В5 «winqa каждые 5 приёмов с Go»).
// Файл — <git-common-dir>/sa-accept.jsonl: вне дерева, не коммитится, по JSON-объекту в строке.
//   приём:  {t, branch, from, to, packages, go}   (пишет accept.mjs после успешного приёма)
//   winqa:  {t, winqa, dev, bypass?}              (пишет winqa-push.mjs при --push; bypass — причина обхода)
// Читается с пределом размера; битая, пустая или чужая строка пропускается, а не роняет скрипт.
import { execFileSync } from 'node:child_process'
import fs from 'node:fs'
import path from 'node:path'

export const JOURNAL = 'sa-accept.jsonl'
/** Больше приёмов с Go после последнего winqa — rc не отправляем. */
export const MAX_GO_ACCEPTS = 5
/** Читаем не больше хвоста этого размера: журнал растёт, но решает только свежая часть. */
export const MAX_BYTES = 4 << 20

/** Общий .git репозитория (из любого worktree): там журнал виден всем копиям. */
export function commonDir(cwd) {
  return execFileSync('git', ['rev-parse', '--path-format=absolute', '--git-common-dir'], { cwd, encoding: 'utf8', stdio: ['ignore', 'pipe', 'pipe'] }).trim()
}

export const journalFile = (cwd) => path.join(commonDir(cwd), JOURNAL)

/** Записи журнала по порядку; нет файла — пусто; крупнее предела — только хвост без обрезанной первой строки. */
export function readJournal(file) {
  const st = fs.statSync(file, { throwIfNoEntry: false })
  if (!st || !st.isFile() || st.size === 0) return []
  const size = Math.min(st.size, MAX_BYTES)
  const buf = Buffer.alloc(size)
  const fd = fs.openSync(file, 'r')
  try {
    fs.readSync(fd, buf, 0, size, st.size - size)
  } finally {
    fs.closeSync(fd)
  }
  let text = buf.toString('utf8')
  if (st.size > size) text = text.slice(text.indexOf('\n') + 1)
  const out = []
  for (const l of text.split('\n')) {
    if (!l.trim()) continue
    try {
      const e = JSON.parse(l)
      if (e && typeof e === 'object' && !Array.isArray(e)) out.push(e)
    } catch {
      // оборванная запись (процесс убили на записи) — не повод падать
    }
  }
  return out
}

export function appendEntry(file, entry) {
  fs.appendFileSync(file, `${JSON.stringify({ t: new Date().toISOString(), ...entry })}\n`)
}

/** Пакеты Go по списку изменённых путей: каталоги .go-файлов; go.mod/go.sum — «go.mod». */
export function goPackages(files) {
  const pk = new Set()
  for (const f of files) {
    if (/\.go$/.test(f)) pk.add(path.posix.dirname(f))
    else if (/(^|\/)go\.(mod|sum)$/.test(f)) pk.add('go.mod')
  }
  return [...pk].sort()
}

const isRc = (name) => /^rc-/.test(name)

/** Последний не-rc winqa и число приёмов с Go после него (rc-круги счёт не сбрасывают: они проверяют сборку, а не накопленное). */
export function sinceWinqa(entries) {
  let last = -1
  entries.forEach((e, i) => {
    if (typeof e.winqa === 'string' && !isRc(e.winqa)) last = i
  })
  const count = entries.slice(last + 1).filter((e) => e.go === true && typeof e.branch === 'string').length
  return { count, winqa: last >= 0 ? entries[last].winqa : null }
}

/** Итог последнего прогона win-qa: 'green' | 'red' | 'unknown' (нет прогонов, идёт, непонятный ответ). */
export function runState(text) {
  let list
  try {
    list = JSON.parse(text || '[]')
  } catch {
    return 'unknown'
  }
  const r = Array.isArray(list) ? list[0] : null
  if (!r || r.status !== 'completed') return 'unknown'
  return r.conclusion === 'success' || r.conclusion === 'skipped' ? 'green' : 'red'
}

/**
 * Можно ли отправлять rc-ветку. gh(args) -> текст stdout, бросает при ошибке. Возвращает {problems, warnings}:
 * problems — отказ с подсказкой «как исправить»; warnings — проверить не удалось (нет сети/gh, нет прогонов), не отказ.
 */
export function checkRc(entries, gh) {
  const problems = []
  const warnings = []
  const { count, winqa } = sinceWinqa(entries)
  if (count > MAX_GO_ACCEPTS) {
    problems.push(
      `после последнего winqa принято ${count} приёмов с Go (предел ${MAX_GO_ACCEPTS}); сначала node scripts/winqa-push.mjs <имя> --push и дождаться зелёного win-qa`,
    )
  }
  if (!winqa) {
    warnings.push('в журнале нет ни одного не-rc winqa — красный ли последний, проверить нечем')
    return { problems, warnings }
  }
  try {
    const st = runState(gh(['run', 'list', '--branch', `winqa/${winqa}`, '--workflow', 'win-qa.yml', '--limit', '1', '--json', 'status,conclusion']))
    if (st === 'red') problems.push(`последний winqa/${winqa} красный; починить и повторить winqa (node scripts/winqa-push.mjs <имя> --push), только потом rc`)
    else if (st === 'unknown') warnings.push(`winqa/${winqa}: прогона win-qa нет или он не завершён — зелёность не подтверждена`)
  } catch (e) {
    warnings.push(`gh не ответил (${String(e.message).split('\n')[0]}) — состояние winqa/${winqa} не проверено`)
  }
  return { problems, warnings }
}
