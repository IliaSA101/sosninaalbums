export interface Photo {
  id: string
  path: string
  name: string
  w: number
  h: number
  // Цветовой профиль (спека 005): пусто — sRGB или профиля нет.
  color?: 'srgb' | 'adobe' | 'other'
  profile?: string
  // Отметки из файла (спека 004): XMP внутри JPEG и/или сайдкар; только чтение.
  xmp?: XmpMarks
}

// Цвет метки: '' — нет. У отметки из файла бывает ещё 'other' (незнакомый текст, он в labelText).
export type Label = '' | 'red' | 'yellow' | 'green' | 'blue' | 'purple'
export type Flag = -1 | 0 | 1
export interface XmpMarks {
  rating?: number
  label?: Label | 'other'
  labelText?: string // только у 'other': выводить текстом, не HTML
  flag?: -1 | 1
}
// Своя отметка (Api.Marks): нет поля — своя не задана, действует отметка из файла. 0 и '' — заданы.
export interface MarkView {
  rating?: number
  label?: Label
  flag?: Flag
}
// Что поменять у выделенных (Api.SetMarks).
export interface MarkPatch {
  rating?: number
  step?: number // ±1 от действующей оценки каждого фото
  label?: Label
  toggle?: boolean // с label: у всех уже этот цвет — снять
  flag?: Flag
}
// Действующая отметка (library/marks.ts: markOf); lr — отметка из файла, если своя с ней расходится.
export interface Mark {
  rating: number
  label: Label | 'other'
  labelText?: string
  flag: Flag
  lr?: Mark
}

// Ход чтения папок (событие scan и State.scan).
export interface ScanProgress {
  active: boolean
  walking: boolean // ещё обходим папки: found растёт
  found: number
  read: number
}

export interface Rect {
  x: number
  y: number
  w: number
  h: number
}

// Место как неповёрнутый прямоугольник и угол вокруг его центра (спека 010); rot нет — 0.
export interface RotRect extends Rect {
  rot?: number
}

// Коробка шаблона (model.Box): доли области, rot — градусы по часовой вокруг центра; только при угле.
// У повёрнутой коробки в области лежит центр: x может быть < 0, x + w > 1.
export type Box = RotRect

// Положение раскладки страницы в её ряду (только при постраничной раскладке P[…]).
export interface PageView {
  photos: number
  variant: number // -1 — блока нет в ряду
  variants: number // ⊞ страницы — при ≥ 2 (см. pageSwitchable)
  rearranged: boolean
}

// Кадр фото в ячейке (спека 006). s — масштаб 1..4; x, y — доля свободного хода −1..1.
export interface Frame {
  s: number
  x: number
  y: number
  r?: number // поворот фото в рамке, ° по часовой (R = 90·четверти + наклон); нет — 0
}

// Кадр, посчитанный в Go: фото целиком (мм разворота; ячейка его обрезает) и эффективный dpi.
export interface FrameView extends Frame {
  img: Rect // фото в своих осях (мм разворота), рамка неповёрнутая
  r: number // полный угол фото; для рисования, на 90 не делить
  turn: number // четверти 0…3 (от Go)
  tilt: number // наклон −44,9…44,9 (от Go)
  dpi: number // 0 — размер фото неизвестен
  flipH: boolean // зеркало ↔ внутри img (геометрия не меняется)
  flipV: boolean // зеркало ↕
  cellRot: number // угол ячейки (= места, вокруг его центра); в «Авто» 0
  z: number // слой в общем стеке разворота (от Go): z-index = 10 + z
}

// Стиль текста. font — id семейства из State.fonts.
export interface TextStyle {
  font: string
  size: number // pt
  color: string // #rrggbb
  align: 'left' | 'center' | 'right'
  bold?: boolean
  italic?: boolean
}

// Строка блока: x — левый край, y — базовая линия, мм разворота.
// Строка текста: x — левый край, y — базовая линия, w — ширина; всё в мм разворота (text.Line в Go).
export interface TextLine {
  text: string
  x: number
  y: number
  w: number
}

