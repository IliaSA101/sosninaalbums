// Тесты release-preflight.mjs: проверки до метки без сети, makensis и сборки — зависимости подменены (как в release-sign.test).
// Запуск: node --test scripts/release-preflight.test.mjs
import assert from 'node:assert/strict'
import { execFileSync, spawnSync } from 'node:child_process'
import fs from 'node:fs'
import os from 'node:os'
import path from 'node:path'
import { test } from 'node:test'
import { fileURLToPath } from 'node:url'
import { rebuildSetup } from './release-rebuild.mjs'
import {
  NSIS_CI,
  PACK_RUNNER,
  checkKey,
  checkReproducible,
  compareWithCi,
  firstDiff,
  goToolchain,
  installerInputs,
  jobRunner,
  nodeWanted,
  quickChecks,
  rebuildChecks,
  report,
  wailsWanted,
} from './release-preflight.mjs'

const root = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..')
const rd = (rel) => fs.readFileSync(path.join(root, rel), 'utf8')
const VER = '0.25.1'
const NAME = `SosninaAlbums-${VER}-windows-x64-setup.exe`

// ---- разбор файлов ----

test('jobRunner: runs-on нужного задания, а не соседнего; CRLF; нет задания; кавычки', () => {
  const y = 'jobs:\n  guide:\n    runs-on: ubuntu-latest\n  pack:\n    name: x\n    needs: [a]\n    runs-on: ubuntu-24.04\n  release:\n    runs-on: windows-latest\n'
  assert.equal(jobRunner(y, 'pack'), 'ubuntu-24.04')
  assert.equal(jobRunner(y, 'guide'), 'ubuntu-latest')
  assert.equal(jobRunner(y.replace(/\n/g, '\r\n'), 'pack'), 'ubuntu-24.04')
  assert.equal(jobRunner(y, 'nope'), null)
  assert.equal(jobRunner('jobs:\n  pack:\n    name: x\n  next:\n    runs-on: a\n', 'pack'), null, 'строка соседнего задания не в счёт')
  assert.equal(jobRunner('jobs:\n  pack:\n    runs-on: "ubuntu-24.04"\n', 'pack'), 'ubuntu-24.04')
  assert.equal(jobRunner('', 'pack'), null)
})

test('настоящие ci.yml, release.yml и go.mod разбираются; раннер pack закреплён', () => {
  assert.match(nodeWanted(rd('.github/workflows/ci.yml')), /^\d+\.\d+\.\d+$/)
  assert.match(goToolchain(rd('go.mod')), /^\d+\.\d+\.\d+$/)
  assert.match(wailsWanted(rd('go.mod')), /^v\d+\.\d+\.\d+$/)
  // NSIS_CI верен только для закреплённого раннера
  assert.equal(jobRunner(rd('.github/workflows/release.yml'), 'pack'), PACK_RUNNER)
  assert.match(NSIS_CI, /^v\d+\.\d+(-\d+)?$/)
  assert.equal(nodeWanted(''), null)
  assert.equal(goToolchain('go 1.26.0\n'), null)
  assert.equal(wailsWanted(''), null)
})

test('installerInputs: exe, значок, оба BMP мастера и четыре DPI-картинки; firstDiff на краях', () => {
  const l = installerInputs(VER)
  assert.equal(l[0], `build/bin/SosninaAlbums-${VER}.exe`)
  assert.equal(l.length, 8)
  assert.ok(l.includes('scripts/win/header.bmp') && l.includes('scripts/win/wizard-175.bmp'))
  assert.equal(firstDiff(Buffer.from('ab'), Buffer.from('ab')), -1)
  assert.equal(firstDiff(Buffer.from('ab'), Buffer.from('ax')), 1)
  assert.equal(firstDiff(Buffer.from('ab'), Buffer.from('abc')), 2)
  assert.equal(firstDiff(Buffer.alloc(0), Buffer.alloc(0)), -1)
})

// ---- ключ ----

