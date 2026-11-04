// Тесты scripts/pack-check.mjs (спека 143, ступень 1): паритет разбора манифеста с Go (testdata/pack-manifest-cases.json,
// то же читает TestManifestCases), проверка архива — хороший, пропуски, отказы, битые, усечённые, пустые и огромные входы, CLI.
// Запуск: node --test scripts/pack-check.test.mjs
import assert from 'node:assert/strict'
import crypto from 'node:crypto'
import fs from 'node:fs'
import os from 'node:os'
import path from 'node:path'
import { test } from 'node:test'
import { spawnSync } from 'node:child_process'
import { fileURLToPath } from 'node:url'
import zlib from 'node:zlib'
import { makeZip } from './library-pack.mjs'
import { LIMITS, checkZip, clean, imageInfo, manifestVerdict, parseManifest, report, validId, validName } from './pack-check.mjs'

const SCRIPT = fileURLToPath(new URL('./pack-check.mjs', import.meta.url))
const CASES = path.resolve(path.dirname(SCRIPT), '..', 'testdata', 'pack-manifest-cases.json')
const sha = (b) => crypto.createHash('sha256').update(b).digest('hex')
const TMP = fs.mkdtempSync(path.join(os.tmpdir(), 'packcheck-'))
test.after(() => fs.rmSync(TMP, { recursive: true, force: true }))
let seq = 0
const put = (buf, name = `p${seq++}.zip`) => {
  const f = path.join(TMP, name)
  fs.writeFileSync(f, buf)
  return f
}

// ---- паритет манифеста с Go ----

/** Байты манифеста случая — те же правила сборки, что в manifestCase.input (manifest_cases_test.go). */
function input(c) {
  if (c.raw !== undefined) return Buffer.from(c.raw, 'utf8')
  const m = { ...c.manifest }
  if (c.gen) m.items = Array.from({ length: c.gen.count }, (_, i) => ({ file: `f${i}.${c.gen.ext}`, kind: c.gen.kind, theme: 'basic', name: 'n', sha256: '0'.repeat(64), size: c.gen.size }))
  const b = Buffer.from(JSON.stringify(m), 'utf8')
  return c.padTo > b.length ? Buffer.concat([b, Buffer.alloc(c.padTo - b.length, 0x20)]) : b
}

test('паритет: каждый случай даёт тот же итог, что Go (TestManifestCases)', () => {
  const { cases } = JSON.parse(fs.readFileSync(CASES, 'utf8'))
  assert.ok(cases.length >= 150, `случаев ${cases.length}: файл урезан`)
  const names = new Set()
  const fails = []
  for (const c of cases) {
    assert.ok(!names.has(c.name), `случай «${c.name}» повторяется`)
    names.add(c.name)
    assert.ok(Object.keys(c.want).length > 0, `${c.name}: нет want`)
    const got = manifestVerdict(input(c), c.app)
    // как в Go: итог проходит через JSON, ключ want, которого нет в итоге, — ошибка случая
    const norm = JSON.parse(JSON.stringify(got))
    for (const [k, want] of Object.entries(c.want)) {
      if (!(k in norm)) fails.push(`${c.name}: ключ «${k}» не бывает в итоге`)
      else if (JSON.stringify(norm[k]) !== JSON.stringify(want)) fails.push(`${c.name}: ${k} = ${JSON.stringify(norm[k])?.slice(0, 160)}, ждали ${JSON.stringify(want).slice(0, 160)}`)
    }
  }
  assert.deepEqual(fails, [])
})

test('вердикт Go-подобный: отказ несёт code и error, успех — поля набора', () => {
  const bad = manifestVerdict(Buffer.from('{'), '0.31.0')
  assert.deepEqual(bad, { ok: false, code: 'manifest', error: 'манифест набора не читается' })
  assert.throws(() => parseManifest(Buffer.from('')), { code: 'manifest' })
})

