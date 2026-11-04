#!/usr/bin/env node
// Публикация роликов справочника F1 в боевой каталог библиотеки (спека 140, design §6 п. 4): строки `videos` из
// run.json съёмки (gv.py → encode.sh) + подписанный каталог seq + 1. Ключ — только у оркестратора.
//
//   node scripts/library-videos-publish.mjs --run <run.json> --dir <папка с файлами роликов>   проверка: без ключа и записи
//   node scripts/library-videos-publish.mjs --run … --dir … --catalog <прежний catalog.json> --key <pem> --out <новый catalog.json> [--seq N]
//
// Проверка (всегда, до записи): у каждой строки файлы лежат в --dir под именем из адреса, размер и sha256 совпадают
// (≤ 16 МБ, fetch.MaxVideoFile), адрес — релиз videos-1 библиотеки, id/версия/длина по правилам Go (videos.go), у каждой
// строки оба вида — webm и mp4 (Mac играет mp4).
// Печатает строки для каталога и команду загрузки файлов. Слияние: новая версия id добавляется (версия не ниже стоящей),
// строки для старых программ (меньший minApp) остаются, затенённые убираются; та же версия с другим содержимым — отказ;
// остальные `videos` и наборы каталога как есть; seq — прежний + 1 или явный --seq (больше прежнего). Подпись прежнего
// каталога сверяется переданным ключом до слияния.
// Go-разбор library-testvideos-verify.go принимает только адреса стенда (fetch.Stand), боевые адреса он пропустит —
// после публикации файлы сверяют скачиванием по url (sha256 из строки).
import crypto from 'node:crypto'
import fs from 'node:fs'
import path from 'node:path'
import { pathToFileURL } from 'node:url'
import { publicRaw, readLimited, sealCatalog, verifyBytes } from './library-pack.mjs'

export const RELEASE = 'videos-1'
export const URL_PREFIX = `https://github.com/IliaSA101/sosninaalbums-library/releases/download/${RELEASE}/`
export const MAX_VIDEO_FILE = 16 << 20 // fetch.MaxVideoFile
export const MAX_VIDEOS = 100 // fetch.MaxVideos
const MAX_RUN = 1 << 20
const ID_RE = /^[a-z0-9][a-z0-9-]{2,39}$/ // fetch.ValidID
const SEMVER = /^\d+\.\d+\.\d+$/
const KINDS = ['webm', 'mp4']
const MAX_CATALOG = 8 << 20 // как в library-pack.mjs

const sha256 = (buf) => crypto.createHash('sha256').update(buf).digest('hex')

/** Строки `videos` из run.json (поле ok.<id>.encoded.video); битый или огромный файл — ошибка, а не паника. */
export function rowsFromRun(file) {
  const st = fs.statSync(file)
  if (!st.isFile() || st.size > MAX_RUN) throw new Error(`${file}: не файл или больше ${MAX_RUN} Б`)
  let run
  try {
    run = JSON.parse(fs.readFileSync(file, 'utf8'))
  } catch (e) {
    throw new Error(`${file}: не JSON (${e.message})`)
  }
  const ok = run && typeof run === 'object' && run.ok && typeof run.ok === 'object' ? run.ok : null
  if (!ok) throw new Error(`${file}: нет поля ok`)
  const rows = Object.values(ok).flatMap((r) => (r?.encoded?.video ? [r.encoded.video] : []))
  if (rows.length === 0) throw new Error(`${file}: нет ни одного закодированного ролика`)
  if (rows.length > MAX_VIDEOS) throw new Error(`роликов ${rows.length} больше ${MAX_VIDEOS}`)
  return rows
}

