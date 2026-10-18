import { describe, expect, it } from 'vitest'
import { coverGuideUrl, spreadGuideUrl, previewUrl, thumbUrl, warmable, wantPreview } from './media'

describe('media', () => {
  it('обычный id', () => {
    expect(thumbUrl('ab12')).toBe('/media/t/ab12?v=3')
    expect(previewUrl('ab12')).toBe('/media/p/ab12?v=3')
  })
  it('id экранируется: ни путь, ни запрос не подменяются', () => {
    expect(thumbUrl('a/../b?x')).toBe('/media/t/a%2F..%2Fb%3Fx?v=3')
    expect(previewUrl('a b#c')).toBe('/media/p/a%20b%23c?v=3')
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
