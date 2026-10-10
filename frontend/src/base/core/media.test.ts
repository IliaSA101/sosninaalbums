import { describe, expect, it } from 'vitest'
import { previewUrl, thumbUrl } from './media'

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
