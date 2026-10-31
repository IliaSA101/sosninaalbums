// Тесты листа образцов в каталоге (спека 144 Г4, security Н5 и Н7): проверки листа и preview.json, поле `preview`
// строки, режим --preview-only (архивы и их sha256 не меняются). Подпись — тестовым ключом, как library-pack.test.mjs.
import assert from 'node:assert/strict'
import crypto from 'node:crypto'
import fs from 'node:fs'
import os from 'node:os'
import path from 'node:path'
import { test } from 'node:test'
import { spawnSync } from 'node:child_process'
import { fileURLToPath } from 'node:url'
import { URL_PREFIX, buildPack, makeZip, publicRaw, readCatalogFile, sealCatalog, verifyBytes } from './library-pack.mjs'
import { BODY_FAIL, BODY_WARN, MAX_PREVIEW, checkBody, jpegFrame, parseMeta, previewField, replacePreviews } from './library-pack-preview.mjs'

const SCRIPT = fileURLToPath(new URL('./library-pack.mjs', import.meta.url))
const RGB_JPEG =
  '/9j/2wCEAAYEBQYFBAYGBQYHBwYIChAKCgkJChQODwwQFxQYGBcUFhYaHSUfGhsjHBYWICwgIyYnKSopGR8tMC0oMCUoKSgBBwcHCggKEwoKEygaFhooKCgoKCgoKCgoKCgoKCgoKCgoKCgoKCgoKCgoKCgoKCgoKCgoKCgoKCgoKCgoKCgoKP/AABEIACAAQANSIgBHEQFCEQH/xAGiAAABBQEBAQEBAQAAAAAAAAAAAQIDBAUGBwgJCgsQAAIBAwMCBAMFBQQEAAABfQECAwAEEQUSITFBBhNRYQcicRQygZGhCCNCscEVUtHwJDNicoIJChYXGBkaJSYnKCkqNDU2Nzg5OkNERUZHSElKU1RVVldYWVpjZGVmZ2hpanN0dXZ3eHl6g4SFhoeIiYqSk5SVlpeYmZqio6Slpqeoqaqys7S1tre4ubrCw8TFxsfIycrS09TV1tfY2drh4uPk5ebn6Onq8fLz9PX29/j5+gEAAwEBAQEBAQEBAQAAAAAAAAECAwQFBgcICQoLEQACAQIEBAMEBwUEBAABAncAAQIDEQQFITEGEkFRB2FxEyIygQgUQpGhscEJIzNS8BVictEKFiQ04SXxFxgZGiYnKCkqNTY3ODk6Q0RFRkdISUpTVFVWV1hZWmNkZWZnaGlqc3R1dnd4eXqCg4SFhoeIiYqSk5SVlpeYmZqio6Slpqeoqaqys7S1tre4ubrCw8TFxsfIycrS09TV1tfY2dri4+Tl5ufo6ery8/T19vf4+fr/2gAMAwEAAhEDEQA/APlSiiigAooooAKKKKACiiigAooooAKKKKACiiigAooooA//2Q=='

const sha = (b) => crypto.createHash('sha256').update(b).digest('hex')
const tmp = () => fs.mkdtempSync(path.join(os.tmpdir(), 'libprev-'))

/** Заголовок JPEG без данных: SOI, APP0, SOF (marker, comps), SOS, EOI — для проверок кадра. */
const JFIF = Buffer.from([0xff, 0xe0, 0x00, 0x10, 0x4a, 0x46, 0x49, 0x46, 0, 1, 1, 0, 0, 1, 0, 1, 0, 0])
/** Сегмент Adobe APP14 с transform. */
const adobeSeg = (transform) => Buffer.concat([Buffer.from([0xff, 0xee, 0x00, 0x0e]), Buffer.from('Adobe'), Buffer.from([0, 100, 0, 0, 0, 0, transform])])

