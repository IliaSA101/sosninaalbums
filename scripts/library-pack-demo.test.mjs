// Тесты примера темы в программе (спека 142 Т0): --zip-only, резерв id, страж ключей, снимки ключей.
// Отдельно от library-pack.test.mjs: тот у предела 900 строк теста. Запуск: node --test scripts/library-pack*.test.mjs
import assert from 'node:assert/strict'
import crypto from 'node:crypto'
import fs from 'node:fs'
import os from 'node:os'
import path from 'node:path'
import { test } from 'node:test'
import { spawnSync } from 'node:child_process'
import { fileURLToPath } from 'node:url'
import { MAX_ZIP, makeZip, readLimited, readZip } from './library-pack.mjs'
import { DEMO_IDS, DEMO_KEYS, checkDemoPack, itemKey, parseKeys, readPublished, snapshotKeys } from './library-pack-demo.mjs'

const ROOT = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..')
const SCRIPT = path.join(ROOT, 'scripts', 'library-pack.mjs')
const tmp = (p) => fs.mkdtempSync(path.join(os.tmpdir(), p))
const sha = (b) => crypto.createHash('sha256').update(b).digest('hex')

/** PNG-заглушка: сигнатура и IHDR (как в library-pack.test.mjs), случайный хвост — свой sha256 у каждой. */
function png(w = 64, h = 64) {
  const b = Buffer.alloc(700)
  crypto.randomBytes(600).copy(b, 33)
  Buffer.from([0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a]).copy(b)
  b.writeUInt32BE(13, 8)
  b.write('IHDR', 12, 'latin1')
  b.writeUInt32BE(w, 16)
  b.writeUInt32BE(h, 20)
  return b
}

const ALBUM = JSON.stringify({ v: 1, steps: [{ tpl: { ratio: 1 } }] })

/** Папка набора: фон, стикер, шаблон разворота и (album) макет — id любой, без -layouts. */
function packDir(id, { album = true, version = 1 } = {}) {
  const d = tmp('libdemo-')
  for (const [f, data] of [['bg/a.png', png()], ['stickers/s.png', png()], ['spreads/t.json', JSON.stringify({ v: 1, pages: 2, n: id })], ['looks/l.json', ALBUM]]) {
    fs.mkdirSync(path.dirname(path.join(d, f)), { recursive: true })
    fs.writeFileSync(path.join(d, f), data)
  }
  const items = [
    { file: 'stickers/s.png', kind: 'sticker', theme: 'wedding', name: 'Стикер' },
    { file: 'bg/a.png', kind: 'bg', theme: 'wedding', name: 'Фон' },
    { file: 'spreads/t.json', kind: 'spread', theme: 'wedding', name: 'Разворот' },
    ...(album ? [{ file: 'looks/l.json', kind: 'album', theme: 'wedding', name: 'Макет' }] : []),
  ]
  fs.writeFileSync(path.join(d, 'manifest.src.json'), JSON.stringify({ id, version, minApp: '0.29.0', title: 'Пример', items }))
  return d
}

const run = (...args) => {
  const r = spawnSync(process.execPath, [SCRIPT, ...args], { encoding: 'utf8' })
  return { code: r.status, text: r.stdout + r.stderr }
}
const zipOnly = (dir, out, ...more) => run('--zip-only', dir, '--out', out, ...more)

test('DEMO_IDS совпадает с ID в internal/demopack/demopack.go', () => {
  const f = path.join(ROOT, 'internal', 'demopack', 'demopack.go')
  const ids = [...fs.readFileSync(f, 'utf8').matchAll(/\bID\s*=\s*"([^"]+)"/g)].map((m) => m[1])
  assert.deepEqual(ids, DEMO_IDS)
})

test('--zip-only: zip без каталога, ключа и --seq; макет альбома в наборе с любым id; круг через readZip', () => {
  const out = path.join(tmp('libdemo-out-'), 'o')
  const r = zipOnly(packDir('some-demo-set'), out)
  assert.equal(r.code, 0, r.text)
  assert.deepEqual(fs.readdirSync(out), ['some-demo-set-1.zip']) // ни каталога, ни tmp
  const { manifest, files } = readPublished(path.join(out, 'some-demo-set-1.zip'), { readZip, readLimited, MAX_ZIP })
  assert.deepEqual(manifest.items.map((i) => i.kind), ['sticker', 'bg', 'spread', 'album'])
  assert.equal(manifest.items.length + 1, files.size) // предметы и manifest.json
  assert.match(r.text, new RegExp(`sha256 ${sha(fs.readFileSync(path.join(out, 'some-demo-set-1.zip')))}`))
  // без --zip-only те же файлы по-прежнему требуют id на -layouts
  const normal = run(packDir('some-demo-set'), '--new', '--key', path.join(tmp('libdemo-k-'), 'нет.pem'))
  assert.equal(normal.code, 2) // нет ключа: до сборки
  const e = zipOnly(packDir('some-demo-set'), out)
  assert.equal(e.code, 0) // повторная сборка перезаписывает тот же файл
})

