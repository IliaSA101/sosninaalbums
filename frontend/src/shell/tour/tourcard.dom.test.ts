// @vitest-environment happy-dom
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { cleanup, fireEvent, render, screen } from '@testing-library/svelte'
import { flushSync, tick } from 'svelte'

// Go подменён: к Api тур ходит только за SetTour и OpenLink (через prefs и aboutUi).
const setTour = vi.hoisted(() =>
  vi.fn(async (_id: string, _state: string) => ({ level: 'simple', showStart: false, first: false, tourDue: false, tourMore: false })),
)
const openLink = vi.hoisted(() => vi.fn(async (_id: string) => {}))
vi.mock('../../../wailsjs/runtime/runtime', () => ({ EventsOn: () => () => {} }))
vi.mock('../../../wailsjs/go/app/App', () => ({ SetTour: setTour, OpenLink: openLink }))

const { default: Tour } = await import('./Tour.svelte')
const { tour } = await import('./tourstate.svelte')
const { store } = await import('../../base/core/store.svelte')
const { prefs } = await import('../../base/prefs/prefs.svelte')
const { modalStack } = await import('../../base/modal/modalstack.svelte')
const { guide } = await import('../guide/guidestate.svelte')
const { spot } = await import('../guide/spotstate.svelte')
const { placesel } = await import('../../editor/places/placesel.svelte')
const { default: Spot } = await import('../guide/Spot.svelte')

const card = () => document.querySelector<HTMLElement>('[data-tour-step]')
const stepId = () => card()?.getAttribute('data-tour-step')
const key = (k: string, target: Element | null = document.activeElement) => fireEvent.keyDown(target ?? document.body, { key: k })
const btn = (name: string) => screen.getByRole('button', { name })

beforeEach(() => {
  prefs.load({ level: 'full', showStart: false, first: false, custom: null, tourDue: true })
  store.loaded = true
  store.toast = null
})
afterEach(async () => {
  cleanup() // слой снят раньше finish: его автозапуск не должен поднять тур заново
  tour.finish('done')
  await new Promise((r) => setTimeout(r, 0)) // ответы SetTour не должны лечь на флаги следующего теста
  spot.cancel()
  guide.open = false
  setTour.mockClear()
  openLink.mockClear()
  tour.started = false
})

async function open(from: 'auto' | 'menu' | 'more' = 'menu') {
  tour.started = from !== 'auto' // иначе слой сам начнёт тур: tourDue стоит с beforeEach
  render(Tour)
  tour.start(from)
  flushSync()
  await tick()
}

