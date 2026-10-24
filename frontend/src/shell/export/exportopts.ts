// Настройки окна экспорта (спека 009 U8, design §6.4): сборка ExportOptions, оценка размера файла,
// последний выбор в localStorage (свойство программы, не проекта — как view.svelte.ts).
import type { ExportOpts, PrintKind, PrintPlan, Settings, View } from '../../base/core/types'
import type { ExportUnit } from './exportrange'
import { newSheetSize, type SheetSettings } from '../../base/geom/sheet'

// Тип лежит в основе (store.runExport), здесь — для окна и тестов.
export type { ExportOpts }
export { exportDoneText } from '../../base/words/errtext'

export type Quality = 100 | 95 | 80
export const QUALITIES: { v: Quality; label: string }[] = [
  { v: 100, label: 'максимум' },
  { v: 95, label: 'печать' },
  { v: 80, label: 'просмотр' },
]
export const DPI_PRESETS: { v: number; label: string }[] = [
  { v: 300, label: 'печать' },
  { v: 150, label: 'просмотр' },
  { v: 72, label: 'экран' },
]

// Выбор в окне. dpi 0 — «из настроек альбома».
export interface ExportPrefs {
  quality: Quality
  dpi: number
  pages: boolean
  cover: boolean // выгружать обложку (00.jpg)
  coverOnly: boolean // только обложка
  print: '' | PrintKind // «Для Фотосферы Pro» / «Для Фабрики Фотокниги» (047, 081); действует, только если у страницы есть такой план
}
export type ExportScope = 'all' | 'current' | 'range'

export const DEFAULT_PREFS: ExportPrefs = { quality: 95, dpi: 0, pages: false, cover: true, coverOnly: false, print: '' }

// Байт на пиксель по качеству. ГИПОТЕЗА по типичным JPEG фотокамер: сверить на настоящих фото при QA.
const BYTES_PER_PX: Record<Quality, number> = { 100: 0.9, 95: 0.45, 80: 0.2 }

// Выгрузка для типографии всегда 300 dpi (Go принудительно ставит то же).
export const PRINT_DPI = 300

const px = (mm: number, dpi: number) => Math.round((mm / 25.4) * dpi)

// Примерный размер одного файла: разворот (или страница при pages) по числу пикселей.
export function estimateBytes(s: SheetSettings & Pick<Settings, 'dpi'>, dpi: number, quality: Quality, pages: boolean): number {
  const d = dpi > 0 ? dpi : s.dpi
  const sh = newSheetSize(s)
  // Лист без шва — один файл; у разворота «по страницам» — половина.
  const w = sh.mid > 0 ? (pages ? 1 : 2) * px(sh.mid, d) : px(sh.w, d)
  return w * px(sh.h, d) * (BYTES_PER_PX[quality] ?? BYTES_PER_PX[95])
}

// «≈ 25 МБ»: до 10 МБ — один знак.
export function formatSize(bytes: number): string {
  const mb = bytes / (1024 * 1024)
  if (mb < 0.1) return '< 0,1 МБ'
  return `≈ ${mb < 10 ? mb.toFixed(1).replace('.', ',') : Math.round(mb)} МБ`
}

// Строка «Перед выгрузкой: …» (035 U8): только то, что есть; нечего сказать — пусто (строки в окне нет).
export function exportWarnings(v: Pick<View, 'emptyPlaces' | 'lowDPI'>, missing: number): string {
  const parts = [
    v.emptyPlaces > 0 ? `пустые места: ${v.emptyPlaces}` : '',
    v.lowDPI > 0 ? `мало разрешения: ${v.lowDPI} фото` : '',
    missing > 0 ? `не найдено фото: ${missing}` : '',
  ].filter(Boolean)
  return parts.length ? `Перед выгрузкой: ${parts.join(', ')}` : ''
}

