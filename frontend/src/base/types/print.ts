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
  markup: GuideView | null // разметка типографии (032, 066); null — файла нет
  pxW: number
  pxH: number
  fix: string // «Не по шаблону Фотосферы…» (047); "" — по шаблону или не Фотосфера
}

// Типография, для которой есть выгрузка (App.PrintPlans, 081).
export type PrintKind = 'fsfera' | 'fabrika'

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
  title: string // подпись флажка: «Для Фотосферы Pro», «Для Фабрики Фотокниги»
  format: string // «30×30»; "" — страница не формат этой типографии (флажок виден с подсказкой off)
  off: string // "" — страница в формате типографии; иначе подсказка у флажка: «страница 300 × 200 мм — не формат Фабрики Фотокниги»
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
}
