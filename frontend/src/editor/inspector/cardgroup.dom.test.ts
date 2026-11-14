// @vitest-environment happy-dom
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { cleanup, fireEvent, render, screen } from '@testing-library/svelte'
import { createRawSnippet, flushSync } from 'svelte'

vi.mock('../../../wailsjs/runtime/runtime', () => ({ EventsOn: () => () => {} }))

const { default: CardGroup } = await import('./CardGroup.svelte')
const { cardGroups, cardContext, dropLanded, GROUP_IDS } = await import('../../base/prefs/cardgroups.svelte')
const { drag } = await import('../../base/drag/drag.svelte')
const { prefs } = await import('../../base/prefs/prefs.svelte')
const { spot } = await import('../../shell/guide/spotstate.svelte')
const { LEVELS } = await import('../../base/prefs/levels')
const { features } = await import('../../base/prefs/levelcustom')
const { groupShown } = await import('./groupsum')
type FeatureId = import('../../base/prefs/levels').FeatureId

// Тело группы: одна строка и, если нужно, зона фона (как в карточке листа).
const body = (html = '<input aria-label="поле">') => createRawSnippet(() => ({ render: () => `<div class="row">${html}</div>` }))
const ZONE = '<div data-drop="backdrop" data-spread="s1" data-scope="spread" aria-label="зона"></div>'
const docked = { context: new Map([cardContext(() => true)]) }
const floating = { context: new Map([cardContext(() => false)]) }
type Props = Record<string, unknown>
const show = (props: Props, ctx: { context: Map<unknown, unknown> } = docked) => render(CardGroup, { props: props as never, ...ctx })

const head = (name: RegExp | string) => screen.getByRole('button', { name })
const bodyOf = (id: string) => document.querySelector<HTMLElement>(`[data-group="${id}"] .body`)!

beforeEach(() => {
  localStorage.clear()
  cardGroups.reset()
  vi.spyOn(prefs, 'can').mockReturnValue(true) // все функции включены; уровни — в последнем describe
})
afterEach(() => {
  cleanup()
  document.querySelectorAll('body > [data-drop]').forEach((e) => e.remove()) // рамки холста из тестов броска
  drag.active = false
  drag.payload = null
  drag.target = null
  spot.cancel()
  vi.restoreAllMocks()
})

describe('CardGroup: заголовок и сворачивание', () => {
  it('по умолчанию открыта; щелчок сворачивает, тело не размонтируется и inert; щелчок обратно открывает', async () => {
    show({ id: 'photo.frame', title: 'Кадр', children: body() })
    const h = head('Кадр')
    expect(h.getAttribute('aria-expanded')).toBe('true')
    expect(h.getAttribute('aria-controls')).toBe(bodyOf('photo.frame').id)
    expect(bodyOf('photo.frame').hasAttribute('inert')).toBe(false)

    await fireEvent.click(h)
    expect(h.getAttribute('aria-expanded')).toBe('false')
    expect(bodyOf('photo.frame').hasAttribute('inert')).toBe(true)
    expect(screen.getByLabelText('поле')).toBeTruthy() // тело в DOM: набранное и якоря F1 живы
    expect(JSON.parse(localStorage.getItem('cardGroups')!)).toEqual({ d: ['photo.frame'], f: [] })

    await fireEvent.click(h)
    expect(h.getAttribute('aria-expanded')).toBe('true')
    expect(bodyOf('photo.frame').hasAttribute('inert')).toBe(false)
    expect(JSON.parse(localStorage.getItem('cardGroups')!)).toEqual({ d: [], f: [] })
  })

  it('заголовок — кнопка: Enter и пробел сворачивают штатно', () => {
    show({ id: 'photo.frame', title: 'Кадр', children: body() })
    expect(head('Кадр').tagName).toBe('BUTTON')
    expect((head('Кадр') as HTMLButtonElement).type).not.toBe('submit')
  })

  it('значение пишется только у свёрнутой, с полным текстом в title', async () => {
    show({ id: 'photo.frame', title: 'Кадр', summary: '120 % · с обрезкой · 2,5°', children: body() })
    expect(screen.queryByText('120 % · с обрезкой · 2,5°')).toBeNull()
    await fireEvent.click(head(/Кадр/))
    const sum = screen.getByText('120 % · с обрезкой · 2,5°')
    expect(sum.getAttribute('title')).toBe('120 % · с обрезкой · 2,5°')
  })

  it('пустое значение — подписи нет', async () => {
    show({ id: 'photo.about', title: 'О снимке', summary: '', children: body() })
    await fireEvent.click(head('О снимке'))
    expect(document.querySelector('.sum')).toBeNull()
  })

  it('панель деталей и плавающая карточка помнят отдельно', async () => {
    show({ id: 'photo.tone', title: 'Цвет и тон', children: body() }, floating)
    expect(document.querySelector('[data-group-host]')!.getAttribute('data-group-host')).toBe('f')
    await fireEvent.click(head('Цвет и тон'))
    expect(cardGroups.isOpen('f', 'photo.tone')).toBe(false)
    expect(cardGroups.isOpen('d', 'photo.tone')).toBe(true)
  })

  it('без контекста карточки — хост панели деталей', () => {
    show({ id: 'photo.tone', title: 'Цвет и тон', children: body() }, { context: new Map() })
    expect(document.querySelector('[data-group-host]')!.getAttribute('data-group-host')).toBe('d')
  })

  it('память читается при показе: свёрнутая с прошлого запуска рисуется свёрнутой', () => {
    cardGroups.toggle('d', 'photo.place')
    show({ id: 'photo.place', title: 'Место', children: body() })
    expect(head('Место').getAttribute('aria-expanded')).toBe('false')
  })
})

