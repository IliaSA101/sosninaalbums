// Типы окна «Макеты альбома» (131): контракт sa-go ↔ sa-ui, design §9. Зеркало Go (app/looks.go, model.LookPlan/LookOpts);
// ответы приходят из биндингов (wailsjs/go/models.ts: mode и role там просто string, здесь — узкие объединения).
// Поля View.look / drafts, SpreadView.lookStep и TextView.draft — у U2/U3, окну они не нужны.
import type { View } from '../../base/core/types'
import type { DecorItem, DecorTheme } from '../../panels/decor/decor'

// Плитка каталога вида «album» (DecorItem + карточка макета).
export interface LookItem extends DecorItem {
  steps?: number // шагов серии
  from?: number // «от N фото»
  vig?: boolean // есть шаг-виньетка
  cover?: boolean // есть оформление обложки
}

export interface LookCatalog {
  kind: 'album'
  themes: DecorTheme[]
  items: LookItem[]
  recent: string[]
  readOnly: boolean
}

export type LookRole = '' | 'repeat' | 'vignette'

export interface LookStep {
  name: string
  role: LookRole
  places: number
  pages: 1 | 2
}

export interface LookInfo {
  steps: LookStep[]
  cover: boolean
  min: number
  max: number
}

export interface LookCount {
  n: number
  places: number
  fit: number
}

// series — весь альбом по серии; add — добавить повторяемые перед концом; bg — только фоны шагов (собранный альбом).
export type LookMode = 'series' | 'add' | 'bg'

export interface LookOpts {
  mode: LookMode
  spreads: number
}

export interface LookPlan {
  mode: LookMode
  trial: boolean
  touched: number
  full: boolean // в местах есть фото из съёмки: показать «Что сделать»
  spreads: number // разворотов макета после постановки (в LookOpts.spreads Go зажимает число в 1…MaxSpreads)
  places: number
  drafts: number
  replaced: number
  anchors: number
  coverTouched: boolean // обложка входит в touched (0.28-131-174)
  coverReplaced: boolean // обложка входит в replaced
  cover: boolean
  left: number
  prev: string
  missing: number
}

export interface LookResult {
  view: View
  plan: LookPlan
}

// SaveLook (design §8): ids — развороты в порядке альбома; repFrom/repTo — позиции повторяемых среди них с 1 (от > до — повторяемых нет).
export interface SaveLookOpts {
  theme: string
  name: string
  ids: string[]
  repFrom: number
  repTo: number
  places: boolean
  drafts: boolean
  cover: boolean
}

// PlanSaveLook: сухой прогон SaveLook. Отказ — ok=false, steps=0, reason — текст ошибки SaveLook как есть; имя, тему и библиотеку не проверяет.
export interface LookPlanSave {
  ok: boolean
  steps: number // шагов после слияния повторяемых
  max: number // предел шагов (24)
  reason: string
}