function jpegStub(w, h, { sof = 0xc0, comps = 3, precision = 8, extra = [], head = [JFIF], tail = [], ids = [1, 2, 3] } = {}) {
  const app = Buffer.concat(head)
  const sofSeg = (m) => {
    const b = Buffer.alloc(10 + 3 * comps)
    b.writeUInt16BE(0xff00 | m, 0)
    b.writeUInt16BE(8 + 3 * comps, 2)
    b[4] = precision
    b.writeUInt16BE(h, 5)
    b.writeUInt16BE(w, 7)
    b[9] = comps
    for (let i = 0; i < comps; i++) b.set([ids[i] ?? i + 1, 0x11, 0], 10 + 3 * i)
    return b
  }
  const sos = Buffer.from([0xff, 0xda, 0x00, 0x08, 0x01, 0x01, 0x00, 0x00, 0x3f, 0x00, 1, 2, 3, 0xff, 0xd9])
  return Buffer.concat([Buffer.from([0xff, 0xd8]), app, sofSeg(sof), ...extra.map((m) => sofSeg(m)), ...tail, sos])
}

/** Папка набора с двумя фонами и стикером; id и версия задают архив и ключи. */
function packDir(id, version, seed = id) {
  const d = tmp()
  fs.mkdirSync(path.join(d, 'bg'))
  fs.mkdirSync(path.join(d, 'stickers'))
  const png = (tag) => {
    const b = Buffer.alloc(700)
    crypto.createHash('sha256').update(tag).digest().copy(b, 33)
    Buffer.from([0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a]).copy(b)
    b.writeUInt32BE(13, 8)
    b.write('IHDR', 12, 'latin1')
    b.writeUInt32BE(64, 16)
    b.writeUInt32BE(64, 20)
    return b
  }
  fs.writeFileSync(path.join(d, 'bg', 'one.png'), png(seed + '1'))
  fs.writeFileSync(path.join(d, 'bg', 'two.png'), png(seed + '2'))
  fs.writeFileSync(path.join(d, 'stickers', 'dot.png'), png(seed + '3'))
  fs.writeFileSync(
    path.join(d, 'manifest.src.json'),
    JSON.stringify({
      id,
      version,
      minApp: '0.29.0',
      title: 'Тест ' + id,
      license: 'CC0-1.0',
      items: [
        { file: 'bg/one.png', kind: 'bg', theme: 'wedding', name: 'Один' },
        { file: 'bg/two.png', kind: 'bg', theme: 'wedding', name: 'Два' },
        { file: 'stickers/dot.png', kind: 'sticker', theme: 'wedding', name: 'Точка' },
      ],
    }),
  )
  return d
}

/** Лист и preview.json набора в папку sheets: ряд фонов и ряд стикеров по манифесту. */
function writeSheet(sheets, manifest, opts = {}) {
  const sums = (kind) => manifest.items.filter((it) => it.kind === kind).map((it) => it.sha256.slice(0, 16))
  const meta = opts.meta ?? { w: 480, h: 320, rows: [{ kind: 'bg', w: 240, h: 160, items: sums('bg') }, { kind: 'sticker', w: 160, h: 160, items: sums('sticker') }] }
  const jpeg = opts.jpeg ?? jpegStub(meta.w, meta.h)
  fs.mkdirSync(sheets, { recursive: true })
  const base = path.join(sheets, `${manifest.id}-${manifest.version}-preview`)
  fs.writeFileSync(`${base}.jpg`, jpeg)
  fs.writeFileSync(`${base}.json`, JSON.stringify(meta))
  return { jpeg, meta }
}

/** Рабочая папка: ключ, каталог из наборов ids (zip сохранён, как опубликованный), папка листов. */
function env(ids) {
  const w = tmp()
  const { privateKey } = crypto.generateKeyPairSync('ed25519')
  fs.writeFileSync(path.join(w, 'key.pem'), privateKey.export({ type: 'pkcs8', format: 'pem' }))
  const out = path.join(w, 'out')
  const run = (...args) => {
    const r = spawnSync(process.execPath, [SCRIPT, ...args, '--key', path.join(w, 'key.pem')], { encoding: 'utf8' })
    return { code: r.status, text: r.stdout + r.stderr }
  }
  const sets = ids.map((id, i) => {
    const r = run(packDir(id, 1), '--out', out, ...(i === 0 ? ['--new'] : []))
    assert.equal(r.code, 0, r.text)
    const zipFile = path.join(out, `${id}-1.zip`)
    const zip = fs.readFileSync(zipFile)
    const { manifest } = buildPack(packDir(id, 1))
    return { id, zipFile, zip, manifest }
  })
  const catFile = path.join(out, 'catalog.json')
  return { w, out, run, sets, catFile, pub: publicRaw(privateKey), sheets: path.join(w, 'sheets'), cat: () => readCatalogFile(catFile) }
}

