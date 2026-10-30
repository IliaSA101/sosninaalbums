// Раскладка ЙЦУКЕН ↔ QWERTY: одна таблица на справочник (033) и поиск фото по имени (0.26-124). Чистая логика.

// Нижний регистр и «ё → е».
export const norm = (s: string): string => s.toLowerCase().replace(/ё/g, 'е')

// Клавиши по позиции: i-я латинская лежит на той же клавише, что i-я русская.
const LATIN = "qwertyuiop[]asdfghjkl;'zxcvbnm,.`"
const CYR = 'йцукенгшщзхъфывапролджэячсмитьбюё'

function swap(q: string, from: string, to: string): string {
  let out = ''
  for (const ch of q.toLowerCase()) {
    const i = from.indexOf(ch)
    out += i >= 0 ? to[i] : ch
  }
  return out
}

// Набрано в английской раскладке вместо русской: «j,kj;rf» → «обложка».
export const fromLatin = (q: string): string => swap(q, LATIN, CYR)

// Набрано в русской раскладке вместо английской: «шьп» → «img».
export const toLatin = (q: string): string => swap(q, CYR, LATIN)
