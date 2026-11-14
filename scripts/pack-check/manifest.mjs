// Манифест набора (pack/manifest.go): разбор как json.Unmarshal в структуру Go, грамматики, пределы, пропуск предмета по виду.
import { E, LIMITS, PackError, clean, ext, fail } from './base.mjs'
import { rawField } from './rawfield.mjs'

/** Число JSON с исходным текстом: целое поле Go принимает только литерал без точки и порядка (1.0 и 1e2 — ошибка). */
export class Num {
  constructor(src) {
    this.src = src
  }
}

export function parseJson(text) {
  // reviver с source (Node 22+) оставляет текст числа; без него — обычные числа и проверка Number.isInteger
  return JSON.parse(text, (_k, v, ctx) => (typeof v === 'number' && ctx?.source !== undefined ? new Num(ctx.source) : v))
}

const I64_MIN = -(1n << 63n)
const I64_MAX = (1n << 63n) - 1n

export function asInt(v) {
  if (v instanceof Num) {
    if (!/^-?\d+$/.test(v.src)) throw new TypeError('не целое')
    const b = BigInt(v.src)
    if (b < I64_MIN || b > I64_MAX) throw new TypeError('вне int64')
    return Number(b)
  }
  if (typeof v === 'number' && Number.isInteger(v)) return v
  throw new TypeError('не целое')
}

export function asStr(v) {
  if (typeof v !== 'string') throw new TypeError('не строка')
  return v.toWellFormed() // Go заменяет одиночные суррогаты на U+FFFD
}

// Имя поля Go сопоставляет без учёта регистра и по простой свёртке: «K» (Кельвин) = k, «ſ» = s.
const fold = (k) => k.replace(/K/g, 'k').replace(/ſ/g, 's').toLowerCase()

/** Два ключа объекта сошлись в одно поле после свёртки регистра: Go берёт последний, JS — по порядку; проверка строже программы. */
export class DupKeyError extends TypeError {
  constructor(field) {
    super(`повтор ключа поля «${field}» (в другом регистре)`)
  }
}

/**
 * Как json.Unmarshal в структуру: поля по имени без регистра, null — пропуск, чужой тип — ошибка.
 * strict — объект, где два ключа дают одно поле (`order` и `ORDER`), отвергается: строже программы (аудит 170, п. 1),
 * зеркало не одобряет то, что Go прочтёт иначе. Включён для объявлений тем; шапка и предметы повторяют Go
 * один в один (общие случаи «ключ themes в другом регистре», «ключ Topic в другом регистре»).
 */
export function decodeObject(v, spec, strict = false) {
  const out = {}
  if (v === null) return out
  if (typeof v !== 'object' || Array.isArray(v) || v instanceof Num) throw new TypeError('не объект')
  const seen = new Set()
  for (const [k, val] of Object.entries(v)) {
    const f = Object.keys(spec).find((n) => n.toLowerCase() === fold(k))
    if (f === undefined) continue
    if (strict && seen.has(f)) throw new DupKeyError(f)
    seen.add(f)
    // null — пропуск, как у строки и числа Go; кроме RawMessage (keepNull): он получает «null» и перекрывает прежнее
    if (val === null && !spec[f].keepNull) continue
    out[f] = spec[f](val)
  }
  return out
}

/** `topic` — RawMessage в Go: не строка не роняет разбор, предмет остаётся в базе (themes: topicOf). */
const asTopic = Object.assign((v) => (typeof v === 'string' ? v.toWellFormed() : ''), { keepNull: true })
const ITEM = { file: asStr, kind: asStr, theme: asStr, name: asStr, sha256: asStr, size: asInt, topic: asTopic }
const asItems = (v) => {
  if (!Array.isArray(v)) throw new TypeError('не массив')
  return v.map((x) => decodeObject(x, ITEM))
}
const HEAD = { format: asInt, id: asStr, version: asInt, minApp: asStr, title: asStr, about: asStr, author: asStr, license: asStr, source: asStr, items: asItems }

