// Число фото по папкам проекта для окна «Папки» (спека 024) и фильтр ленты по папкам (054). Go не нужен: у каждого фото есть путь.

// Ключ пути: без различия регистра, разделитель «/», без завершающего «/».
export function folderKey(p: string): string {
  return p.toLowerCase().replaceAll('\\', '/').replace(/\/+$/, '')
}

// Ключ папки → её первый номер в списке (повтор папки не считается).
function folderIndex(folders: readonly string[]): Map<string, number> {
  const idx = new Map<string, number>()
  folders.forEach((f, i) => {
    const k = folderKey(f)
    if (!idx.has(k)) idx.set(k, i)
  })
  return idx
}

// Номер папки фото или −1. Сканер обходит папки по порядку списка и отбрасывает дубли, поэтому фото
// вложенной или повторной папки достаётся первой из подходящих.
function folderOf(idx: ReadonlyMap<string, number>, path: string): number {
  let p = folderKey(path)
  let best = -1
  for (let i = p.lastIndexOf('/'); i >= 0; i = p.lastIndexOf('/')) {
    p = p.slice(0, i)
    const at = idx.get(p)
    if (at !== undefined && (best < 0 || at < best)) best = at
  }
  return best
}

// Сколько фото у каждой папки; результат параллелен folders.
export function folderCounts(folders: readonly string[], photos: readonly { path: string }[]): number[] {
  const idx = folderIndex(folders)
  const out = folders.map(() => 0)
  for (const ph of photos) {
    const at = folderOf(idx, ph.path)
    if (at >= 0) out[at]++
  }
  return out
}

// Строка окна «Папки»: «В ленте 162 из 162 фото» — и при нескрытых папках, чтобы высота окна не менялась.
export const inFeedText = (shown: number, total: number): string => `В ленте ${shown} из ${total} фото`

// Лента без скрытых папок (054). hidden — ключи folderKey(); пусто — тот же массив (без копии и без новой зависимости).
// Фото вне всех папок (быть не должно) остаётся.
export function filterFolders<P extends { path: string }>(folders: readonly string[], photos: P[], hidden: ReadonlySet<string>): P[] {
  if (hidden.size === 0) return photos
  const idx = folderIndex(folders)
  const off = folders.map((f) => hidden.has(folderKey(f)))
  return photos.filter((ph) => {
    const at = folderOf(idx, ph.path)
    return at < 0 || !off[at]
  })
}

// filterFolders с запоминанием последнего ответа: правка альбома даёт новый view.folders (тот же состав),
// но пока фото, папки и скрытые те же — возвращается прежний массив, и ленту не пересчитывают (054).
export function cachedFilterFolders() {
  let photos: unknown = null
  let key = ''
  let out: unknown = null
  return <P extends { path: string }>(folders: readonly string[], list: P[], hidden: ReadonlySet<string>): P[] => {
    if (hidden.size === 0) return list
    const k = folders.map(folderKey).join('\n') + '\n\n' + [...hidden].sort().join('\n')
    if (list === photos && k === key) return out as P[]
    photos = list
    key = k
    out = filterFolders(folders, list, hidden)
    return out as P[]
  }
}
