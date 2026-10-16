import { afterEach, describe, expect, it } from 'vitest'
import { cachedFilterFolders, folderCounts, filterFolders } from './folders'
import { folderfilter } from './folderfilter.svelte'

const ph = (...paths: string[]) => paths.map((path) => ({ path }))

describe('folderCounts', () => {
  it('фото вложенной папки достаётся первой папке списка', () => {
    const photos = ph('D:\\a\\sub\\1.jpg', 'D:\\a\\2.jpg')
    expect(folderCounts(['D:\\a\\sub', 'D:\\a'], photos)).toEqual([1, 1])
    expect(folderCounts(['D:\\a', 'D:\\a\\sub'], photos)).toEqual([2, 0])
  })

  it('регистр и разделители не важны', () => {
    expect(folderCounts(['d:/Свадьба/'], ph('D:\\свадьба\\x\\1.jpg'))).toEqual([1])
  })

  it('D:/a не захватывает D:/ab', () => {
    expect(folderCounts(['D:/a', 'D:/ab'], ph('D:/ab/1.jpg', 'D:/a/2.jpg'))).toEqual([1, 1])
  })

  it('пустые и недоступные папки — 0', () => {
    expect(folderCounts(['D:/a', 'Z:/gone'], ph('D:/a/1.jpg'))).toEqual([1, 0])
    expect(folderCounts([], ph('D:/a/1.jpg'))).toEqual([])
    expect(folderCounts(['D:/a'], [])).toEqual([0])
  })

  it('корень диска и повтор папки', () => {
    expect(folderCounts(['D:\\'], ph('D:\\1.jpg'))).toEqual([1])
    expect(folderCounts(['D:/a', 'd:\\A'], ph('D:/a/1.jpg'))).toEqual([1, 0])
  })
})

describe('filterFolders', () => {
  const photos = ph('D:/a/1.jpg', 'D:/b/2.jpg', 'D:/c/3.jpg', 'D:/b/x/4.jpg')
  const keys = (h: string[]) => new Set(h)

  it('скрытая папка уходит вместе с вложенными, остальные на месте', () => {
    expect(filterFolders(['D:/a', 'D:/b', 'D:/c'], photos, keys(['d:/b'])).map((p) => p.path)).toEqual(['D:/a/1.jpg', 'D:/c/3.jpg'])
  })

  it('inactive same array: ничего не скрыто — тот же массив', () => {
    expect(filterFolders(['D:/a', 'D:/b'], photos, new Set())).toBe(photos)
  })

  it('nested: вложенная папка — как folderCounts, фото у первой подходящей', () => {
    const folders = ['D:/b/x', 'D:/b']
    // 4.jpg принадлежит первой в списке (b/x) — скрыть вторую (b) его не трогает
    expect(filterFolders(folders, photos, keys(['d:/b'])).map((p) => p.path)).toEqual(['D:/a/1.jpg', 'D:/c/3.jpg', 'D:/b/x/4.jpg'])
    expect(folderCounts(folders, photos)).toEqual([1, 1])
    expect(filterFolders(folders, photos, keys(['d:/b/x'])).map((p) => p.path)).toEqual(['D:/a/1.jpg', 'D:/b/2.jpg', 'D:/c/3.jpg'])
  })

  it('регистр и разделители не важны; фото вне папок остаётся; все скрыты — пусто', () => {
    expect(filterFolders(['d:\\A\\'], ph('D:/a/1.jpg'), keys(['d:/a']))).toEqual([])
    expect(filterFolders(['D:/a'], ph('E:/z/1.jpg'), keys(['d:/a']))).toHaveLength(1)
  })
})

describe('folderfilter', () => {
  afterEach(() => folderfilter.reset())

  it('toggle скрывает и возвращает по ключу пути', () => {
    folderfilter.toggle('D:\\Банкет\\')
    expect(folderfilter.active).toBe(true)
    expect(folderfilter.isHidden('d:/банкет')).toBe(true)
    folderfilter.toggle('d:/Банкет')
    expect(folderfilter.active).toBe(false)
  })

  it('prune: убранная папка забыта, остальные скрыты по-прежнему', () => {
    folderfilter.toggle('D:/a')
    folderfilter.toggle('D:/b')
    folderfilter.prune(['D:/b', 'D:/c', 'D:/d'])
    expect(folderfilter.isHidden('D:/a')).toBe(false)
    expect(folderfilter.isHidden('D:/b')).toBe(true)
  })

  it('prune: осталась одна папка — пусто', () => {
    folderfilter.toggle('D:/a')
    folderfilter.prune(['D:/a'])
    expect(folderfilter.active).toBe(false)
  })

  it('hiddenIn и allHidden', () => {
    const f = ['D:/a', 'D:/b']
    folderfilter.toggle('D:/a')
    expect(folderfilter.hiddenIn(f)).toBe(1)
    expect(folderfilter.allHidden(f)).toBe(false)
    folderfilter.toggle('D:/b')
    expect(folderfilter.allHidden(f)).toBe(true)
    expect(folderfilter.allHidden([])).toBe(false)
  })

  it('reset и showAll — все показаны', () => {
    folderfilter.toggle('D:/a')
    folderfilter.reset()
    expect(folderfilter.active).toBe(false)
    folderfilter.toggle('D:/a')
    folderfilter.showAll()
    expect(folderfilter.active).toBe(false)
  })
})

describe('cachedFilterFolders', () => {
  const photos = ph('D:/a/1.jpg', 'D:/b/2.jpg')
  it('тот же состав — тот же массив; смена фото, папок или скрытых — новый', () => {
    const f = cachedFilterFolders()
    const hidden = new Set(['d:/b'])
    const first = f(['D:/a', 'D:/b'], photos, hidden)
    expect(first).toEqual([photos[0]])
    expect(f(['D:/a', 'D:/b'], photos, new Set(['d:/b']))).toBe(first)
    expect(f(['D:/a', 'D:/b'], [...photos], hidden)).not.toBe(first)
    const second = f(['D:/a', 'D:/b'], photos, new Set(['d:/a']))
    expect(second).toEqual([photos[1]])
    expect(f(['D:/a'], photos, new Set(['d:/a']))).not.toBe(second)
  })

  it('ничего не скрыто — исходный массив', () => {
    expect(cachedFilterFolders()(['D:/a'], photos, new Set())).toBe(photos)
  })
})
