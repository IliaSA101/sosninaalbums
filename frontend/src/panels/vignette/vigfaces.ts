// Подгон по лицам (130): тексты кнопки, окна и тоста — чистые функции под тесты. Состояние и вызовы — vigfaces.svelte.ts.
import { keymap } from '../../base/keys/keymap.svelte'
import type { FaceReport } from './facesapi'

export const FACES_LABEL = 'Подогнать по лицам'
// Клавиша отмены — из реестра keymap (страж keys.guard), руками её не пишем.
export const facesTip = (): string =>
  `${keymap.tip('Лица одного размера, глаза на одной линии. Групповое фото и «Целиком» не трогаются; «Отменить»', 'undo')} возвращает`
// Окно как у 098 (правленые кадры): вопрос и кнопки; N — только в галочке (keepText).
export const FACES_ASK = { title: 'Подогнать по лицам?', ask: 'Кадр у части фото уже настроен. Подогнать по лицам?', ok: 'Подогнать' }
export const FACES_TOAST_MS = 20_000

// Подпись кнопки: пока идёт поиск — «Лица 12 / 25», как «Экспорт 12 / 40»; число мест Go ещё не знает — «Лица…».
export function facesLabel(p: { done: number; total: number } | null): string {
  if (!p) return FACES_LABEL
  return p.total > 0 ? `Лица ${p.done} / ${p.total}` : 'Лица…'
}

// Группа итога: заголовок и причины со счётчиками. Одна причина на место (design §5), поэтому группы не пересекаются.
// Причины в скобках короткие (строка ≈ 60–80 знаков); полные названия — в F1 (vig-faces).
type Group = { title: string; reasons: [keyof FaceReport, string][] }
const GROUPS: Group[] = [
  {
    title: 'Проверьте',
    reasons: [
      ['noFace', 'без лица'],
      ['many', 'несколько лиц'],
      ['missing', 'нет фото'],
    ],
  },
  {
    title: 'Не по норме',
    reasons: [
      ['small', 'мелкое'],
      ['big', 'крупное'],
      ['edge', 'у края'],
    ],
  },
  {
    title: 'Не тронуто',
    reasons: [
      ['contain', '«Целиком»'],
      ['manual', 'вручную'],
      ['changed', 'изменили'],
    ],
  },
]

const num = (r: FaceReport, k: keyof FaceReport): number => {
  const v = r[k]
  return typeof v === 'number' ? v : 0
}

// Сколько мест с фото в отчёте: подогнанные и все три группы.
export const facesTotal = (r: FaceReport): number =>
  num(r, 'fitted') + GROUPS.reduce((a, g) => a + g.reasons.reduce((b, [k]) => b + num(r, k), 0), 0)

// Строка тоста, одна, ≈ 60 знаков: «Подогнано 20 из 25 · Проверьте 3 · Не по норме 1 · Не тронуто 1».
// Нулевые группы не пишутся; если в группе причина одна — она в скобках: «Проверьте 2 (без лица)».
// idle — Go ничего не изменил (повтор): «Лица уже подогнаны: менять нечего» и те же группы.
export function faceText(r: FaceReport, idle = false): string {
  if (facesTotal(r) === 0) return 'На листе нет фото для подгона'
  const head = idle ? 'Лица уже подогнаны: менять нечего' : `Подогнано ${num(r, 'fitted')} из ${facesTotal(r)}`
  const parts = GROUPS.flatMap((g) => {
    const used = g.reasons.filter(([k]) => num(r, k) > 0)
    const n = used.reduce((a, [k]) => a + num(r, k), 0)
    if (n === 0) return []
    return [used.length === 1 ? `${g.title} ${n} (${used[0][1]})` : `${g.title} ${n}`]
  })
  return [head, ...parts].join(' · ')
}

// Что выделяет «Показать»: первые две группы (Go отдаёт их ID мест), без повторов; пусто — кнопки в тосте нет.
export const faceShowIds = (r: FaceReport): string[] => [...new Set([...(r.check ?? []), ...(r.short ?? [])])]
