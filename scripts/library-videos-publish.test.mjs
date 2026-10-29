// Тесты публикации роликов F1 (140, Т6): разбор run.json, проверка файлов, слияние и подпись каталога.
import assert from 'node:assert/strict'
import crypto from 'node:crypto'
import fs from 'node:fs'
import os from 'node:os'
import path from 'node:path'
import { after, test } from 'node:test'
import { publicRaw, readCatalogFile, sealCatalog, verifyBytes } from './library-pack.mjs'
import { MAX_VIDEO_FILE, URL_PREFIX, checkRow, main, mergeVideos, readSignedCatalog, rowsFromRun } from './library-videos-publish.mjs'

const sha = (b) => crypto.createHash('sha256').update(b).digest('hex')

const tmpDirs = []
after(() => tmpDirs.forEach((d) => fs.rmSync(d, { recursive: true, force: true }))) // не копить vpub-* в /tmp

/** Папка с файлами роликов и run.json по списку {id, version, minApp, salt}; salt меняет содержимое файлов. */
function env(list) {
  const d = fs.mkdtempSync(path.join(os.tmpdir(), 'vpub-'))
  tmpDirs.push(d)
  const ok = {}
  for (const { id, version = 1, seconds = 25, minApp = '0.30.0', salt = '' } of list) {
    const files = {}
    for (const kind of ['webm', 'mp4']) {
      const data = Buffer.from(`${id}|${version}|${kind}|${salt}`)
      fs.writeFileSync(path.join(d, `${id}-${version}.${kind}`), data)
      files[kind] = { size: data.length, sha256: sha(data), url: `${URL_PREFIX}${id}-${version}.${kind}` }
    }
    ok[id] = { id, encoded: { video: { id, version, minApp, seconds, files } } }
  }
  const run = path.join(d, 'run.json')
  fs.writeFileSync(run, JSON.stringify({ ok, failed: {} }))
  return { d, run, ok }
}

const key = () => crypto.generateKeyPairSync('ed25519').privateKey

test('run.json: строки берутся у всех закодированных статей; пустой, битый и огромный файл — ошибка', () => {
  const e = env([{ id: 'autobuild' }, { id: 'cover' }])
  assert.deepEqual(rowsFromRun(e.run).map((r) => r.id), ['autobuild', 'cover'])
  const bad = path.join(e.d, 'bad.json')
  fs.writeFileSync(bad, '{"ok": ')
  assert.throws(() => rowsFromRun(bad), /не JSON/)
  fs.writeFileSync(bad, '')
  assert.throws(() => rowsFromRun(bad), /не JSON/)
  fs.writeFileSync(bad, '{"ok":{}}')
  assert.throws(() => rowsFromRun(bad), /нет ни одного/)
  fs.writeFileSync(bad, '[]')
  assert.throws(() => rowsFromRun(bad), /нет поля ok/)
  fs.writeFileSync(bad, ' '.repeat((1 << 20) + 1))
  assert.throws(() => rowsFromRun(bad), /больше/)
  assert.throws(() => rowsFromRun(e.d), /не файл/)
})

test('checkRow: годная строка чиста; чужой адрес, подмена файла, размер, id, версия, длина, нет файла — отказ', () => {
  const e = env([{ id: 'export' }])
  const row = e.ok.export.encoded.video
  assert.deepEqual(checkRow(row, e.d), [])
  const clone = () => JSON.parse(JSON.stringify(row))
  const url = clone()
  url.files.webm.url = 'https://example.com/export-1.webm'
  assert.match(checkRow(url, e.d).join(), /адрес/)
  fs.writeFileSync(path.join(e.d, 'export-1.mp4'), 'подмена!!')
  assert.match(checkRow(row, e.d).join(), /export-1\.mp4/)
  fs.rmSync(path.join(e.d, 'export-1.webm'))
  assert.match(checkRow(row, e.d).join(), /нет файла/)
  const tooBig = clone()
  tooBig.files.mp4.size = MAX_VIDEO_FILE + 1
  assert.match(checkRow(tooBig, e.d).join(), /export-1\.mp4/)
  for (const [patch, rx] of [
    [{ id: '../x' }, /id/],
    [{ id: 'ab' }, /id/],
    [{ version: 0 }, /версия/],
    [{ version: 1000000 }, /версия/],
    [{ seconds: 0 }, /длина/],
    [{ seconds: 601 }, /длина/],
    [{ minApp: '0.30' }, /minApp/],
    [{ files: {} }, /нет файлов/],
  ]) {
    assert.match(checkRow({ ...clone(), ...patch }, e.d).join(), rx)
  }
  assert.match(checkRow(null, e.d).join(), /id/)
})

