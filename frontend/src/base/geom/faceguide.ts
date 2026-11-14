// Силуэт под лицо (130, silhouette.md): ответ Go App.FaceGuide — окно места и контур в мм, начало — левый верх окна.
// Интерфейс ничего не считает, только переводит точки в строки пути SVG (viewBox = окно в мм) и отбрасывает мусор.

export interface FaceGuideData {
  on: boolean
  w: number
  h: number
  eyeY: number
  path: number[][]
}

const EPS = 1e-6

const fin = (x: unknown): x is number => typeof x === 'number' && Number.isFinite(x)

// Ответ пригоден к показу: силуэт включён, окно положительное, линия глаз и все точки — конечные числа, точек хватает на фигуру.
export function validGuide(g: FaceGuideData | null | undefined): g is FaceGuideData {
  if (!g || !g.on || !fin(g.w) || !fin(g.h) || g.w <= 0 || g.h <= 0 || !fin(g.eyeY)) return false
  return Array.isArray(g.path) && g.path.length >= 3 && g.path.every((p) => Array.isArray(p) && p.length === 2 && fin(p[0]) && fin(p[1]))
}

// Сотые доли мм — достаточно для экрана и короче в DOM.
const num = (v: number) => String(Math.round(v * 100) / 100)

// Замкнутый контур одним путём; последнюю точку с первой соединяет Z (Go их не повторяет).
export function polyD(path: number[][]): string {
  return path.map((p, i) => `${i ? 'L' : 'M'}${num(p[0])} ${num(p[1])}`).join('') + 'Z'
}

// Вуаль: окно минус силуэт (fill-rule: evenodd). Что у силуэта выходит за окно по бокам, обрезает сам svg.
export function veilD(g: FaceGuideData): string {
  return `M0 0H${num(g.w)}V${num(g.h)}H0Z${polyD(g.path)}`
}

// Линия контура: без звеньев, лежащих на низу окна или ниже, — иначе по нижнему краю тянулась бы лишняя черта.
// Обход начинается сразу после первого такого звена, чтобы связный кусок (в том числе через замыкающее звено)
// остался одним подпутём M…L…; если таких звеньев нет, линия замкнута (Z).
export function outlineD(g: FaceGuideData): string {
  const p = g.path
  const n = p.length
  const low = (i: number) => p[i][1] >= g.h - EPS && p[(i + 1) % n][1] >= g.h - EPS
  let first = 0
  let closed = true
  for (let i = 0; i < n; i++) {
    if (low(i)) {
      first = i + 1
      closed = false
      break
    }
  }
  let d = ''
  let pen = false // перо опущено: предыдущее звено нарисовано
  for (let k = 0; k < n; k++) {
    const i = (first + k) % n
    if (low(i)) {
      pen = false
      continue
    }
    const a = p[i]
    const b = p[(i + 1) % n]
    d += (pen ? '' : `M${num(a[0])} ${num(a[1])}`) + `L${num(b[0])} ${num(b[1])}`
    pen = true
  }
  return closed ? d + 'Z' : d
}