test('checkKey: нет файла, не файл, пустой, права шире 0600 (кроме Windows), хороший', () => {
  const st = (o) => () => ({ isFile: () => true, size: 100, mode: 0o100600, ...o })
  assert.match(checkKey('/k', () => { throw new Error('ENOENT') }, 'linux').hint, /нет файла/)
  assert.equal(checkKey('/k', st({ isFile: () => false }), 'linux').ok, false)
  assert.equal(checkKey('/k', st({ size: 0 }), 'linux').ok, false)
  assert.match(checkKey('/k', st({ mode: 0o100644 }), 'linux').hint, /chmod 600/)
  assert.equal(checkKey('/k', st({ mode: 0o100640 }), 'darwin').ok, false)
  assert.equal(checkKey('/k', st({ mode: 0o100666 }), 'win32').ok, true, 'на Windows права не проверяются')
  assert.equal(checkKey('/k', st({}), 'linux').ok, true)
})

test('ключ не читается и не печатается: подмена чтения на путь ключа не срабатывает, в выводе нет содержимого', () => {
  const dir = fs.mkdtempSync(path.join(os.tmpdir(), 'preflight-key-'))
  const key = path.join(dir, 'release-key.pem')
  fs.writeFileSync(key, 'SECRET-KEY-CONTENT', { mode: 0o600 })
  const touched = []
  const orig = { read: fs.readFileSync, open: fs.openSync }
  fs.readFileSync = (p, ...a) => (String(p) === key ? (touched.push('read'), orig.read(p, ...a)) : orig.read(p, ...a))
  fs.openSync = (p, ...a) => (String(p) === key ? (touched.push('open'), orig.open(p, ...a)) : orig.open(p, ...a))
  try {
    const r = checkKey(key, fs.statSync, process.platform)
    assert.equal(r.ok, true)
    assert.ok(!JSON.stringify(r).includes('SECRET'))
    assert.deepEqual(touched, [], 'содержимое ключа не открывалось')
  } finally {
    fs.readFileSync = orig.read
    fs.openSync = orig.open
    fs.rmSync(dir, { recursive: true, force: true })
  }
})

// ---- быстрые проверки ----

/** Окружение, в котором всё хорошо; ov переопределяет ответ exec по ключу «cmd arg0» и файлы. */
function goodDeps(ov = {}) {
  const files = {
    '.github/workflows/release.yml': 'jobs:\n  pack:\n    runs-on: ubuntu-24.04\n',
    '.github/workflows/ci.yml': 'node-version: "24.11.1"\n',
    'go.mod': 'go 1.26.0\ntoolchain go1.26.10\nrequire github.com/wailsapp/wails/v2 v2.16.0\n',
    ...ov.files,
  }
  const answers = {
    'makensis -VERSION': { code: 0, out: NSIS_CI },
    'gh auth': { code: 0, out: '' },
    'go version': { code: 0, out: 'go version go1.26.10 linux/amd64' },
    'wails version': { code: 0, out: '\nWails CLI v2.16.0\n' },
    'git status': { code: 0, out: '' },
    'git tag': { code: 0, out: '' },
    'node scripts/release-check.mjs': { code: 0, out: '' },
    curl: { code: 0, out: 'HTTP/2 302\r\nlocation: https://cdn/x\r\n\r\nHTTP/2 200\r\ncontent-length: 1234\r\n\r\n' },
    ...ov.answers,
  }
  return {
    demoZip: () => ('demoZip' in ov ? ov.demoZip : { name: 'wedding-navy-watercolor-demo-1.zip', size: 1234 }),
    exec: (cmd, args) => (cmd === 'curl' ? answers.curl : null) ?? answers[`${cmd} ${args[0]}`] ?? answers[`${cmd} ${args.join(' ')}`] ?? { code: 1, out: 'не подменено' },
    read: (rel) => files[rel] ?? null,
    stat: ov.stat ?? (() => ({ isFile: () => true, size: 10, mode: 0o100600 })),
    platform: 'linux',
    nodeVersion: ov.nodeVersion ?? 'v24.11.1',
  }
}
const failsOf = (res) => res.filter((r) => !r.ok)

