// Независимая пересборка установщика из метки (спека 111, design §5.4): release-sign сравнивает её побайтно с
// установщиком черновика, чтобы при взломе аккаунта или CI не подписать чужой файл. Только node:* и git/npm/bash.
//
// Чистая копия метки — `git worktree` под .gotmp (общий кэш Go и папка инструментов находятся через общий .git, как у
// агентов), затем те же шаги, что в CI: `npm ci` и `scripts/build.sh --installer` (exe для Windows с -trimpath и
// установщик). CGO_ENABLED=0 задан явно: CI собирает так же, иначе в exe разойдутся build ID и пометка CGO.
import fs from 'node:fs'
import path from 'node:path'
import { execFileSync } from 'node:child_process'

/** Окружение сборки: переменные Go, меняющие байты результата, сбрасываются (CI их не задаёт); CGO выключен. */
export function buildEnv(base) {
  const env = { ...base, CGO_ENABLED: '0' }
  for (const k of ['GOFLAGS', 'GOEXPERIMENT', 'GOAMD64', 'GOOS', 'GOARCH', 'GOWORK', 'CC', 'CXX']) delete env[k]
  return env
}

/** Запуск внешней команды: вывод идёт в stderr (ход сборки), сбой — исключение с именем команды. */
export function defaultRun(cmd, args, { cwd, env }) {
  try {
    return execFileSync(cmd, args, { cwd, env, stdio: ['ignore', 2, 2], maxBuffer: 1 << 28 })
  } catch (e) {
    throw new Error(`${cmd} ${args.slice(0, 2).join(' ')}: не удалось${e.code === 'ENOENT' ? ` (${cmd} не найден)` : ''}`)
  }
}

/**
 * Собирает установщик метки tag в <work>/src и возвращает путь к его копии в <work>. repo — корень репозитория,
 * где лежит метка. run подменяется в тестах; fail(текст) делает исключение отказа. Копия метки удаляется всегда.
 */
export function rebuildSetup({ tag, version, work, repo, env = process.env, run = defaultRun, fail = (m) => new Error(m) }) {
  const src = path.join(work, 'src')
  const e = buildEnv(env)
  // след прошлого сбойного прогона: папку work стёрли, а запись worktree осталась
  run('git', ['worktree', 'prune'], { cwd: repo, env: e })
  try {
    run('git', ['worktree', 'add', '--detach', src, `refs/tags/${tag}`], { cwd: repo, env: e })
  } catch {
    throw fail(`не удалось взять метку ${tag} из этого репозитория: пересобирать нечего (метка создаётся здесь при выпуске, проверьте git tag -l)`)
  }
  try {
    run('npm', ['ci'], { cwd: path.join(src, 'frontend'), env: e })
    run('bash', ['scripts/build.sh', '--installer'], { cwd: src, env: e })
    const out = path.join(src, 'build', 'bin', `SosninaAlbums-${version}-windows-x64-setup.exe`)
    if (!fs.existsSync(out)) throw fail(`пересборка не создала ${path.basename(out)}`)
    // копия метки сейчас удалится, поэтому файл переносим
    const kept = path.join(work, path.basename(out))
    fs.copyFileSync(out, kept)
    return kept
  } finally {
    try {
      run('git', ['worktree', 'remove', '--force', src], { cwd: repo, env: e })
    } catch {
      fs.rmSync(src, { recursive: true, force: true })
      run('git', ['worktree', 'prune'], { cwd: repo, env: e })
    }
  }
}
