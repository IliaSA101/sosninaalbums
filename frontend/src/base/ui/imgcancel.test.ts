import { describe, expect, it } from 'vitest'
import { cancelOnUnmount } from './imgcancel'

// DOM в тестах нет (jsdom не подключён): подставной элемент с тем же removeAttribute.
function fakeImg(src: string) {
  const attrs = new Map<string, string>([['src', src]])
  return {
    img: { removeAttribute: (n: string) => void attrs.delete(n) } as unknown as HTMLImageElement,
    has: (n: string) => attrs.has(n),
  }
}

describe('cancelOnUnmount', () => {
  it('пока плитка стоит — src на месте; при снятии у сохранённого элемента src нет', () => {
    const { img, has } = fakeImg('/media/thumb/a')
    const off = cancelOnUnmount(img)
    expect(has('src')).toBe(true)
    off()
    expect(has('src')).toBe(false)
  })
  it('повторное снятие не бросает', () => {
    const { img } = fakeImg('/media/thumb/a')
    const off = cancelOnUnmount(img)
    off()
    expect(() => off()).not.toThrow()
  })
})

// Подключение: каждый <img> с thumbUrl несёт {@attach cancelOnUnmount}.
const files = import.meta.glob(
  ['/src/panels/library/Thumb.svelte', '/src/panels/timeline/SpreadChips.svelte', '/src/editor/photo/Cell.svelte'],
  { query: '?raw', import: 'default', eager: true },
) as Record<string, string>

describe('подключение к <img> миниатюр', () => {
  const imgs = (s: string) => (s.replace(/<!--[\s\S]*?-->/g, '').match(/<img\b[\s\S]*?\/>/g) ?? []).filter((t) => t.includes('thumbUrl('))
  const cases: [string, number][] = [
    ['/src/panels/library/Thumb.svelte', 1],
    ['/src/panels/timeline/SpreadChips.svelte', 2],
    ['/src/editor/photo/Cell.svelte', 1],
  ]
  for (const [path, n] of cases) {
    it(`${path.split('/').pop()}: ${n} шт.`, () => {
      const tags = imgs(files[path] ?? '')
      expect(tags).toHaveLength(n)
      for (const t of tags) expect(t).toContain('{@attach cancelOnUnmount}')
    })
  }
})

// То же для превью: снятый <img> с previewUrl не должен слать запрос в очередь после смены реестра (089N).
const previewFiles = import.meta.glob(
  [
    '/src/editor/photo/Cell.svelte',
    '/src/editor/photo/PhotoPreview.svelte',
    '/src/editor/backdrop/BackdropLayer.svelte',
    '/src/editor/sticker/StickerLayer.svelte',
  ],
  { query: '?raw', import: 'default', eager: true },
) as Record<string, string>

describe('подключение к <img> превью', () => {
  const imgs = (s: string) => (s.replace(/<!--[\s\S]*?-->/g, '').match(/<img\b[\s\S]*?\/>/g) ?? []).filter((t) => t.includes('previewUrl('))
  const cases: [string, number][] = [
    ['/src/editor/photo/Cell.svelte', 1],
    ['/src/editor/photo/PhotoPreview.svelte', 2],
    ['/src/editor/backdrop/BackdropLayer.svelte', 1],
    ['/src/editor/sticker/StickerLayer.svelte', 1],
  ]
  for (const [path, n] of cases) {
    it(`${path.split('/').pop()}: ${n} шт.`, () => {
      const tags = imgs(previewFiles[path] ?? '')
      expect(tags).toHaveLength(n)
      for (const t of tags) expect(t).toContain('{@attach cancelOnUnmount}')
    })
  }
})