const only = (...pairs) => pairs.flatMap((s) => ['--preview-only', `${s.id}=${s.zipFile}`])

// ---- jpegFrame ----

test('jpegFrame: baseline YCbCr и серый — размеры кадра; прогрессивный, CMYK, иная точность, второй кадр — отказ', () => {
  assert.deepEqual(jpegFrame(jpegStub(1920, 576)), { marker: 0xc0, w: 1920, h: 576, comps: 3 })
  assert.equal(jpegFrame(jpegStub(8, 8, { sof: 0xc1 })).marker, 0xc1)
  assert.equal(jpegFrame(jpegStub(8, 8, { comps: 1 })).comps, 1)
  assert.throws(() => jpegFrame(jpegStub(8, 8, { sof: 0xc2 })), /не baseline/) // прогрессивный
  assert.throws(() => jpegFrame(jpegStub(8, 8, { sof: 0xc3 })), /не baseline/)
  assert.throws(() => jpegFrame(jpegStub(8, 8, { comps: 4 })), /компонентов 4/) // CMYK
  assert.throws(() => jpegFrame(jpegStub(8, 8, { comps: 2 })), /компонентов 2/)
  assert.throws(() => jpegFrame(jpegStub(8, 8, { precision: 12 })), /8 бит/)
  assert.throws(() => jpegFrame(jpegStub(8, 8, { extra: [0xc0] })), /второй кадр/)
})

test('jpegFrame: пустой, усечённый, без SOF, битая длина, лишние маркеры и случайные входы — исключение, не паника и не зависание', () => {
  const good = jpegStub(480, 320)
  for (const bad of [Buffer.alloc(0), Buffer.from([0xff]), Buffer.from([0xff, 0xd8]), Buffer.from('не JPEG'), good.subarray(0, 12), good.subarray(0, 25), Buffer.from([0xff, 0xd8, 0xff, 0xda, 0, 2, 0xff, 0xd9])]) {
    assert.throws(() => jpegFrame(bad), Error, `${bad.length} байт`)
  }
  const len0 = Buffer.from(good)
  len0.writeUInt16BE(0, 4) // длина APP0 меньше 2
  assert.throws(() => jpegFrame(len0), /длина сегмента/)
  const huge = Buffer.from(good)
  huge.writeUInt16BE(0xffff, 4) // длина вылезает за буфер
  assert.throws(() => jpegFrame(huge), /длина сегмента/)
  // не больше 64 сегментов до развёртки, SOS считается: APP×62 + SOF + SOS = 64 — годно, APP×63 + SOF + SOS = 65 — отказ
  const app = Buffer.from([0xff, 0xe1, 0x00, 0x02])
  const withApps = (k) => Buffer.concat([Buffer.from([0xff, 0xd8]), ...Array(k).fill(app), jpegStub(8, 8).subarray(2 + JFIF.length)])
  assert.equal(jpegFrame(withApps(62)).w, 8)
  assert.throws(() => jpegFrame(withApps(63)), /больше 64 сегментов/)
  assert.throws(() => jpegFrame(withApps(70)), /больше 64 сегментов/)
  // случайные и мутированные входы: только Error (или годный результат), конечное время
  for (let i = 0; i < 600; i++) {
    const b = i % 2 ? crypto.randomBytes(1 + (i % 90)) : Buffer.from(good)
    if (i % 2 === 0) b[i % b.length] = crypto.randomBytes(1)[0]
    try {
      jpegFrame(b)
    } catch (e) {
      assert.ok(e instanceof Error && !(e instanceof RangeError), e.message)
    }
  }
})

