// Тесты release-sign.mjs (спека 111 №111-9): body и конверт, отказы подписи, общая фикстура для Go и node.
// Запуск: node --test scripts/release-sign.test.mjs. Настоящих ключей не касается (docs/private/*key*.pem).
//
// Фикстура internal/update/testdata/ — конверты, которые читает Go (TestManifestFixture):
//   update.json          верный конверт {"sig","body"}; update.pub — открытый ключ (base64, 32 байта);
//   body.json            внутренний JSON (то, что подписано), для сверки разбора;
//   update-tampered.json тот же sig, но в body изменён один знак — подпись не сходится;
//   update-otherkey.json подписан другим ключом — update.pub его не принимает.
// Ключи воспроизводимы: seed = sha256("sosninaalbums-update-test-key") и sha256("sosninaalbums-update-other-key"),
// подпись ed25519 детерминирована — тест пересобирает файлы и сверяет побайтно; UPDATE_FIXTURE=1 пишет их заново.
import assert from 'node:assert/strict'
import crypto from 'node:crypto'
import fs from 'node:fs'
import os from 'node:os'
import path from 'node:path'
import { spawnSync } from 'node:child_process'
import { test } from 'node:test'
import { fileURLToPath } from 'node:url'
import { publicRaw, verifyBytes } from './library-pack.mjs'
import {
  ASSET,
  SignError,
  buildBody,
  cleanNote,
  keyFromSeed,
  loadKey,
  newKey,
  openEnvelope,
  parseSums,
  compareRebuilt,
  parseWhatsNew,
  programKeys,
  sealEnvelope,
  setupName,
  signRelease,
  testKeyPath,
} from './release-sign.mjs'

const REPO = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..')
const FIXTURE = path.join(REPO, 'internal', 'update', 'testdata')
const seed = (s) => crypto.createHash('sha256').update(s).digest()
const KEY = keyFromSeed(seed('sosninaalbums-update-test-key'))
const OTHER = keyFromSeed(seed('sosninaalbums-update-other-key'))
const sha = (b) => crypto.createHash('sha256').update(b).digest('hex')
const tmp = () => fs.mkdtempSync(path.join(os.tmpdir(), 'relsign-'))
const throwsSign = (fn, re) => assert.throws(fn, (e) => e instanceof SignError && re.test(e.message), String(re))

const NOTES = ['Развороты: меню правой кнопки', 'Новые раскладки «Разброс»', 'Файл альбома обновлён']
const SETUP = (ver = '0.25.1') => ({ name: setupName(ver), size: 8166433, sha256: sha('sosninaalbums-update-fixture-setup') })
const body = (over = {}) => buildBody({ version: '0.25.1', date: '2026-11-02', minAuto: '0.25.0', notes: NOTES, setup: SETUP(), ...over })

// ---- фикстура Go + node ----

function fixtureFiles() {
  const b = body()
  const env = sealEnvelope(b, KEY)
  const bytes = Buffer.from(JSON.stringify(b))
  const tampered = Buffer.from(bytes)
  tampered[tampered.indexOf('0.25.1')] ^= 0x01 // «1» -> «0» в версии: тот же sig, другой body
  return {
    'update.json': JSON.stringify(env) + '\n',
    'update.pub': publicRaw(KEY) + '\n',
    'body.json': bytes.toString('utf8') + '\n',
    'update-tampered.json': JSON.stringify({ sig: env.sig, body: tampered.toString('base64') }) + '\n',
    'update-otherkey.json': JSON.stringify(sealEnvelope(b, OTHER)) + '\n',
  }
}

test('фикстура internal/update/testdata: файлы совпадают с воспроизведёнными', () => {
  const want = fixtureFiles()
  if (process.env.UPDATE_FIXTURE) {
    fs.mkdirSync(FIXTURE, { recursive: true })
    for (const [name, text] of Object.entries(want)) fs.writeFileSync(path.join(FIXTURE, name), text)
  }
  for (const [name, text] of Object.entries(want)) {
    assert.equal(fs.readFileSync(path.join(FIXTURE, name), 'utf8'), text, `${name}: запустите UPDATE_FIXTURE=1 node --test scripts/release-sign.test.mjs`)
  }
})

