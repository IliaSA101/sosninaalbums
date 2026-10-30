import { baseOf } from './copies'
import type { SpreadView } from './types'

// Набор из ids; тот же объект prev, если состав не изменился (вид приходит новыми массивами на каждую правку).
export function keepSet(prev: Set<string>, ids: readonly string[]): Set<string> {
  if (prev.size === ids.length && ids.every((id) => prev.has(id))) return prev
  return new Set(ids)
}

// Что лист даёт счёту (098): записи мест (photos) и все вхождения файла — места, фоны, стикеры, рамочки.
// Фон с album: true — копия фона альбома на листе, не вхождение листа; сам фон альбома считает Nav (один раз по albumBg).
type Counted = Pick<SpreadView, 'photos'> & {
  backdrops?: { photo: string; album?: boolean }[] | null
  stickers?: { photo?: string }[] | null
  places?: { border?: { photo?: string } | null }[] | null
}

// ID файлов всех вхождений листа (с повторами): зеркало countByView из Go-теста uses_parity_test.go.
export function usesOf(s: Counted): string[] {
  const out = (s.photos ?? []).filter(Boolean)
  for (const b of s.backdrops ?? []) if (!b.album && b.photo) out.push(b.photo)
  for (const st of s.stickers ?? []) if (st.photo) out.push(st.photo)
  for (const p of s.places ?? []) if (p.border?.photo) out.push(p.border.photo)
  return out.map(baseOf)
}

// Число вхождений файла: листы (uses) + фон альбома (albumPhoto, View.albumBg.photo) один раз.
export function useCountOf(uses: ReadonlyMap<string, number>, albumPhoto: string | undefined, id: string): number {
  const b = baseOf(id)
  return (uses.get(b) ?? 0) + (albumPhoto && baseOf(albumPhoto) === b ? 1 : 0)
}

// Фото «в альбоме» (075-16): набор id по листам. Пересчёт идёт по разворотам, чьи объекты сменились
// (дельта вида сохраняет неизменённые — viewdelta.mergeView), а не по всем 3200 фото на каждую правку.
// Набор неизменяемый: пока состав не изменился, возвращается тот же объект — зависимые списки не пересчитываются.
// 098: в набор входят и ID записей мест, и ID файла (baseOf) — фото, стоящее только копией, тоже «в альбоме».
// Число вхождений (uses) считается по файлу; карта новая только после правки счёта.
export class UsedIndex {
  private recs = new Map<string, number>() // записи мест: ID → сколько раз
  private bases = new Map<string, number>() // файлы мест: ID файла → сколько мест
  private counts = new Map<string, number>() // вхождения: ID файла → сколько
  private dirty = false
  private prev: readonly SpreadView[] = []
  private out = new Set<string>()
  private usesOut: ReadonlyMap<string, number> = new Map()

  // Вхождений файла по листам (без фона альбома).
  get uses(): ReadonlyMap<string, number> {
    return this.usesOut
  }

  // Мест, где стоит файл (записи всех копий).
  places(id: string): number {
    return this.bases.get(baseOf(id)) ?? 0
  }

  update(sheets: readonly SpreadView[]): Set<string> {
    if (sheets === this.prev) return this.out
    const now = new Set(sheets)
    const was = new Set(this.prev)
    // touched — ID, чьё наличие в наборе менялось по ходу; итог сверяется с прежним набором
    // (разворот, заменённый своей копией, набор не меняет).
    const touched = new Set<string>()
    for (const s of was) if (!now.has(s)) this.shift(s, -1, touched)
    for (const s of now) if (!was.has(s)) this.shift(s, 1, touched)
    this.prev = sheets
    if (this.differs(touched)) this.out = new Set([...this.recs.keys(), ...this.bases.keys()])
    if (this.dirty) {
      this.usesOut = new Map(this.counts)
      this.dirty = false
    }
    return this.out
  }

  private differs(touched: Set<string>): boolean {
    for (const k of touched) if (this.out.has(k) !== (this.recs.has(k) || this.bases.has(k))) return true
    return false
  }

  // photos от Go бывает null (пустой срез).
  private shift(s: SpreadView, d: 1 | -1, touched: Set<string>) {
    for (const id of s.photos ?? []) {
      bump(this.recs, id, d, touched)
      bump(this.bases, baseOf(id), d, touched)
    }
    for (const id of usesOf(s)) {
      bump(this.counts, id, d)
      this.dirty = true
    }
  }
}

// Счётчик ключа на d; ключ, появившийся или исчезнувший, попадает в touched.
function bump(m: Map<string, number>, id: string, d: 1 | -1, touched?: Set<string>) {
  const c = (m.get(id) ?? 0) + d
  if (c > 0) m.set(id, c)
  else m.delete(id)
  if (c <= 1) touched?.add(id)
}
