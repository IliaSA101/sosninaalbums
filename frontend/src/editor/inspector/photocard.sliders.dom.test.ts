// @vitest-environment happy-dom
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { cleanup, fireEvent, render, screen } from '@testing-library/svelte'

// Ползунки карточки «Фото» (спека 124 U4): Масштаб, Наклон, Яркость, Контраст, Прозрачность — одна строка TiltRow
// с числом и ⊗. Один жест — один вызов Go (один шаг отмены), ⊗ — свой вызов, крайние значения зажимает строка.
const api = vi.hoisted(() => ({
  SetFrame: vi.fn(),
  ResetFrame: vi.fn(),
  SetTone: vi.fn(),
  SetPhotoFade: vi.fn(),
  SetFrameAngle: vi.fn(),
  Person: vi.fn(),
}))
vi.mock('../../../wailsjs/runtime/runtime', () => ({ EventsOn: () => () => {} }))
vi.mock('../../../wailsjs/go/app/App', async (orig) => {
  const all = await orig<Record<string, unknown>>()
  return { ...Object.fromEntries(Object.keys(all).map((k) => [k, vi.fn(async () => undefined)])), ...api }
})

const { default: InspectorFrame } = await import('./InspectorFrame.svelte')
const { store } = await import('../../base/core/store.svelte')
const { prefs } = await import('../../base/prefs/prefs.svelte')
const { placesel } = await import('../places/placesel.svelte')
const { cardGroups, cardContext } = await import('../../base/prefs/cardgroups.svelte')
const { framelive } = await import('../photo/framelive.svelte')
const { adjlive } = await import('../photo/adjlive.svelte')
const { borderlive } = await import('../border/borderlive.svelte')
const { borderAct } = await import('../border/borderact.svelte')

const docked = { context: new Map([cardContext(() => true)]) }
const floating = { context: new Map([cardContext(() => false)]) }
// Ответ Go — тот же вид с новой ревизией: карточка остаётся на месте, как после настоящей правки
const ok = async () => ({ ...store.view, rev: store.view.rev + 1 })

function setup(frame: object = {}, contain = false) {
  store.photos = [{ id: 'a', name: 'IMG_1.jpg', w: 2667, h: 4000 }] as never
  store.view = {
    ...store.view,
    minDPI: 200,
    spreads: [
      {
        id: 's1',
        grid: contain,
        photos: ['a'],
        places: [{ id: 'p', photo: 'a', x: 10, y: 10, w: 100, h: 100, shape: 'rect', round: 0, ...(contain ? { fit: 'contain' } : {}) }],
        frames: [
          {
            s: 1.2,
            x: 0.1,
            y: 0.2,
            img: { x: 0, y: 0, w: 100, h: 100 },
            dpi: 340,
            tilt: 2.5,
            bright: 10,
            contrast: 0,
            fade: 0.3,
            ...frame,
          },
        ],
        rects: [],
        captions: [],
      },
    ],
  } as never
  vi.spyOn(store, 'selectedCell', 'get').mockReturnValue('a')
  vi.spyOn(prefs, 'can').mockImplementation(() => true)
  placesel.refs = ['p']
}
// Правка дошла до конца: onend погасил предпросмотр (после ответа Go)
const settleFrame = () => vi.waitFor(() => expect(framelive.of('a')).toBeNull())
const settleTone = () => new Promise((r) => setTimeout(r, 0)) // ответ Go обработан, затвор строки открыт
const num = (n: string) => screen.getByLabelText(n) as HTMLInputElement
const slider = (n: string) => screen.getByRole('slider', { name: n }) as HTMLInputElement
const rst = (n: string) => screen.getByRole('button', { name: n }) as HTMLButtonElement

beforeEach(() => {
  localStorage.clear()
  cardGroups.reset()
  for (const f of [api.SetFrame, api.ResetFrame, api.SetTone, api.SetPhotoFade, api.SetFrameAngle]) f.mockImplementation(ok)
  api.Person.mockResolvedValue({ last: '', first: '', middle: '', subject: '', teacher: false, fromFile: false, place: false })
})
afterEach(() => {
  cleanup()
  placesel.refs = []
  framelive.clear()
  adjlive.clear()
  borderlive.release()
  borderAct.end()
  vi.clearAllMocks()
  vi.restoreAllMocks()
})

