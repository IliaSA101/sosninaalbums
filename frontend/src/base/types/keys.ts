// Типы горячих клавиш: JSON Go-пакета internal/keymap (View, Entry, SetResult — design 074 §6).
// Свои, не из wailsjs/models: имена там зависят от namespace, а View пересекается с типом альбома.

export type KeyScope = 'app' | 'canvas' | 'library' | 'bg'

// Строка каталога (keymap.Command).
export interface KeyCommand {
  id: string // совпадает с командой интерфейса (commands.ts)
  label: string
  short: string // на клавише карты
  hint?: string
  group: string
  scope: KeyScope
  feature?: string // функция уровня интерфейса
  defaults: string[]
  fixed?: boolean // служебная: клавиши не меняются
  repeat?: boolean // срабатывает и от автоповтора удержания
  dialogs?: boolean // действует и под открытым окном
}

// Команда с действующими клавишами (keymap.Entry).
export interface KeymapEntry extends KeyCommand {
  keys: string[]
  custom: boolean // своё отличается от стандартного
}

export interface KeyGroup {
  id: string
  title: string
}

export interface KeyScopeInfo {
  id: KeyScope
  title: string
  rank: number // больше — главнее при одной клавише
}

// Служебная клавиша с замком на карте (keymap.Reserved).
export interface KeyLocked {
  chord: string
  anyMods?: boolean
  prefix?: boolean
  reason: string
}

// State.keymap и ответы окна (keymap.View).
export interface KeymapView {
  commands: KeymapEntry[]
  groups: KeyGroup[]
  scopes: KeyScopeInfo[]
  locked: KeyLocked[]
  custom: Record<string, string[]> | null // своё как есть — для «Вернуть»
  changed: number // команд со своим
}

// Команда и её сочетание в ответах правки (keymap.Ref).
export interface KeyRef {
  id: string
  label: string
  chord: string
}

// Ответ окну на правку (keymap.SetResult).
export interface KeymapSetResult {
  view: KeymapView
  conflict: KeyRef | null // не записано: спросить «Заменить?»
  replaced: KeyRef | null
  shadows: KeyRef[]
}
