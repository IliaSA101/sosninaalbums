import { beforeEach, describe, expect, it, vi } from 'vitest'
import { Reorder, type ReorderSrc } from './reorder.svelte'

// Кадры — макрозадачами: снятие класса landing ждёт два кадра.
vi.stubGlobal('requestAnimationFrame', (cb: () => void) => setTimeout(cb, 0))

// Лента из трёх групп по 100 px с чертой 14: x 0, 114, 228; середины 50, 164, 278.
const el = (x: number) => ({ offsetLeft: x, offsetTop: 8, offsetWidth: 100, offsetHeight: 80 }) as HTMLElement
const view = { left: 0, scroll: 0 } // лента на экране и её прокрутка
const src: ReorderSrc = {
  mode: 'line',
  items: () => [el(0), el(114), el(228)],
  origin: () => ({ x: view.left - view.scroll, y: 500 }),
  area: () => ({ left: 0, top: 500, right: 1000, bottom: 600 }),
}
const Y = 540 // внутри ленты по высоте

let r: Reorder
beforeEach(() => {
  view.left = 0
  view.scroll = 0
  r = new Reorder()
})

// Ждём по условию, не по таймеру: settle ждёт tick() и два кадра, на нагруженной машине это дольше фиксированных мс.
const until = (cond: () => boolean) => vi.waitFor(() => expect(cond()).toBe(true), { timeout: 2000, interval: 2 })