// Предел разворотов типографии (085): в обычной выгрузке — строка-предупреждение (только совет, «Выгрузить» доступна);
// в режиме печати (любой типографии) пределы проверяет Go в плане (красный список), второй строки не нужно.
export function limitWarn(v: Pick<View, 'pageLimit'>, printOn: boolean): string {
  return printOn ? '' : (v.pageLimit?.warn ?? '')
}

// Что попадёт в выгрузку — для итога по объёму (035 U22). Обложка считается как один разворот.
export interface SizeScope {
  scope: ExportScope
  spreads: number // всего разворотов (для «Все»)
  units: ExportUnit[] | null // разобранный диапазон; null — ввод неверен
  cover: boolean // обложка уйдёт в выгрузку (в том числе «только обложка»)
  only: boolean // только обложка
  print?: boolean // для типографии (047, 081): все развороты, 300 dpi, разворотами; обложка — как в cover
}

// «≈ X на разворот, всего ≈ Y»; «всего» — если файлов больше одного, неверный диапазон — без итога.
export function sizeSummary(
  s: SheetSettings & Pick<Settings, 'dpi'>,
  dpi: number,
  quality: Quality,
  pages: boolean,
  unit: string,
  w: SizeScope,
): string {
  if (w.print) {
    dpi = PRINT_DPI
    pages = false
    w = { ...w, scope: 'all', only: false }
  }
  const one = estimateBytes(s, dpi, quality, pages)
  const spread = estimateBytes(s, dpi, quality, false)
  const page = estimateBytes(s, dpi, quality, true)
  let total: number | null = w.cover ? spread : 0
  if (!w.only) {
    if (w.scope === 'all') total += w.spreads * spread
    else if (w.scope === 'current') total += spread
    else if (w.units) total += w.units.reduce((sum, [, p]) => sum + (p === 0 ? spread : page), 0)
    else total = null
  }
  const head = `${formatSize(one)} на ${unit}`
  return total !== null && total > one ? `${head}, всего ${formatSize(total)}` : head
}

// Пункты обложки показываются, если она есть и проект не лист-виньетка (Go в этих случаях флаги игнорирует).
export function coverAvailable(v: Pick<View, 'cover' | 'settings'>): boolean {
  return !!v.cover && !v.settings.sheet
}

// План включённой типографии: выбор запомнен, и такой план пришёл (флажок показан). Иначе null — режима нет.
// План чужого формата (format "") тоже берём: в block у него запрет, и тихо уйти в обычную выгрузку нельзя (120).
export function activePlan(plans: readonly PrintPlan[] | null, pref: '' | PrintKind): PrintPlan | null {
  return (pref && plans?.find((p) => p.kind === pref)) || null
}

// Щелчок по флажку типографии: включённый выключается, другой переключает (включён не больше одного).
export const togglePrint = (pref: '' | PrintKind, kind: PrintKind, checked: boolean): '' | PrintKind =>
  checked ? kind : pref === kind ? '' : pref

// Качество в режиме печати: у плана с quality > 0 оно своё (Go игнорирует выбор окна), иначе — выбор.
export function printQuality(plan: Pick<PrintPlan, 'quality'> | null, q: Quality): Quality {
  if (!plan || plan.quality <= 0) return q
  return plan.quality >= 100 ? 100 : plan.quality >= 95 ? 95 : 80
}

// Подсказка у флажка: «(папка «Иванов», 300 dpi, качество 100, с обложкой)».
export function printHint(plan: Pick<PrintPlan, 'folder' | 'quality'>, hasCover: boolean): string {
  const q = plan.quality > 0 ? `, качество ${plan.quality}` : ''
  return `(папка «${plan.folder}», ${PRINT_DPI} dpi${q}${hasCover ? ', с обложкой' : ''})`
}

