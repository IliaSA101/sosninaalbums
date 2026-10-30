import { describe, expect, it } from 'vitest'
import type { DecorItem } from '../decor/decor'
import { neighbor, peekCaption, peekGuess, peekHint, peekSrc, thumbSrc } from './peekaddr'

// Чистая часть крупного просмотра плиток (133 U2): адреса, подпись, строка клавиш, соседи.
const mk = (over: Partial<DecorItem> = {}): DecorItem => ({
  id: 'x',
  theme: 't',
  name: 'Имя',
  url: '/media/gen/art?k=x&v=1',
  own: false,
  aspect: 0,
  w: 0,
  h: 0,
  ...over,
})
const sp = { w: 600, h: 300, mid: 300 } as never

describe('neighbor', () => {
  const ids = ['a', 'b', 'c']
  it('соседи, край и чужой id', () => {
    expect(neighbor(ids, 'b', 1)).toBe('c')
    expect(neighbor(ids, 'b', -1)).toBe('a')
    expect(neighbor(ids, 'a', -1)).toBe('a')
    expect(neighbor(ids, 'c', 1)).toBe('c')
    expect(neighbor(ids, 'zz', 1)).toBe('zz')
    expect(neighbor([], 'a', 1)).toBe('a')
  })
})

describe('peekSrc / thumbSrc', () => {
  it('peek из Go — как есть, без px', () => {
    expect(peekSrc(mk({ peek: '/media/lib/p/aa?v=1' }), 'sticker', sp, 'spread', 1920, 2)).toBe('/media/lib/p/aa?v=1')
  })
  it('встроенное — px кратен 64 и ≤ 4096', () => {
    expect(peekSrc(mk(), 'sticker', sp, 'spread', 1000, 1)).toBe('/media/gen/art?k=x&v=1&px=1024')
    expect(peekSrc(mk(), 'sticker', sp, 'spread', 3840, 2)).toBe('/media/gen/art?k=x&v=1&px=4096')
  })
  it('разворот — /media/tpl с px ≤ 1024 и r', () => {
    expect(peekSrc(mk({ url: '/media/tpl/t1?v=1', pages: 2 }), 'spread', sp, 'spread', 3840, 2)).toBe('/media/tpl/t1?v=1&px=1024&r=2')
  })
  it('миниатюра: /media/lib без px, встроенное с px плитки', () => {
    expect(thumbSrc(mk({ url: '/media/lib/t/aa?v=1' }), 'sticker', sp, 'spread', 120, 2)).toBe('/media/lib/t/aa?v=1')
    expect(thumbSrc(mk(), 'sticker', sp, 'spread', 120, 2)).toBe('/media/gen/art?k=x&v=1&px=256')
  })
})

describe('peekGuess / peekCaption / peekHint', () => {
  it('пропорции до загрузки', () => {
    expect(peekGuess(mk(), 'bg', sp, 'spread')).toBe(2)
    expect(peekGuess(mk({ w: 900, h: 300 }), 'bg', sp, 'spread')).toBe(3) // свой фон — по его размеру
    expect(peekGuess(mk({ w: 800, h: 400 }), 'sticker', sp, 'spread')).toBe(2)
    expect(peekGuess(mk({ aspect: 0.5 }), 'sticker', sp, 'spread')).toBe(0.5)
    expect(peekGuess(mk(), 'sticker', sp, 'spread')).toBe(1)
  })
  it('подпись: свой, набор, встроенное, страница', () => {
    expect(peekCaption(mk({ own: true }), 2, 5)).toBe('Имя · свой · 2 из 5')
    expect(peekCaption(mk({ pack: 'Школа' }), 1, 3)).toBe('Имя · набор «Школа» · 1 из 3')
    expect(peekCaption(mk(), 1, 1)).toBe('Имя · 1 из 1')
    expect(peekCaption(mk({ pages: 1, pack: 'Школа' }), 1, 2)).toBe('Имя · страница · набор «Школа» · 1 из 2')
  })
  it('строка клавиш: без зума и в режиме замены', () => {
    expect(peekHint('Ctrl', false, false)).toBe('← → — соседние · Enter — поставить · Esc — назад')
    expect(peekHint('⌘', true, true)).toContain('⌘+колесо или щипок — зум')
    expect(peekHint('⌘', true, true)).toContain('Enter — заменить')
  })
})