const sizeText = (n) => (n >= 1 << 20 ? `${n >> 20} МБ` : `${n >> 10} КБ`)

function deviceName(id) {
  id = id.toLowerCase()
  if (['con', 'prn', 'aux', 'nul', 'conin$', 'conout$'].includes(id)) return true
  const r = [...id]
  return r.length === 4 && (id.startsWith('com') || id.startsWith('lpt')) && '0123456789¹²³'.includes(r[3])
}

/** id набора ^[a-z0-9][a-z0-9-]{2,39}$ и не устройство Windows (из него строится имя файла реестра). */
export const validId = (id) => /^[a-z0-9][a-z0-9-]{2,39}$/.test(id) && !deviceName(id)

/** Имя предмета: до 3 частей через «/», часть ^[a-z0-9][a-z0-9._-]{0,63}$, без «..». */
export const validName = (name) => /^[a-z0-9][a-z0-9._-]{0,63}(\/[a-z0-9][a-z0-9._-]{0,63}){0,2}$/.test(name) && !name.includes('..')

const validSum = (s) => /^[0-9a-f]{64}$/.test(s)

function parseVer(s) {
  const parts = s.split('.')
  if (parts.length !== 3 || parts.some((p) => !/^\d{1,6}$/.test(p))) return null
  return parts.map(Number)
}

const cmpVer = (a, b) => {
  for (let i = 0; i < 3; i++) if (a[i] !== b[i]) return a[i] > b[i] ? 1 : -1
  return 0
}

/** Причина, по которой программа предмет не ставит; "" — ставит (pack.itemSkip). */
function itemSkip(it) {
  const e = ext(it.file)
  switch (it.kind) {
    case 'bg':
    case 'sticker':
      if (e === '.svg') return 'SVG в наборах не принимается — нужен PNG или JPEG'
      if (e !== '.png' && e !== '.jpg' && e !== '.jpeg') return 'не картинка JPEG или PNG'
      break
    case 'frame':
      if (e === '.svg') return 'SVG в наборах не принимается — нужен PNG'
      if (e !== '.png') return 'рамочка в наборе — только PNG с прозрачностью'
      break
    case 'spread':
      if (e !== '.json') return 'шаблон разворота — не .json'
      break
    case 'print':
      if (e !== '.saprint') return 'типография в наборе — не .saprint'
      break
    case 'album':
      if (e !== '.json') return 'макет альбома — не .json'
      break
    default:
      return 'незнакомый вид — нужна новая версия программы'
  }
  const limit = LIMITS.kind[it.kind]
  return it.size > limit ? `больше предела набора (${sizeText(limit)})` : ''
}

// ---- темы наборов (pack/themes.go, спека 170) ----

export const BUILTIN_THEMES = ['basic', 'wedding', 'kids', 'kindergarten', 'school', 'grad', 'holiday']
export const SERVICE_THEMES = ['recent', 'all', 'new', 'get', 'mine', 'brand', 'legacy', 'constructor']
export const RESERVED_NAMES = ['Недавние', 'Все', 'Новые', 'Можно скачать', 'Мои', 'Фирменные', 'Прежние макеты', 'Универсальные', 'Свадьба', 'Дети', 'Детский сад', 'Школа', 'Выпускной', 'Праздник']
const MAX_REASONS = 16 // причин в журнале; остальное — числом

/** id темы набора: ^[a-z]{1,16}$, не встроенный и не служебный (pack.ValidThemeID). */
export const validThemeId = (id) => /^[a-z]{1,16}$/.test(id) && !BUILTIN_THEMES.includes(id) && !SERVICE_THEMES.includes(id)

const LETTER = 'A-Za-zА-яЁё'

/**
 * Имя темы — белый список (pack.ValidThemeName, 170-28): кириллица, латиница, цифры, пробел, «-»; есть буква; в слове одна
 * письменность; без пробела по краям и двух подряд; 1…20 знаков.
 */