test('грамматики id и имени, чистка текста', () => {
  assert.ok(validId('abc') && !validId('ab') && !validId('con') && !validId('com1') && validId('com10'))
  assert.ok(validName('a/b/c.png') && !validName('a/b/c/d.png') && !validName('a..b') && !validName('A.png'))
  assert.equal(clean('  a​b \t c  ', 60), 'ab c')
  assert.equal(clean('я'.repeat(70), 60), 'я'.repeat(60))
})

// ---- сборка тестовых наборов ----

/** PNG-заглушка: сигнатура и IHDR, хвост случайный — свой sha256 у каждой. */
function png(w = 64, h = 64) {
  const b = Buffer.alloc(300)
  crypto.randomBytes(200).copy(b, 40)
  Buffer.from([0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a]).copy(b)
  b.writeUInt32BE(13, 8)
  b.write('IHDR', 12, 'latin1')
  b.writeUInt32BE(w, 16)
  b.writeUInt32BE(h, 20)
  return b
}

/** JPEG-заглушка: SOI, SOF0 с размерами, EOI. */
function jpeg(w = 64, h = 64) {
  const b = Buffer.alloc(24)
  Buffer.from([0xff, 0xd8, 0xff, 0xc0, 0x00, 0x11, 0x08]).copy(b)
  b.writeUInt16BE(h, 7)
  b.writeUInt16BE(w, 9)
  b[11] = 3
  b[22] = 0xff
  b[23] = 0xd9
  return b
}

/** Набор: items [{file, kind, data}] -> zip; over — правка манифеста, extra — лишние файлы архива. */
function pack(items, { over = {}, extra = [], lie = {} } = {}) {
  const manifest = {
    format: 1,
    id: 'test-pack',
    version: 1,
    minApp: '0.16.0',
    title: 'Тестовый набор',
    about: '',
    author: '',
    license: '',
    source: '',
    items: items.map((i) => ({ file: i.file, kind: i.kind, theme: 'basic', name: i.file, sha256: lie[i.file]?.sha256 ?? sha(i.data), size: lie[i.file]?.size ?? i.data.length })),
    ...over,
  }
  return makeZip([{ name: 'manifest.json', data: Buffer.from(JSON.stringify(manifest)) }, ...items.map((i) => ({ name: i.file, data: i.data })), ...extra])
}

const BG = { file: 'bg/a.png', kind: 'bg', data: png() }
const STK = { file: 'st/s.png', kind: 'sticker', data: png(100, 100) }
const SPR = { file: 'sp/t.json', kind: 'spread', data: Buffer.from('{"v":1}') }
const GOOD = () => pack([BG, STK, SPR], { extra: [{ name: 'LICENSE.txt', data: Buffer.from('CC0') }] })

/** Правка записей центрального каталога: fn(buf, offset, name) на каждую запись. */
function patchCentral(zip, fn) {
  const b = Buffer.from(zip)
  for (let p = b.indexOf(Buffer.from([0x50, 0x4b, 0x01, 0x02])); p >= 0 && p < b.length; ) {
    const nl = b.readUInt16LE(p + 28)
    fn(b, p, b.toString('utf8', p + 46, p + 46 + nl))
    const next = b.indexOf(Buffer.from([0x50, 0x4b, 0x01, 0x02]), p + 46 + nl)
    if (next < 0) break
    p = next
  }
  return b
}

// ---- хорошие и пропуски ----

test('хороший набор: ставится, LICENSE.txt игнорируется', async () => {
  const r = await checkZip(put(GOOD()), { app: '0.31.0' })
  assert.equal(r.verdict, 'ok', r.error)
  assert.deepEqual(r.installed, ['bg/a.png', 'st/s.png', 'sp/t.json'])
  assert.deepEqual(r.skipped, [])
  assert.deepEqual(report(r), ['набор «Тестовый набор» (test-pack, версия 1): ставится 3, пропущено 0', 'ставится'])
})

