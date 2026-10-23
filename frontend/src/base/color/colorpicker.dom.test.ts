// @vitest-environment happy-dom
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { cleanup, fireEvent, render, screen } from '@testing-library/svelte'
import { flushSync, tick } from 'svelte'

// Окно цвета (109, вариант 1): наборы вкладками, «Свои», «С фото», пипетка, свой оттенок; живой показ при наведении
// и фокусе, один onpick на выбор, показ держится до ответа Go.
const api = vi.hoisted(() => ({
  MyColors: vi.fn(),
  ImportColors: vi.fn(),
  RememberColor: vi.fn(),
  ForgetColor: vi.fn(),
  SpreadPhotoColors: vi.fn(),
}))
vi.mock('../../../wailsjs/go/app/App', () => api)

const { default: ColorPicker } = await import('./ColorPicker.svelte')
const { mycolors } = await import('./mycolors.svelte')
const { eyedrop } = await import('./eyedrop.svelte')
const { hsvToHex } = await import('./palette')
const { tip } = await import('../ui/tip.svelte')

beforeEach(async () => {
  localStorage.clear()
  api.MyColors.mockResolvedValue([])
  api.RememberColor.mockImplementation(async (h: string) => [h])
  api.ForgetColor.mockResolvedValue([])
  api.SpreadPhotoColors.mockResolvedValue([])
  await mycolors.load()
  mycolors.list = []
  api.MyColors.mockClear()
})
afterEach(() => {
  cleanup()
  eyedrop.cancel()
  tip.hide()
  vi.clearAllMocks()
  vi.restoreAllMocks()
})

type Props = { onpick?: (h: string) => unknown; onpreview?: (h: string | null) => void; sheet?: string; value?: string }
async function open(props: Props = {}) {
  const onpick = props.onpick ?? vi.fn()
  const onpreview = props.onpreview ?? vi.fn()
  const r = render(ColorPicker, { value: props.value ?? '#ffffff', label: 'Цвет фона', onpick, onpreview, sheet: props.sheet })
  await fireEvent.click(screen.getByRole('button', { name: 'Цвет фона' }))
  flushSync()
  return { onpick, onpreview, ...r }
}
const chip = (hex: string) => screen.getAllByRole('button', { name: new RegExp(hex) })[0]
const tabBtn = (title: string) => screen.getByRole('tab', { name: title })
const selected = () => screen.getByRole('tab', { selected: true }).textContent