export function validThemeName(name) {
  if (name.length < 1 || name.length > LIMITS.maxThemeName) return false
  if (!new RegExp(`^[${LETTER}0-9 -]+$`).test(name)) return false
  if (name.startsWith(' ') || name.endsWith(' ') || name.includes('  ')) return false
  if (!new RegExp(`[${LETTER}]`).test(name)) return false
  return name.split(/[ -]/).every((w) => !(/[A-Za-z]/.test(w) && /[А-яЁё]/.test(w)))
}

/** Регистр по белому списку, как pack.themeFold: toLowerCase расходится с Go на «İ» и конечной «Σ». */
export const themeFold = (s) => s.replace(/[A-ZА-Я]/g, (c) => String.fromCharCode(c.charCodeAt(0) + 0x20)).replace(/Ё/g, 'ё')

const THEME = { id: asStr, name: asStr, base: asStr, order: asInt }

/** Одна запись: [тема, ""] или [null, причина]. Порядок проверок — как pack.themeDecl. */
function themeDecl(v, ids, names) {
  let d
  try {
    d = { id: '', name: '', base: '', ...decodeObject(v, THEME, true) }
  } catch (e) {
    return [null, e instanceof DupKeyError ? 'повтор' : 'тип'] // повтор ключа — слово словаря «повтор», как у id
  }
  if (!validThemeId(d.id)) return [null, 'id']
  if (ids.has(d.id)) return [null, 'повтор']
  if (!validThemeName(d.name) || names.has(themeFold(d.name))) return [null, 'имя']
  const base = d.base === '' ? 'basic' : d.base
  if (!BUILTIN_THEMES.includes(base)) return [null, 'база']
  const order = d.order === undefined ? LIMITS.themeOrderDef : d.order
  if (order < LIMITS.themeOrderMin || order > LIMITS.themeOrderMax) return [null, 'order']
  return [{ id: d.id, name: d.name, base, order }, '']
}

/**
 * Объявления из сырого отрезка `themes` (Buffer или undefined), pack.ParseThemes: больше 4 КиБ — поле пропущено целиком;
 * не массив — пусто; первые 16 записей, каждая отдельно. Итог: темы, причины словами словаря и число пропущенных записей.
 */
export function parseThemes(raw) {
  const out = { themes: [], why: [], n: 0 }
  if (raw === undefined) return out
  if (raw.length > LIMITS.maxThemesRaw) return { ...out, why: ['размер'], n: 1 }
  let recs
  try {
    recs = parseJson(raw.toString('utf8'))
  } catch {
    return { ...out, why: ['тип'], n: 1 }
  }
  if (recs === null) return out
  if (!Array.isArray(recs)) return { ...out, why: ['тип'], n: 1 }
  const ids = new Set()
  const names = new Set(RESERVED_NAMES.map(themeFold))
  for (const [i, r] of recs.entries()) {
    if (i >= LIMITS.maxThemes) {
      out.n += recs.length - i
      if (out.why.length < MAX_REASONS) out.why.push('предел')
      break
    }
    const [t, why] = themeDecl(r, ids, names)
    if (why) {
      out.n++
      if (out.why.length < MAX_REASONS) out.why.push(why)
      continue
    }
    ids.add(t.id)
    names.add(themeFold(t.name))
    out.themes.push(t)
  }
  return out
}

/** Тема предмета из `topic`: id из объявлений манифеста или встроенная тема, иначе "" (pack.topicOf). */
const topicOf = (topic, themes) => (BUILTIN_THEMES.includes(topic) || themes.some((t) => t.id === topic) ? topic : '')

export function emptyErr(skips) {
  if (skips.length === 0) return new PackError('empty', E.empty)
  let msg = `${clean(skips[0].file, 64)} — ${skips[0].why}`
  if (skips.length > 1) msg += ` (и ещё ${skips.length - 1})`
  return fail('empty', msg)
}

