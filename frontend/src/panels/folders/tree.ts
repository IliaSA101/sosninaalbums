// Дерево окна «Папки» (спека 093): узлы строятся по путям фото, Go не нужен. Ключ узла — `d:<folderKey(папки)>`;
// у отдельных фото (097, ветка — loose.ts) — `g:loose` (ветка), `b:<id>` (добавление), `f:<folderKey(файла)>`.
// Фильтр хранит ключи узлов, чьи «свои» фото скрыты: проверка фото — поиск по ключу его папки (и файла, если скрыт файл).
import { folderName } from '../library/names'
import { addedWhen } from './loose'
import { folderKey } from './folders'
import type { LooseView } from '../../base/core/types'

export type NodeKind = 'folder' | 'group' | 'batch' | 'file'

export interface TreeNode {
  key: string
  kind: NodeKind
  name: string
  path: string // как пришло: у корня — строка View.folders (её ждёт RemoveFolder), у подпапки — путь из пути фото
  own: number // фото в самой папке
  total: number // с подпапками
  kids: TreeNode[]
  root: boolean // добавлено пользователем: у строки есть ✕
  parent: TreeNode | null
  hint?: string // бледная родительская папка: две добавленные папки с одним именем; у отдельного фото — его папка
  missing?: boolean // отдельное фото: чтение кончилось, а в ленте его нет (097 п. 9)
  loose?: { id: string; file: string; photo?: string } // добавление и файл для RemoveLoose (file '' — всё добавление)
}

type WithPath = { path: string; id?: string }

export const nodeKey = (dir: string): string => 'd:' + folderKey(dir)

// Счётчик вычислений ключа (тест «key memo»): фильтр на каждую правку не должен резать строки заново.
export const treeStats = { keyCalc: 0 }

// Ключ папки фото живёт на самом объекте фото: Go присылает новый список объектов — память уходит вместе с ним.
const dirKeys = new WeakMap<object, string>()

export function photoDirKey(p: WithPath): string {
  let k = dirKeys.get(p)
  if (k === undefined) {
    treeStats.keyCalc++
    k = 'd:' + folderKey(dirOf(p.path))
    dirKeys.set(p, k)
  }
  return k
}

// Ключ файла (отдельное фото): тоже на объекте фото.
const fileKeys = new WeakMap<object, string>()

export function photoFileKey(p: WithPath): string {
  let k = fileKeys.get(p)
  if (k === undefined) {
    k = 'f:' + folderKey(p.path)
    fileKeys.set(p, k)
  }
  return k
}

function dirOf(path: string): string {
  const i = Math.max(path.lastIndexOf('/'), path.lastIndexOf('\\'))
  return i < 0 ? '' : path.slice(0, i)
}

// Естественный порядок имён, как в ленте: «Часть 2» раньше «Часть 10».
const collator = new Intl.Collator('ru', { numeric: true, sensitivity: 'base' })

function folderNode(key: string, name: string, path: string, root: boolean): TreeNode {
  return { key, kind: 'folder', name, path, own: 0, total: 0, kids: [], root, parent: null }
}

// Дерево: корни — добавленные папки в порядке списка; под ними — только папки, где есть фото (сами или глубже).
// Повтор корня (старый список) не даёт второй строки. Вложенный корень — своя строка, фото достаётся ближайшему: Go оставляет
// подпапку отдельным корнем, если сканер в неё не заходит (имя на «.», имя устройства, ссылка или junction).
// Фото из View.loose в дерево папок не попадает, даже если лежит под корнем за такой подпапкой (A/.edits/x.jpg):
// Go держит его отдельным, считается оно только в ветке «Фото отдельно» (иначе двойной счёт, флажок корня его прячет).
export function buildTree(roots: readonly string[], photos: readonly WithPath[], loose: readonly LooseView[] = []): TreeNode[] {
  const nodes = new Map<string, TreeNode>()
  const out: TreeNode[] = []
  for (const r of roots) {
    const key = nodeKey(r)
    if (nodes.has(key)) continue
    const n = folderNode(key, folderName(r), r, true)
    nodes.set(key, n)
    out.push(n)
  }
  // Папка фото → её узел (null — вне всех корней); подъём к корню — один раз на папку, не на фото.
  const byDir = new Map<string, TreeNode | null>()
  const apart = looseIds(loose)
  for (const ph of photos) {
    if (ph.id !== undefined && apart.has(ph.id)) continue
    const dk = photoDirKey(ph)
    let n = byDir.get(dk)
    if (n === undefined) {
      n = attach(nodes, dk, ph.path)
      byDir.set(dk, n)
    }
    if (n) n.own++
  }
  for (const n of out) finish(n)
  markSameNames(out)
  return out
}