test('jpegFrame: до SOS маркеры без длины (FF 00, FF 01, FF D8, FF D0…D7, FF D9) — отказ, как в Go; FF 00 с длиной не прячет кадр', () => {
  const body = jpegStub(8, 8).subarray(2 + JFIF.length) // SOF0 + SOS
  const withBefore = (...mid) => Buffer.concat([Buffer.from([0xff, 0xd8]), ...mid, body])
  // обход ревью: FF 00 с длиной, накрывающей чужие SOF2+SOS; настоящий baseline-кадр идёт следом
  const hidden = jpegStub(8, 8, { sof: 0xc2 }).subarray(2 + JFIF.length)
  const cover = Buffer.from([0xff, 0x00, 0, 0])
  cover.writeUInt16BE(2 + hidden.length, 2)
  assert.throws(() => jpegFrame(withBefore(cover, hidden)), /маркер без длины/)
  for (const m of [0x00, 0x01, 0xd0, 0xd7, 0xd8, 0xd9]) {
    assert.throws(() => jpegFrame(withBefore(Buffer.from([0xff, m, 0x00, 0x02]))), /маркер без длины/, `FF ${m.toString(16)}`)
  }
  assert.equal(jpegFrame(withBefore(Buffer.from([0xff, 0xdb, 0x00, 0x02]))).w, 8) // обычный сегмент с длиной проходит
})

test('jpegFrame: три компонента, которые Go прочёл бы как RGB (Adobe transform 0, id R,G,B без JFIF) — отказ; JFIF, Adobe YCbCr и серый — годны', () => {
  const rgb = [0x52, 0x47, 0x42]
  const none = (head) => jpegStub(8, 8, { head })
  assert.throws(() => jpegFrame(jpegStub(8, 8, { head: [adobeSeg(0)] })), /RGB/) // Adobe transform 0
  assert.throws(() => jpegFrame(jpegStub(8, 8, { head: [], ids: rgb })), /RGB/) // id R,G,B, нет JFIF
  assert.throws(() => jpegFrame(jpegStub(8, 8, { head: [], tail: [adobeSeg(0)] })), /RGB/) // Adobe после кадра, до SOS — Go читает
  assert.equal(jpegFrame(jpegStub(8, 8, { head: [adobeSeg(0), JFIF] })).comps, 3) // JFIF до кадра — Go дальше не читает
  assert.equal(jpegFrame(jpegStub(8, 8, { head: [adobeSeg(0)], tail: [JFIF] })).comps, 3) // JFIF после кадра, до SOS — тоже YCbCr
  assert.equal(jpegFrame(jpegStub(8, 8, { head: [adobeSeg(1)] })).comps, 3) // Adobe YCbCr
  assert.equal(jpegFrame(none([])).comps, 3) // без APP: id 1,2,3 — YCbCr
  assert.equal(jpegFrame(jpegStub(8, 8, { head: [JFIF], ids: rgb })).comps, 3) // JFIF сильнее id
  assert.equal(jpegFrame(jpegStub(8, 8, { comps: 1, head: [adobeSeg(0)] })).comps, 1) // серый не RGB
  // последний APP0 решает (как в Go, сверено с image/jpeg.DecodeConfig): не-JFIF после JFIF до кадра сбрасывает признак
  const notJfif = Buffer.from(JFIF)
  notJfif[7] = 0x58
  assert.throws(() => jpegFrame(jpegStub(8, 8, { head: [JFIF, notJfif], ids: rgb })), /RGB/)
  assert.equal(jpegFrame(jpegStub(8, 8, { head: [JFIF], tail: [notJfif], ids: rgb })).comps, 3) // после кадра с JFIF до него — не читается
  // настоящий кодировщик: id компонентов R,G,B, без JFIF (625 байт)
  assert.throws(() => jpegFrame(Buffer.from(RGB_JPEG, 'base64')), /RGB/)
})

// ---- parseMeta / previewField ----

const okRow = (kind = 'bg', items = ['0123456789abcdef']) => ({ kind, w: 240, h: 160, items })
const okMeta = () => ({ w: 480, h: 320, rows: [okRow(), okRow('sticker', ['fedcba9876543210'])] })