describe('ColorPicker: живой показ', () => {
  it('наведение — onpreview, уход — onpreview(null)', async () => {
    const { onpreview } = await open()
    await fireEvent.pointerEnter(chip('#4a4643'))
    expect(onpreview).toHaveBeenLastCalledWith('#4a4643')
    await fireEvent.pointerLeave(chip('#4a4643'))
    expect(onpreview).toHaveBeenLastCalledWith(null)
    expect(onpreview).toHaveBeenCalledTimes(2)
  })

  it('фокус с клавиатуры — тоже показ', async () => {
    const { onpreview } = await open()
    await fireEvent.focus(chip('#1f1d1c'))
    expect(onpreview).toHaveBeenLastCalledWith('#1f1d1c')
    await fireEvent.blur(chip('#1f1d1c'))
    expect(onpreview).toHaveBeenLastCalledWith(null)
  })

  it('наведение: шапка называет цвет, подсказка — «Имя · #hex»', async () => {
    await open()
    await fireEvent.pointerEnter(chip('#4a4643'))
    expect(screen.getByRole('dialog').textContent).toContain('Графит')
    expect(tip.state?.text).toBe('Графит · #4a4643')
  })

  it('щелчок — один onpick, окно закрыто, цвет запомнен; показ держится до ответа', async () => {
    let answer!: () => void
    const onpick = vi.fn(() => new Promise<void>((r) => (answer = r)))
    const { onpreview } = await open({ onpick })
    const el = chip('#4a4643')
    await fireEvent.pointerEnter(el)
    await fireEvent.click(el)
    expect(onpick).toHaveBeenCalledExactlyOnceWith('#4a4643')
    expect(api.RememberColor).toHaveBeenCalledExactlyOnceWith('#4a4643')
    expect(screen.queryByRole('dialog')).toBeNull()
    // Окно закрылось, мышь «ушла» с образца — показ не снимается, старый цвет не мелькает.
    await fireEvent.pointerLeave(el)
    await fireEvent.blur(el)
    expect(onpreview).not.toHaveBeenCalledWith(null)
    answer()
    await tick()
    await Promise.resolve()
    expect(onpreview).toHaveBeenLastCalledWith(null)
  })

  it('закрытие без выбора (Esc) — onpreview(null), onpick не зовётся', async () => {
    const { onpick, onpreview } = await open()
    await fireEvent.pointerEnter(chip('#4a4643'))
    await fireEvent.keyDown(document.body, { key: 'Escape' })
    flushSync()
    expect(screen.queryByRole('dialog')).toBeNull()
    expect(onpreview).toHaveBeenLastCalledWith(null)
    expect(onpick).not.toHaveBeenCalled()
  })

  it('щелчок мимо окна — показ снят', async () => {
    const { onpick, onpreview } = await open()
    await fireEvent.pointerEnter(chip('#4a4643'))
    await fireEvent.pointerDown(document.body)
    flushSync()
    expect(screen.queryByRole('dialog')).toBeNull()
    expect(onpreview).toHaveBeenLastCalledWith(null)
    expect(onpick).not.toHaveBeenCalled()
  })

  it('окно размонтировали во время показа — показ снят', async () => {
    const { onpreview, unmount } = await open()
    await fireEvent.pointerEnter(chip('#4a4643'))
    unmount()
    expect(onpreview).toHaveBeenLastCalledWith(null)
  })

  it('без onpreview окно работает как прежде', async () => {
    const onpick = vi.fn()
    render(ColorPicker, { value: '', onpick })
    await fireEvent.click(screen.getByRole('button', { name: 'Цвет' }))
    await fireEvent.pointerEnter(chip('#4a4643'))
    await fireEvent.click(chip('#4a4643'))
    expect(onpick).toHaveBeenCalledExactlyOnceWith('#4a4643')
  })
})

describe('ColorPicker: HEX', () => {
  it('правильный HEX во время ввода — показ; неверный — нет; Enter — один onpick', async () => {
    const { onpick, onpreview } = await open()
    const field = screen.getByLabelText('Цвет в формате HEX')
    await fireEvent.input(field, { target: { value: '#12ab' } })
    expect(onpreview).not.toHaveBeenCalled()
    await fireEvent.input(field, { target: { value: '#12AB34' } })
    expect(onpreview).toHaveBeenLastCalledWith('#12ab34')
    await fireEvent.keyDown(field, { key: 'Enter' })
    expect(onpick).toHaveBeenCalledExactlyOnceWith('#12ab34')
  })

  it('неверный HEX по Enter — ошибка и ни одной правки', async () => {
    const { onpick } = await open()
    const field = screen.getByLabelText('Цвет в формате HEX')
    await fireEvent.input(field, { target: { value: 'мусор' } })
    await fireEvent.keyDown(field, { key: 'Enter' })
    expect(screen.getByRole('alert').textContent).toContain('#rrggbb')
    expect(onpick).not.toHaveBeenCalled()
  })

  it('«zzz» помечен ошибкой сразу; неполный «#12a» — нет; правильный ввод снимает', async () => {
    const { onpick } = await open()
    const field = screen.getByLabelText('Цвет в формате HEX')
    await fireEvent.input(field, { target: { value: '#12a' } })
    expect(field.getAttribute('aria-invalid')).toBeNull()
    expect(field.classList.contains('bad')).toBe(false)
    await fireEvent.input(field, { target: { value: 'zzz' } })
    expect(field.getAttribute('aria-invalid')).toBe('true')
    expect(field.classList.contains('bad')).toBe(true)
    expect(screen.getByRole('alert').textContent).toContain('#rrggbb')
    await fireEvent.input(field, { target: { value: '#12ab34' } })
    expect(field.getAttribute('aria-invalid')).toBeNull()
    expect(screen.queryByRole('alert')).toBeNull()
    expect(onpick).not.toHaveBeenCalled()
  })

  it('неполный HEX и уход фокуса внутри окна — помечен, ничего не выбрано; пустое поле — не ошибка', async () => {
    const { onpick } = await open()
    const field = screen.getByLabelText('Цвет в формате HEX')
    await fireEvent.input(field, { target: { value: '#12a' } })
    await fireEvent.blur(field, { relatedTarget: screen.getByRole('button', { name: /Свой оттенок/ }) })
    expect(field.getAttribute('aria-invalid')).toBe('true')
    await fireEvent.input(field, { target: { value: '' } })
    await fireEvent.blur(field, { relatedTarget: screen.getByRole('button', { name: /Свой оттенок/ }) })
    expect(field.getAttribute('aria-invalid')).toBeNull()
    expect(onpick).not.toHaveBeenCalled()
  })

  it('уход из поля без правки не выбирает и не закрывает окно', async () => {
    const { onpick } = await open({ value: '#123456' })
    const field = screen.getByLabelText('Цвет в формате HEX')
    expect((field as HTMLInputElement).value).toBe('#123456')
    await fireEvent.blur(field)
    expect(onpick).not.toHaveBeenCalled()
    expect(screen.getByRole('dialog')).toBeTruthy()
  })
})

