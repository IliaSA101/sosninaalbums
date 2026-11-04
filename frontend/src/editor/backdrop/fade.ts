// Прозрачность фото фона (спека 017): в Go доля 0…1 с шагом 0,01, в интерфейсе — проценты.
// Зажим и округление окончательные делает Go; здесь только перевод для ползунка и CSS.

// Доля → целые проценты для подписи и ползунка.
export function fadePct(f: number): number {
  return Number.isFinite(f) ? Math.min(100, Math.max(0, Math.round(f * 100))) : 0
}

// Проценты ползунка → доля для Go (мусор — 0).
export function pctFade(p: number): number {
  return Number.isFinite(p) ? Math.min(100, Math.max(0, Math.round(p))) / 100 : 0
}

// CSS opacity рамки: непрозрачность = 1 − прозрачность, как в экспорте. При 0 % атрибута нет.
export function opacityOf(fade: number): number | undefined {
  const p = fadePct(fade)
  return p > 0 ? (100 - p) / 100 : undefined
}