/** Проверка строки и файлов в dir; возвращает список ошибок (пустой — годится). */
export function checkRow(row, dir) {
  const bad = []
  const { id, version, minApp, seconds, files } = row ?? {}
  if (typeof id !== 'string' || !ID_RE.test(id)) bad.push(`id «${id}» не подходит`)
  if (!Number.isInteger(version) || version < 1 || version > 999999) bad.push(`${id}: версия ${version}`)
  if (typeof minApp !== 'string' || !SEMVER.test(minApp)) bad.push(`${id}: minApp ${minApp}`)
  if (!Number.isInteger(seconds) || seconds < 1 || seconds > 600) bad.push(`${id}: длина ${seconds} с`)
  const kinds = files && typeof files === 'object' ? Object.keys(files) : []
  if (kinds.length === 0) bad.push(`${id}: нет файлов`)
  // оба вида обязательны: Go допускает один, но без mp4 Mac (WebKit не играет webm) получит ролик, который не воспроизведётся
  else for (const kind of KINDS) if (!kinds.includes(kind)) bad.push(`${id}: нет вида ${kind} (нужны webm и mp4)`)
  for (const kind of kinds) {
    const f = files[kind]
    const name = `${id}-${version}.${kind}`
    if (!KINDS.includes(kind)) {
      bad.push(`${id}: вид «${kind}» не webm и не mp4`)
      continue
    }
    if (f?.url !== URL_PREFIX + name) bad.push(`${name}: адрес ${f?.url} ≠ ${URL_PREFIX + name}`)
    const p = path.join(dir, name)
    let st
    try {
      st = fs.statSync(p)
    } catch {
      bad.push(`${name}: нет файла в ${dir}`)
      continue
    }
    if (!st.isFile() || st.size === 0 || st.size > MAX_VIDEO_FILE) bad.push(`${name}: размер ${st.size} (1…${MAX_VIDEO_FILE})`)
    else if (st.size !== f.size) bad.push(`${name}: размер ${st.size} ≠ ${f.size} в строке`)
    else if (sha256(fs.readFileSync(p)) !== f.sha256) bad.push(`${name}: sha256 не совпал со строкой`)
  }
  return bad
}

/** Сравнение x.y.z по числам: <0, 0, >0. */
const cmpVer = (a, b) => {
  const [x, y] = [a, b].map((s) => String(s).split('.').map(Number))
  for (let i = 0; i < 3; i++) if (x[i] !== y[i]) return (x[i] ?? 0) - (y[i] ?? 0)
  return 0
}

/** Содержимое строки для сравнения: ключи по порядку, чтобы порядок полей в JSON не менял вывод. */
const canon = (v) => JSON.stringify(v, (_, x) => (x && typeof x === 'object' && !Array.isArray(x) ? Object.fromEntries(Object.entries(x).sort(([a], [b]) => (a < b ? -1 : 1))) : x))

/**
 * Каталог с новыми строками. Go (videos.go) держит несколько строк одного id с разными версиями, программа берёт
 * самую новую с minApp не выше себя (pick.go, 140-17) — поэтому строка для старой программы остаётся, пока её не
 * затенила новая версия с тем же или меньшим minApp. Та же версия: то же содержимое — без изменений, другое — отказ.
 * seq + 1 или явный (> прежнего).
 */
export function mergeVideos(cat, rows, seq) {
  let list = Array.isArray(cat?.videos) ? [...cat.videos] : []
  for (const r of rows) {
    const same = list.filter((v) => v?.id === r.id)
    for (const had of same) {
      if (had.version === r.version && canon(had) !== canon(r)) {
        throw new Error(`${r.id}: версия ${r.version} уже в каталоге с другим содержимым (sha256, размер, minApp или длина) — поднимите версию`)
      }
      if (had.version > r.version) throw new Error(`${r.id}: версия ${r.version} ниже стоящей в каталоге (${had.version})`)
    }
    if (same.some((v) => v.version === r.version)) continue // то же содержимое: строка уже стоит
    // старая версия с minApp не ниже новой никому не достанется (pick берёт новую) — убираем, прочие остаются
    list = list.filter((v) => v?.id !== r.id || !(cmpVer(v.minApp, r.minApp) >= 0)) // нечитаемый minApp — оставить
    list.push(r)
  }
  if (list.length > MAX_VIDEOS) throw new Error(`в каталоге стало бы ${list.length} роликов (предел ${MAX_VIDEOS})`)
  const prev = cat?.seq ?? 0
  const next = seq ?? prev + 1
  if (!Number.isInteger(next) || next <= prev) throw new Error(`seq ${next} не больше прежнего ${prev}: клиент такой каталог не примет`)
  return { ...cat, seq: next, videos: list }
}

