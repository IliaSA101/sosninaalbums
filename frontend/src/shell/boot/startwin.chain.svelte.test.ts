// @vitest-environment happy-dom
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { flushSync } from 'svelte'
import { chainSeeds, chainSteps, chainTimeout, drain, runChain, type Chain } from '../../base/core/randchain'

// Цепочка состояний стартовых окон и открытия .salbum извне (К3): стартовое окно, любое другое окно, окно хода
// «Открываем альбом» и события `open:external` (двойной щелчок, второй запуск). QA ловил здесь «стартовое окно
// поверх альбома из двойного щелчка» (State.fromFile). Случайные действия, после каждого — инварианты; провал печатает
// семя и цепочку (SA_CHAIN_SEED=<семя> повторит).
const handlers: Record<string, (...a: unknown[]) => void> = {}
const openCalls: { windows: number }[] = []
const idle = vi.fn<() => Promise<void>>()
const view = { spreads: [], folders: [], rev: 1, dirty: false, path: '' }

vi.mock('../../../wailsjs/runtime/runtime', () => ({
  EventsOn: (name: string, fn: (...a: unknown[]) => void) => {
    handlers[name] = fn
    return () => {}
  },
}))
vi.mock('../../../wailsjs/go/app/App', () => ({
  GetState: () => Promise.resolve({ view, photos: [], missing: [] }),
  OpenExternal: () => {
    openCalls.push({ windows: modalStack.count() })
    return Promise.resolve({ view: { ...view, rev: view.rev + openCalls.length }, photos: [] })
  },
  SetLevel: async () => ({}),
  MissingNames: () => Promise.resolve({}),
}))
vi.mock('../../editor/text/edit.svelte', () => ({ edit: { idle: () => idle() } }))

const { store } = await import('../../base/core/store.svelte')
const { jobs } = await import('../../base/core/jobs.svelte')
const { noHooks } = await import('../../base/core/hooks')
const { modalStack } = await import('../../base/modal/modalstack.svelte')
const { prefs } = await import('../../base/prefs/prefs.svelte')
const { extOpen } = await import('./extopen.svelte')

interface Model {
  other: number[] // обычные окна (справочник, настройки…)
  progress: number | null // окно хода «Открываем альбом»
  requests: number
  lastToast: unknown
}

const WAIT = /откроется, когда закроете окно/

const chain: Chain<Model> = {
  // Эффекты, цепочка промисов, StartDialog: окно стоит в стеке, пока prefs.startOpen.
  settle: async () => {
    for (let i = 0; i < 3; i++) {
      flushSync()
      await drain()
    }
  },
  actions: [
    {
      name: 'request',
      run: (m, r) => {
        const name = `${r.pick(['A', 'B', 'Свадьба'])}.salbum`
        m.requests++
        handlers['open:external'](name)
        // Событие закрывает стартовое окно сразу: «стартовое окно поверх альбома из двойного щелчка».
        if (prefs.startOpen) throw new Error(`стартовое окно осталось открытым после open:external ${name}`)
        return name
      },
    },
    {
      name: 'requestTwice',
      run: (m) => {
        m.requests += 2
        handlers['open:external']('X.salbum')
        handlers['open:external']('Y.salbum')
      },
    },
    { name: 'startOn', run: () => prefs.load({ level: 'simple', showStart: true, first: false, custom: null, tourDue: false } as never) },
    {
      name: 'startFirst',
      run: () => prefs.load({ level: 'simple', showStart: false, first: true, custom: null, tourDue: false } as never),
    },
    { name: 'startClose', run: () => prefs.closeStart() },
    {
      name: 'news',
      run: () => {
        prefs.news = !prefs.news
      },
    },
    {
      name: 'openWindow',
      run: (m) => {
        m.other.push(modalStack.push())
      },
    },
    {
      name: 'closeWindow',
      run: (m, r) => {
        if (m.other.length) modalStack.remove(m.other.splice(r.int(m.other.length), 1)[0])
      },
    },
    {
      name: 'progressOn',
      run: (m) => {
        if (m.progress !== null) return
        jobs.openShown = true
        m.progress = modalStack.push()
      },
    },
    {
      name: 'progressOff',
      run: (m) => {
        if (m.progress === null) return
        modalStack.remove(m.progress)
        m.progress = null
        jobs.openShown = false
      },
    },
    {
      name: 'toastClear',
      run: () => {
        store.toast = null
      },
    },
  ],
  invariants: [
    // Файл не залежался: имени в очереди нет, если окон нет. С окнами — ждёт закрытия.
    () => (extOpen.name !== null && !modalStack.any() && startId === null ? `файл ${extOpen.name} не открыт при свободном экране` : null),
    // Открытие — только на свободном экране: в момент OpenExternal в стеке ни одного окна.
    () => {
      const bad = openCalls.find((c) => c.windows > 0)
      return bad ? `OpenExternal вызван при открытых окнах (${bad.windows})` : null
    },
    // Открытий не больше запросов (два события подряд — одно открытие).
    (m) => (openCalls.length > m.requests ? `открытий ${openCalls.length} больше событий ${m.requests}` : null),
    // Тост «откроется, когда закроете окно» — только если есть окно, которое можно закрыть (не одно окно хода).
    (m) => {
      if (store.toast === m.lastToast) return null
      m.lastToast = store.toast
      if (store.toast && WAIT.test(store.toast.text) && m.other.length === 0 && startId === null && m.progress !== null) {
        return `тост «${store.toast.text}» при единственном окне хода`
      }
      if (store.toast && WAIT.test(store.toast.text) && !modalStack.any()) return 'тост «откроется, когда закроете окно» без единого окна'
      return null
    },
  ],
}

let stop: () => void
// StartDialog: окно стоит в стеке, пока открыт prefs.startOpen, и уходит из него при закрытии (как Modal в {#if}).
let startId: number | null = null
let stopMirror: () => void

beforeEach(() => {
  for (const k of Object.keys(handlers)) delete handlers[k]
  openCalls.length = 0
  idle.mockReset()
  idle.mockResolvedValue(undefined)
  store.toast = null
  store.plug({ ...noHooks, external: (n) => extOpen.request(n) })
  prefs.startOpen = false
  prefs.news = false
  prefs.first = false
  jobs.openShown = false
  extOpen.name = null
  stop = extOpen.listen()
  startId = null
  stopMirror = $effect.root(() => {
    $effect(() => {
      if (prefs.startOpen) {
        if (startId === null) startId = modalStack.push()
      } else if (startId !== null) {
        modalStack.remove(startId)
        startId = null
      }
    })
  })
})
afterEach(() => {
  stop()
  stopMirror()
  if (startId !== null) modalStack.remove(startId)
  extOpen.name = null
  jobs.openShown = false
  prefs.startOpen = false
  flushSync()
})

describe('цепочки состояний: стартовые окна и открытие извне', () => {
  for (const seed of chainSeeds()) {
    it(
      `семя ${seed}: ${chainSteps()} случайных действий`,
      async () => {
        const m: Model = { other: [], progress: null, requests: 0, lastToast: null }
        await runChain(m, chain, seed, chainSteps())
        // Конец: все окна закрыты — очередь обязана опустеть, последнее событие откроется.
        for (const id of m.other) modalStack.remove(id)
        if (m.progress !== null) modalStack.remove(m.progress)
        jobs.openShown = false
        prefs.closeStart()
        await chain.settle?.()
        expect(extOpen.name, 'после закрытия всех окон файл должен открыться').toBeNull()
      },
      chainTimeout(chainSteps()),
    )
  }
})
