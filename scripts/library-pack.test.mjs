// Тесты выпуска набора библиотеки (спека 067 D1): zip, sha256, подпись каталога.
import assert from 'node:assert/strict'
import crypto from 'node:crypto'
import fs from 'node:fs'
import os from 'node:os'
import path from 'node:path'
import { test } from 'node:test'
import { buildPack, catalogRow, makeZip, publicRaw, readCatalogFile, readZip, requireBase, sealCatalog, signBytes, upsertCatalog, verifyBytes } from './library-pack.mjs'

function setDir() {
  const d = fs.mkdtempSync(path.join(os.tmpdir(), 'libpack-'))
  fs.mkdirSync(path.join(d, 'bg'))
  fs.mkdirSync(path.join(d, 'spreads'))
  fs.writeFileSync(path.join(d, 'bg', 'one.png'), crypto.randomBytes(500))
  fs.writeFileSync(path.join(d, 'spreads', 'hero.json'), JSON.stringify({ v: 1, pages: 2 }))
  fs.writeFileSync(
    path.join(d, 'manifest.src.json'),
    JSON.stringify({
      id: 'test-set',
      version: 2,
      minApp: '0.16.0',
      title: 'Тест',
      license: 'CC0-1.0',
      items: [
        { file: 'bg/one.png', kind: 'bg', theme: 'wedding', name: 'Один' },
        { file: 'spreads/hero.json', kind: 'spread', name: 'Главное' },
      ],
    }),
  )
  return d
}

test('zip читается, sha256 и размеры совпадают с манифестом', () => {
  const { manifest, files } = buildPack(setDir())
  const zip = readZip(makeZip(files))
  const m = JSON.parse(zip.get('manifest.json').toString())
  assert.deepEqual(m, manifest)
  for (const it of m.items) {
    const data = zip.get(it.file)
    assert.equal(data.length, it.size)
    assert.equal(crypto.createHash('sha256').update(data).digest('hex'), it.sha256)
  }
  assert.equal(m.items[1].theme, 'basic')
})

test('плохие имена и повторы отвергаются', () => {
  const d = setDir()
  const p = path.join(d, 'manifest.src.json')
  const src = JSON.parse(fs.readFileSync(p, 'utf8'))
  src.items[1].file = '../x.json'
  fs.writeFileSync(p, JSON.stringify(src))
  assert.throws(() => buildPack(d))
  src.items[1].file = 'BG/One.png'
  fs.writeFileSync(p, JSON.stringify(src))
  assert.throws(() => buildPack(d))
})

test('каталог подписан, порча байта ломает подпись', () => {
  const { manifest, files } = buildPack(setDir())
  const zip = makeZip(files)
  const cat = upsertCatalog(null, catalogRow(manifest, zip, 'test-set-2.zip'))
  assert.equal(cat.packs[0].counts.bg, 1)
  assert.equal(cat.packs[0].size, zip.length)
  assert.ok(cat.packs[0].url.startsWith('https://github.com/IliaSA101/sosninaalbums-library/releases/download/'))
  const data = Buffer.from(JSON.stringify(cat, null, 2) + '\n')
  const { privateKey } = crypto.generateKeyPairSync('ed25519')
  const pub = publicRaw(privateKey)
  assert.equal(Buffer.from(pub, 'base64').length, 32)
  const sig = signBytes(data, privateKey)
  assert.equal(verifyBytes(data, sig, pub), true)
  const bad = Buffer.from(data)
  bad[10] ^= 1
  assert.equal(verifyBytes(bad, sig, pub), false)
  const other = publicRaw(crypto.generateKeyPairSync('ed25519').privateKey)
  assert.equal(verifyBytes(data, sig, other), false)
})

test('конверт каталога: подпись по байтам body, seq растёт', () => {
  const { manifest, files } = buildPack(setDir())
  const row = catalogRow(manifest, makeZip(files), 'a.zip')
  const cat = upsertCatalog(null, row)
  assert.equal(cat.seq, 1)
  assert.equal(upsertCatalog(cat, row).seq, 2)
  assert.equal(upsertCatalog(cat, row, 7).seq, 7)
  assert.throws(() => upsertCatalog(null, row, 0))
  const { privateKey } = crypto.generateKeyPairSync('ed25519')
  const env = sealCatalog(cat, privateKey)
  assert.deepEqual(Object.keys(env).sort(), ['body', 'sig'])
  const body = Buffer.from(env.body, 'base64')
  assert.equal(verifyBytes(body, env.sig, publicRaw(privateKey)), true)
  assert.deepEqual(JSON.parse(body.toString()), cat)
  const f = path.join(os.tmpdir(), `lib-cat-${process.pid}.json`)
  fs.writeFileSync(f, JSON.stringify(env))
  assert.equal(readCatalogFile(f).seq, 1)
  fs.rmSync(f)
})

