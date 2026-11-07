// Тон фото (068): строка CSS filter для живого показа. Зеркало Go (model.Tone.CSS): та же строка, что
// FrameView.filter; паритет держит testdata/parity/tone.json. Формулу в пикселях считает Go (экспорт).
export const MAX_TONE = 50

export interface Tone {
  gray?: boolean
  bright?: number // −50…50, %
  contrast?: number // −50…50, %
}

const clampLevel = (v: number | undefined) => (Number.isFinite(v) ? Math.min(MAX_TONE, Math.max(-MAX_TONE, Math.round(v as number))) : 0)

// "grayscale(1) brightness(120%) contrast(80%)": нулевые части опущены, без тона — "".
export function toneCss(t: Tone | null | undefined): string {
  if (!t) return ''
  const b = clampLevel(t.bright)
  const c = clampLevel(t.contrast)
  const parts: string[] = []
  if (t.gray) parts.push('grayscale(1)')
  if (b !== 0) parts.push(`brightness(${100 + b}%)`)
  if (c !== 0) parts.push(`contrast(${100 + c}%)`)
  return parts.join(' ')
}

// Все ли фото ids на развороте s уже ч/б (подпись пункта «Сделать ч/б / цветным»); пусто — нет.
export function allGray(s: { photos: string[]; frames?: { gray?: boolean }[] } | undefined, ids: string[]): boolean {
  return ids.length > 0 && ids.every((id) => !!s?.frames?.[s.photos.indexOf(id)]?.gray)
}
