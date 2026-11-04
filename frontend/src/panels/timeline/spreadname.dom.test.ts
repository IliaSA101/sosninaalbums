// @vitest-environment happy-dom
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { cleanup, fireEvent, render } from '@testing-library/svelte'
import { flushSync } from 'svelte'

// Go подменён: нужен только SetSpreadName — ответ с новым названием, как отдал бы бэкенд.
const rename = vi.hoisted(() => vi.fn())
vi.mock('../../../wailsjs/runtime/runtime', () => ({ EventsOn: () => () => {} }))
vi.mock('../../../wailsjs/go/app/App', () => ({ SetSpreadName: rename }))

const { default: Timeline } = await import('./Timeline.svelte')
const { store } = await import('../../base/core/store.svelte')
const { prefs } = await import('../../base/prefs/prefs.svelte')
const { naming } = await import('./naming.svelte')
const { run } = await import('../../shell/menu/commands')

const sp = (id: string, name = '') => ({ id, name, photos: [], rects: [], w: 400, h: 200, kind: '' })
const setView = (spreads: ReturnType<typeof sp>[], rev: number) => {
  store.view = { ...store.view, rev, spreads, settings: {} } as never
}

// Номер разворота: <div class="num"> с подписью «1», «2 · Сборы»…
const nums = (c: HTMLElement) => Array.from(c.querySelectorAll('.group .num'), (n) => n.textContent?.trim())
const num = (c: HTMLElement, n: number) => c.querySelectorAll('.group .num')[n]
const field = (c: HTMLElement) => c.querySelector<HTMLInputElement>('input[aria-label="Название разворота"]')

// Синглтоны переживают тест: запомненный «первый щелчок» прошлого теста в пределах 500 мс стал бы половиной двойного.
const forgetClick = () =>
  naming.capture({
    target: null,
    clientX: -1000,
    clientY: -1000,
    timeStamp: 0,
    ctrlKey: false,
    shiftKey: false,
    metaKey: false,
    altKey: false,
    stopPropagation: () => {},
    preventDefault: () => {},
  })

beforeEach(() => {
  naming.stop()
  forgetClick()
  prefs.level = 'full' // названия разворотов — функция «Зум и названия»
  setView([sp('a'), sp('b', 'Сборы')], 1)
  store.currentId = 'a'
  store.onCover = false
  rename.mockImplementation(async (id: string, name: string) => {
    setView([sp('a', id === 'a' ? name : ''), sp('b', id === 'b' ? name : 'Сборы')], 2)
    return store.view
  })
})
afterEach(() => {
  cleanup()
  rename.mockReset()
})

// Двойной щелчок по номеру n-го разворота → поле ввода. Лента ловит его на корне (naming.capture, 094 §3.1):
// два щелчка подряд; второй может попасть в другую группу (`second`) — поле всё равно у разворота первого.
async function edit(c: HTMLElement, n: number, second = n) {
  await fireEvent.click(num(c, n))
  await fireEvent.click(num(c, second))
  flushSync()
  return field(c)!
}