// id фото, которые Go держит отдельными (View.loose).
function looseIds(loose: readonly LooseView[]): Set<string> {
  const ids = new Set<string>()
  for (const b of loose) for (const f of b.files ?? []) ids.add(f.photo)
  return ids
}

// Подъём от папки фото до ближайшего известного узла; недостающие звенья создаются, только если дошли до корня.
function attach(nodes: Map<string, TreeNode>, dk: string, photoPath: string): TreeNode | null {
  const chain: string[] = []
  let k = dk.slice(2)
  for (;;) {
    const found = nodes.get('d:' + k)
    if (found) return link(nodes, found, chain, photoPath)
    chain.push(k)
    const i = k.lastIndexOf('/')
    if (i < 0) return null
    k = k.slice(0, i)
  }
}

function link(nodes: Map<string, TreeNode>, top: TreeNode, chain: string[], photoPath: string): TreeNode {
  if (chain.length === 0) return top
  // Имена — из исходного пути фото (регистр как на диске); строчные буквы ключа «/» не добавляют, звенья совпадают.
  const sep = photoPath.includes('\\') ? '\\' : '/'
  const segs = dirOf(photoPath).split(/[\\/]/)
  let parent = top
  for (let j = chain.length - 1; j >= 0; j--) {
    const k = chain[j]
    const keySegs = k.split('/')
    const depth = keySegs.length
    const name = segs[depth - 1] ?? keySegs[depth - 1]
    const n = folderNode('d:' + k, name, segs.length >= depth ? segs.slice(0, depth).join(sep) : k, false)
    n.parent = parent
    parent.kids.push(n)
    nodes.set(n.key, n)
    parent = n
  }
  return parent
}

function finish(n: TreeNode): number {
  let t = n.own
  for (const k of n.kids) t += finish(k)
  n.kids.sort((a, b) => collator.compare(a.name, b.name))
  n.total = t
  return t
}

// Одинаковые имена добавленных папок: у каждой бледно родительская папка (полный путь — в подсказке строки).
function markSameNames(roots: TreeNode[]) {
  const seen = new Map<string, number>()
  for (const r of roots) seen.set(r.name.toLowerCase(), (seen.get(r.name.toLowerCase()) ?? 0) + 1)
  for (const r of roots) {
    if ((seen.get(r.name.toLowerCase()) ?? 0) < 2) continue
    const up = dirOf(r.path.replace(/[\\/]+$/, ''))
    if (up) r.hint = folderName(up)
  }
}

// buildTree с памятью: правка альбома даёт новый view.folders того же состава — дерево не перестраивается.
export function cachedTree() {
  let photos: unknown = null
  let key = ''
  let out: TreeNode[] = []
  return (roots: readonly string[], list: readonly WithPath[], loose: readonly LooseView[] = []): TreeNode[] => {
    // Отдельные — в ключе по составу, а не по ссылке: View.loose приходит новым массивом на каждый вид.
    const k = roots.map(folderKey).join('\n') + '\u0001' + [...looseIds(loose)].sort().join('\n')
    if (list === photos && k === key) return out
    photos = list
    key = k
    out = buildTree(roots, list, loose)
    return out
  }
}

export interface Row {
  node: TreeNode
  depth: number
}

// Плоский список строк: дети — только под раскрытым узлом.
export function rows(forest: readonly TreeNode[], isOpen: (n: TreeNode) => boolean): Row[] {
  const out: Row[] = []
  const walk = (n: TreeNode, depth: number) => {
    out.push({ node: n, depth })
    if (n.kids.length > 0 && isOpen(n)) for (const k of n.kids) walk(k, depth + 1)
  }
  for (const n of forest) walk(n, 0)
  return out
}

// Строки с фото (своими): одна — фильтровать нечего, флажков нет (как 054 при одной папке).
export function photoRows(forest: readonly TreeNode[]): number {
  let c = 0
  const walk = (n: TreeNode) => {
    if (n.own > 0) c++
    for (const k of n.kids) walk(k)
  }
  for (const n of forest) walk(n)
  return c
}

// Показанные фото узла с подпапками; один проход по дереву на смену скрытых.
export function visibleCounts(forest: readonly TreeNode[], hidden: ReadonlySet<string>): Map<TreeNode, number> {
  const m = new Map<TreeNode, number>()
  const walk = (n: TreeNode): number => {
    let v = hidden.has(n.key) ? 0 : n.own
    for (const k of n.kids) v += walk(k)
    m.set(n, v)
    return v
  }
  for (const n of forest) walk(n)
  return m
}

