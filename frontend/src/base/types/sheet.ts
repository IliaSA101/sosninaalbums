import type { model } from '../../../wailsjs/go/models'

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
// shape/round (044) — форма и скругление, только когда заданы: '' / 'oval' и 0…50 % меньшей стороны.
export type Box = RotRect & { shape?: string; round?: number }

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
  gray: boolean // ч/б (068)
  bright: number // яркость −50…50, %
  contrast: number // контраст −50…50, %
  filter: string // готовая строка CSS filter из Go; '' — без тона
  cellRot: number // угол ячейки (= места, вокруг его центра); в «Авто» 0
  z: number // слой в общем стеке разворота (от Go): z-index = 10 + z
  shape?: string // форма места (044): '' / нет — прямоугольник, 'oval' — эллипс в рамке; у «Авто» нет
  radius?: number // радиус скругления, мм (от Go); для рисования
  // Окно рамочки (079): фото сидит в нём, а не во всём месте. Без рамочки win = ячейка; нет поля (старый вид, «Авто») — то же.
  win?: Rect // окно, мм разворота, в осях места до поворота; img посчитан для него
  winShape?: string // форма окна: 'oval' — овал, вписанный в окно; иначе скругление winRadius
  winRadius?: number // скругление окна, мм (max(R − min(отступы), 0)); нет — прямые углы
  fade?: number // прозрачность фото 0…1 (0 — непрозрачное), шаг 0,01; рамочка не прозрачнеет
  own?: number // свой кадр (098): пропорции окна (ширина/высота), в котором стоит кадр; нет — кадр от места
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
  who?: ('last' | 'first' | 'middle' | 'subject')[] // поля имени шаблона по порядку (037); у своей подписи нет
}

// Заглушка на пустом месте листа с шаблоном подписи (030): серый «[Фамилия]» или табличка «Орлова [Имя]» (071), только на холсте.
// place — ID места ("" в «Авто»), photo — "" у пустого места.
export interface StubView extends TextView {
  place: string
  photo: string
  who?: ('last' | 'first' | 'middle' | 'subject')[] // поля имени шаблона: у заглушки фото и у пустого места (071)
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
  subj?: string // предмет учителя (049); ключ как у Go (json:"subj")
}

// Api.Person / Api.PlacePerson: действующее имя; fromFile — взято из имени файла.
export interface PersonView {
  last: string
  first: string
  middle: string
  subject: string
  fromFile: boolean // под табличкой с именем false; под табличкой только с предметом (071-16) бывает true — имя фото из файла
  teacher: boolean // место учителя виньетки: карточка показывает «Предмет»
  place: string // ID места, чья табличка видна (071); "" — видно имя фото. У PlacePerson — ID места, если табличка не пуста
  seatNamed: boolean // в табличке этого места есть имя (а не только предмет); false и при place == ""
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
// Рамочка места (079, от Go: model.BorderView). URL и maskUrl — без px, UI дописывает &px= сам.
export interface BorderView {
  kind: string // встроенная: ключ вида; у картинки ''
  photo: string // картинка: ID записи альбома
  lib: string // ключ своей в библиотеке (my:…); у встроенной ''
  preset: string // плитка sa:fr-…, поставившая рамочку; '' — правили руками
  name: string // имя вида или плитки
  inks: string[] // действующие цвета
  width: number // толщина, мм (встроенная)
  defWidth?: number // толщина вида по умолчанию, мм (двойной щелчок по «Толщине»); нет — двойной щелчок молчит
  scale: number // картинка: 1 — как задумано
  ins: number[] // отступы окна [l, t, r, b], мм
  url: string
  maskUrl: string // '' — маски нет
  pic: boolean
  open: boolean
  known: boolean // false — вида нет в этой сборке (файл новее)
}

export interface PlaceView extends RotRect {
  id: string
  photo: string // "" — пустое место
  fit: '' | 'contain' // "" — с обрезкой (cover + кадр), contain — вписано целиком
  spine: boolean // пересекает корешок при выключенном layflat
  rot: number // градусы по часовой вокруг центра места, (−180, 180]
  z: number // слой в общем стеке разворота (от Go)
  role?: string // роль места виньетки: 'pupil' | 'teacher' | 'group'; нет или '' — ученик (031)
  shape?: string // форма (044): '' / нет — прямоугольник, 'oval' — эллипс, вписанный в рамку
  round?: number // скругление углов, % меньшей стороны, 0…50; у oval 0
  radius?: number // то же в мм (считает Go): для рисования
  border?: BorderView // рамочка (079); нет — без рамочки
}

// Место для SetPlaces: только геометрия (мм разворота).
export interface PlaceBox extends Rect {
  id: string
}

// Ручка рамки места: стороны света.
export type Handle = 'n' | 's' | 'e' | 'w' | 'ne' | 'nw' | 'se' | 'sw'

// Область фото фона: весь разворот, одна страница или (обложка «Задняя + корешок + лицевая», 061) корешок.
// Узоры, цвет и сетка корешок не принимают.
export type BackdropScope = 'spread' | 'left' | 'right' | 'spine'

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
  lib?: string // ключ своего из библиотеки (048, `my:<16 hex>`); нет — обычное фото
  album?: boolean // это фон альбома (098): щелчком не выделяется, лист его своим фоном не считает
}

