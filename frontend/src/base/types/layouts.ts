import type { model } from '../../../wailsjs/go/models'
import type { View } from './app'
import type { Box } from './sheet'

// Варианты для галереи раскладок (Api.LayoutChoices). Rects — весь разворот в порядке photos.
// Семейство раскладки (041, 051): подпись для фильтра подборки; Go шлёт строкой, пусто — без семейства.
export type Family = 'dense' | 'hero' | 'air' | 'bleed' | 'strip' | 'figure'

// Тип — прямо из биндинга (key, rects, family, twin, formName), полей не дублируем.
// formName: «Лента», «Лесенка», «Плотно»… — у каждой плитки; twin: −1 — своя картинка.
// convertValues — служебный метод класса биндинга, в данных его нет.
export type Choice = Omit<model.Choice, 'convertValues'>

export interface ChoiceRows {
  inOrder: Choice[]
  rearranged: Choice[]
  block: '' | 'same' | 'many' // почему ряд с перестановкой пуст
  more: boolean // ряд inOrder можно досчитать: плитка «Ещё»
  extra?: Choice[] // 041: «Другие варианты» — воздушные раскладки и с главным фото (8–16 фото) вне ряда «Авто» (только у разворота)
}

// Разнообразие автосборки (041): 0 «Меньше» (как 0.10), 1 «Обычное», 2 «Больше».
export type Variety = 0 | 1 | 2

// Ответ Go на «сколько получится разворотов» (027, design §8; 051: from/to/cuts): ползунок окна автосборки и подсказка.
// Тип — прямо из биндинга, полей не дублируем.
export type AutoPlan = model.AutoPlan

// Диапазон «фото на разворот» окна автосборки (051 §8): поля AutoOpts кроме spreads и variety.
// auto — диапазон от среднего (minPer и maxPer тогда не действуют); series — не разрывать серии съёмки.
export type AutoRange = Pick<model.AutoOpts, 'auto' | 'minPer' | 'maxPer' | 'series'>

// Ответ App.AutoPreview (065): ряд «Варианты» (образцы по числу фото плана) и «Что соберётся». Тип — из биндинга.
export type AutoPreview = model.AutoPreview

export interface Choices {
  spread: string
  rev: number // для какого rev посчитано
  whole: ChoiceRows
  left: ChoiceRows | null // null — раскладка не постраничная или на странице < 2 фото
  right: ChoiceRows | null
}

export type LayoutScope = 'spread' | 'left' | 'right'

// Результат правки сетки: left — фото, не вставшие или ушедшие в ленту.
export interface GridResult {
  view: View
  left: number
  named: string[] // ID фото, получивших имя из названия файла этим шагом (060); Go шлёт всегда
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
  named: string[] // ID фото, получивших имя из названия файла этим шагом (060)
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
  stickers: number // стикеров заменится (049 Ф2, «Копировать оформление»); Go шлёт всегда
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
