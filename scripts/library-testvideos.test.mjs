// Тесты тестовых роликов справочника (140, Т7): формат строки videos, подпись каталога, разбор Go.
import assert from 'node:assert/strict'
import crypto from 'node:crypto'
import fs from 'node:fs'
import os from 'node:os'
import path from 'node:path'
import { spawnSync } from 'node:child_process'
import { test } from 'node:test'
import { publicRaw, readCatalogFile, sealCatalog, verifyBytes } from './library-pack.mjs'
import { BASE, MAX_VIDEO_FILE, buildVideos, ffmpegArgs, parseArgs, parseIds, publish, root, sha256, siteName, verifyGo, videoRow, withVideos } from './library-testvideos.mjs'

const have = (cmd) => spawnSync(cmd, [cmd === 'go' ? 'version' : '-version'], { encoding: 'utf8' }).status === 0

/** Тестовое окружение: ключ, site/ с каталогом без роликов, кэш. */
function env(packs = []) {
  const d = fs.mkdtempSync(path.join(os.tmpdir(), 'libvideos-'))
  const site = path.join(d, 'site')
  fs.mkdirSync(site)
  const key = crypto.generateKeyPairSync('ed25519').privateKey
  const keyFile = path.join(d, 'key.pem')
  fs.writeFileSync(keyFile, key.export({ type: 'pkcs8', format: 'pem' }))
  fs.writeFileSync(path.join(site, 'catalog.json'), JSON.stringify(sealCatalog({ format: 1, seq: 2, packs }, key)) + '\n')
  return { d, site, key, keyFile, cacheDir: path.join(d, 'cache') }
}

/** Ролик-болванка: содержимое зависит от id, версии и вида, длина — разная. */
const fakeEnc = ({ id, version, kind, big }) => Buffer.from(`${id}|${version}|${kind}|${big ? 'big' : ''}`.repeat(big ? 50 : 1))

test('строка ролика: поля, sha256 и размер настоящие, адрес — раздача 127.0.0.1:34190', () => {
  const files = { webm: Buffer.from('webm-bytes'), mp4: Buffer.from('mp4-bytes!') }
  const row = videoRow({ id: 'export', version: 1, minApp: '0.28.0', seconds: 10, files })
  assert.deepEqual(Object.keys(row), ['id', 'version', 'minApp', 'seconds', 'files'])
  assert.deepEqual(Object.keys(row.files).sort(), ['mp4', 'webm'])
  for (const kind of ['webm', 'mp4']) {
    assert.equal(row.files[kind].size, files[kind].length)
    assert.equal(row.files[kind].sha256, sha256(files[kind]))
    assert.match(row.files[kind].sha256, /^[0-9a-f]{64}$/)
    assert.equal(row.files[kind].url, `${BASE}export-1.${kind}`)
  }
  // Только один вид — строка с одним ключом (нужное Go-разбору «хотя бы один»).
  assert.deepEqual(Object.keys(videoRow({ id: 'export', version: 1, minApp: '0.28.0', seconds: 1, files: { mp4: files.mp4 } }).files), ['mp4'])
})

test('битый ролик: sha256 в каталоге не сходится с файлом, но остаётся 64 hex; размер честный', () => {
  const files = { webm: Buffer.from('a'), mp4: Buffer.from('b') }
  const ok = videoRow({ id: 'dpi', version: 1, minApp: '0.28.0', seconds: 8, files })
  const bad = videoRow({ id: 'dpi', version: 1, minApp: '0.28.0', seconds: 8, files, wrongSha: true })
  for (const kind of ['webm', 'mp4']) {
    assert.notEqual(bad.files[kind].sha256, ok.files[kind].sha256)
    assert.match(bad.files[kind].sha256, /^[0-9a-f]{64}$/)
    assert.equal(bad.files[kind].size, ok.files[kind].size)
  }
  // sha, начинающаяся на «0», тоже портится (замена первого знака не даёт ту же строку).
  let n = 0
  while (!sha256(Buffer.from(String(n))).startsWith('0')) n++
  const zero = Buffer.from(String(n))
  assert.notEqual(videoRow({ id: 'dpi', version: 1, minApp: '0.28.0', seconds: 1, files: { webm: zero }, wrongSha: true }).files.webm.sha256, sha256(zero))
})