describe('ColorPicker: вкладки', () => {
  it('шесть вкладок с sheet, пять без; первая — «Нейтральные», у каждой 8 образцов', async () => {
    await open({ sheet: 's1' })
    expect(screen.getAllByRole('tab').map((t) => t.textContent)).toEqual([
      'Нейтральные',
      'Пастель',
      'Пудра и нюд',
      'Шампань',
      'Природа',
      'С фото',
    ])
    expect(selected()).toBe('Нейтральные')
    expect(screen.getByRole('tabpanel').querySelectorAll('.chip')).toHaveLength(8)
    cleanup()
    await open()
    expect(screen.getAllByRole('tab')).toHaveLength(5)
    expect(screen.queryByRole('tab', { name: 'С фото' })).toBeNull()
  })

  it('вкладка помнится между открытиями', async () => {
    await open()
    await fireEvent.click(tabBtn('Природа'))
    expect(selected()).toBe('Природа')
    expect(chip('#b8c4a9')).toBeTruthy()
    await fireEvent.keyDown(document.body, { key: 'Escape' })
    flushSync()
    await fireEvent.click(screen.getByRole('button', { name: 'Цвет фона' }))
    flushSync()
    expect(selected()).toBe('Природа')
    expect(localStorage.getItem('colorTab')).toBe('nature')
  })

  it('запомнена «С фото», а листа нет — открывается первая', async () => {
    localStorage.setItem('colorTab', 'photo')
    await open()
    expect(selected()).toBe('Нейтральные')
  })

  it('← → по вкладкам по кругу; повторный щелчок по выбранной ничего не ломает', async () => {
    await open()
    await fireEvent.keyDown(screen.getByRole('tablist'), { key: 'ArrowLeft' })
    await tick()
    expect(selected()).toBe('Природа')
    await fireEvent.keyDown(screen.getByRole('tablist'), { key: 'ArrowRight' })
    await tick()
    expect(selected()).toBe('Нейтральные')
    await fireEvent.click(tabBtn('Нейтральные'))
    expect(selected()).toBe('Нейтральные')
  })

  it('«С фото»: цвета Go', async () => {
    api.SpreadPhotoColors.mockResolvedValue(['#112233', '#445566'])
    await open({ sheet: 's1' })
    await fireEvent.click(tabBtn('С фото'))
    await tick()
    await tick()
    expect(api.SpreadPhotoColors).toHaveBeenCalledExactlyOnceWith('s1')
    expect(chip('#112233').getAttribute('aria-label')).toBe('С фото · #112233')
    expect(screen.getByRole('tabpanel').querySelectorAll('.chip')).toHaveLength(2)
  })

  it('«С фото» без фото на листе — «На развороте нет фото»', async () => {
    await open({ sheet: 's1' })
    await fireEvent.click(tabBtn('С фото'))
    await tick()
    await tick()
    expect(screen.getByRole('tabpanel').textContent).toContain('На развороте нет фото')
  })

  it('«С фото»: Go отказал — сообщение, а не пустота', async () => {
    api.SpreadPhotoColors.mockRejectedValue(new Error('нет листа'))
    await open({ sheet: 's1' })
    await fireEvent.click(tabBtn('С фото'))
    await tick()
    await tick()
    expect(screen.getByRole('tabpanel').textContent).toContain('Не получилось')
  })
})

