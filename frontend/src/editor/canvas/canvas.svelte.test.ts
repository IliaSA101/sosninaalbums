import { beforeEach, describe, expect, it } from 'vitest'
import { canvas } from './canvas.svelte'

// Окно 1000×600 px, разворот 600×300 мм, k0 = 1 px/мм.
beforeEach(() => {
  canvas.vp = { w: 1000, h: 600, k0: 1, sw: 600, ph: 300 }
  canvas.fit()
  canvas.zoomSeq = 0
})

describe('Canvas.zoomSeq / setZoom', () => {
  it('zoomSeq растёт только при смене z', () => {
    canvas.zoomAt(2, 500, 300)
    expect(canvas.zoomSeq).toBe(1)
    canvas.panBy(30, 0) // сдвиг листа — не смена масштаба
    expect(canvas.zoomSeq).toBe(1)
    canvas.zoomAt(1, 500, 300)
    expect(canvas.zoomSeq).toBe(1)
    canvas.fit()
    expect(canvas.zoomSeq).toBe(2)
    canvas.fit() // уже вписан
    expect(canvas.zoomSeq).toBe(2)
  })

  it('setZoom держит центр окна', () => {
    canvas.zoomAt(2, 500, 300)
    canvas.panBy(-40, 25)
    const { cx, cy } = canvas.cur
    canvas.setZoom(3)
    expect(canvas.cur.z).toBeCloseTo(3, 10)
    expect(canvas.cur.cx).toBeCloseTo(cx, 10)
    expect(canvas.cur.cy).toBeCloseTo(cy, 10)
  })

  it('setZoom: края шкалы', () => {
    canvas.setZoom(8)
    expect(canvas.cur.z).toBe(8)
    canvas.setZoom(20)
    expect(canvas.cur.z).toBe(8)
    canvas.setZoom(1) // левый край — вписать
    expect(canvas.cur).toEqual({ z: 1, cx: 300, cy: 150 })
  })
})