describe('CardGroup: «⋯»', () => {
  it('нет списка — нет кнопки; со списком пункт зовёт onmore и закрывает меню', async () => {
    const { unmount } = show({ id: 'spread.vig', title: 'Виньетка', children: body() })
    expect(screen.queryByRole('button', { name: /Ещё/ })).toBeNull()
    unmount()

    const onmore = vi.fn()
    show({
      id: 'spread.vig',
      title: 'Виньетка',
      more: [{ id: 'save', label: 'Сохранить как шаблон…' }],
      onmore,
      moreGuide: 'vig-save',
      children: body(),
    })
    const more = screen.getByRole('button', { name: 'Ещё: Виньетка' })
    expect(more.getAttribute('data-guide-hold')).toBe('vig-save')
    await fireEvent.click(more)
    await fireEvent.click(document.querySelector('[data-item="save"]')!) // меню скрыто до замера (в happy-dom размеры 0)
    expect(onmore).toHaveBeenCalledExactlyOnceWith(expect.objectContaining({ id: 'save' }))
    expect(screen.queryByRole('menu')).toBeNull()
  })

  it('«⋯» остаётся в заголовке свёрнутой группы', async () => {
    show({ id: 'spread.vig', title: 'Виньетка', more: [{ id: 'a', label: 'Раз' }], children: body() })
    await fireEvent.click(head('Виньетка'))
    expect(screen.getByRole('button', { name: 'Ещё: Виньетка' })).toBeTruthy()
  })
})

describe('CardGroup: кольцо справочника', () => {
  it('сворачивание группы с кольцом гасит его; кольцо снаружи не трогает', async () => {
    show({ id: 'photo.frame', title: 'Кадр', children: body() })
    const inside = screen.getByLabelText('поле')
    const outside = document.body.appendChild(document.createElement('button'))
    spot.flash(outside, '', '', undefined, { hold: true })
    await fireEvent.click(head('Кадр'))
    expect(spot.cur?.el).toBe(outside)
    await fireEvent.click(head('Кадр')) // открыли
    spot.flash(inside, '', '', undefined, { hold: true })
    await fireEvent.click(head('Кадр')) // свернули руками
    expect(spot.cur).toBeNull()
  })

  it('раскрытие кольцо не гасит', async () => {
    cardGroups.toggle('d', 'photo.frame')
    show({ id: 'photo.frame', title: 'Кадр', children: body() })
    spot.flash(screen.getByLabelText('поле'), '', '', undefined, { hold: true })
    await fireEvent.click(head('Кадр'))
    expect(spot.cur).not.toBeNull()
  })
})