export type Check = 'on' | 'off' | 'mixed'

export function checkState(n: TreeNode, visible: number): Check {
  return visible === n.total ? 'on' : visible === 0 ? 'off' : 'mixed'
}

function eachOwn(n: TreeNode, f: (n: TreeNode) => void) {
  if (n.own > 0) f(n)
  for (const k of n.kids) eachOwn(k, f)
}

function subtreeVisible(n: TreeNode, hidden: ReadonlySet<string>): number {
  let v = 0
  eachOwn(n, (x) => (v += hidden.has(x.key) ? 0 : x.own))
  return v
}

// Флажок: «—» и пустой — показать всё поддерево; галочка — скрыть всё поддерево.
export function toggleSubtree(n: TreeNode, hidden: Set<string>) {
  const show = checkState(n, subtreeVisible(n, hidden)) !== 'on'
  eachOwn(n, (x) => (show ? hidden.delete(x.key) : hidden.add(x.key)))
}

// «только»: в ленте остаётся одно поддерево.
export function onlyThis(forest: readonly TreeNode[], n: TreeNode, hidden: Set<string>) {
  for (const r of forest) eachOwn(r, (x) => hidden.add(x.key))
  eachOwn(n, (x) => hidden.delete(x.key))
}

// Лента без скрытых узлов; ничего не скрыто — тот же массив. Фото вне дерева остаётся. Ключ файла смотрим, только
// если скрыт хоть один отдельный файл. В дереве папок отдельных нет (buildTree их пропускает), поэтому флажок папки
// их не прячет, даже когда они лежат под корнем за «.»-подпапкой; ключи папки и файла не спорят.
export function filterTree<P extends WithPath>(photos: P[], hidden: ReadonlySet<string>): P[] {
  if (hidden.size === 0) return photos
  const files = [...hidden].some((h) => h.startsWith('f:'))
  return photos.filter((p) => !hidden.has(photoDirKey(p)) && !(files && hidden.has(photoFileKey(p))))
}

// filterTree с запоминанием: пока фото и скрытые те же, ответ прежний, и лента не пересчитывается (054).
export function cachedFilterTree() {
  let photos: unknown = null
  let key = ''
  let out: unknown = null
  return <P extends WithPath>(list: P[], hidden: ReadonlySet<string>): P[] => {
    if (hidden.size === 0) return list
    const k = [...hidden].sort().join('\n')
    if (list === photos && k === key) return out as P[]
    photos = list
    key = k
    out = filterTree(list, hidden)
    return out as P[]
  }
}

// Чистка по списку папок, а не по фото: во время чтения фото ещё нет, а скрытое терять нельзя.
// Ключи отдельных (ветка, добавления, файлы) живут, пока они есть в View.loose (looseKeys из loose.ts).
export function pruneHidden(hidden: Set<string>, roots: readonly string[], looseKeys: ReadonlySet<string> = new Set()) {
  const rk = roots.map(folderKey)
  for (const h of [...hidden]) {
    if (!h.startsWith('d:')) {
      if (!looseKeys.has(h)) hidden.delete(h)
      continue
    }
    const k = h.slice(2)
    if (!rk.some((r) => k === r || k.startsWith(r + '/'))) hidden.delete(h)
  }
}

// Сколько фото альбома (места и фоны — набор used) лежит в узле с подпапками: столько уйдёт из ленты со стоящими на листах.
// Выпавшие из ленты (файла нет) не считаются — их и так нет в photos. Вложенный корень (A\.edits рядом с A) — не в поддереве A:
// его фото останутся в ленте и не считаются.
export function inAlbum(n: TreeNode, photos: readonly (WithPath & { id: string })[], used: ReadonlySet<string>): number {
  const keys = new Set<string>()
  const walk = (x: TreeNode) => {
    keys.add(x.key)
    for (const k of x.kids) walk(k)
  }
  walk(n)
  let c = 0
  for (const p of photos) if (used.has(p.id) && keys.has(photoDirKey(p))) c++
  return c
}

// Узел по ключу и его предки (раскрыть перед подсветкой). Нет — ближайший существующий предок по пути ключа.
export function findNode(forest: readonly TreeNode[], key: string): TreeNode | null {
  const all = new Map<string, TreeNode>()
  const walk = (n: TreeNode) => {
    all.set(n.key, n)
    for (const k of n.kids) walk(k)
  }
  for (const n of forest) walk(n)
  for (let k = key; k.length > 2;) {
    const n = all.get(k)
    if (n) return n
    const i = k.lastIndexOf('/')
    if (i < 2) return null
    k = k.slice(0, i)
  }
  return null
}

