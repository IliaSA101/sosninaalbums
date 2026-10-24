// Команды очистки ветки winqa/<имя> (спека 118): артефакты и прогоны этой ветки (win-qa, ci, release), затем сама ветка.
// Чистая логика без сети: gh подаёт winqa-push.mjs, здесь только сбор списков (gh подменяем в тестах) и построение команд.

// Строки `{id, run, branch}` (по одной в строке JSON) -> артефакты нужной ветки
export function parseArtifacts(text, branch) {
  return text
    .split('\n')
    .filter((l) => l.trim())
    .map((l) => JSON.parse(l))
    .filter((a) => a.branch === branch)
}

// Workflow, которые бывают на ветке winqa: win-qa.yml (сценарии QA), ci.yml (CI публичного дерева до publish-main), release.yml (пробный dry_run)
export const WORKFLOWS = ['win-qa.yml', 'ci.yml', 'release.yml']

// Прогоны ветки по всем workflow: gh(args) -> текст JSON (`gh run list --json databaseId,createdAt`). Каждому прогону — поле workflow.
export function gatherRuns(gh, branch, workflows = WORKFLOWS) {
  const runs = []
  for (const workflow of workflows) {
    const list = JSON.parse(gh(['run', 'list', '--branch', branch, '--workflow', workflow, '--limit', '100', '--json', 'databaseId,createdAt']) || '[]')
    for (const r of list) runs.push({ ...r, workflow })
  }
  return runs
}

// runs: [{databaseId, createdAt, workflow?}], artifacts: [{id, run}]. keepLast — оставить самый новый прогон каждого workflow
// и его артефакты (без workflow у прогонов — один общий список). Новизну решает createdAt.
// Порядок: артефакты, прогоны, ветка (ветку удаляем последней, чтобы повтор после сбоя нашёл прогоны по имени ветки).
export function buildCleanup(branch, runs, artifacts, keepLast) {
  const sorted = [...runs].sort((a, b) => String(b.createdAt).localeCompare(String(a.createdAt)))
  const kept = new Set()
  if (keepLast) {
    const seen = new Set()
    for (const r of sorted) {
      const w = r.workflow ?? ''
      if (seen.has(w)) continue
      seen.add(w)
      kept.add(r.databaseId)
    }
  }
  const drop = sorted.filter((r) => !kept.has(r.databaseId))
  const cmds = []
  for (const a of artifacts) {
    if (kept.has(a.run)) continue
    cmds.push(['gh', 'api', '-X', 'DELETE', `repos/{owner}/{repo}/actions/artifacts/${a.id}`])
  }
  for (const r of drop) cmds.push(['gh', 'run', 'delete', String(r.databaseId)])
  cmds.push(['git', 'push', 'origin', '--delete', branch])
  return cmds
}