// Сборка опций. current — номер текущего разворота с 1. «Все» — пустой диапазон.
// hasCover — есть ли что выгружать: без обложки флаги уходят false. «Только обложка» включает обложку и гасит диапазон.
// onCover — открыта обложка: «Текущий» тогда значит обложку, а не разворот с номером, на котором стояли до неё.
// plan — включённая типография (activePlan); null — обычная выгрузка.
export function buildOptions(
  scope: ExportScope,
  current: number,
  rangeText: string,
  p: ExportPrefs,
  hasCover = false,
  onCover = false,
  plan: Pick<PrintPlan, 'kind' | 'quality'> | null = null,
): ExportOpts {
  // Для типографии Go берёт всё сам (диапазон, страницы, dpi и «только обложка» игнорирует), но уходят честные значения.
  if (plan) {
    const quality = plan.quality > 0 ? plan.quality : p.quality
    return { range: '', quality, dpi: PRINT_DPI, pages: false, cover: hasCover, coverOnly: false, print: plan.kind }
  }
  const only = hasCover && (p.coverOnly || (scope === 'current' && onCover))
  const range = only || scope === 'all' ? '' : scope === 'current' ? String(current) : rangeText.trim()
  return { range, quality: p.quality, dpi: p.dpi, pages: p.pages, cover: hasCover && (p.cover || only), coverOnly: only, print: '' }
}

// Строки памятки в окне экспорта: первые три через « · » (формат, страницы, развороты), остальное — в окне «Готово».
export const memoLine = (memo: readonly string[]): string => memo.slice(0, 3).join(' · ')

// «Исправить» стоит у строки блока с номером plan.fixAt (Go: -1 — такой строки нет); по тексту строки не ищем.
export const hasFix = (plan: { coverFix: boolean; fixAt: number }, i: number): boolean => plan.coverFix && i === plan.fixAt

// Допуск «Выгрузить» (047, 081). Пока планы не пришли (null), а типография выбрана, выгружать нельзя: блок неизвестен.
export function canExport(o: { busy: boolean; bad: boolean; printPref: '' | PrintKind; plans: readonly PrintPlan[] | null }): boolean {
  if (o.busy) return false
  if (o.printPref && o.plans === null) return false
  const plan = activePlan(o.plans, o.printPref)
  if (plan) return plan.block.length === 0
  return !o.bad
}

// Запоздавший ответ плана: номер запроса уже не последний.
export const planStale = (mine: number, last: number): boolean => mine !== last

// Образец имени файла для окна: номер разворота с нулями до ширины Go (internal/export: max(2, цифр в total)).
export function sampleName(n: number, total: number, pages: boolean): string {
  const w = Math.max(2, String(Math.max(total, n, 1)).length)
  const num = String(Math.max(1, n)).padStart(w, '0')
  return pages ? `(${num}-1.jpg, ${num}-2.jpg)` : `(${num}.jpg)`
}

const KEY = 'exportOpts'

// Выбор типографии из хранилища. Прежняя редакция (до 081) хранила флажок fsfera: true; чужое значение print — «выключено».
function printOf(v: { print?: unknown; fsfera?: unknown }): ExportPrefs['print'] {
  if (v.print === 'fsfera' || v.print === 'fabrika') return v.print
  return v.print === undefined && v.fsfera === true ? 'fsfera' : ''
}

export function loadPrefs(): ExportPrefs {
  const o = { ...DEFAULT_PREFS }
  try {
    const v = JSON.parse(localStorage.getItem(KEY) ?? 'null')
    if (v && typeof v === 'object') {
      if (v.quality === 100 || v.quality === 95 || v.quality === 80) o.quality = v.quality
      if (Number.isInteger(v.dpi) && v.dpi >= 72 && v.dpi <= 1200) o.dpi = v.dpi
      if (typeof v.pages === 'boolean') o.pages = v.pages
      if (typeof v.cover === 'boolean') o.cover = v.cover
      if (typeof v.coverOnly === 'boolean') o.coverOnly = v.coverOnly
      o.print = printOf(v)
    }
  } catch {
    // Без хранилища или с порченым значением — по умолчанию.
  }
  return o
}

export function savePrefs(p: ExportPrefs) {
  try {
    localStorage.setItem(KEY, JSON.stringify(p))
  } catch {
    // Не сохранилось — не страшно.
  }
}
