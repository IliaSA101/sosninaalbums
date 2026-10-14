import { describe, expect, it } from 'vitest'
import { AUTO_LABEL, emptyCanvas } from './emptycanvas'

describe('пустой холст', () => {
  it('с фото главная кнопка — «Автосборка…»', () => {
    const c = emptyCanvas(false, 12)
    expect(c.primary).toBe('auto')
    expect(AUTO_LABEL).toBe('Автосборка…')
    expect(c.hint.join(' ')).toContain('Выделите фото')
  })

  it('без фото — прежний текст и «пустой разворот» главным', () => {
    const c = emptyCanvas(false, 0)
    expect(c.primary).toBe('spread')
    expect(c.hint).toEqual(['Сначала добавьте папку с фотографиями (кнопка внизу)', 'или перетащите папку в окно'])
  })

  it('крупный просмотр: подсказка про F, главная зависит от фото', () => {
    expect(emptyCanvas(true, 0)).toEqual({ hint: ['Выключите крупный просмотр (F), чтобы добавить фото'], primary: 'spread' })
    expect(emptyCanvas(true, 3).primary).toBe('auto')
  })
})
