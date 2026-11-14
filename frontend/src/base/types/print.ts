// Типы обложки и печати (спека 032, 047).
import type { BackdropScope, Rect, SpreadView } from './sheet'

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
  safe?: number // безопасная линия от загиба, мм (047); 0 — Settings.Safe. Задаёт пресет, в карточке не правится
  spineSafe?: number // то же от корешка
}

// Типография в списке «Типография»: group — Фотосфера / Нетпринт / Фабрика Фотокниги / Свои размеры.
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

// Сторонка обложки для раскладок мест (127): рамка — безопасная зона, мм листа; состояние считает Go.
export interface CoverSide {
  scope: 'left' | 'right' // left — Задняя, right — Лицевая
  label: string
  box: Rect
  places: number // мест на сторонке
  known: boolean // форма узнана: ‹ › листают; false — «· Ручная ·»
  alone: boolean // в ряду одна раскладка, и она стоит
}

// Область фото фона раскладки с подписью (061): список и порядок решает Go.
export interface CoverScope {
  scope: BackdropScope
  label: string
}

// Направляющая PSD (066): v — вертикальная; at — мм от левого (v) или верхнего края своего листа.
export interface GuideLine {
  v: boolean
  at: number
}

// Разметка типографии из Go (066): обложка и развороты; ok/image/warn/note решает Go.
export interface GuideView {
  name: string // имя файла без папки
  key: string // отпечаток файла (путь+размер+время), 12 hex: другой файл — другой key даже при том же имени
  ok: boolean // файл на месте и прочитан
  image: boolean // есть картинка для подложки
  warn?: string // размер или пропорции не те
  note?: string // почему видны только направляющие
  lines?: GuideLine[] // направляющие PSD, ≤ 200
}

export interface CoverView {
  sheet: SpreadView // ID "cover"
  spec: CoverSpec
  preset: string
  spine: number // мм при текущем числе разворотов
  n: number
  over: boolean // разворотов больше, чем в таблице пресета
  layout: CoverLayout
  spineBg: string // #rrggbb плашки корешка; "" — нет
  spineText: string // ID надписи на корешке; "" — нет (058, решение №19)
  zones: CoverZone[]
  drop: CoverZone[] // зоны броска; kind: back front wrap spine
  scopes: CoverScope[] // области фона текущей раскладки: front — Лицевая; wrap — Вся обложка; both — Задняя, Корешок, Лицевая
  safe: Rect[]
  textMaxW: number
  sides: CoverSide[] // только сторонки с непустой зоной; порядок: задняя, лицевая (127)
  markup: GuideView | null // разметка типографии (032, 066); null — файла нет
  pxW: number
  pxH: number
  fix: string // «Не по шаблону Фотосферы…» (047); "" — по шаблону или не Фотосфера
  missing: string // «Типография не найдена: <id>» (123); "" — профиль найден или типографии нет
}

// Типография, для которой есть выгрузка (App.PrintPlans, 081): id профиля реестра — fsfera, fabrika, netprint или own-… у своей (123 Б3).
export type PrintKind = string

// Что уходит в App.Export (JSON-теги Go).
export interface ExportOpts {
  range: string
  quality: number
  dpi: number
  pages: boolean
  cover: boolean // 00.jpg (032); Go игнорирует без обложки и в листе
  coverOnly: boolean // только обложка: диапазон игнорируется
  print: PrintKind | '' // выгрузка для типографии (047, 081): Go сам берёт всё, разворотами, 300 dpi
}

// Кнопка размера страницы из Go (App.PagePresets, 047). Нет поля (0/false) — пресет его не задаёт.
export interface PagePreset {
  label: string
  group: string // "" — общий ряд; «Фотосфера» — свой ряд
  w: number
  h: number
  dpi?: number
  layflat?: boolean
  safe?: number
  bleed?: number // обрез, мм (066); 0/нет — пресет его не задаёт
  hint?: string
}

