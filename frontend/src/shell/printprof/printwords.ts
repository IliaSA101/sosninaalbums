// Окно «Своя типография» (123 Б3): слова и чистые помощники. Числа и ошибки считает Go (CheckPrintProfile),
// здесь — только подписи, пометка «(своя)» и разбор ответа для показа.
import type { PrintCheckRow, PrintEditCheck, PrintInfo, PrintPlan } from '../../base/core/types'

export const OWN_MARK = '(своя)'
export const NEW_ITEM = 'Своя типография…'
export const WINDOW_TITLE = 'Своя типография'
export const BASE_NONE = 'Без основы (без обложки)'
export const BASE_HINT = 'Изделия обложки, корешки и пределы берутся из основы'
export const NOT_OWN_HINT = 'Встроенную типографию не отправить и не удалить: выберите в «Основе» свою'
export const SAVE_BAD_HINT = 'Исправьте отмеченные поля'
export const NO_FORMATS = 'Ни одного формата. Добавьте строку.'
export const NO_DPI_LINE = 'Сначала плотность — тогда посчитаем, что даёт альбом'
export const IMPORT_HINT = 'Типография — файл .saprint'
export const REMOVE_NOTE = 'Вернуть её можно только файлом .saprint'
export const MAX_FORMATS = 32

// Значение пункта «Своя типография…» в списке «Типография»: невидимый знак, которого в названиях типографий быть не может
// (Go отвергает такие названия), поэтому с группой оно не совпадёт.
export const NEW_VALUE = '\u2063own'

// Пометка «(своя)»: дописывается, если названия ещё нет в самом названии — Go сам предлагает «<основа> (своя)»
// и «(своя 2)», дважды писать не нужно.
export function ownLabel(name: string): string {
  return /\(своя( \d+)?\)/.test(name) ? name : `${name} ${OWN_MARK}`
}

// Название типографии в списке: у своей — с пометкой. Имя без профиля в реестре (общий ряд "" и пр.) — как есть.
export function listLabel(name: string, infos: readonly PrintInfo[]): string {
  return infos.some((i) => i.source === 'own' && i.name === name) ? ownLabel(name) : name
}

// Подпись пункта «Печать» окна «Экспорт»: Go пишет «Для «Альфа»»; у своей пометка идёт после — «Для «Альфа» (своя)».
// kind плана — id типографии.
export function planLabel(p: Pick<PrintPlan, 'kind' | 'title'>, infos: readonly PrintInfo[]): string {
  if (!infos.some((i) => i.source === 'own' && i.id === p.kind)) return p.title
  return /\(своя( \d+)?\)/.test(p.title) ? p.title : `${p.title} ${OWN_MARK}`
}

export interface BaseGroup {
  label: string
  items: PrintInfo[]
}

// Список «Основа»: встроенные, из наборов, свои — без пустых групп; «Без основы» стоит отдельным пунктом.
export function baseGroups(infos: readonly PrintInfo[]): BaseGroup[] {
  const by = (s: PrintInfo['source']) => infos.filter((i) => i.source === s)
  return [
    { label: 'Встроенные', items: by('builtin') },
    { label: 'Из наборов', items: by('library') },
    { label: 'Свои', items: by('own') },
  ].filter((g) => g.items.length > 0)
}

export const isOwnId = (infos: readonly PrintInfo[], id: string): boolean => infos.some((i) => i.id === id && i.source === 'own')

// Миллиметры из px: до сотых, запятая; целое без хвоста («303,95», «304»).
export const mmFmt = (v: number): string => String(+v.toFixed(2)).replace('.', ',')

// Страница по px посчитана (иначе в ячейке «= —»).
export const pageOk = (r: PrintCheckRow | undefined): boolean => !!r && r.pageW > 0
export const sideOk = (r: PrintCheckRow | undefined): boolean => !!r && r.sideW > 0

// «= 303,95 × 202,95 мм» или «= —».
export function pageCell(r: PrintCheckRow | undefined): string {
  return r && pageOk(r) ? `= ${mmFmt(r.pageW)} × ${mmFmt(r.pageH)} мм` : '= —'
}

// Сторонка обложки: не считана и px не введены — «без обложки»; введены, но не годны — «= —».
export function sideCell(r: PrintCheckRow | undefined, coverW: number, coverH: number): string {
  if (r && sideOk(r)) return `= ${mmFmt(r.sideW)} × ${mmFmt(r.sideH)} мм`
  return coverW || coverH ? '= —' : 'без обложки'
}

// Корешок при числе разворотов — «, корешок 12 мм»; 0 — нет основы с обложкой.
export const spineNote = (spine: number): string => (spine > 0 ? `, корешок ${mmFmt(spine)} мм` : '')

// «Файлы: 1-00.jpg — обложка, 1-01.jpg…1-12.jpg».
export const namesLine = (names: string): string => (names ? `Файлы: ${names}` : '')

// Строка «Сейчас альбом даёт …» (lines[0]) с совпавшим форматом — зелёная; совпадение сообщает Go признаком hit.
export const isHitLine = (c: PrintEditCheck | null, i: number): boolean => i === 0 && !!c?.formats?.some((r) => r.hit)

// Поля, у которых в окне есть своё место для ошибки; всё прочее (form и любой ключ, которого окно не знает) — общей строкой.
export const FIELD_KEYS = ['name', 'base', 'dpi', 'safe', 'bleed', 'formats', 'spreads', 'prefix', 'quality', 'url'] as const

export function fieldErr(c: PrintEditCheck | null, key: string): string {
  return c?.errs?.[key] ?? ''
}

// Общая строка формы: ключ form и ошибки полей, для которых места нет.
export function formErr(c: PrintEditCheck | null): string {
  if (!c?.errs) return ''
  return Object.entries(c.errs)
    .filter(([k, v]) => v && !(FIELD_KEYS as readonly string[]).includes(k))
    .map(([, v]) => v)
    .join('. ')
}

// Красное поле строки формата: Go называет поле ошибки (field) — подпись, разворот px или обложка px.
export const rowBad = (r: PrintCheckRow | undefined, part: 'name' | 'px' | 'cover'): boolean => r?.field === part

export const shareHint = (name: string): string => `Записать «${name}.saprint»`
export const removeHint = (name: string): string => `Удалить типографию «${name}»`

export const savedText = (name: string): string => `Типография «${name}» сохранена и выбрана для альбома`
export const removedText = (name: string): string => `Типография «${name}» удалена`
export const sharedText = (file: string): string => `Типография записана в файл «${file}»`
export const replacedText = (name: string, version: number): string => `Типография «${name}» заменена: версия ${version}`
export const sameText = (name: string): string => `Типография «${name}» уже есть: ничего не изменилось`
export const addedTitle = (name: string): string => `Типография «${name}» добавлена из файла`
export const COPIED_LINE = 'Номер из файла уже занят — добавлена копией'

export const removeQuestion = (name: string, id: string): string =>
  `Удалить типографию «${name}»? Альбомы, где она выбрана, не изменятся: в карточке обложки будет написано «Типография не найдена: ${id}». Отменить удаление нельзя.`

// Сводка файла в вопросе: «Форматы: … · Файлы: … · Сайт: …».
export const summaryLine = (lines: readonly string[]): string => lines.join(' · ')