describe('ColorPicker: свои цвета', () => {
  it('«+» добавляет текущий цвет', async () => {
    await open({ value: '#abcdef' })
    await fireEvent.click(screen.getByRole('button', { name: 'Добавить текущий цвет в свои' }))
    expect(api.RememberColor).toHaveBeenCalledExactlyOnceWith('#abcdef')
    flushSync()
    expect(screen.getByRole('dialog')).toBeTruthy() // «+» окно не закрывает
  })

  it('«+» серый, когда цвет уже в своих: подсказка «Уже в своих цветах»; после добавления — тоже', async () => {
    mycolors.list = ['#abcdef']
    await open({ value: '#abcdef' })
    const plus = screen.getByRole('button', { name: 'Уже в своих цветах' })
    expect(plus.getAttribute('aria-disabled')).toBe('true')
    expect(plus.hasAttribute('disabled')).toBe(false) // подсказка видна только у не-disabled
    await fireEvent.pointerEnter(plus)
    expect(tip.state?.text).toBe('Уже в своих цветах')
    await fireEvent.click(plus)
    expect(api.RememberColor).not.toHaveBeenCalled()
    cleanup()
    mycolors.list = []
    await open({ value: '#abcdef' })
    await fireEvent.click(screen.getByRole('button', { name: 'Добавить текущий цвет в свои' }))
    await tick()
    expect(screen.getByRole('button', { name: 'Уже в своих цветах' }).getAttribute('aria-disabled')).toBe('true')
  })

  it('образец набора после щелчка попадает в «Свои» (109-qa-1)', async () => {
    await open()
    await fireEvent.click(chip('#4a4643'))
    await tick()
    await fireEvent.click(screen.getByRole('button', { name: 'Цвет фона' }))
    flushSync()
    expect(mycolors.list).toEqual(['#4a4643'])
    expect(screen.getAllByRole('button', { name: /Графит/ })).toHaveLength(2) // в наборе и в «Своих»
  })

  it('«+» серый, пока цвет не задан', async () => {
    await open({ value: '' })
    const plus = screen.getByRole('button', { name: 'Добавить текущий цвет в свои' })
    expect(plus.getAttribute('aria-disabled')).toBe('true')
    await fireEvent.click(plus)
    expect(api.RememberColor).not.toHaveBeenCalled() // щелчок по серой «+» ничего не делает
  })

  it('правый щелчок убирает свой цвет, обычный образец — нет', async () => {
    mycolors.list = ['#123456']
    await open()
    await fireEvent.contextMenu(chip('#4a4643'))
    expect(api.ForgetColor).not.toHaveBeenCalled()
    await fireEvent.contextMenu(chip('#123456'))
    expect(api.ForgetColor).toHaveBeenCalledExactlyOnceWith('#123456')
  })

  it('Delete на своём образце убирает его', async () => {
    mycolors.list = ['#123456']
    await open()
    await fireEvent.keyDown(chip('#4a4643'), { key: 'Delete' })
    expect(api.ForgetColor).not.toHaveBeenCalled()
    await fireEvent.keyDown(chip('#123456'), { key: 'Delete' })
    expect(api.ForgetColor).toHaveBeenCalledExactlyOnceWith('#123456')
  })

  it('щелчок по своему цвету — обычный выбор', async () => {
    mycolors.list = ['#123456']
    const { onpick } = await open()
    await fireEvent.click(chip('#123456'))
    expect(onpick).toHaveBeenCalledExactlyOnceWith('#123456')
    expect(screen.queryByRole('dialog')).toBeNull()
  })
})

