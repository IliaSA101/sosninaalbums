// Точки, куда верхние группы (editor/, panels/, shell/) подключают себя при старте: основа их не
// импортирует (docs/STRUCTURE.md, правило 9). Подключение — shell/boot/wire.ts (wire() в main.ts и в
// тестах, которым нужен верх). Без него store работает «голым»: хуки пустые, ленты не фильтруются.
import type { KeymapView } from '../types/keys'
import type { FontFamily, MarkView, Photo, View } from './types'

// Что выбрано для автосборки (034): откуда фото и сколько из них уже в альбоме — для строки окна.
export interface PickResult {
  ids: string[]
  skipped: number
  from: 'selected' | 'free'
  inAlbum: number
  filtered: boolean
  again?: boolean // 078: окно открыто как «Пересобрать» (те же фото последней сборки)
}

export type MarkMap = Readonly<Record<string, MarkView>>

export interface StoreHooks {
  // --- события жизни: верх сбрасывает и обрезает свои состояния ---
  // После каждой смены альбома (apply): отметки по marksRev, выделения и окна по новым разворотам.
  applied(view: View): void
  // Переход на другой разворот: снять выделение места, отменить авто, закрыть галерею.
  went(): void
  // Состояние заменено целиком (запуск, открытие/создание альбома): отметки прошлого альбома не показывать.
  reloaded(): void
  // Выделение разворотов в ленте больше не к чему (группа удалена, альбом заменён).
  spreadsCleared(): void
  // Шрифты из GetState: зарегистрировать в document.fonts.
  fontsLoaded(fonts: FontFamily[]): void
  // Горячие клавиши из GetState (State.keymap); пока Go поля не шлёт (до G2) — не зовётся.
  keymapLoaded(view: KeymapView): void

  // --- данные верха, из которых store строит ленту ---
  // Свои отметки фото (ID -> поля, поставленные в программе).
  marks(): MarkMap
  // Порядок ленты (075): «По имени» — тот же массив, «По дате съёмки» — по ключу shot.
  order(photos: Photo[]): Photo[]
  // Фото без скрытых папок (054): первый слой ленты, до «Не в альбоме» и отметок.
  folders(photos: Photo[]): Photo[]
  // Лента после фильтра по отметкам.
  visible(photos: Photo[]): Photo[]
  // Фото не в местах и не на фоне (автосборка, фильтр «Неиспользованные»).
  free(photos: Photo[], used: ReadonlySet<string>, backdrop: ReadonlySet<string>): Photo[]
  // Что отдать автосборке / «Заполнить пустые места»; пусто — тост с причиной уже показан.
  pick(moveUsed: boolean): PickResult
  // 078: из фото последней сборки убрать отклонённые (флажок ✕) — в пересборку они не идут
  notRejected(ids: readonly string[]): string[]

  // --- выделение на холсте ---
  selectedRefs(): readonly string[]
  select(ids: string[]): void

  // --- бросок на ручной разворот (042) ---
  // Фото из ленты на лист-сетку (Enter, двойной щелчок): места, плитка, вопрос. true — обработано.
  dropGrid(spread: string, ids: string[]): Promise<boolean>
}

// Пустые хуки: основа без верха.
export const noHooks: StoreHooks = {
  applied: () => {},
  went: () => {},
  reloaded: () => {},
  spreadsCleared: () => {},
  fontsLoaded: () => {},
  keymapLoaded: () => {},
  marks: () => ({}),
  order: (photos) => photos,
  folders: (photos) => photos,
  visible: (photos) => photos,
  free: (photos, used, backdrop) => photos.filter((p) => !used.has(p.id) && !backdrop.has(p.id)),
  pick: () => ({ ids: [], skipped: 0, from: 'free', inAlbum: 0, filtered: false }),
  notRejected: (ids) => [...ids],
  selectedRefs: () => [],
  select: () => {},
  dropGrid: async () => false,
}