test('фикстура: верный конверт открывается, порча и чужой ключ — отказ', () => {
  const pub = fs.readFileSync(path.join(FIXTURE, 'update.pub'), 'utf8').trim()
  assert.equal(Buffer.from(pub, 'base64').length, 32)
  const read = (n) => fs.readFileSync(path.join(FIXTURE, n), 'utf8')
  assert.deepEqual(openEnvelope(read('update.json'), pub), JSON.parse(read('body.json')))
  throwsSign(() => openEnvelope(read('update-tampered.json'), pub), /подпись/)
  throwsSign(() => openEnvelope(read('update-otherkey.json'), pub), /подпись/)
  const small = read('update.json').length
  assert.ok(small < 16 << 10, `конверт ${small} байт — меньше предела 16 КБ`)
})

// ---- body ----

test('body: ключи по порядку design §5.1, строки очищены, файлы по ключу windows-x64-setup', () => {
  const b = body({ notes: ['  раз\u0007  два\n', '', 'три'] })
  assert.deepEqual(Object.keys(b), ['format', 'app', 'version', 'date', 'minAuto', 'notes', 'files'])
  assert.deepEqual(b.notes, ['раз два', 'три'])
  assert.equal(b.format, 1)
  assert.equal(b.app, 'sosninaalbums')
  assert.deepEqual(Object.keys(b.files['windows-x64-setup']), ['name', 'size', 'sha256'])
  assert.equal(cleanNote('  а\t\tб \u0000'), 'а б')
  assert.equal(body({ minAuto: '' }).minAuto, '') // minAuto необязателен
})

test('body: отказы — версия, дата, minAuto, строки, имя, размер, хеш', () => {
  throwsSign(() => body({ version: '0.25' }), /ждали X\.Y\.Z/)
  throwsSign(() => body({ version: '0.25.1-rc1' }), /ждали X\.Y\.Z/)
  throwsSign(() => body({ date: '2.11.2026' }), /дата/)
  throwsSign(() => body({ minAuto: 'abc' }), /minAuto/)
  throwsSign(() => body({ notes: [] }), /пусто/)
  throwsSign(() => body({ notes: ['', '  '] }), /пусто/)
  assert.equal(body({ notes: Array.from({ length: 8 }, (_, i) => `строка ${i}`) }).notes.length, 8) // предел включительно
  throwsSign(() => body({ notes: Array.from({ length: 9 }, (_, i) => `строка ${i}`) }), /9 строк, больше 8/)
  assert.equal(body({ notes: ['я'.repeat(300)] }).notes[0].length, 300)
  throwsSign(() => body({ notes: ['я'.repeat(301)] }), /длиннее 300/)
  throwsSign(() => body({ setup: { ...SETUP(), name: 'SosninaAlbums-0.25.0-windows-x64-setup.exe' } }), /имя установщика/)
  for (const size of [0, (1 << 20) - 1, (80 << 20) + 1, 8.5, -1]) throwsSign(() => body({ setup: { ...SETUP(), size } }), /размер/)
  for (const size of [1 << 20, 80 << 20]) assert.equal(body({ setup: { ...SETUP(), size } }).files['windows-x64-setup'].size, size) // края включительно
  for (const bad of ['', 'ABC', 'a'.repeat(63), 'A'.repeat(64)]) throwsSign(() => body({ setup: { ...SETUP(), sha256: bad } }), /sha256/)
})