describe('Масштаб', () => {
  it('число: один SetFrame с масштабом; 999 зажимается в 400 %, 1 — в 100 %', async () => {
    setup()
    render(InspectorFrame, docked)
    await fireEvent.change(num('Масштаб кадра, процент'), { target: { value: '150' } })
    await settleFrame()
    expect(api.SetFrame).toHaveBeenCalledTimes(1)
    expect(api.SetFrame.mock.calls[0].slice(0, 2)).toEqual(['a', 1.5])
    await fireEvent.change(num('Масштаб кадра, процент'), { target: { value: '999' } })
    await settleFrame()
    expect(api.SetFrame.mock.calls[1][1]).toBe(4)
    await fireEvent.change(num('Масштаб кадра, процент'), { target: { value: '1' } })
    expect(api.SetFrame.mock.calls[2][1]).toBe(1)
  })

  it('перетаскивание: предпросмотр через framelive, один SetFrame на отпускании, просмотр гаснет', async () => {
    setup()
    render(InspectorFrame, docked)
    const r = slider('Масштаб кадра')
    for (const v of ['130', '160', '200']) await fireEvent.input(r, { target: { value: v } })
    expect(framelive.of('a')!.s).toBe(2)
    expect(framelive.fitting('a')).toBe(true) // на ходу виден силуэт под лицо (130)
    expect(api.SetFrame).not.toHaveBeenCalled()
    await fireEvent.change(r, { target: { value: '200' } })
    await settleFrame()
    expect(api.SetFrame).toHaveBeenCalledTimes(1)
    expect(framelive.fitting('a')).toBe(false)
  })

  it('силуэт гаснет сразу на отпускании, не дожидаясь ответа Go', async () => {
    setup()
    let done!: () => void
    api.SetFrame.mockImplementationOnce(() => new Promise((res) => (done = () => res(undefined))))
    render(InspectorFrame, docked)
    const r = slider('Масштаб кадра')
    await fireEvent.input(r, { target: { value: '200' } })
    expect(framelive.fitting('a')).toBe(true)
    void fireEvent.change(r, { target: { value: '200' } })
    await Promise.resolve()
    expect(api.SetFrame).toHaveBeenCalledTimes(1)
    expect(framelive.fitting('a')).toBe(false) // кадр ещё держится до ответа, но не как подгонка
    expect(framelive.of('a')!.s).toBe(2)
    done()
  })

  it('потянули и вернули к исходному: change не приходит, просмотр и силуэт всё равно гаснут', async () => {
    setup({ s: 1.2 })
    render(InspectorFrame, docked)
    const r = slider('Масштаб кадра')
    await fireEvent.input(r, { target: { value: '170' } })
    expect(framelive.fitting('a')).toBe(true)
    await fireEvent.input(r, { target: { value: '120' } })
    expect(framelive.fitting('a')).toBe(false)
    expect(framelive.of('a')).toBeNull()
    expect(api.SetFrame).not.toHaveBeenCalled()
  })

  it('⊗ зовёт ResetFrame один раз; на масштабе по умолчанию без своего кадра — серая', async () => {
    setup()
    render(InspectorFrame, docked)
    await fireEvent.click(rst('Сбросить кадр'))
    expect(api.ResetFrame).toHaveBeenCalledExactlyOnceWith('a')
    expect(api.SetFrame).not.toHaveBeenCalled()
    cleanup()
    setup({ s: 1, x: 0, y: 0 })
    render(InspectorFrame, docked)
    expect(rst('Сбросить кадр').disabled).toBe(true)
  })

  it('«Вписано целиком» — ползунка масштаба нет', () => {
    setup({}, true)
    render(InspectorFrame, docked)
    expect(screen.queryByRole('slider', { name: 'Масштаб кадра' })).toBeNull()
  })
})

describe('Яркость и Контраст', () => {
  it('число: один SetTone с обоими значениями; 99 → 50, −99 → −50', async () => {
    setup()
    render(InspectorFrame, docked)
    await fireEvent.change(num('Контраст, значение'), { target: { value: '99' } })
    await settleTone()
    expect(api.SetTone).toHaveBeenLastCalledWith('a', 10, 50)
    await fireEvent.change(num('Яркость, значение'), { target: { value: '-99' } })
    await settleTone()
    expect(api.SetTone).toHaveBeenLastCalledWith('a', -50, 0)
    expect(api.SetTone).toHaveBeenCalledTimes(2)
  })

  it('перетаскивание = один SetTone; до отпускания Go не зовётся, просмотр гаснет', async () => {
    setup()
    render(InspectorFrame, docked)
    const r = slider('Яркость')
    for (const v of ['11', '20', '31']) await fireEvent.input(r, { target: { value: v } })
    expect(adjlive.of('a')?.bright).toBe(31)
    expect(api.SetTone).not.toHaveBeenCalled()
    await fireEvent.change(r, { target: { value: '31' } })
    await settleTone()
    expect(api.SetTone).toHaveBeenCalledExactlyOnceWith('a', 31, 0)
  })

  it('⊗ — SetTone с нулём в своей строке; строка в нуле — серая и ничего не зовёт', async () => {
    setup()
    render(InspectorFrame, docked)
    expect(rst('Сбросить: контраст').disabled).toBe(true)
    await fireEvent.click(rst('Сбросить: яркость'))
    expect(api.SetTone).toHaveBeenCalledExactlyOnceWith('a', 0, 0)
  })

  it('возврат к исходному значению — холостая правка: Go не зовётся, просмотр гаснет', async () => {
    setup()
    render(InspectorFrame, docked)
    const r = slider('Яркость')
    await fireEvent.input(r, { target: { value: '25' } })
    await fireEvent.input(r, { target: { value: '10' } })
    await fireEvent.change(r, { target: { value: '10' } })
    expect(api.SetTone).not.toHaveBeenCalled()
    expect(adjlive.of('a')).toBeNull()
  })
})

