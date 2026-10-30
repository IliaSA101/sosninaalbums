// Строка «Ролик» карточки F1 (спека 140, design §4): состояния и тексты. Чистая логика, без Svelte и сети.
import { sizeLabel } from '../dialogs/backups'

// Ответ Go (app.GuideVideo): ролик каталога и скачан ли он.
export interface VideoItem {
  id: string
  version: number
  seconds: number
  size: number
  ready: boolean
}

// Ход скачивания (событие guide:video): байты. В models.ts типа нет.
export interface GuideVideoProgress {
  id: string
  done: number
  total: number
}

// watch — «▶ Смотреть» (скачан или будет загружен в окне ролика, 0.30-140-31); busy — идёт скачивание вне окна
// («все ролики», «Скачать заново», справочник закрывали и открыли); broken — окно не смогло сыграть скачанный файл.
export type RowState = 'watch' | 'busy' | 'broken'

// Известные программе ролики: встроенный список ∪ кэш каталога (id статьи → секунды).
export const hasVideo = (id: string, builtin: Readonly<Record<string, unknown>>, items: ReadonlyMap<string, VideoItem>): boolean =>
  id in builtin || items.has(id)

// loading — ролик, который сейчас грузится в окне ролика: ход виден там, строка остаётся «▶ Смотреть» (под окном).
export function rowState(
  id: string,
  item: VideoItem | undefined,
  busy: readonly string[],
  failed: ReadonlySet<string>,
  loading: string | null = null,
): RowState {
  if (failed.has(id) && item?.ready) return 'broken'
  if (busy.includes(id) && loading !== id) return 'busy'
  return 'watch'
}

// Загрузка без согласия на сеть сначала задаёт вопрос «Обратиться к GitHub?».
export const getAction = (allowed: boolean): 'download' | 'ask' => (allowed ? 'download' : 'ask')

// «48 с», «1 мин 5 с».
export function secondsLabel(s: number): string {
  const n = Math.max(0, Math.round(s))
  if (n < 60) return `${n} с`
  const rest = n % 60
  return rest ? `${Math.floor(n / 60)} мин ${rest} с` : `${Math.floor(n / 60)} мин`
}

// МБ в десятых, как в окне «Наборы» (1024×1024): «0,5».
const mb = (n: number): string => (n / 1048576).toFixed(1).replace('.', ',')

// «Скачиваем… 0,3 из 0,5 МБ»; размер неизвестен или ход чужого ролика — без чисел.
export function busyLine(p: GuideVideoProgress | null, id: string): string {
  if (!p || p.id !== id || p.total <= 0) return 'Скачиваем…'
  return `Скачиваем… ${mb(Math.min(p.done, p.total))} из ${mb(p.total)} МБ`
}

// Доля полосы 0…100; null — неопределённая.
export function percent(p: GuideVideoProgress | null, id: string): number | null {
  if (!p || p.id !== id || p.total <= 0) return null
  return Math.max(0, Math.min(100, Math.round((p.done / p.total) * 100)))
}

// «все ролики (7, 3 МБ)»: ещё не скачанные; ссылка нужна, когда их больше одного. Размер неизвестен хоть у одного — без него.
export function allLink(items: readonly VideoItem[]): { ids: string[]; label: string } | null {
  const todo = items.filter((i) => !i.ready)
  if (todo.length < 2) return null
  const known = todo.every((i) => i.size > 0)
  const bytes = todo.reduce((n, i) => n + i.size, 0)
  return { ids: todo.map((i) => i.id), label: `все ролики (${todo.length}${known ? `, ${sizeLabel(bytes)}` : ''})` }
}

// Тип файла по ответу: только то, что отдаёт Go; иное окно не играет (строка «не открылся»).
export function blobType(contentType: string | null): 'video/webm' | 'video/mp4' | '' {
  const t = (contentType ?? '').split(';')[0].trim().toLowerCase()
  return t === 'video/webm' || t === 'video/mp4' ? t : ''
}

export const BROKEN_TEXT = 'Ролик не открылся в окне программы'