test('слияние: свои id заменяются, прочие videos и наборы остаются, seq + 1; версия ниже и seq не выше — отказ', () => {
  const e = env([{ id: 'autobuild' }, { id: 'cover', version: 2 }])
  const rows = rowsFromRun(e.run)
  const cat = { format: 1, seq: 7, packs: [{ id: 'p1' }], videos: [{ id: 'filter', version: 1, minApp: '0.30.0' }, { id: 'cover', version: 1, minApp: '0.30.0' }] }
  const next = mergeVideos(cat, rows)
  assert.equal(next.seq, 8)
  assert.deepEqual(next.packs, cat.packs)
  assert.deepEqual(next.videos.map((v) => `${v.id}/${v.version}`).sort(), ['autobuild/1', 'cover/2', 'filter/1'])
  assert.equal(mergeVideos({ format: 1, seq: 3, packs: [] }, rows, 10).seq, 10) // каталог без videos
  assert.throws(() => mergeVideos(cat, rows, 7), /не больше прежнего/)
  assert.throws(() => mergeVideos({ ...cat, videos: [{ id: 'cover', version: 3, minApp: '0.30.0' }] }, rows), /ниже стоящей/)
  const many = { format: 1, seq: 1, videos: Array.from({ length: 100 }, (_, i) => ({ id: `v${i}-aa`, version: 1 })) }
  assert.throws(() => mergeVideos(many, rows), /предел/)
})

test('слияние: та же версия с тем же содержимым — без изменений, с другим sha256, размером, minApp или длиной — отказ', () => {
  const a = rowsFromRun(env([{ id: 'cover' }]).run)
  const cat = { format: 1, seq: 4, videos: a }
  const same = mergeVideos(cat, JSON.parse(JSON.stringify(a)))
  assert.deepEqual(same.videos, a)
  assert.equal(same.seq, 5)
  const other = rowsFromRun(env([{ id: 'cover', salt: 'пересъёмка' }]).run)
  assert.throws(() => mergeVideos(cat, other), /другим содержимым.*поднимите версию/)
  const size = JSON.parse(JSON.stringify(a))
  size[0].files.mp4.size += 1
  assert.throws(() => mergeVideos(cat, size), /другим содержимым/)
  assert.throws(() => mergeVideos(cat, [{ ...a[0], minApp: '0.31.0' }]), /другим содержимым/)
  assert.throws(() => mergeVideos(cat, [{ ...a[0], seconds: 40 }]), /другим содержимым/)
  // порядок полей в JSON каталога не считается различием
  const reordered = { files: a[0].files, seconds: a[0].seconds, minApp: a[0].minApp, version: a[0].version, id: a[0].id }
  assert.deepEqual(mergeVideos({ format: 1, seq: 1, videos: [reordered] }, a).videos, [reordered])
})

test('слияние: строка для старой программы (меньший minApp) остаётся, затенённая новой — убирается, как в pick.go', () => {
  const one = (version, minApp) => rowsFromRun(env([{ id: 'cover', version, minApp }]).run)[0]
  const v1 = one(1, '0.30.0')
  const v2 = one(2, '0.32.0')
  const both = mergeVideos({ format: 1, seq: 1, videos: [v1] }, [v2])
  assert.deepEqual(both.videos.map((v) => `${v.version}/${v.minApp}`), ['1/0.30.0', '2/0.32.0'])
  // третья версия с тем же minApp, что у второй, затеняет вторую; первая (0.30.0) остаётся
  assert.deepEqual(mergeVideos(both, [one(3, '0.32.0')]).videos.map((v) => v.version), [1, 3])
  // с minApp ниже первой затеняет обе
  assert.deepEqual(mergeVideos(both, [one(4, '0.29.0')]).videos.map((v) => v.version), [4])
  // сравнение по числам, а не по строке: 0.10.0 ниже 0.30.0, 0.9.0 тоже
  assert.deepEqual(mergeVideos({ format: 1, seq: 1, videos: [v1] }, [one(5, '0.10.0')]).videos.map((v) => v.version), [5])
  assert.deepEqual(mergeVideos({ format: 1, seq: 1, videos: [v1] }, [one(6, '0.9.0')]).videos.map((v) => v.version), [6])
  // версия ниже стоящей по-прежнему отказ, даже при большем minApp
  assert.throws(() => mergeVideos({ format: 1, seq: 1, videos: [v2] }, [{ ...v1, minApp: '0.40.0' }]), /ниже стоящей/)
})

