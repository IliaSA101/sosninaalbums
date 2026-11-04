// Тесты release-rebuild.mjs (пересборка установщика из метки перед подписью, спека 111 design §5.4).
// Запуск: node --test scripts/release-rebuild.test.mjs. Настоящих npm и build.sh не зовёт: они подменены записью файла;
// git — настоящий, на временном репозитории (проверяет, что копия метки создаётся и всегда удаляется).
import assert from 'node:assert/strict'
import fs from 'node:fs'
import os from 'node:os'
import path from 'node:path'
import { execFileSync } from 'node:child_process'
import { test } from 'node:test'
import { BMP_FILES, buildEnv, rebuildSetup } from './release-rebuild.mjs'

const TAG = 'v0.25.1'
const NAME = 'SosninaAlbums-0.25.1-windows-x64-setup.exe'
const git = (cwd, ...args) => execFileSync('git', args, { cwd, encoding: 'utf8', stdio: ['ignore', 'pipe', 'pipe'] })

/** Временный репозиторий с меткой: frontend/ и scripts/ есть, чтобы cwd подмен существовал. */
function repo() {
  const dir = fs.mkdtempSync(path.join(os.tmpdir(), 'relreb-'))
  const r = path.join(dir, 'repo')
  fs.mkdirSync(path.join(r, 'frontend'), { recursive: true })
  fs.writeFileSync(path.join(r, 'frontend', 'package.json'), '{}\n')
  fs.writeFileSync(path.join(r, 'marker.txt'), 'метка\n')
  git(dir, 'init', '-q', r)
  git(r, 'config', 'core.autocrlf', 'false') // тест проверяет логику, а не настройки машины (CRLF на Windows)
  git(r, 'add', '.')
  git(r, '-c', 'user.name=t', '-c', 'user.email=t@t', '-c', 'commit.gpgsign=false', 'commit', '-q', '-m', 'x')
  git(r, 'tag', TAG)
  return { r, work: path.join(dir, 'work') }
}

/** run: git настоящий, npm и bash — подмена (журнал; bash кладёт установщик, если не задано иное). */
function runner(log, { setup = Buffer.from('setup'), failOn = '' } = {}) {
  return (cmd, args, o) => {
    log.push({ cmd, args, cwd: o.cwd, env: o.env })
    if (failOn === cmd) throw new Error(`${cmd}: не удалось`)
    if (cmd === 'git') return execFileSync('git', args, { cwd: o.cwd, stdio: ['ignore', 'pipe', 'pipe'] })
    if (cmd === 'bash' && setup) {
      const out = path.join(o.cwd, 'build', 'bin')
      fs.mkdirSync(out, { recursive: true })
      fs.writeFileSync(path.join(out, NAME), setup)
    }
    return Buffer.alloc(0)
  }
}

test('buildEnv: CGO_ENABLED=0, переменные Go с влиянием на байты сброшены, прочие целы', () => {
  const e = buildEnv({ CGO_ENABLED: '1', GOFLAGS: '-tags=x', GOOS: 'linux', GOARCH: 'arm64', GOAMD64: 'v3', GOEXPERIMENT: 'e', CC: 'gcc', PATH: '/bin', HOME: '/h' })
  assert.equal(e.CGO_ENABLED, '0')
  for (const k of ['GOFLAGS', 'GOOS', 'GOARCH', 'GOAMD64', 'GOEXPERIMENT', 'CC']) assert.ok(!(k in e), k)
  assert.equal(e.PATH, '/bin')
  assert.equal(e.HOME, '/h')
  assert.deepEqual(buildEnv({}), { CGO_ENABLED: '0' }) // пустое окружение
})

