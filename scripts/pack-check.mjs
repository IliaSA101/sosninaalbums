#!/usr/bin/env node
// Проверка своего набора библиотеки (спека 143, ступень 1; design §9, 0.31-143-18): те же правила, что у «Файл ▸ Наборы
// библиотеки ▸ Добавить из файла», без программы. Node 22.2+, без зависимостей. Скачивать вместе с папкой pack-check/ рядом
// (или всю папку scripts/): модули base, manifest, content, zip лежат в ней.
//
//   node pack-check.mjs набор.zip [--app x.y.z] [--strict]
//   --app     версия программы для minApp (без него minApp не сверяется с версией, только грамматика)
//   --strict  пропуск предмета тоже считать провалом
//
// Итог: «ставится» (код 0), «пропуск: …» (предмет не ставится, остальные да; код 0, с --strict — 1), «отказ: …» (код 1;
// в том числе когда пропущены все предметы — «в наборе нечего ставить»), 2 — неверный запуск или старый Node.
// Правила манифеста и архива — копия internal/decor/pack (manifest.go, archive.go, eocd.go): паритет держат
// testdata/pack-manifest-cases.json (манифест, блок limits) и testdata/pack-archive-cases.json (архив) — те же файлы читают
// TestManifestCases, TestLimits, TestArchiveCases в Go. Что скрипт не повторяет — 0.31-143-39 в docs/specs/0.31/decisions.md.
// Расхождение с Go, если оно есть, — ошибка этого файла: программа решает.
//
// Границы чтения: размер файла и счётчики архива проверяются до чтения записей, каждая запись идёт потоком
// (в памяти — только первые 4 МБ для заголовка), размер и sha256 — по фактическим байтам, а не по заголовкам zip.
import fs from 'node:fs'
import path from 'node:path'
import zlib from 'node:zlib'
import { fileURLToPath } from 'node:url'
import { LIMITS, PackError, clean, fail } from './pack-check/base.mjs'
import { contentSkip, imageInfo } from './pack-check/content.mjs'
import { emptyErr, manifestVerdict, parseManifest, validId, validName } from './pack-check/manifest.mjs'
import { checkEntries, readEntries, streamEntry } from './pack-check/zip.mjs'

export { LIMITS, PackError, clean, imageInfo, manifestVerdict, parseManifest, validId, validName }

const HEAD_KEEP = 4 << 20 // сколько байт предмета держим для заголовка картинки или JSON

/**
 * Чего не хватает Node: crc32 (22.2), текст числа в JSON.parse (21+), toWellFormed (20+); "" — всё есть.
 * Без них набор получил бы ложное «архив повреждён» или молча принял format: 1.0.
 */
export function nodeProblem(z = zlib, parse = JSON.parse) {
  let src
  try {
    parse('1', (_k, _v, ctx) => {
      src = ctx?.source
    })
  } catch {
    src = undefined
  }
  return typeof z.crc32 === 'function' && src === '1' && typeof ''.toWellFormed === 'function' ? '' : 'нужен Node 22.2 или новее'
}

/**
 * Проверка архива набора (pack.OpenReader + Extract + проверки содержимого). Итог {verdict: ok|skip|reject, error?, id, title,
 * version, installed: [file], skipped: [{file, why}], frames}. Не бросает: любая беда читаемого файла — reject с причиной.
 */
