// Фото, которые можно раскладывать автоматически (автосборка, «Заполнить пустые места», фильтр
// «Неиспользованные»): не в местах альбома и не на фоне. Фото фона в used не входит (спека 009,
// решение владельца), но раскладка не должна снова ставить его в места.
import { withSkipped } from '../../base/words/errtext'
import type { PickResult } from '../../base/core/hooks'
import type { Photo } from '../../base/core/types'
import { markOf, type OwnMarks } from '../marks/marks'

export { withSkipped }

export function freePhotos(photos: readonly Photo[], used: ReadonlySet<string>, backdrop: ReadonlySet<string>): Photo[] {
  return photos.filter((p) => !used.has(p.id) && !backdrop.has(p.id))
}

// Что отдать автосборке и «Заполнить пустые места» (спека 004, п. 17). С выделением — как выбрано:
// автосборка переносит уже стоящие фото в новые развороты (moveUsed), заполнение мест их пропускает
// (Go не ставит фото из альбома). Без выделения — видимые свободные фото, кроме отклонённых;
// skipped — сколько отклонённых пропущено.
export function autoPick(
  visible: readonly Photo[],
  selected: readonly string[],
  used: ReadonlySet<string>,
  backdrop: ReadonlySet<string>,
  marks: OwnMarks,
  moveUsed: boolean,
): { ids: string[]; skipped: number } {
  if (selected.length > 0) return { ids: moveUsed ? [...selected] : selected.filter((id) => !used.has(id)), skipped: 0 }
  const free = freePhotos(visible, used, backdrop)
  const kept = free.filter((p) => markOf(p, marks[p.id]).flag !== -1)
  return { ids: kept.map((p) => p.id), skipped: free.length - kept.length }
}

// 078: пересборка берёт фото последней сборки без отклонённых после неё (то же правило, что у autoPick).
export function notRejected(ids: readonly string[], byId: ReadonlyMap<string, Photo>, marks: OwnMarks): string[] {
  return ids.filter((id) => {
    const p = byId.get(id)
    return !p || markOf(p, marks[id]).flag !== -1
  })
}

// Фильтр «Не в альбоме» пуст потому, что все фото уже разложены: лента говорит об этом и даёт «Показать все» (035 U11).
export function allInAlbum(filter: 'all' | 'unused', freeCount: number, total: number): boolean {
  return filter === 'unused' && freeCount === 0 && total > 0
}

// Текст пустой ленты «Не в альбоме»: при скрытых папках речь о показанных (054).
export function allInAlbumText(foldersFiltered: boolean): string {
  return foldersFiltered ? 'Все фото показанных папок уже в альбоме' : 'Все фото уже в альбоме'
}

// Тост, когда автосборке нечего брать.
export function emptyReason(selected: boolean, skipped: number, filtered: boolean): string {
  if (skipped > 0) return withSkipped('', skipped)
  if (selected) return 'Выделенные фото уже в альбоме'
  return filtered ? 'Под фильтр не подходит ни одно свободное фото' : 'Все фото уже в альбоме'
}

// Что нужно от store, чтобы выбрать фото (store передаёт себя).
export interface PickHost {
  readonly visible: Photo[]
  readonly selectedIds: string[]
  readonly used: ReadonlySet<string>
  readonly backdropUsed: ReadonlySet<string>
  readonly marks: OwnMarks
  notify(text: string): void
}

// autoPick для store: пусто — тост с причиной (ids пуст, вызывающий выходит).
export function pickFor(h: PickHost, moveUsed: boolean, filtered: boolean): PickResult {
  const r = autoPick(h.visible, h.selectedIds, h.used, h.backdropUsed, h.marks, moveUsed)
  if (r.ids.length === 0) h.notify(emptyReason(h.selectedIds.length > 0, r.skipped, filtered))
  const sel = h.selectedIds.length > 0
  return { ...r, from: sel ? 'selected' : 'free', inAlbum: sel ? r.ids.filter((id) => h.used.has(id)).length : 0, filtered }
}
