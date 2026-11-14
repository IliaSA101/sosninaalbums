// Сырой отрезок значения поля верхнего уровня JSON — как json.RawMessage в Go: байты от первого знака значения до последнего,
// пробелы и \uXXXX на месте. Нужен пределу `themes` ≤ 4 КиБ: JSON.parse длину исходного текста не отдаёт, а в Go предел считается
// по сырым байтам (аудит 170 п. 5). Работает по байтам, а не по тексту: битый UTF-8 в Go остаётся своими байтами, а в строке
// стал бы U+FFFD (3 байта). Вызывать только для уже разобранного JSON — проверки правильности здесь нет.

const WS = new Set([0x20, 0x09, 0x0a, 0x0d])

/** Конец строки, начатой кавычкой в позиции i: индекс после закрывающей кавычки. */
function stringEnd(buf, i) {
  for (i++; i < buf.length; i++) {
    if (buf[i] === 0x5c) i++
    else if (buf[i] === 0x22) return i + 1
  }
  return buf.length
}

/** Конец значения, начатого в позиции i: строка, массив/объект с вложенностью или литерал. */
function valueEnd(buf, i) {
  const c = buf[i]
  if (c === 0x22) return stringEnd(buf, i)
  if (c !== 0x5b && c !== 0x7b) {
    while (i < buf.length && !WS.has(buf[i]) && buf[i] !== 0x2c && buf[i] !== 0x7d && buf[i] !== 0x5d) i++
    return i
  }
  let depth = 0
  while (i < buf.length) {
    const b = buf[i]
    if (b === 0x22) {
      i = stringEnd(buf, i)
      continue
    }
    if (b === 0x5b || b === 0x7b) depth++
    else if (b === 0x5d || b === 0x7d) depth--
    i++
    if (depth === 0) return i
  }
  return i
}

/**
 * Сырой отрезок последнего поля верхнего уровня, чьё имя (после раскрытия \u-экранов) подходит match; undefined — поля нет.
 * null — тоже значение: в Go RawMessage получает «null» и перекрывает прежнее, как любое другое повторение ключа.
 */
export function rawField(buf, match) {
  let i = 0
  const skip = () => {
    while (i < buf.length && WS.has(buf[i])) i++
  }
  skip()
  if (buf[i] !== 0x7b) return undefined
  i++
  let found
  for (;;) {
    skip()
    if (buf[i] !== 0x22) return found
    const ke = stringEnd(buf, i)
    const key = JSON.parse(buf.subarray(i, ke).toString('utf8'))
    i = ke
    skip()
    i++ // «:»
    skip()
    const ve = valueEnd(buf, i)
    if (match(key)) found = buf.subarray(i, ve)
    i = ve
    skip()
    if (buf[i] !== 0x2c) return found
    i++
  }
}
