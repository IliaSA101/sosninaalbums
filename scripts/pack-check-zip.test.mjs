// Тесты scripts/pack-check.mjs, часть 2 (спека 143, ступень 1; ревью С1): архив как у Go (testdata/pack-archive-cases.json,
// TestArchiveCases), пределы (блок limits файла pack-manifest-cases.json), все предметы пропущены, дубли, версия Node,
// ошибка чтения посреди записи, дескриптор данных. Запуск: node --test scripts/pack-check-zip.test.mjs
import assert from 'node:assert/strict'
import crypto from 'node:crypto'
import fs from 'node:fs'
import os from 'node:os'
import path from 'node:path'
import { test } from 'node:test'
import { spawnSync } from 'node:child_process'
import { fileURLToPath, pathToFileURL } from 'node:url'
import zlib from 'node:zlib'
import { makeZip } from './library-pack.mjs'
import { LIMITS, checkZip, nodeProblem, report } from './pack-check.mjs'

const HERE = path.dirname(fileURLToPath(import.meta.url))
const SCRIPT = path.join(HERE, 'pack-check.mjs')
const TESTDATA = path.resolve(HERE, '..', 'testdata')
const sha = (b) => crypto.createHash('sha256').update(b).digest('hex')
const TMP = fs.mkdtempSync(path.join(os.tmpdir(), 'packcheck2-'))
test.after(() => fs.rmSync(TMP, { recursive: true, force: true }))
let seq = 0
const put = (buf, name = `z${seq++}.zip`) => {
  const f = path.join(TMP, name)
  fs.writeFileSync(f, buf)
  return f
}

// ---- архив: общие случаи с Go ----

/**
 * Архив случая testdata/pack-archive-cases.json (те же правила сборки, что caseArchive в archive_cases_test.go): манифест,
 * files, extra — всё Store; prefix нулей перед zip; eocdRecords — число записей в конце каталога вместо настоящего, dirSizeMinus — на сколько размер каталога в конце меньше настоящего.
 */
function caseZip(c, dirOver = {}) {
  const items = c.files.map((f) => ({ file: f.name, kind: 'spread', theme: 'basic', name: 'n', sha256: sha(Buffer.from(f.text)), size: Buffer.byteLength(f.text) }))
  const manifest = Buffer.from(JSON.stringify({ format: 1, id: 'arch-test', version: 1, minApp: '0.1.0', title: 'Т', items }))
  const list = [{ name: Buffer.from('manifest.json'), data: manifest }]
  for (const f of [...c.files, ...(c.extra ?? [])]) list.push({ name: f.nameHex ? Buffer.from(f.nameHex, 'hex') : Buffer.from(f.name), data: Buffer.from(f.text), madeBy: f.madeBy, attrs: f.attrs, dd: f.dd, ...dirOver })
  return zipFrom(list, { prefix: c.prefix ?? 0, records: c.eocdRecords, dirSizeMinus: c.dirSizeMinus ?? 0 })
}

