// @vitest-environment happy-dom
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { cleanup, render } from '@testing-library/svelte'
import { flushSync, tick } from 'svelte'
import { chainSeeds, chainSteps, chainTimeout, runChain, type Chain } from '../../base/core/randchain'

// Цепочка состояний тура «Первые шаги» (К3). QA 0.22–0.25 находил тут одно и то же: тур открывался заново после
// закрытия, шаг 5 не снимал выделение, закрытый тур оставлял кольцо. Тур (tourstate + слой Tour.svelte с автозапуском)
// ведут случайные действия: старт, «Далее/Назад», закрытие, смена уровня, окна, выделение, тосты; после каждого —
// инварианты. Провал печатает семя и цепочку (SA_CHAIN_SEED=<семя> повторит).
const cfg = vi.hoisted(() => ({ first: '', calls: [] as string[][] }))
const setTour = vi.hoisted(() =>
  vi.fn(async (id: string, state: string) => {
    cfg.calls.push([id, state])
    // Как Go (internal/prefs/tours.go): done не откатывается в pending.
    if (id === 'first' && !(cfg.first === 'done' && state === 'pending')) cfg.first = state
    return {
      level: 'full',
      showStart: false,
      first: false,
      tourDue: cfg.first !== 'done',
      tourMore: id === 'spread' ? false : state === 'done',
    }
  }),
)
vi.mock('../../../wailsjs/runtime/runtime', () => ({ EventsOn: () => () => {} }))
vi.mock('../../../wailsjs/go/app/App', () => ({
  SetTour: setTour,
  SetLevel: async () => ({}),
  LogUI: async () => {},
  OpenLink: async () => {},
}))
vi.mock('../../editor/text/fonts', () => ({ loadFonts: async () => {}, styleSupport: () => true }))

const { default: Tour } = await import('./Tour.svelte')
const { tour } = await import('./tourstate.svelte')
const { MORE_TOAST, SKIP_TOAST } = await import('./steps')
const { store } = await import('../../base/core/store.svelte')
const { jobs } = await import('../../base/core/jobs.svelte')
const { modalStack } = await import('../../base/modal/modalstack.svelte')
const { prefs } = await import('../../base/prefs/prefs.svelte')
const { spot } = await import('../guide/spotstate.svelte')
const { wire } = await import('../boot/wire')

wire()

interface Model {
  view: unknown // вид, с которым начали: тур проект не правит
  windows: number[] // id открытых окон в modalStack
  everStarted: boolean // тур уже открывался в этом сеансе (любым способом)
  userSel: boolean // человек трогал выделение с последнего старта
  selAtStart: string | null
  moreToasts: number // сколько раз показано предложение продолжения
  lastToast: unknown
}

const spread = (id: string, photos: string[]) => ({
  id,
  grid: false,
  w: 600,
  h: 300,
  mid: 300,
  photos,
  rects: [],
  places: [],
  texts: [],
  captions: [],
  backdrops: [],
})
const VIEW = {
  spreads: [spread('s1', ['a1', 'a2']), spread('s2', []), spread('s3', ['c1'])],
  settings: { pageW: 300, pageH: 300, margin: 12, gap: 3, layflat: true, foldAny: false, dpi: 300, bg: '' },
  folders: [{ path: '/a' }],
  rev: 5,
  safe: [],
}

const LEVELS = ['simple', 'standard', 'full'] as const
const load = (o: { level?: string; showStart?: boolean; tourDue?: boolean; tourMore?: boolean; custom?: unknown }) =>
  prefs.load({
    level: 'full',
    showStart: false,
    first: false,
    custom: null,
    tourDue: prefs.tourDue,
    tourMore: prefs.tourMore,
    ...o,
  } as never)

