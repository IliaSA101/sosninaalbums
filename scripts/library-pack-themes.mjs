// Темы набора при выпуске (спека 170, design §2.1, §6): правила объявлений `themes` и `topic` предметов
// manifest.src.json и сверка с прежним каталогом. Правила берутся из зеркала разбора программы
// (pack-check/manifest.mjs — те же id, имя по белому списку, регистр), поверх них — строже программы:
// плохая запись здесь отказ выпуску, а не пропуск с строкой в log.txt у фотографа. Только node:*.
//
//   srcThemes(src)                      объявления и ссылки предметов -> [{id,name,base,order?}]; нарушения — одно исключение
//   checkCatalogThemes(themes, packs, id)  сверка со строками других наборов прежнего каталога (поле themes строки)
//   themeRow(themes)                    список для строки каталога: [{id,name,base}] (без order, §2.3)
//   checkMirror(manifest)               самопроверка: зеркало разбора программы принимает ровно то, что записано
import { BUILTIN_THEMES, RESERVED_NAMES, parseManifest, themeFold, validThemeId, validThemeName } from './pack-check/manifest.mjs'

export const MAX_THEMES = 16 // объявлений в наборе (pack.MaxThemes)
export const MAX_PACK_THEMES = 64 // тем наборов в программе (pack.MaxPackThemes): остальные программа не покажет
const DECL_KEYS = ['id', 'name', 'base', 'order']
const isObject = (v) => v !== null && typeof v === 'object' && !Array.isArray(v)

// Латиница и цифры, неотличимые на глаз от кириллицы: «Bce» ≈ «Все», «Mope» ≈ «Море», «3има» ≈ «Зима» (аудит 170, п. 2).
// Белый список имени пропускает слово целиком латиницей, поэтому имена сравниваются по «скелету».
const LOOKALIKE = { A: 'А', B: 'В', C: 'С', E: 'Е', H: 'Н', K: 'К', M: 'М', O: 'О', P: 'Р', T: 'Т', X: 'Х', a: 'а', c: 'с', e: 'е', o: 'о', p: 'р', x: 'х', y: 'у', 3: 'З', 0: 'О' }
/** Скелет имени для сверки: двойники кириллицы заменены, затем регистр по правилу программы. */
export const themeSkeleton = (name) => themeFold(String(name).replace(/[ABCEHKMOPTXacepoxy30]/g, (c) => LOOKALIKE[c]))
const q = (s) => JSON.stringify(String(s).slice(0, 40))

/**
 * Объявления и ссылки manifest.src.json. Правила: ≤ 16 объявлений; запись — объект только с id, name, base, order;
 * id и имя — как у программы; база — явно, одна из семи встроенных; order — целое 1…99 (нет — решит программа, 50);
 * ни повторов id, ни имён-двойников (без учёта регистра) и имён встроенных строк. `topic` предмета — объявленный id,
 * а `theme` предмета — база этой темы (в theme идёт база: её видят 0.29–0.39); id темы набора в `theme` не пишется.
 * Объявленная тема без предметов — отказ. Все нарушения — одним исключением (один прогон — все причины).
 */
export function srcThemes(src) {
  const errs = []
  const raw = src.themes ?? []
  if (!Array.isArray(raw)) throw new Error('themes: нужен список объявлений')
  if (raw.length > MAX_THEMES) errs.push(`themes: не больше ${MAX_THEMES} объявлений, а их ${raw.length}`)
  const names = new Set(RESERVED_NAMES.map(themeSkeleton))
  const decls = new Map()
  const themes = []
  raw.forEach((d, i) => {
    const at = `themes[${i}]`
    if (!isObject(d)) {
      errs.push(`${at}: не объект`)
      return
    }
    for (const k of Object.keys(d)) if (!DECL_KEYS.includes(k)) errs.push(`${at}: неизвестное поле ${q(k)}`)
    if (typeof d.id !== 'string' || !validThemeId(d.id)) errs.push(`${at} id ${q(d.id)}: нужно ^[a-z]{1,16}$, не встроенная и не служебная тема`)
    else if (decls.has(d.id)) errs.push(`${at} id ${q(d.id)} повторяется`)
    if (typeof d.name !== 'string' || !validThemeName(d.name)) errs.push(`${at} имя ${q(d.name)}: кириллица, латиница, цифры, пробел и «-», есть буква, одна письменность в слове, 1…20 знаков`)
    else if (names.has(themeSkeleton(d.name))) errs.push(`${at} имя ${q(d.name)} совпадает с другой темой или встроенной строкой списка (латиница и цифры, похожие на кириллицу, считаются ею)`)
    if (typeof d.base !== 'string' || !BUILTIN_THEMES.includes(d.base)) errs.push(`${at} base ${q(d.base)}: нужна одна из встроенных тем (${BUILTIN_THEMES.join(', ')})`)
    if (d.order !== undefined && !(Number.isInteger(d.order) && d.order >= 1 && d.order <= 99)) errs.push(`${at} order ${q(d.order)}: нужно целое 1…99`)
    if (typeof d.id === 'string') decls.set(d.id, d)
    if (typeof d.name === 'string') names.add(themeSkeleton(d.name))
    themes.push({ id: d.id, name: d.name, base: d.base, ...(d.order === undefined ? {} : { order: d.order }) })
  })
  const used = new Set()
  for (const it of src.items ?? []) {
    const eff = it.theme ?? 'basic' // как buildPack: без theme — «Универсальные»
    if (it.topic !== undefined) {
      if (typeof it.topic !== 'string' || !decls.has(it.topic)) errs.push(`${q(it.file)}: topic ${q(it.topic)} без объявления в themes`)
      else {
        used.add(it.topic)
        if (eff !== decls.get(it.topic).base) errs.push(`${q(it.file)}: theme ${q(eff)} при topic ${q(it.topic)}: в theme идёт база темы (${decls.get(it.topic).base})`)
      }
    } else if (decls.has(eff)) errs.push(`${q(it.file)}: theme ${q(eff)} — id темы набора: в theme идёт база, id — в topic`)
  }
  for (const id of decls.keys()) if (!used.has(id)) errs.push(`тема ${q(id)} объявлена, но ни один предмет не ссылается на неё в topic`)
  if (errs.length > 0) throw new Error(`темы набора:\n - ${errs.join('\n - ')}`)
  return themes
}