/** Zip из записей {name: Buffer, data, madeBy, attrs, dd}; имена байтами, потому что cp866 и негодный UTF-8 нужны как есть. */
function zipFrom(list, { prefix = 0, records, dirSizeMinus = 0 } = {}) {
  const parts = [Buffer.alloc(prefix)]
  const central = []
  let off = 0
  for (const e of list) {
    const flags = e.dd ? 8 : 0
    const crc = zlib.crc32(e.data)
    const lh = Buffer.alloc(30)
    lh.writeUInt32LE(0x04034b50, 0)
    lh.writeUInt16LE(20, 4)
    lh.writeUInt16LE(flags, 6)
    lh.writeUInt16LE(0x21, 12)
    lh.writeUInt32LE(crc, 14)
    lh.writeUInt32LE(e.data.length, 18)
    lh.writeUInt32LE(e.data.length, 22)
    lh.writeUInt16LE(e.name.length, 26)
    parts.push(lh, e.name, e.data)
    if (e.dd) {
      const d = Buffer.alloc(16)
      d.writeUInt32LE(0x08074b50, 0)
      d.writeUInt32LE(e.ddCrc ?? crc, 4)
      d.writeUInt32LE(e.data.length, 8)
      d.writeUInt32LE(e.data.length, 12)
      parts.push(d)
    }
    const ch = Buffer.alloc(46)
    ch.writeUInt32LE(0x02014b50, 0)
    ch.writeUInt16LE(e.madeBy ?? 20, 4)
    ch.writeUInt16LE(20, 6)
    ch.writeUInt16LE(flags, 8)
    ch.writeUInt16LE(0x21, 14)
    ch.writeUInt32LE(crc, 16)
    ch.writeUInt32LE(e.data.length, 20)
    ch.writeUInt32LE(e.data.length, 24)
    ch.writeUInt16LE(e.name.length, 28)
    ch.writeUInt32LE(e.attrs ?? 0, 38)
    ch.writeUInt32LE(off, 42)
    central.push(ch, e.name)
    off += 30 + e.name.length + e.data.length + (e.dd ? 16 : 0)
  }
  const cd = Buffer.concat(central)
  const end = Buffer.alloc(22)
  end.writeUInt32LE(0x06054b50, 0)
  end.writeUInt16LE(records ?? list.length, 8)
  end.writeUInt16LE(records ?? list.length, 10)
  end.writeUInt32LE(cd.length - dirSizeMinus, 12)
  end.writeUInt32LE(off, 16)
  return Buffer.concat([...parts, cd, end])
}

test('архив: каждый случай даёт тот же итог, что Go (TestArchiveCases)', async () => {
  const { cases } = JSON.parse(fs.readFileSync(path.join(TESTDATA, 'pack-archive-cases.json'), 'utf8'))
  assert.ok(cases.length >= 20, `случаев ${cases.length}: файл урезан`)
  const names = new Set()
  const fails = []
  for (const c of cases) {
    assert.ok(!names.has(c.name), `случай «${c.name}» повторяется`)
    names.add(c.name)
    const r = await checkZip(put(caseZip(c), 'case.zip'))
    const ok = r.verdict !== 'reject'
    if (ok !== c.want.ok) fails.push(`${c.name}: итог ${r.verdict} «${r.error ?? ''}», ждали ${c.want.ok ? 'успех' : 'отказ'}`)
    else if (!ok && !r.error.includes(c.want.error)) fails.push(`${c.name}: отказ «${r.error}», ждали «${c.want.error}»`)
  }
  assert.deepEqual(fails, [])
})

test('пределы: LIMITS скрипта равны блоку limits файла случаев (его же сверяют TestLimits и TestPackLimits с Go)', () => {
  const { limits } = JSON.parse(fs.readFileSync(path.join(TESTDATA, 'pack-manifest-cases.json'), 'utf8'))
  assert.deepEqual(JSON.parse(JSON.stringify(LIMITS)), limits)
})

test('дескриптор данных: подпись необязательна, CRC в нём обязан совпасть с каталогом', async () => {
  const manifest = (data) => Buffer.from(JSON.stringify({ format: 1, id: 'arch-test', version: 1, minApp: '0.1.0', title: 'Т', items: [{ file: 'sp/a.json', kind: 'spread', theme: 'basic', name: 'n', sha256: sha(data), size: data.length }] }))
  const data = Buffer.from('{"v":1}')
  const mk = (over) => zipFrom([{ name: Buffer.from('manifest.json'), data: manifest(data) }, { name: Buffer.from('sp/a.json'), data, dd: true, ...over }])
  assert.equal((await checkZip(put(mk({})))).verdict, 'ok')
  const bad = await checkZip(put(mk({ ddCrc: 1 })))
  assert.equal(bad.verdict, 'reject')
  assert.match(bad.error, /не сходится с манифестом по размеру/)
  // флаг 8 в каталоге, а дескриптора в файле нет: на его месте следующая запись — Go сверяет её байты как CRC и отказывает
  const noDesc = zipFrom([{ name: Buffer.from('manifest.json'), data: manifest(data) }, { name: Buffer.from('sp/a.json'), data }])
  const p = noDesc.indexOf(Buffer.from([0x50, 0x4b, 0x01, 0x02]), noDesc.indexOf(Buffer.from([0x50, 0x4b, 0x01, 0x02])) + 4)
  noDesc.writeUInt16LE(8, p + 8)
  assert.equal((await checkZip(put(noDesc))).verdict, 'reject')
})

