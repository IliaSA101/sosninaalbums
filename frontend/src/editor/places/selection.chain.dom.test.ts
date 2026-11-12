// @vitest-environment happy-dom
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { cleanup, render } from '@testing-library/svelte'
import { flushSync, tick } from 'svelte'
import { chainSeeds, chainSteps, chainTimeout, runChain, type Chain } from '../../base/core/randchain'

// Цепочка состояний выделения (К3): placesel (места, фото, тексты группы, стикеры) + edit.sel (одиночный текст) +
// spreadsel (группы в ленте) + store.selectedCell. Случайные щелчки, Shift/Ctrl, рамка, Ctrl+A, правки вида и переходы
// между листами; после каждого шага — инварианты. Провал печатает семя и цепочку (SA_CHAIN_SEED=<семя> повторит).
vi.mock('../../../wailsjs/runtime/runtime', () => ({ EventsOn: () => () => {} }))
vi.mock('../../../wailsjs/go/app/App', () => ({}))
vi.mock('./fonts', () => ({ loadFonts: async () => {} }))
vi.mock('../text/fonts', () => ({ loadFonts: async () => {}, styleSupport: () => true }))

const { store } = await import('../../base/core/store.svelte')
const { wire } = await import('../../shell/boot/wire')
const { placesel } = await import('./placesel.svelte')
const { edit } = await import('../text/edit.svelte')
const { spreadsel } = await import('../../panels/timeline/spreadsel.svelte')
const { default: TextLayer } = await import('../text/TextLayer.svelte')

wire()

// Сцена: что лежит на каждом листе. Правка вида — удаление и возврат объектов.
interface Sheet {
  id: string
  places: { id: string; photo: string }[]
  texts: string[]
  stickers: string[]
}
interface Model {
  scene: Sheet[]
  rev: number
  gone: string[] // удалённые объекты, чтобы вернуть
}

const freshScene = (): Sheet[] =>
  ['s1', 's2', 's3'].map((id, k) => ({
    id,
    places: [1, 2, 3, 4].map((i) => ({ id: `${id}p${i}`, photo: i % 2 === 0 ? '' : `${id}f${i + k}` })),
    texts: [`${id}t1`, `${id}t2`, `${id}t3`],
    stickers: [`${id}k1`, `${id}k2`],
  }))

function viewOf(m: Model) {
  return {
    spreads: m.scene.map((s) => ({
      id: s.id,
      grid: true,
      w: 600,
      h: 300,
      mid: 300,
      photos: s.places.filter((p) => p.photo).map((p) => p.photo),
      rects: [],
      places: s.places.map((p, i) => ({ id: p.id, photo: p.photo, x: 10 + i * 60, y: 20, w: 50, h: 50, z: i })),
      texts: s.texts.map((id) => ({ id, x: 0, y: 0, w: 10, h: 10 })),
      captions: [],
      backdrops: [],
      stickers: s.stickers.map((id) => ({ id, x: 0, y: 0, w: 10, h: 10 })),
    })),
    settings: { pageW: 300, pageH: 300, margin: 12, gap: 3, layflat: true, foldAny: false, dpi: 300, bg: '' },
    folders: [],
    rev: m.rev,
    safe: [],
  }
}

const apply = (m: Model) => {
  m.rev++
  store.apply(viewOf(m) as never)
  flushSync()
}

// Живой слой текстов большого разворота: его эффекты («фото выбрали — текст отпускаем», «блок пропал», «другой
// разворот») и есть правила, которые проверяет цепочка. Сами блоки не рисуются (тексты в сцене без стиля), эффектам
// нужен только id листа; рамки текстов — забота TextOverlay.
const layerProps = () => ({ spread: { id: store.spread?.id ?? '', texts: [], captions: [] } as never, tg: {} as never })
let layer: ReturnType<typeof render> | null = null

const cur = (m: Model): Sheet => m.scene[store.current]
// Всё, что можно выделить на текущем листе.
const refsOf = (s: Sheet) => s.places.map((p) => p.photo || p.id)
const dups = (a: readonly string[]) => a.length !== new Set(a).size