test('whatsnew.ts: версия и строки с кавычками и экранированием; незакрытый массив — отказ', () => {
  const ts = `// комментарий ]\nexport const WHATS_NEW_VERSION = '0.25.1'\n\nexport const WHATS_NEW: readonly string[] = [\n  'Кнопка «Обновить» — \\'раз\\'', // хвост ]\n  "два \\u00e9 \\n три",\n  \`три\`,\n]\n`
  const r = parseWhatsNew(ts)
  assert.equal(r.version, '0.25.1')
  assert.deepEqual(r.notes, ["Кнопка «Обновить» — 'раз'", 'два é \n три', 'три'])
  assert.deepEqual(parseWhatsNew("export const WHATS_NEW_VERSION = '1.0.0'\nexport const WHATS_NEW = []\n").notes, [])
  throwsSign(() => parseWhatsNew("export const WHATS_NEW_VERSION = '1.0.0'\nexport const WHATS_NEW = ['а'"), /не закрыт/)
  throwsSign(() => parseWhatsNew("export const WHATS_NEW = ['а']"), /WHATS_NEW_VERSION/)
  throwsSign(() => parseWhatsNew("export const WHATS_NEW_VERSION = '1.0.0'"), /WHATS_NEW/)
  // настоящий файл репозитория разбирается и укладывается в пределы
  const real = parseWhatsNew(fs.readFileSync(path.join(REPO, 'frontend/src/shell/dialogs/whatsnew.ts'), 'utf8'))
  assert.match(real.version, /^\d+\.\d+\.\d+$/)
  assert.ok(real.notes.length >= 1 && body({ version: real.version, setup: SETUP(real.version), notes: real.notes }).notes.length <= 8)
})

test('SHA256SUMS: формат sha256sum, «*» у имени, пустые строки; мусор и два хеша — отказ', () => {
  const a = 'a'.repeat(64)
  const b = 'B'.repeat(64)
  const m = parseSums(`${a}  one.exe\n${b} *dir/two.exe\n\n`)
  assert.equal(m.get('one.exe'), a)
  assert.equal(m.get('two.exe'), 'b'.repeat(64)) // имя без пути, хеш нижним регистром
  assert.equal(parseSums('').size, 0)
  throwsSign(() => parseSums('hash one.exe\n'), /не по формату/)
  throwsSign(() => parseSums(`${a}  x.exe\n${'c'.repeat(64)}  x.exe\n`), /два разных хеша/)
})

// ---- подпись и ключи ----

test('конверт: подпись по байтам body; верный ключ принимает, чужой и порча — нет', () => {
  const env = sealEnvelope(body(), KEY)
  assert.deepEqual(Object.keys(env), ['sig', 'body'])
  const bytes = Buffer.from(env.body, 'base64')
  assert.equal(verifyBytes(bytes, env.sig, publicRaw(KEY)), true)
  assert.equal(verifyBytes(bytes, env.sig, publicRaw(OTHER)), false)
  const text = JSON.stringify(env)
  assert.equal(openEnvelope(text, publicRaw(KEY)).version, '0.25.1')
  throwsSign(() => openEnvelope(text, publicRaw(OTHER)), /подпись/)
  throwsSign(() => openEnvelope('не json', publicRaw(KEY)), /не JSON/)
  throwsSign(() => openEnvelope('{"sig":1}', publicRaw(KEY)), /нет sig и body/)
})

test('--new-key: файл 0600, открытый ключ 32 байта, существующий не перезаписывается', () => {
  const dir = tmp()
  const file = path.join(dir, 'sub', 'k.pem')
  const pub = newKey(file)
  assert.equal(Buffer.from(pub, 'base64').length, 32)
  assert.equal(fs.statSync(file).mode & 0o077, 0)
  assert.equal(publicRaw(loadKey(file)), pub)
  const before = fs.readFileSync(file)
  throwsSign(() => newKey(file), /не перезаписывается/)
  assert.deepEqual(fs.readFileSync(file), before)
  assert.notEqual(newKey(path.join(dir, 'k2.pem')), pub) // каждая пара своя
  throwsSign(() => loadKey(path.join(dir, 'none.pem')), /нет ключа/)
  fs.writeFileSync(path.join(dir, 'junk.pem'), 'не ключ')
  throwsSign(() => loadKey(path.join(dir, 'junk.pem')), /не читается/)
  const { privateKey } = crypto.generateKeyPairSync('ec', { namedCurve: 'P-256' })
  fs.writeFileSync(path.join(dir, 'ec.pem'), privateKey.export({ type: 'pkcs8', format: 'pem' }))
  throwsSign(() => loadKey(path.join(dir, 'ec.pem')), /ed25519/)
})

// Временная папка внутри <корень>/.gotmp/: --test-key пишет только туда
const tmpIn = () => {
  const base = path.join(REPO, '.gotmp')
  fs.mkdirSync(base, { recursive: true })
  return fs.mkdtempSync(path.join(base, 'relsign-'))
}