describe('название разворота в ленте (двойной щелчок, F2)', () => {
  it('поле открывается с текущим названием, номера остальных не трогает', async () => {
    const { container } = render(Timeline)
    expect(nums(container)).toEqual(['1', '2 · Сборы'])
    const input = await edit(container, 1)
    expect(input.value).toBe('Сборы')
    expect(nums(container)).toEqual(['1'])
  })

  it('второй щелчок попал в другую группу (лента уехала): поле у разворота первого щелчка, перехода нет', async () => {
    const { container } = render(Timeline)
    const input = await edit(container, 0, 1)
    expect(input.closest('[data-spread]')!.getAttribute('data-spread')).toBe('a')
    expect(nums(container)).toEqual(['2 · Сборы']) // у «б» номер на месте, поле только у «а»
    expect(store.currentId).toBe('a') // второй щелчок погашен: на «б» не ушли
  })

  it('одиночный щелчок и щелчок с Ctrl первым поля не открывают', async () => {
    const { container } = render(Timeline)
    await fireEvent.click(num(container, 0))
    expect(field(container)).toBeNull()
    // Далеко от первого щелчка (в одной точке и сразу — это уже второй щелчок двойного)
    await fireEvent.click(num(container, 1), { ctrlKey: true, clientX: 300 }) // Ctrl — выделение групп, не начало двойного
    await fireEvent.click(num(container, 1), { clientX: 300 })
    expect(field(container)).toBeNull()
  })

  it('F2 открывает поле текущего разворота и не дублирует его; на обложке молчит', () => {
    const { container } = render(Timeline)
    store.go(1)
    run('renameSpread')
    flushSync()
    expect(field(container)!.value).toBe('Сборы')
    run('renameSpread')
    flushSync()
    expect(container.querySelectorAll('input[aria-label="Название разворота"]')).toHaveLength(1)
    naming.stop()
    flushSync()
    store.onCover = true
    run('renameSpread')
    flushSync()
    expect(field(container)).toBeNull()
  })

  it('F2 без функции «названия» молчит', () => {
    prefs.level = 'simple'
    const { container } = render(Timeline)
    run('renameSpread')
    flushSync()
    expect(field(container)).toBeNull()
  })

  it('двойной щелчок → Esc: поле ушло, значение прежнее, Go не звали; повторно — снова открывается', async () => {
    const { container } = render(Timeline)
    const input = await edit(container, 0)
    input.value = 'Черновик'
    await fireEvent.input(input)
    await fireEvent.keyDown(input, { key: 'Escape' })
    flushSync()
    expect(field(container)).toBeNull()
    expect(rename).not.toHaveBeenCalled()
    expect(nums(container)).toEqual(['1', '2 · Сборы'])
    forgetClick() // прошлый двойной щелчок израсходован, но тест держит интервал в мс
    expect((await edit(container, 0)).value).toBe('') // введённое при отмене не запомнилось
  })

  it('Esc после Esc/blur не сохраняет второй раз (blur от удаления поля)', async () => {
    const { container } = render(Timeline)
    const input = await edit(container, 0)
    input.value = 'X'
    await fireEvent.input(input)
    await fireEvent.keyDown(input, { key: 'Escape' })
    await fireEvent.blur(input)
    expect(rename).not.toHaveBeenCalled()
  })

  it('Enter с новым названием → один вызов Go с очищенным значением; номер обновился', async () => {
    const { container } = render(Timeline)
    const input = await edit(container, 0)
    input.value = '  Сборы невесты \n'
    await fireEvent.input(input)
    await fireEvent.keyDown(input, { key: 'Enter' })
    await vi.waitFor(() => expect(nums(container)).toEqual(['1 · Сборы невесты', '2 · Сборы']))
    expect(rename).toHaveBeenCalledExactlyOnceWith('a', 'Сборы невесты')
    expect(field(container)).toBeNull()
  })

  it('Enter без изменения — холостая правка: Go не зовётся', async () => {
    const { container } = render(Timeline)
    const input = await edit(container, 1)
    await fireEvent.keyDown(input, { key: 'Enter' })
    flushSync()
    expect(rename).not.toHaveBeenCalled()
    expect(field(container)).toBeNull()
  })

  it('потеря фокуса сохраняет правку (как Enter), Enter+blur — один вызов', async () => {
    const { container } = render(Timeline)
    const input = await edit(container, 0)
    input.value = 'Банкет'
    await fireEvent.input(input)
    await fireEvent.keyDown(input, { key: 'Enter' })
    await fireEvent.blur(input)
    expect(rename).toHaveBeenCalledTimes(1)
  })

  it('пустое название снимает имя', async () => {
    const { container } = render(Timeline)
    const input = await edit(container, 1)
    input.value = '   '
    await fireEvent.input(input)
    await fireEvent.keyDown(input, { key: 'Enter' })
    expect(rename).toHaveBeenCalledExactlyOnceWith('b', '')
  })

  it('уровень без функции «названия»: двойной щелчок поле не открывает', async () => {
    prefs.level = 'simple'
    const { container } = render(Timeline)
    await edit(container, 0)
    expect(field(container)).toBeNull()
  })

  it('Esc в поле не всплывает до страницы (не снимает выделение)', async () => {
    const page = vi.fn()
    window.addEventListener('keydown', page)
    const { container } = render(Timeline)
    const input = await edit(container, 0)
    await fireEvent.keyDown(input, { key: 'Escape' })
    expect(page).not.toHaveBeenCalled()
    window.removeEventListener('keydown', page)
  })
})
