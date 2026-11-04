// Сравнение `watch` ряда плиток по значениям (050): родитель отдаёт новый массив при каждой перерисовке,
// а сбрасывать прокрутку нужно только при смене самих значений (не при «Ещё»).
export function sameWatch(a: unknown, b: unknown): boolean {
  if (Object.is(a, b)) return true
  if (!Array.isArray(a) || !Array.isArray(b) || a.length !== b.length) return false
  return a.every((v, i) => Object.is(v, b[i]))
}
