// Отдельные фото в окне «Папки» (097, вариант B макета владельца): ветка «Фото отдельно» после папок → добавления
// «21.10, 14:05» → файлы с бледной папкой-источником. Узлы — те же TreeNode, что у папок: флажок, «только», ✕ общие.
// Здесь же тексты тоста и вопроса «Убрать фото?». Пути и имена — только текстом (в разметке без {@html}).
import { folderName } from '../library/names'
import { folderKey } from './folders'
import type { TreeNode } from './tree'
import type { LooseAdded, LooseView } from '../../base/core/types'

export const GROUP_KEY = 'g:loose'
export const GROUP_NAME = 'Фото отдельно'
export const batchKey = (id: string): string => 'b:' + id
export const fileKey = (path: string): string => 'f:' + folderKey(path)

const pad = (n: number) => String(n).padStart(2, '0')

// Подпись добавления: сегодня — «14:05», в этом году — «21.10, 14:05», раньше — «21.10.2025, 14:05». at — Unix-секунды.
export function batchLabel(at: number, now: Date, sec = false): string {
  const d = new Date(at * 1000)
  const time = `${pad(d.getHours())}:${pad(d.getMinutes())}` + (sec ? ':' + pad(d.getSeconds()) : '')
  const year = d.getFullYear() === now.getFullYear()
  if (year && d.getMonth() === now.getMonth() && d.getDate() === now.getDate()) return time
  const day = `${pad(d.getDate())}.${pad(d.getMonth() + 1)}`
  return year ? `${day}, ${time}` : `${day}.${d.getFullYear()}, ${time}`
}

// Подписи всех добавлений: у двух в одну минуту — с секундами «14:05:33», иначе строки не различить.
export function batchLabels(list: readonly LooseView[], now: Date): string[] {
  const plain = list.map((b) => batchLabel(b.at, now))
  const seen = new Map<string, number>()
  for (const l of plain) seen.set(l, (seen.get(l) ?? 0) + 1)
  return plain.map((l, i) => ((seen.get(l) ?? 0) > 1 ? batchLabel(list[i].at, now, true) : l))
}

// «сегодня в 14:05» для подписи без даты: «Добавлено 14:05» читается как число.
export const addedWhen = (label: string): string => (/^[\d:]+$/.test(label) ? `сегодня в ${label}` : label)

// Имя файла и его папки — свои: tree.ts берёт отсюда addedWhen, кольца импорта нет.
const segs = (p: string): string[] => p.split(/[\\/]/).filter(Boolean)
const baseName = (p: string): string => segs(p).at(-1) ?? p

function looseNode(key: string, kind: TreeNode['kind'], name: string, path: string, root: boolean): TreeNode {
  return { key, kind, name, path, own: 0, total: 0, kids: [], root, parent: null }
}

// Ветка «Фото отдельно»; нет добавлений — null. has(photo) — фото в ленте; пока идёт чтение, отсутствующее не «нет на диске».
// why(photo) — причина, по которой найденный файл не прочитан (117): тогда «не читается», а не «нет на диске».
// Повтор ключа (не бывает при инварианте Go) пропускается: ключ строки в списке окна обязан быть один.
export function looseGroup(
  list: readonly LooseView[],
  has: (photo: string) => boolean,
  why: (photo: string) => string | undefined,
  scanning: boolean,
  now: Date,
): TreeNode | null {
  if (list.length === 0) return null
  const g = looseNode(GROUP_KEY, 'group', GROUP_NAME, '', false)
  const labels = batchLabels(list, now)
  const seen = new Set<string>()
  list.forEach((b, i) => {
    const bk = batchKey(b.id)
    if (seen.has(bk)) return
    seen.add(bk)
    const bn = looseNode(bk, 'batch', labels[i], '', true)
    bn.loose = { id: b.id, file: '' }
    bn.parent = g
    for (const f of b.files ?? []) {
      const fk = fileKey(f.path)
      if (seen.has(fk)) continue
      seen.add(fk)
      const here = has(f.photo)
      const fn = looseNode(fk, 'file', baseName(f.path), f.path, true)
      fn.own = fn.total = here ? 1 : 0
      const bad = here || scanning ? undefined : why(f.photo) // пока идёт чтение, причина может быть прошлой
      if (bad) fn.bad = bad
      fn.missing = !here && !scanning && !bad
      const dir = segs(f.path).at(-2)
      if (dir) fn.hint = dir
      fn.loose = { id: b.id, file: f.path, photo: f.photo }
      fn.parent = bn
      bn.kids.push(fn)
      bn.total += fn.total
    }
    if (bn.kids.length === 0) return
    g.kids.push(bn)
    g.total += bn.total
  })
  return g.kids.length > 0 ? g : null
}

