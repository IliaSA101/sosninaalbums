// Линейки по краям разворота (спека 010, design-ui §3): метки в мм, подписи без слипания.
// Чистая логика, без Svelte; px↔мм разворота считает вызывающий (ширина разворота на экране).

export interface Tick {
  at: number // мм от угла разворота
  major: boolean // крупное деление (с подписью)
  label: string // подпись крупного, у мелкого пусто
}

// Шаги подписи, мм; берётся наименьший, при котором подписи не ближе MIN_LABEL_PX.
export const STEPS = [1, 2, 5, 10, 20, 50, 100, 200]
export const MIN_LABEL_PX = 48

// Мелкие деления: шаг/5, у 2 и 20 — шаг/2 (иначе деления не попадают на целые мм).
function minorOf(step: number): number {
  return step === 2 || step === 20 ? step / 2 : step / 5
}

// Шаг подписи для масштаба pxPerMm.
export function labelStep(pxPerMm: number): number {
  return STEPS.find((s) => s * pxPerMm >= MIN_LABEL_PX) ?? STEPS[STEPS.length - 1]
}

const round3 = (v: number): number => Math.round(v * 1000) / 1000 + 0 // + 0 убирает -0

// Деления на отрезке [mm0, mm1] (мм разворота) при pxPerMm. Индекс деления — целое, поэтому
// дробные шаги (0,2 мм) не накапливают ошибку.
export function ticks(mm0: number, mm1: number, pxPerMm: number): Tick[] {
  if (!(pxPerMm > 0) || !(mm1 >= mm0)) return []
  const step = labelStep(pxPerMm)
  const minor = minorOf(step)
  const per = Math.round(step / minor)
  const out: Tick[] = []
  for (let i = Math.ceil(mm0 / minor - 1e-9); i * minor <= mm1 + 1e-9; i++) {
    const major = i % per === 0
    const at = round3(i * minor)
    out.push({ at, major, label: major ? String(at) : '' })
  }
  return out
}

// Полоса выделенного на линейке: [от, до] в мм, зажатая в [0, max]; нет пересечения — null.
export function selSpan(from: number, to: number, max: number): [number, number] | null {
  const a = Math.max(0, Math.min(from, to))
  const b = Math.min(max, Math.max(from, to))
  return b > a ? [a, b] : null
}
