import { describe, expect, it } from 'vitest'
import { folderCounts } from './folders'

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
