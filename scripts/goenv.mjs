// Кэши Go и golangci-lint — в .gotmp основной копии (общие для всех worktree).
// Почему: пользовательская переменная GOCACHE перебивала env из .claude/settings.json,
// и кэш проекта за 19–20.10 ушёл в D:\gocache (+36 ГБ) мимо .gotmp (20.10).
import { spawnSync } from 'node:child_process'
import path from 'node:path'

// Корень основной копии: рядом с общим .git; не git — сама папка here.
export function mainRoot(here) {
  const common = spawnSync('git', ['rev-parse', '--path-format=absolute', '--git-common-dir'], { cwd: here, encoding: 'utf8' })
  return common.status === 0 ? path.dirname(common.stdout.trim()) : here
}

export function goCaches(here) {
  const main = mainRoot(here)
  return {
    GOCACHE: path.join(main, '.gotmp', 'cache-go'),
    GOLANGCI_LINT_CACHE: path.join(main, '.gotmp', 'cache-golangci'),
  }
}

// Поверх любых пользовательских значений.
export const withGoCaches = (base, here) => ({ ...base, ...goCaches(here) })