export async function checkZip(file, { app = '' } = {}) {
  let fd
  try {
    fd = fs.openSync(file, 'r')
    const st = fs.fstatSync(fd)
    if (!st.isFile()) throw new PackError('other', 'набор — не обычный файл')
    if (st.size > LIMITS.maxZip) throw new PackError('other', `архив набора больше ${LIMITS.maxZip >> 20} МБ`)
    const files = checkEntries(readEntries(fd, st.size), st.size)
    const mf = files.get('manifest.json')
    if (!mf) throw fail('damaged', 'нет manifest.json')
    if (mf.usize > LIMITS.maxManifest) throw fail('manifest', `больше ${LIMITS.maxManifest >> 10} КБ`)
    const mr = await streamEntry(fd, mf, st.size, LIMITS.maxManifest, LIMITS.maxManifest)
    if (mr.over || mr.crcBad || mr.n !== mf.usize) throw fail('damaged', 'manifest.json не читается')
    const m = parseManifest(mr.head, app)
    for (const it of m.items) {
      const zf = files.get(it.file)
      if (!zf) throw fail('damaged', `нет «${it.file}»`)
      if (zf.usize !== it.size) throw fail('damaged', `размер «${it.file}» не сходится с манифестом`)
    }
    const installed = []
    const skipped = [...m.skipped]
    const seen = new Set()
    let frames = 0
    for (const it of m.items) {
      const r = await streamEntry(fd, files.get(it.file), st.size, it.size, HEAD_KEEP)
      if (r.over || r.crcBad || r.n !== it.size) throw fail('damaged', `«${it.file}» не сходится с манифестом по размеру`)
      if (r.sha !== it.sha256) throw fail('damaged', `«${it.file}» не сходится с манифестом (sha256)`)
      let why = contentSkip(it, r.head)
      const key = `${it.kind} ${r.sha}` // тот же вид и те же байты; шаблон программа сводит ещё и по каноническому JSON
      if (!why && seen.has(key)) why = 'повтор в наборе'
      if (why) {
        skipped.push({ file: it.file, why })
        continue
      }
      seen.add(key)
      installed.push(it.file)
      if (it.kind === 'frame') frames++
    }
    if (installed.length === 0) throw emptyErr(skipped) // программа отказывает целиком (ErrPackEmpty), а не «ставит 0»
    const verdict = skipped.length > 0 ? 'skip' : 'ok'
    return { verdict, id: m.id, title: m.title, version: m.version, installed, skipped, frames }
  } catch (e) {
    if (e instanceof PackError) return { verdict: 'reject', error: e.message, installed: [], skipped: [], frames: 0 }
    return { verdict: 'reject', error: e.code === 'ENOENT' ? 'файл набора не открывается' : 'файл набора не читается', installed: [], skipped: [], frames: 0 }
  } finally {
    if (fd !== undefined) fs.closeSync(fd)
  }
}

/** Строки отчёта: заголовок набора, пропуски и итог. */
export function report(r) {
  if (r.verdict === 'reject') return [`отказ: ${r.error}`]
  const lines = [`набор «${r.title}» (${r.id}, версия ${r.version}): ставится ${r.installed.length}, пропущено ${r.skipped.length}`]
  for (const s of r.skipped) lines.push(`пропуск: ${s.file} — ${s.why}`)
  if (r.frames > 0) lines.push('рамочки: окно прозрачности проверит программа') // здесь только размер, окно читает установка
  lines.push(r.verdict === 'ok' ? 'ставится' : 'ставится с пропусками')
  return lines
}

const USAGE = 'использование: node pack-check.mjs набор.zip [--app x.y.z] [--strict]'

async function main(argv) {
  const problem = nodeProblem()
  if (problem) {
    console.error(problem)
    return 2
  }
  const args = argv.slice()
  const flag = (name) => {
    const i = args.indexOf(name)
    if (i >= 0) args.splice(i, 1)
    return i >= 0
  }
  const strict = flag('--strict')
  const ai = args.indexOf('--app')
  const app = ai >= 0 ? (args.splice(ai, 2)[1] ?? '') : ''
  if (args.length !== 1 || args[0].startsWith('--')) {
    console.error(USAGE)
    return 2
  }
  const r = await checkZip(args[0], { app })
  for (const line of report(r)) console.log(line)
  return r.verdict === 'reject' || (strict && r.verdict === 'skip') ? 1 : 0
}

if (process.argv[1] && path.resolve(process.argv[1]) === fileURLToPath(import.meta.url)) {
  main(process.argv.slice(2)).then((code) => {
    process.exitCode = code
  })
}