describe('ColorPicker: свой оттенок', () => {
  // Полотна в happy-dom без размеров: задаём 250×110 и 250×12 от нуля.
  function sized() {
    vi.spyOn(HTMLCanvasElement.prototype, 'getBoundingClientRect').mockImplementation(function (this: HTMLElement) {
      const h = this.classList.contains('hue') ? 12 : 110
      return { left: 0, top: 0, width: 250, height: h, right: 250, bottom: h, x: 0, y: 0, toJSON: () => ({}) }
    })
  }
  const field = (cls: string) => document.querySelector<HTMLCanvasElement>(`canvas.${cls}`)!

  it('поле раскрывается кнопкой «Свой оттенок» и сворачивается ею же', async () => {
    await open()
    expect(document.querySelector('canvas')).toBeNull()
    await fireEvent.click(screen.getByRole('button', { name: /Свой оттенок/ }))
    expect(document.querySelectorAll('canvas')).toHaveLength(2)
    await fireEvent.click(screen.getByRole('button', { name: /Свой оттенок/ }))
    expect(document.querySelector('canvas')).toBeNull()
  })

  it('протяжка — onpreview на ходу, onpick один раз на отпускании, окно открыто', async () => {
    sized()
    const { onpick, onpreview } = await open()
    await fireEvent.click(screen.getByRole('button', { name: /Свой оттенок/ }))
    const sv = field('sv')
    await fireEvent.pointerDown(sv, { clientX: 125, clientY: 55, button: 0 })
    await fireEvent.pointerMove(sv, { clientX: 250, clientY: 0 })
    expect(onpreview).toHaveBeenCalledTimes(2)
    expect(onpreview).toHaveBeenNthCalledWith(1, hsvToHex({ h: 0, s: 0.5, v: 0.5 }))
    expect(onpreview).toHaveBeenLastCalledWith('#ff0000')
    expect(onpick).not.toHaveBeenCalled()
    await fireEvent.pointerUp(sv, { clientX: 250, clientY: 0 })
    expect(onpick).toHaveBeenCalledExactlyOnceWith('#ff0000')
    expect(screen.getByRole('dialog')).toBeTruthy()
    expect((screen.getByLabelText('Цвет в формате HEX') as HTMLInputElement).value).toBe('#ff0000')
  })

  it('полоса оттенка: отпускание — тоже один onpick', async () => {
    sized()
    const { onpick, onpreview } = await open({ value: '#ff0000' })
    await fireEvent.click(screen.getByRole('button', { name: /Свой оттенок/ }))
    const hue = field('hue')
    await fireEvent.pointerDown(hue, { clientX: 125, clientY: 6, button: 0 })
    await fireEvent.pointerUp(hue, { clientX: 125, clientY: 6 })
    expect(onpreview).toHaveBeenCalledWith('#00ffff')
    expect(onpick).toHaveBeenCalledExactlyOnceWith('#00ffff')
  })

  it('вторая протяжка до ответа Go: ответ первой её показ не снимает', async () => {
    sized()
    let answer!: () => void
    const onpick = vi.fn(() => new Promise<void>((r) => (answer = r)))
    const onpreview = vi.fn()
    await open({ onpick, onpreview })
    await fireEvent.click(screen.getByRole('button', { name: /Свой оттенок/ }))
    const sv = field('sv')
    await fireEvent.pointerDown(sv, { clientX: 250, clientY: 0, button: 0 })
    await fireEvent.pointerUp(sv)
    await fireEvent.pointerDown(sv, { clientX: 125, clientY: 0, button: 0 })
    onpreview.mockClear()
    answer()
    await tick()
    await Promise.resolve()
    expect(onpreview).not.toHaveBeenCalledWith(null)
    await fireEvent.pointerUp(sv)
    expect(onpick).toHaveBeenCalledTimes(2)
  })

  it('движение мыши без нажатия ничего не показывает и не выбирает', async () => {
    sized()
    const { onpick, onpreview } = await open()
    await fireEvent.click(screen.getByRole('button', { name: /Свой оттенок/ }))
    await fireEvent.pointerMove(field('sv'), { clientX: 100, clientY: 50 })
    await fireEvent.pointerUp(field('sv'))
    expect(onpreview).not.toHaveBeenCalled()
    expect(onpick).not.toHaveBeenCalled()
  })

  it('прерванный жест — показ снят, правки нет', async () => {
    sized()
    const { onpick, onpreview } = await open()
    await fireEvent.click(screen.getByRole('button', { name: /Свой оттенок/ }))
    await fireEvent.pointerDown(field('sv'), { clientX: 100, clientY: 50, button: 0 })
    await fireEvent.pointerCancel(field('sv'))
    expect(onpreview).toHaveBeenLastCalledWith(null)
    expect(onpick).not.toHaveBeenCalled()
  })

  it('окно целиком в пределах экрана и с полем оттенка', async () => {
    vi.spyOn(HTMLElement.prototype, 'offsetWidth', 'get').mockReturnValue(272)
    vi.spyOn(HTMLElement.prototype, 'offsetHeight', 'get').mockReturnValue(640)
    window.innerWidth = 1280
    window.innerHeight = 760
    sized()
    await open()
    await fireEvent.click(screen.getByRole('button', { name: /Свой оттенок/ }))
    flushSync()
    const pop = screen.getByRole('dialog')
    expect(pop.querySelectorAll('canvas')).toHaveLength(2)
    const left = parseFloat(pop.style.left)
    const top = parseFloat(pop.style.top)
    expect(left).toBeGreaterThanOrEqual(0)
    expect(left + 272).toBeLessThanOrEqual(1280)
    expect(top).toBeGreaterThanOrEqual(0)
    expect(top + 640).toBeLessThanOrEqual(760)
  })
})