test('Deflate-запись читается так же, как Store', async () => {
  const json = Buffer.from(JSON.stringify({ v: 1, pad: 'x'.repeat(5000) }))
  const r = await checkZip(put(pack([BG, { file: 'sp/big.json', kind: 'spread', data: json }])))
  assert.equal(r.verdict, 'ok', r.error)
})

test('пропуски: SVG, чужой вид, расширение не по содержимому, размеры картинок, типография, битый JSON', async () => {
  const svg = Buffer.from('<svg/>')
  const items = [
    BG,
    { file: 'bg/v.svg', kind: 'bg', data: svg },
    { file: 'x/m.mp4', kind: 'video', data: Buffer.from('v') },
    { file: 'bg/j.png', kind: 'bg', data: jpeg() }, // JPEG под именем .png
    { file: 'bg/huge.png', kind: 'bg', data: png(20000, 20000) }, // 400 Мп
    { file: 'st/wide.png', kind: 'sticker', data: png(5000, 10) },
    { file: 'fr/f.png', kind: 'frame', data: png(5000, 5000) },
    { file: 'p/t.saprint', kind: 'print', data: Buffer.from('{}') },
    { file: 'sp/bad.json', kind: 'spread', data: Buffer.from('не json') },
    { file: 'al/l.json', kind: 'album', data: Buffer.from('{"v":1,"steps":[]}') }, // шагов 0: не макет
    { file: 'bg/ok.jpg', kind: 'bg', data: jpeg(4000, 3000) },
  ]
  const r = await checkZip(put(pack(items)), { app: '0.31.0' })
  assert.equal(r.verdict, 'skip', r.error)
  assert.deepEqual(r.installed, ['bg/a.png', 'bg/ok.jpg'])
  const why = Object.fromEntries(r.skipped.map((s) => [s.file, s.why]))
  assert.match(why['bg/v.svg'], /SVG/)
  assert.match(why['x/m.mp4'], /незнакомый вид/)
  assert.match(why['bg/j.png'], /не совпадает с расширением/)
  assert.match(why['bg/huge.png'], /фон больше 100 Мп/)
  assert.match(why['st/wide.png'], /стикер больше 4096/)
  assert.match(why['fr/f.png'], /рамочка больше 4096/)
  assert.match(why['p/t.saprint'], /только набор из каталога/)
  assert.match(why['sp/bad.json'], /не читается/)
  assert.match(why['al/l.json'], /макет альбома не читается/)
  assert.equal(r.skipped.length, 9)
})

test('imageInfo: PNG, JPEG, усечённые и чужие', () => {
  assert.deepEqual(imageInfo(png(3, 4)), { type: 'png', w: 3, h: 4 })
  assert.deepEqual(imageInfo(jpeg(5, 6)), { type: 'jpeg', w: 5, h: 6 })
  for (const bad of [Buffer.alloc(0), Buffer.from([0xff]), Buffer.from([0xff, 0xd8]), jpeg().subarray(0, 8), png().subarray(0, 20), Buffer.from('GIF89a')]) assert.throws(() => imageInfo(bad), Error)
})

// ---- отказы ----

const rejected = async (zip, re, opts) => {
  const r = await checkZip(typeof zip === 'string' ? zip : put(zip), opts)
  assert.equal(r.verdict, 'reject', `ждали отказ «${re}», получили ${r.verdict}`)
  assert.match(r.error, re)
  return r
}

test('отказ: sha256 и размер по фактическим байтам, а не по заголовкам zip', async () => {
  await rejected(pack([BG], { lie: { 'bg/a.png': { sha256: sha(Buffer.from('другое')) } } }), /sha256/)
  await rejected(pack([BG], { lie: { 'bg/a.png': { size: BG.data.length + 1 } } }), /размер «bg\/a.png» не сходится/)
  // заголовок zip говорит верно, а байты записи подменены при том же размере: хэш ловит
  const zip = Buffer.from(GOOD())
  const at = zip.indexOf(BG.data)
  zip[at + 100] ^= 0xff
  const r = await checkZip(put(zip))
  assert.equal(r.verdict, 'reject')
  assert.match(r.error, /не сходится|повреждён/)
})

