#!/usr/bin/env node
// Тестовые ролики справочника F1 для стенда QA (спека 140, Т7; docs/qa/library.md «140 ролики F1»).
// Кладёт в подписанный тестовый каталог строки `videos` (формат — internal/decor/fetch/videos.go) и рисует
// ролики ffmpeg'ом в .testphotos/library/site/ (в git их нет; раздаёт 127.0.0.1:34190, как наборы).
//
//   node scripts/library-testvideos.mjs [--ids filter:8,layout-gallery:9,export:10] [--big [id[:сек]]] [--broken [id]]
//                                       [--v2 id] [--future id] [--min-app X.Y.Z] [--no-verify]   // lang-ok: строка запуска
//   node scripts/library-testvideos.mjs --print-key    открытый тестовый ключ (из уже созданного, нового не делает)
//   node scripts/library-testvideos.mjs --clean        убрать ролики из каталога и файлы из site/
//
// Каталог site/catalog.json уже должен быть (library-testsets.mjs; тестовый ключ — library-pack.mjs --test-key):
// скрипт читает его, заменяет поле videos целиком, seq + 1, подписывает тем же ключом. library-testsets.mjs
// пересобирает каталог с нуля и стирает videos — после него скрипт запускают снова. library-pack.mjs
// (upsertCatalog) и reseal из docs/qa/library.md videos сохраняют (0.30-140-26).
//
// Состав: --ids — обычные ролики (по умолчанию три статьи F1, 8–10 с); --big — ролик ~12 МБ (по умолчанию
// 30 с, статья autobuild) для хода и «Отменить»; --broken — ролик, у которого sha256 в каталоге не сходится с
// файлом (ошибка «скачался с ошибкой»); --v2 id — версия 2 уже названного ролика (переснят, строка снова
// «Скачать»); --future id — версия 3 с minApp 99.0.0 (программа её пропускает и берёт прежнюю).
// minApp роликов — версия программы из wails.json (стенд её и показывает), иначе --min-app.
// Параметры кодирования — design 140 §5: webm VP9 `-crf 40 -b:v 0 -row-mt 1`, mp4 H.264
// `-crf 28 -preset slow -pix_fmt yuv420p -movflags +faststart`, 1280 по ширине, 15 к/с, без звука.
// У большого ролика режим по битрейту (2,6 / 3,2 Мбит/с) и шум: размер предсказуем, а не зависит от кадра.
import crypto from 'node:crypto'
import fs from 'node:fs'
import path from 'node:path'
import { spawnSync } from 'node:child_process'
import { fileURLToPath } from 'node:url'
import { publicRaw, readCatalogFile, sealCatalog } from './library-pack.mjs'

export const root = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..')
export const BASE = 'http://127.0.0.1:34190/'
export const MAX_VIDEO_FILE = 16 << 20 // fetch.MaxVideoFile
export const DEFAULT_IDS = 'filter:8,layout-gallery:9,export:10'
export const BIG_ID = 'autobuild'
export const BROKEN_ID = 'fill-empty'
const ID_RE = /^[a-z0-9][a-z0-9-]{1,38}[a-z0-9]$/ // грубее fetch.ValidID, но до Go-разбора ловит опечатку
const SEMVER = /^\d+\.\d+\.\d+$/
const KINDS = ['webm', 'mp4']
const FONTS = [
  '/usr/share/fonts/truetype/dejavu/DejaVuSans-Bold.ttf',
  '/usr/share/fonts/dejavu/DejaVuSans-Bold.ttf',
  '/System/Library/Fonts/Supplemental/Arial Bold.ttf',
  'C:/Windows/Fonts/arialbd.ttf',
]

/** Разбор «id[:сек],…»; мусор — исключение. */
export function parseIds(s, defSec = 8) {
  return s
    .split(',')
    .filter(Boolean)
    .map((part) => {
      const [id, sec] = part.split(':')
      const seconds = sec === undefined ? defSec : Number(sec)
      if (!ID_RE.test(id)) throw new Error(`id ролика «${id}»: строчные латинские буквы, цифры, дефис`)
      if (!Number.isInteger(seconds) || seconds < 1 || seconds > 600) throw new Error(`длина «${part}»: целое 1…600 с`)
      return { id, seconds }
    })
}