// Блок текста (рамка в мм разворота, строки уже перенесены в Go).
export interface TextView {
  id: string
  x: number
  y: number
  w: number
  h: number
  rot: number // градусы по часовой вокруг (x, y), (−180, 180]; у подписи — угол её места
  auto: boolean // ширина по тексту (считает Go); у подписей false
  text: string
  style: TextStyle
  lines: TextLine[]
  z: number // слой в общем стеке разворота; у подписи — сразу над её фото
}

// Подпись к фото: id пуст, рамка — место подписи.
export interface CaptionView extends TextView {
  photo: string
  over: boolean // на фото, а не под ним
  overflow: number // мм, которых не хватает; 0 — помещается
}

// Место в режиме «Сетка» (спека 007), мм разворота. Порядок в SpreadView.places = порядок слоёв (z-index).
export interface PlaceView extends RotRect {
  id: string
  photo: string // "" — пустое место
  fit: '' | 'contain' // "" — с обрезкой (cover + кадр), contain — вписано целиком
  spine: boolean // пересекает корешок при выключенном layflat
  rot: number // градусы по часовой вокруг центра места, (−180, 180]
  z: number // слой в общем стеке разворота (от Go)
}

// Место для SetPlaces: только геометрия (мм разворота).
export interface PlaceBox extends Rect {
  id: string
}

// Ручка рамки места: стороны света.
export type Handle = 'n' | 's' | 'e' | 'w' | 'ne' | 'nw' | 'se' | 'sw'

// Область фото фона: весь разворот или одна страница.
export type BackdropScope = 'spread' | 'left' | 'right'

// Фото на фоне (спека 009, design-backdrop §4), всё в мм разворота. Геометрию считает Go.
export interface BackdropView {
  scope: BackdropScope
  photo: string
  area: Rect // область: фото видно только внутри
  box: Rect & { rot: number } // рамка до поворота; rot — градусы по часовой вокруг (x, y)
  img: Rect // фото целиком в осях рамки (от её левого верхнего угла до поворота)
  frame: Frame
  flipH: boolean
  flipV: boolean
  dpi: number // 0 — размер фото неизвестен
  fade: number // прозрачность фото 0…1 (0 — непрозрачное), шаг 0,01
}

export interface SpreadView {
  id: string
  name: string // название в программе (лента, плитки); "" — без названия, в экспорт не идёт
  photos: string[]
  rects: Rect[]
  frames: FrameView[] // по одному на фото, в том же порядке (Go шлёт [], не null)
  texts: TextView[] // Go шлёт [], не null
  captions: CaptionView[] // Go шлёт [], не null
  bg: [string, string] // действующий цвет левой и правой страницы
  bgSet: [string, string] // как задано у разворота; "" — наследуется
  layout: string // ключ действующей раскладки; "" — разворот пуст
  variant: number // позиция в ряду листания; -1 — раскладки нет в рядах
  variants: number // длина этого ряда
  rearranged: boolean // ряд листания — «С перестановкой порядка»
  pages: PageView[] | null // null — раскладка не постраничная; иначе [левая, правая]
  // Режим «Сетка» (спека 007): photos/rects/frames — из мест, layout "", variant -1, pages null.
  grid: boolean
  places: PlaceView[] | null // null — «Авто»
  backdrops: BackdropView[] | null // 0–2 шт.; Go может прислать null
}

// Варианты для галереи раскладок (Api.LayoutChoices). Rects — весь разворот в порядке photos.
export interface Choice {
  key: string
  rects: Rect[]
}

export interface ChoiceRows {
  inOrder: Choice[]
  rearranged: Choice[]
  block: '' | 'same' | 'many' // почему ряд с перестановкой пуст
}

export interface Choices {
  spread: string
  rev: number // для какого rev посчитано
  whole: ChoiceRows
  left: ChoiceRows | null // null — раскладка не постраничная или на странице < 2 фото
  right: ChoiceRows | null
}

export type LayoutScope = 'spread' | 'left' | 'right'