test('отказ: запись длиннее заявленного — манифест и заголовок zip согласованы, а поток отдаёт больше', async () => {
  const real = Buffer.alloc(5000, 0x20)
  const lie = { 'sp/t.json': { size: 1000, sha256: sha(Buffer.alloc(1000, 0x20)) } }
  const zip = patchCentral(pack([{ file: 'sp/t.json', kind: 'spread', data: real }], { lie }), (b, p, name) => name === 'sp/t.json' && b.writeUInt32LE(1000, p + 24))
  await rejected(zip, /«sp\/t\.json» не сходится с манифестом по размеру/)
})

test('отказ: нет файла, нет манифеста, манифест не тот', async () => {
  await rejected(makeZip([{ name: 'manifest.json', data: Buffer.from(JSON.stringify({ format: 1, id: 'abc', version: 1, minApp: '0.1.0', title: 'T', items: [{ file: 'a.png', kind: 'bg', theme: 'basic', name: 'a', sha256: sha(png()), size: png().length }] })) }]), /нет «a\.png»/)
  await rejected(makeZip([{ name: 'a.png', data: png() }]), /нет manifest\.json/)
  await rejected(pack([BG], { over: { format: 2 } }), /новой версии программы/)
  await rejected(pack([BG], { over: { minApp: '0.99.0' } }), /нужна версия Sosnina Albums 0\.99\.0 или новее/, { app: '0.31.0' })
  await rejected(pack([{ file: 'x.svg', kind: 'bg', data: Buffer.from('<svg/>') }]), /нечего ставить/)
  const big = Buffer.alloc(300 << 10, 0x20)
  await rejected(makeZip([{ name: 'manifest.json', data: big }]), /больше 256 КБ/)
})

test('отказ: опасные имена, повтор без учёта регистра, шифрование, метод, сжатие', async () => {
  for (const bad of ['../x.png', '/abs.png', 'a\\b.png', 'c:x.png', 'a//b', './a', 'a\u0001b']) {
    await rejected(pack([BG], { extra: [{ name: bad, data: Buffer.from('x') }] }), /недопустимое имя в архиве/)
  }
  await rejected(pack([BG], { extra: [{ name: 'LICENSE.txt', data: Buffer.from('a') }, { name: 'license.TXT', data: Buffer.from('b') }, { name: 'License.txt', data: Buffer.from('c') }] }), /повторяется/)
  const zip = GOOD()
  await rejected(patchCentral(zip, (b, p) => b.writeUInt16LE(b.readUInt16LE(p + 8) | 1, p + 8)), /зашифрованный/)
  await rejected(patchCentral(zip, (b, p) => b.writeUInt16LE(12, p + 10)), /незнакомый метод/)
  // zip-бомба по заголовку: 1 байт сжатого заявлен как 100 МБ
  await rejected(patchCentral(zip, (b, p, name) => name === 'sp/t.json' && b.writeUInt32LE(100 << 20, p + 24)), /zip-бомб/)
  // сжатый размер больше самого файла
  await rejected(patchCentral(zip, (b, p, name) => name === 'sp/t.json' && b.writeUInt32LE(zip.length + 1, p + 20)), /zip-бомб/)
})

