import { describe, expect, it } from 'vitest'
import { photoMenu, type MenuCtx } from './photomenu'

// 131: у надписи-заготовки в меню правого щелчка — «Сделать обычной надписью»; уровень без буфера его не прячет (вопрос 20).
const ctx = (o: Partial<MenuCtx> = {}): MenuCtx =>
  ({ grid: true, places: 0, texts: 1, stickers: 0, photos: 0, missing: 0, copyable: true, pasteable: false, ...o }) as MenuCtx
const ids = (c: MenuCtx, can: (f: never) => boolean = () => true) => photoMenu(c, can as never).map((i) => i.id)

describe('меню надписи: заготовка', () => {
  it('заготовка — первый пункт, дальше прежнее меню надписи', () => {
    const m = photoMenu(ctx({ draft: true }))
    expect(m[0]).toMatchObject({ id: 'undraft', label: 'Сделать обычной надписью' })
    expect(m.map((i) => i.id)).toContain('delete')
    expect(m.map((i) => i.id)).toContain('clipCopy')
  })
  it('обычная надпись — без пункта', () => {
    expect(ids(ctx())).not.toContain('undraft')
    expect(ids(ctx({ draft: false }))).not.toContain('undraft')
  })
  it('уровень без буфера: у заготовки меню из одного пункта, у обычной надписи меню нет', () => {
    const noClip = (f: string) => f !== 'clipboard'
    expect(ids(ctx({ draft: true }), noClip)).toEqual(['undraft'])
    expect(ids(ctx(), noClip)).toEqual([])
  })
  it('пункт не имеет клавиши и не вырастает в подменю', () => {
    const it = photoMenu(ctx({ draft: true }))[0]
    expect(it.hint).toBeUndefined()
    expect(it.sub).toBeUndefined()
  })
})
