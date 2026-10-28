import { describe, expect, it } from 'vitest'
import { guideVideoUrl, lookUrl, originalUrl, coverGuideUrl, spreadGuideUrl, previewUrl, thumbUrl, warmable, wantPreview } from './media'

describe('media', () => {
  it('обычный id', () => {
    expect(thumbUrl('ab12')).toBe('/media/t/ab12?v=5')
    expect(previewUrl('ab12')).toBe('/media/p/ab12?v=5')
  })
  it('повторная загрузка миниатюры (088): метка r сбивает кэш, 0 — адрес прежний', () => {
    expect(thumbUrl('ab12', 0)).toBe('/media/t/ab12?v=5')
    expect(thumbUrl('ab12', 2)).toBe('/media/t/ab12?v=5&r=2')
  })
  it('id экранируется: ни путь, ни запрос не подменяются', () => {
    expect(thumbUrl('a/../b?x')).toBe('/media/t/a%2F..%2Fb%3Fx?v=5')
    expect(previewUrl('a b#c')).toBe('/media/p/a%20b%23c?v=5')
    expect(originalUrl('a/../b?x')).toBe('/media/o/a%2F..%2Fb%3Fx?v=5')
  })
  it('оригинал (128): адрес /media/o/<id>', () => {
    expect(originalUrl('ab12')).toBe('/media/o/ab12?v=5')
  })
})

describe('разметка обложки', () => {
  it('в адресе только номер правки', () => {
    expect(coverGuideUrl(7)).toBe('/media/cover-guide?r=7')
    expect(coverGuideUrl(NaN)).toBe('/media/cover-guide?r=0')
    expect(spreadGuideUrl(7)).toBe('/media/spread-guide?r=7')
    expect(spreadGuideUrl(NaN)).toBe('/media/spread-guide?r=0')
  })
})

describe('превью при многих фото', () => {
  it('до 16 фото — превью всегда', () => {
    expect(wantPreview(16, 10, 1)).toBe(true)
    expect(warmable(16)).toBe(true)
  })
  it('17+ фото: только если миниатюры 400 px не хватает', () => {
    expect(wantPreview(17, 0, 1)).toBe(false) // ещё не измерена
    expect(wantPreview(40, 400, 1)).toBe(false)
    expect(wantPreview(40, 401, 1)).toBe(true)
    expect(wantPreview(40, 250, 2)).toBe(true) // 500 физических px
    expect(wantPreview(40, 150, 2)).toBe(false)
  })
  it('соседей с 17+ фото не прогреваем', () => {
    expect(warmable(17)).toBe(false)
    expect(warmable(64)).toBe(false)
  })
})

describe('макеты альбома (131)', () => {
  it('lookUrl: ключ, шаг, ширина, пропорция и фон', () => {
    expect(lookUrl('0123456789abcdef', { i: 0, px: 256, r: 1 })).toBe('/media/look/0123456789abcdef?v=1&i=0&px=256&r=1&bg=0')
    expect(lookUrl('0123456789abcdef', { i: 3, px: 512, r: 1.3333333, bg: true })).toBe(
      '/media/look/0123456789abcdef?v=1&i=3&px=512&r=1.3333&bg=1',
    )
    expect(lookUrl('0123456789abcdef', { i: 0, px: 256, r: 1, front: true })).toBe(
      '/media/look/0123456789abcdef?v=1&i=0&px=256&r=1&bg=0&f=1',
    )
  })
  it('lookUrl: не ключ (не 16 строчных hex) — адреса нет; мусор в числах не попадает в адрес', () => {
    for (const bad of ['', 'a/../b?x', '0123456789ABCDEF', '0123456789abcde', '0123456789abcdef0', 'my:0123456789abcdef'])
      expect(lookUrl(bad, { i: 1, px: 256, r: 1 }), bad).toBe('')
    expect(lookUrl('0123456789abcdef', { i: NaN, px: NaN, r: NaN })).toBe('/media/look/0123456789abcdef?v=1&i=0&px=0&r=1&bg=0')
  })
})

describe('ролик справочника (140)', () => {
  it('адрес — /media/guide-video/<id> без ?v=, id кодируется', () => {
    expect(guideVideoUrl('autobuild')).toBe('/media/guide-video/autobuild')
    expect(guideVideoUrl('a/b?c')).toBe('/media/guide-video/a%2Fb%3Fc')
  })
})