test('отказ: предмет в архиве — папка или ссылка, а не обычный файл', async () => {
  const zip = GOOD()
  await rejected(patchCentral(zip, (b, p, name) => name === 'bg/a.png' && b.writeUInt32LE(0x10, p + 38)), /нет «bg\/a\.png»/) // DOS: папка
  const link = (b, p, name) => {
    if (name !== 'bg/a.png') return
    b.writeUInt16LE(3 << 8, p + 4) // Unix
    b.writeUInt32LE((0o120777 << 16) >>> 0, p + 38) // симлинк
  }
  await rejected(patchCentral(zip, link), /нет «bg\/a\.png»/)
  const file = (b, p, name) => {
    if (name !== 'bg/a.png') return
    b.writeUInt16LE(3 << 8, p + 4)
    b.writeUInt32LE((0o100644 << 16) >>> 0, p + 38)
  }
  assert.equal((await checkZip(put(patchCentral(zip, file)))).verdict, 'ok') // Unix-файл как обычно
})

test('отказ: счётчики каталога — записей больше 2000, каталог больше 1 МБ', async () => {
  const zip = Buffer.from(GOOD())
  const eocd = zip.lastIndexOf(Buffer.from([0x50, 0x4b, 0x05, 0x06]))
  const many = Buffer.from(zip)
  many.writeUInt16LE(2001, eocd + 10)
  await rejected(many, /больше 2000 записей/)
  // каталог больше 1 МБ: объявленный размер должен помещаться в файл, иначе это «не читается как zip» (как у Go)
  const bigDir = Buffer.concat([Buffer.alloc(3 << 20), zip])
  bigDir.writeUInt32LE(2 << 20, (3 << 20) + eocd + 12)
  await rejected(bigDir, /каталог архива больше 1 МБ/)
  const outside = Buffer.from(zip)
  outside.writeUInt32LE(2 << 20, eocd + 12)
  await rejected(outside, /не читается как zip/)
  // ZIP64: число записей в локаторе — тот же предел
  const z64 = Buffer.concat([zip.subarray(0, eocd)])
  const rec = Buffer.alloc(56)
  rec.writeUInt32LE(0x06064b50, 0)
  rec.writeBigUInt64LE(44n, 4)
  rec.writeBigUInt64LE(5000n, 32)
  rec.writeBigUInt64LE(5000n, 40)
  rec.writeBigUInt64LE(BigInt(zip.readUInt32LE(eocd + 16)), 48)
  const loc = Buffer.alloc(20)
  loc.writeUInt32LE(0x07064b50, 0)
  loc.writeBigUInt64LE(BigInt(z64.length), 8)
  loc.writeUInt32LE(1, 16)
  const end = Buffer.from(zip.subarray(eocd))
  end.writeUInt16LE(0xffff, 10)
  await rejected(Buffer.concat([z64, rec, loc, end]), /больше 2000 записей|каталог архива больше/)
})

test('пустые, усечённые и мусорные входы — отказ без исключения', async () => {
  await rejected(Buffer.alloc(0), /повреждён/)
  await rejected(Buffer.from('PK'), /повреждён/)
  await rejected(Buffer.alloc(100000, 0), /повреждён/)
  await rejected(crypto.randomBytes(5000), /повреждён/)
  const zip = GOOD()
  for (let n = 0; n < zip.length; n += Math.max(1, zip.length >> 6)) {
    const r = await checkZip(put(zip.subarray(0, n)))
    assert.equal(r.verdict, 'reject', `усечённый на ${n}`)
  }
  await rejected(path.join(TMP, 'нет-такого.zip'), /не открывается/)
  await rejected(TMP, /не обычный файл/) // папка
})

test('огромный файл — отказ по размеру до чтения (разреженный 300 МБ + 1)', async () => {
  const f = path.join(TMP, 'huge.zip')
  const fd = fs.openSync(f, 'w')
  fs.ftruncateSync(fd, (300 << 20) + 1)
  fs.closeSync(fd)
  await rejected(f, /больше 300 МБ/)
  fs.rmSync(f)
})