test('каталог: замена по id, старее — отказ', () => {
  const { manifest, files } = buildPack(setDir())
  const row = catalogRow(manifest, makeZip(files), 'a.zip')
  const c1 = upsertCatalog(null, row)
  assert.equal(upsertCatalog(c1, { ...row, version: 3 }).packs.length, 1)
  assert.throws(() => upsertCatalog(c1, { ...row, version: 1 }))
})

test('--seq: не больше seq прежнего каталога — отказ; больше — принят', () => {
  const { manifest, files } = buildPack(setDir())
  const row = catalogRow(manifest, makeZip(files), 'a.zip')
  const c5 = upsertCatalog(null, row, 5)
  assert.throws(() => upsertCatalog(c5, row, 5), /больше seq прежнего/)
  assert.throws(() => upsertCatalog(c5, row, 3), /больше seq прежнего/)
  assert.equal(upsertCatalog(c5, row, 6).seq, 6)
  assert.equal(upsertCatalog(c5, row).seq, 6)
  assert.equal(upsertCatalog(null, row, 1).seq, 1) // нового каталога прежнего seq нет
})

test('нет файла каталога: отказ без --new; с --new — новый; --new при готовом каталоге — отказ', () => {
  const { manifest, files } = buildPack(setDir())
  const cat = upsertCatalog(null, catalogRow(manifest, makeZip(files), 'a.zip'))
  assert.throws(() => requireBase(null, false), /--new/)
  assert.equal(requireBase(null, true), null)
  assert.equal(requireBase(cat, false), cat)
  assert.throws(() => requireBase(cat, true), /уже есть/)
})

/** PNG-заглушка: сигнатура и IHDR нужного размера (остальное для проверки заголовка не нужно). */
function pngStub(w, h) {
  const b = Buffer.alloc(700)
  Buffer.from([0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a]).copy(b)
  b.writeUInt32BE(13, 8)
  b.write('IHDR', 12, 'latin1')
  b.writeUInt32BE(w, 16)
  b.writeUInt32BE(h, 20)
  return b
}

test('рамочка (079): kind frame — PNG в пределах, counts.frame; не PNG и больше предела отвергаются', () => {
  const d = setDir()
  fs.mkdirSync(path.join(d, 'frames'))
  fs.writeFileSync(path.join(d, 'frames', 'ring.png'), pngStub(120, 160))
  const p = path.join(d, 'manifest.src.json')
  const src = JSON.parse(fs.readFileSync(p, 'utf8'))
  src.minApp = '0.20.0'
  src.items.push({ file: 'frames/ring.png', kind: 'frame', theme: 'wedding', name: 'Кольцо' })
  fs.writeFileSync(p, JSON.stringify(src))
  const { manifest } = buildPack(d)
  assert.equal(manifest.items[2].kind, 'frame')
  assert.deepEqual(catalogRow(manifest, Buffer.from('zip'), 'v1').counts, { bg: 1, sticker: 0, spread: 1, frame: 1, print: 0 })
  fs.writeFileSync(path.join(d, 'frames', 'ring.jpg'), crypto.randomBytes(700))
  src.items[2].file = 'frames/ring.jpg'
  fs.writeFileSync(p, JSON.stringify(src))
  assert.throws(() => buildPack(d), /только PNG/)
  fs.writeFileSync(path.join(d, 'frames', 'big.png'), Buffer.alloc((30 << 20) + 1))
  src.items[2].file = 'frames/big.png'
  fs.writeFileSync(p, JSON.stringify(src))
  assert.throws(() => buildPack(d), /размер вне предела/)
})