/** Разбор и проверка манифеста (pack.ParseManifest): app — версия программы x.y.z, иное — minApp не сверяется. */
export function parseManifest(data, app = '') {
  if (data.length > LIMITS.maxManifest) throw fail('manifest', `больше ${LIMITS.maxManifest >> 10} КБ`)
  let m
  try {
    m = decodeObject(parseJson(Buffer.from(data).toString('utf8')), HEAD)
  } catch {
    throw fail('manifest')
  }
  m = { format: 0, id: '', version: 0, minApp: '', title: '', about: '', author: '', license: '', source: '', items: [], ...m }
  if (m.format > LIMITS.format) throw new PackError('newer', E.newer)
  if (m.format !== LIMITS.format) throw fail('manifest', 'нет format')
  if (!validId(m.id)) throw fail('manifest', 'недопустимый id набора')
  if (m.version < 1 || m.version > 999999) throw fail('manifest', 'версия набора — от 1 до 999999')
  const need = parseVer(m.minApp)
  if (!need) throw fail('manifest', 'minApp — x.y.z')
  const have = parseVer(app)
  if (have && cmpVer(need, have) > 0) throw new PackError('appOld', `${E.appOld} ${m.minApp} или новее`)
  m.title = clean(m.title, 60)
  if (m.title === '') throw fail('manifest', 'у набора нет названия')
  m.about = clean(m.about, 300)
  m.author = clean(m.author, 60)
  m.license = clean(m.license, 40)
  m.source = clean(m.source, 200)
  if (m.items.length === 0 || m.items.length > LIMITS.maxItems) throw fail('manifest', `предметов должно быть от 1 до ${LIMITS.maxItems}`)
  const { themes, why: themeSkips, n: themeSkipN } = parseThemes(rawField(Buffer.from(data), (k) => fold(k) === 'themes'))
  const seen = new Set()
  const items = []
  const skipped = []
  let total = 0
  for (const it0 of m.items) {
    const it = { file: '', kind: '', theme: '', name: '', sha256: '', size: 0, topic: '', ...it0 }
    if (!validName(it.file)) throw fail('manifest', `недопустимое имя файла «${clean(it.file, 64)}»`)
    const low = it.file.toLowerCase()
    if (seen.has(low)) throw fail('manifest', `файл «${it.file}» указан дважды`)
    seen.add(low)
    if (!validSum(it.sha256) || it.size < 1) throw fail('manifest', `у «${it.file}» нет sha256 или размера`)
    const why = itemSkip(it)
    if (why) {
      skipped.push({ file: it.file, why })
      continue
    }
    total += it.size
    if (total > LIMITS.maxTotal) throw fail('manifest', `распакованным больше ${LIMITS.maxTotal >> 30} ГБ`)
    if (!/^[a-z]{1,16}$/.test(it.theme)) it.theme = 'basic' // тему знает программа: незнакомую сводит к «Универсальным»
    it.name = clean(it.name, 60)
    it.topic = topicOf(it.topic, themes)
    items.push(it)
  }
  if (items.length === 0) throw emptyErr(skipped)
  return { ...m, items, skipped, themes, themeSkips, themeSkipN }
}

/** Итог разбора для сверки с Go (testdata/pack-manifest-cases.json): ok или code+error, поля набора, предметы, пропуски. */
export function manifestVerdict(data, app = '') {
  try {
    const m = parseManifest(data, app)
    const { title, about, author, license, source } = m
    // topic — только непустой: ожидания старых случаев с целым списком предметов не меняются
    const items = m.items.map(({ file, kind, theme, name, size, topic }) => ({ file, kind, theme, name, size, ...(topic ? { topic } : {}) }))
    const { themes, themeSkips, themeSkipN } = m
    return { ok: true, code: '', error: '', title, about, author, license, source, items, skipped: m.skipped, themes, themeSkips, themeSkipN }
  } catch (e) {
    if (e instanceof PackError) return { ok: false, code: e.code, error: e.message }
    throw e
  }
}