test('запись с неверной CRC при верном sha — отказ', async () => {
  const zip = Buffer.from(GOOD())
  const p = zip.indexOf(Buffer.from([0x50, 0x4b, 0x01, 0x02]))
  // CRC первой записи (manifest.json) в каталоге
  zip.writeUInt32LE((zip.readUInt32LE(p + 16) ^ 0xdeadbeef) >>> 0, p + 16)
  await rejected(zip, /manifest\.json не читается/)
})

test('фаззинг: случайные порчи хорошего набора не бросают исключение и не зависают', async () => {
  const zip = GOOD()
  let seed = 12345
  const rnd = (n) => {
    seed = (Math.imul(seed, 1103515245) + 12345) >>> 0
    return seed % n
  }
  const verdicts = new Set(['ok', 'skip', 'reject'])
  for (let i = 0; i < 400; i++) {
    const b = Buffer.from(zip)
    for (let k = 0, m = 1 + rnd(4); k < m; k++) b[rnd(b.length)] = rnd(256)
    const r = await checkZip(put(i % 7 === 0 ? b.subarray(0, rnd(b.length)) : b, 'fuzz.zip'))
    assert.ok(verdicts.has(r.verdict), `итог ${r.verdict}`)
    if (r.verdict === 'reject') assert.equal(typeof r.error, 'string')
  }
})

test('фаззинг манифеста: порча байтов и усечение дают итог, а не исключение', () => {
  const good = Buffer.from(JSON.stringify({ format: 1, id: 'abc', version: 1, minApp: '0.1.0', title: 'Т', items: [{ file: 'a.png', kind: 'bg', theme: 'kids', name: 'я', sha256: 'a'.repeat(64), size: 5 }] }))
  let seed = 777
  const rnd = (n) => {
    seed = (Math.imul(seed, 1103515245) + 12345) >>> 0
    return seed % n
  }
  for (let i = 0; i < 2000; i++) {
    const b = Buffer.from(good)
    for (let k = 0, m = 1 + rnd(3); k < m; k++) b[rnd(b.length)] = rnd(256)
    const v = manifestVerdict(i % 5 === 0 ? b.subarray(0, rnd(b.length)) : b, i % 2 ? '0.31.0' : 'dev')
    assert.equal(typeof v.ok, 'boolean')
    if (!v.ok) assert.match(v.code, /^(newer|manifest|appOld|empty)$/)
  }
})

// ---- CLI ----

const cli = (...args) => spawnSync(process.execPath, [SCRIPT, ...args], { encoding: 'utf8' })

test('CLI: ставится — код 0; пропуск — 0, с --strict — 1; отказ — 1; без аргумента — 2', () => {
  const good = cli(put(GOOD()))
  assert.equal(good.status, 0, good.stderr)
  assert.match(good.stdout, /ставится\n$/)
  const skip = put(pack([BG, { file: 'bg/v.svg', kind: 'bg', data: Buffer.from('<svg/>') }]))
  const s = cli(skip)
  assert.equal(s.status, 0)
  assert.match(s.stdout, /пропуск: bg\/v\.svg — SVG/)
  assert.match(s.stdout, /ставится с пропусками/)
  assert.equal(cli(skip, '--strict').status, 1)
  const bad = cli(put(Buffer.from('не zip')))
  assert.equal(bad.status, 1)
  assert.match(bad.stdout, /^отказ: /)
  assert.equal(cli().status, 2)
  assert.equal(cli('--oops').status, 2)
  assert.equal(cli(put(GOOD()), '--app', '0.31.0').status, 0) // minApp набора 0.16.0 не новее программы
  const old = cli(put(GOOD()), '--app', '0.10.0')
  assert.equal(old.status, 1)
  assert.match(old.stdout, /нужна версия Sosnina Albums 0\.16\.0 или новее/)
})

test('CLI: путь к несуществующему файлу — отказ, а не исключение', () => {
  const r = cli(path.join(TMP, 'нет.zip'))
  assert.equal(r.status, 1)
  assert.match(r.stdout, /файл набора не открывается/)
  assert.equal(r.stderr, '')
})
