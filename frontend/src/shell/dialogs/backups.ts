// Подписи окна «Резервные копии» (спека 019 E): время, размер, состав. Go присылает числа, текст — здесь.
import { plural } from '../../base/words/plural'

const MONTHS = ['янв', 'фев', 'мар', 'апр', 'мая', 'июн', 'июл', 'авг', 'сен', 'окт', 'ноя', 'дек']

const pad = (n: number) => String(n).padStart(2, '0')
const dayStart = (d: Date) => new Date(d.getFullYear(), d.getMonth(), d.getDate()).getTime()

// «15:30» по местному времени (для тоста «Альбом восстановлен из копии от 15:30»).
export function timeLabel(ms: number): string {
  const d = new Date(ms)
  return `${pad(d.getHours())}:${pad(d.getMinutes())}`
}

// «Сегодня 15:30», «Вчера 09:12», «3 окт 18:40»; копия другого года — «3 окт 2025». now — под тест.
export function whenLabel(ms: number, now: number): string {
  const d = new Date(ms)
  const days = Math.round((dayStart(new Date(now)) - dayStart(d)) / 86_400_000)
  if (days === 0) return `Сегодня ${timeLabel(ms)}`
  if (days === 1) return `Вчера ${timeLabel(ms)}`
  const date = `${d.getDate()} ${MONTHS[d.getMonth()]}`
  return d.getFullYear() === new Date(now).getFullYear() ? `${date} ${timeLabel(ms)}` : `${date} ${d.getFullYear()}`
}

// «512 Б», «128 КБ», «1,2 МБ».
export function sizeLabel(bytes: number): string {
  if (bytes < 1024) return `${Math.max(0, Math.round(bytes))} Б`
  const kb = Math.round(bytes / 1024)
  if (kb < 1024) return `${kb} КБ`
  return `${(bytes / 1048576).toFixed(1).replace('.', ',')} МБ`
}

// «8 разворотов, 120 фото».
export function countLabel(spreads: number, photos: number): string {
  return `${spreads} ${plural(spreads, ['разворот', 'разворота', 'разворотов'])}, ${photos} фото`
}
