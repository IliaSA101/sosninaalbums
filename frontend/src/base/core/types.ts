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
  text: string // с подставленными полями (030): так видно на холсте
  src?: string // шаблон с {Фамилия}…, если отличается от text; в редактор идёт он
  style: TextStyle
  lines: TextLine[]
  z: number // слой в общем стеке разворота; у подписи — сразу над её фото
}

// Подпись к фото: id пуст, рамка — место подписи.
export interface CaptionView extends TextView {
  photo: string
  over: boolean // на фото, а не под ним
  overflow: number // мм, которых не хватает; 0 — помещается
  sheet?: boolean // стиль от листа (SpreadView.caps), а не свой
  pad?: number // действующий отступ от фото, мм (нужен от Go; нет — 1)
  who?: ('last' | 'first' | 'middle')[] // поля имени шаблона по порядку (037); у своей подписи нет
}

// Заглушка на пустом месте листа с шаблоном подписи (030): серый «[Фамилия]», только на холсте.
// place — ID места ("" в «Авто»), photo — "" у пустого места.
export interface StubView extends TextView {
  place: string
  photo: string
  who?: ('last' | 'first' | 'middle')[] // у заглушки фото; у пустого места нет
}

// Поля шапки листа для {Заголовок} {Подзаголовок} {Год} {Руководитель} (031 dialog §4): свойство листа, не альбома.
// Старые {Школа} {Класс} {Учитель} Go понимает как синонимы. Пустое поле — нет ключа.
export interface Vars {
  title?: string
  subtitle?: string
  year?: string
  lead?: string
}

// Имя человека на фото (Api.SetPerson); все поля пусты — снова имя из файла.
export interface Person {
  last?: string
  first?: string
  middle?: string
}

// Api.Person: действующее имя; fromFile — взято из имени файла.
export interface PersonView {
  last: string
  first: string
  middle: string
  fromFile: boolean
}

// Общий стиль подписей листа (SpreadView.caps). wrap: true — переносить, иначе ужимать кегль.
export interface SheetCaps {
  style: TextStyle
  pad: number // мм, 0…20
  over?: boolean
  wrap?: boolean
  text?: string // шаблон подписи фото без своей; "" — нет
}

// Место в режиме «Сетка» (спека 007), мм разворота. Порядок в SpreadView.places = порядок слоёв (z-index).
export interface PlaceView extends RotRect {
  id: string
  photo: string // "" — пустое место
  fit: '' | 'contain' // "" — с обрезкой (cover + кадр), contain — вписано целиком
  spine: boolean // пересекает корешок при выключенном layflat
  rot: number // градусы по часовой вокруг центра места, (−180, 180]
  z: number // слой в общем стеке разворота (от Go)
  role?: string // роль места виньетки: 'pupil' | 'teacher' | 'group'; нет или '' — ученик (031)
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
  w: number // ширина листа, мм (Go): разворот 2·pageW, лист виньетки pageW, обложка — свой размер
  h: number // высота листа, мм
  mid: number // шов, мм; 0 — шва и левой/правой страниц нет
  rects: Rect[]
  frames: FrameView[] // по одному на фото, в том же порядке (Go шлёт [], не null)
  texts: TextView[] // Go шлёт [], не null
  captions: CaptionView[] // Go шлёт [], не null
  bg: [string, string] // действующий цвет левой и правой страницы
  bgSet: [string, string] // как задано у разворота; "" — наследуется
  layout: string // ключ действующей раскладки; "" — разворот пуст
  variant: number // позиция в ряду листания; -1 — раскладки нет в рядах
  variants: number // длина этого ряда
  more: boolean // ряд «Авто» можно досчитать (разворот на 17+ фото): счётчик «12+»
  rearranged: boolean // ряд листания — «С перестановкой порядка»
  pages: PageView[] | null // null — раскладка не постраничная; иначе [левая, правая]
  // Режим «Сетка» (спека 007): photos/rects/frames — из мест, layout "", variant -1, pages null.
  grid: boolean
  places: PlaceView[] | null // null — «Авто»
  backdrops: BackdropView[] | null // 0–2 шт.; Go может прислать null
  lowDPI: number // кадры и фоны разворота с 0 < dpi < View.minDPI
  stubs?: StubView[] // заглушки подписей (030); нет поля — пусто
  caps?: SheetCaps // общий стиль подписей листа; нет — у каждой подписи свой
  kind?: '' | 'vignette' // вид листа (031); нет или '' — обычный
  vig?: VignetteView // параметры виньетки; есть только при kind = 'vignette'
  vars?: Vars // свои поля шапки листа (031 dialog); нет — пусты
  fields?: string[] // имена для меню «Вставить поле» на этом листе; нет — Фамилия, Имя, Отчество
}