test('rebuildSetup: копия метки, npm ci, build.sh --installer, файл переносится, копия удалена', () => {
  const { r, work } = repo()
  fs.mkdirSync(work, { recursive: true })
  fs.writeFileSync(path.join(work, NAME), 'DRAFT')
  const log = []
  const file = rebuildSetup({ tag: TAG, version: '0.25.1', work, repo: r, env: { PATH: process.env.PATH, GOFLAGS: '-x' }, run: runner(log, { setup: Buffer.from('MZ-setup') }) })
  assert.equal(file, path.join(work, 'rebuilt', NAME))
  assert.notEqual(file, path.join(work, NAME), 'пересборка не затирает черновик с тем же именем')
  assert.equal(fs.readFileSync(file, 'utf8'), 'MZ-setup')
  assert.equal(fs.readFileSync(path.join(work, NAME), 'utf8'), 'DRAFT', 'черновик цел')
  const src = path.join(work, 'src')
  const npm = log.find((c) => c.cmd === 'npm')
  assert.deepEqual(npm.args, ['ci'])
  assert.equal(npm.cwd, path.join(src, 'frontend'))
  const build = log.find((c) => c.cmd === 'bash')
  assert.deepEqual(build.args, ['scripts/build.sh', '--installer'])
  assert.equal(build.cwd, src)
  assert.equal(build.env.CGO_ENABLED, '0')
  assert.ok(!('GOFLAGS' in build.env))
  assert.ok(!fs.existsSync(src), 'копия метки удалена')
  assert.ok(!git(r, 'worktree', 'list').includes('src'), 'запись worktree снята')
  // метка была видна в копии: содержимое — из коммита метки
  assert.ok(log.some((c) => c.cmd === 'git' && c.args.includes(`refs/tags/${TAG}`)))
})

test('rebuildSetup: сбой сборки и отсутствие результата — копия всё равно удалена, ошибка наружу', () => {
  for (const mode of [{ failOn: 'npm' }, { failOn: 'bash' }, { setup: null }]) {
    const { r, work } = repo()
    fs.mkdirSync(work, { recursive: true })
    const log = []
    assert.throws(() => rebuildSetup({ tag: TAG, version: '0.25.1', work, repo: r, run: runner(log, mode) }), /не удалось|не создала/)
    assert.ok(!fs.existsSync(path.join(work, 'src')), JSON.stringify(mode))
    assert.ok(!git(r, 'worktree', 'list').includes('src'), JSON.stringify(mode))
    assert.ok(!fs.existsSync(path.join(work, NAME)), 'неполный результат не оставлен')
  }
})

test('rebuildSetup: метки нет — отказ с понятным текстом, до npm и сборки не доходит', () => {
  const { r, work } = repo()
  fs.mkdirSync(work, { recursive: true })
  const log = []
  assert.throws(() => rebuildSetup({ tag: 'v9.9.9', version: '9.9.9', work, repo: r, run: runner(log) }), /не удалось взять метку v9\.9\.9/)
  assert.ok(!log.some((c) => c.cmd === 'npm' || c.cmd === 'bash'))
})

test('rebuildSetup: остатки прошлого сбойного прогона (запись worktree без папки) не мешают', () => {
  const { r, work } = repo()
  fs.mkdirSync(work, { recursive: true })
  git(r, 'worktree', 'add', '--detach', path.join(work, 'src'), `refs/tags/${TAG}`)
  fs.rmSync(work, { recursive: true, force: true }) // как делает signRelease в начале
  fs.mkdirSync(work, { recursive: true })
  const file = rebuildSetup({ tag: TAG, version: '0.25.1', work, repo: r, run: runner([]) })
  assert.ok(fs.existsSync(file))
})

test('rebuildSetup: bmpMtime — время выставляется у обоих BMP до npm и сборки; без параметра touch не зовётся', () => {
  const { r, work } = repo()
  fs.mkdirSync(work, { recursive: true })
  const log = []
  const src = path.join(work, 'src')
  const run = runner(log)
  const touched = []
  const touch = (f, sec) => touched.push([f, sec, log.length])
  rebuildSetup({ tag: TAG, version: '0.25.1', work, repo: r, run, bmpMtime: 1792809130, touch })
  assert.deepEqual(BMP_FILES, ['scripts/win/wizard.bmp', 'scripts/win/header.bmp'])
  assert.deepEqual(touched.map((t) => t.slice(0, 2)), BMP_FILES.map((f) => [path.join(src, f), 1792809130]))
  const firstBuild = log.findIndex((c) => c.cmd === 'npm')
  assert.ok(touched.every((t) => t[2] <= firstBuild), 'штампы выставлены до npm ci и build.sh')
  // граница: ноль — тоже заданное время (эпоха), а не «нет параметра»
  touched.length = 0
  rebuildSetup({ tag: TAG, version: '0.25.1', work, repo: r, run: runner([]), bmpMtime: 0, touch })
  assert.equal(touched.length, 2)
  // без параметра
  touched.length = 0
  rebuildSetup({ tag: TAG, version: '0.25.1', work, repo: r, run: runner([]), touch })
  assert.equal(touched.length, 0)
})

