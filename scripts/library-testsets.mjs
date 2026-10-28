#!/usr/bin/env node
// Тестовые наборы библиотеки для QA (спека 067 D1, docs/qa/library.md): в .testphotos/library/
//   src/ivory, src/sage    — папки наборов (картинки рисуются здесь, в git их нет);
//   site/                  — каталог и подпись тестовым ключом + хорошие архивы (раздаёт SA_LIBRARY_URL);
//   bad/                   — плохие архивы для AddPackPath: bad-hash, bomb, traversal, newer-app.
// Ключ: node scripts/library-pack.mjs --test-key (кладёт в .gotmp/, печатает открытый).
// Каталог здесь собирается с нуля (намеренно): поле videos стирается — ролики после этого скрипта:
//   node scripts/library-testvideos.mjs (спека 140).
// Лист образцов (спека 144): для каждого набора library-verify -sprite (go build в .gotmp) делает <id>-1-preview.jpg
// и .json, их проверяет previewField, строка каталога получает поле preview, лист лежит в site/ рядом с архивом.
// --min-app X.Y.Z — minApp в манифестах наборов (по умолчанию 0.16.0): чтобы собрать наборы для стенда старой версии.
import crypto from 'node:crypto'
import fs from 'node:fs'
import path from 'node:path'
import zlib from 'node:zlib'
import { spawnSync } from 'node:child_process'
import { fileURLToPath } from 'node:url'
import { URL_PREFIX, buildPack, catalogRow, makeZip, readLimited, sealCatalog, upsertCatalog } from './library-pack.mjs'
import { previewField, readSheet } from './library-pack-preview.mjs'

const root = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..')
const base = path.join(root, '.testphotos', 'library')

const ai = process.argv.indexOf('--min-app')
const minApp = ai < 0 ? '0.16.0' : process.argv[ai + 1]
if (!/^\d+\.\d+\.\d+$/.test(minApp ?? '')) {
  console.error('--min-app: нужна версия вида X.Y.Z, например 0.15.0')
  process.exit(2)
}

/** PNG w×h из одной краски с лёгким градиентом по вертикали (RGBA, без внешних библиотек). */
function png(w, h, [r, g, b], alpha = 255) {
  const row = Buffer.alloc(1 + w * 4)
  const raw = Buffer.alloc((1 + w * 4) * h)
  for (let y = 0; y < h; y++) {
    const k = 1 - (0.25 * y) / h
    for (let x = 0; x < w; x++) row.set([Math.floor(r * k), Math.floor(g * k), Math.floor(b * k), alpha], 1 + x * 4)
    row.copy(raw, y * (1 + w * 4))
  }
  const chunk = (type, data) => {
    const t = Buffer.from(type)
    const len = Buffer.alloc(4)
    len.writeUInt32BE(data.length)
    const crc = Buffer.alloc(4)
    crc.writeUInt32BE(zlib.crc32(Buffer.concat([t, data])))
    return Buffer.concat([len, t, data, crc])
  }
  const ihdr = Buffer.alloc(13)
  ihdr.writeUInt32BE(w, 0)
  ihdr.writeUInt32BE(h, 4)
  ihdr[8] = 8
  ihdr[9] = 6
  return Buffer.concat([
    Buffer.from([0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a]),
    chunk('IHDR', ihdr),
    chunk('IDAT', zlib.deflateSync(raw)),
    chunk('IEND', Buffer.alloc(0)),
  ])
}

const tpl = (pages, bg, places) => ({
  v: 1,
  pages,
  ratio: pages === 2 ? 1.5 : 0.75,
  places,
  page: Array.from({ length: pages }, () => ({ color: bg })),
})

function writeSet(id, title, color, bgNames) {
  const dir = path.join(base, 'src', id)
  fs.rmSync(dir, { recursive: true, force: true })
  for (const d of ['bg', 'stickers', 'spreads']) fs.mkdirSync(path.join(dir, d), { recursive: true })
  const items = []
  bgNames.forEach((n, i) => {
    fs.writeFileSync(path.join(dir, 'bg', `${id}-0${i + 1}.png`), png(600, 400, color.map((c) => Math.max(0, c - i * 12))))
    items.push({ file: `bg/${id}-0${i + 1}.png`, kind: 'bg', theme: 'wedding', name: n })
  })
  fs.writeFileSync(path.join(dir, 'stickers', 'dot.png'), png(256, 256, [200, 160, 60], 200))
  items.push({ file: 'stickers/dot.png', kind: 'sticker', theme: 'wedding', name: 'Точка' })
  const box = (x, y, w, h) => ({ x, y, w, h })
  fs.writeFileSync(
    path.join(dir, 'spreads', 'hero.json'),
    JSON.stringify(tpl(2, '#f4efe6', [box(0.05, 0.08, 0.42, 0.84), box(0.53, 0.08, 0.42, 0.4), box(0.53, 0.52, 0.42, 0.4)])),
  )
  items.push({ file: 'spreads/hero.json', kind: 'spread', theme: 'wedding', name: 'Главное и два' })
  fs.writeFileSync(path.join(dir, 'LICENSE.txt'), 'CC0-1.0\n')
  fs.writeFileSync(
    path.join(dir, 'manifest.src.json'),
    JSON.stringify({ id, version: 1, minApp, title, about: 'Тестовый набор для QA', author: 'QA', license: 'CC0-1.0', items }, null, 2),
  )
  return dir
}

