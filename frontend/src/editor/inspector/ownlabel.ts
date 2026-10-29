// Подпись своего кадра (098): пропорции окна «16 : 9». Малые целые дают точную подпись, иначе — десятичная.
// Только текст: сами пропорции и их смысл считает Go (FrameView.own).
export function ownLabel(own: number): string {
  if (!Number.isFinite(own) || own <= 0) return ''
  for (let d = 1; d <= 10; d++) {
    const n = Math.round(own * d)
    // Допуск 0,5 %: кадр хранится с округлением 1e-4, а 16 : 9 ≠ 1,7778 ровно
    if (n > 0 && Math.abs(n / d / own - 1) < 0.005) return `${n} : ${d}`
  }
  return `${own.toFixed(2).replace('.', ',')} : 1`
}
