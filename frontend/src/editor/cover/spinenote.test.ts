import { describe, expect, it } from 'vitest'
import { SpineNote } from './spinenote.svelte'

describe('отметка «Корешок X → Y»', () => {
  it('первый вид только запоминает корешок', () => {
    const n = new SpineNote()
    n.see(12, false)
    expect(n.note).toBeNull()
  })

  it('корешок изменился — отметка с прежним и новым значением', () => {
    const n = new SpineNote()
    n.see(12, false)
    n.see(14, false)
    expect(n.note).toEqual({ from: 12, to: 14 })
    expect(n.text).toBe('Корешок 12 → 14 мм')
  })

  it('тот же корешок отметку не трогает', () => {
    const n = new SpineNote()
    n.see(12, false)
    n.see(14, false)
    n.see(14, false)
    expect(n.note).toEqual({ from: 12, to: 14 })
  })

  it('две правки подряд складываются', () => {
    const n = new SpineNote()
    n.see(12, false)
    n.see(14, false)
    n.see(16, false)
    expect(n.note).toEqual({ from: 12, to: 16 })
  })

  it('вернулись к прежнему значению — отметка гаснет сама', () => {
    const n = new SpineNote()
    n.see(12, false)
    n.see(14, false)
    n.see(12, false)
    expect(n.note).toBeNull()
  })

  it('обложку открыли (dismiss) или щёлкнули по отметке — отметка гаснет; дальше сравнение от нового значения', () => {
    const n = new SpineNote()
    n.see(12, false)
    n.see(14, false)
    n.dismiss()
    expect(n.note).toBeNull()
    n.see(14, false)
    expect(n.note).toBeNull()
    n.see(16, false)
    expect(n.note).toEqual({ from: 14, to: 16 })
  })

  it('пока обложка открыта, отметка не появляется, но корешок запоминается', () => {
    const n = new SpineNote()
    n.see(12, true)
    n.see(14, true)
    expect(n.note).toBeNull()
    n.see(16, false)
    expect(n.note).toEqual({ from: 14, to: 16 })
  })

  it('обложки нет или другой альбом — память сброшена', () => {
    const n = new SpineNote()
    n.see(12, false)
    n.see(14, false)
    n.see(null, false)
    expect(n.note).toBeNull()
    n.see(20, false) // новая обложка: сравнивать не с чем
    expect(n.note).toBeNull()
    n.reset()
    n.see(22, false)
    expect(n.note).toBeNull()
  })
})