// Раскрыт без щелчка: добавленная папка и ветка «Фото отдельно»; подпапка и добавление — свёрнуты.
export const openByDefault = (n: Pick<TreeNode, 'kind' | 'root'>): boolean => n.kind === 'group' || (n.kind === 'folder' && n.root)

export function ancestors(n: TreeNode): TreeNode[] {
  const out: TreeNode[] = []
  for (let p = n.parent; p; p = p.parent) out.unshift(p)
  return out
}

// --- тексты окна ---

// «35 (253)»: в самой папке и с подпапками; без подпапок — одно число. Ветка и добавление — одно число, файл — без числа.
export function countParts(n: TreeNode): { own: string; all: string } {
  if (n.kind === 'file') return { own: '', all: '' }
  if (n.kind !== 'folder') return { own: String(n.total), all: '' }
  return n.kids.length > 0 ? { own: String(n.own), all: `(${n.total})` } : { own: String(n.total), all: '' }
}

export function rowTitle(n: TreeNode): string {
  if (n.kind === 'group') return 'Фото, добавленные по одному'
  if (n.kind === 'batch') return 'Добавлено ' + addedWhen(n.name)
  if (n.kind === 'file') return n.missing ? `${n.path}\nНет на диске` : n.path
  return n.kids.length > 0 ? `${n.path}\n${n.own} — в самой папке, ${n.total} — с подпапками` : n.path
}

// Подсказка и имя ✕: папку убирают из списка, отдельные фото — из ленты (097, как в макете).
export const removeTitle = (n: TreeNode): string => (n.kind === 'folder' ? 'Убрать папку' : 'Убрать из ленты')
export const removeLabel = (n: TreeNode): string => (n.kind === 'folder' ? `Убрать папку «${n.name}»` : `Убрать «${n.name}» из ленты`)

// Без папок, но с отдельными фото — «Папки» без числа (097-7).
export function buttonText(folders: number, filtered: boolean): string {
  const t = folders > 0 ? `Папки ${folders}` : 'Папки'
  return filtered ? `${t} · фильтр` : t
}

export function buttonTitle(shown: number, total: number, filtered: boolean): string {
  return filtered ? `В ленте ${shown} из ${total} фото: часть папок скрыта` : 'Папки с фото: добавить, убрать, перечитать'
}

// 1 фото «стоит», 2–4 и 5+ — «стоят» (фото не склоняется).
const stand = (n: number) => (n % 10 === 1 && n % 100 !== 11 ? 'стоит' : 'стоят')

// Вопрос «Убрать папку?»: n — фото альбома в папке, total — все её фото.
export function removeText(node: TreeNode, n: number): string {
  const sub = node.kids.length > 0 ? ' и её подпапок' : ''
  const gone = node.total === 1 ? 'пропадёт 1 фото папки' : `пропадут все ${node.total} фото папки`
  const stay = n === 1 ? 'На листе оно останется' : 'На листах они останутся'
  return `В альбоме ${stand(n)} ${n} фото из «${node.name}»${sub}. ${stay}, а из ленты ${gone}.`
}

// Событие Go `folders:added` (093 контракт): within "" — добавлена; within == dir — уже была; иначе — входит в within.
export interface FolderAdded {
  dir: string
  within: string
  merged: number
  looseIn: number
}

export type AddedCase = 'new' | 'same' | 'inside'

export function addedCase(e: FolderAdded): AddedCase {
  if (!e.within) return 'new'
  return folderKey(e.within) === folderKey(e.dir) ? 'same' : 'inside'
}

// Тост после добавления; '' — без тоста (обычная новая папка). found — строка подпапки есть в дереве (у неё есть фото).
export function addedText(e: FolderAdded, found = true): string {
  const name = folderName(e.dir)
  const c = addedCase(e)
  if (c === 'same') return `«${folderName(e.within)}» уже добавлена — отмечена в списке` // имя как в списке, а не как выбрали
  if (c === 'inside') {
    const head = `«${name}» уже входит в «${folderName(e.within)}»`
    return found ? `${head} — отмечена в списке` : head
  }
  const parts: string[] = []
  if (e.merged > 0) parts.push(`в неё вошли прежние папки: ${e.merged}`)
  if (e.looseIn > 0) parts.push(`отдельные фото из неё (${e.looseIn}) теперь в папке`)
  return parts.length ? `Папка «${name}» добавлена — ${parts.join(', ')}` : ''
}
