import { describe, expect, it } from 'vitest'
import cases from '../../../../testdata/parity/autopick.json'
import { UsedIndex } from '../../base/core/usedindex'
import type { MarkView, Photo, SpreadView } from '../../base/core/types'
import { defaultSpreads } from '../../shell/autobuild/autobuild'
import { autoPick } from './freephotos'

type PickCase = {
  name: string
  photos: { id: string; xmp?: Photo['xmp'] }[]
  own: Record<string, MarkView>
  placed: string[]
  backdrop: string[]
  want: string[]
  skipped: number
}
type SpreadsCase = { photos: number; range: [number, number] | null; want: number }
const file = cases as { pick: PickCase[]; spreads: SpreadsCase[] }

// «В альбоме» считает тот же UsedIndex, что и окно: запись места и файл копии («a-2» занимает «a»).
const usedOf = (placed: string[]): Set<string> => new UsedIndex().update([{ photos: placed } as unknown as SpreadView])

// Общие примеры с Go (TestAutobuildFree): выбор фото и число разворотов окна.
describe('автосборка по testdata/parity/autopick.json', () => {
  for (const c of file.pick) {
    it(`выбор фото: ${c.name}`, () => {
      const photos = c.photos.map((p): Photo => ({ id: p.id, path: p.id, name: p.id, w: 100, h: 100, xmp: p.xmp }))
      const r = autoPick(photos, [], usedOf(c.placed), new Set(c.backdrop), c.own, true)
      expect(r.ids).toEqual(c.want)
      expect(r.skipped).toBe(c.skipped)
    })
  }
  for (const c of file.spreads) {
    it(`разворотов: ${c.photos} фото, диапазон ${c.range ? c.range.join('–') : 'авто'}`, () => {
      expect(defaultSpreads(c.photos, c.range)).toBe(c.want)
    })
  }
})