test('quickChecks: всё хорошо — одиннадцать OK и нет FAIL', () => {
  const res = quickChecks(VER, goodDeps(), '/k.pem')
  assert.equal(res.length, 11)
  assert.deepEqual(failsOf(res), [])
})

test('quickChecks: каждая поломка даёт ровно один FAIL с подсказкой', () => {
  const cases = [
    ['нет makensis', { answers: { 'makensis -VERSION': { code: 127, out: 'ENOENT' } } }, /apt install nsis/],
    ['другая версия makensis', { answers: { 'makensis -VERSION': { code: 0, out: 'v3.10' } } }, /NSIS_CI/],
    ['раннер pack не закреплён', { files: { '.github/workflows/release.yml': 'jobs:\n  pack:\n    runs-on: ubuntu-latest\n' } }, /ubuntu-24\.04/],
    ['нет release.yml', { files: { '.github/workflows/release.yml': null } }, /ubuntu-24\.04/],
    ['gh не вошёл', { answers: { 'gh auth': { code: 1, out: 'not logged' } } }, /gh auth login/],
    ['нет ключа', { stat: () => { throw new Error('ENOENT') } }, /нет файла/],
    ['другой Node', { nodeVersion: 'v22.1.0' }, /в CI 24\.11\.1/],
    ['Go другой', { answers: { 'go version': { code: 0, out: 'go version go1.25.0 linux/amd64' } } }, /go1\.26\.10/],
    ['нет wails', { answers: { 'wails version': { code: 127, out: 'ENOENT' } } }, /go install .*wails@v2\.16\.0/],
    ['другой wails', { answers: { 'wails version': { code: 0, out: 'Wails CLI v2.15.0' } } }, /v2\.16\.0/],
    ['грязная копия', { answers: { 'git status': { code: 0, out: ' M a.go\n' } } }, /закоммитьте/],
    ['метка есть', { answers: { 'git tag': { code: 0, out: 'v0.25.1\n' } } }, /не перевешиваем/],
    ['release-check красный', { answers: { 'node scripts/release-check.mjs': { code: 1, out: 'a\nb\n' } } }, /a \| b/],
    ['ассет примера не опубликован', { answers: { curl: { code: 0, out: 'HTTP/2 404\n\n' } } }, /опубликуйте релиз wedding-navy-watercolor-demo-v1/],
    ['ассет примера другой длины', { answers: { curl: { code: 0, out: 'HTTP/2 200\ncontent-length: 99\n' } } }, /длина в сети 99/],
    ['нет сети для ассета', { answers: { curl: { code: 6, out: '' } } }, /нет сети/],
    ['нет zip примера', { demoZip: null }, /нет ровно одного zip/],
  ]
  for (const [name, ov, hint] of cases) {
    const bad = failsOf(quickChecks(VER, goodDeps(ov), '/k.pem'))
    assert.equal(bad.length, 1, `${name}: ${JSON.stringify(bad)}`)
    assert.match(bad[0].hint, hint, name)
  }
})

// ---- воспроизводимость ----

/** Копия коммита во временной папке: входы установщика на месте, fake run пишет установщик, зависящий от mtime «чувствительных» входов. */
function fakeSrc(sensitive = []) {
  const src = fs.mkdtempSync(path.join(os.tmpdir(), 'preflight-src-'))
  for (const f of installerInputs(VER)) {
    fs.mkdirSync(path.dirname(path.join(src, f)), { recursive: true })
    fs.writeFileSync(path.join(src, f), f)
  }
  const stamp = (f) => Math.floor(fs.statSync(path.join(src, f)).mtimeMs / 1000)
  const content = () => `setup:${sensitive.map((f) => `${f}=${stamp(f)}`).join(',')}`
  const calls = []
  const io = {
    run: (cmd, args, o) => {
      calls.push([cmd, ...args])
      const out = args[args.indexOf('--out') + 1]
      fs.mkdirSync(out, { recursive: true })
      fs.writeFileSync(path.join(out, NAME), content())
      assert.equal(o.cwd, src)
    },
    touch: (f, sec) => fs.utimesSync(f, sec, sec),
    mtime: (f) => (fs.existsSync(f) ? fs.statSync(f).mtimeMs / 1000 : null),
    read: (f) => fs.readFileSync(f),
    now: () => Date.now(),
  }
  const setup = path.join(src, 'build', 'bin', NAME)
  fs.mkdirSync(path.dirname(setup), { recursive: true })
  fs.writeFileSync(setup, content())
  return { src, io, setup, calls }
}
const rm = (d) => fs.rmSync(d, { recursive: true, force: true })

