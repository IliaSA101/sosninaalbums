// Тексты окна «Имена» (043) по плану Go: строки над и под предпросмотром. Чистая логика, без DOM.
import type { SpreadView } from '../../base/core/types'
import type { FeatureId } from '../../base/prefs/levels'
import { isCover } from '../../base/geom/sheet'
import { canMakeVignette, isVignette } from '../vignette/vignette'
import type { NamesFile, NamesLeft, NamesPlan, NamesRow } from './nametypes'
import { personLine } from './names'

// Сколько строк без фото показываем; остальные — «…ещё N».
export const LEFT_SHOWN = 5

// «Класс.xlsx, лист «Класс» (листов: 2), строк: 26» — что загружено из файла.
export function fileInfo(f: Pick<NamesFile, 'file' | 'sheet' | 'sheets' | 'rows'>): string {
  const sheet = f.sheet ? `, лист «${f.sheet}»${f.sheets > 1 ? ` (листов: ${f.sheets})` : ''}` : ''
  return `${f.file}${sheet}, строк: ${f.rows}`
}

// «Столбцы: Фамилия, Имя · Предмет → учитель · не используются: №». Без заголовка в таблице — пусто.
export function columnsLine(plan: Pick<NamesPlan, 'cols' | 'ignored'> | null): string {
  if (!plan?.cols) return ''
  const used = plan.cols.filter((c) => c && c !== 'Предмет' && c !== 'Файл')
  const parts = [Array.from(new Set(used)).join(', ')]
  if (plan.cols.includes('Предмет')) parts.push('Предмет → учитель')
  if (plan.cols.includes('Файл')) parts.push('Файл → по имени файла')
  if (plan.ignored.length > 0) parts.push(`не используются: ${plan.ignored.join(', ')}`)
  return `Столбцы: ${parts.filter(Boolean).join(' · ')}`
}

// Пустой предпросмотр (нет ни мест, ни фото; 071-11): onCap — «На развороте» / «На листе» / «На обложке» (unitWords).
// Дальше окно показывает кнопку виньетки (noSeatsAction), «Вставить» неактивна. На обложке виньетки нет — совет другой.
export const noSeatsLine = (onCap: string, cover = false): string =>
  cover ? `${onCap} нет мест для имён` : `${onCap} нет мест. Сделайте виньетку или ручную сетку — имена лягут на места по порядку`

// Кнопка пустого предпросмотра: 'setup' — лист уже виньетка, открыть окно; 'make' — обычный лист, сначала сделать
// виньеткой (окно для не-виньетки закрывается само); 'none' — кнопки нет (обложка, уровень без виньетки).
export type NoSeatsAction = 'setup' | 'make' | 'none'
export function noSeatsAction(
  sp: Pick<SpreadView, 'id' | 'kind' | 'vig'> | null | undefined,
  can: (f: FeatureId) => boolean,
): NoSeatsAction {
  if (!sp || isCover(sp)) return 'none'
  if (isVignette(sp)) return 'setup'
  return canMakeVignette(sp, can) ? 'make' : 'none'
}

// Ключ строки предпросмотра: место (лист с местами) или фото («Авто»).
export const rowKey = (r: Pick<NamesRow, 'place' | 'photo'>): string => r.place || r.photo

// Лист с местами (виньетка, ручная сетка) — у строк есть место; «Авто» — только фото.
export const hasSeats = (p: Pick<NamesPlan, 'rows'>): boolean => p.rows.some((r) => r.place)

// Файл в строке: у пустого места фото нет.
export const EMPTY_SEAT = 'место без фото'

// Итог внизу окна: «Подписано: 23 из 25 · строк лишних: 1 · мест без имени: 2» (на «Авто» — «фото без имени»).
export const summaryLine = (p: Pick<NamesPlan, 'named' | 'seats' | 'extra' | 'missing' | 'rows'>): string =>
  `Подписано: ${p.named} из ${p.seats} · строк лишних: ${p.extra} · ${hasSeats(p) ? 'мест' : 'фото'} без имени: ${p.missing}`

// Правая часть строки «Куда ляжет»: групповое место виньетки — «без имени», нет имени — «—».
export const whoText = (r: Pick<NamesRow, 'no' | 'who'>, vig: boolean): string =>
  !r.no && vig ? 'без имени' : r.who ? personLine(r.who) : '—'

const REASONS: Record<string, (file: string) => string> = {
  extra: () => 'нет места на листе',
  nofile: (f) => `нет фото «${f}»`,
  many: (f) => `несколько фото «${f}»`,
  dup: (f) => `фото «${f}» уже занято`,
  noseat: () => 'нет места учителя',
}

// «стр. 3: Орлов Олег — нет фото «99»».
export const leftLine = (l: NamesLeft): string =>
  `стр. ${l.line}: ${personLine(l.who)} — ${(REASONS[l.reason] ?? (() => l.reason))(l.file)}`

// Строки без фото: первые LEFT_SHOWN и сколько осталось (extra — полное число от Go, в left не больше 50).
export function leftList(plan: Pick<NamesPlan, 'left' | 'extra'>): { items: string[]; more: number } {
  const items = plan.left.slice(0, LEFT_SHOWN).map(leftLine)
  return { items, more: Math.max(0, plan.extra - items.length) }
}

// Подсказка виньетки: в таблице людей не столько, сколько мест. Только для вставки по порядку.
export function vigHint(plan: NamesPlan | null): string {
  if (!plan || plan.byFile) return ''
  // Места считает Go вместе с пустыми.
  const t = plan.teacherSeats
  const p = plan.pupilSeats
  if (t + p === 0) return ''
  if (plan.teachers > 0) {
    if (plan.teachers === t && plan.pupils === p) return ''
    return `В таблице: учителей ${plan.teachers}, учеников ${plan.pupils}; на листе мест: ${t} и ${p}.`
  }
  if (plan.pupils === 0 || plan.pupils === t + p) return ''
  return `В таблице: строк ${plan.pupils}; на листе мест: ${t + p}.`
}