/** Внутренний каталог из файла-конверта; подпись сверяется ключом key (защита от каталога, подписанного тестовым ключом). */
export function readSignedCatalog(file, key) {
  if (!fs.existsSync(file)) throw new Error(`${file}: нет файла каталога`)
  let env
  try {
    env = JSON.parse(readLimited(file, MAX_CATALOG, 'каталог').toString('utf8'))
  } catch (e) {
    throw new Error(`${file}: не каталог (${e.message})`)
  }
  let ok = false
  try {
    ok = typeof env?.body === 'string' && typeof env.sig === 'string' && verifyBytes(Buffer.from(env.body, 'base64'), env.sig, publicRaw(key))
  } catch {
    ok = false
  }
  if (!ok) throw new Error(`${file}: подпись прежнего каталога не сходится с переданным ключом — не тот ключ или каталог подписан тестовым`)
  return JSON.parse(Buffer.from(env.body, 'base64').toString('utf8'))
}

function opt(argv, name) {
  const i = argv.indexOf(name)
  if (i < 0) return undefined
  const v = argv[i + 1]
  if (v === undefined || v.startsWith('--')) throw new Error(`${name}: нужно значение`)
  return v
}

export function main(argv) {
  const run = opt(argv, '--run')
  const dir = opt(argv, '--dir')
  if (!run || !dir) throw new Error('использование: --run <run.json> --dir <папка роликов> [--catalog c.json --key pem --out new.json [--seq N]]')
  const rows = rowsFromRun(path.resolve(run))
  const bad = rows.flatMap((r) => checkRow(r, path.resolve(dir)))
  if (bad.length) throw new Error('проверка не прошла:\n  ' + bad.join('\n  '))
  const files = rows.flatMap((r) => Object.keys(r.files).map((k) => path.join(path.resolve(dir), `${r.id}-${r.version}.${k}`)))
  console.log(`роликов ${rows.length}, файлов ${files.length}; проверка размера и sha256 пройдена`)
  console.log(`релиз ${RELEASE} (после «да» владельца): gh release create ${RELEASE} -R IliaSA101/sosninaalbums-library --title "Ролики справочника F1" ${files.map((f) => `"${f}"`).join(' ')}`)
  const catPath = opt(argv, '--catalog')
  const keyPath = opt(argv, '--key')
  const out = opt(argv, '--out')
  if (!catPath && !keyPath && !out) return
  if (!catPath || !keyPath || !out) throw new Error('для каталога нужны вместе --catalog, --key и --out')
  const key = crypto.createPrivateKey(fs.readFileSync(path.resolve(keyPath)))
  const cat = readSignedCatalog(path.resolve(catPath), key)
  const seqArg = opt(argv, '--seq')
  const next = mergeVideos(cat, rows, seqArg === undefined ? undefined : Number(seqArg))
  const env = sealCatalog(next, key)
  if (!verifyBytes(Buffer.from(env.body, 'base64'), env.sig, publicRaw(key))) throw new Error('самопроверка подписи не прошла')
  fs.writeFileSync(path.resolve(out), JSON.stringify(env) + '\n')
  console.log(`каталог ${out}: seq ${next.seq}, роликов ${next.videos.length}, наборов ${(next.packs ?? []).length}`)
  console.log(`после релиза и push каталога: для каждого файла curl -sL <url> | sha256sum — сверить со строкой каталога`)
}

if (process.argv[1] && import.meta.url === pathToFileURL(process.argv[1]).href) {
  try {
    main(process.argv.slice(2))
  } catch (e) {
    console.error(e.message)
    process.exit(1)
  }
}
