// Номера разворотов из строки «3, 5-8» (окно «Копировать сетку…», спека 007). Номера с 1.
export interface Ranges {
  nums: number[] // по возрастанию, без повторов; пусто при ошибке
  error: string | null // текст для подсветки поля; null — ввод верный
}

const fail = (error: string): Ranges => ({ nums: [], error })

// max — число разворотов: допустимы номера 1..max. Разделители: запятая, точка с запятой, пробел;
// диапазон — «5-8» (дефис, короткое и длинное тире, пробелы вокруг допустимы).
export function parseRanges(text: string, max: number): Ranges {
  if (!(max >= 1)) return fail('Нет разворотов')
  const src = text.replace(/\s*[-–—]\s*/g, '-').trim()
  if (src === '') return fail('Укажите номера разворотов')
  const set = new Set<number>()
  for (const tok of src.split(/[,;\s]+/)) {
    if (tok === '') continue // хвостовая запятая
    const m = /^(\d{1,6})(?:-(\d{1,6}))?$/.exec(tok)
    if (!m) return fail(`Не понимаю «${tok}»`)
    const a = +m[1]
    const b = m[2] === undefined ? a : +m[2]
    if (a < 1 || b < 1) return fail('Номера начинаются с 1')
    if (a > b) return fail(`Диапазон «${tok}» задан наоборот`)
    if (b > max) return fail(`Разворота ${b} нет (всего ${max})`)
    for (let n = a; n <= b; n++) set.add(n)
  }
  return { nums: [...set].sort((x, y) => x - y), error: null }
}

// Обратно: [3, 5, 6, 7, 8] -> «3, 5-8» (для подсказок и подписи итога).
export function formatRanges(nums: number[]): string {
  const a = [...new Set(nums)].sort((x, y) => x - y)
  const out: string[] = []
  for (let i = 0; i < a.length;) {
    let j = i
    while (j + 1 < a.length && a[j + 1] === a[j] + 1) j++
    out.push(j - i >= 2 ? `${a[i]}-${a[j]}` : a.slice(i, j + 1).join(', '))
    i = j + 1
  }
  return out.join(', ')
}
