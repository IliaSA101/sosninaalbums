import type { Settings } from '../../base/core/types'

// Числовые поля формы «Настройки альбома»; bleed (066) — обрез, мм.
export const NUMS = ['pageW', 'pageH', 'margin', 'gap', 'dpi', 'safe', 'bleed'] as const

// Поле number при пустом вводе даёт null — приводим к числу при сравнении и сохранении. Всё, что форма не
// показывает, но Go хранит в Settings, обязано пройти здесь: SetSettings принимает структуру целиком, и
// потерянное поле стёрлось бы при «Применить» (так было бы с обрезом).
export const norm = (x: Settings): Settings => ({
  pageW: Number(x.pageW),
  pageH: Number(x.pageH),
  margin: Number(x.margin),
  gap: Number(x.gap),
  dpi: Number(x.dpi),
  safe: Number(x.safe),
  bleed: Number(x.bleed) || 0, // нет поля = 0
  layflat: !!x.layflat,
  foldAny: !!x.layflat && !!x.foldAny, // как зажим в Go: без layflat «любых» нет
  bg: x.bg ?? '', // "" — белый
  sheet: x.sheet, // Go режим не меняет: лист остаётся листом
})

export function same(a: Settings, b: Settings): boolean {
  const x = norm(a)
  const y = norm(b)
  return x.layflat === y.layflat && x.foldAny === y.foldAny && x.bg === y.bg && NUMS.every((k) => x[k] === y[k])
}