test('parseMeta: годное читается; каждый предел и тип — отказ, чужое поле и не-объект — отказ', () => {
  assert.deepEqual(parseMeta(JSON.stringify(okMeta())), okMeta())
  const bad = (mut, re) => {
    const m = okMeta()
    mut(m)
    assert.throws(() => parseMeta(JSON.stringify(m)), re)
  }
  bad((m) => (m.w = 4097), /вне пределов/)
  bad((m) => (m.h = 2049), /вне пределов/)
  bad((m) => ((m.w = 4096), (m.h = 1025)), /вне пределов/) // больше 4 Мп
  bad((m) => (m.w = 0), /вне пределов/)
  bad((m) => (m.w = '480'), /вне пределов/)
  bad((m) => (m.h = 320.5), /вне пределов/)
  bad((m) => (m.rows = []), /рядов 1…5/)
  bad((m) => (m.rows = {}), /рядов 1…5/)
  bad((m) => (m.rows = Array.from({ length: 6 }, (_, i) => okRow(['bg', 'sticker', 'frame', 'spread', 'album', 'bg'][i]))), /рядов 1…5/)
  bad((m) => (m.rows[1].kind = 'bg'), /повторён/)
  bad((m) => (m.rows[1].kind = 'print'), /неизвестен/)
  bad((m) => (m.rows[0].w = 31), /клетка/)
  bad((m) => (m.rows[0].w = 481), /клетка/)
  bad((m) => (m.rows[0].h = '160'), /клетка/)
  bad((m) => (m.rows[0].items = []), /items 1…12/)
  bad((m) => ((m.w = 4096), (m.rows[0].items = Array.from({ length: 13 }, (_, i) => i.toString(16).padStart(16, '0')))), /items 1…12/)
  bad((m) => (m.rows[0].items = [1]), /16 hex/)
  bad((m) => (m.rows[0].items = ['ABCDEF0123456789']), /16 hex/)
  bad((m) => (m.rows[0].items = ['0123456789abcdef', '0123456789abcdef']), /без повторов/)
  bad((m) => (m.rows[0].items = Array.from({ length: 3 }, (_, i) => i.toString(16).padStart(16, '0'))), /не помещаются/) // 3·240 > 480
  bad((m) => (m.h = 319), /не помещаются/) // Σh = 320
  bad((m) => (m.extra = 1), /неизвестное поле extra/)
  bad((m) => (m.rows[0].name = 'x'), /неизвестное поле name/)
  for (const raw of ['', 'null', '[]', '1', '"x"', '{', '{"w":1,"h":1,"rows":null}']) assert.throws(() => parseMeta(raw), Error, raw)
  // Мутации годного текста: либо годный разбор, либо Error — не паника другого вида.
  const text = Buffer.from(JSON.stringify(okMeta()))
  for (let i = 0; i < 500; i++) {
    const b = Buffer.from(text)
    b[(i * 7) % b.length] = crypto.randomBytes(1)[0]
    try {
      parseMeta(b.toString('utf8'))
    } catch (e) {
      assert.ok(e instanceof Error && !(e instanceof TypeError), e.message)
    }
  }
})

test('previewField: поле строки — sha256 и размер считает сам; w×h не от листа, прогрессивный, items не из манифеста, размер — отказ', () => {
  const { manifest } = buildPack(packDir('field-set', 3))
  const sums = (k) => manifest.items.filter((it) => it.kind === k).map((it) => it.sha256.slice(0, 16))
  const meta = { w: 480, h: 320, rows: [okRow('bg', sums('bg')), okRow('sticker', sums('sticker'))] }
  meta.rows[1].w = meta.rows[1].h = 160
  const jpeg = jpegStub(480, 320)
  const f = previewField(jpeg, JSON.stringify(meta), manifest, URL_PREFIX)
  assert.deepEqual(Object.keys(f), ['url', 'sha256', 'size', 'w', 'h', 'rows'])
  assert.equal(f.url, `${URL_PREFIX}field-set-v3/field-set-3-preview.jpg`)
  assert.equal(f.sha256, sha(jpeg))
  assert.equal(f.size, jpeg.length)
  assert.deepEqual(f.rows, meta.rows)
  const fails = (j, m, re) => assert.throws(() => previewField(j, JSON.stringify(m), manifest, URL_PREFIX), re)
  fails(jpegStub(481, 320), meta, /кадр JPEG 481×320/) // w×h листа не те, что в preview.json
  fails(jpegStub(480, 321), meta, /кадр JPEG 480×321/)
  fails(jpegStub(480, 320, { sof: 0xc2 }), meta, /не baseline/)
  fails(jpegStub(480, 320, { comps: 4 }), meta, /компонентов 4/)
  fails(Buffer.alloc(0), meta, /вне 1…/)
  fails(Buffer.concat([jpeg, Buffer.alloc(MAX_PREVIEW)]), meta, /вне 1…/)
  fails(Buffer.from('GIF89a'), meta, /не JPEG/)
  const stray = structuredClone(meta)
  stray.rows[0].items[0] = '0'.repeat(16) // нет такого предмета
  fails(jpeg, stray, /нет в манифесте/)
  const kind = structuredClone(meta) // ключ есть, но у предмета другой вид
  kind.rows[0].items = sums('sticker')
  fails(jpeg, kind, /нет в манифесте/)
  fails(jpeg, { ...meta, rows: [] }, /рядов 1…5/)
})