test('checkReproducible: байты не зависят от mtime — ok, installer.sh один раз; mtime входов после проверки прежние', () => {
  const { src, io, setup, calls } = fakeSrc([])
  try {
    const ms = (f) => Math.round(fs.statSync(path.join(src, f)).mtimeMs) // utimes теряет доли миллисекунды
    const before = installerInputs(VER).map(ms)
    const r = checkReproducible({ src, version: VER, setup, io, env: {} })
    assert.deepEqual(r, { ok: true, culprits: [], at: -1 })
    assert.equal(calls.length, 1)
    assert.deepEqual(calls[0].slice(0, 2), ['bash', 'scripts/win/installer.sh'])
    assert.deepEqual(installerInputs(VER).map(ms), before)
  } finally {
    rm(src)
  }
})

test('checkReproducible: mtime BMP попадает в байты — FAIL с именем файла-виновника, прочие не названы; mtime восстановлен', () => {
  const bmp = 'scripts/win/header.bmp'
  const { src, io, setup } = fakeSrc([bmp])
  try {
    const ms = () => Math.round(fs.statSync(path.join(src, bmp)).mtimeMs)
    const before = ms()
    const r = checkReproducible({ src, version: VER, setup, io, env: {} })
    assert.equal(r.ok, false)
    assert.deepEqual(r.culprits, [bmp])
    assert.ok(r.at >= 0)
    assert.equal(ms(), before)
  } finally {
    rm(src)
  }
})

test('checkReproducible: несколько чувствительных входов — все названы; нет части входов (нет icon.ico) — проверка идёт по остальным', () => {
  const two = ['scripts/win/header.bmp', 'scripts/win/wizard.bmp']
  const { src, io, setup } = fakeSrc(two)
  try {
    assert.deepEqual(checkReproducible({ src, version: VER, setup, io, env: {} }).culprits.sort(), [...two].sort())
    fs.rmSync(path.join(src, 'build', 'windows', 'icon.ico'))
    assert.deepEqual(checkReproducible({ src, version: VER, setup, io, env: {} }).culprits.length, 2)
  } finally {
    rm(src)
  }
})

// ---- сверка с CI и пересборка ----

test('compareWithCi: равны — OK, разный байт и разная длина — FAIL с позицией', () => {
  assert.equal(compareWithCi(NAME, Buffer.from('abc'), Buffer.from('abc'), '7').ok, true)
  assert.match(compareWithCi(NAME, Buffer.from('abc'), Buffer.from('abd'), '7').hint, /байт 2/)
  assert.match(compareWithCi(NAME, Buffer.from('abc'), Buffer.from('ab'), '7').hint, /3 байт, здесь 2/)
  assert.equal(compareWithCi(NAME, Buffer.alloc(0), Buffer.alloc(0), '7').ok, true)
})

/** Зависимости rebuildChecks: rebuild вызывает verify на fakeSrc, установщик локальный — localBytes, CI — ciBytes. */
function rebuildDeps({ sensitive = [], local = 'LOCAL', ci = 'LOCAL', run = {}, rebuildError, head = 'a'.repeat(40) } = {}) {
  const { src, io, setup } = fakeSrc(sensitive)
  const tmp = fs.mkdtempSync(path.join(os.tmpdir(), 'preflight-out-'))
  const rebuilt = path.join(tmp, NAME)
  fs.writeFileSync(rebuilt, local)
  const ciFile = path.join(tmp, 'ci.exe')
  fs.writeFileSync(ciFile, ci)
  const seen = {}
  const answers = { 'git rev-parse': { code: 0, out: `${head}\n` }, 'git cat-file': { code: 0, out: '' }, ...run }
  return {
    seen,
    cleanup: () => (rm(src), rm(tmp)),
    deps: {
      exec: (cmd, args) => answers[`${cmd} ${args[0]}`] ?? { code: 1, out: '' },
      rebuild: (a) => {
        seen.rebuild = { ref: a.ref, version: a.version }
        if (rebuildError) throw new Error(rebuildError)
        a.verify({ src, setup })
        return rebuilt
      },
      readBuf: (f) => fs.readFileSync(f),
      download: (id) => (seen.downloaded = id) && ciFile,
      io,
      env: {},
    },
  }
}

