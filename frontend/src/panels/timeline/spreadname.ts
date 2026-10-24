// Название разворота (спека 008): подпись «1 · Сборы», нормализация ввода.

export const NAME_MAX = 60 // как CleanName в Go: знаков, не байтов

// Подпись разворота: без названия — только номер.
export const spreadLabel = (i: number, name: string | undefined): string => (name ? `${i + 1} · ${name}` : `${i + 1}`)

// sheet — у листа виньетки «лист», не «разворот». Подсказка к подписи: полное название; в ленте ещё и как переименовать.
export const labelTitle = (i: number, name: string | undefined, hint = false, sheet = false): string | undefined => {
  if (!name) return hint ? `Двойной щелчок — название ${sheet ? 'листа' : 'разворота'}` : undefined
  return hint ? `${spreadLabel(i, name)}\nДвойной щелчок — переименовать` : spreadLabel(i, name)
}

// Ввод пользователя → то, что уйдёт в Go: пробелы по краям и переводы строк долой, не длиннее NAME_MAX знаков.
export const normName = (s: string): string =>
  Array.from(s.replace(/[\r\n\t]+/g, ' ').trim())
    .slice(0, NAME_MAX)
    .join('')
    .trim()

// Есть ли смысл звать Go: название реально поменялось.
export const nameChanged = (prev: string | undefined, input: string): boolean => normName(input) !== (prev ?? '')

// Двойной щелчок по номеру (094 §3.1): второй щелчок принадлежит развороту первого. После первого лента едет
// к группе, и второй попадает в чужую — поэтому место и время сверяем сами, а не по dblclick и e.detail.
export interface Click {
  id: string
  x: number
  y: number
  t: number
}

export const DBL_MS = 500 // системное значение Windows по умолчанию
export const DBL_PX = 6 // экранных px

// Разворот первого щелчка, если второй не позже DBL_MS и не дальше DBL_PX (включительно); иначе null.
export function dblOwner(first: Click | null, now: { x: number; y: number; t: number }): string | null {
  if (!first) return null
  const dt = now.t - first.t
  if (dt < 0 || dt > DBL_MS) return null
  return Math.hypot(now.x - first.x, now.y - first.y) <= DBL_PX ? first.id : null
}