describe('ColorPicker: «Взять цвет с фото»', () => {
  it('без листа кнопки нет', async () => {
    await open()
    expect(screen.queryByRole('button', { name: /Взять цвет с фото/ })).toBeNull()
  })

  it('кнопка запускает режим; подпись — как на полосе над холстом (109 п. 6)', async () => {
    const { onpreview } = await open({ sheet: 's1' })
    await fireEvent.pointerEnter(chip('#4a4643'))
    expect(screen.getByRole('button', { name: /Взять цвет с фото/ }).textContent).toContain(
      'Щёлкните по развороту — возьмём цвет, каким он выйдет в печати',
    )
    await fireEvent.click(screen.getByRole('button', { name: /Взять цвет с фото/ }))
    expect(eyedrop.on).toBe(true)
    expect(eyedrop.sheet).toBe('s1') // слой на холсте встаёт над этим листом
    expect(onpreview).toHaveBeenLastCalledWith(null)
  })

  it('щелчок мимо в режиме — выход из режима, окно открыто, событие до выделения не доходит (109 п. 4)', async () => {
    await open({ sheet: 's1' })
    await fireEvent.click(screen.getByRole('button', { name: /Взять цвет с фото/ }))
    const seen = vi.fn()
    window.addEventListener('pointerdown', seen) // всплытие: так снимается выделение на листе
    window.addEventListener('click', seen)
    await fireEvent.pointerDown(document.body)
    await fireEvent.click(document.body) // click того же щелчка тоже гасится
    flushSync()
    expect(screen.getByRole('dialog')).toBeTruthy()
    expect(eyedrop.on).toBe(false)
    expect(seen).not.toHaveBeenCalled()
    await fireEvent.pointerDown(document.body) // следующий щелчок — обычный: «мимо» закрывает окно
    await fireEvent.click(document.body)
    flushSync()
    expect(seen).toHaveBeenCalled()
    expect(screen.queryByRole('dialog')).toBeNull()
    window.removeEventListener('pointerdown', seen)
    window.removeEventListener('click', seen)
  })

  it('щелчок по слою пипетки в режиме — не «мимо»: режим идёт, слой берёт цвет сам', async () => {
    await open({ sheet: 's1' })
    await fireEvent.click(screen.getByRole('button', { name: /Взять цвет с фото/ }))
    const layer = document.createElement('div')
    layer.dataset.eyedrop = ''
    document.body.appendChild(layer)
    await fireEvent.pointerDown(layer)
    flushSync()
    expect(eyedrop.on).toBe(true)
    expect(screen.getByRole('dialog')).toBeTruthy()
    layer.remove()
  })

  it('щелчок мимо без режима по-прежнему закрывает окно', async () => {
    await open({ sheet: 's1' })
    await fireEvent.pointerDown(document.body)
    flushSync()
    expect(screen.queryByRole('dialog')).toBeNull()
  })

  it('Esc в режиме — выход из режима, окно открыто; второй Esc — закрыто', async () => {
    await open({ sheet: 's1' })
    await fireEvent.click(screen.getByRole('button', { name: /Взять цвет с фото/ }))
    await fireEvent.keyDown(document.body, { key: 'Escape' })
    flushSync()
    expect(eyedrop.on).toBe(false)
    expect(screen.getByRole('dialog')).toBeTruthy()
    await fireEvent.keyDown(document.body, { key: 'Escape' })
    flushSync()
    expect(screen.queryByRole('dialog')).toBeNull()
  })

  it('повторное нажатие кнопки выходит из режима', async () => {
    await open({ sheet: 's1' })
    const b = screen.getByRole('button', { name: /Взять цвет с фото/ })
    await fireEvent.click(b)
    await fireEvent.click(b)
    expect(eyedrop.on).toBe(false)
    expect(screen.getByRole('dialog')).toBeTruthy()
  })

  it('цвет взят — один onpick, «Свои», окно закрыто, режима нет', async () => {
    const { onpick } = await open({ sheet: 's1' })
    await fireEvent.click(screen.getByRole('button', { name: /Взять цвет с фото/ }))
    eyedrop.finish('#A1B2C3')
    flushSync()
    expect(onpick).toHaveBeenCalledExactlyOnceWith('#a1b2c3')
    expect(api.RememberColor).toHaveBeenCalledExactlyOnceWith('#a1b2c3')
    expect(screen.queryByRole('dialog')).toBeNull()
    expect(eyedrop.on).toBe(false)
  })

  it('окно закрыли кнопкой-образцом или размонтировали — режим снят', async () => {
    const { unmount } = await open({ sheet: 's1' })
    await fireEvent.click(screen.getByRole('button', { name: /Взять цвет с фото/ }))
    await fireEvent.click(screen.getByRole('button', { name: 'Цвет фона' }))
    flushSync()
    expect(eyedrop.on).toBe(false)
    await fireEvent.click(screen.getByRole('button', { name: 'Цвет фона' }))
    flushSync()
    await fireEvent.click(screen.getByRole('button', { name: /Взять цвет с фото/ }))
    unmount()
    expect(eyedrop.on).toBe(false)
  })
})

