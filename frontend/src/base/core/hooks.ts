// Точки, куда верхние группы (editor/, panels/, shell/) подключают себя при старте: основа их не
// импортирует (docs/STRUCTURE.md, правило 9). Подключение — shell/boot/wire.ts (wire() в main.ts и в
// тестах, которым нужен верх). Без него store работает «голым»: хуки пустые, ленты не фильтруются.
import type { KeymapView } from '../types/keys'
import type { FontFamily, MarkView, NewerInfo, Photo, View } from './types'

// Что выбрано для автосборки (034): откуда фото и сколько из них уже в альбоме — для строки окна.
export interface PickResult {
  ids: string[]
  skipped: number
  from: 'selected' | 'free' | 'album'
  inAlbum: number
  filtered: boolean
  again?: boolean // 078: окно открыто как «Пересобрать» (те же фото последней сборки)
  alt?: PickResult // 080: второй путь для ссылки в строке источника (у alt своего alt нет)
  bin?: string // 083: без выделения фото взяты из показанной подборки — её имя для строки окна
  // 107: «Весь альбом заново» (from 'album'). ids — фото альбома по ленте (+ свободные при addFree); Go берёт голые развороты сам.
  freeIds?: string[] // свободные фото ленты, которые можно добавить галочкой (с фильтром ленты, без отклонённых)
  addFree?: boolean // галочка «Добавить свободные фото ленты» (выключена)
  base?: PickResult // откуда пришли: щелчок по «Свободные фото» / «Выделенные» в строке «Собрать» возвращает этот выбор
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
  // Шрифты из GetState или события `fonts`: зарегистрировать в document.fonts. fresh — пришли событием (загрузка,
  // открыт собранный альбом): новые свои начертания ещё надо проверить окном (082-14).
  fontsLoaded(fonts: FontFamily[], fresh?: boolean): void
  // State.freshFonts: новые начертания из папки альбома (запуск, открытие) — проверить окном, как после загрузки.
  freshFonts(files: string[]): void
  // Горячие клавиши из GetState (State.keymap); пока Go поля не шлёт (до G2) — не зовётся.
  keymapLoaded(view: KeymapView): void

  // --- данные верха, из которых store строит ленту ---
  // Свои отметки фото (ID -> поля, поставленные в программе).
  marks(): MarkMap
  // Порядок ленты (075): «По имени» — тот же массив, «По дате съёмки» — по ключу shot.
  order(photos: Photo[]): Photo[]
  // Фото без скрытых папок (054): первый слой ленты, до «Не в альбоме» и отметок.
  folders(photos: Photo[]): Photo[]
  // Фото показанной подборки (083): после папок, до «Не в альбоме» и отметок; без подборки — тот же массив.
  bin(photos: Photo[]): Photo[]
  // Лента после фильтра по отметкам.
  visible(photos: Photo[]): Photo[]
  // Фото не в местах и не на фоне (автосборка, фильтр «Неиспользованные»).
  free(photos: Photo[], used: ReadonlySet<string>, backdrop: ReadonlySet<string>): Photo[]
  // Что отдать автосборке / «Заполнить пустые места»; пусто — тост с причиной уже показан.
  pick(moveUsed: boolean): PickResult
  // 107: свободные фото ленты без учёта выделения (галочка «Добавить свободные» в режиме альбома); тоста нет
  pickFree(): string[]
  // 078: из фото последней сборки убрать отклонённые (флажок ✕) — в пересборку они не идут
  notRejected(ids: readonly string[]): string[]

  // --- выделение на холсте ---
  selectedRefs(): readonly string[]
  select(ids: string[]): void

  // --- бросок на ручной разворот (042) ---
  // Фото из ленты на лист-сетку (Enter, двойной щелчок): места, плитка, вопрос. true — обработано.
  dropGrid(spread: string, ids: string[]): Promise<boolean>

  // --- копии фото (098) ---
  // Ворота броска из ленты: ids фото, уже стоящих в альбоме, — вопрос «Фото уже в альбоме». Ответ — ID для Go
  // (запрос копии `<id>-new` или запись переносимого места); null — «Отмена», правки не будет. Нечего спрашивать — ids как есть.
  gate(ids: string[]): Promise<string[] | null>

  // --- открытие .salbum извне (112) ---
  // State.pending: файл в очереди Go пришёл раньше подписки на событие `open:external`. name — имя без пути.
  external(name: string): void
  // State.newer: файл новее программы — окно «Альбом из новой версии» (в ответах открытия приходит вместе с cancelled).
  newer(info: NewerInfo): void
}

// Пустые хуки: основа без верха.
export const noHooks: StoreHooks = {
  applied: () => {},
  went: () => {},
  reloaded: () => {},
  spreadsCleared: () => {},
  fontsLoaded: () => {},
  freshFonts: () => {},
  keymapLoaded: () => {},
  marks: () => ({}),
  order: (photos) => photos,
  folders: (photos) => photos,
  bin: (photos) => photos,
  visible: (photos) => photos,
  free: (photos, used, backdrop) => photos.filter((p) => !used.has(p.id) && !backdrop.has(p.id)),
  pick: () => ({ ids: [], skipped: 0, from: 'free', inAlbum: 0, filtered: false }),
  pickFree: () => [],
  notRejected: (ids) => [...ids],
  selectedRefs: () => [],
  select: () => {},
  dropGrid: async () => false,
  gate: async (ids) => ids,
  external: () => {},
  newer: () => {},
}