export interface Settings {
  pageW: number
  pageH: number
  margin: number
  gap: number
  layflat: boolean
  dpi: number
  bg: string // #rrggbb, цвет альбома; "" — белый
}

export interface View {
  spreads: SpreadView[]
  settings: Settings
  folders: string[]
  path: string
  dirty: boolean
  canUndo: boolean
  canRedo: boolean
  rev: number // растёт при каждой реальной правке/undo/redo
  maxSpreadPhotos: number // больше фото в развороте не склеиваем (спека 003)
  safe?: Rect[] | null // безопасная зона, мм, координаты разворота (спека 005); считает Go
  minDPI: number // порог «!»: min(200, dpi настроек)
  lowDPI: number // фото на разворотах с 0 < dpi < minDPI
  emptyPlaces: number // пустых мест во всём альбоме (спека 007)
  textMaxW: number // предел ширины блока «по тексту», мм (спека 008)
  backdropUsed: string[] | null // фото, стоящие на фоне: автосборка и «Неиспользованные» их не берут
  marksRev: number // растёт при смене своих отметок (спека 004): тогда UI перечитывает Api.Marks()
}

// Результат правки сетки: left — фото, не вставшие или ушедшие в ленту.
export interface GridResult {
  view: View
  left: number
}

// Куда ставить сетку: разворот или его страница.
export interface GridTarget {
  spread: string
  scope: LayoutScope
}

// Сухой прогон копирования (PlanCopyGrid): что случится, если копировать.
export interface GridPlan {
  frozen: number // «Авто»-развороты станут сеткой
  left: number // фото вернутся в ленту
  texts: number // блоков текста заменится
}

// Карточка галереи сеток. boxes — доли области scope шаблона, 0..1, в порядке мест.
export interface GridChoice {
  id: string // шаблон: его ID; встроенная: ключ раскладки
  name: string // "" у встроенных
  scope: 'page' | 'spread'
  boxes: Box[]
  drop: number // сколько фото уйдёт в ленту при применении: значок «−N»
}

export interface GridChoices {
  n: number
  builtin: GridChoice[] // «Из раскладок»
  mine: GridChoice[] // «Мои шаблоны»
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
  faces: FontFaceInfo[]
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
  notice?: string
  version?: string
  scan?: ScanProgress
  fonts?: FontFamily[]
  missing?: string[]
  prefs?: Prefs
  recent?: RecentProject[]
}

// Что тащим мышью.
export type Payload = { kind: 'photos'; ids: string[]; fromSpread?: string } | { kind: 'spread'; id: string; index: number }

// Куда отпускаем.
export type Target =
  | { kind: 'cell'; spread: string; index: number; photo: string; side: 'before' | 'after' | 'center' }
  | { kind: 'spread'; spread: string }
  | { kind: 'mini'; spread: string; index: number; side: 'before' | 'after' }
  // Миниатюра фото в ленте альбома (Timeline): вставка до или после неё.
  | { kind: 'chip'; spread: string; index: number; photo: string; side: 'before' | 'after' }
  | { kind: 'new'; at: number }
  | { kind: 'library' }
  // Место разворота-сетки (пустое или занятое): сторон нет, всё место — «центр».
  | { kind: 'place'; spread: string; place: string; photo: string | null }
  // Зона «Фото фона» в карточке разворота (спека 009): фото встаёт на фон области.
  // many — тянут несколько фото: зона не подсвечивается, бросок объясняет тостом.
  | { kind: 'backdrop'; spread: string; scope: BackdropScope; many?: boolean }

// Вкладка галереи под разворотом: раскладки «Авто» или сетки (спека 007).
export type GalleryTab = 'layouts' | 'grids'

// Зона последнего нажатия мыши (store.zone): Ctrl+A и Del действуют по ней.
export type Zone = 'canvas' | 'library'

// Что уходит в App.Export (JSON-теги Go).
export interface ExportOpts {
  range: string
  quality: number
  dpi: number
  pages: boolean
}