let built = false // сборка раз за запуск скрипта

/** Лист образцов набора из его архива: собирает library-verify (Go) и запускает -sprite; результат — в sprites/. */
function sprite(zipFile) {
  const exe = path.join(root, '.gotmp', process.platform === 'win32' ? 'library-verify.exe' : 'library-verify')
  const files = ['library-verify.go', 'library-verify-sheets.go', 'library-verify-looks.go', 'library-verify-sprite.go'].map((f) => path.join(root, 'scripts', f))
  const out = path.join(base, 'sprites')
  const run = (cmd, args) => {
    const r = spawnSync(cmd, args, { cwd: root, encoding: 'utf8' })
    if (r.status !== 0) {
      console.error(`${cmd} ${args[0]}: код ${r.status ?? r.error?.message}\n${r.stdout}${r.stderr}`)
      process.exit(1)
    }
  }
  if (!built) run('go', ['build', '-o', exe, ...files]) // всегда: кэш Go сам пропускает неизменное, по mtime устаревший exe не отличить
  built = true
  fs.mkdirSync(out, { recursive: true })
  run(exe, ['-sprite', zipFile, '-out', out])
  return out
}

fs.mkdirSync(path.join(base, 'site'), { recursive: true })
fs.mkdirSync(path.join(base, 'bad'), { recursive: true })

// Хорошие наборы + подписанный каталог (если есть тестовый ключ).
const keyFile = path.join(root, '.gotmp', 'library-test-key.pem')
const key = fs.existsSync(keyFile) ? crypto.createPrivateKey(fs.readFileSync(keyFile)) : null
let cat = null
for (const [id, title, color] of [
  ['ivory', 'Свадьба — айвори', [244, 239, 230]],
  ['sage', 'Свадьба — шалфей', [190, 205, 185]],
]) {
  const { manifest, files } = buildPack(writeSet(id, title, color, ['Фон 1', 'Фон 2']))
  const zip = makeZip(files)
  const tag = `${id}-${manifest.version}.zip`
  fs.writeFileSync(path.join(base, 'site', tag), zip)
  const row = catalogRow(manifest, zip, tag)
  const sheets = sprite(path.join(base, 'site', tag))
  const { jpeg, metaText } = readSheet(sheets, manifest, { readLimited })
  row.preview = previewField(jpeg, metaText, manifest, URL_PREFIX)
  fs.writeFileSync(path.join(base, 'site', `${id}-${manifest.version}-preview.jpg`), jpeg)
  cat = upsertCatalog(cat, row)
}
// Для стенда адреса архивов — локальные (SA_LIBRARY_URL); путь после хоста — как в файле site/.
for (const p of cat.packs) {
  p.url = `http://127.0.0.1:34190/${path.basename(p.url)}`
  p.preview.url = `http://127.0.0.1:34190/${path.basename(p.preview.url)}`
}
fs.rmSync(path.join(base, 'site', 'catalog.json.sig'), { force: true })
if (key) fs.writeFileSync(path.join(base, 'site', 'catalog.json'), JSON.stringify(sealCatalog(cat, key)) + '\n')
else console.log('нет тестового ключа: node scripts/library-pack.mjs --test-key, затем запустить снова — каталог не записан')

// Плохие архивы: манифест собирается из хорошего набора и портится.
const good = buildPack(path.join(base, 'src', 'ivory'))
const withManifest = (mutate, extra = []) => {
  const m = JSON.parse(good.files[0].data.toString())
  mutate(m)
  const rest = good.files.slice(1).filter((f) => !extra.some((e) => e.name === f.name))
  return makeZip([{ name: 'manifest.json', data: Buffer.from(JSON.stringify(m)) }, ...rest, ...extra])
}
fs.writeFileSync(path.join(base, 'bad', 'bad-hash.zip'), withManifest((m) => (m.items[0].sha256 = '0'.repeat(64))))
fs.writeFileSync(path.join(base, 'bad', 'newer-app.zip'), withManifest((m) => (m.minApp = '99.0.0')))
fs.writeFileSync(path.join(base, 'bad', 'traversal.zip'), withManifest(() => {}, [{ name: '../evil.png', data: png(8, 8, [1, 2, 3]) }]))
// Бомба: 150 МБ нулей в одном «фоне» сжимаются в ~150 КБ — степень больше 200× (+1 МБ), архив отклоняется
// целиком по заголовку, до чтения (design §4.2); размер в манифесте честный, и он больше предела фона (60 МБ).
const zeros = Buffer.alloc(150 << 20)
fs.writeFileSync(
  path.join(base, 'bad', 'bomb.zip'),
  withManifest(
    (m) => {
      m.items[0].size = zeros.length
    },
    [{ name: 'bg/ivory-01.png', data: zeros, deflate: true }],
  ),
)
console.log(`наборы для QA: ${path.relative(root, base)} (site/ — раздача, bad/ — плохие архивы)`)