test('рамочка (079-35): minApp ниже 0.20.0 и сторона PNG больше 4096 или не PNG — ошибка сборки', () => {
  const d = setDir()
  fs.mkdirSync(path.join(d, 'frames'))
  const p = path.join(d, 'manifest.src.json')
  const src = JSON.parse(fs.readFileSync(p, 'utf8'))
  src.items.push({ file: 'frames/ring.png', kind: 'frame', theme: 'wedding', name: 'Кольцо' })
  const put = (minApp, png) => {
    src.minApp = minApp
    fs.writeFileSync(path.join(d, 'frames', 'ring.png'), png)
    fs.writeFileSync(p, JSON.stringify(src))
  }
  put('0.19.9', pngStub(100, 100))
  assert.throws(() => buildPack(d), /minApp не ниже 0\.20\.0/)
  put('0.9.0', pngStub(100, 100)) // сравнение числовое, не строковое
  assert.throws(() => buildPack(d), /minApp не ниже/)
  put('0.20.0', pngStub(100, 100))
  assert.equal(buildPack(d).manifest.items[2].kind, 'frame')
  put('1.0.0', pngStub(4096, 1)) // предел включительно
  assert.equal(buildPack(d).manifest.items[2].kind, 'frame')
  put('0.20.0', pngStub(4097, 100))
  assert.throws(() => buildPack(d), /сторона PNG 4097×100/)
  put('0.20.0', pngStub(100, 4097))
  assert.throws(() => buildPack(d), /сторона PNG/)
  put('0.20.0', pngStub(0, 100))
  assert.throws(() => buildPack(d), /сторона PNG/)
  put('0.20.0', crypto.randomBytes(700)) // .png без сигнатуры
  assert.throws(() => buildPack(d), /нет PNG-заголовка/)
  // набор без рамочек старого minApp по-прежнему собирается
  const d2 = setDir()
  assert.equal(buildPack(d2).manifest.minApp, '0.16.0')
})

test('типография (123): kind print — .saprint, JSON-объект ≤ 64 КБ, minApp не ниже 0.26.0, counts.print', () => {
  const d = setDir()
  fs.mkdirSync(path.join(d, 'prints'))
  const p = path.join(d, 'manifest.src.json')
  const src = JSON.parse(fs.readFileSync(p, 'utf8'))
  src.items.push({ file: 'prints/fabrika.saprint', kind: 'print', name: 'Фабрика' })
  const put = (minApp, data, file = 'prints/fabrika.saprint') => {
    src.minApp = minApp
    src.items[2].file = file
    fs.writeFileSync(path.join(d, ...file.split('/')), data)
    fs.writeFileSync(p, JSON.stringify(src))
  }
  const profile = (extra = {}) => JSON.stringify({ format: 1, id: 'fabrika', version: 2, name: 'Фабрика Фотокниги', ...extra })

  put('0.26.0', profile())
  const { manifest, files } = buildPack(d)
  assert.equal(manifest.items[2].kind, 'print')
  assert.deepEqual(catalogRow(manifest, Buffer.from('zip'), 'v1').counts, { bg: 1, sticker: 0, spread: 1, frame: 0, print: 1 })
  assert.equal(readZip(makeZip(files)).get('prints/fabrika.saprint').toString(), profile()) // круг через zip

  put('0.25.1', profile()) // минимум по версии, числовое сравнение
  assert.throws(() => buildPack(d), /minApp не ниже 0\.26\.0/)
  put('0.9.0', profile())
  assert.throws(() => buildPack(d), /minApp не ниже/)
  put('1.0.0', profile()) // выше минимума — можно
  assert.equal(buildPack(d).manifest.items[2].kind, 'print')

  put('0.26.0', profile(), 'prints/fabrika.json')
  assert.throws(() => buildPack(d), /только \.saprint/)
  put('0.26.0', '{не json')
  assert.throws(() => buildPack(d), /не JSON/)
  put('0.26.0', '')
  assert.throws(() => buildPack(d), /размер вне предела/)
  for (const bad of ['[]', 'null', '"x"', '42']) {
    put('0.26.0', bad)
    assert.throws(() => buildPack(d), /не JSON-объект/, bad)
  }
  put('0.26.0', profile({ format: 2 }))
  assert.throws(() => buildPack(d), /format профиля/)
  put('0.26.0', profile({ id: 'Fab' }))
  assert.throws(() => buildPack(d), /id профиля/)
  put('0.26.0', profile({ name: '  ' }))
  assert.throws(() => buildPack(d), /нет name/)
  put('0.26.0', profile({ pad: 'x'.repeat(64 << 10) })) // за пределом 64 КБ
  assert.throws(() => buildPack(d), /размер вне предела/)
  const head = Buffer.from(profile({ pad: '' }))
  const fill = (64 << 10) - head.length // с этим добавком файл ровно 64 КБ
  put('0.26.0', profile({ pad: 'x'.repeat(fill) }))
  assert.equal(fs.statSync(path.join(d, 'prints', 'fabrika.saprint')).size, 64 << 10)
  assert.equal(buildPack(d).manifest.items[2].size, 64 << 10) // предел включительно
})