const chain: Chain<Model> = {
  settle: async () => {
    flushSync()
    await tick()
    flushSync()
  },
  actions: [
    {
      name: 'startMenu',
      run: (m) => {
        m.userSel = false
        m.selAtStart = store.selectedCell
        m.everStarted = true
        tour.start('menu')
      },
    },
    {
      name: 'startMore',
      run: (m) => {
        m.userSel = false
        m.selAtStart = store.selectedCell
        m.everStarted = true
        tour.start('more')
      },
    },
    // Прямо на шаг «Фото» (он сам выделяет первое фото и снимает выделение при закрытии) и обратно к «Раскладкам».
    {
      name: 'toPhotoStep',
      run: (m) => {
        if (!tour.open) {
          m.userSel = false
          m.selAtStart = store.selectedCell
          m.everStarted = true
          tour.start('menu')
        }
        tour.go(tour.steps.findIndex((x) => x.id === 'photo'))
      },
    },
    { name: 'next', run: () => tour.next() },
    { name: 'back', run: () => tour.back() },
    {
      name: 'go',
      run: (_m, r) => {
        const i = r.int(7)
        tour.go(i)
        return String(i)
      },
    },
    { name: 'done', run: () => tour.finish('done') },
    { name: 'skipClick', run: () => tour.finish('skip', 'click') },
    { name: 'skipKey', run: () => tour.finish('skip', 'key') },
    {
      name: 'level',
      run: (_m, r) => {
        const l = r.pick(LEVELS)
        load({ level: l })
        return l
      },
    },
    // «Свой» уровень без части функций: шаги 5 и 6 (раскладки, поворот фото) пропадают из тура, число шагов меняется.
    {
      name: 'custom',
      run: (_m, r) => {
        const off = ['variants', 'turn'].filter(() => r.chance(0.5))
        load({ level: 'custom', custom: { base: 'simple', on: [], off } })
        return off.join('+') || 'всё'
      },
    },
    // Go ответил: тур нужен / не нужен (флаги приходят из его ответов).
    {
      name: 'tourDue',
      run: (_m, r) => {
        const v = r.chance(0.5)
        prefs.tourDue = v
        return String(v)
      },
    },
    {
      name: 'tourMore',
      run: (_m, r) => {
        const v = r.chance(0.5)
        prefs.tourMore = v
        return String(v)
      },
    },
    {
      name: 'startWindow',
      run: (_m, r) => {
        const v = r.chance(0.5)
        if (v) prefs.startOpen = true
        else prefs.closeStart()
        return String(v)
      },
    },
    {
      name: 'openWindow',
      run: (m) => {
        m.windows.push(modalStack.push())
      },
    },
    {
      name: 'closeWindow',
      run: (m) => {
        const id = m.windows.pop()
        if (id !== undefined) modalStack.remove(id)
      },
    },
    {
      name: 'opening',
      run: (_m, r) => {
        jobs.openShown = r.chance(0.5)
        return String(jobs.openShown)
      },
    },
    {
      name: 'loaded',
      run: (_m, r) => {
        store.loaded = r.chance(0.8)
        return String(store.loaded)
      },
    },
    {
      name: 'userSelect',
      run: (m, r) => {
        m.userSel = true
        const p = r.pick(['a1', 'a2', 'c1', null])
        store.selectedCell = p
        return String(p)
      },
    },
    {
      name: 'goSpread',
      run: (_m, r) => {
        const i = r.int(3)
        store.go(i)
        return String(i)
      },
    },
    {
      name: 'toast',
      run: (_m, r) => {
        if (r.chance(0.5)) store.notify('Чужой тост')
        else store.toast = null
      },
    },
    {
      name: 'spotForeign',
      run: (_m, r) => {
        if (r.chance(0.5)) spot.cancel()
      },
    },
  ],
  invariants: [
    // Закрытый тур ничего не держит.
    () => (!tour.open && (tour.popup || tour.ring !== null || spot.cur?.hold) ? 'тур закрыт, а кольцо/ловушка живы' : null),
    // Шаг всегда существует.
    () => {
      if (!tour.open || tour.at === 'intro' || tour.at === 'final') return null
      return Number.isInteger(tour.at) && tour.at >= 0 && tour.at < tour.steps.length
        ? null
        : `шаг ${tour.at} вне 0..${tour.steps.length - 1} (уровень ${prefs.level})`
    },
    // Тур проект не правит.
    (m) => (store.view !== m.view ? 'тур заменил store.view' : null),
    // done не откатывается в pending.
    () => {
      const done = cfg.calls.findIndex((c) => c[0] === 'first' && c[1] === 'done')
      return done >= 0 && cfg.calls.slice(done + 1).some((c) => c[0] === 'first' && c[1] === 'pending')
        ? `после done ушёл pending: ${JSON.stringify(cfg.calls)}`
        : null
    },
    // Предложение продолжения — один раз.
    (m) => {
      if (store.toast !== m.lastToast) {
        m.lastToast = store.toast
        if (store.toast?.text === MORE_TOAST) m.moreToasts++
      }
      return m.moreToasts > 1 ? 'предложение продолжения показано дважды' : null
    },
    // Выделение, поставленное самим туром, на закрытии снимается; своё человек не теряет.
    (m) =>
      !tour.open && m.selAtStart === null && !m.userSel && store.selectedCell !== null
        ? `после тура осталось выделение ${store.selectedCell}`
        : null,
  ],
}