test('checkBody: тело каталога — предупреждение от 512 КиБ, отказ от 768 КиБ', () => {
  const body = (n) => ({ format: 1, seq: 1, packs: [], pad: 'x'.repeat(n - 40) })
  assert.equal(checkBody(body(1000)), '')
  assert.match(checkBody(body(BODY_WARN + 10)), /близко/)
  assert.throws(() => checkBody(body(BODY_FAIL + 100)), /больше 768 КиБ/)
})

// ---- обычный выпуск с --sheets ----

test('CLI --sheets: строка получает preview; без листа — без поля; плохой лист — отказ до записи', () => {
  const e = env([])
  const dir = packDir('plain-set', 1)
  const { manifest } = buildPack(dir)
  const miss = e.run(dir, '--new', '--out', e.out, '--sheets', e.sheets) // листа нет
  assert.equal(miss.code, 1)
  assert.deepEqual(fs.existsSync(e.out) ? fs.readdirSync(e.out) : [], [])
  writeSheet(e.sheets, manifest, { jpeg: jpegStub(480, 321) }) // кадр не того размера
  const bad = e.run(dir, '--new', '--out', e.out, '--sheets', e.sheets)
  assert.equal(bad.code, 1)
  assert.match(bad.text, /кадр JPEG 480×321/)
  assert.deepEqual(fs.existsSync(e.out) ? fs.readdirSync(e.out) : [], []) // ни zip, ни каталога
  const { jpeg } = writeSheet(e.sheets, manifest)
  const ok = e.run(dir, '--new', '--out', e.out, '--sheets', e.sheets)
  assert.equal(ok.code, 0, ok.text)
  const row = e.cat().packs[0]
  assert.equal(row.preview.sha256, sha(jpeg))
  assert.equal(row.preview.url, `${URL_PREFIX}plain-set-v1/plain-set-1-preview.jpg`)
  const noSheet = env([])
  assert.equal(noSheet.run(packDir('plain-set', 1), '--new', '--out', noSheet.out).code, 0)
  assert.equal('preview' in noSheet.cat().packs[0], false) // без --sheets поля нет
})

// ---- --preview-only ----

