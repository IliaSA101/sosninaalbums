import { afterEach, describe, expect, it } from 'vitest'
import { dropCatalog, useCatalog } from '../../base/keys/keymap.fixture'
import { AUTO_LABEL, emptyCanvas } from './emptycanvas'

afterEach(() => dropCatalog())

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

  it('«Во весь экран»: подсказка с клавишей из реестра, главная зависит от фото', () => {
    useCatalog()
    expect(emptyCanvas(true, 0)).toEqual({ hint: ['Выйдите из режима «Во весь экран» (F), чтобы добавить фото'], primary: 'spread' })
    expect(emptyCanvas(true, 3).primary).toBe('auto')
  })

  it('своя клавиша видна в подсказке сразу; нет клавиши — без скобок', () => {
    useCatalog({ toggleBig: ['KeyB'] })
    expect(emptyCanvas(true, 0).hint).toEqual(['Выйдите из режима «Во весь экран» (B), чтобы добавить фото'])
    useCatalog({ toggleBig: [] })
    expect(emptyCanvas(true, 0).hint).toEqual(['Выйдите из режима «Во весь экран», чтобы добавить фото'])
  })
})
