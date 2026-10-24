// Команды очистки ветки winqa/<имя> (спека 118): артефакты и прогоны win-qa этой ветки, затем сама ветка.
// Чистая логика без сети: списки прогонов и артефактов приносит winqa-push.mjs (gh), здесь только построение команд.

// Строки `{id, run, branch}` (по одной в строке JSON) -> артефакты нужной ветки
export function parseArtifacts(text, branch) {
  return text
    .split('\n')
    .filter((l) => l.trim())
    .map((l) => JSON.parse(l))
    .filter((a) => a.branch === branch)
}

// runs: [{databaseId, createdAt}], artifacts: [{id, run}]. keepLast — оставить самый новый прогон и его артефакты.
// Порядок: артефакты, прогоны, ветка (ветку удаляем последней, чтобы повтор после сбоя нашёл прогоны по имени ветки).
export function buildCleanup(branch, runs, artifacts, keepLast) {
  const sorted = [...runs].sort((a, b) => String(b.createdAt).localeCompare(String(a.createdAt)))
  const kept = keepLast && sorted.length ? sorted[0].databaseId : null
  const drop = sorted.filter((r) => r.databaseId !== kept)
  const cmds = []
  for (const a of artifacts) {
    if (kept !== null && a.run === kept) continue
    cmds.push(['gh', 'api', '-X', 'DELETE', `repos/{owner}/{repo}/actions/artifacts/${a.id}`])
  }
  for (const r of drop) cmds.push(['gh', 'run', 'delete', String(r.databaseId)])
  cmds.push(['git', 'push', 'origin', '--delete', branch])
  return cmds
}
