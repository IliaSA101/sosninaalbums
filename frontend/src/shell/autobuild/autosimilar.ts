// Блок «Фото» окна автосборки (078): чистые подписи и слияние списка. Состояние запросов — autosimilar.svelte.ts.
import { plural } from '../../base/words/plural'
import type { SigState, SimilarDrop } from '../../base/core/types'
import type { Unit } from '../../base/geom/sheet'
import { unitWord } from './autobuild'

// «Убирать похожие кадры» (078-3): уровни Go = пороги подписи 12 / 24 / 34. Подсказка идёт под строкой, как у «Разнообразия».
export const SIMILAR_OPTIONS: readonly { v: number; label: string; hint: string; title: string }[] = [
  { v: 0, label: 'Нет', hint: 'Сборка берёт все фото', title: 'Ничего не убирать' },
  { v: 1, label: 'Дубли', hint: 'Почти одинаковые кадры серии — остаётся лучший', title: 'Только почти одинаковые кадры подряд' },
  { v: 2, label: 'Очень похожие', hint: 'Та же сцена с небольшим сдвигом — остаётся лучший', title: 'Дубли и кадры той же сцены' },
  { v: 3, label: 'Похожие', hint: 'Убирает больше всего — проверьте ряд ниже', title: 'Все похожие кадры рядом в ленте' },
]

// Что остаётся: общий хвост подсказки (сц. 2) — отметки фотографа главнее подписей
export const SIMILAR_KEEP = '«Беру» и 4–5★ остаются всегда'

export const similarHint = (v: number): string => {
  const o = SIMILAR_OPTIONS.find((x) => x.v === v) ?? SIMILAR_OPTIONS[0]
  return o.v > 0 ? `${o.hint} · ${SIMILAR_KEEP}` : o.hint
}

// «Ч/б развороты» (078-7): доля разворотов целиком
export const GRAY_OPTIONS: readonly { v: number; label: string }[] = [
  { v: 0, label: 'Нет' },
  { v: 1, label: '10 %' },
  { v: 2, label: '20 %' },
  { v: 3, label: '30 %' },
]
export const GRAY_HINT = 'Наименее цветные, не подряд; первый и последний — цветные'

// «Ч/б станет: 3 разворота»; уровень выбран, а разворотов мало — честно говорим, что ни одного
export const grayText = (level: number, k: number, unit: Unit): string => {
  if (level <= 0) return ''
  return k > 0 ? `Ч/б станет: ${k} ${unitWord(k, unit)}` : `Ч/б не будет: ${unit.plural[2]} мало`
}

export const sigsText = (s: SigState | null): string => (s ? `Сверяю кадры: ${s.done} из ${s.total}…` : 'Сверяю кадры…')

const KADR: [string, string, string] = ['кадра', 'кадра', 'кадров'] // после «не возьму» — родительный
const POHOZH: [string, string, string] = ['похожего', 'похожих', 'похожих']

// Строка под «Убирать похожие кадры» без «Показать»: сколько не возьмём (сц. 2); возвращённые считаются отдельно.
export function similarStatus(o: {
  level: number
  waiting: boolean
  sigs: SigState | null
  dropped: number
  kept: number
  error: boolean
}): string {
  if (o.level <= 0) return ''
  if (o.waiting) return sigsText(o.sigs)
  if (o.error) return 'Не удалось сверить кадры'
  if (o.dropped > 0) return `Не возьму ${o.dropped} ${plural(o.dropped, POHOZH)} ${plural(o.dropped, KADR)}`
  return o.kept > 0 ? `Все похожие кадры возвращены (${o.kept})` : 'Похожих кадров нет'
}

// Показать список имеет смысл, когда есть что показывать: убираем что-то или уже вернули
export const canShowDrops = (dropped: number, kept: number): boolean => dropped > 0 || kept > 0

// Ключ набора «Вернуть»: размер и сумма хешей строк — не зависит от порядка, без сортировки 10 000 строк на каждый вопрос.
export function keepKey(keep: ReadonlySet<string>): string {
  let sum = 0
  for (const id of keep) {
    let h = 2166136261
    for (let i = 0; i < id.length; i++) h = Math.imul(h ^ id.charCodeAt(i), 16777619)
    sum = (sum + (h >>> 0)) >>> 0
  }
  return `${keep.size}:${sum}`
}

// Показанный ряд: ответ Go (кого убираем) + прежние плитки, которые пользователь вернул (Go их уже не отдаёт);
// порядок — порядок ленты.
export function mergeDrops(
  prev: readonly SimilarDrop[] | null,
  res: readonly SimilarDrop[],
  keep: ReadonlySet<string>,
  order: Readonly<Record<string, number>>,
): SimilarDrop[] {
  const m = new Map<string, SimilarDrop>()
  for (const d of prev ?? []) if (keep.has(d.id)) m.set(d.id, d)
  for (const d of res) m.set(d.id, d)
  return [...m.values()].sort((a, b) => (order[a.id] ?? 0) - (order[b.id] ?? 0))
}

// Подсказка плитки: имя, статус и на что похож
export const dropTitle = (name: string, like: string, back: boolean): string =>
  `${name} — ${back ? 'вернётся в сборку' : 'не возьму'}; похож на ${like}. Щелчок — ${back ? 'убрать снова' : 'вернуть'}`

// Сколько вернулось из показанного
export const keptOf = (drops: readonly SimilarDrop[], keep: ReadonlySet<string>): number => drops.filter((d) => keep.has(d.id)).length
