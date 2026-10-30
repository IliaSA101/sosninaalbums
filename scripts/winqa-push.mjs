// Ветка для проверки Windows-exe в GitHub Actions (спека 118): `node scripts/winqa-push.mjs <имя> [ref] [--push] [--repo <папка>]`.
// Строит из дерева ref (по умолчанию HEAD) без путей scripts/publish-exclude.txt коммит-сироту (без родителей, автор из git config)
// и печатает команду отправки в origin refs/heads/winqa/<имя>; сам отправляет только с --push. Ветка winqa/** запускает win-qa.yml.
// `--delete <имя> [--push]` — удаление ветки (печатает команду, с --push выполняет). Рабочую копию и индекс проекта не трогает.
// `--cleanup <имя> [--keep-last] [--yes]` — удалить ветку, артефакты и прогоны этой ветки: win-qa, ci и release (без --yes только печать команд;
// --keep-last оставляет последний прогон каждого workflow с артефактами, чтобы успеть посмотреть итог). Построение команд — winqa-cleanup.mjs.
import { execFileSync } from 'node:child_process'
import path from 'node:path'
import { publicTree } from './public-tree.mjs'
import { buildCleanup, gatherRuns, parseArtifacts } from './winqa-cleanup.mjs'

const args = process.argv.slice(2)
const flag = (f) => args.includes(f)
const repoAt = args.indexOf('--repo')
const repo = repoAt >= 0 ? path.resolve(args[repoAt + 1]) : path.resolve(path.dirname(new URL(import.meta.url).pathname.replace(/^\/(\w:)/, '$1')), '..')
const delAt = args.indexOf('--delete')
const cleanAt = args.indexOf('--cleanup')
const rest = args.filter((a, i) => !a.startsWith('--') && args[i - 1] !== '--repo')
const name = rest[0]
const ref = rest[1] || 'HEAD'

if (!name || !/^[a-z0-9][a-z0-9._-]*$/.test(name)) {
  console.error('usage: node scripts/winqa-push.mjs <имя> [ref] [--push]   |   --delete <имя> [--push]   |   --cleanup <имя> [--keep-last] [--yes]')
  console.error('имя ветки: латиница нижнего регистра, цифры, . _ -')
  process.exit(2)
}
const branch = `winqa/${name}`
const git = (a, env = {}) => execFileSync('git', a, { cwd: repo, encoding: 'utf8', env: { ...process.env, ...env } }).trim()

if (cleanAt >= 0) {
  const gh = (a) => execFileSync('gh', a, { cwd: repo, encoding: 'utf8' }).trim()
  let runs = []
  let artifacts = []
  try {
    runs = gatherRuns(gh, branch)
    artifacts = parseArtifacts(gh(['api', '--paginate', 'repos/{owner}/{repo}/actions/artifacts?per_page=100', '--jq', '.artifacts[] | {id, run: .workflow_run.id, branch: .workflow_run.head_branch}']), branch)
  } catch (e) {
    console.error('gh не ответил (нужны gh и вход в GitHub): ' + e.message)
    process.exit(1)
  }
  const cmds = buildCleanup(branch, runs, artifacts, flag('--keep-last'))
  for (const c of cmds) {
    console.log(c.join(' '))
    if (flag('--yes')) execFileSync(c[0], c.slice(1), { cwd: repo, stdio: 'inherit' })
  }
  if (!flag('--yes')) console.log('(печать команд; выполнить - добавить --yes)')
  process.exit(0)
}

if (delAt >= 0) {
  const cmd = ['push', 'origin', '--delete', branch]
  console.log('git ' + cmd.join(' '))
  if (flag('--push')) git(cmd)
  process.exit(0)
}

try {
  const { tree, dropped } = publicTree(git, ref, git(['show', `${ref}:scripts/publish-exclude.txt`]))
  const commit = git(['commit-tree', tree, '-m', `winqa ${name}: ${git(['rev-parse', '--short', ref])}`])
  console.log(`коммит ${commit.slice(0, 7)}, убрано путей: ${dropped}`)
  // Ветка одноразовая и пересоздаётся сиротой каждый раз, поэтому отправка принудительная
  const cmd = ['push', '--force', 'origin', `${commit}:refs/heads/${branch}`]
  console.log('git ' + cmd.join(' '))
  if (flag('--push')) git(cmd)
} catch (e) {
  console.error(`winqa-push: ${e.message}`)
  process.exit(1)
}