test('--preview-only: две пары — один прогон, один seq+1; меняется только preview; архивы и sha256 прежние; подпись сходится', () => {
  const e = env(['first-set', 'second-set', 'third-set'])
  // Прежний каталог несёт ролики: они, как и нетронутая третья строка, переходят байт в байт.
  const base = { ...e.cat(), videos: [{ id: 'filter', version: 1 }] }
  const key = crypto.createPrivateKey(fs.readFileSync(path.join(e.w, 'key.pem')))
  fs.writeFileSync(e.catFile, JSON.stringify(sealCatalog(base, key)) + '\n')
  const [a, b] = e.sets
  writeSheet(e.sheets, a.manifest)
  writeSheet(e.sheets, b.manifest)
  const thirdBefore = JSON.stringify(base.packs[2])
  const filesBefore = fs.readdirSync(e.out).sort()
  const zipsBefore = e.sets.map((s) => sha(fs.readFileSync(s.zipFile)))
  const r = e.run(...only(a, b), '--sheets', e.sheets, '--catalog', e.catFile)
  assert.equal(r.code, 0, r.text)
  // Архивы: ни новых файлов, ни изменённых байтов, sha256 в строках каталога прежние.
  assert.deepEqual(fs.readdirSync(e.out).sort(), filesBefore)
  assert.deepEqual(e.sets.map((s) => sha(fs.readFileSync(s.zipFile))), zipsBefore)
  const after = e.cat()
  assert.equal(after.seq, base.seq + 1) // две пары — один seq
  assert.deepEqual(after.videos, base.videos)
  assert.deepEqual(after.packs.map((p) => p.sha256), base.packs.map((p) => p.sha256))
  after.packs.forEach((p, i) => {
    const { preview, ...rest } = p
    assert.deepEqual(rest, base.packs[i]) // прочие поля — как были
    assert.equal(preview !== undefined, i < 2)
  })
  assert.equal(after.packs[0].preview.sha256, sha(fs.readFileSync(path.join(e.sheets, 'first-set-1-preview.jpg'))))
  assert.deepEqual(Object.keys(after.packs[0]).slice(0, -1), Object.keys(base.packs[0])) // preview — в конец строки
  assert.deepEqual(after.packs[2], base.packs[2])
  assert.equal(JSON.stringify(after.packs[2]), thirdBefore) // байты нетронутой строки
  // Подпись по точным байтам body проверяется ключом прогона.
  const env2 = JSON.parse(fs.readFileSync(e.catFile, 'utf8'))
  assert.equal(verifyBytes(Buffer.from(env2.body, 'base64'), env2.sig, e.pub), true)
  // Круг: повтор с теми же листами — поле на месте (не дубль), seq ещё +1, строки прежние.
  const again = e.run(...only(a), '--sheets', e.sheets, '--catalog', e.catFile)
  assert.equal(again.code, 0, again.text)
  assert.deepEqual(e.cat().packs, after.packs)
  assert.equal(e.cat().seq, base.seq + 2)
  // Один набор из каталога — тоже работает (край: одна пара) и --seq больше прежнего принят.
  assert.equal(e.run(...only(b), '--sheets', e.sheets, '--catalog', e.catFile, '--seq', '50').code, 0)
  assert.equal(e.cat().seq, 50)
})

test('--preview-only: чужой архив, неизвестный id, повтор, нет листа, плохой лист — отказ, причины всех пар сразу, каталог не тронут', () => {
  const e = env(['one-set', 'two-set'])
  const [a, b] = e.sets
  writeSheet(e.sheets, a.manifest)
  const before = fs.readFileSync(e.catFile, 'utf8')
  const stranger = path.join(e.w, 'stranger.zip') // тот же id, другие байты
  fs.writeFileSync(stranger, makeZip(buildPack(packDir('one-set', 1, 'другое')).files))
  const run = (...args) => e.run(...args, '--sheets', e.sheets, '--catalog', e.catFile)
  const notSame = run('--preview-only', `one-set=${stranger}`)
  assert.equal(notSame.code, 1)
  assert.match(notSame.text, /sha256 архива не совпадает/)
  assert.match(run('--preview-only', `ghost-set=${a.zipFile}`).text, /ghost-set: набора ghost-set нет в каталоге/)
  assert.match(run('--preview-only', `two-set=${a.zipFile}`).text, /архив — набор one-set v1, в каталоге two-set/) // zip не того набора
  // Лист второго набора отсутствует, а у первого — прогрессивный: оба провала в одном прогоне, ничего не записано.
  writeSheet(e.sheets, a.manifest, { jpeg: jpegStub(480, 320, { sof: 0xc2 }) })
  const both = run(...only(a, b))
  assert.equal(both.code, 1)
  assert.match(both.text, /one-set: .*не baseline/)
  assert.match(both.text, /two-set: .*ENOENT/)
  assert.match(both.text, /отказ \(2\)/)
  assert.equal(fs.readFileSync(e.catFile, 'utf8'), before)
  // Предмет листа не из манифеста этого zip.
  writeSheet(e.sheets, a.manifest, { meta: { w: 480, h: 160, rows: [okRow('bg', ['0'.repeat(16)])] }, jpeg: jpegStub(480, 160) })
  assert.match(run(...only(a)).text, /нет в манифесте/)
  // Командная строка: повтор id, пара без «=», пустой набор пар, нет --sheets/--catalog, папка набора, неизвестный ключ — код 2.
  for (const args of [
    ['--preview-only', `one-set=${a.zipFile}`, '--preview-only', `one-set=${a.zipFile}`],
    ['--preview-only', a.zipFile],
    ['--preview-only', `one-set=${a.zipFile}`, '--bogus', 'x'],
    ['--preview-only', `one-set=${a.zipFile}`, packDir('dir-set', 1)],
    ['--preview-only'],
  ]) {
    const r = run(...args)
    assert.equal(r.code, 2, args.join(' ') + '\n' + r.text)
  }
  assert.equal(e.run('--preview-only', `one-set=${a.zipFile}`, '--catalog', e.catFile).code, 2) // нет --sheets
  assert.equal(e.run('--preview-only', `one-set=${a.zipFile}`, '--sheets', e.sheets).code, 2) // нет --catalog
  assert.equal(e.run('--preview-only', `one-set=${a.zipFile}`, '--sheets', e.sheets, '--catalog', path.join(e.w, 'nope.json')).code, 2)
  assert.equal(fs.readFileSync(e.catFile, 'utf8'), before)
})