describe('TourCard: приветствие и шаги', () => {
  it('приветствие со списком шагов; «Начать» ведёт к шагу 1, фокус на «Дальше»', async () => {
    await open()
    expect(stepId()).toBe('intro')
    expect(screen.getByRole('dialog').textContent).toContain('Покажу на вашем экране')
    expect(screen.getAllByRole('listitem')).toHaveLength(7)
    expect(document.activeElement?.textContent).toBe('Начать')
    await fireEvent.click(btn('Начать'))
    expect(stepId()).toBe('folders')
    expect(document.activeElement?.textContent).toBe('Дальше')
    expect(card()?.textContent).toContain('Шаг 1 из 7 · Папка со съёмкой')
  })

  it('строка списка — сразу этот шаг; отметок в config не пишет (запуск из меню не автоматический)', async () => {
    prefs.load({ level: 'full', showStart: false, first: false, custom: null })
    await open()
    await fireEvent.click(btn('6. Фото на развороте'))
    expect(stepId()).toBe('photo')
    expect(setTour).not.toHaveBeenCalled()
  })

  it('Enter и → — дальше, ← — назад (фокус в карточке)', async () => {
    await open()
    await fireEvent.click(btn('Начать'))
    await key('Enter')
    expect(stepId()).toBe('library')
    await key('ArrowRight')
    expect(stepId()).toBe('autobuild')
    await key('ArrowLeft')
    expect(stepId()).toBe('library')
    expect(document.activeElement?.textContent).toBe('Дальше')
  })

  it('Enter на «Пропустить» нажимает его, а не листает', async () => {
    await open()
    await fireEvent.click(btn('Начать'))
    const skip = btn('Пропустить')
    skip.focus()
    await key('Enter', skip)
    expect(stepId()).toBe('folders')
  })

  it('без фокуса в карточке клавиши не перехватываются', async () => {
    await open()
    await fireEvent.click(btn('Начать'))
    ;(document.activeElement as HTMLElement).blur()
    const ev = new KeyboardEvent('keydown', { key: 'ArrowRight', bubbles: true, cancelable: true })
    document.body.dispatchEvent(ev)
    flushSync()
    expect(stepId()).toBe('folders')
    expect(ev.defaultPrevented).toBe(false)
  })

  it('клавиша в карточке не доходит до программы', async () => {
    await open()
    const seen = vi.fn()
    window.addEventListener('keydown', seen)
    await key('ArrowRight')
    await key('Delete')
    window.removeEventListener('keydown', seen)
    expect(seen).not.toHaveBeenCalled()
  })

  it('Esc — конец: SetTour(first, done) и тост', async () => {
    await open('auto')
    expect(setTour).toHaveBeenCalledWith('first', 'pending')
    await key('Escape')
    expect(card()).toBeNull()
    expect(tour.open).toBe(false)
    expect(setTour).toHaveBeenCalledWith('first', 'done')
    expect(store.toast?.text).toContain('Справка ▸ Первые шаги')
  })

  it('«Пропустить» — тост; «Готово» — финал без тоста, «Закрыть» — done', async () => {
    await open('auto')
    await fireEvent.click(btn('Начать'))
    await fireEvent.click(btn('Пропустить'))
    expect(store.toast?.text).toContain('Первые шаги')
    store.toast = null
    cleanup()
    await open('auto')
    await fireEvent.click(btn('7. Выгрузка'))
    expect(card()?.textContent).toContain('Шаг 7 из 7')
    await fireEvent.click(btn('Готово'))
    expect(stepId()).toBe('final')
    expect(store.toast).toBeNull()
    setTour.mockClear()
    await fireEvent.click(btn('Закрыть'))
    expect(setTour).toHaveBeenCalledWith('first', 'done')
    expect(store.toast).toBeNull()
  })

  it('«Видео о программе» и «Подробнее в справочнике» — только OpenLink и окно справочника', async () => {
    await open()
    await fireEvent.click(btn('Видео о программе ↗'))
    expect(openLink).toHaveBeenCalledWith('videos')
    await fireEvent.click(btn('Начать'))
    await fireEvent.click(btn('Подробнее в справочнике'))
    expect(guide.open).toBe(true)
    expect(guide.sel).toBe('folders')
    expect(setTour).not.toHaveBeenCalled()
  })

  it('открыто Modal — карточка скрыта; окно закрылось — шаг на месте', async () => {
    await open()
    await fireEvent.click(btn('Начать'))
    await key('ArrowRight')
    expect(stepId()).toBe('library')
    const id = modalStack.push()
    flushSync()
    expect(card()).toBeNull()
    modalStack.remove(id)
    flushSync()
    expect(stepId()).toBe('library')
    await tick()
    expect(document.activeElement?.textContent).toBe('Дальше')
  })

  it('«Свой» без раскладок: шагов 6, шаг пропущен', async () => {
    prefs.load({ level: 'custom', showStart: false, first: false, custom: { base: 'full', on: [], off: ['variants'] }, tourDue: true })
    await open()
    expect(card()?.textContent).toContain('6 шагов')
    expect(screen.getAllByRole('listitem')).toHaveLength(6)
    expect(screen.queryByText('5. Раскладки')).toBeNull()
  })

  it('живой текст: добавили папку на шаге 1 — карточка пишет «Фото уже в ленте»', async () => {
    await open()
    await fireEvent.click(btn('Начать'))
    expect(card()?.textContent).toContain('+ Папка с фото')
    store.view = { ...store.view, folders: [{ path: '/a', name: 'a', photos: 3 } as never] }
    flushSync()
    expect(card()?.textContent).toContain('Фото уже в ленте')
    store.view = { ...store.view, folders: [] }
  })
})