/** Аргументы командной строки -> параметры; неизвестный ключ — исключение. */
export function parseArgs(argv) {
  const o = { ids: parseIds(DEFAULT_IDS), big: null, broken: null, v2: [], future: [], minApp: null, verify: true, printKey: false, clean: false }
  const value = (i) => (argv[i + 1] && !argv[i + 1].startsWith('--') ? argv[i + 1] : null)
  for (let i = 0; i < argv.length; i++) {
    const a = argv[i]
    const v = value(i)
    if (['--ids', '--min-app', '--v2', '--future'].includes(a) && v === null) throw new Error(`${a}: нужно значение`)
    switch (a) {
      case '--ids':
        o.ids = parseIds(v)
        i++
        break
      case '--big':
        o.big = parseIds(v ?? BIG_ID, 30)[0]
        if (v) i++
        break
      case '--broken':
        o.broken = parseIds(v ?? BROKEN_ID, 8)[0]
        if (v) i++
        break
      case '--v2':
        o.v2.push(v)
        i++
        break
      case '--future':
        o.future.push(v)
        i++
        break
      case '--min-app':
        o.minApp = v
        i++
        break
      case '--no-verify':
        o.verify = false
        break
      case '--print-key':
        o.printKey = true
        break
      case '--clean':
        o.clean = true
        break
      default:
        throw new Error(`неизвестный аргумент ${a}`)
    }
  }
  if (o.minApp !== null && !SEMVER.test(o.minApp)) throw new Error('--min-app: версия вида X.Y.Z')
  return o
}

export const sha256 = (buf) => crypto.createHash('sha256').update(buf).digest('hex')

/** Имя файла на раздаче: «<id>-<версия>.<вид>» — оно же последняя часть адреса (assetURL: без «..», ≤ 100). */
export const siteName = (id, version, kind) => `${id}-${version}.${kind}`

/** Строка videos по файлам {webm: Buffer, mp4: Buffer}; wrongSha — испортить sha256 (файл остаётся честным). */
export function videoRow({ id, version, minApp, seconds, files, wrongSha = false }) {
  const out = {}
  for (const kind of KINDS) {
    const data = files[kind]
    if (!data) continue
    const sum = sha256(data)
    out[kind] = {
      size: data.length,
      sha256: wrongSha ? (sum[0] === '0' ? '1' : '0') + sum.slice(1) : sum,
      url: BASE + siteName(id, version, kind),
    }
  }
  return { id, version, minApp, seconds, files: out }
}

/** Аргументы ffmpeg для одного вида; big — режим по битрейту и шум (размер ~12 МБ за 30 с). */
export function ffmpegArgs({ kind, seconds, label, out, font, big = false }) {
  const text = `${label} %{pts\\:hms}`
  const draw = [font ? `fontfile=${font}` : null, `text='${text}'`, 'fontsize=64', 'fontcolor=white', 'box=1', 'boxcolor=black@0.6', 'x=40', 'y=40']
    .filter(Boolean)
    .join(':')
  const vf = (big ? 'noise=alls=60:allf=t,' : '') + `drawtext=${draw}`
  const src = ['-f', 'lavfi', '-i', `testsrc=size=1280x720:rate=15:duration=${seconds}`, '-vf', vf, '-an']
  // VP9 перебирает заданный битрейт на шуме (~1,25×): 2,6 Мбит/с дают ~12 МБ за 30 с, как x264 на 3,2
  const rate = ['-b:v', '2600k', '-minrate', '2600k', '-maxrate', '2600k']
  const codec =
    kind === 'webm'
      ? ['-c:v', 'libvpx-vp9', ...(big ? [...rate, '-deadline', 'good', '-cpu-used', '4'] : ['-crf', '40', '-b:v', '0']), '-row-mt', '1']
      : ['-c:v', 'libx264', ...(big ? ['-b:v', '3200k', '-maxrate', '3200k', '-bufsize', '6400k'] : ['-crf', '28']), '-preset', 'slow', '-pix_fmt', 'yuv420p', '-movflags', '+faststart']
  return ['-y', '-v', 'error', ...src, ...codec, out]
}

function findFont() {
  return FONTS.find((f) => fs.existsSync(f)) ?? null
}

/** Ролик рисуется ffmpeg'ом; готовый файл с тем же ключом (id, версия, длина, режим) берётся из кэша. */
function encode({ id, version, seconds, kind, big }, cacheDir) {
  const out = path.join(cacheDir, `${id}-v${version}-${seconds}s${big ? '-big' : ''}.${kind}`)
  if (fs.existsSync(out) && fs.statSync(out).size > 0) return fs.readFileSync(out)
  const args = ffmpegArgs({ kind, seconds, label: `${id} v${version}`, out, font: findFont(), big })
  const r = spawnSync('ffmpeg', args, { encoding: 'utf8' })
  if (r.error || r.status !== 0) {
    fs.rmSync(out, { force: true })
    throw new Error(`ffmpeg (${id} ${kind}): ${r.error?.message ?? r.stderr}`)
  }
  return fs.readFileSync(out)
}

/** Версия программы стенда из wails.json (info.productVersion). */
function appVersion() {
  const w = JSON.parse(fs.readFileSync(path.join(root, 'wails.json'), 'utf8'))
  return w.info.productVersion
}