describe('Прозрачность', () => {
  it('число: один SetPhotoFade долей; фокус в строке открывает серию, уход закрывает', async () => {
    setup()
    render(InspectorFrame, docked)
    await fireEvent.focusIn(num('Прозрачность фото, процент'), { relatedTarget: document.body })
    await fireEvent.change(num('Прозрачность фото, процент'), { target: { value: '40' } })
    expect(api.SetPhotoFade).toHaveBeenCalledTimes(1)
    const [ids, fade, merge] = api.SetPhotoFade.mock.calls[0]
    expect([ids, fade]).toEqual([['a'], 0.4])
    expect(merge).not.toBe('') // стрелки и протяжки в одном фокусе сливаются в шаг
    await fireEvent.focusOut(num('Прозрачность фото, процент'), { relatedTarget: document.body })
    expect(borderAct.session.merge('fade:s1:a')).toBe('')
  })

  it('150 зажимается в 100 %', async () => {
    setup()
    render(InspectorFrame, docked)
    await fireEvent.change(num('Прозрачность фото, процент'), { target: { value: '150' } })
    expect(api.SetPhotoFade.mock.calls[0][1]).toBe(1)
  })

  it('⊗ — отдельный шаг без серии (merge пустой), значение 0', async () => {
    setup()
    render(InspectorFrame, docked)
    await fireEvent.focusIn(slider('Прозрачность фото, %'), { relatedTarget: document.body })
    await fireEvent.click(rst('Сбросить: прозрачность'))
    expect(api.SetPhotoFade).toHaveBeenCalledExactlyOnceWith(['a'], 0, '')
  })

  it('прозрачность 0 — ⊗ серая', () => {
    setup({ fade: 0 })
    render(InspectorFrame, docked)
    expect(rst('Сбросить: прозрачность').disabled).toBe(true)
  })
})

describe('Наклон: протяжка', () => {
  it('потянули и вернули к исходному: сетка (framelive) гаснет без change', async () => {
    setup({ tilt: 2.5 })
    render(InspectorFrame, docked)
    const r = slider('Наклон')
    await fireEvent.input(r, { target: { value: '5' } })
    framelive.set('a', { s: 1.2, x: 0.1, y: 0.2, r: 5 }, true) // кадр на ходу считает liveTilt по геометрии; здесь — как будто посчитал
    expect(framelive.tilting('a')).toBe(true)
    await fireEvent.input(r, { target: { value: '2.5' } })
    expect(framelive.tilting('a')).toBe(false)
    expect(framelive.of('a')).toBeNull()
    expect(api.SetFrameAngle).not.toHaveBeenCalled()
  })
})

describe('Наклон: ⊗', () => {
  it('сбрасывает в 0° одним SetFrameAngle; при 0° — серая', async () => {
    setup()
    render(InspectorFrame, docked)
    await fireEvent.click(rst('Сбросить: наклон'))
    expect(api.SetFrameAngle).toHaveBeenCalledTimes(1)
    expect(api.SetFrameAngle.mock.calls[0].slice(0, 3)).toEqual(['s1', 'a', 0])
    cleanup()
    setup({ tilt: 0 })
    render(InspectorFrame, docked)
    expect(rst('Сбросить: наклон').disabled).toBe(true)
  })
})

describe('Вид строк: панель и узкая карточка', () => {
  const ROWS = 5 // Масштаб, Наклон, Яркость, Контраст, Прозрачность

  it('панель деталей: все пять — одной строкой, у каждой ⊗', () => {
    setup()
    render(InspectorFrame, docked)
    expect(document.querySelectorAll('.box.line')).toHaveLength(ROWS)
    expect(document.querySelectorAll('.box .reset')).toHaveLength(ROWS)
  })

  it('плавающая карточка: дорожка второй строкой у всех пяти, ⊗ на месте', () => {
    setup()
    render(InspectorFrame, floating)
    expect(document.querySelectorAll('.box')).toHaveLength(ROWS)
    expect(document.querySelectorAll('.box.line')).toHaveLength(0)
    expect(document.querySelectorAll('.box .reset')).toHaveLength(ROWS)
  })

  it('уровень без «Цвета и тона» и «Кадра»: строк нет вовсе', () => {
    setup()
    vi.spyOn(prefs, 'can').mockImplementation(() => false)
    render(InspectorFrame, docked)
    expect(document.querySelectorAll('.box')).toHaveLength(0)
  })

  it('один кадр без прозрачности и тона (уровень «frame»): остаются Масштаб; Наклон скрыт', () => {
    setup()
    vi.spyOn(prefs, 'can').mockImplementation((f) => f === 'frame')
    render(InspectorFrame, docked)
    expect(document.querySelectorAll('.box')).toHaveLength(1)
    expect(screen.queryByRole('slider', { name: 'Масштаб кадра' })).not.toBeNull()
  })
})
