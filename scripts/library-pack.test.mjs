// Тесты выпуска набора библиотеки (спека 067 D1): zip, sha256, подпись каталога.
import assert from 'node:assert/strict'
import crypto from 'node:crypto'
import fs from 'node:fs'
import os from 'node:os'
import path from 'node:path'
import { test } from 'node:test'
import { spawnSync } from 'node:child_process'
import { fileURLToPath } from 'node:url'
import { MAX_ZIP, MAX_ZIP_WAVE2, zipLimit, assertZipSize, buildPack, checkAlbumFile, catalogRow, checkSpreadFile, checkPrev, checkPublish, imageInfo, jpegSize, makeZip, orderWarnings, publicRaw, readCatalogFile, readPrev, readZip, requireBase, sealCatalog, signBytes, upsertCatalog, verifyBytes } from './library-pack.mjs'

function setDir() {
  const d = fs.mkdtempSync(path.join(os.tmpdir(), 'libpack-'))
  fs.mkdirSync(path.join(d, 'bg'))
  fs.mkdirSync(path.join(d, 'spreads'))
  fs.writeFileSync(path.join(d, 'bg', 'one.png'), pngStub(64, 64, true))
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

test('каталог: публикация набора сохраняет videos (ролики F1) и подпись сходится, круг файл → upsert → файл', () => {
  const { manifest, files } = buildPack(setDir())
  const row = catalogRow(manifest, makeZip(files), 'a.zip')
  const videos = [
    { id: 'filter', version: 1, files: { webm: { url: 'https://x/a.webm', size: 5, sha256: 'a'.repeat(64) } } },
    { id: 'export', version: 2, files: { mp4: { url: 'https://x/b.mp4', size: 6, sha256: 'b'.repeat(64) } } },
  ]
  const { privateKey } = crypto.generateKeyPairSync('ed25519')
  const f = path.join(os.tmpdir(), `lib-cat-videos-${process.pid}.json`)
  try {
    // Замена своей строки и добавление новой: videos остаются на месте и в том же порядке.
    for (const second of [row, { ...row, id: 'other' }]) {
      fs.writeFileSync(f, JSON.stringify(sealCatalog({ format: 1, seq: 3, packs: [row], videos }, privateKey)))
      const next = upsertCatalog(readCatalogFile(f), second)
      assert.deepEqual(next.videos, videos)
      assert.equal(next.seq, 4)
      assert.equal(next.format, 1)
      const env = sealCatalog(next, privateKey)
      assert.equal(verifyBytes(Buffer.from(env.body, 'base64'), env.sig, publicRaw(privateKey)), true)
      fs.writeFileSync(f, JSON.stringify(env))
      assert.deepEqual(readCatalogFile(f).videos, videos)
    }
    // Пустой список сохраняется как есть, а у каталога без роликов поле не появляется.
    assert.deepEqual(upsertCatalog({ format: 1, seq: 1, packs: [], videos: [] }, row).videos, [])
    assert.equal('videos' in upsertCatalog({ format: 1, seq: 1, packs: [] }, row), false)
    assert.equal('videos' in upsertCatalog(null, row), false)
    // Поля формата и seq из прежнего каталога не перетирают новые.
    assert.equal(upsertCatalog({ format: 7, seq: 2, packs: [], videos }, row, 9).format, 1)
    assert.equal(upsertCatalog({ format: 7, seq: 2, packs: [], videos }, row, 9).seq, 9)
  } finally {
    fs.rmSync(f, { force: true })
  }
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

/** PNG-заглушка: сигнатура и IHDR нужного размера (остальное для проверки заголовка не нужно); noise — случайный хвост (другой sha256). */
function pngStub(w, h, noise = false) {
  const b = Buffer.alloc(700)
  if (noise) crypto.randomBytes(600).copy(b, 33)
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
  assert.deepEqual(catalogRow(manifest, Buffer.from('zip'), 'v1').counts, { bg: 1, sticker: 0, spread: 1, frame: 1, print: 0, album: 0 })
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

test('шаблон с рамочками и заготовками (131): borders/drafts — только minApp не ниже 0.28.0', () => {
  const d = setDir()
  const p = path.join(d, 'manifest.src.json')
  const src = JSON.parse(fs.readFileSync(p, 'utf8'))
  const put = (minApp, spread) => {
    src.minApp = minApp
    fs.writeFileSync(p, JSON.stringify(src))
    fs.writeFileSync(path.join(d, 'spreads', 'hero.json'), typeof spread === 'string' ? spread : JSON.stringify(spread))
  }
  const tpl = (extra) => ({ v: 1, pages: 1, page: [{}], places: [{ x: 0.1, y: 0.1, w: 0.5, h: 0.5 }], ...extra })
  const borders = { borders: [{ i: 0, kind: 'line' }] }
  const drafts = { drafts: [{ w: 0.4, text: '[Имена]' }] }

  put('0.27.0', tpl({}))
  assert.equal(buildPack(d).manifest.items[1].kind, 'spread') // без новых списков 0.27 достаточно
  put('0.27.0', tpl({ borders: [], drafts: [] })) // пустые списки — не использование
  assert.equal(buildPack(d).manifest.items[1].kind, 'spread')
  put('0.27.0', tpl(borders))
  assert.throws(() => buildPack(d), /borders требует minApp не ниже 0\.28\.0/)
  put('0.27.0', tpl(drafts))
  assert.throws(() => buildPack(d), /drafts требует minApp не ниже 0\.28\.0/)
  put('0.20.1', tpl({ ...borders, ...drafts })) // числовое сравнение, оба списка названы
  assert.throws(() => buildPack(d), /borders\/drafts требует/)
  put('0.9.0', tpl(drafts))
  assert.throws(() => buildPack(d), /minApp не ниже/)
  for (const ok of ['0.28.0', '0.28.1', '1.0.0']) {
    put(ok, tpl({ ...borders, ...drafts }))
    assert.equal(buildPack(d).manifest.items[1].kind, 'spread', ok)
  }
  // Go разбирает ключи без учёта регистра: "Borders"/"DRAFTS" — те же списки.
  put('0.27.0', tpl({ Borders: borders.borders }))
  assert.throws(() => buildPack(d), /borders требует minApp не ниже 0\.28\.0/)
  put('0.27.0', tpl({ DRAFTS: drafts.drafts }))
  assert.throws(() => buildPack(d), /drafts требует/)
  put('0.28.0', tpl({ Borders: borders.borders }))
  assert.equal(buildPack(d).manifest.items[1].kind, 'spread')
  // Не JSON и не объект здесь не ловим (программа пропустит сама); borders в массиве верхнего уровня — не шаблон.
  for (const odd of ['{не json', '[]', 'null', '42', '"borders"']) {
    put('0.27.0', odd)
    assert.doesNotThrow(() => checkSpreadFile('x.json', Buffer.from(odd), '0.27.0'), odd)
  }
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
  assert.deepEqual(catalogRow(manifest, Buffer.from('zip'), 'v1').counts, { bg: 1, sticker: 0, spread: 1, frame: 0, print: 1, album: 0 })
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

test('rejects zip over 70 MB', () => {
  assert.doesNotThrow(() => assertZipSize(Buffer.alloc(70 << 20)))
  assert.throws(() => assertZipSize(Buffer.alloc((70 << 20) + 1)), /больше предела 70 МБ/)
})

test('предел zip по minApp (0.30-wave2-1): ниже 0.29.0 — 70 МБ, от 0.29.0 — 240 МБ; граница включительно', () => {
  const over70 = Buffer.alloc((70 << 20) + 1)
  assert.equal(zipLimit('0.28.9'), MAX_ZIP)
  assert.equal(zipLimit('0.29.0'), MAX_ZIP_WAVE2)
  assert.equal(zipLimit('0.30.1'), MAX_ZIP_WAVE2)
  assert.equal(zipLimit(undefined), MAX_ZIP) // без minApp (самопроверки, пример) — прежнее
  assert.ok(MAX_ZIP_WAVE2 < 300 << 20) // ниже жёсткого fetch.MaxZip
  assert.throws(() => assertZipSize(over70, '0.28.0'), /больше предела 70 МБ/)
  assert.doesNotThrow(() => assertZipSize(over70, '0.29.0'))
  assert.doesNotThrow(() => assertZipSize(Buffer.alloc(MAX_ZIP_WAVE2), '0.29.0'))
  assert.throws(() => assertZipSize(Buffer.alloc(MAX_ZIP_WAVE2 + 1), '0.29.0'), /больше предела 240 МБ \(minApp 0\.29\.0\)/)
  assert.doesNotThrow(() => assertZipSize(Buffer.alloc(0), '0.29.0')) // пустой — не наше дело (манифест не даст)
})

test('readZip: распакованное целиком больше 1 ГБ — ошибка, не раздувание памяти', () => {
  // 17 записей по 64 МБ нулей: в сжатом виде крошечные, суммарно > 1 ГБ
  const chunk = Buffer.alloc(64 << 20)
  const files = Array.from({ length: 17 }, (_, i) => ({ name: `z${i}.bin`, data: chunk }))
  assert.throws(() => readZip(makeZip(files)), /распакованное больше 1024 МБ/)
})

test('keeps keys of previous version: пропавший sha256 — отказ, --drop разрешает, перепаковка даёт те же ключи', () => {
  const d = setDir()
  const prev = buildPack(d).manifest
  // перепаковка той же папки: sha256 те же (регресс на наборах 0.17)
  const again = buildPack(d).manifest
  assert.deepEqual(
    again.items.map((i) => i.sha256),
    prev.items.map((i) => i.sha256),
  )
  assert.doesNotThrow(() => checkPrev(prev, again))
  // v2: картинку заменили, макет (spread) не ключ
  fs.writeFileSync(path.join(d, 'bg', 'one.png'), pngStub(64, 64, true))
  const next = buildPack(d).manifest
  assert.throws(() => checkPrev(prev, next), /пропали ключи прежней \(1\): bg\/one\.png/)
  assert.doesNotThrow(() => checkPrev(prev, next, ['bg/one.png']))
  assert.throws(() => checkPrev(prev, next, ['нет/такого.png']), /нет в прежней версии/)
  // потеря spread ключом не считается
  const noSpread = { ...prev, items: prev.items.filter((i) => i.kind !== 'spread') }
  assert.doesNotThrow(() => checkPrev(prev, noSpread))
})

test('каталог не сортируется: прежние на месте, новая в конец, замена сохраняет позицию', () => {
  const { manifest, files } = buildPack(setDir())
  const zip = makeZip(files)
  const row = (id, version = 1) => ({ ...catalogRow({ ...manifest, id, version }, zip, `${id}.zip`) })
  let cat = upsertCatalog(null, row('wedding-navy'))
  cat = upsertCatalog(cat, row('school-autumn'))
  cat = upsertCatalog(cat, row('aaa-first'))
  assert.deepEqual(
    cat.packs.map((p) => p.id),
    ['wedding-navy', 'school-autumn', 'aaa-first'],
  )
  cat = upsertCatalog(cat, row('school-autumn', 2))
  assert.deepEqual(
    cat.packs.map((p) => `${p.id}:${p.version}`),
    ['wedding-navy:1', 'school-autumn:2', 'aaa-first:1'],
  )
})

test('writes needs: поле в строке каталога, не в манифесте; minApp от 0.27.0; без needs поля нет', () => {
  const d = setDir()
  const p = path.join(d, 'manifest.src.json')
  const src = JSON.parse(fs.readFileSync(p, 'utf8'))
  const build = (patch) => {
    fs.writeFileSync(p, JSON.stringify({ ...src, ...patch }))
    return buildPack(d)
  }
  const plain = build({})
  assert.equal(plain.needs, undefined)
  assert.ok(!('needs' in catalogRow(plain.manifest, makeZip(plain.files), 'a.zip', plain.needs)))
  const ok = build({ minApp: '0.27.0', needs: 'wedding-navy-watercolor' })
  const row = catalogRow(ok.manifest, makeZip(ok.files), 'a.zip', ok.needs)
  assert.equal(row.needs, 'wedding-navy-watercolor')
  assert.ok(!('needs' in ok.manifest))
  assert.throws(() => build({ minApp: '0.26.0', needs: 'wedding-navy-watercolor' }), /minApp не ниже 0\.27\.0/)
  assert.throws(() => build({ minApp: '0.27.0', needs: 'Плохой id' }), /needs/)
  assert.throws(() => build({ minApp: '0.27.0', needs: 'test-set' }), /сам набор/)
})

/** Папка набора макетов: id, minApp и файлы макетов layouts/*.json; sample — шаги по умолчанию. */
function layoutsDir(files = { 'layouts/wedding.json': null }, patch = {}) {
  const d = fs.mkdtempSync(path.join(os.tmpdir(), 'libpack-look-'))
  fs.mkdirSync(path.join(d, 'layouts'))
  const items = []
  for (const [file, data] of Object.entries(files)) {
    fs.writeFileSync(path.join(d, ...file.split('/')), data ?? lookJSON())
    items.push({ file, kind: 'album', name: file })
  }
  fs.writeFileSync(path.join(d, 'manifest.src.json'), JSON.stringify({ id: 'wedding-layouts', version: 1, minApp: '0.28.0', title: 'Макеты', needs: 'wedding-set', items, ...patch }))
  return d
}

const ST = { tpl: { pages: 2, ratio: 2 } } // шаг: ratio обязателен (эталон мест)
const lookJSON = (extra = {}) => JSON.stringify({ v: 1, steps: [ST], ...extra })

test('макеты альбома (131): kind album — .json, id на -layouts, minApp не ниже 0.28.0, counts.album, круг через zip', () => {
  const d = layoutsDir()
  const { manifest, files } = buildPack(d)
  assert.equal(manifest.items[0].kind, 'album')
  const zip = makeZip(files)
  assert.equal(catalogRow(manifest, zip, 'v1').counts.album, 1)
  assert.equal(readZip(zip).get('layouts/wedding.json').toString(), lookJSON())

  const put = (patch, files) => buildPack(layoutsDir(files, patch))
  assert.throws(() => put({ minApp: '0.27.0' }), /minApp не ниже 0\.28\.0/)
  assert.throws(() => put({ minApp: '0.9.0' }), /minApp не ниже 0\.28\.0/) // числовое сравнение, не строковое
  assert.equal(put({ minApp: '0.100.0' }).manifest.minApp, '0.100.0')
  assert.equal(put({ minApp: '1.0.0' }).manifest.items[0].kind, 'album')
  assert.throws(() => put({ id: 'wedding-set' }), /только в наборе с id <имя>-layouts/)
  assert.throws(() => put({ id: 'layouts-wedding' }), /-layouts/)
  assert.throws(() => put({}, { 'layouts/wedding.txt': null }), /только \.json/)
  assert.throws(() => put({}, { 'layouts/a.json': '' }), /размер вне предела/)
  assert.throws(() => put({}, { 'layouts/a.json': ' '.repeat((512 << 10) + 1) }), /размер вне предела/)
  const head = Buffer.from(lookJSON({ pad: '' })).length
  const ok = put({}, { 'layouts/a.json': lookJSON({ pad: 'x'.repeat((512 << 10) - head) }) }) // ровно 512 КБ — предел включительно
  assert.equal(ok.manifest.items[0].size, 512 << 10)
  // несколько макетов сразу, один набор
  const many = put({}, { 'layouts/a.json': null, 'layouts/b.json': lookJSON({ steps: [ST, ST] }), 'layouts/c.json': null })
  assert.equal(catalogRow(many.manifest, makeZip(many.files), 'v1').counts.album, 3)
  // старый набор без макетов собирается как раньше
  assert.equal(buildPack(setDir()).manifest.minApp, '0.16.0')
})

test('макеты альбома (131): содержимое — JSON-объект v 1 с шагами 1…24; пустое и битое — ошибка, не паника', () => {
  const bad = (data, re) => assert.throws(() => checkAlbumFile('a.json', Buffer.from(data)), re, String(data))
  assert.doesNotThrow(() => checkAlbumFile('a.json', Buffer.from(lookJSON())))
  assert.doesNotThrow(() => checkAlbumFile('a.json', Buffer.from(lookJSON({ steps: Array.from({ length: 24 }, () => ST) }))))
  bad(lookJSON({ steps: Array.from({ length: 25 }, () => ST) }), /steps 1…24/)
  bad(lookJSON({ steps: [] }), /steps 1…24/)
  bad(lookJSON({ steps: 'x' }), /steps 1…24/)
  bad(JSON.stringify({ v: 1 }), /steps 1…24/)
  bad(lookJSON({ v: 2 }), /v макета/)
  bad(lookJSON({ v: '1' }), /v макета/)
  for (const x of ['[]', 'null', '"x"', '42']) bad(x, /не JSON-объект/)
  bad('{не json', /не JSON/)
  bad('', /не JSON/)
  bad('{"v":1,"steps":[{"tpl"', /не JSON/) // усечённый
  // ratio шага и сторон обложки — число 0,2…5 (Р2): без него LookBoxes сожмёт разворот в квадрат
  for (const t of [{ pages: 2 }, { ratio: 0 }, { ratio: 5.5 }, { ratio: '2' }, { ratio: null }]) bad(lookJSON({ steps: [ST, { tpl: t }] }), /steps\[1\]\.tpl\.ratio — число 0.2…5/)
  for (const st of [{}, { tpl: null }, { tpl: [] }]) bad(lookJSON({ steps: [st] }), /steps\[0\]\.tpl — не объект/)
  bad(lookJSON({ cover: { front: { pages: 1 } } }), /cover\.front\.ratio/)
  bad(lookJSON({ cover: { back: 'x' } }), /cover\.back — не объект/)
  assert.doesNotThrow(() => checkAlbumFile('a.json', Buffer.from(lookJSON({ cover: { bg: { color: '#fff' }, back: { ratio: 0.2 }, front: { ratio: 5 } } }))))
  // ключ библиотеки — как есть: экранированный сборка мусора 0.27 не увидит (А5); вложенный и в незнакомом поле — можно
  const key = 'my:0123456789abcdef'
  assert.doesNotThrow(() => checkAlbumFile('a.json', Buffer.from(lookJSON({ steps: [{ tpl: { ratio: 2, page: [{ lib: key }] } }], auto: { x: [key] } }))))
  const esc = lookJSON({ steps: [{ tpl: { ratio: 2, page: [{ lib: key }] } }] }).replace(key, 'my\\u003a' + key.slice(3))
  assert.ok(esc.includes('\\u003a'))
  bad(esc, /ключ my:0123456789abcdef записан не как есть/)
  // через buildPack: ошибка называет файл
  assert.throws(() => buildPack(layoutsDir({ 'layouts/x.json': '{"v":1' })), /layouts\/x\.json: макет альбома — не JSON/)
})

test('TestPackPrevKeepsAlbumPath: путь записи album прежней версии остаётся в новой, иначе отказ; --drop разрешает', () => {
  const v1 = buildPack(layoutsDir({ 'layouts/a.json': null, 'layouts/b.json': lookJSON({ steps: [ST, ST] }) })).manifest
  // тот же путь, другое содержимое (правка макета): ключ меняется, путь держит — можно
  const edited = buildPack(layoutsDir({ 'layouts/a.json': lookJSON({ name: 'новый' }), 'layouts/b.json': lookJSON({ steps: [ST, ST] }) })).manifest
  assert.notEqual(edited.items[0].sha256, v1.items[0].sha256)
  assert.doesNotThrow(() => checkPrev(v1, edited))
  // новый макет в добавок — можно
  const more = buildPack(layoutsDir({ 'layouts/a.json': null, 'layouts/b.json': lookJSON({ steps: [ST, ST] }), 'layouts/c.json': null })).manifest
  assert.doesNotThrow(() => checkPrev(v1, more))
  // путь переименован — отказ, с именем пропавшего
  const renamed = buildPack(layoutsDir({ 'layouts/a2.json': null, 'layouts/b.json': lookJSON({ steps: [ST, ST] }) })).manifest
  assert.throws(() => checkPrev(v1, renamed), /пропали пути макетов альбома прежней \(1\): layouts\/a\.json/)
  assert.doesNotThrow(() => checkPrev(v1, renamed, ['layouts/a.json']))
  assert.throws(() => checkPrev(v1, renamed, ['layouts/zzz.json']), /нет в прежней версии/)
  // все макеты убраны — отказ по каждому; --drop одного не снимает отказ по другому
  const none = { ...v1, items: [] }
  assert.throws(() => checkPrev(v1, none), /\(2\)/)
  assert.throws(() => checkPrev(v1, none, ['layouts/a.json']), /\(1\): layouts\/b\.json/)
  assert.doesNotThrow(() => checkPrev(v1, none, ['layouts/a.json', 'layouts/b.json']))
  // предметы других видов этим правилом не затронуты: пропавший spread по-прежнему можно
  const withSpread = { ...v1, items: [...v1.items, { file: 'spreads/s.json', kind: 'spread', sha256: 'f'.repeat(64) }] }
  assert.doesNotThrow(() => checkPrev(withSpread, v1))
  // через checkPublish: каталог + --prev
  const zip1 = makeZip(buildPack(layoutsDir({ 'layouts/a.json': null })).files)
  const cat = upsertCatalog(null, catalogRow(v1, zip1, 'a.zip'))
  const next = { ...renamed, version: 2 }
  assert.throws(() => checkPublish(cat, next, undefined, readPrev(zip1)), /пропали пути макетов/)
})

/** JPEG-заглушка: SOI, APP0, SOF0 нужного размера, SOS и конец (декодировать её нельзя — заголовок читается). */
function jpegStub(w, h, noise = false) {
  const app0 = Buffer.from([0xff, 0xe0, 0x00, 0x10, 0x4a, 0x46, 0x49, 0x46, 0x00, 1, 1, 0, 0, 1, 0, 1, 0, 0])
  const sof = Buffer.from([0xff, 0xc0, 0x00, 0x0b, 8, h >> 8, h & 255, w >> 8, w & 255, 1, 1, 0x11, 0])
  const sos = Buffer.from([0xff, 0xda, 0x00, 0x08, 1, 1, 0, 0, 0x3f, 0, ...(noise ? crypto.randomBytes(300) : [])])
  return Buffer.concat([Buffer.from([0xff, 0xd8]), app0, sof, sos, Buffer.from([0xff, 0xd9])])
}

/** Папка набора с одним фоном и стикером нужных размеров. */
function rasterSet(bg, sticker, bgFile = 'bg/a.png', stFile = 'stickers/s.png') {
  const d = fs.mkdtempSync(path.join(os.tmpdir(), 'libpack-r-'))
  fs.mkdirSync(path.join(d, 'bg'))
  fs.mkdirSync(path.join(d, 'stickers'))
  fs.writeFileSync(path.join(d, bgFile), bg)
  fs.writeFileSync(path.join(d, stFile), sticker)
  fs.writeFileSync(
    path.join(d, 'manifest.src.json'),
    JSON.stringify({
      id: 'raster-set',
      version: 1,
      minApp: '0.16.0',
      title: 'Растры',
      items: [
        { file: bgFile, kind: 'bg', name: 'Фон' },
        { file: stFile, kind: 'sticker', name: 'Стикер' },
      ],
    }),
  )
  return d
}

test('rejects sticker side over 4096', () => {
  assert.equal(buildPack(rasterSet(pngStub(100, 100), pngStub(4096, 4096))).manifest.items.length, 2) // предел включительно
  assert.throws(() => buildPack(rasterSet(pngStub(100, 100), pngStub(4097, 10))), /сторона стикера 4097×10/)
  assert.throws(() => buildPack(rasterSet(pngStub(100, 100), pngStub(10, 4097))), /сторона стикера/)
  assert.throws(() => buildPack(rasterSet(pngStub(100, 100), pngStub(70000, 70000))), /сторона стикера/)
  // то же для JPEG по SOF
  const j = (w, h) => rasterSet(pngStub(100, 100), jpegStub(w, h), 'bg/a.png', 'stickers/s.jpg')
  assert.equal(buildPack(j(4096, 100)).manifest.items[1].file, 'stickers/s.jpg')
  assert.throws(() => buildPack(j(4097, 100)), /сторона стикера 4097×100/)
  assert.throws(() => buildPack(j(100, 5000)), /сторона стикера/)
})

test('rejects bg over 100 Mp', () => {
  assert.equal(buildPack(rasterSet(pngStub(10000, 10000), pngStub(10, 10))).manifest.items.length, 2) // ровно 100 Мп — можно
  assert.throws(() => buildPack(rasterSet(pngStub(10000, 10001), pngStub(10, 10))), /больше 100 Мп/)
  assert.throws(() => buildPack(rasterSet(pngStub(70000, 70000), pngStub(10, 10))), /больше 100 Мп/)
  const j = (w, h) => rasterSet(jpegStub(w, h), pngStub(10, 10), 'bg/a.jpg')
  assert.equal(buildPack(j(10000, 10000)).manifest.items.length, 2)
  assert.throws(() => buildPack(j(10001, 10000)), /фон 10001×10000 больше 100 Мп/)
  // сторона фона не ограничена, только площадь
  assert.equal(buildPack(rasterSet(pngStub(65535, 1500), pngStub(10, 10))).manifest.items.length, 2)
})

test('растр: пустой, усечённый, чужой по расширению и огромный файл — ошибка, не паника и не чтение в память', () => {
  const put = (bg, name = 'bg/a.png') => rasterSet(bg, pngStub(10, 10), name)
  assert.throws(() => buildPack(put(Buffer.alloc(0))), /размер вне предела/)
  assert.throws(() => buildPack(put(pngStub(10, 10).subarray(0, 20))), /PNG/)
  assert.throws(() => buildPack(put(jpegStub(10, 10).subarray(0, 5), 'bg/a.jpg')), /JPEG/)
  assert.throws(() => buildPack(put(jpegStub(10, 10), 'bg/a.png')), /не совпадает с расширением/)
  assert.throws(() => buildPack(put(pngStub(10, 10), 'bg/a.jpg')), /не совпадает с расширением/)
  assert.throws(() => buildPack(put(Buffer.from('GIF89a'.repeat(100)))), /не PNG и не JPEG/)
  assert.throws(() => buildPack(put(pngStub(0, 10))), /размер картинки 0×10/)
  const d = put(pngStub(10, 10))
  fs.truncateSync(path.join(d, 'bg', 'a.png'), (60 << 20) + 1) // разреженный файл больше предела фона: не читается
  assert.throws(() => buildPack(d), /размер вне предела/)
  const big = put(pngStub(10, 10))
  fs.truncateSync(path.join(big, 'manifest.src.json'), 2 << 20)
  assert.throws(() => buildPack(big), /больше предела/)
})

test('jpegSize: SOF после APP и заполнителей; битые и усечённые заголовки — исключение, на случайных входах без зависаний', () => {
  assert.deepEqual(jpegSize(jpegStub(321, 123)), { w: 321, h: 123 })
  const fill = Buffer.concat([Buffer.from([0xff, 0xd8, 0xff, 0xff, 0xff]), jpegStub(5, 6).subarray(2)])
  assert.deepEqual(jpegSize(fill), { w: 5, h: 6 })
  assert.throws(() => jpegSize(Buffer.from([0xff, 0xd8])), /нет SOF|заголовка/)
  assert.throws(() => jpegSize(Buffer.from([0xff, 0xd8, 0xff, 0xc0, 0x00, 0x0b, 8])), /усечённый/)
  assert.throws(() => jpegSize(Buffer.from([0xff, 0xd8, 0xff, 0xe0, 0x00, 0x01, 0, 0])), /длина/)
  assert.throws(() => jpegSize(Buffer.from([0xff, 0xd8, 0x00, 0x00, 0x00, 0x00])), /разметка/)
  assert.throws(() => imageInfo(Buffer.alloc(0)), /не PNG/)
  for (let i = 0; i < 2000; i++) {
    const b = Buffer.concat([Buffer.from([0xff, 0xd8]), crypto.randomBytes(1 + (i % 64))])
    try {
      jpegSize(b)
    } catch (e) {
      assert.ok(e instanceof Error && !(e instanceof RangeError), `${e}`) // чтение за краем — не RangeError
    }
  }
})

test('checkPublish: набор уже в каталоге — нужен --prev с тем же id и sha256 строки каталога', () => {
  const d = setDir()
  const v1 = buildPack(d)
  const zip1 = makeZip(v1.files)
  const cat = upsertCatalog(null, catalogRow(v1.manifest, zip1, 'a.zip'))
  const prev = readPrev(zip1)
  const next = { ...v1.manifest, version: 3 }
  assert.throws(() => checkPublish(cat, next, undefined, undefined), /нужен --prev/)
  assert.doesNotThrow(() => checkPublish(cat, next, undefined, prev))
  assert.doesNotThrow(() => checkPublish(null, next, undefined, undefined)) // нового набора прежнего нет
  assert.throws(() => checkPublish(cat, next, undefined, { ...prev, manifest: { ...prev.manifest, id: 'other-set' } }), /архив набора other-set/)
  assert.throws(() => checkPublish(cat, next, undefined, { ...prev, sha256: '0'.repeat(64) }), /не совпадает со строкой каталога/)
  // --prev проверяет и ключи
  assert.throws(() => checkPublish(cat, { ...next, items: [] }, undefined, prev), /пропали ключи/)
})

test('checkPublish (0.30-wave2-1): новый id в готовом каталоге — minApp не ниже 0.29.0; старый id и пустой каталог — прежнее правило', () => {
  const { manifest, files } = buildPack(setDir()) // minApp 0.16.0
  const zip = makeZip(files)
  const oldZip = makeZip(buildPack(setDir()).files.map((f) => (f.name === 'manifest.json' ? { ...f, data: Buffer.from(JSON.stringify({ ...manifest, id: 'old-set' })) } : f)))
  const cat = upsertCatalog(null, catalogRow({ ...manifest, id: 'old-set' }, oldZip, 'a.zip'))
  const fresh = (minApp) => ({ ...manifest, id: 'wave2-set', minApp })
  for (const low of ['0.16.0', '0.28.0', '0.28.9']) assert.throws(() => checkPublish(cat, fresh(low), undefined, undefined), /новый набор wave2-set.*не ниже 0\.29\.0/)
  assert.doesNotThrow(() => checkPublish(cat, fresh('0.29.0'), undefined, undefined))
  assert.doesNotThrow(() => checkPublish(cat, fresh('0.30.0'), undefined, undefined))
  assert.doesNotThrow(() => checkPublish(null, fresh('0.16.0'), undefined, undefined)) // первый набор нового каталога (--new)
  // пустой, но подписанный каталог — тоже каталог: старые клиенты его читают
  assert.throws(() => checkPublish({ format: 1, seq: 3, packs: [] }, fresh('0.28.0'), undefined, undefined), /новый набор/)
  // повторный выпуск старого набора с низким minApp — прежнее правило (нужен --prev, дальше без отказа по minApp)
  const old = { ...manifest, id: 'old-set', version: 3 }
  assert.throws(() => checkPublish(cat, old, undefined, undefined), /нужен --prev/)
  assert.doesNotThrow(() => checkPublish(cat, old, undefined, readPrev(oldZip)))
  // несколько новых подряд: каждый проверяется сам, прошедший не открывает путь следующему
  const cat2 = upsertCatalog(cat, catalogRow(fresh('0.29.0'), zip, 'b.zip'))
  assert.throws(() => checkPublish(cat2, { ...manifest, id: 'wave2-b', minApp: '0.27.0' }, undefined, undefined), /новый набор wave2-b/)
  assert.doesNotThrow(() => checkPublish(cat2, { ...manifest, id: 'wave2-c', minApp: '0.29.0' }, undefined, undefined))
})

test('checkPublish: needs — донор в каталоге и раньше по порядку', () => {
  const { manifest, files } = buildPack(setDir())
  const zip = makeZip(files)
  const row = (id) => catalogRow({ ...manifest, id }, zip, `${id}.zip`)
  let cat = upsertCatalog(null, row('donor-set'))
  cat = upsertCatalog(cat, row('other-set'))
  const addon = { ...manifest, id: 'addon-set', minApp: '0.29.0' } // новый id — не ниже 0.29.0 (0.30-wave2-1)
  assert.doesNotThrow(() => checkPublish(cat, addon, 'donor-set', undefined))
  assert.throws(() => checkPublish(cat, addon, 'ghost-set', undefined), /донора ghost-set нет в каталоге/)
  assert.throws(() => checkPublish(null, addon, 'donor-set', undefined), /донора donor-set нет/)
  // дополнение уже стоит раньше донора: переставить строки нельзя
  const early = upsertCatalog(upsertCatalog(null, row('addon-set')), row('donor-set'))
  const prev = readPrev(zip)
  const early2 = { ...early, packs: early.packs.map((p) => (p.id === 'addon-set' ? { ...p, sha256: prev.sha256 } : p)) }
  assert.throws(() => checkPublish(early2, { ...addon, items: manifest.items }, 'donor-set', { ...prev, manifest: { ...manifest, id: 'addon-set' } }), /после addon-set/)
  assert.doesNotThrow(() => checkPublish(cat, addon, 'other-set', undefined))
})

test('orderWarnings: дополнение раньше донора — предупреждение; порядок донор → дополнение, без needs и без каталога — тихо', () => {
  const { manifest, files } = buildPack(setDir())
  const zip = makeZip(files)
  const row = (id, needs) => catalogRow({ ...manifest, id }, zip, `${id}.zip`, needs)
  const good = upsertCatalog(upsertCatalog(null, row('donor-set')), row('addon-set', 'donor-set'))
  assert.deepEqual(orderWarnings(good), [])
  assert.deepEqual(orderWarnings(null), [])
  assert.deepEqual(orderWarnings({ packs: [] }), [])
  const bad = upsertCatalog(upsertCatalog(upsertCatalog(null, row('addon-b', 'donor-set')), row('addon-a', 'donor-set')), row('donor-set'))
  assert.deepEqual(orderWarnings(bad), ['дополнение addon-b стоит в каталоге раньше донора donor-set', 'дополнение addon-a стоит в каталоге раньше донора donor-set'])
  // донор отсутствует — порядок не нарушен (это отказ checkPublish, не предупреждение)
  assert.deepEqual(orderWarnings(upsertCatalog(null, row('addon-set', 'ghost-set'))), [])
})

// ---- CLI целиком (child_process): связка main ----

const SCRIPT = fileURLToPath(new URL('./library-pack.mjs', import.meta.url))

/** Временная папка с тестовым ключом ed25519 (боевой docs/private не нужен). */
function cliEnv() {
  const w = fs.mkdtempSync(path.join(os.tmpdir(), 'libpack-cli-'))
  const { privateKey } = crypto.generateKeyPairSync('ed25519')
  fs.writeFileSync(path.join(w, 'key.pem'), privateKey.export({ type: 'pkcs8', format: 'pem' }))
  const out = path.join(w, 'out')
  const run = (...args) => {
    const r = spawnSync(process.execPath, [SCRIPT, ...args, '--key', path.join(w, 'key.pem'), '--out', out], { encoding: 'utf8' })
    return { code: r.status, text: r.stdout + r.stderr }
  }
  const listing = () => (fs.existsSync(out) ? fs.readdirSync(out).sort() : [])
  const catalog = () => readCatalogFile(path.join(out, 'catalog.json'))
  return { w, out, run, listing, catalog }
}

const fixedBg = pngStub(64, 64, true) // один и тот же фон во всех версиях: ключи сходятся

/** Папка набора: id, версия, minApp, needs; seed меняет LICENSE.txt (другой архив). */
function cliSet(id, version, extra = {}, seed = '') {
  const d = setDir()
  fs.writeFileSync(path.join(d, 'bg', 'one.png'), fixedBg)
  const p = path.join(d, 'manifest.src.json')
  fs.writeFileSync(p, JSON.stringify({ ...JSON.parse(fs.readFileSync(p, 'utf8')), id, version, ...extra }))
  if (seed) fs.writeFileSync(path.join(d, 'LICENSE.txt'), seed)
  return d
}

test('CLI: набор уже в каталоге — без --prev отказ до записи; с чужим и не тем архивом отказ; с опубликованным — выпуск', () => {
  const e = cliEnv()
  const first = e.run(cliSet('my-set', 1), '--new')
  assert.equal(first.code, 0, first.text)
  const state = () => ({ files: e.listing(), cat: fs.readFileSync(path.join(e.out, 'catalog.json'), 'utf8') })
  const before = state()
  const v2 = cliSet('my-set', 2)

  const noPrev = e.run(v2)
  assert.equal(noPrev.code, 1)
  assert.match(noPrev.text, /нужен --prev/)
  assert.deepEqual(state(), before) // ни v2-zip, ни новой подписи

  const published = path.join(e.out, 'my-set-1.zip')
  const stranger = path.join(e.w, 'stranger.zip') // другой набор: id не совпадает
  fs.writeFileSync(stranger, makeZip(buildPack(cliSet('other-set', 1)).files))
  const wrongId = e.run(v2, '--prev', stranger)
  assert.equal(wrongId.code, 1)
  assert.match(wrongId.text, /архив набора other-set/)

  const repacked = path.join(e.w, 'repacked.zip') // тот же id, но не опубликованный архив (другие байты)
  fs.writeFileSync(repacked, makeZip(buildPack(cliSet('my-set', 1, {}, 'другая лицензия')).files))
  const wrongSha = e.run(v2, '--prev', repacked)
  assert.equal(wrongSha.code, 1)
  assert.match(wrongSha.text, /sha256 архива не совпадает/)
  assert.deepEqual(state(), before)

  const ok = e.run(v2, '--prev', published)
  assert.equal(ok.code, 0, ok.text)
  assert.deepEqual(e.listing(), ['catalog.json', 'my-set-1.zip', 'my-set-2.zip'])
  assert.deepEqual(e.catalog().packs.map((p) => `${p.id}:${p.version}`), ['my-set:2'])
  assert.equal(e.catalog().seq, 2)
})

test('CLI: needs без донора в каталоге — отказ, папка вывода пуста; с донором — выпуск; донор не раньше — отказ', () => {
  const e = cliEnv()
  const addon = (id) => cliSet(id, 1, { minApp: '0.29.0', needs: 'donor-set' })
  const noCatalog = e.run(addon('addon-set'), '--new') // донор не выпущен: каталога ещё нет
  assert.equal(noCatalog.code, 1)
  assert.match(noCatalog.text, /донора donor-set нет в каталоге/)
  assert.deepEqual(e.listing(), []) // до записи: папка вывода не создана или пуста

  assert.equal(e.run(cliSet('donor-set', 1), '--new').code, 0)
  const before = e.listing()
  const ghost = e.run(cliSet('addon-set', 1, { minApp: '0.29.0', needs: 'ghost-set' }))
  assert.equal(ghost.code, 1)
  assert.match(ghost.text, /донора ghost-set нет/)
  assert.deepEqual(e.listing(), before)

  const ok = e.run(addon('addon-set'))
  assert.equal(ok.code, 0, ok.text)
  assert.deepEqual(e.catalog().packs.map((p) => `${p.id}${p.needs ? '>' + p.needs : ''}`), ['donor-set', 'addon-set>donor-set'])
})

test('CLI: отказ по содержимому набора и по ключам — до записи; булев ключ перед папкой; неизвестный ключ и ключ без значения', () => {
  const e = cliEnv()
  const bad = rasterSet(pngStub(100, 100), pngStub(4097, 10))
  const r = e.run(bad, '--new')
  assert.equal(r.code, 1)
  assert.match(r.text, /сторона стикера/)
  assert.deepEqual(e.listing(), [])

  assert.equal(e.run('--new', cliSet('my-set', 1)).code, 0) // --new стоит перед папкой
  assert.equal(e.run(cliSet('my-set', 2), '--bogus').code, 2)
  const noVal = spawnSync(process.execPath, [SCRIPT, cliSet('my-set', 2), '--prev'], { encoding: 'utf8' })
  assert.equal(noVal.status, 2)
  assert.match(noVal.stderr, /--prev: нужно значение/)
  const help = spawnSync(process.execPath, [SCRIPT, '--help'], { encoding: 'utf8' })
  assert.equal(help.status, 0)
  assert.match(help.stdout, /--test-key/)
})

test('CLI (0.30-wave2-1): новый id с низким minApp в готовом каталоге — отказ до записи; 0.29.0 — выпуск; старый id — как прежде', () => {
  const e = cliEnv()
  assert.equal(e.run(cliSet('my-set', 1), '--new').code, 0) // первый набор нового каталога: minApp 0.16.0 можно
  const before = { files: e.listing(), cat: fs.readFileSync(path.join(e.out, 'catalog.json'), 'utf8') }
  for (const low of ['0.16.0', '0.28.0']) {
    const r = e.run(cliSet('wave2-set', 1, { minApp: low }))
    assert.equal(r.code, 1, r.text)
    assert.match(r.text, /новый набор wave2-set.*minApp не ниже 0\.29\.0/)
  }
  assert.deepEqual({ files: e.listing(), cat: fs.readFileSync(path.join(e.out, 'catalog.json'), 'utf8') }, before) // ничего не записано
  const ok = e.run(cliSet('wave2-set', 1, { minApp: '0.29.0' }))
  assert.equal(ok.code, 0, ok.text)
  const rep = e.run(cliSet('my-set', 2), '--prev', path.join(e.out, 'my-set-1.zip')) // повторный выпуск старого — с minApp 0.16.0
  assert.equal(rep.code, 0, rep.text)
  assert.deepEqual(e.catalog().packs.map((p) => `${p.id}:${p.version}:${p.minApp}`), ['my-set:2:0.16.0', 'wave2-set:1:0.29.0'])
})
