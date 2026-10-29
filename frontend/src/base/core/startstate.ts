import type { Photo, ScanProgress } from './types'

// Какие поля уже обновило событие Go с момента, как мы начали ждать снимок (GetState, Open/New).
export type Seen = { photos: boolean; scan: boolean; missing: boolean }

export const noEvents = (): Seen => ({ photos: false, scan: false, missing: false })

export type Live = { photos: Photo[]; scan: ScanProgress; missing: string[] }

// Событие, пришедшее раньше ответа, свежее снимка: Go меняет photos/scan/missing
// только с последующим событием, поэтому снимок мог быть собран до него и затёр бы новые данные.
// Поля, которых нет в снимке (replaceState их не отдаёт), остаются как есть.
export function mergeSnapshot(cur: Live, snap: Partial<Live> | undefined, seen: Seen): Live {
  const s = snap ?? {}
  return {
    photos: seen.photos ? cur.photos : (s.photos ?? cur.photos),
    scan: seen.scan ? cur.scan : (s.scan ?? cur.scan),
    missing: seen.missing ? cur.missing : (s.missing ?? cur.missing),
  }
}