// «Закрытый тур не открывается сам»: открытие без команды бывает только первым и только при tourDue.
const watchAuto = (m: Model): (() => void) => {
  let was = tour.open
  return () => {
    if (!was && tour.open && tour.from === 'auto' && m.everStarted) throw new Error('автозапуск открыл тур повторно')
    if (tour.open && tour.from === 'auto') m.everStarted = true
    was = tour.open
  }
}

beforeEach(() => {
  cfg.first = ''
  cfg.calls.length = 0
  setTour.mockClear()
  tour.started = false
  load({ level: 'full', showStart: false, tourDue: true, tourMore: false })
  prefs.startOpen = false
  jobs.openShown = false
  store.toast = null
  store.loaded = true
  store.apply(VIEW as never)
  store.go(0)
  render(Tour)
  flushSync()
})
afterEach(() => {
  cleanup()
  tour.finish('done')
  tour.started = false
  spot.cancel()
  jobs.openShown = false
  document.body.innerHTML = ''
})

describe('цепочки состояний: тур «Первые шаги»', () => {
  for (const seed of chainSeeds()) {
    it(
      `семя ${seed}: ${chainSteps()} случайных действий`,
      async () => {
        // Автозапуск мог сработать сразу при монтировании: это первый и законный запуск.
        const m: Model = {
          view: store.view,
          windows: [],
          everStarted: tour.open,
          userSel: false,
          selAtStart: null,
          moreToasts: 0,
          lastToast: null,
        }
        const check = watchAuto(m)
        const settle = chain.settle
        await runChain(
          m,
          {
            ...chain,
            settle: async () => {
              await settle?.()
              check()
            },
          },
          seed,
          chainSteps(),
        )
        for (const id of m.windows) modalStack.remove(id)
      },
      chainTimeout(chainSteps()),
    )
  }

  // Смена уровня на «Свой» без «Раскладок»/«Поворота фото» при открытом туре сокращает шаги (7 → 5): номер шага
  // зажимается в список, карточка и кольцо не остаются без цели.
  it('БАГ: уровень «Свой» при открытом туре — номер шага возвращается в пределы', () => {
    load({ level: 'full', tourDue: false })
    tour.start('menu')
    tour.go(6)
    load({ level: 'custom', custom: { base: 'simple', on: [], off: ['variants', 'turn'] } })
    flushSync()
    expect(tour.steps).toHaveLength(5)
    expect(tour.at).toBe(4)
    expect(tour.step).toBeDefined()
    expect(tour.open).toBe(true)
  })

  it('уровень «Свой» при открытом туре: шаг в пределах списка остаётся на месте', () => {
    load({ level: 'full', tourDue: false })
    tour.start('menu')
    tour.go(2)
    load({ level: 'custom', custom: { base: 'simple', on: [], off: ['variants', 'turn'] } })
    flushSync()
    expect(tour.at).toBe(2)
  })

  it('приветствие и финал при смене уровня не трогаются', () => {
    load({ level: 'full', tourDue: false })
    tour.start('menu')
    load({ level: 'custom', custom: { base: 'simple', on: [], off: ['variants', 'turn'] } })
    flushSync()
    expect(tour.at).toBe('intro')
  })

  it('шагов не осталось — тур закрывается как пропущенный', () => {
    load({ level: 'full', tourDue: false })
    tour.start('menu')
    tour.go(3)
    tour.fit(0)
    expect(tour.open).toBe(false)
    expect(store.toast?.text).toBe(SKIP_TOAST)
  })
})
