// Окно «QR-код» (100 п. 2): чистые тексты. Код и образец строит Go (QRPreview) — здесь только строки под полем.
import type { app } from '../../../wailsjs/go/models'

export type QRInfo = Pick<app.QRInfo, 'text' | 'side' | 'path' | 'minMM'>

export const QR_TITLE = 'QR-код'
export const QR_OPEN = 'QR-код по ссылке…'
export const QR_OPEN_TIP = 'Код по ссылке ляжет стикером на страницу: ссылку программа не открывает и в сеть не ходит'
// Образец обновляется не на каждую букву: пауза после ввода, мс.
export const QR_DELAY = 150

// Строки под полем: что окажется в коде (если ссылку дополнили) и минимальный размер печати.
export function qrLines(info: Pick<QRInfo, 'text' | 'minMM'> | null, input: string): string[] {
  if (!info) return []
  const out: string[] = []
  if (info.text !== input.trim()) out.push(`Будет в коде: ${info.text}`)
  out.push(`Печатать не мельче ${info.minMM} мм — иначе телефон может не прочитать`)
  return out
}