test('--preview-only: пустой, усечённый и огромный лист или zip — ошибка, не паника; seq не больше прежнего — отказ', () => {
  const e = env(['edge-set'])
  const [a] = e.sets
  const before = fs.readFileSync(e.catFile, 'utf8')
  const args = [...only(a), '--sheets', e.sheets, '--catalog', e.catFile]
  const base = path.join(e.sheets, 'edge-set-1-preview')
  const good = writeSheet(e.sheets, a.manifest)
  for (const [name, data] of [['пустой', Buffer.alloc(0)], ['усечённый', good.jpeg.subarray(0, 15)], ['огромный', Buffer.alloc(MAX_PREVIEW + 1, 0xff)]]) {
    fs.writeFileSync(`${base}.jpg`, data)
    const r = e.run(...args)
    assert.equal(r.code, 1, name + r.text)
    assert.doesNotMatch(r.text, /TypeError|RangeError|at .*\.mjs:\d+/, name) // сообщение, не стек
  }
  writeSheet(e.sheets, a.manifest)
  for (const [name, data] of [['пустой json', ''], ['битый json', '{"w":'], ['огромный json', ' '.repeat(65 << 10)]]) {
    fs.writeFileSync(`${base}.json`, data)
    assert.equal(e.run(...args).code, 1, name)
  }
  writeSheet(e.sheets, a.manifest)
  const cut = path.join(e.w, 'cut.zip')
  fs.writeFileSync(cut, a.zip.subarray(0, a.zip.length >> 1))
  assert.equal(e.run('--preview-only', `edge-set=${cut}`, '--sheets', e.sheets, '--catalog', e.catFile).code, 1)
  assert.equal(e.run('--preview-only', `edge-set=${path.join(e.w, 'empty')}`, '--sheets', e.sheets, '--catalog', e.catFile).code, 1)
  assert.equal(e.run(...args, '--seq', String(e.cat().seq)).code, 1) // seq не больше прежнего
  assert.equal(e.run(...args, '--seq', '0').code, 1)
  assert.equal(fs.readFileSync(e.catFile, 'utf8'), before)
})

test('replacePreviews: меняется только preview; чужое изменение строки — внутренняя ошибка; порядок строк и прочие поля сохранены', () => {
  const rows = ['a-set', 'b-set', 'c-set'].map((id) => ({ id, version: 1, sha256: sha(id), counts: { bg: 1 } }))
  const base = { format: 1, seq: 4, packs: rows, videos: [] }
  const field = { url: 'u', sha256: 'h', size: 1, w: 1, h: 1, rows: [] }
  const next = replacePreviews(base, new Map([['b-set', field]]), 5)
  assert.equal(next.seq, 5)
  assert.deepEqual(next.packs.map((p) => p.id), ['a-set', 'b-set', 'c-set'])
  assert.deepEqual(next.packs[1].preview, field)
  assert.deepEqual(next.packs[0], rows[0])
  assert.deepEqual(next.videos, [])
  assert.equal('preview' in base.packs[1], false) // исходный каталог не изменён
  assert.throws(() => replacePreviews(base, new Map([['x-set', field]]), 5), /нет в каталоге/)
  assert.equal(replacePreviews(base, new Map(), 5).packs.length, 3) // нет пар — строки те же
  // подмена: строка, чьё поле при каждом чтении иное, — сверка «прочее не тронуто» ловит и отказывает до подписи
  let n = 0
  const drift = { id: 'a-set', version: 1, get counts() { return { bg: ++n } } }
  assert.throws(() => replacePreviews({ ...base, packs: [drift] }, new Map(), 5), /внутренняя ошибка/)
})
