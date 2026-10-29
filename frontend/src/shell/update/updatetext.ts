// Тексты и кнопки обновления (111, design §3.1): чистые функции от UpdateInfo. Go готовых строк не шлёт, кроме error.
import { bytesOf } from '../packs/packnet'
import type { UpdateInfo } from './updateapi'

// Что делает кнопка: update.svelte.ts связывает действие с методом Go.
export type UpdateAct = 'check' | 'download' | 'cancel' | 'install' | 'page'

export interface UpdateBtn {
  act: UpdateAct
  label: string
  primary?: boolean
  disabled?: boolean
}

export interface UpdateView {
  line: string // строка состояния
  btns: UpdateBtn[]
  pageLink: boolean // ссылка «Страница выпуска»
  hint: string // подсказка под строкой («Поставьте установщиком…»)
  notes: string[] // «Что нового» из подписанного файла (недоверенный текст)
  pct: number | null // полоса хода; null — полосы нет
  busy: boolean // полоса без известной доли
}

export const CHECK_LABEL = 'Проверить обновления'
export const START_LABEL = 'Проверять при запуске (раз в день)'
export const PAGE_LABEL = 'Страница выпуска'
export const SETUP_HINT = 'Поставьте установщиком — дальше программа будет обновляться сама'
export const WHATS_NEW_LABEL = 'Что нового'
export const RESTART_LABEL = 'Перезапустить'

// Повтор после ошибки: есть предложенная версия (ошибка скачивания) — снова скачиваем, иначе снова проверяем.
export const retryAct = (i: UpdateInfo): UpdateAct => (i.version && i.mode !== 'page' ? 'download' : 'check')

const check = (disabled = false): UpdateBtn => ({ act: 'check', label: CHECK_LABEL, disabled })

// «Скачиваем обновление — 3 из 8 МБ»; размер неизвестен — без чисел.
export function progressLine(i: Pick<UpdateInfo, 'done' | 'total'>): string {
  return i.total > 0 ? `Скачиваем обновление — ${bytesOf(i.done, i.total)}` : 'Скачиваем обновление…'
}

// Доля 0…100; размер неизвестен — null (полоса без значения).
export function progressPct(i: Pick<UpdateInfo, 'done' | 'total'>): number | null {
  if (i.total <= 0) return null
  return Math.max(0, Math.min(100, Math.round((i.done / i.total) * 100)))
}

function readyLine(i: UpdateInfo): string {
  const head = `Обновление ${i.version} готово.`
  return i.mode === 'setup'
    ? `${head} Программа закроется и запустит установщик`
    : `${head} Программа закроется, установит его и откроется снова; альбом сохранится`
}

// Блок «Обновления» в «О программе»: что показать по фазе и режиму.
// retry — что не удалось (знает update.svelte.ts); не задано — по данным Go.
export function updateView(i: UpdateInfo, retry?: UpdateAct): UpdateView {
  const v: UpdateView = {
    line: '',
    btns: [check()],
    pageLink: false,
    hint: i.mode === 'setup' ? SETUP_HINT : '',
    notes: [],
    pct: null,
    busy: false,
  }
  switch (i.phase) {
    case 'checking':
      v.line = 'Проверяем…'
      v.btns = [check(true)]
      break
    case 'latest':
      v.line = 'Установлена последняя версия'
      break
    case 'available':
      v.line = `Вышла версия ${i.version}`
      v.notes = i.notes ?? []
      // Mac, Linux: «Скачать» ведёт на страницу выпуска, отдельная ссылка была бы вторым входом в то же место.
      // «Проверить обновления» постоянная (С5): стоит первой, как в остальных состояниях блока.
      v.btns = [
        check(),
        i.mode === 'page' ? { act: 'page', label: 'Скачать', primary: true } : { act: 'download', label: 'Обновить', primary: true },
      ]
      v.pageLink = i.mode !== 'page'
      break
    case 'downloading':
      v.line = progressLine(i)
      v.pct = progressPct(i)
      v.busy = v.pct === null
      v.btns = [{ act: 'cancel', label: 'Отменить' }]
      break
    case 'ready':
      v.line = readyLine(i)
      v.btns = [{ act: 'install', label: i.mode === 'setup' ? 'Перезапустить и установить' : 'Перезапустить и обновить', primary: true }]
      break
    case 'error': {
      v.line = i.error
      v.btns = [{ act: retry ?? retryAct(i), label: 'Повторить' }]
      v.pageLink = true
      break
    }
    default:
      break // idle: строки нет, одна кнопка
  }
  return v
}

// Тост при запуске: нашлось новее (update:offer). Windows — «Обновить» (скачивание в фоне), Mac и Linux — «Скачать».
export function offerToast(i: UpdateInfo): { text: string; label: string; act: 'download' | 'page' } {
  const text = `Вышла версия ${i.version}`
  return i.mode === 'page' ? { text, label: 'Скачать', act: 'page' } : { text, label: 'Обновить', act: 'download' }
}

// Скачали и проверили: можно ставить. Переносной копии нажатие запускает установщик.
export function readyToast(i: UpdateInfo): { text: string; label: string } {
  return { text: `Обновление ${i.version} готово`, label: i.mode === 'setup' ? 'Установить' : RESTART_LABEL }
}

export const startedText = (v: string): string => `Скачиваем обновление ${v}…`
export const justText = (v: string): string => `Программа обновлена до ${v}`
export const failedText = (v: string): string => `Обновление до ${v} не установилось`