// Параметры виньетки листа (031, SpreadView.vig). places — мест сейчас, max — предел (64), group — у шаблона есть групповое.
export interface VignetteView {
  tpl: string
  teachers: number
  pupils: number
  ratio?: string // '2:3' | '3:4' | '4:5' | '1:1'; нет — 3:4
  ttext?: string // шаблон подписи учителя; нет — как у листа
  head?: string
  places: number
  max: number
  group: boolean
  tplName?: string // название шаблона на листе (свой — имя из файла; удалённый — копия имени)
  manual?: boolean // места правили руками: «Ручная», «Вернуть авто» зовёт ResetVignette
  ownName?: string // имя своего шаблона из копии на листе
}

// Окно «Настроить виньетку» (031 dialog §5): параметры листа и поля шапки одним запросом.
export interface VigSetup {
  vig: { tpl: string; teachers: number; pupils: number; ratio: string; ttext: string; head: string }
  vars: Vars
}

// Сухой прогон окна (Api.PreviewVignette): лист после «Применить» (ID прежний) и сколько фото уйдёт в ленту.
export interface VigPreview {
  spread: SpreadView
  left: number
}

// Превью шаблона виньетки (Api.VignetteChoices): boxes — доли листа, roles — по одной на место.
export interface VigChoice {
  id: string
  name: string
  boxes: Box[]
  roles: string[]
  pupilW: number // ширина портрета ученика, мм
  small: boolean // портрет мельче 15 мм
  over: boolean // мест больше предела
  own?: boolean // свой шаблон (id «my:<id>»), после каталога
  ratio?: string // портрет своего шаблона; выбор ставит его в форму
  gone?: boolean // стоит на листе, но удалён из файла шаблонов: переименовать и удалить нельзя
}

// ---------- Обложка (спека 032): геометрию и зоны считает Go, UI переводит мм в проценты ----------

// Шаг таблицы корешка: до upto разворотов включительно — mm.
export interface SpineStep {
  upto: number
  mm: number
}

// Размеры обложки (CoverSpec). w/h = 0 — по странице альбома; нет steps — корешок = a + b·n.
export interface CoverSpec {
  preset: string // id пресета или "custom"
  w?: number
  h?: number
  wrap: number
  hinge?: number
  steps?: SpineStep[]
  a?: number
  b?: number
}

// Типография в списке «Типография»: group — Фотосфера / Нетпринт / Свои размеры.
export interface CoverPreset {
  id: string
  name: string
  group: string
  spec: CoverSpec
}

// Зона направляющих (kind: back front spine hinge wrap trim safe) или броска (back front wrap), мм листа.
export interface CoverZone {
  kind: string
  rect: Rect
  label: string
}

export type CoverLayout = 'front' | 'both' | 'wrap'

export interface CoverView {
  sheet: SpreadView // ID "cover"
  spec: CoverSpec
  preset: string
  spine: number // мм при текущем числе разворотов
  n: number
  over: boolean // разворотов больше, чем в таблице пресета
  layout: CoverLayout
  spineBg: string // #rrggbb плашки корешка; "" — нет
  zones: CoverZone[]
  drop: CoverZone[]
  safe: Rect[]
  textMaxW: number
  guide: string // имя файла разметки; "" — нет
  guideOK: boolean
  guideWarn: boolean // пропорции разметки не совпадают с листом
  pxW: number
  pxH: number
}

