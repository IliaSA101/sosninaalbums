// Блок «Раскладки» окна автосборки (065): чистая логика под тесты. Состояние запросов — autobuild.svelte.ts.
import { plural } from '../../base/words/plural'
import type { AutoPreview, Family } from '../../base/core/types'
import { unitOf, type Unit } from '../../base/geom/sheet'
import { FAMILY_IDS } from '../../base/prefs/families'
import { FAMILIES, galleryLines, galleryRow, type FamilyPass, type GalleryRow } from '../../panels/gallery/variants'

const SPREAD = unitOf(undefined)

// Щелчок по сегменту семейств (сценарий 6): «Все» — сбросить; из «Все» остаётся одно (как в полосе «Варианты»);
// иначе семейство добавляется или снимается. Пустого выбора нет: снял последнее или нажал все шесть — снова «Все» (`[]`).
// Схлопываем только при всех шести, а не при «всех показанных»: в пуле бывают и скрытые сегменты, их выбор терять нельзя.
// Порядок результата — порядок сегментов.
export function toggleFamily(cur: readonly Family[], id: Family | 'all'): Family[] {
  if (id === 'all') return []
  const next = cur.length === 0 ? [id] : cur.includes(id) ? cur.filter((f) => f !== id) : [...cur, id]
  if (next.length === 0 || FAMILY_IDS.every((f) => next.includes(f))) return []
  return FAMILY_IDS.filter((f) => next.includes(f))
}

// Сегменты фильтра: семейства, что есть в «Вариантах» окна, и выбранные (выбор не прячется) — в порядке сегментов.
// Нет ответа (ошибка, пока не считалось) — все шесть: иначе снятый сегмент пропал бы вместе с ошибкой.
export function shownFamilies(pv: Pick<AutoPreview, 'samples'> | null, cur: readonly Family[]): Family[] {
  if (!pv) return FAMILIES.map((f) => f.id)
  const have = new Set<string>(cur)
  for (const s of pv.samples) for (const c of [...s.inOrder, ...s.extra]) have.add(c.family)
  return FAMILIES.filter((f) => have.has(f.id)).map((f) => f.id)
}

const IN_SPREAD: [string, string, string] = ['развороте', 'разворотах', 'разворотах']
const IN_SHEET: [string, string, string] = ['листе', 'листах', 'листах']

// «на 7 разворотах таких нет — там лучшая раскладка»; запасных нет — пусто.
export const fallbackNote = (fallback: number, unit: Unit = SPREAD): string =>
  fallback > 0 ? `на ${fallback} ${plural(fallback, unit === SPREAD ? IN_SPREAD : IN_SHEET)} таких нет — там лучшая раскладка` : ''

// Строка состояния про семейства (сценарий 7); запасные раскладки окно показывает отдельно (fallbackNote, с выделением).
export function familiesHint(cur: readonly Family[]): string {
  if (cur.length === 0) return 'Автосборка выбирает из всех раскладок'
  return `Только: ${FAMILIES.filter((f) => cur.includes(f.id))
    .map((f) => f.label)
    .join(', ')}`
}

// Ряд и строка бледнеют, пока ответ не свежий: план или предпросмотр в пути, либо «от» > «до» (ответ прежний, поля уже другие).
export const layoutsWaiting = (o: { pending: boolean; previewBusy: boolean; inverted: boolean }): boolean =>
  o.pending || o.previewBusy || o.inverted

// Что показать на месте строки состояния: ошибка, нет фото, ожидание (сценарии 5 и 11); '' — показать familiesHint.
export function layoutsStatus(o: { error: boolean; empty: boolean; waiting: boolean }): string {
  if (o.error) return 'Не удалось показать раскладки'
  if (o.empty) return 'Нет фото для сборки'
  if (o.waiting) return 'Считаю…'
  return ''
}

// Подсказка плитки «Что соберётся»: форма; у запасной — почему она здесь.
export const spreadTitle = (s: { formName: string; fallback: boolean }): string | undefined =>
  s.fallback ? `${s.formName || 'Раскладка'} — нет раскладок выбранных семейств, взята лучшая` : s.formName || undefined

export interface AutoPart {
  photos: number // «k фото» — подпись черты
  group: number // с какого разворота сборки взят образец
  row: GalleryRow // «Авто» и за чертой «другие» после фильтра; parts пусто — таких раскладок нет
}

// Ключ раскрытых «Похожих» части: число фото и линия ряда.
export const twinKey = (photos: number, line: string): string => `${photos}|${line}`

// «Варианты» окна: части по числу фото, ряды как в полосе («Авто», за чертой «другие»), под фильтром набора семейств.
// count — плитки после фильтра (число в переключателе, как «В порядке ленты 32»).
export function autoRows(
  pv: Pick<AutoPreview, 'samples'>,
  cur: readonly Family[],
  openTwins: readonly string[],
): { parts: AutoPart[]; count: number } {
  const pass: FamilyPass = (c) => cur.length === 0 || cur.includes(c.family as Family)
  let count = 0
  const parts = pv.samples.map((s) => {
    const lines = galleryLines('spread', { inOrder: s.inOrder, rearranged: [], extra: s.extra }, { why: '', canMore: false })
    const pre = `${s.photos}|`
    const open = openTwins.filter((k) => k.startsWith(pre)).map((k) => k.slice(pre.length))
    const row = galleryRow(lines, 'inOrder', { family: pass, openTwins: open })
    count += [...s.inOrder, ...s.extra].filter((c) => pass(c)).length
    return { photos: s.photos, group: s.group, row }
  })
  return { parts, count }
}