test('--zip-only: чужие ключи, лишнее, нет --out и два режима — код 2 до записи; плохой набор — код 1', () => {
  const out = path.join(tmp('libdemo-out-'), 'o')
  const d = packDir('some-demo-set')
  for (const extra of [['--key', 'k.pem'], ['--catalog', 'c.json'], ['--seq', '3'], ['--new'], ['--prev', 'p.zip'], ['--drop', 'x'], ['--bogus'], ['лишнее'], ['--donor', 'z.zip'], ['--snapshot', 'z.zip']]) {
    assert.equal(zipOnly(d, out, ...extra).code, 2, extra.join(' '))
  }
  assert.equal(run('--zip-only', d).code, 2) // нет --out
  assert.equal(run('--zip-only', '--out', out).code, 2) // нет значения у режима
  assert.equal(run('--zip-only', d, '--out').code, 2)
  assert.equal(fs.existsSync(out), false)
  const bad = packDir('some-demo-set')
  fs.writeFileSync(path.join(bad, 'looks', 'l.json'), '{"v":2}')
  const r = zipOnly(bad, out)
  assert.equal(r.code, 1)
  assert.match(r.text, /v макета альбома/)
  assert.equal(fs.existsSync(out), false)
})

test('резерв id: набор с id примера в каталог не идёт, а frames-demo собирается', () => {
  assert.throws(() => checkDemoPack({ id: DEMO_IDS[0], items: [] }, ''), /занят примером/)
  const w = tmp('libdemo-cli-')
  const { privateKey } = crypto.generateKeyPairSync('ed25519')
  fs.writeFileSync(path.join(w, 'key.pem'), privateKey.export({ type: 'pkcs8', format: 'pem' }))
  const cli = (dir) => run(dir, '--new', '--key', path.join(w, 'key.pem'), '--out', path.join(w, 'out'))
  const reserved = cli(packDir(DEMO_IDS[0], { album: false }))
  assert.equal(reserved.code, 1)
  assert.match(reserved.text, /только через --zip-only/)
  assert.equal(fs.existsSync(path.join(w, 'out')), false)
  const ok = cli(packDir('wedding-navy-watercolor-frames-demo', { album: false }))
  assert.equal(ok.code, 0, ok.text)
  assert.ok(fs.existsSync(path.join(w, 'out', 'wedding-navy-watercolor-frames-demo-1.zip')))
  assert.equal(zipOnly(packDir(DEMO_IDS[0]), path.join(w, 'z')).code, 0) // а --zip-only id примера собирает
})

const K = (n) => `my:${n.toString(16).padStart(16, '0')}`
const item = (kind, n, file = `f${n}`) => ({ kind, file, sha256: K(n).slice(3).padEnd(64, '0') })

test('страж ключей: у донора каждая строка demo-v1.keys остаётся (по ключу; шаблон и макет — по пути), у прочих наборов не трогает', () => {
  const keys = ['donor-set bg ' + K(1), 'donor-set spread ' + K(2) + ' spreads/t.json', 'donor-set album ' + K(3) + ' looks/l.json', 'other-set sticker ' + K(4)].join('\n') + '\n'
  const full = { id: 'donor-set', items: [item('bg', 1), item('spread', 99, 'spreads/t.json'), item('album', 98, 'looks/l.json')] }
  checkDemoPack(full, keys) // шаблон и макет переписаны (другие байты), путь на месте — можно
  assert.doesNotThrow(() => checkDemoPack({ id: 'free-set', items: [] }, keys))
  assert.throws(() => checkDemoPack({ ...full, items: full.items.slice(1) }, keys), new RegExp(`ключ примера в программе пропал: bg ${K(1)}$`))
  assert.throws(() => checkDemoPack({ ...full, items: [full.items[0], item('spread', 2, 'spreads/other.json'), full.items[2]] }, keys), /пропал: spread my:\d+ spreads\/t\.json/)
  assert.throws(() => checkDemoPack({ ...full, items: [item('sticker', 1), ...full.items.slice(1)] }, keys), /пропал: bg/) // тот же ключ, другой вид
  assert.doesNotThrow(() => checkDemoPack({ id: 'donor-set', items: [] }, '')) // пустой файл — строк нет
})

