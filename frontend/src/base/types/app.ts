import type { Photo, ScanProgress } from './marks'
import type { CoverView } from './print'
import type { Rect, SpreadView } from './sheet'

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
  libPhotos?: string[] // ID записей со своим из библиотеки (048): их нет в «Найти пропавшие», пропавшие ищутся загрузкой
  marksRev: number // растёт при смене своих отметок (спека 004): тогда UI перечитывает Api.Marks()
  unfilled?: string[] // пустые переменные из текстов, по алфавиту
  unnamed?: number // фото с полями имени в подписи и без имени
  cover?: CoverView | null // обложка (032); нет — её нет в альбоме
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