test('CLI --new-key и --test-key: печатают открытый ключ, закрытый в вывод не попадает', () => {
  const dir = tmpIn()
  const script = path.join(REPO, 'scripts', 'release-sign.mjs')
  const run = (...args) => spawnSync(process.execPath, [script, ...args], { encoding: 'utf8' })
  const r = run('--new-key', path.join(dir, 'a.pem'))
  assert.equal(r.status, 0, r.stderr)
  const pem = fs.readFileSync(path.join(dir, 'a.pem'), 'utf8')
  const body64 = pem.replace(/-----[^-]+-----|\s/g, '')
  assert.ok(!r.stdout.includes(body64), 'закрытый ключ напечатан')
  assert.match(r.stdout, /открытый ключ \(base64, 32 байта\): [A-Za-z0-9+/]{43}=/)
  assert.equal(run('--new-key', path.join(dir, 'a.pem')).status, 1) // уже есть
  assert.equal(run('--new-key').status, 2)
  assert.equal(run().status, 2)
  const t = run('--test-key', path.join(dir, 't.pem'))
  assert.equal(t.status, 0, t.stderr)
  assert.match(t.stdout, /открытый ключ/)
  assert.equal(run('--test-key', path.join(dir, 't.pem')).status, 0) // одноразовый: заменяется
  fs.rmSync(dir, { recursive: true, force: true })
})

test('--test-key: пишет и удаляет файл только внутри .gotmp/ (docs/private и прочее — отказ, файл цел)', () => {
  const dir = tmp()
  const root = path.join(dir, 'repo')
  fs.mkdirSync(path.join(root, '.gotmp', 'sub'), { recursive: true })
  fs.mkdirSync(path.join(root, 'docs', 'private'), { recursive: true })
  const real = path.join(root, 'docs', 'private', 'release-key.pem')
  fs.writeFileSync(real, 'настоящий ключ')
  // годные: по умолчанию и внутри .gotmp
  assert.equal(testKeyPath(undefined, root), path.join(root, '.gotmp', 'release-test-key.pem'))
  assert.equal(testKeyPath('--dry', root), path.join(root, '.gotmp', 'release-test-key.pem')) // следующий флаг — не путь
  assert.equal(testKeyPath(path.join(root, '.gotmp', 'sub', 't.pem'), root), path.join(root, '.gotmp', 'sub', 't.pem'))
  // негодные: настоящий ключ, соседний каталог, сам .gotmp, выход через .., чужой диск/tmp
  for (const bad of [real, path.join(root, 'x.pem'), path.join(root, '.gotmp'), path.join(root, '.gotmp', '..', 'x.pem'),
    path.join(root, '.gotmp', '..', 'docs', 'private', 'release-key.pem'), path.join(dir, 'other.pem'), path.join(root, '.gotmpx', 'a.pem')]) {
    throwsSign(() => testKeyPath(bad, root), /только внутри \.gotmp/)
  }
  assert.equal(fs.readFileSync(real, 'utf8'), 'настоящий ключ')
  // ссылка внутри .gotmp, ведущая наружу, — тоже отказ
  try {
    fs.symlinkSync(path.join(root, 'docs', 'private'), path.join(root, '.gotmp', 'link'), 'dir')
    throwsSign(() => testKeyPath(path.join(root, '.gotmp', 'link', 'release-key.pem'), root), /только внутри \.gotmp/)
  } catch (e) {
    if (e instanceof SignError || e.code !== 'EPERM') throw e // без прав на ссылки (Windows) проверка пропускается
  }
  // CLI: путь вне .gotmp — код 2, файл не тронут
  const script = path.join(REPO, 'scripts', 'release-sign.mjs')
  const victim = path.join(dir, 'victim.pem')
  fs.writeFileSync(victim, 'чужой файл')
  const r = spawnSync(process.execPath, [script, '--test-key', victim], { encoding: 'utf8' })
  assert.equal(r.status, 2, r.stderr)
  assert.match(r.stderr, /только внутри \.gotmp/)
  assert.equal(fs.readFileSync(victim, 'utf8'), 'чужой файл')
})

