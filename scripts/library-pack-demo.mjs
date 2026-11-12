// Пример темы «свадьба» в программе (спека 142, design §1): сборка zip без каталога и подписи, резерв id,
// страж ключей и снимки ключей опубликованных zip. Подключается из library-pack.mjs (там и общие проверки
// предметов); сам не импортирует его — обмен через объект lib, без цикла модулей.
//
//   node scripts/library-pack.mjs --zip-only <папка> --out <папка>
//       zip набора с теми же проверками предметов, но без каталога, ключа подписи и --seq; макеты альбома
//       разрешены в наборе с любым id. Публикуется такой zip отдельно (релиз в репозитории библиотеки).
//   node scripts/library-pack.mjs --snapshot <zip> --out <папка>
//       <id>-v<версия>.keys: строки `вид ключ [путь]` опубликованного zip.
//   node scripts/library-pack.mjs --demo-keys <zip примера> --donor <zip> [--donor <zip>]... --out <папка>
//       demo-v<версия>.keys: строки `донор вид ключ [путь]` — у какого набора каталога лежит каждый предмет.
//
// Ключ строки — `my:` + первые 16 hex sha256 байтов файла предмета (для картинок и макета это ключ программы;
// у шаблона разворота программа берёт ключ от канонического JSON, но ему нужен Go, а здесь — только путь).
// Путь пишется у шаблона разворота и макета альбома: их находят по пути, а не по байтам (v2 донора может
// переписать файл, путь держит альбомы — 0.28-131).
import crypto from 'node:crypto'
import fs from 'node:fs'
import path from 'node:path'

/** Точные id, занятые примером в программе: в каталог такой набор не попадает (0.30-142-29). */
export const DEMO_IDS = ['wedding-navy-watercolor-demo']
export const DEMO_KEYS = 'internal/demopack/testdata/demo-v1.keys' // от корня репозитория
export const DEMO_MAX_ZIP = 20 << 20 // бюджет сборки: pre-commit пропускает internal/demopack/*.zip до 20 МиБ
const MAX_KEYS = 1 << 20 // файл ключей: сотни строк — десятки КБ
const MAX_MANIFEST = 256 << 10 // как в buildPack
const PATH_KINDS = ['spread', 'album']
const KINDS = ['bg', 'sticker', 'spread', 'frame', 'print', 'album']
const ID_RE = /^[a-z0-9][a-z0-9-]{2,39}$/
const KEY_RE = /^my:[0-9a-f]{16}$/
const FILE_RE = /^[a-z0-9][a-z0-9._-]{0,63}(\/[a-z0-9][a-z0-9._-]{0,63}){0,2}$/
const VALUE_FLAGS = ['--zip-only', '--snapshot', '--demo-keys', '--donor', '--out']
const MODES = ['--zip-only', '--snapshot', '--demo-keys']

/** Ошибка разбора командной строки: код выхода 2. */
const usage = (msg) => Object.assign(new Error(msg), { code: 2 })

export const isDemoCmd = (argv) => argv.some((a) => MODES.includes(a))

export const itemKey = (sha256) => `my:${sha256.slice(0, 16)}`

const fields = (kind, key, file) => [kind, key, ...(PATH_KINDS.includes(kind) ? [file] : [])]

/** Строки снимка `вид ключ [путь]` по предметам манифеста. */
export function snapshotKeys(manifest) {
  return manifest.items.map((it) => fields(it.kind, itemKey(it.sha256), it.file).join(' ') + '\n').join('')
}

/** Файл ключей -> [{donor?, kind, key, path?}]; битая строка, повтор и лишнее поле — исключение с номером строки. */
export function parseKeys(text, withDonor = false) {
  const out = []
  const seen = new Set()
  text.split(/\r?\n/).forEach((line, i) => {
    if (line === '') return
    const f = line.split(' ')
    const bad = (why) => new Error(`ключи, строка ${i + 1}: ${why}`)
    const donor = withDonor ? f.shift() : undefined
    if (withDonor && !ID_RE.test(donor ?? '')) throw bad('нет id донора')
    const [kind, key, file, ...rest] = f
    if (!KINDS.includes(kind)) throw bad(`вид ${kind}`)
    if (!KEY_RE.test(key ?? '')) throw bad('ключ — my: и 16 hex')
    if (PATH_KINDS.includes(kind) !== (file !== undefined) || rest.length > 0) throw bad(PATH_KINDS.includes(kind) ? 'нужен ровно один путь' : 'путь только у шаблона разворота и макета альбома')
    if (file !== undefined && (!FILE_RE.test(file) || file.includes('..'))) throw bad(`путь ${file}`)
    const id = [donor, kind, key, file].join(' ')
    if (seen.has(id)) throw bad('повтор')
    seen.add(id)
    out.push({ donor, kind, key, path: file })
  })
  return out
}

/** Файл ключей примера из репозитория; нет файла — исключение (страж не должен молча отключаться). */
export function readDemoKeys(root) {
  const file = path.join(root, ...DEMO_KEYS.split('/'))
  const st = fs.statSync(file)
  if (!st.isFile() || st.size > MAX_KEYS) throw new Error(`${DEMO_KEYS}: не обычный файл или больше ${MAX_KEYS >> 20} МБ`)
  return fs.readFileSync(file, 'utf8')
}

/**
 * Проверки набора перед выпуском в каталог: id примера занят; а у набора-донора из demo-v1.keys должна остаться
 * каждая строка — иначе пример в программе потеряет предмет, который уже лежит в чьих-то альбомах (0.30-142-21).
 */
