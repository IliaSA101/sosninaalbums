// @vitest-environment happy-dom
import { afterEach, beforeEach, describe, expect, it, vi, type Mock } from 'vitest'
import { pinchOn, pinching, type PinchSink } from './pinch'

// happy-dom не знает GestureEvent: событие собираем руками, поля — как у WebKit.
const gesture = (type: string, init: { scale?: number; clientX?: number; clientY?: number } = {}) => {
  const e = new Event(type, { bubbles: true, cancelable: true })
  for (const [k, v] of Object.entries(init)) Object.defineProperty(e, k, { value: v })
  return e
}

let node: HTMLElement
let child: HTMLElement
let sink: Mock<PinchSink>
let off: () => void

beforeEach(() => {
  node = document.createElement('div')
  child = document.createElement('span')
  node.append(child)
  document.body.append(node)
  sink = vi.fn<PinchSink>()
  node.getBoundingClientRect = () => ({
    left: 100,
    top: 50,
    width: 200,
    height: 100,
    right: 300,
    bottom: 150,
    x: 100,
    y: 50,
    toJSON: () => ({}),
  })
  off = pinchOn(node, sink)
})
afterEach(() => {
  off()
  node.remove()
})

describe('pinchOn (134-2)', () => {
  it('все три события погашены; приёмник получает отношение и clientX/Y', () => {
    const start = gesture('gesturestart', { scale: 1, clientX: 140, clientY: 90 })
    child.dispatchEvent(start)
    const change = gesture('gesturechange', { scale: 1.5, clientX: 141, clientY: 91 })
    child.dispatchEvent(change)
    const end = gesture('gestureend', { scale: 2, clientX: 142, clientY: 92 })
    child.dispatchEvent(end)
    expect([start, change, end].map((e) => e.defaultPrevented)).toEqual([true, true, true])
    expect(sink).toHaveBeenNthCalledWith(1, 1.5, 141, 91)
    expect(sink).toHaveBeenNthCalledWith(2, 2 / 1.5, 142, 92)
    expect(sink).toHaveBeenCalledTimes(2)
  })

  it('skip: без вызова, но жест погашен', () => {
    off()
    off = pinchOn(node, sink, { skip: (t) => t === child })
    const ev = [gesture('gesturestart', { scale: 1 }), gesture('gesturechange', { scale: 2 }), gesture('gestureend', { scale: 2 })]
    for (const e of ev) child.dispatchEvent(e)
    expect(ev.map((e) => e.defaultPrevented)).toEqual([true, true, true])
    expect(sink).not.toHaveBeenCalled()
    expect(pinching()).toBe(false)
    // над самим узлом (не skip) — работает
    node.dispatchEvent(gesture('gesturestart', { scale: 1 }))
    node.dispatchEvent(gesture('gesturechange', { scale: 2 }))
    expect(sink).toHaveBeenCalledTimes(1)
  })

  it('skip решается на gesturestart: жест, начатый вне skip, идёт и над skip; начатый над skip — не идёт', () => {
    off()
    off = pinchOn(node, sink, { skip: (t) => t === child })
    node.dispatchEvent(gesture('gesturestart', { scale: 1 }))
    child.dispatchEvent(gesture('gesturechange', { scale: 2 }))
    expect(sink).toHaveBeenCalledTimes(1)
    child.dispatchEvent(gesture('gestureend', { scale: 2 })) // конец жеста сбрасывает состояние, даже если цель другая
    expect(pinching()).toBe(false)
    sink.mockClear()
    child.dispatchEvent(gesture('gesturestart', { scale: 1 }))
    node.dispatchEvent(gesture('gesturechange', { scale: 2 }))
    node.dispatchEvent(gesture('gestureend', { scale: 2 }))
    expect(sink).not.toHaveBeenCalled()
    expect(pinching()).toBe(false)
  })

  it('поворот (множитель 1) приёмнику не отдаётся', () => {
    node.dispatchEvent(gesture('gesturestart', { scale: 1 }))
    node.dispatchEvent(gesture('gesturechange', { scale: 1, clientX: 1, clientY: 1 }))
    node.dispatchEvent(gesture('gesturechange', { scale: 1.00001, clientX: 1, clientY: 1 }))
    expect(sink).not.toHaveBeenCalled()
  })

  it('без clientX/Y — середина узла', () => {
    node.dispatchEvent(gesture('gesturestart', { scale: 1 }))
    node.dispatchEvent(gesture('gesturechange', { scale: 2 }))
    expect(sink).toHaveBeenCalledWith(2, 200, 100)
  })

  it('pinching() true посреди жеста и false после gestureend', () => {
    node.dispatchEvent(gesture('gesturestart', { scale: 1 }))
    expect(pinching()).toBe(true)
    node.dispatchEvent(gesture('gestureend', { scale: 1 }))
    expect(pinching()).toBe(false)
  })

  it('после снятия событий нет, незаконченный жест сброшен', () => {
    node.dispatchEvent(gesture('gesturestart', { scale: 1 }))
    off()
    expect(pinching()).toBe(false)
    const e = gesture('gesturechange', { scale: 2 })
    node.dispatchEvent(e)
    expect(e.defaultPrevented).toBe(false)
    expect(sink).not.toHaveBeenCalled()
  })

  it('повторный gestureend — без второго вызова', () => {
    node.dispatchEvent(gesture('gesturestart', { scale: 1 }))
    node.dispatchEvent(gesture('gestureend', { scale: 2 }))
    node.dispatchEvent(gesture('gestureend', { scale: 2 }))
    expect(sink).toHaveBeenCalledTimes(1)
  })
})