test('checkRow: нужны оба вида — без mp4 или без webm отказ (Mac играет mp4)', () => {
  const e = env([{ id: 'export' }])
  const row = e.ok.export.encoded.video
  for (const gone of ['mp4', 'webm']) {
    const r = JSON.parse(JSON.stringify(row))
    delete r.files[gone]
    assert.match(checkRow(r, e.d).join(), new RegExp(`нет вида ${gone}`))
  }
})

test('CLI: каталог, подписанный другим (тестовым) ключом, не принимается до слияния; битый конверт — ошибка, не паника', () => {
  const e = env([{ id: 'autobuild' }])
  const prod = key()
  const prodFile = path.join(e.d, 'prod.pem')
  fs.writeFileSync(prodFile, prod.export({ type: 'pkcs8', format: 'pem' }))
  const catFile = path.join(e.d, 'catalog.json')
  fs.writeFileSync(catFile, JSON.stringify(sealCatalog({ format: 1, seq: 14 }, key())) + '\n')
  const out = path.join(e.d, 'new.json')
  const log = console.log
  console.log = () => {}
  try {
    assert.throws(() => main(['--run', e.run, '--dir', e.d, '--catalog', catFile, '--key', prodFile, '--out', out]), /подпись прежнего каталога/)
    assert.equal(fs.existsSync(out), false)
    assert.throws(() => readSignedCatalog(path.join(e.d, 'нет.json'), prod), /нет файла/)
    for (const junk of ['', '{', '[]', '{"body":1,"sig":2}', '{"body":"!!","sig":"!!"}']) {
      fs.writeFileSync(catFile, junk)
      assert.throws(() => readSignedCatalog(catFile, prod), /не каталог|подпись/, junk)
    }
  } finally {
    console.log = log
  }
})

test('CLI: проверка без ключа ничего не пишет; с ключом — подписанный каталог с теми же наборами и роликами', () => {
  const e = env([{ id: 'autobuild' }, { id: 'looks' }, { id: 'export' }])
  const k = key()
  const keyFile = path.join(e.d, 'k.pem')
  fs.writeFileSync(keyFile, k.export({ type: 'pkcs8', format: 'pem' }))
  const catFile = path.join(e.d, 'catalog.json')
  fs.writeFileSync(catFile, JSON.stringify(sealCatalog({ format: 1, seq: 14, packs: [{ id: 'p1' }] }, k)) + '\n')
  const log = console.log
  console.log = () => {}
  try {
    main(['--run', e.run, '--dir', e.d])
    const out = path.join(e.d, 'new.json')
    assert.equal(fs.existsSync(out), false)
    main(['--run', e.run, '--dir', e.d, '--catalog', catFile, '--key', keyFile, '--out', out])
    const env2 = JSON.parse(fs.readFileSync(out, 'utf8'))
    assert.ok(verifyBytes(Buffer.from(env2.body, 'base64'), env2.sig, publicRaw(k)))
    const cat = readCatalogFile(out)
    assert.equal(cat.seq, 15)
    assert.equal(cat.packs.length, 1)
    assert.deepEqual(cat.videos.map((v) => v.id), ['autobuild', 'looks', 'export'])
    assert.throws(() => main(['--run', e.run, '--dir', e.d, '--key', keyFile]), /вместе/)
    fs.writeFileSync(path.join(e.d, 'looks-1.webm'), 'x')
    assert.throws(() => main(['--run', e.run, '--dir', e.d]), /проверка не прошла/)
    assert.throws(() => main(['--run', e.run]), /использование/)
  } finally {
    console.log = log
  }
})