test('страж ключей в CLI: набор-донор без своей строки — отказ до записи', () => {
  const keys = fs.readFileSync(path.join(ROOT, ...DEMO_KEYS.split('/')), 'utf8')
  const donor = parseKeys(keys, true)[0].donor
  const w = tmp('libdemo-cli-')
  const { privateKey } = crypto.generateKeyPairSync('ed25519')
  fs.writeFileSync(path.join(w, 'key.pem'), privateKey.export({ type: 'pkcs8', format: 'pem' }))
  const r = run(packDir(donor, { album: false }), '--new', '--key', path.join(w, 'key.pem'), '--out', path.join(w, 'out'))
  assert.equal(r.code, 1)
  assert.match(r.text, /ключ примера в программе пропал: /)
  assert.equal(fs.existsSync(path.join(w, 'out')), false)
})

test('parseKeys: круг со снимком; пустое, битое, повтор, лишнее поле и случайный мусор — исключение или пусто, не паника', () => {
  const snap = snapshotKeys({ items: [item('bg', 1), item('spread', 2, 'spreads/a.json'), item('album', 3, 'looks/l.json')] })
  assert.deepEqual(parseKeys(snap).map((e) => [e.kind, e.key, e.path]), [['bg', K(1), undefined], ['spread', K(2), 'spreads/a.json'], ['album', K(3), 'looks/l.json']])
  assert.deepEqual(parseKeys(''), [])
  assert.deepEqual(parseKeys('\n\n'), [])
  assert.equal(parseKeys('donor-set bg ' + K(1) + '\r\n', true).length, 1) // CRLF на Windows-клоне
  for (const bad of ['bg', 'bg my:xx', 'foo ' + K(1), 'bg ' + K(1) + ' extra', 'spread ' + K(1), 'spread ' + K(1) + ' ../a.json', 'spread ' + K(1) + ' a b', 'bg ' + K(1) + '\nbg ' + K(1), 'bg  ' + K(1), 'bg ' + K(1) + ' ']) {
    assert.throws(() => parseKeys(bad), /ключи, строка/, JSON.stringify(bad))
  }
  assert.throws(() => parseKeys('bg ' + K(1), true), /нет id донора/)
  assert.throws(() => parseKeys('-x bg ' + K(1), true), /нет id донора/)
  for (let i = 0; i < 300; i++) {
    const junk = crypto.randomBytes(1 + (i % 40)).toString('latin1').replace(/[\x00-\x08]/g, ' ')
    try {
      parseKeys(junk, i % 2 === 0)
    } catch (e) {
      assert.match(e.message, /ключи, строка/)
    }
  }
})

test('снимок и ключи примера: круг через zip; предмет не из доноров, битый, пустой, усечённый и огромный zip — ошибка', () => {
  const lib = { readZip, readLimited, MAX_ZIP }
  const w = tmp('libdemo-snap-')
  const mk = (name, id, items) => {
    const files = items.map((i) => ({ name: i.file, data: i.data }))
    const manifest = { format: 1, id, version: 1, items: items.map((i) => ({ file: i.file, kind: i.kind, sha256: sha(i.data), size: i.data.length })) }
    fs.writeFileSync(path.join(w, name), makeZip([{ name: 'manifest.json', data: Buffer.from(JSON.stringify(manifest)) }, ...files]))
    return path.join(w, name)
  }
  const bg = { file: 'bg/a.png', kind: 'bg', data: png() }
  const sp = { file: 'spreads/t.json', kind: 'spread', data: Buffer.from('{"v":1}') }
  const donor = mk('donor.zip', 'donor-set', [bg, sp])
  const extra = mk('extra.zip', 'extra-set', [{ file: 'stickers/s.png', kind: 'sticker', data: png() }])
  const demo = mk('demo.zip', 'demo-set', [bg, sp])
  const out = path.join(w, 'out')
  assert.equal(run('--snapshot', donor, '--out', out).code, 0)
  assert.equal(fs.readFileSync(path.join(out, 'donor-set-v1.keys'), 'utf8'), `bg ${itemKey(sha(bg.data))}\nspread ${itemKey(sha(sp.data))} spreads/t.json\n`)
  const ok = run('--demo-keys', demo, '--donor', donor, '--donor', extra, '--out', out)
  assert.equal(ok.code, 0, ok.text)
  assert.deepEqual(parseKeys(fs.readFileSync(path.join(out, 'demo-v1.keys'), 'utf8'), true).map((e) => e.donor), ['donor-set', 'donor-set'])
  const lost = run('--demo-keys', demo, '--donor', extra, '--out', out)
  assert.equal(lost.code, 1)
  assert.match(lost.text, /не найден ни в одном доноре/)
  assert.equal(run('--demo-keys', demo, '--out', out).code, 2) // доноров нет
  assert.equal(run('--snapshot', donor, '--donor', extra, '--out', out).code, 2)

  const wrongSha = path.join(w, 'wrong.zip') // манифест врёт про sha256 предмета
  const m = { format: 1, id: 'wrong-set', version: 1, items: [{ file: 'bg/a.png', kind: 'bg', sha256: '0'.repeat(64), size: bg.data.length }] }
  fs.writeFileSync(wrongSha, makeZip([{ name: 'manifest.json', data: Buffer.from(JSON.stringify(m)) }, { name: 'bg/a.png', data: bg.data }]))
  fs.writeFileSync(path.join(w, 'empty.zip'), '')
  fs.writeFileSync(path.join(w, 'cut.zip'), fs.readFileSync(donor).subarray(0, 200))
  fs.writeFileSync(path.join(w, 'nomanifest.zip'), makeZip([{ name: 'x.txt', data: Buffer.from('x') }]))
  fs.writeFileSync(path.join(w, 'huge.zip'), '')
  fs.truncateSync(path.join(w, 'huge.zip'), MAX_ZIP + 1) // разреженный файл: в память не читается
  for (const name of ['wrong.zip', 'empty.zip', 'cut.zip', 'nomanifest.zip', 'huge.zip', 'нет-такого.zip']) {
    const r = run('--snapshot', path.join(w, name), '--out', path.join(w, 'bad'))
    assert.equal(r.code, 1, name)
    assert.doesNotMatch(r.text, /at .*\.mjs:\d+/, `${name}: стек вместо сообщения`)
  }
  assert.equal(fs.existsSync(path.join(w, 'bad')), false)
  assert.throws(() => readPublished(path.join(w, 'huge.zip'), lib), /больше предела/)
})