/** Список объявлений для строки каталога (§2.3): [{id,name,base}] без order; клиент читает из него только число. */
export const themeRow = (themes) => themes.map(({ id, name, base }) => ({ id, name, base }))

/**
 * Сверка с прежним каталогом (170-21): строки других наборов с полем themes. Один id — одно имя и одна база;
 * разные id — разные имена (без учёта регистра); тем наборов в каталоге не больше 64 (остальные программа не покажет).
 * Строка самого набора пропускается: она заменяется. Поле строки не список — не мешает.
 */
export function checkCatalogThemes(themes, packs, setId) {
  if (themes.length === 0) return
  const errs = []
  const seen = new Map() // id -> {name, base, set}
  for (const p of packs) {
    if (p.id === setId || !Array.isArray(p.themes)) continue
    for (const t of p.themes) if (isObject(t) && typeof t.id === 'string' && !seen.has(t.id)) seen.set(t.id, { name: t.name, base: t.base, set: p.id })
  }
  const byName = new Map([...seen].map(([id, t]) => [themeSkeleton(t.name), { id, set: t.set }]))
  for (const t of themes) {
    const o = seen.get(t.id)
    if (o) {
      if (o.name !== t.name) errs.push(`тема ${q(t.id)}: имя ${q(t.name)}, а в наборе ${o.set} — ${q(o.name)}: один id — одно имя`)
      if (o.base !== t.base) errs.push(`тема ${q(t.id)}: база ${q(t.base)}, а в наборе ${o.set} — ${q(o.base)}: один id — одна база`)
      continue
    }
    const twin = byName.get(themeSkeleton(t.name))
    if (twin) errs.push(`тема ${q(t.id)}: имя ${q(t.name)} уже у темы ${q(twin.id)} набора ${twin.set}: разные id — разные имена`)
  }
  const total = new Set([...seen.keys(), ...themes.map((t) => t.id)]).size
  if (total > MAX_PACK_THEMES) errs.push(`тем наборов в каталоге ${total}, а программа показывает ${MAX_PACK_THEMES}`)
  if (errs.length > 0) throw new Error(`темы набора против прежнего каталога:\n - ${errs.join('\n - ')}`)
}

/**
 * Самопроверка собранного манифеста зеркалом разбора программы: все объявления приняты как записаны (порядок по
 * умолчанию — 50), пропусков нет, `topic` каждого предмета дошёл. Расхождение — ошибка инструмента, а не набора.
 */
export function checkMirror(manifest) {
  if (!manifest.themes?.length) return
  let m
  try {
    m = parseManifest(Buffer.from(JSON.stringify(manifest)))
  } catch (e) {
    throw new Error(`темы набора: зеркало разбора программы не приняло манифест: ${e.message}`)
  }
  const want = manifest.themes.map((t) => ({ ...t, order: t.order ?? 50 }))
  if (m.themeSkipN > 0 || JSON.stringify(m.themes) !== JSON.stringify(want)) {
    throw new Error(`темы набора: программа примет не всё (пропущено ${m.themeSkipN}: ${m.themeSkips.join(', ')}); расхождение с правилами инструмента — ошибка pack-check`)
  }
  const lost = manifest.items.filter((it, i) => (it.topic ?? '') !== (m.items.find((x) => x.file === it.file)?.topic ?? ''))
  if (lost.length > 0) throw new Error(`темы набора: topic потерян у ${lost.length} предметов (${lost[0].file}…): расхождение с pack-check`)
}
