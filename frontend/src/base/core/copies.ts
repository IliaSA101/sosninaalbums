// ID копии фото (098): «<ID файла>-<n>», n = 2…99. Зеркало Go model.BaseID / CopyNo / CopyReq / IsCopyReq;
// паритет — testdata/parity/copyid.json (пишет Go TestCopyIDParity).

export const MAX_COPY = 99 // номера копий 2…99 (98 копий одного файла)
const REQ_TAIL = '-new' // запрос копии от интерфейса: «<ID файла>-new»

// Номер копии: 0 — не копия (суффикса нет, он не число без знака и нулей впереди, < 2 или > 99, ID файла пуст).
export function copyNo(id: string): number {
  const i = id.lastIndexOf('-')
  if (i <= 0) return 0
  const tail = id.slice(i + 1)
  if (!/^[1-9]\d?$/.test(tail)) return 0
  const n = Number(tail)
  return n >= 2 && n <= MAX_COPY ? n : 0
}

// ID файла: «ab…cd-3» → «ab…cd»; не копия — id как есть.
export function baseOf(id: string): string {
  return copyNo(id) ? id.slice(0, id.lastIndexOf('-')) : id
}

// Запрос копии для AddPhotos / NewSpread / PlacePhotos / DropPhotos: «<ID файла>-new».
export function copyReq(base: string): string {
  return base + REQ_TAIL
}

// Это запрос копии (а не ID записи) с непустым ID файла.
export function isCopyReq(id: string): boolean {
  return id.length > REQ_TAIL.length && id.endsWith(REQ_TAIL)
}
