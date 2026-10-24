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
  crop?: number // 104: подрезка, % по стороне: 0 | 5 | 10 | 15; нет — 0
  sheet?: boolean // альбом из отдельных листов (031): pageW — ширина листа, шва нет
}

// Фон альбома для строки настроек (098, model.AlbumBgView): узор или фото, один на альбом; нет в View — фона нет.
export interface AlbumBg {
  name: string // имя плитки
  lib?: string // ключ своего фото из библиотеки; нет — узор
  fade: number // прозрачность 0…1
  pages: boolean // на страницах листов, а не только на развороте
  preset?: string // плитка каталога узора: образец и отметка в окне «Фоны»
  photo?: string // ID записи фото фона (образец /media/p/<id>, когда ключа библиотеки уже нет); нет — узор
}

// Вид вхождения (106, model.Use*): по нему интерфейс выбирает переход, подпись он не разбирает.
export type UseKind = 'place' | 'backdrop' | 'sticker' | 'border' | 'albumBg'

// Где стоит фото (084, Api.PhotoUses): sheet — индекс листа, -1 — «Фон альбома» (098); label — подпись для людей.
export interface PhotoUse {
  sheet: number
  label: string
  kind: UseKind
  spread: string // ID листа (обложка — COVER_ID); '' — фон альбома
  page: string // «Разворот 3» | «Лист 2» | «Обложка» | «Фон альбома» — текст ссылки
  what: string // «место» | «фон» | «стикер» | «рамочка»; '' — фон альбома
  pick?: string // что выделить: запись места, ID стикера, область фона; у фона альбома нет
}

// Ответ Api.WhereUsed (098, до 098 G2 — заглушка по design §2.4): где стоит файл и его копии.
export interface UsedAt {
  id: string // ID файла (BaseID переданного)
  uses: PhotoUse[] // по семье; не null
  places: number // сколько мест у всех записей семьи
  move: string // запись единственного места; '' — мест 0 или больше одного
}

export interface View {
  spreads: SpreadView[]
  albumBg?: AlbumBg // фон альбома (098); нет — его нет
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
  // Первые шаги (113, Api.SetTour): главный тур стартует сам / предложить продолжение (один раз).
  // Необязательные: Go отдаёт всегда, а тесты и старые ответы их не знают.
  tourDue?: boolean
  tourMore?: boolean
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

// Связь .salbum с программой (112, app.AssocView): can — выпуск на Windows (вопрос и пункт меню есть),
// on — связь ведёт в эту программу, ask — показать вопрос.
export interface AssocView {
  can: boolean
  on: boolean
  ask: boolean
}

// Ответ Api.GetState / OpenProject / NewProject: всё, чем живёт окно.
// 112: окно «Альбом из новой версии» (Go: app.NewerInfo). Тексты и кнопки собирает shell/ask/newerask.ts.
export interface NewerInfo {
  name: string // «Свадьба.salbum»; у черновика пусто
  draft: boolean // черновик при старте
  need: string // «0.26»; "" — версия в файле неизвестна
  mine: string // «0.25» — эта программа
  have: string // «0.26» — найденная на компьютере новая версия; "" — нет (или не Windows)
}

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
  unread?: Record<string, string> // 117: отдельные файлы, найденные, но не прочитанные: ID → причина; нет ключа — читается
  prefs?: Prefs
  recent?: RecentProject[]
  keymap?: KeymapView | null // горячие клавиши (074); Go начнёт слать с G2 — до этого undefined
  cancelled?: boolean // только OpenProject: диалог закрыт без выбора, состояние прежнее
  newer?: NewerInfo // 112: файл новее программы — окно «Альбом из новой версии» (вместе с cancelled в ответах открытия)
  pending?: string // 112: имя .salbum в очереди внешнего открытия (GetState не чистит, чистит OpenExternal)
  assoc?: AssocView // 112: пересчитывается при каждом GetState
  fromFile?: boolean // 112: запуск двойным щелчком по альбому и он открыт — стартовое окно не нужно
  stand?: boolean // стенд QA (сборка dev + SA_STAND): окна ОС и браузер не открываются, ссылки молчат
}