describe('ColorPicker: клавиши окна не доходят до команд программы', () => {
  // Команды программы (удалить выделенное, листать развороты) слушает App на window в фазе всплытия.
  const app = vi.fn()
  beforeEach(() => window.addEventListener('keydown', app))
  afterEach(() => {
    window.removeEventListener('keydown', app)
    app.mockClear()
  })

  it('Delete на образце набора и «Свои» — до App не доходит', async () => {
    mycolors.list = ['#123456']
    await open({ sheet: 's1' })
    await fireEvent.keyDown(chip('#4a4643'), { key: 'Delete' })
    await fireEvent.keyDown(chip('#123456'), { key: 'Delete' })
    expect(app).not.toHaveBeenCalled()
  })

  it('Backspace и стрелки на образце, вкладке и «+» — до App не доходят', async () => {
    await open({ value: '#abcdef', sheet: 's1' })
    for (const el of [chip('#4a4643'), tabBtn('С фото'), screen.getByRole('button', { name: 'Добавить текущий цвет в свои' })]) {
      await fireEvent.keyDown(el, { key: 'Backspace' })
      await fireEvent.keyDown(el, { key: 'ArrowDown' })
      await fireEvent.keyDown(el, { key: 'Delete' })
    }
    expect(app).not.toHaveBeenCalled()
  })

  it('Esc по-прежнему ведёт окно: закрывает его', async () => {
    await open()
    await fireEvent.keyDown(chip('#4a4643'), { key: 'Escape' })
    flushSync()
    expect(screen.queryByRole('dialog')).toBeNull()
  })

  it('вне окна клавиши идут как обычно', async () => {
    await open()
    await fireEvent.keyDown(document.body, { key: 'Delete' })
    expect(app).toHaveBeenCalledTimes(1)
  })
})