/** Все строки videos и файлы для site/ по параметрам; ничего не пишет. */
export function buildVideos(o, { minApp, cacheDir, enc }) {
  enc ??= encode
  const rows = []
  const files = new Map()
  const add = (spec, extra = {}) => {
    const { id, seconds, version = 1, big = false, wrongSha = false } = { ...spec, ...extra }
    const data = {}
    for (const kind of KINDS) {
      data[kind] = enc({ id, version, seconds, kind, big }, cacheDir)
      if (data[kind].length > MAX_VIDEO_FILE) throw new Error(`${id}.${kind}: ${data[kind].length} Б — больше предела MaxVideoFile`)
      files.set(siteName(id, version, kind), data[kind])
    }
    rows.push(videoRow({ id, version, minApp: extra.minApp ?? minApp, seconds, files: data, wrongSha }))
  }
  for (const s of o.ids) add(s)
  if (o.big) add(o.big, { big: true })
  if (o.broken) add(o.broken, { wrongSha: true })
  const sec = (id) => [...o.ids, o.big, o.broken].find((s) => s?.id === id)?.seconds
  for (const id of o.v2) {
    if (sec(id) === undefined) throw new Error(`--v2 ${id}: ролика нет среди --ids/--big/--broken`)
    add({ id, seconds: sec(id) }, { version: 2 })
  }
  for (const id of o.future) {
    if (sec(id) === undefined) throw new Error(`--future ${id}: ролика нет среди --ids/--big/--broken`)
    add({ id, seconds: sec(id) }, { version: 3, minApp: '99.0.0' })
  }
  return { rows, files }
}

/** Каталог с новыми videos: seq + 1, остальное как было. */
export function withVideos(cat, rows) {
  return { ...cat, seq: (cat?.seq ?? 0) + 1, videos: rows }
}

const isVideoFile = (n) => /\.(webm|mp4)$/.test(n)

/** Закрытый тестовый ключ из .gotmp (создаёт library-pack.mjs --test-key). */
function loadKey(keyFile) {
  if (!fs.existsSync(keyFile)) throw new Error('нет тестового ключа: node scripts/library-pack.mjs --test-key')
  return crypto.createPrivateKey(fs.readFileSync(keyFile))
}

/** Пишет site/: файлы роликов и подписанный каталог. */
export function publish(o, { site, keyFile, minApp, cacheDir, enc }) {
  const key = loadKey(keyFile)
  const catFile = path.join(site, 'catalog.json')
  const cat = readCatalogFile(catFile)
  if (!cat) throw new Error('нет site/catalog.json: сначала node scripts/library-testsets.mjs')
  fs.mkdirSync(cacheDir, { recursive: true })
  const { rows, files } = o.clean ? { rows: [], files: new Map() } : buildVideos(o, { minApp, cacheDir, enc })
  for (const n of fs.readdirSync(site).filter(isVideoFile)) fs.rmSync(path.join(site, n))
  for (const [name, data] of files) fs.writeFileSync(path.join(site, name), data)
  const next = withVideos(cat, rows)
  fs.writeFileSync(catFile, JSON.stringify(sealCatalog(next, key)) + '\n')
  return { rows, seq: next.seq, pub: publicRaw(key) }
}

/** Разбор каталога Go глазами программы (scripts/library-testvideos-verify.go); провал — строка с причиной. */
export function verifyGo(catFile, pub, want) {
  const r = spawnSync('go', ['run', path.join(root, 'scripts', 'library-testvideos-verify.go'), catFile, pub, ...want], { cwd: root, encoding: 'utf8' })
  return { ok: r.status === 0, out: (r.stdout ?? '') + (r.stderr ?? '') }
}

function main() {
  const o = parseArgs(process.argv.slice(2))
  const keyFile = path.join(root, '.gotmp', 'library-test-key.pem')
  if (o.printKey) return console.log(publicRaw(loadKey(keyFile)))
  const base = path.join(root, '.testphotos', 'library')
  const site = path.join(base, 'site')
  if (!fs.existsSync(site)) throw new Error('нет .testphotos/library/site: node scripts/library-testsets.mjs')
  const { rows, seq, pub } = publish(o, { site, keyFile, minApp: o.minApp ?? appVersion(), cacheDir: path.join(base, 'videos') })
  for (const r of rows) {
    const sizes = KINDS.map((k) => `${k} ${(r.files[k].size / 1024 / 1024).toFixed(2)} МБ`).join(', ')
    console.log(`  ${r.id} v${r.version}: ${r.seconds} с, minApp ${r.minApp}, ${sizes}`)
  }
  console.log(`каталог: seq ${seq}, роликов ${rows.length} (site/); тестовый ключ — node scripts/library-testvideos.mjs --print-key`)
  if (!o.verify) return
  const want = rows.map((r) => `${r.id}/${r.version}`)
  const v = verifyGo(path.join(site, 'catalog.json'), pub, want)
  process.stdout.write(v.out)
  if (!v.ok) process.exit(1)
}

if (process.argv[1] && path.resolve(process.argv[1]) === fileURLToPath(import.meta.url)) {
  try {
    main()
  } catch (e) {
    console.error(e.message)
    process.exit(2)
  }
}