describe('живая перестановка (096)', () => {
  it('своё место — цели нет, смещений нет', () => {
    const s = r.session(src, 0, 50, Y)
    expect(r.phase).toBe('drag')
    expect(r.busy).toBe(true)
    expect(s.hit(60, Y)).toBeNull()
    expect(r.to).toBe(0)
    expect(r.css(1)).toBeUndefined()
    expect(r.css(2)).toBeUndefined()
    expect(r.css(0)).toBe('translate(10px, 0px)') // поднятый едет за курсором
    expect(r.lifted(0)).toBe(true)
    s.drop(null)
    expect(r.phase).toBe('idle')
    expect(r.css(0)).toBeUndefined()
  })

  it('отпускание — цель gap, land без перехода, после ответа смещения сняты', async () => {
    const s = r.session(src, 0, 50, Y)
    // за серединой второй группы: промежуток — после неё
    expect(s.hit(170, Y)).toEqual({ kind: 'gap', at: 2, to: 1 })
    expect(r.css(1)).toBe('translate(-114px, 0px)')
    expect(r.css(2)).toBeUndefined()
    // за серединой третьей — в конец
    expect(s.hit(290, Y)).toEqual({ kind: 'gap', at: 3, to: 2 })
    expect(r.css(2)).toBe('translate(-114px, 0px)')
    s.drop({ kind: 'gap', at: 3, to: 2 })
    expect(r.phase).toBe('land')
    expect(r.landing).toBe(true) // переходы выключены: встаёт сразу
    expect(r.lifted(0)).toBe(false)
    expect(r.css(0)).toBe('translate(228px, 0px)') // на место последней
    expect(r.css(1)).toBe('translate(-114px, 0px)') // соседи держатся до ответа
    s.settle(true)
    await until(() => !r.busy) // последним снимается busy — после tick() и двух кадров
    expect(r.phase).toBe('idle')
    expect(r.css(0)).toBeUndefined()
    expect(r.css(2)).toBeUndefined()
    expect(r.landing).toBe(false)
    expect(r.busy).toBe(false)
  })

  it('перенос назад: соседи вправо, поднятый — на место первой', () => {
    const s = r.session(src, 2, 278, Y)
    expect(s.hit(40, Y)).toEqual({ kind: 'gap', at: 0, to: 0 })
    expect(r.css(0)).toBe('translate(114px, 0px)')
    expect(r.css(1)).toBe('translate(114px, 0px)')
    s.drop({ kind: 'gap', at: 0, to: 0 })
    expect(r.css(2)).toBe('translate(-228px, 0px)')
  })

  it('ошибка Go — смещения назад', async () => {
    const s = r.session(src, 0, 50, Y)
    s.hit(290, Y)
    s.drop({ kind: 'gap', at: 3, to: 2 })
    s.settle(false)
    expect(r.phase).toBe('idle')
    expect(r.landing).toBe(false) // назад — с переходом
    expect(r.css(1)).toBeUndefined()
    expect(r.busy).toBe(true) // 150 мс возврата черты ещё скрыты
    await new Promise((res) => setTimeout(res, 200))
    expect(r.busy).toBe(false)
  })

  it('Esc и уход за 60 px — назад, возврат — продолжается', () => {
    const s = r.session(src, 0, 50, Y)
    expect(s.hit(290, 610)).not.toBeNull() // 10 px ниже ленты — ещё внутри
    expect(s.hit(290, 661)).toBeNull() // дальше 60 px
    expect(r.out).toBe(true)
    expect(r.lifted(0)).toBe(false) // поднятый уезжает на место с переходом
    expect(r.css(0)).toBeUndefined()
    expect(r.css(2)).toBeUndefined()
    expect(s.hit(290, 560)).toEqual({ kind: 'gap', at: 3, to: 2 }) // вернулся
    expect(r.css(2)).toBe('translate(-114px, 0px)')
    expect(r.lifted(0)).toBe(true)
    s.cancel() // Esc
    expect(r.phase).toBe('idle')
    expect(r.css(2)).toBeUndefined()
    expect(s.hit(290, 560)).toBeNull() // после отмены сессия молчит
  })

  it('автопрокрутка: поднятый держится под курсором', () => {
    const s = r.session(src, 0, 50, Y)
    s.hit(60, Y)
    expect(r.css(0)).toBe('translate(10px, 0px)')
    view.scroll = 200 // лента уехала на 200 px, курсор стоит
    expect(s.hit(60, Y)).toEqual({ kind: 'gap', at: 2, to: 1 }) // точка содержимого 260 — за серединой второй
    expect(r.css(0)).toBe('translate(210px, 0px)')
  })

  it('запоздалый ответ прежней сессии не трогает новую', async () => {
    const a = r.session(src, 0, 50, Y)
    a.hit(170, Y)
    a.cancel()
    const b = r.session(src, 1, 164, Y)
    a.settle(true)
    a.cancel()
    // settle прежней сессии без фазы land уходит в back() синхронно; отрицательное условие — даём лентам отработать
    for (let i = 0; i < 5; i++) await new Promise((res) => setTimeout(res, 0))
    expect(r.phase).toBe('drag')
    expect(r.from).toBe(1)
    expect(b.hit(40, Y)).toEqual({ kind: 'gap', at: 0, to: 0 })
  })

  it('пока бросок приземляется, новый жест ничего не двигает', () => {
    const a = r.session(src, 0, 50, Y)
    a.hit(290, Y)
    a.drop({ kind: 'gap', at: 3, to: 2 })
    const b = r.session(src, 1, 164, Y)
    expect(b.hit(40, Y)).toBeNull()
    expect(r.from).toBe(0)
    expect(r.phase).toBe('land')
  })

  it('сетка: поднятый едет в обе стороны, соседи — по порядку чтения', () => {
    // 2 в ряд, ячейка 100×80, зазор 10: (0,0) (110,0) (0,90) (110,90)
    const cell = (k: number) =>
      ({ offsetLeft: (k % 2) * 110, offsetTop: Math.floor(k / 2) * 90, offsetWidth: 100, offsetHeight: 80 }) as HTMLElement
    const grid: ReorderSrc = {
      mode: 'grid',
      items: () => [0, 1, 2, 3].map(cell),
      origin: () => ({ x: 0, y: 0 }),
      area: () => ({ left: 0, top: 0, right: 210, bottom: 170 }),
    }
    const s = r.session(grid, 0, 50, 40)
    expect(s.hit(170, 130)).toEqual({ kind: 'gap', at: 4, to: 3 })
    expect(r.css(0)).toBe('translate(120px, 90px)')
    expect(r.css(1)).toBe('translate(-110px, 0px)')
    expect(r.css(2)).toBe('translate(110px, -90px)') // крайняя в ряду — в конец предыдущего
    expect(r.css(3)).toBe('translate(-110px, 0px)')
  })
})