test('набор, написанный Go (archive/zip: флаг 8, дескриптор с подписью), проходит', async () => {
  const f = path.join(HERE, '..', 'internal', 'decor', 'pack', 'testdata', 'd1-ivory.zip')
  const r = await checkZip(f)
  assert.notEqual(r.verdict, 'reject', r.error)
})

// ---- все предметы пропущены, дубли, содержимое ----

const png = (w, h) => {
  const b = Buffer.alloc(300, 7)
  Buffer.from([0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a]).copy(b)
  b.write('IHDR', 12, 'latin1')
  b.writeUInt32BE(w, 16)
  b.writeUInt32BE(h, 20)
  return b
}
function pack(items, over = {}) {
  const manifest = {
    format: 1,
    id: 'test-pack',
    version: 1,
    minApp: '0.16.0',
    title: 'Тестовый набор',
    items: items.map((i) => ({ file: i.file, kind: i.kind, theme: 'basic', name: i.file, sha256: sha(i.data), size: i.data.length })),
    ...over,
  }
  return makeZip([{ name: 'manifest.json', data: Buffer.from(JSON.stringify(manifest)) }, ...items.map((i) => ({ name: i.file, data: i.data }))])
}
const cli = (...args) => spawnSync(process.execPath, [SCRIPT, ...args], { encoding: 'utf8' })

test('все предметы отсеяны по содержимому — отказ, как ErrPackEmpty (а не «ставится 0»)', async () => {
  const printOnly = put(pack([{ file: 'p/t.saprint', kind: 'print', data: Buffer.from('{}') }]))
  const r = await checkZip(printOnly)
  assert.equal(r.verdict, 'reject')
  assert.match(r.error, /^в наборе нечего ставить: p\/t\.saprint — типографию ставит только набор из каталога/)
  const stickers = put(pack([{ file: 'st/a.png', kind: 'sticker', data: png(5000, 10) }, { file: 'st/b.png', kind: 'sticker', data: Buffer.from('не картинка') }]))
  const s = await checkZip(stickers)
  assert.equal(s.verdict, 'reject')
  assert.match(s.error, /нечего ставить: st\/a\.png — стикер больше 4096 px по стороне \(и ещё 1\)/)
  const c = cli(stickers)
  assert.equal(c.status, 1)
  assert.match(c.stdout, /^отказ: в наборе нечего ставить/)
  // хоть один предмет ставится — прежний итог «с пропусками»
  const some = await checkZip(put(pack([{ file: 'st/a.png', kind: 'sticker', data: png(5000, 10) }, { file: 'st/b.png', kind: 'sticker', data: png(10, 10) }])))
  assert.equal(some.verdict, 'skip')
})

test('содержимое: повтор предмета, шаблон и макет из новой версии, макет длиннее 24 шагов, шаблон без v', async () => {
  const j = (o) => Buffer.from(JSON.stringify(o))
  const same = png(20, 20)
  const steps = (n) => j({ v: 1, steps: Array.from({ length: n }, () => ({})) })
  const r = await checkZip(
    put(
      pack([
        { file: 'bg/a.png', kind: 'bg', data: same },
        { file: 'bg/b.png', kind: 'bg', data: same }, // те же байты того же вида
        { file: 'st/a.png', kind: 'sticker', data: same }, // другой вид — не повтор
        { file: 'sp/new.json', kind: 'spread', data: j({ v: 2 }) },
        { file: 'sp/old.json', kind: 'spread', data: j({ pages: 1 }) },
        { file: 'sp/ok.json', kind: 'spread', data: j({ v: 1 }) },
        { file: 'al/new.json', kind: 'album', data: j({ v: 2, steps: [{}] }) },
        { file: 'al/long.json', kind: 'album', data: steps(25) },
        { file: 'al/edge.json', kind: 'album', data: steps(24) },
        { file: 'al/str.json', kind: 'album', data: j({ v: 1, steps: 'x' }) },
      ]),
    ),
  )
  assert.equal(r.verdict, 'skip', r.error)
  assert.deepEqual(r.installed, ['bg/a.png', 'st/a.png', 'sp/ok.json', 'al/edge.json'])
  const why = Object.fromEntries(r.skipped.map((s) => [s.file, s.why]))
  assert.equal(why['bg/b.png'], 'повтор в наборе')
  assert.equal(why['sp/new.json'], 'шаблон из новой версии программы')
  assert.equal(why['sp/old.json'], 'шаблон разворота не читается')
  assert.equal(why['al/new.json'], 'макет из новой версии программы')
  assert.equal(why['al/long.json'], 'в макете больше 24 разворотов')
  assert.equal(why['al/str.json'], 'макет альбома не читается')
})