test('programKeys: base64-ключи из keys.go; нет файла — null', () => {
  const dir = tmp()
  const f = path.join(dir, 'keys.go')
  const k1 = publicRaw(KEY)
  fs.writeFileSync(f, `package update\n\nvar releaseKeys = []string{\n\t"${k1}", // текущий\n\t"${publicRaw(OTHER)}",\n\t"не ключ",\n}\n`)
  assert.deepEqual(programKeys(f), [k1, publicRaw(OTHER)])
  assert.equal(programKeys(path.join(dir, 'none.go')), null)
})

// ---- выпуск целиком (gh и git подменены) ----

const TAG = 'v0.25.1'
const WHATSNEW = `export const WHATS_NEW_VERSION = '0.25.1'\nexport const WHATS_NEW: readonly string[] = [\n  'раз',\n  'два',\n]\n`

/** Макет: ключ, keys.go, черновик (gh отдаёт файлы), журнал вызовов. over: правки черновика. */
function release(over = {}) {
  const dir = tmp()
  const keyPath = path.join(dir, 'key.pem')
  fs.writeFileSync(keyPath, KEY.export({ type: 'pkcs8', format: 'pem' }), { mode: 0o600 })
  const keysFile = path.join(dir, 'keys.go')
  fs.writeFileSync(keysFile, `var k = []string{"${publicRaw(over.programKey ?? KEY)}"}\n`)
  const setup = over.setup ?? Buffer.alloc(2 << 20, 7)
  const calls = []
  const draft = { [setupName(over.fileVersion ?? '0.25.1')]: setup }
  if (over.sums !== null) draft['SHA256SUMS.txt'] = over.sums ?? `${sha(setup)}  ${setupName('0.25.1')}\n`
  const deps = {
    gh: (args) => {
      calls.push(args)
      if (args[1] === 'download') for (const [n, data] of Object.entries(draft)) fs.writeFileSync(path.join(args[args.indexOf('-D') + 1], n), data)
      return Buffer.alloc(0)
    },
    git: (args) => {
      calls.push(['git', ...args])
      return args[0] === 'show' && args[1] === '-s' ? '2026-11-02\n' : (over.whatsnew ?? WHATSNEW)
    },
    // пересборка из метки: по умолчанию даёт те же байты, что черновик; over.rebuilt — иные
    rebuild: ({ tag, version, work }) => {
      calls.push(['rebuild', tag, version])
      const f = path.join(work, over.rebuiltName ?? setupName(version))
      fs.writeFileSync(f, over.rebuilt ?? setup)
      return f
    },
  }
  const opts = { tag: TAG, keyPath, dry: false, keysFile, work: path.join(dir, 'work'), ...over.opts }
  return { opts, deps, calls, setup, dir }
}

test('выпуск: скачивает, сверяет хеш, подписывает, проверяет и загружает ровно один раз', () => {
  const r = release()
  const out = signRelease(r.opts, r.deps)
  assert.equal(out.uploaded, true)
  assert.equal(path.basename(out.file), ASSET)
  const gh = r.calls.filter((c) => c[0] !== 'git' && c[0] !== 'rebuild')
  assert.deepEqual(gh[0].slice(0, 5), ['release', 'download', TAG, '-p', '*-windows-x64-setup.exe'])
  assert.deepEqual(gh[1], ['release', 'upload', TAG, out.file, '--clobber']) // повторная подпись черновика
  assert.equal(gh.length, 2)
  assert.deepEqual(r.calls.filter((c) => c[0] === 'rebuild'), [['rebuild', TAG, '0.25.1']], 'установщик пересобран ровно раз, до подписи')
  assert.ok(r.calls.some((c) => c[0] === 'git' && c[2] === `${TAG}:frontend/src/shell/dialogs/whatsnew.ts`))
  const parsed = openEnvelope(fs.readFileSync(out.file, 'utf8'), publicRaw(KEY))
  assert.equal(parsed.version, '0.25.1')
  assert.equal(parsed.date, '2026-11-02')
  assert.equal(parsed.minAuto, '0.25.0')
  assert.deepEqual(parsed.notes, ['раз', 'два'])
  assert.deepEqual(parsed.files['windows-x64-setup'], { name: setupName('0.25.1'), size: r.setup.length, sha256: sha(r.setup) })
})