// Варианты для галереи раскладок (Api.LayoutChoices). Rects — весь разворот в порядке photos.
// Семейство раскладки (041): подпись для фильтра подборки; пусто — Go ещё не размечает.
export type Family = 'dense' | 'hero' | 'air' | 'bleed'

export interface Choice {
  key: string
  rects: Rect[]
  family?: Family | '' // нет или пусто — без семейства
  twin?: number // индекс более раннего в этом ряду с тем же видом; −1 или нет — своя картинка
}

export interface ChoiceRows {
  inOrder: Choice[]
  rearranged: Choice[]
  block: '' | 'same' | 'many' // почему ряд с перестановкой пуст
  more: boolean // ряд inOrder можно досчитать: плитка «Ещё»
  extra?: Choice[] // 041: «Другие варианты» — воздушные раскладки и с главным фото (8–16 фото) вне ряда «Авто» (только у разворота)
}

// Разнообразие автосборки (041): 0 «Меньше» (как 0.10), 1 «Обычное», 2 «Больше».
export type Variety = 0 | 1 | 2

// Ответ Go на «сколько получится разворотов» (027, design §8): ползунок окна автосборки и подсказка.
export interface AutoPlan {
  photos: number // фото в сборку (без сеток и фонов)
  spreads: number // N после зажима
  minSpreads: number // низ ползунка: ⌈photos / maxPer⌉
  maxSpreads: number // верх: min(photos, 1000 − разворотов)
  avg: number
  min: number // «от a»
  max: number // «до b»
  reuse: number // 035 U1: сколько пустых разворотов в конце займёт сборка (min(хвост, N))
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
  safe: number // безопасная зона, мм от края страницы; 0 — без зоны
  sheet?: boolean // альбом из отдельных листов (031): pageW — ширина листа, шва нет
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
  unfilled?: string[] // пустые переменные из текстов, по алфавиту
  unnamed?: number // фото с полями имени в подписи и без имени
  cover?: CoverView | null // обложка (032); нет — её нет в альбоме
}

// Ответ Api.PasteNames: extra — лишних строк списка, missing — фото листа без имени.
export interface NamesResult {
  view: View
  extra: number
  missing: number
}

// Результат правки сетки: left — фото, не вставшие или ушедшие в ленту.
export interface GridResult {
  view: View
  left: number
}

// Итог броска фото на ручной разворот (042, Api.DropPhotos). ask — правки не было: нужен вопрос и второй вызов с rest.
export interface DropResult {
  view: View
  ask: boolean
  fit: number // сколько встанет (встало) в имеющиеся места, с целевым
  total: number // сколько фото пришло на лист
  fresh: string[] | null // ID новых мест плитки: выделяются, как после «Место»
  grown: number // на сколько выросло «Учеников» нетронутой виньетки
  out: boolean // прежнее фото целевого места ушло в ленту (замена)
  left: number // не встало: «Не добавлять» или обложка
  small: boolean // место плитки мельче 1/10 меньшей стороны страницы
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
  group: string // script | kids | serif | sans
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
  cancelled?: boolean // только OpenProject: диалог закрыт без выбора, состояние прежнее
}

// Что тащим мышью.
export type Payload = { kind: 'photos'; ids: string[]; fromSpread?: string } | { kind: 'spread'; id: string; index: number }

// Куда отпускаем.
export type Target =
  | { kind: 'cell'; spread: string; index: number; photo: string; side: 'before' | 'after' | 'center' }
  // at — доля ширины листа под курсором 0…1 (есть только у самого листа на холсте; у соседа в ряду нет)
  | { kind: 'spread'; spread: string; at?: number }
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
  // Плитка обложки в ленте (032): бросок фото создаёт обложку; несколько фото — тост «только одно».
  | { kind: 'cover'; many?: boolean }

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
  cover: boolean // 00.jpg (032); Go игнорирует без обложки и в листе
  coverOnly: boolean // только обложка: диапазон игнорируется
}