test('рамочки: отчёт говорит, что окно прозрачности проверит программа', async () => {
  const r = await checkZip(put(pack([{ file: 'fr/a.png', kind: 'frame', data: png(100, 100) }])))
  assert.equal(r.verdict, 'ok', r.error)
  assert.deepEqual(report(r), ['набор «Тестовый набор» (test-pack, версия 1): ставится 1, пропущено 0', 'рамочки: окно прозрачности проверит программа', 'ставится'])
  const none = await checkZip(put(pack([{ file: 'bg/a.png', kind: 'bg', data: png(100, 100) }])))
  assert.ok(!report(none).some((l) => l.startsWith('рамочки')))
})

// ---- Node, ошибка чтения ----

test('версия Node: нет crc32 или текста числа в JSON.parse — «нужен Node 22.2 или новее»', () => {
  assert.equal(nodeProblem(), '')
  assert.equal(nodeProblem({}, JSON.parse), 'нужен Node 22.2 или новее')
  assert.equal(nodeProblem(zlib, (text) => JSON.parse(text)), 'нужен Node 22.2 или новее') // reviver не вызывается
  assert.equal(nodeProblem(zlib, () => { throw new Error('x') }), 'нужен Node 22.2 или новее')
})

test('CLI на старом Node (нет zlib.crc32): сообщение и код 2, а не «архив повреждён»', () => {
  const hook = path.join(TMP, 'no-crc32.mjs')
  fs.writeFileSync(hook, "import zlib from 'node:zlib'\ndelete zlib.crc32\n")
  const r = spawnSync(process.execPath, ['--import', pathToFileURL(hook).href, SCRIPT, put(pack([{ file: 'bg/a.png', kind: 'bg', data: png(10, 10) }]))], { encoding: 'utf8' })
  assert.equal(r.status, 2)
  assert.match(r.stderr, /нужен Node 22\.2 или новее/)
  assert.equal(r.stdout, '')
})

test('ошибка чтения файла на любом шаге — отказ, а не падение процесса (в том числе внутри потока распаковки)', async () => {
  const zip = put(pack([{ file: 'sp/big.json', kind: 'spread', data: Buffer.from(JSON.stringify({ v: 1, pad: 'x'.repeat(9000) })) }, { file: 'bg/a.png', kind: 'bg', data: png(10, 10) }]), 'eio.zip')
  const orig = fs.readSync
  let calls = 0
  let failAt = 0
  fs.readSync = (...a) => {
    if (++calls === failAt) throw Object.assign(new Error('EIO'), { code: 'EIO' })
    return orig(...a)
  }
  try {
    assert.equal((await checkZip(zip)).verdict, 'ok')
    const total = calls
    assert.ok(total > 5, `чтений ${total}`)
    for (failAt = 1; failAt <= total; failAt++) {
      calls = 0
      const r = await checkZip(zip)
      assert.equal(r.verdict, 'reject', `ошибка на чтении ${failAt}`)
      assert.equal(typeof r.error, 'string')
    }
  } finally {
    fs.readSync = orig
  }
})