test('выпуск --dry: всё, кроме загрузки; повтор заменяет прежний файл', () => {
  const r = release({ opts: { dry: true } })
  const first = signRelease(r.opts, r.deps)
  assert.equal(first.uploaded, false)
  assert.ok(fs.existsSync(first.file))
  assert.ok(!r.calls.some((c) => c[1] === 'upload'))
  assert.equal(signRelease(r.opts, r.deps).sha256, first.sha256)
})

test('выпуск: отказы — нет SHA256SUMS.txt, хеш не сошёлся, нет строки, нет ключа, чужая версия, ключа нет в программе', () => {
  const noUpload = (r) => assert.ok(!r.calls.some((c) => c[1] === 'upload'), 'при отказе ничего не загружено')
  let r = release({ sums: null })
  throwsSign(() => signRelease(r.opts, r.deps), /нет SHA256SUMS\.txt/)
  noUpload(r)
  r = release({ sums: `${'0'.repeat(64)}  ${setupName('0.25.1')}\n` })
  throwsSign(() => signRelease(r.opts, r.deps), /не равен строке SHA256SUMS/)
  noUpload(r)
  r = release({ sums: `${'0'.repeat(64)}  other.exe\n` })
  throwsSign(() => signRelease(r.opts, r.deps), /нет строки для/)
  r = release({ opts: { keyPath: '/nonexistent/key.pem' } })
  throwsSign(() => signRelease(r.opts, r.deps), /нет ключа/)
  assert.equal(r.calls.length, 0, 'без ключа даже не скачиваем')
  r = release({ whatsnew: WHATSNEW.replace('0.25.1', '0.25.0') })
  throwsSign(() => signRelease(r.opts, r.deps), /WHATS_NEW_VERSION 0\.25\.0/)
  noUpload(r)
  r = release({ fileVersion: '0.25.0' })
  throwsSign(() => signRelease(r.opts, r.deps), /не от версии 0\.25\.1/)
  r = release({ programKey: OTHER })
  throwsSign(() => signRelease(r.opts, r.deps), /нет в internal\/update\/keys\.go/)
  noUpload(r)
  throwsSign(() => signRelease({ ...release().opts, tag: '0.25.1' }, release().deps), /vX\.Y\.Z/)
  r = release({ setup: Buffer.alloc(1000, 1) }) // меньше 1 МБ: программа такой файл не примет
  throwsSign(() => signRelease(r.opts, r.deps), /размер установщика/)
  noUpload(r)
})

test('выпуск: QA-режим --from не загружает и не требует ключ в keys.go', () => {
  const r = release({ programKey: OTHER })
  const from = path.join(r.dir, 'lenta')
  fs.mkdirSync(from)
  fs.writeFileSync(path.join(from, setupName('0.25.1')), r.setup)
  fs.writeFileSync(path.join(from, 'SHA256SUMS.txt'), `${sha(r.setup)}  ${setupName('0.25.1')}\n`)
  fs.writeFileSync(path.join(r.dir, 'wn.ts'), WHATSNEW)
  const out = signRelease({ ...r.opts, from, skipRebuild: true, whatsnew: path.join(r.dir, 'wn.ts'), date: '2026-01-02' }, r.deps)
  assert.equal(out.uploaded, false)
  assert.equal(r.calls.length, 0) // ни gh, ни git
  assert.equal(openEnvelope(fs.readFileSync(out.file, 'utf8'), publicRaw(KEY)).date, '2026-01-02')
  // и в папке без SHA256SUMS.txt — тот же отказ
  fs.rmSync(path.join(from, 'SHA256SUMS.txt'))
  throwsSign(() => signRelease({ ...r.opts, from, skipRebuild: true, whatsnew: path.join(r.dir, 'wn.ts'), date: '2026-01-02' }, r.deps), /нет SHA256SUMS\.txt/)
})

// ---- независимая пересборка установщика (design §5.4, §10) ----