test('аргументы: состав по умолчанию, края и мусор', () => {
  const d = parseArgs([])
  assert.deepEqual(d.ids.map((s) => s.id), ['filter', 'layout-gallery', 'export'])
  assert.ok(d.ids.every((s) => s.seconds >= 8 && s.seconds <= 10))
  assert.equal(d.big, null)
  assert.equal(d.broken, null)
  assert.deepEqual(parseArgs(['--big']).big, { id: 'autobuild', seconds: 30 })
  assert.deepEqual(parseArgs(['--big', 'export:20']).big, { id: 'export', seconds: 20 })
  assert.deepEqual(parseArgs(['--broken', '--no-verify']).broken, { id: 'fill-empty', seconds: 8 })
  assert.equal(parseArgs(['--no-verify']).verify, false)
  assert.deepEqual(parseIds('a-b:1,c-d:600'), [{ id: 'a-b', seconds: 1 }, { id: 'c-d', seconds: 600 }])
  assert.deepEqual(parseIds(''), [])
  for (const bad of ['Export', 'ab', 'x_y1', 'export:0', 'export:601', 'export:1.5', '../export', 'export:abc']) assert.throws(() => parseIds(bad), undefined, bad)
  assert.throws(() => parseArgs(['--ids']), /нужно значение/)
  assert.throws(() => parseArgs(['--min-app', '0.30']), /X\.Y\.Z/)
  assert.throws(() => parseArgs(['--nope']), /неизвестный/)
})

test('ffmpeg: параметры design §5, 1280 в ширину, 15 к/с, без звука', () => {
  const webm = ffmpegArgs({ kind: 'webm', seconds: 9, label: 'export v1', out: 'o.webm', font: null }).join(' ')
  assert.match(webm, /testsrc=size=1280x720:rate=15:duration=9/)
  assert.match(webm, /-an /)
  assert.match(webm, /-c:v libvpx-vp9 -crf 40 -b:v 0 -row-mt 1 o\.webm$/)
  assert.match(webm, /export v1/)
  const mp4 = ffmpegArgs({ kind: 'mp4', seconds: 9, label: 'export v1', out: 'o.mp4', font: '/f.ttf' }).join(' ')
  assert.match(mp4, /-c:v libx264 -crf 28 -preset slow -pix_fmt yuv420p -movflags \+faststart o\.mp4$/)
  assert.match(mp4, /fontfile=\/f\.ttf/)
  // Большой: по битрейту и с шумом, без crf.
  const big = ffmpegArgs({ kind: 'webm', seconds: 30, label: 'x', out: 'o.webm', font: null, big: true }).join(' ')
  assert.match(big, /noise=/)
  assert.doesNotMatch(big, /-crf/)
})