test('rebuildChecks: без --against — сборка с HEAD, воспроизводимость ok, сверки с CI нет', () => {
  const h = rebuildDeps()
  try {
    const res = rebuildChecks(VER, {}, h.deps)
    assert.deepEqual(failsOf(res), [])
    assert.equal(res.length, 2)
    assert.equal(h.seen.rebuild.ref, 'a'.repeat(40))
    assert.equal(h.seen.downloaded, undefined)
  } finally {
    h.cleanup()
  }
})

test('rebuildChecks: воспроизводимость нарушена — FAIL называет файл; сбой самой сборки — один FAIL с причиной', () => {
  const h = rebuildDeps({ sensitive: ['scripts/win/wizard.bmp'] })
  try {
    const bad = failsOf(rebuildChecks(VER, {}, h.deps))
    assert.equal(bad.length, 1)
    assert.match(bad[0].hint, /scripts\/win\/wizard\.bmp/)
  } finally {
    h.cleanup()
  }
  const e = rebuildDeps({ rebuildError: 'npm ci: не удалось' })
  try {
    const res = rebuildChecks(VER, {}, e.deps)
    assert.equal(res.length, 1)
    assert.match(res[0].hint, /npm ci/)
  } finally {
    e.cleanup()
  }
})

test('rebuildChecks --against: сборка на headSha прогона, сверка с артефактом; расхождение и все отказы — FAIL', () => {
  const sha = 'b'.repeat(40)
  const view = (o) => ({ 'gh run': { code: 0, out: JSON.stringify({ headSha: sha, conclusion: 'success', ...o }) } })
  const same = rebuildDeps({ run: view({}) })
  try {
    const res = rebuildChecks(VER, { against: '555' }, same.deps)
    assert.deepEqual(failsOf(res), [])
    assert.equal(res.length, 3)
    assert.equal(same.seen.rebuild.ref, sha)
    assert.equal(same.seen.downloaded, '555')
  } finally {
    same.cleanup()
  }
  const diff = rebuildDeps({ run: view({}), ci: 'CI-OTHER' })
  try {
    const bad = failsOf(rebuildChecks(VER, { against: '555' }, diff.deps))
    assert.equal(bad.length, 1)
    assert.match(bad[0].what, /не совпал с CI/)
  } finally {
    diff.cleanup()
  }
  const refusals = [
    ['прогон не успешен', { 'gh run': { code: 0, out: JSON.stringify({ headSha: sha, conclusion: 'failure' }) } }, /failure/],
    ['gh отдал не JSON', { 'gh run': { code: 1, out: 'нет такого прогона' } }, /gh run view/],
    ['коммита нет локально', { ...view({}), 'git cat-file': { code: 128, out: '' } }, /не найден здесь/],
  ]
  for (const [name, run, re] of refusals) {
    const h = rebuildDeps({ run })
    try {
      const res = rebuildChecks(VER, { against: '555' }, h.deps)
      assert.equal(res.length, 1, name)
      assert.match(res[0].what, re, name)
      assert.equal(h.seen.rebuild, undefined, `${name}: до сборки не дошли`)
    } finally {
      h.cleanup()
    }
  }
  const noArt = rebuildDeps({ run: view({}) })
  noArt.deps.download = () => null
  try {
    assert.match(rebuildChecks(VER, { against: '555' }, noArt.deps)[0].what, /packages/)
  } finally {
    noArt.cleanup()
  }
})