const chain: Chain<Model> = {
  settle: async () => {
    await layer?.rerender(layerProps())
    flushSync()
    await tick()
  },
  actions: [
    {
      name: 'pick',
      run: (m, r) => {
        const ref = r.pick(refsOf(cur(m)))
        const add = r.chance(0.5)
        placesel.pick(ref, add)
        return `${ref},${add}`
      },
    },
    {
      name: 'select',
      run: (m, r) => {
        const refs = refsOf(cur(m)).filter(() => r.chance(0.4))
        placesel.select(refs)
        return refs.join('+')
      },
    },
    {
      name: 'pickText',
      run: (m, r) => {
        if (cur(m).texts.length === 0) return 'пусто'
        const id = r.pick(cur(m).texts)
        placesel.pickText(id)
        return id
      },
    },
    {
      name: 'pickSticker',
      run: (m, r) => {
        if (cur(m).stickers.length === 0) return 'пусто'
        const id = r.pick(cur(m).stickers)
        const add = r.chance(0.5)
        placesel.pickSticker(id, add)
        return `${id},${add}`
      },
    },
    { name: 'selectAll', run: () => placesel.selectAll() },
    {
      name: 'pickHit',
      run: (m, r) => {
        const s = cur(m)
        const hit = {
          refs: refsOf(s).filter(() => r.chance(0.4)),
          texts: s.texts.filter(() => r.chance(0.3)),
          stickers: s.stickers.filter(() => r.chance(0.3)),
        }
        const add = r.chance(0.5)
        placesel.pickHit(hit, add)
        return `${hit.refs.length}/${hit.texts.length}/${hit.stickers.length},${add}`
      },
    },
    { name: 'clear', run: () => placesel.clear() },
    {
      name: 'editSelect',
      run: (m, r) => {
        if (cur(m).texts.length === 0) return 'пусто'
        const id = r.pick(cur(m).texts)
        edit.select({ kind: 'text', spread: cur(m).id, id })
        return id
      },
    },
    { name: 'editClear', run: () => edit.clear() },
    {
      name: 'cellNull',
      run: () => {
        store.selectedCell = null
      },
    },
    {
      name: 'go',
      run: (m, r) => {
        const i = r.int(m.scene.length)
        store.go(i)
        return String(i)
      },
    },
    {
      name: 'removeObject',
      run: (m, r) => {
        const s = cur(m)
        const kind = r.pick(['place', 'text', 'sticker'] as const)
        if (kind === 'place' && s.places.length > 1)
          m.gone.push(`${s.id}:p:${JSON.stringify(s.places.splice(r.int(s.places.length), 1)[0])}`)
        else if (kind === 'text' && s.texts.length > 0) m.gone.push(`${s.id}:t:${s.texts.splice(r.int(s.texts.length), 1)[0]}`)
        else if (kind === 'sticker' && s.stickers.length > 0) m.gone.push(`${s.id}:k:${s.stickers.splice(r.int(s.stickers.length), 1)[0]}`)
        apply(m)
        return kind
      },
    },
    {
      name: 'restoreObject',
      run: (m) => {
        const g = m.gone.pop()
        if (!g) return 'нечего'
        const [sid, kind, rest] = [g.slice(0, 2), g[3], g.slice(5)]
        const s = m.scene.find((x) => x.id === sid)
        if (!s) return 'нет листа'
        if (kind === 'p') s.places.push(JSON.parse(rest))
        else if (kind === 't') s.texts.push(rest)
        else s.stickers.push(rest)
        apply(m)
        return kind
      },
    },
    {
      name: 'spreadToggle',
      run: (m, r) => {
        const id = r.pick(m.scene).id
        spreadsel.toggle(id)
        return id
      },
    },
    {
      name: 'spreadExtend',
      run: (m, r) => {
        const to = r.pick(m.scene).id
        spreadsel.extend(
          m.scene.map((s) => s.id),
          cur(m).id,
          to,
        )
        return to
      },
    },
    { name: 'spreadClear', run: () => spreadsel.clear() },
  ],
  invariants: [
    () => (dups(placesel.refs) ? `дубль в refs: ${placesel.refs}` : null),
    () => (dups(placesel.texts) ? `дубль в texts: ${placesel.texts}` : null),
    () => (dups(placesel.stickers) ? `дубль в stickers: ${placesel.stickers}` : null),
    () => (dups(spreadsel.ids) ? `дубль в spreadsel: ${spreadsel.ids}` : null),
    // Выделенное существует на текущем листе: после правки вида prune, после перехода clear.
    (m) => {
      const s = cur(m)
      const bad = placesel.refs.filter((x) => !refsOf(s).includes(x))
      return bad.length ? `refs вне листа ${s.id}: ${bad}` : null
    },
    (m) => {
      const bad = placesel.texts.filter((x) => !cur(m).texts.includes(x))
      return bad.length ? `texts вне листа: ${bad}` : null
    },
    (m) => {
      const bad = placesel.stickers.filter((x) => !cur(m).stickers.includes(x))
      return bad.length ? `stickers вне листа: ${bad}` : null
    },
    (m) =>
      spreadsel.ids.some((id) => !m.scene.some((s) => s.id === id)) ? `spreadsel держит несуществующий разворот: ${spreadsel.ids}` : null,
    // «Единственный текст без фото и стикеров — одиночный»: он живёт в edit.sel, а не в texts.
    () =>
      placesel.texts.length === 1 && placesel.refs.length === 0 && placesel.stickers.length === 0 && !edit.sel
        ? `один текст группы без пары: ${placesel.texts}`
        : null,
    // Одиночный текст и группа не живут вместе.
    () => (edit.sel?.kind === 'text' && placesel.texts.length > 0 ? `edit.sel ${edit.sel.id} и texts ${placesel.texts}` : null),
    // «Выделение фото и текста взаимоисключающие» (edit.svelte.ts).
    // Это же касается пустых мест: refs не живут рядом с выделенным текстом.
    () => (edit.sel?.kind === 'text' && placesel.refs.length > 0 ? `текст ${edit.sel.id} и места ${placesel.refs} выделены вместе` : null),
    // edit.sel не висит на пропавшем блоке.
    (m) => {
      if (edit.sel?.kind !== 'text') return null
      const sh = m.scene.find((s) => s.id === edit.sel?.spread)
      return sh && sh.texts.includes(edit.sel.id) ? null : `edit.sel ${edit.sel.id} на пропавшем блоке`
    },
    () =>
      placesel.count !== placesel.refs.length + placesel.texts.length + placesel.stickers.length + (edit.sel?.kind === 'text' ? 1 : 0)
        ? 'count расходится'
        : null,
  ],
}

