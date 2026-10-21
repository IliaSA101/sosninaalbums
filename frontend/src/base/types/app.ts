import type { KeymapView } from './keys'
import type { Bin, Photo, ScanProgress } from './marks'
import type { CoverView, GuideView } from './print'
import type { Rect, SpreadView } from './sheet'

export interface Settings {
  pageW: number
  pageH: number
  margin: number
  gap: number
  layflat: boolean
  foldAny: boolean // при layflat: true — через корешок любые фото, false — только широкие (064)
  dpi: number
  bg: string // #rrggbb, цвет альбома; "" — белый
  safe: number // безопасная зона, мм от края страницы; 0 — без зоны
  bleed?: number // обрез, мм (066): полоса по внешним краям разворота; нет/0 — без полосы
  sheet?: boolean // альбом из отдельных листов (031): pageW — ширина листа, шва нет
}

export interface View {
  spreads: SpreadView[]
  // Цепочка видов (075): seq растёт у App на каждый отданный вид, 0 — вне цепочки. base > 0 — дельта к виду с seq == base:
  // spreads — только изменённые, order — ID всех разворотов по порядку (viewdelta.ts).
  seq?: number
  base?: number
  order?: string[]
  settings: Settings
  folders: string[]
  loose: LooseView[] // 097: добавления отдельных фото по порядку; файл не лежит ни в одной папке folders (инвариант Go)
  path: string
  dirty: boolean
  canUndo: boolean
  canRedo: boolean
  rev: number // растёт при каждой реальной правке/undo/redo
  maxSpreadPhotos: number // больше фото в развороте не склеиваем (спека 003)
  safe?: Rect[] | null // безопасная зона, мм, координаты разворота (спека 005); считает Go
  bleed?: Rect[] | null // полосы обреза, мм разворота: верх, низ, лево, право (066); null — обреза нет
  spreadMarkup?: GuideView | null // разметка типографии разворотов (066); null — файла нет
  minDPI: number // порог «!»: min(200, dpi настроек)
  lowDPI: number // фото на разворотах с 0 < dpi < minDPI
  emptyPlaces: number // пустых мест во всём альбоме (спека 007)
  textMaxW: number // предел ширины блока «по тексту», мм (спека 008)
  backdropUsed: string[] | null // фото, стоящие на фоне: автосборка и «Неиспользованные» их не берут
  libPhotos?: string[] // ID записей со своим из библиотеки (048): их нет в «Найти пропавшие», пропавшие ищутся загрузкой
  marksRev: number // растёт при смене своих отметок (спека 004): тогда UI перечитывает Api.Marks()
  unfilled?: string[] // пустые переменные из текстов, по алфавиту
  unnamed?: number // фото с полями имени в подписи и без имени
  cover?: CoverView | null // обложка (032); нет — её нет в альбоме
  pageLimit?: PageLimitView // предел разворотов типографии (085); нет поля — типография неизвестна
  albumFonts?: FontUse[] // 082: свои шрифты, стоящие в альбоме; Go начнёт слать с G4 — до этого undefined
  bins: Bin[] // 083: подборки фото в порядке создания
  again?: boolean // 078: «Пересобрать…» ещё можно (Go считает по версиям проекта; отметки её не закрывают)
}

// Добавление отдельных фото (097): at — Unix-секунды, подпись — местным временем в окне «Папки».
export interface LooseView {
  id: string
  at: number
  files: LooseFileView[]
}

// Файл добавления: photo — ID фото ленты (его же ждёт набор «в альбоме»); файла нет в ленте — нет на диске или ещё читается.
export interface LooseFileView {
  path: string
  photo: string
}

// Итог добавления отдельных фото (097): ответ ChooseFiles/AddFilePaths (с view) и событие `loose:added` (без него).
// batch '' — ничего не добавлено; inFolder — уже в папке альбома (folder — эта папка); dup — уже добавлены отдельно.
export interface LooseAdded {
  batch: string
  added: number
  inFolder: number
  folder: string
  dup: number
  skipped: number // не фото, не файл
  bad: number // не локальный путь
  over: number // сверх предела 2 000
}

// Предел разворотов типографии (085): тексты готовит Go, UI их только показывает.
export interface PageLimitView {
  who: string // «Нетпринт «Премиум»» | «Фотосфера»
  range: string // «от 10 до 60 разворотов» | «до 57 разворотов»
  count: number
  min: number // 0 — не задано
  max: number // 0 — не задано
  warn: string // '' — в пределах; иначе текст предупреждения
}

// Начертание шрифта (State.fonts[].faces[]): file — имя в /media/font/<file>.
export interface FontFaceInfo {
  file: string
  bold: boolean
  italic: boolean
}

// Семейство шрифтов из State.fonts. id — для TextStyle.font, css — для font-family.
export interface FontFamily {
  id: string
  name: string
  css: string
  group: string // my (свои, 082) | script | kids | serif | sans | display
  faces: FontFaceInfo[]
  hidden?: boolean // 082: убран × из списка; текущий выбор остаётся видимым
  noCyr?: boolean // 082: в шрифте нет русских букв — образец латиницей
}

// Свои гарнитуры, стоящие в альбоме (View.albumFonts, 082): нет в State.fonts — «нет на этом компьютере».
export interface FontUse {
  id: string
  name: string // '' — имя неизвестно (взято из шаблона)
}

// Итог ImportFonts / ImportFontPaths (app.FontImport, 082): faces — ключи новых начертаний для checkFaces.
export interface FontAdded {
  id: string
  name: string
  faces: number
  noCyr: boolean
  files: string[] // файлы этой гарнитуры, записанные этой загрузкой; пусто — лишь вернулась из скрытых
}
export interface FontImport {
  added: FontAdded[]
  faces: string[] // '<16 hex>.ttf|otf'
  skipped: { name: string; why: string }[]
}

// Настройки программы (State.prefs, Api.SetLevel / SetCustom / SetShowStart): level — simple | standard | full | custom;
// first — первый запуск 0.5.0; custom — набор «Своего» (разница с основой), null/нет — не настраивали.
export interface Prefs {
  level: string
  showStart: boolean
  first: boolean
  custom?: { base: string; on: string[]; off: string[] } | null
}

// Недавний альбом (State.recent): exists и modified из os.Stat на стороне Go.
export interface RecentProject {
  path: string
  name: string
  exists: boolean
  modified: number
}

// Резервная копия альбома (Api.Backups): time — unix-мс, size — байты.
export interface BackupInfo {
  name: string
  time: number
  size: number
  spreads: number
  photos: number
}

// Ответ Api.GetState / OpenProject / NewProject: всё, чем живёт окно.
export interface State {
  view: View
  photos: Photo[]
  notice?: string // ошибка открытия при старте
  info?: string // разовый тост вида info (например «Добавлены шрифты из папки альбома»)
  freshFonts?: string[] // файлы новых начертаний из папки альбома: окно их проверяет (082-14)
  version?: string
  scan?: ScanProgress
  fonts?: FontFamily[]
  missing?: string[]
  prefs?: Prefs
  recent?: RecentProject[]
  keymap?: KeymapView | null // горячие клавиши (074); Go начнёт слать с G2 — до этого undefined
  cancelled?: boolean // только OpenProject: диалог закрыт без выбора, состояние прежнее
}