test('данные примера в репозитории: 18 предметов в порядке design §1, бюджет, ключи согласованы с zip и снимками доноров', () => {
  const dir = path.join(ROOT, 'internal', 'demopack')
  const zipFile = path.join(dir, `${DEMO_IDS[0]}-1.zip`)
  assert.ok(fs.statSync(zipFile).size <= 20 << 20, 'бюджет pre-commit')
  const { manifest, files } = readPublished(zipFile, { readZip, readLimited, MAX_ZIP })
  assert.deepEqual([manifest.id, manifest.version, manifest.minApp, manifest.license], [DEMO_IDS[0], 1, '0.29.0', 'CC0-1.0'])
  assert.equal(manifest.title, 'Свадьба — синяя акварель (пример)')
  assert.match(manifest.about, /^Часть набора «Свадьба — синяя акварель» с рамками и макетом альбома\. Пример уйдёт сам, когда встанет весь набор$/)
  assert.ok(files.has('LICENSE.txt'))
  const count = (k) => manifest.items.filter((i) => i.kind === k).length
  assert.deepEqual([count('bg'), count('sticker'), count('spread'), count('frame'), count('album')], [4, 7, 4, 2, 1])
  const order = manifest.items.map((i) => i.kind)
  assert.deepEqual(order, ['sticker', 'sticker', 'sticker', 'sticker', 'sticker', 'sticker', 'sticker', 'frame', 'frame', 'bg', 'bg', 'bg', 'bg', 'spread', 'spread', 'spread', 'spread', 'album'])
  assert.equal(manifest.items.at(-1).file, 'looks/wedding-navy-watercolor.json') // путь, как в -layouts: на нём псевдоним макета

  const rows = parseKeys(fs.readFileSync(path.join(dir, 'testdata', 'demo-v1.keys'), 'utf8'), true)
  assert.deepEqual(rows.map((r) => [r.kind, r.key, r.path]), parseKeys(snapshotKeys(manifest)).map((r) => [r.kind, r.key, r.path]))
  assert.deepEqual([...new Set(rows.map((r) => r.donor))].sort(), ['wedding-navy-watercolor', 'wedding-navy-watercolor-frames', 'wedding-navy-watercolor-layouts'])
  for (const r of rows) {
    const snap = parseKeys(fs.readFileSync(path.join(dir, 'testdata', `${r.donor}-v1.keys`), 'utf8'))
    assert.ok(snap.some((s) => s.kind === r.kind && s.key === r.key && s.path === r.path), `${r.donor} ${r.kind} ${r.key}`)
  }
  // макет ссылается только на предметы примера: плитки без дыр
  const have = new Set(rows.map((r) => r.key))
  const refs = [...files.get('looks/wedding-navy-watercolor.json').toString('utf8').matchAll(/my:[0-9a-f]{16}/g)].map((m) => m[0])
  assert.equal(new Set(refs).size, 13)
  for (const k of refs) assert.ok(have.has(k), `макет ссылается на ${k}, которого нет в примере`)
})
