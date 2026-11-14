import { describe, expect, it } from 'vitest'
import { colorWords, eyeHint } from './colorwords'

describe('colorWords: лист / разворот / обложка', () => {
  it('альбом разворотов', () => {
    expect(colorWords(undefined, 's1')).toEqual({ by: 'по развороту', onCap: 'На развороте' })
    expect(eyeHint(colorWords(undefined, 's1'))).toBe('Щёлкните по развороту — возьмём цвет, каким он выйдет в печати')
  })
  it('альбом листов', () => {
    const st = { sheet: { w: 204, h: 304 } } as never
    expect(colorWords(st, 's1')).toEqual({ by: 'по листу', onCap: 'На листе' })
  })
  it('обложка — своё слово, листы не важны; без листа — как у разворота', () => {
    expect(colorWords({ sheet: { w: 204, h: 304 } } as never, 'cover').by).toBe('по обложке')
    expect(colorWords(undefined, undefined).by).toBe('по развороту')
  })
})