describe('TourCard: место по высоте текущего шага', () => {
  // happy-dom высоты не считает: высота карточки = длина её текста (у шагов она разная).
  const heightOf = vi.spyOn(HTMLElement.prototype, 'offsetHeight', 'get')
  const widthOf = vi.spyOn(HTMLElement.prototype, 'offsetWidth', 'get')
  beforeEach(() => {
    heightOf.mockImplementation(function (this: HTMLElement) {
      return this.hasAttribute('data-tour-step') ? (this.textContent ?? '').length : 0
    })
    widthOf.mockImplementation(function (this: HTMLElement) {
      return this.hasAttribute('data-tour-step') ? 320 : 0
    })
  })
  afterEach(() => {
    heightOf.mockReset()
    widthOf.mockReset()
  })

  it('смена шага с другой высотой текста: top считается по новой высоте сразу, без кадра с прошлой', async () => {
    await open()
    await fireEvent.click(btn('Начать'))
    const tops: number[] = []
    for (let i = 0; i < 6; i++) {
      const c = card()!
      const h = (c.textContent ?? '').length
      // По центру окна (кольца в тесте нет): top = (окно − высота) / 2 текущего текста
      expect(parseFloat(c.style.top), `шаг ${i + 1}`).toBe(Math.round(((window.innerHeight - h) / 2) * 1000) / 1000 || 0)
      tops.push(h)
      await key('ArrowRight')
    }
    expect(new Set(tops).size, 'тексты шагов разной высоты').toBeGreaterThan(2)
  })
})

describe('TourCard: шаг 5 при выделенном фото', () => {
  it('выделение снимается, проект не правится: rev тот же, вызовов Go нет', async () => {
    const view = { ...store.view, rev: 7, spreads: [{ id: 's1', photos: ['p1'], texts: [], grid: false } as never] }
    store.view = view
    placesel.refs = ['p1']
    await open()
    await fireEvent.click(btn('5. Раскладки'))
    expect(stepId()).toBe('layouts')
    expect(placesel.refs).toEqual([])
    expect(store.view.rev).toBe(7)
    expect(store.view).toBe(view)
    expect(setTour).not.toHaveBeenCalled()
    store.view = { ...store.view, spreads: [] }
  })
})

describe('кольцо тура и «Показать» справочника', () => {
  it('кольцо чужого «Показать» прячет тур; после его конца тур на месте', async () => {
    render(Spot)
    await open()
    await fireEvent.click(btn('Начать'))
    const el = document.createElement('button')
    el.setAttribute('data-guide', 'x')
    document.body.append(el)
    spot.flash(el, 'Чужое', 'где')
    flushSync()
    expect(card()).toBeNull()
    spot.cancel()
    flushSync()
    expect(stepId()).toBe('folders')
    el.remove()
  })
})

describe('Tour: автозапуск и продолжение', () => {
  it('ждёт закрытия стартового окна и Modal, потом открывает приветствие и пишет pending', async () => {
    prefs.startOpen = true
    tour.started = false
    render(Tour)
    flushSync()
    expect(tour.open).toBe(false)
    prefs.startOpen = false
    const id = modalStack.push()
    flushSync()
    expect(tour.open).toBe(false)
    modalStack.remove(id)
    flushSync()
    expect(tour.open).toBe(true)
    expect(stepId()).toBe('intro')
    expect(setTour).toHaveBeenCalledWith('first', 'pending')
  })

  it('tourDue ложь — тур сам не приходит', () => {
    prefs.load({ level: 'full', showStart: false, first: false, custom: null, tourDue: false })
    tour.started = false
    render(Tour)
    flushSync()
    expect(tour.open).toBe(false)
  })

  it('ссылка стартового окна закрывает его и открывает приветствие, автозапуск не двоится', () => {
    prefs.load({ level: 'full', showStart: true, first: false, custom: null, tourDue: true })
    tour.started = false
    render(Tour)
    tour.start('menu')
    flushSync()
    expect(prefs.startOpen).toBe(false)
    expect(stepId()).toBe('intro')
    expect(setTour).not.toHaveBeenCalledWith('first', 'pending')
  })

  it('tourMore и фото на развороте — один тост с «Показать»; «Показать» запускает шаги 5–7', async () => {
    prefs.load({ level: 'full', showStart: false, first: false, custom: null, tourMore: true })
    tour.started = true
    store.view = { ...store.view, spreads: [{ id: 's1', photos: ['p1'], texts: [], grid: false } as never] }
    render(Tour)
    flushSync()
    expect(store.toast?.text).toContain('Показать, как подобрать раскладку')
    expect(setTour).toHaveBeenCalledWith('spread', 'done')
    store.toast?.action?.run()
    flushSync()
    expect(card()?.textContent).toContain('Шаг 1 из 3 · Раскладки')
    store.view = { ...store.view, spreads: [] }
  })

  it('без фото тост продолжения не появляется', () => {
    prefs.load({ level: 'full', showStart: false, first: false, custom: null, tourMore: true })
    tour.started = true
    render(Tour)
    flushSync()
    expect(store.toast).toBeNull()
  })
})