// Готовность к выгрузке для типографии (App.PrintPlans, 047, 081, 120). В списке планы всех типографий, кроме листа 031:
// страница чужого формата тоже приходит (off непусто, в block — запрет).
// Тексты title/done/site/siteButton готовит Go, UI их рисует.
export interface PrintPlan {
  kind: PrintKind
  title: string // подпись пункта списка «Печать»: «Для Фотосферы Pro», «Для Фабрики Фотокниги»
  format: string // «30×30»; "" — страница не формат этой типографии (пункт виден с подсказкой off)
  off: string // "" — страница в формате типографии; иначе подсказка у пункта списка: «страница 300 × 200 мм — не формат Фабрики Фотокниги»
  pages: string // «Тонкие» | «Картон» | «Двойной картон» | ""
  spreads: number
  spine: number
  min: number
  max: number
  folder: string // имя папки книги
  coverName: string // имя файла обложки: 00.jpg / 1-00.jpg
  quality: number // > 0 — качество задаёт типография, выбор в окне игнорируется
  block: string[] // мешает выгрузке
  coverFix: boolean // в block есть «не по шаблону»: у строки «Исправить»
  fixAt: number // номер этой строки в block; -1 — её нет
  warn: string[]
  memo: string[]
  done: string // заголовок окна «Готово»
  site: string // строка «На сайте: …»
  siteButton: string // «Открыть Фотосферу» / «Открыть Фабрику Фотокниги»
  default: boolean // 123 А2: пункт, который окно выбирает при открытии (printshop.DefaultKind); Go шлёт всегда
  px: string // 123 А2: «7181×2398 px» — файл разворота типографии (в памятке — строка «Файлы»); "" — не посчитан
}

// Ответ App.PagePx (123, design §1.3): страница в мм по введённым px при 300 dpi; err — строка под полем.
export interface PagePxView {
  pageW: number
  pageH: number
  err: string
}

// ---------- Окно «Своя типография» (123 Б3): поля и расчёт приходят из Go, UI показывает ----------

export type PrintSource = 'builtin' | 'library' | 'own'

// Строка формата в типографии из App.PrintProfiles (pxW/pxH — файл разворота, если задан).
export interface PrintInfoFormat {
  name: string
  pageW: number
  pageH: number
  pxW?: number
  pxH?: number
}

// Типография реестра (App.PrintProfiles): встроенная, из набора библиотеки или своя.
export interface PrintInfo {
  id: string
  name: string
  source: PrintSource
  version: number
  hasExport: boolean
  formats: PrintInfoFormat[]
}

// Строка «подпись · разворот px · обложка px» (0/0 в обложке — формата без обложки).
export interface PrintEditFormat {
  name: string
  pxW: number
  pxH: number
  coverW: number
  coverH: number
}

// Поля окна (App.PrintProfileEdit / CheckPrintProfile / SavePrintProfile). Числа — как ввёл пользователь.
export interface PrintEdit {
  name: string
  base: string // id типографии-основы; "" — без обложки
  dpi: number
  layflat: boolean
  safe: number
  bleed: number
  formats: PrintEditFormat[]
  spreads: number // при скольких разворотах введены px обложки; 0 — как в альбоме
  prefix: string
  quality: number // 100 | 95 | 0
  url: string
}

// Итог строки формата: страница и сторонка в мм из px либо ошибка под строкой (sideW 0 — обложки нет).
// field — где ошибка: "name" подпись, "px" разворот, "cover" обложка, "" — ошибки нет; hit — строка совпала с альбомом.
export interface PrintCheckRow {
  pageW: number
  pageH: number
  sideW: number
  sideH: number
  err: string
  field: string
  hit: boolean
}

// Расчёт окна (App.CheckPrintProfile): errs — поле → текст (name base dpi safe bleed formats spreads prefix quality url form).
export interface PrintEditCheck {
  ok: boolean
  errs: Record<string, string>
  formats: PrintCheckRow[]
  spine: number
  lines: string[]
  names: string
}

// Итог «Добавить из файла…» (App.ImportPrintProfile / ConfirmImportPrintProfile).
export interface PrintImport {
  canceled: boolean
  info: PrintInfo
  ask: '' | 'replace' | 'same'
  question: string
  token: string
  copied: boolean
  lines: string[]
}

// Итог «Поделиться файлом…».
export interface PrintShare {
  canceled: boolean
  name: string
}