test('состав: ролики, большой, битый, v2 и будущий; всё ниже предела MaxVideoFile', () => {
  const o = parseArgs(['--big', '--broken', '--v2', 'export', '--future', 'export'])
  const { rows, files } = buildVideos(o, { minApp: '0.28.0', cacheDir: 'не нужен', enc: fakeEnc })
  assert.deepEqual(
    rows.map((r) => `${r.id}/${r.version}`),
    ['filter/1', 'layout-gallery/1', 'export/1', 'autobuild/1', 'fill-empty/1', 'export/2', 'export/3'],
  )
  assert.equal(rows.find((r) => r.id === 'export' && r.version === 3).minApp, '99.0.0')
  assert.equal(rows.find((r) => r.id === 'export' && r.version === 2).minApp, '0.28.0')
  // Файл на раздаче совпадает с sha строки — кроме битого.
  for (const r of rows) {
    for (const [kind, f] of Object.entries(r.files)) {
      const data = files.get(siteName(r.id, r.version, kind))
      assert.equal(data.length, f.size)
      assert.ok(f.size > 0 && f.size <= MAX_VIDEO_FILE)
      assert.equal(sha256(data) === f.sha256, r.id !== 'fill-empty', `${r.id} ${kind}`)
    }
  }
  assert.throws(() => buildVideos(parseArgs(['--v2', 'nope']), { minApp: '0.28.0', cacheDir: '', enc: fakeEnc }), /нет среди/)
  assert.throws(() => buildVideos(parseArgs([]), { minApp: '0.28.0', cacheDir: '', enc: () => Buffer.alloc(MAX_VIDEO_FILE + 1) }), /предела/)
})

test('каталог: подпись сходится, наборы целы, seq + 1, videos заменяется целиком, чужие файлы роликов убираются', () => {
  const packs = [{ id: 'ivory', version: 1 }]
  const e = env(packs)
  fs.writeFileSync(path.join(e.site, 'old-1.webm'), 'старый')
  fs.writeFileSync(path.join(e.site, 'ivory-1.zip'), 'zip')
  const o = parseArgs([])
  const r = publish(o, { site: e.site, keyFile: e.keyFile, minApp: '0.28.0', cacheDir: e.cacheDir, enc: fakeEnc })
  assert.equal(r.seq, 3)
  assert.equal(r.pub, publicRaw(e.key))
  const raw = fs.readFileSync(path.join(e.site, 'catalog.json'), 'utf8')
  const envl = JSON.parse(raw)
  assert.equal(verifyBytes(Buffer.from(envl.body, 'base64'), envl.sig, r.pub), true)
  const cat = readCatalogFile(path.join(e.site, 'catalog.json'))
  assert.deepEqual(cat.packs, packs)
  assert.equal(cat.videos.length, 3)
  assert.ok(!fs.existsSync(path.join(e.site, 'old-1.webm')))
  assert.ok(fs.existsSync(path.join(e.site, 'ivory-1.zip')), 'архивы наборов не трогаются')
  assert.ok(fs.existsSync(path.join(e.site, 'export-1.webm')) && fs.existsSync(path.join(e.site, 'export-1.mp4')))
  // Повтор с другим составом: videos заменяется, а не дополняется; seq растёт.
  const r2 = publish(parseArgs(['--ids', 'export:5']), { site: e.site, keyFile: e.keyFile, minApp: '0.28.0', cacheDir: e.cacheDir, enc: fakeEnc })
  assert.equal(r2.seq, 4)
  assert.deepEqual(readCatalogFile(path.join(e.site, 'catalog.json')).videos.map((v) => v.id), ['export'])
  assert.ok(!fs.existsSync(path.join(e.site, 'filter-1.webm')))
  // --clean: роликов нет совсем (строка «Ролик» у статей пропадает), каталог по-прежнему подписан.
  const r3 = publish(parseArgs(['--clean']), { site: e.site, keyFile: e.keyFile, minApp: '0.28.0', cacheDir: e.cacheDir, enc: fakeEnc })
  assert.equal(r3.seq, 5)
  assert.deepEqual(readCatalogFile(path.join(e.site, 'catalog.json')).videos, [])
  assert.equal(fs.readdirSync(e.site).filter((n) => /\.(webm|mp4)$/.test(n)).length, 0)
})