// looseGroup с памятью: правка альбома без смены добавлений, ленты, причин и чтения — та же ветка.
export function cachedLoose() {
  let last: unknown[] = []
  let out: TreeNode | null = null
  return (
    list: readonly LooseView[],
    photos: ReadonlyMap<string, unknown>,
    unread: Readonly<Record<string, string>>,
    scanning: boolean,
    now = new Date(),
  ): TreeNode | null => {
    const day = now.toDateString() // в полночь «14:05» становится «21.10, 14:05»
    const key = [list, photos, unread, scanning, day]
    if (key.every((k, i) => k === last[i])) return out
    last = key
    out = looseGroup(
      list,
      (id) => photos.has(id),
      (id) => unread[id],
      scanning,
      now,
    )
    return out
  }
}

// Ключи ветки, добавлений и файлов — то, что фильтр и раскрытие помнят между правками (pruneHidden).
export function looseKeys(list: readonly LooseView[]): Set<string> {
  const out = new Set<string>()
  if (list.length > 0) out.add(GROUP_KEY)
  for (const b of list) {
    out.add(batchKey(b.id))
    for (const f of b.files ?? []) out.add(fileKey(f.path))
  }
  return out
}

// Сколько фото узла (добавления или файла) стоит в альбоме; пропавшие с диска тоже — они на листах.
export function looseInAlbum(n: TreeNode, used: ReadonlySet<string>): number {
  const files = n.kind === 'file' ? [n] : n.kids
  return files.filter((f) => f.loose?.photo && used.has(f.loose.photo)).length
}

// 1 фото «стоит», 2–4 и 5+ — «стоят» (как у папок).
const stand = (n: number) => (n % 10 === 1 && n % 100 !== 11 ? 'стоит' : 'стоят')

// Вопрос «Убрать фото?» (097 п. 7): n — фото узла в альбоме. Добавление из одного файла спрашивает как файл.
export function looseAskText(n: TreeNode, inAlbum: number): string {
  const file = n.kind === 'file' ? n : n.kids.length === 1 ? n.kids[0] : null
  if (file) return `Фото «${file.name}» стоит в альбоме. На листе оно останется, а из ленты пропадёт.`
  const total = n.kids.length
  const stay =
    inAlbum === 1 ? `На листе оно останется, а из ленты пропадут все ${total} фото.` : 'На листах они останутся, а из ленты пропадут.'
  return `${inAlbum} из ${total} фото, добавленных ${addedWhen(n.name)}, ${stand(inAlbum)} в альбоме. ${stay}`
}

const LIMIT = '2 000' // model.MaxLooseFiles

// Тост после добавления (097 п. 2): части через « · »; null — ничего не выбрано (отмена системного окна).
export function looseToast(r: LooseAdded): { text: string; kind: 'info' | 'error' } | null {
  const parts: string[] = []
  if (r.added > 0) parts.push(`Добавлено ${r.added} фото`)
  const place = r.folder ? `в папке «${folderName(r.folder)}»` : 'в папке альбома'
  const only = r.added + r.dup + r.skipped + r.bad + r.over === 0
  if (r.inFolder > 0) parts.push(only ? `Фото уже есть ${place} — не добавлено` : `${r.inFolder} уже есть ${place} — не добавлено`)
  if (r.dup > 0) parts.push(`уже добавлены раньше: ${r.dup}`)
  if (r.skipped > 0) parts.push(`не фото: ${r.skipped}`)
  if (r.bad > 0) parts.push(`не добавлено путей: ${r.bad} — поддерживаются только локальные пути вида C:\\папка`)
  if (r.over > 0) parts.push(`отдельных фото не больше ${LIMIT} — не добавлено ${r.over}`)
  if (parts.length === 0) return null
  const text = parts.join(' · ')
  // Ничего не добавилось из-за пути или предела — как отказ броска папки (красный); остальное — справка.
  const kind = r.added === 0 && (r.bad > 0 || r.over > 0) ? 'error' : 'info'
  return { text: text[0].toUpperCase() + text.slice(1), kind }
}