export function checkDemoPack(manifest, keysText) {
  if (DEMO_IDS.includes(manifest.id)) throw new Error(`id ${manifest.id} занят примером в программе: его zip собирается только через --zip-only`)
  const keys = new Set(manifest.items.map((it) => `${it.kind} ${itemKey(it.sha256)}`))
  const files = new Set(manifest.items.map((it) => `${it.kind} ${it.file}`))
  for (const e of parseKeys(keysText, true)) {
    if (e.donor !== manifest.id) continue
    if (!(e.path === undefined ? keys.has(`${e.kind} ${e.key}`) : files.has(`${e.kind} ${e.path}`))) {
      throw new Error(`ключ примера в программе пропал: ${e.kind} ${e.key}${e.path ? ` ${e.path}` : ''}`)
    }
  }
}

/** Читает опубликованный zip: размер — до чтения, предметы — по sha256 манифеста; -> {manifest, files:Map}. */
export function readPublished(file, lib) {
  const files = lib.readZip(lib.readLimited(file, lib.MAX_ZIP, file))
  const mf = files.get('manifest.json')
  if (!mf || mf.length > MAX_MANIFEST) throw new Error(`${file}: нет manifest.json или он больше ${MAX_MANIFEST >> 10} КБ`)
  const manifest = JSON.parse(mf.toString('utf8'))
  if (typeof manifest?.id !== 'string' || !Number.isInteger(manifest.version) || !Array.isArray(manifest.items)) throw new Error(`${file}: манифест без id, version или items`)
  for (const it of manifest.items) {
    const data = files.get(it.file)
    const sha = data && crypto.createHash('sha256').update(data).digest('hex')
    if (!KINDS.includes(it.kind) || sha !== it.sha256 || data.length !== it.size) throw new Error(`${file}: предмет ${it.file} не сходится с манифестом`)
  }
  return { manifest, files }
}

/** Значения повторяемого ключа командной строки. */
const all = (argv, name) => argv.flatMap((a, i) => (a === name ? [argv[i + 1]] : []))

/** Разбор строки: ровно один режим, только свои ключи, у каждого есть значение. */
function parseArgs(argv) {
  argv.forEach((a, i) => {
    if (a.startsWith('--') && !VALUE_FLAGS.includes(a)) throw usage(`${a}: в этом режиме не нужен (каталога и подписи нет)`)
    if (!a.startsWith('--') && !VALUE_FLAGS.includes(argv[i - 1])) throw usage(`лишний аргумент ${a}`)
    if (VALUE_FLAGS.includes(a) && (argv[i + 1] === undefined || argv[i + 1].startsWith('--'))) throw usage(`${a}: нужно значение`)
  })
  const modes = MODES.filter((m) => argv.includes(m))
  if (modes.length !== 1) throw usage(`нужен ровно один режим из ${MODES.join(', ')}`)
  const out = all(argv, '--out')
  if (out.length !== 1) throw usage('нужен ровно один --out <папка>')
  const val = all(argv, modes[0])
  if (val.length !== 1) throw usage(`${modes[0]} задан несколько раз`)
  return { mode: modes[0], value: val[0], out: path.resolve(out[0]), donors: all(argv, '--donor') }
}

/** Запись через временный файл: частичного результата в папке нет. */
function writeAtomic(dir, name, data) {
  fs.mkdirSync(dir, { recursive: true })
  const tmp = path.join(dir, `${name}.tmp`)
  fs.writeFileSync(tmp, data)
  fs.renameSync(tmp, path.join(dir, name))
}

/** Режимы без каталога; ошибка — исключение (у ошибки командной строки code 2). lib — buildPack, makeZip, assertZipSize, readZip, readLimited, MAX_ZIP. */
export function demoMain(argv, lib) {
  const o = parseArgs(argv)
  if (o.mode !== '--demo-keys' && o.donors.length > 0) throw usage('--donor только для --demo-keys')
  if (o.mode === '--zip-only') {
    const { manifest, files } = lib.buildPack(path.resolve(o.value), { zipOnly: true })
    const zip = lib.makeZip(files)
    lib.assertZipSize(zip)
    if (DEMO_IDS.includes(manifest.id) && zip.length > DEMO_MAX_ZIP) throw new Error(`zip примера ${zip.length} байт больше бюджета ${DEMO_MAX_ZIP >> 20} МиБ`)
    const tag = `${manifest.id}-${manifest.version}.zip`
    writeAtomic(o.out, tag, zip)
    console.log(`${tag}: ${zip.length} байт, предметов ${manifest.items.length}, sha256 ${crypto.createHash('sha256').update(zip).digest('hex')}`)
  } else if (o.mode === '--snapshot') {
    const { manifest } = readPublished(path.resolve(o.value), lib)
    const name = `${manifest.id}-v${manifest.version}.keys`
    writeAtomic(o.out, name, snapshotKeys(manifest))
    console.log(`${name}: ${manifest.items.length} строк`)
  } else {
    if (o.donors.length === 0) throw usage('--demo-keys: нужен хотя бы один --donor <zip>')
    const demo = readPublished(path.resolve(o.value), lib).manifest
    const donors = o.donors.map((d) => readPublished(path.resolve(d), lib).manifest)
    const lines = demo.items.map((it) => {
      const hit = donors.find((d) => d.items.some((x) => x.kind === it.kind && x.sha256 === it.sha256 && (!PATH_KINDS.includes(it.kind) || x.file === it.file)))
      if (!hit) throw new Error(`предмет примера ${it.file} не найден ни в одном доноре (по байтам и, у шаблона и макета, по пути)`)
      return [hit.id, ...fields(it.kind, itemKey(it.sha256), it.file)].join(' ') + '\n'
    })
    const name = `demo-v${demo.version}.keys`
    writeAtomic(o.out, name, lines.join(''))
    console.log(`${name}: ${lines.length} строк`)
  }
}