// Узор программы на фоне области (048): параметры генератора, не файл. Рисует /media/gen по url.
export interface PatternSpec {
  kind: string
  inks?: string[]
  scale?: number // 0.25…4; нет — 1
  seed?: number
  fade?: number // прозрачность 0…1, как у фото фона
}

export interface PatternView {
  scope: BackdropScope
  area: Rect // область, мм листа
  pat: PatternSpec // после зажима, inks дополнены цветами генератора
  name: string // пусто — генератор новее этой версии
  inks: number // сколько цветов показывать
  random: boolean // есть «Перемешать»
  known: boolean // false — «Нужна новая версия программы»: рисуется пусто
  url: string // без px: UI дописывает &px=
  preset?: string // плитка каталога `sa:<ключ>`, поставившая узор (№201); нет — правили руками или альбом старше поля
  album?: boolean // это фон альбома (098): щелчком не выделяется, лист его своим фоном не считает
}

// Стикер на листе (048 Ф2, design §6): картинка поверх мест, мм листа, рамка до поворота, поворот вокруг центра.
// Процедурный — art и url без px (UI дописывает &px=), свой из библиотеки — photo (запись в Project.Photos) и lib.
// Тип берётся из сгенерированных биндингов (поля и регистр не расходятся с Go); без convertValues — чтобы
// тесты и разворот из события собирались обычными объектами. rot — градусы по часовой, (−180, 180]; fade — 0…1;
// z — слой в общем стеке листа (от Go); inks — сколько цветов показывать; gen — процедурный; dpi — у своего,
// 0 — размер неизвестен; known=false — «Нужна новая версия программы»; stretch — плашка/волна без своей пропорции.
export type StickerView = Omit<model.StickerView, 'convertValues'>

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
  patterns?: PatternView[] // узоры фона (048), 0–2 шт., под фото фона; нет поля — пусто
  stickers?: StickerView[] // стикеры (048 Ф2) в порядке слоёв; нет поля — пусто
  noAlbumBg?: boolean // фон альбома на этом листе скрыт (098): «Фон альбома скрыт · Показать»; нет — виден
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
  shape?: string // форма мест учеников и учителей (044): '' / 'oval'
  round?: number // скругление этих мест, %
  places: number
  max: number
  group: boolean
  tplName?: string // название шаблона на листе (свой — имя из файла; удалённый — копия имени)
  manual?: boolean // места правили руками: «Ручная», «Вернуть авто» зовёт ResetVignette
  ownName?: string // имя своего шаблона из копии на листе
}

// Окно «Настроить виньетку» (031 dialog §5): параметры листа и поля шапки одним запросом.
export interface VigSetup {
  vig: { tpl: string; teachers: number; pupils: number; ratio: string; ttext: string; head: string; shape?: string; round?: number }
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
  shape?: string // форма мест своего шаблона (044), по образцу ratio; нет — шаблон формы не задаёт
  round?: number // скругление, %
  gone?: boolean // стоит на листе, но удалён из файла шаблонов: переименовать и удалить нельзя
  // 049 Ф1 (Go отдаёт с G3): вид для фильтра (side, big, inrow, group, round, staff; у своих пусто); у каталожных
  // «кругов» ещё shape: 'oval' и ratio: '1:1' — подсказка окну
  family?: string
  head?: Box // шапка листа (доли листа): превью рисует её тоном заголовка
}