test('compareRebuilt: равные байты проходят; один байт, обрезка и пустая пересборка — отказ с номером байта', () => {
  const a = Buffer.alloc(2 << 20, 7)
  compareRebuilt(TAG, 'x.exe', a, Buffer.from(a))
  const one = Buffer.from(a)
  one[1234567] ^= 1
  throwsSign(() => compareRebuilt(TAG, 'x.exe', a, one), /байт 1234567\)\. Ничего не подписано/)
  throwsSign(() => compareRebuilt(TAG, 'x.exe', a, a.subarray(0, 100)), /пересборка 100 байт.*байт 100\)/)
  throwsSign(() => compareRebuilt(TAG, 'x.exe', a, Buffer.alloc(0)), /байт 0\)/)
  throwsSign(() => compareRebuilt(TAG, 'x.exe', a, Buffer.concat([a, Buffer.alloc(1)])), new RegExp(`байт ${a.length}\\)`))
})

test('выпуск: пересобранный установщик не совпал — отказ, ничего не подписано и не загружено', () => {
  const setup = Buffer.alloc(2 << 20, 7)
  const evil = Buffer.from(setup)
  evil[100] = 8 // подмена на одном байте: sha256 в SHA256SUMS.txt черновика у неё «честный»
  const r = release({ setup: evil, sums: `${sha(evil)}  ${setupName('0.25.1')}\n`, rebuilt: setup })
  throwsSign(() => signRelease(r.opts, r.deps), /не совпал с пересобранным из метки v0\.25\.1.*Ничего не подписано/)
  assert.ok(!r.calls.some((c) => c[1] === 'upload'))
  assert.ok(!fs.existsSync(path.join(r.opts.work, TAG, ASSET)), 'конверт не создан')
  // --dry тоже пересобирает: «почти настоящий» прогон не обходит сверку
  const d = release({ setup: evil, sums: `${sha(evil)}  ${setupName('0.25.1')}\n`, rebuilt: setup, opts: { dry: true } })
  throwsSign(() => signRelease(d.opts, d.deps), /не совпал/)
})

test('выпуск: пересборка дала файл с другим именем — отказ; сбой самой пересборки пробрасывается без подписи', () => {
  let r = release({ rebuiltName: 'other.exe' })
  throwsSign(() => signRelease(r.opts, r.deps), /пересборка дала other\.exe/)
  r = release()
  r.deps.rebuild = () => {
    throw new SignError('npm ci: не удалось')
  }
  throwsSign(() => signRelease(r.opts, r.deps), /npm ci/)
  assert.ok(!r.calls.some((c) => c[1] === 'upload'))
})

test('--skip-rebuild без --from — отказ (код 2): в настоящей подписи пересборка обязательна', () => {
  const r = release({ opts: { skipRebuild: true } })
  assert.throws(() => signRelease(r.opts, r.deps), (e) => e instanceof SignError && e.code === 2 && /только вместе с --from/.test(e.message))
  assert.equal(r.calls.length, 0, 'ни скачивания, ни пересборки')
  // и через CLI: код 2, подписи нет
  const p = spawnSync(process.execPath, [path.join(REPO, 'scripts', 'release-sign.mjs'), TAG, '--skip-rebuild', '--dry'], { encoding: 'utf8' })
  assert.equal(p.status, 2)
  assert.match(p.stderr, /только вместе с --from/)
})

test('QA-режим --from с --skip-rebuild пересборку не зовёт; без --skip-rebuild --from тоже пересобирает', () => {
  const r = release()
  const from = path.join(r.dir, 'lenta')
  fs.mkdirSync(from)
  fs.writeFileSync(path.join(from, setupName('0.25.1')), r.setup)
  fs.writeFileSync(path.join(from, 'SHA256SUMS.txt'), `${sha(r.setup)}  ${setupName('0.25.1')}\n`)
  const qa = { ...r.opts, from, whatsnew: path.join(r.dir, 'wn.ts'), date: '2026-01-02' }
  fs.writeFileSync(qa.whatsnew, WHATSNEW)
  signRelease({ ...qa, skipRebuild: true }, r.deps)
  assert.ok(!r.calls.some((c) => c[0] === 'rebuild'))
  signRelease(qa, r.deps)
  assert.equal(r.calls.filter((c) => c[0] === 'rebuild').length, 1)
})