test('rebuildChecks: настоящий rebuildSetup с ref и verify (git настоящий, сборка подменена) — вывод ок, копия удалена', () => {
  const dir = fs.mkdtempSync(path.join(os.tmpdir(), 'preflight-real-'))
  const repo = path.join(dir, 'repo')
  const g = (...a) => execFileSync('git', ['-c', 'user.name=t', '-c', 'user.email=t@t', '-c', 'commit.gpgsign=false', ...a], { cwd: repo, encoding: 'utf8', stdio: ['ignore', 'pipe', 'pipe'] })
  fs.mkdirSync(path.join(repo, 'frontend'), { recursive: true })
  fs.writeFileSync(path.join(repo, 'frontend', 'package.json'), '{}')
  for (const f of installerInputs(VER).filter((x) => x.startsWith('scripts/'))) {
    fs.mkdirSync(path.dirname(path.join(repo, f)), { recursive: true })
    fs.writeFileSync(path.join(repo, f), f)
  }
  execFileSync('git', ['init', '-q', repo])
  g('config', 'core.autocrlf', 'false') // не зависеть от CRLF-настроек машины
  g('add', '.')
  g('commit', '-q', '-m', 'x')
  const work = path.join(dir, 'work')
  fs.mkdirSync(work, { recursive: true })
  const run = (cmd, args, o) => {
    if (cmd === 'git') return execFileSync('git', args, { cwd: o.cwd, stdio: 'ignore' })
    if (cmd === 'bash' && args[0] === 'scripts/build.sh') {
      for (const f of ['build/bin', 'build/windows']) fs.mkdirSync(path.join(o.cwd, f), { recursive: true })
      fs.writeFileSync(path.join(o.cwd, installerInputs(VER)[0]), 'exe')
      fs.writeFileSync(path.join(o.cwd, 'build/windows/icon.ico'), 'ico')
      fs.writeFileSync(path.join(o.cwd, 'build/bin', NAME), 'SETUP')
    } else if (cmd === 'bash') {
      const out = args[args.indexOf('--out') + 1]
      fs.mkdirSync(out, { recursive: true })
      fs.writeFileSync(path.join(out, NAME), 'SETUP')
    }
    return Buffer.alloc(0)
  }
  try {
    const head = g('rev-parse', 'HEAD').trim()
    const io = { run, touch: (f, s) => fs.utimesSync(f, s, s), mtime: (f) => (fs.existsSync(f) ? fs.statSync(f).mtimeMs / 1000 : null), read: (f) => fs.readFileSync(f), now: () => Date.now() }
    const deps = {
      exec: (cmd, args) => (cmd === 'git' ? { code: 0, out: head } : { code: 1, out: '' }),
      rebuild: ({ ref, version, verify }) => rebuildSetup({ tag: `v${version}`, version, ref, work, repo, run, verify }),
      readBuf: (f) => fs.readFileSync(f),
      io,
      env: {},
    }
    const res = rebuildChecks(VER, {}, deps)
    assert.deepEqual(failsOf(res), [], JSON.stringify(res))
    assert.equal(res.length, 2)
    assert.ok(!fs.existsSync(path.join(work, 'src')))
  } finally {
    rm(dir)
  }
})

// ---- вывод и CLI ----

test('report: код 0 без FAIL, 1 при любом; пустой список — OK', () => {
  assert.deepEqual(report([]), { text: 'ИТОГО: OK (0)', code: 0 })
  const r = report([{ ok: true, what: 'a', hint: '' }, { ok: false, what: 'b', hint: 'как исправить' }])
  assert.equal(r.code, 1)
  assert.match(r.text, /^OK   a\nFAIL b — как исправить\nИТОГО: FAIL \(1 из 2\)$/)
})

test('CLI без версии и с --against без значения — использование, код 2', () => {
  const script = path.join(root, 'scripts', 'release-preflight.mjs')
  for (const args of [[], ['--rebuild'], ['0.25.1', '--against'], ['0.25.1', '--against', '--rebuild']]) {
    const r = spawnSync('node', [script, ...args], { encoding: 'utf8' })
    assert.equal(r.status, 2, args.join(' '))
    assert.match(r.stderr, /использование/)
  }
})