test('rebuildSetup: bmpMtime настоящим utimes — mtime файлов в копии равен заданному', () => {
  const { r, work } = repo()
  fs.mkdirSync(path.join(r, 'scripts', 'win'), { recursive: true })
  for (const f of BMP_FILES) fs.writeFileSync(path.join(r, f), 'bmp')
  git(r, 'add', '.')
  git(r, '-c', 'user.name=t', '-c', 'user.email=t@t', '-c', 'commit.gpgsign=false', 'commit', '-q', '-m', 'bmp')
  git(r, 'tag', '-f', TAG)
  fs.mkdirSync(work, { recursive: true })
  const seen = []
  const run = (cmd, args, o) => {
    if (cmd === 'bash') for (const f of BMP_FILES) seen.push(fs.statSync(path.join(o.cwd, f)).mtimeMs)
    return runner([])(cmd, args, o)
  }
  rebuildSetup({ tag: TAG, version: '0.25.1', work, repo: r, run, bmpMtime: 1792809130 })
  assert.deepEqual(seen, [1792809130000, 1792809130000])
})

test('rebuildSetup: ref вместо метки — копия берётся с этого коммита, метка не нужна', () => {
  const { r, work } = repo()
  fs.mkdirSync(work, { recursive: true })
  const sha = git(r, 'rev-parse', 'HEAD').trim()
  const log = []
  const file = rebuildSetup({ tag: 'v9.9.9', version: '0.25.1', ref: sha, work, repo: r, run: runner(log) })
  assert.ok(fs.existsSync(file))
  const add = log.find((c) => c.cmd === 'git' && c.args[0] === 'worktree' && c.args[1] === 'add')
  assert.equal(add.args.at(-1), sha)
  assert.ok(!fs.existsSync(path.join(work, 'src')), 'копия удалена')
})

test('rebuildSetup: ref неизвестен — отказ с текстом про коммит, а не про метку', () => {
  const { r, work } = repo()
  fs.mkdirSync(work, { recursive: true })
  const log = []
  assert.throws(() => rebuildSetup({ tag: TAG, version: '0.25.1', ref: 'deadbeef'.repeat(5), work, repo: r, run: runner(log) }), /не удалось взять коммит/)
  assert.ok(!log.some((c) => c.cmd === 'npm' || c.cmd === 'bash'))
})

test('rebuildSetup: verify видит копию и собранный установщик до удаления копии; его сбой не оставляет копию', () => {
  const { r, work } = repo()
  fs.mkdirSync(work, { recursive: true })
  let seen
  rebuildSetup({ tag: TAG, version: '0.25.1', work, repo: r, run: runner([]), verify: ({ src, setup }) => (seen = { src, setup, exists: fs.existsSync(setup) }) })
  assert.equal(seen.src, path.join(work, 'src'))
  assert.equal(seen.setup, path.join(work, 'src', 'build', 'bin', NAME))
  assert.equal(seen.exists, true)
  fs.rmSync(path.join(work, 'rebuilt'), { recursive: true, force: true })
  assert.throws(
    () =>
      rebuildSetup({
        tag: TAG,
        version: '0.25.1',
        work,
        repo: r,
        run: runner([]),
        verify: () => {
          throw new Error('verify упал')
        },
      }),
    /verify упал/,
  )
  assert.ok(!fs.existsSync(path.join(work, 'src')))
  assert.ok(!fs.existsSync(path.join(work, 'rebuilt', NAME)), 'после упавшей проверки результата нет')
})
