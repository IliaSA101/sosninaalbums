import { afterEach, describe, expect, it } from 'vitest'
import { folderKey, hasSources, inFeedText } from './folders'
import { setOsForTest } from '../../base/core/platform'
import { folderfilter } from './folderfilter.svelte'
import { buildTree, filterTree, findNode, nodeKey, type TreeNode } from './tree'

// Случаи folderCounts / filterFolders / cachedFilterFolders (054) перенесены в tree.test.ts (093).
const ph = (...paths: string[]) => paths.map((path) => ({ path }))
const PHOTOS = ph('D:/a/1.jpg', 'D:/b/2.jpg', 'D:/b/x/3.jpg', 'D:/c/4.jpg')
const forest = buildTree(['D:/a', 'D:/b', 'D:/c'], PHOTOS)
const at = (dir: string) => findNode(forest, nodeKey(dir)) as TreeNode

describe('folderKey', () => {
  afterEach(() => setOsForTest(null))

  it('регистр, разделители и хвостовой «/» не важны', () => {
    expect(folderKey('D:\\Банкет\\')).toBe('d:/банкет')
    expect(folderKey('d:/банкет')).toBe('d:/банкет')
  })

  it('Linux различает регистр: две папки — две строки; Windows и macOS сливают в одну', () => {
    const roots = ['/x/Весна', '/x/ВЕСНА']
    const photos = () => ph('/x/Весна/1.jpg', '/x/ВЕСНА/2.jpg') // ключ папки кэшируется на объекте фото — под каждую ОС свежие
    setOsForTest('linux')
    expect(folderKey('/x/Весна')).not.toBe(folderKey('/x/ВЕСНА'))
    const lin = buildTree(roots, photos())
    expect(lin.map((r) => [r.name, r.total])).toEqual([
      ['Весна', 1],
      ['ВЕСНА', 1],
    ])
    for (const os of ['windows', 'mac'] as const) {
      setOsForTest(os)
      expect(folderKey('/x/Весна')).toBe(folderKey('/x/ВЕСНА'))
      const t = buildTree(roots, photos())
      expect(t).toHaveLength(1)
      expect(t[0].total).toBe(2)
    }
  })
})

describe('folderfilter', () => {
  afterEach(() => folderfilter.reset())

  it('toggle скрывает папку с подпапками и возвращает', () => {
    folderfilter.toggle(at('D:\\B\\'))
    expect(folderfilter.active).toBe(true)
    expect(filterTree(PHOTOS, folderfilter.hidden).map((p) => p.path)).toEqual(['D:/a/1.jpg', 'D:/c/4.jpg'])
    folderfilter.toggle(at('d:/b'))
    expect(folderfilter.active).toBe(false)
  })

  it('only и showAll', () => {
    folderfilter.only(forest, at('D:/b/x'))
    expect(filterTree(PHOTOS, folderfilter.hidden).map((p) => p.path)).toEqual(['D:/b/x/3.jpg'])
    folderfilter.showAll()
    expect(folderfilter.active).toBe(false)
  })

  it('prune: убранная папка забыта, остальные скрыты по-прежнему', () => {
    folderfilter.toggle(at('D:/a'))
    folderfilter.toggle(at('D:/b'))
    folderfilter.prune(['D:/b', 'D:/c', 'D:/d'])
    expect([...folderfilter.hidden].sort()).toEqual([nodeKey('D:/b'), nodeKey('D:/b/x')])
  })

  it('prune: осталась одна папка — скрытая подпапка остаётся (093: фильтр и при одном корне)', () => {
    folderfilter.toggle(at('D:/b/x'))
    folderfilter.prune(['D:/b'])
    expect(folderfilter.active).toBe(true)
  })

  it('allHidden — по ленте: фото есть, показанных нет', () => {
    expect(folderfilter.allHidden(0, 4)).toBe(false) // без фильтра — не «скрыты»
    folderfilter.toggle(at('D:/a'))
    expect(folderfilter.allHidden(3, 4)).toBe(false)
    expect(folderfilter.allHidden(0, 4)).toBe(true)
    expect(folderfilter.allHidden(0, 0)).toBe(false)
  })

  it('раскрытие: добавленная папка раскрыта, подпапка свёрнута; ▸ переключает; reset — как было', () => {
    const b = at('D:/b')
    const x = at('D:/b/x')
    expect([folderfilter.isOpen(b), folderfilter.isOpen(x)]).toEqual([true, false])
    folderfilter.toggleOpen(b)
    folderfilter.expand([x])
    expect([folderfilter.isOpen(b), folderfilter.isOpen(x)]).toEqual([false, true])
    folderfilter.expand([b])
    expect(folderfilter.isOpen(b)).toBe(true)
    folderfilter.prune(['D:/a']) // папку b убрали — её раскрытие забыто
    expect(folderfilter.flip.size).toBe(0)
    folderfilter.toggleOpen(b)
    folderfilter.reset()
    expect(folderfilter.isOpen(b)).toBe(true)
  })
})

describe('inFeedText', () => {
  it('тот же вид при скрытых и нескрытых папках', () => {
    expect(inFeedText(162, 162)).toBe('В ленте 162 из 162 фото')
    expect(inFeedText(0, 162)).toBe('В ленте 0 из 162 фото')
  })
})

describe('hasSources (097)', () => {
  it('папки или отдельные фото — есть откуда брать; ни того ни другого — пустая лента зовёт добавить', () => {
    expect(hasSources({ folders: [], loose: [] })).toBe(false)
    expect(hasSources({ folders: [] })).toBe(false)
    expect(hasSources({ folders: ['D:/a'], loose: [] })).toBe(true)
    expect(hasSources({ folders: [], loose: [{}] })).toBe(true)
  })
})