describe('CardGroup: перенос фото и бросок', () => {
  // Цель переноса и элемент под курсором (в happy-dom раскладки нет): zone — зона из группы, иначе рамка на холсте.
  let under: Element[] = []
  beforeEach(() => {
    under = []
    Object.defineProperty(document, 'elementsFromPoint', { configurable: true, value: () => under })
  })
  afterEach(() => {
    delete (document as { elementsFromPoint?: unknown }).elementsFromPoint
  })
  const drop = (target: object | null, el: Element[] = under) => {
    under = el
    drag.active = true
    drag.payload = { kind: 'photos', ids: ['a'] }
    drag.target = target as never
    flushSync()
  }
  const zoneEl = () => [screen.getByLabelText('зона')]
  // Рамка фона на холсте (BackdropEditor): те же data-drop, data-spread и data-scope, но вне группы.
  const canvasFrame = () => {
    const f = document.createElement('div')
    f.dataset.drop = 'backdrop'
    f.dataset.spread = 's1'
    f.dataset.scope = 'spread'
    document.body.append(f)
    return [f]
  }
  const release = () => {
    drag.active = false
    drag.payload = null
    drag.target = null
    flushSync()
  }

  beforeEach(() => cardGroups.toggle('d', 'spread.bg'))

  it('пока тянут фото, группа с зоной раскрыта без записи; отпустили — снова свёрнута', () => {
    show({ id: 'spread.bg', title: 'Фон', dropOpen: true, children: body(ZONE) })
    expect(head('Фон').getAttribute('aria-expanded')).toBe('false')
    drop(null)
    expect(head('Фон').getAttribute('aria-expanded')).toBe('true')
    expect(cardGroups.isOpen('d', 'spread.bg')).toBe(false)
    release()
    expect(head('Фон').getAttribute('aria-expanded')).toBe('false')
  })

  it('без dropOpen и при переносе разворота группа остаётся свёрнутой', () => {
    show({ id: 'spread.caps', title: 'Подписи', children: body() })
    cardGroups.toggle('d', 'spread.caps')
    drop(null)
    expect(head('Подписи').getAttribute('aria-expanded')).toBe('false')
    release()
    show({ id: 'spread.bg', title: 'Фон', dropOpen: true, children: body(ZONE) })
    drag.active = true
    drag.payload = { kind: 'spread', id: 's', index: 0 }
    flushSync()
    expect(head('Фон').getAttribute('aria-expanded')).toBe('false')
  })

  it('удачный бросок в зону группы: остаётся открытой и запоминается', () => {
    show({ id: 'spread.bg', title: 'Фон', dropOpen: true, children: body(ZONE) })
    drop({ kind: 'backdrop', spread: 's1', scope: 'spread' }, zoneEl())
    release()
    expect(cardGroups.isOpen('d', 'spread.bg')).toBe(false) // Go ещё не ответил
    dropLanded.notify('s1', 'spread')
    flushSync()
    expect(cardGroups.isOpen('d', 'spread.bg')).toBe(true)
    expect(head('Фон').getAttribute('aria-expanded')).toBe('true')
    expect(JSON.parse(localStorage.getItem('cardGroups')!)).toEqual({ d: [], f: [] })
  })

  it('бросок на другую зону (рамка фона на холсте) группу не раскрывает', () => {
    show({ id: 'spread.bg', title: 'Фон', dropOpen: true, children: body(ZONE) })
    drop({ kind: 'backdrop', spread: 's1', scope: 'spread' }, canvasFrame()) // тот же scope, но элемент вне группы
    release()
    dropLanded.notify('s1', 'spread')
    flushSync()
    expect(cardGroups.isOpen('d', 'spread.bg')).toBe(false)
  })

  it('бросок мимо (цели нет) и ответ Go чужого броска группу не раскрывают', () => {
    show({ id: 'spread.bg', title: 'Фон', dropOpen: true, children: body(ZONE) })
    drop({ kind: 'backdrop', spread: 's1', scope: 'spread' }, zoneEl())
    drop(null) // увели курсор и отпустили
    release()
    dropLanded.notify('s1', 'spread')
    flushSync()
    expect(cardGroups.isOpen('d', 'spread.bg')).toBe(false)
  })

  it('отказ Go (счётчик не менялся) — группа свёрнута', () => {
    show({ id: 'spread.bg', title: 'Фон', dropOpen: true, children: body(ZONE) })
    drop({ kind: 'backdrop', spread: 's1', scope: 'spread' })
    release()
    flushSync()
    expect(cardGroups.isOpen('d', 'spread.bg')).toBe(false)
  })

  it('на ответ Go реагирует только одна группа, где была зона', () => {
    show({ id: 'spread.bg', title: 'Фон', dropOpen: true, children: body(ZONE) })
    show({ id: 'cover.bg', title: 'Фон обложки', dropOpen: true, children: body() })
    cardGroups.toggle('d', 'cover.bg')
    drop({ kind: 'backdrop', spread: 's1', scope: 'spread' }, zoneEl())
    release()
    dropLanded.notify('s1', 'spread')
    flushSync()
    expect(cardGroups.isOpen('d', 'spread.bg')).toBe(true)
    expect(cardGroups.isOpen('d', 'cover.bg')).toBe(false)
  })
})

// Здесь только согласие CardGroup с groupShown (тело всегда задано). Что на уровне нет группы с пустым телом —
// страж карточки «Фото»: photocard.order.dom.test.ts, «на каждом уровне нет группы с пустым телом».
describe('CardGroup: уровни — рисуется ровно по groupShown', () => {
  const levels = [...LEVELS.map((l) => [l, features(l, null)] as const), ['«Свой» без функций', new Set<FeatureId>()] as const]
  for (const [name, set] of levels) {
    it(`${name}: группа рисуется ровно тогда, когда у неё есть строки`, () => {
      vi.spyOn(prefs, 'can').mockImplementation((f) => set.has(f))
      for (const id of GROUP_IDS) {
        const { unmount } = show({ id, title: id, children: body('<span>строка</span>') })
        const sec = document.querySelector(`[data-group="${id}"]`)
        expect(!!sec, `${name} ${id}`).toBe(groupShown(id, prefs.can))
        if (sec) expect(sec.querySelector('.inner')!.textContent).toBe('строка')
        unmount()
      }
    })
  }
})