test('отказы: нет каталога, нет ключа; каталог при отказе не меняется', () => {
  const e = env()
  const before = fs.readFileSync(path.join(e.site, 'catalog.json'), 'utf8')
  const opts = { site: e.site, keyFile: path.join(e.d, 'нет.pem'), minApp: '0.28.0', cacheDir: e.cacheDir, enc: fakeEnc }
  assert.throws(() => publish(parseArgs([]), opts), /тестового ключа/)
  fs.rmSync(path.join(e.site, 'catalog.json'))
  assert.throws(() => publish(parseArgs([]), { ...opts, keyFile: e.keyFile }), /library-testsets/)
  fs.writeFileSync(path.join(e.site, 'catalog.json'), before)
  assert.throws(() => publish(parseArgs(['--ids', 'export:5']), { ...opts, keyFile: e.keyFile, enc: () => Buffer.alloc(MAX_VIDEO_FILE + 1) }), /предела/)
  assert.equal(fs.readFileSync(path.join(e.site, 'catalog.json'), 'utf8'), before)
  assert.deepEqual(withVideos(null, []), { seq: 1, videos: [] })
})

test('разбор Go: каталог проходит fetch.Stand + Client.Parse, строки роликов не пропущены', { skip: !have('go') && 'нет go' }, () => {
  const e = env()
  const o = parseArgs(['--big', '--broken', '--v2', 'export', '--future', 'export'])
  const r = publish(o, { site: e.site, keyFile: e.keyFile, minApp: '0.28.0', cacheDir: e.cacheDir, enc: fakeEnc })
  const want = r.rows.map((x) => `${x.id}/${x.version}`)
  const ok = verifyGo(path.join(e.site, 'catalog.json'), r.pub, want)
  assert.ok(ok.ok, ok.out)
  assert.match(ok.out, /роликов 7, пропущено 0/)
  // Чужой ключ — отказ разбора, а не молчаливо пустой каталог.
  const alien = verifyGo(path.join(e.site, 'catalog.json'), publicRaw(crypto.generateKeyPairSync('ed25519').privateKey), want)
  assert.equal(alien.ok, false)
  // Названного ролика нет — провал.
  assert.equal(verifyGo(path.join(e.site, 'catalog.json'), r.pub, ['nope/1']).ok, false)
  // Испорченная строка (адрес не из раздачи) попадает в «пропущено», и проверка падает.
  const cat = readCatalogFile(path.join(e.site, 'catalog.json'))
  cat.videos[0].files.webm.url = 'https://example.com/x.webm'
  fs.writeFileSync(path.join(e.site, 'catalog.json'), JSON.stringify(sealCatalog(cat, e.key)) + '\n')
  const skipped = verifyGo(path.join(e.site, 'catalog.json'), r.pub, [])
  assert.equal(skipped.ok, false)
  assert.match(skipped.out, /ПРОПУЩЕНО ролик filter/)
})

test('настоящий ffmpeg: оба вида, 1 с, каталог проходит разбор Go', { skip: (!have('ffmpeg') || !have('go')) && 'нет ffmpeg или go' }, () => {
  const e = env()
  const o = parseArgs(['--ids', 'export:1'])
  const r = publish(o, { site: e.site, keyFile: e.keyFile, minApp: '0.28.0', cacheDir: e.cacheDir })
  for (const kind of ['webm', 'mp4']) {
    const data = fs.readFileSync(path.join(e.site, `export-1.${kind}`))
    assert.equal(r.rows[0].files[kind].size, data.length)
    assert.equal(r.rows[0].files[kind].sha256, sha256(data))
    assert.ok(data.length > 1000)
  }
  // Кэш: второй запуск берёт файл, не пересчитывая (время изменения не меняется).
  const cached = path.join(e.cacheDir, 'export-v1-1s.webm')
  const t = fs.statSync(cached).mtimeMs
  publish(o, { site: e.site, keyFile: e.keyFile, minApp: '0.28.0', cacheDir: e.cacheDir })
  assert.equal(fs.statSync(cached).mtimeMs, t)
  const v = verifyGo(path.join(e.site, 'catalog.json'), r.pub, ['export/1'])
  assert.ok(v.ok, v.out)
})

test('корень скрипта — репозиторий', () => {
  assert.ok(fs.existsSync(path.join(root, 'wails.json')))
})