beforeEach(() => {
  placesel.clear()
  edit.clear()
  spreadsel.clear()
  store.apply({ ...viewOf({ scene: freshScene(), rev: 0, gone: [] }), rev: 1 } as never)
  store.go(0)
  layer = render(TextLayer, layerProps())
  flushSync()
})
afterEach(() => {
  cleanup()
  layer = null
})

describe('цепочки состояний: выделение', () => {
  for (const seed of chainSeeds()) {
    it(
      `семя ${seed}: ${chainSteps()} случайных действий`,
      async () => {
        const m: Model = { scene: freshScene(), rev: 10, gone: [] }
        store.apply(viewOf(m) as never)
        store.go(0)
        await runChain(m, chain, seed, chainSteps())
      },
      chainTimeout(chainSteps()),
    )
  }

  // Найдено цепочкой (семя 424242): щелчок по ПУСТОМУ месту при выделенном тексте оставлял текст выделенным —
  // эффект слоя текстов смотрел на store.selectedCell, который у пустого места null. Теперь — на placesel.refs.
  it('пустое место при выделенном тексте — текст отпускается', async () => {
    const m: Model = { scene: freshScene(), rev: 10, gone: [] }
    store.apply(viewOf(m) as never)
    store.go(0)
    await chain.settle?.()
    edit.select({ kind: 'text', spread: 's1', id: 's1t1' })
    placesel.pick('s1p2', false) // s1p2 — пустое место
    await chain.settle?.()
    expect(edit.sel).toBeNull()
    expect(placesel.refs).toEqual(['s1p2'])
  })

  it('Shift+щелчок по месту при выделенном тексте — текст уходит в группу, не теряется', async () => {
    const m: Model = { scene: freshScene(), rev: 10, gone: [] }
    store.apply(viewOf(m) as never)
    store.go(0)
    edit.select({ kind: 'text', spread: 's1', id: 's1t1' })
    placesel.pick('s1p2', true)
    await chain.settle?.()
    expect(placesel.texts).toEqual(['s1t1'])
    expect(placesel.refs).toEqual(['s1p2'])
    expect(edit.sel).toBeNull()
  })

  it('пустое место без текста: выделение места не трогается', async () => {
    const m: Model = { scene: freshScene(), rev: 10, gone: [] }
    store.apply(viewOf(m) as never)
    store.go(0)
    placesel.pick('s1p2', false)
    await chain.settle?.()
    expect(placesel.refs).toEqual(['s1p2'])
  })

  it('сам прогон детерминирован: одно семя — одна цепочка', async () => {
    const trace = async (): Promise<string> => {
      placesel.clear()
      edit.clear()
      spreadsel.clear()
      const m: Model = { scene: freshScene(), rev: 10, gone: [] }
      store.apply(viewOf(m) as never)
      store.go(0)
      await runChain(m, chain, 99, 30)
      return [placesel.refs, placesel.texts, placesel.stickers, spreadsel.ids, store.current].join('|')
    }
    expect(await trace()).toBe(await trace())
  })
})
