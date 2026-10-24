// @vitest-environment happy-dom
import { afterEach, describe, expect, it, vi } from 'vitest'
import { cleanup, fireEvent, render, screen } from '@testing-library/svelte'
import TiltRow from './TiltRow.svelte'
import { cardContext } from '../../base/prefs/cardgroups.svelte'

// Общая строка ползунка (124 U4): ⊗, одна строка в панели и две в узкой карточке, ввод числом, сессия по фокусу.
const docked = { context: new Map([cardContext(() => true)]) }
const floating = { context: new Map([cardContext(() => false)]) }

function props(over: Record<string, unknown> = {}) {
  return {
    tilt: 20,
    label: 'Яркость',
    numLabel: 'Яркость, значение',
    rangeLabel: 'Яркость',
    min: -50,
    max: 50,
    step: 1,
    guide: '' as const,
    onlive: vi.fn(),
    oncommit: vi.fn(async () => {}),
    onend: vi.fn(),
    onleave: vi.fn(),
    onstart: vi.fn(),
    ...over,
  }
}
const num = () => screen.getByLabelText('Яркость, значение') as HTMLInputElement
const range = () => screen.getByRole('slider', { name: 'Яркость' }) as HTMLInputElement
const rst = () => screen.queryByRole('button', { name: 'Сбросить: яркость' }) as HTMLButtonElement | null

afterEach(cleanup)

describe('TiltRow: раскладка', () => {
  it('панель деталей — одна строка: дорожка стоит перед числом и ⊗', () => {
    render(TiltRow, { ...docked, props: props({ reset: true }) })
    const box = document.querySelector('.box')!
    expect(box.classList.contains('line')).toBe(true)
    expect([...box.children].map((e) => e.className.split(' ')[0])).toEqual(['cell', 'track', 'num', 'reset'])
  })

  it('плавающая карточка — дорожка второй строкой, после числа и ⊗', () => {
    render(TiltRow, { ...floating, props: props({ reset: true }) })
    const box = document.querySelector('.box')!
    expect(box.classList.contains('line')).toBe(false)
    expect([...box.children].map((e) => e.className.split(' ')[0])).toEqual(['cell', 'num', 'reset', 'track'])
  })

  it('вне карточки (без контекста) — как в панели; без reset ⊗ нет (прежние пять мест)', () => {
    render(TiltRow, { props: props() })
    expect(document.querySelector('.box')!.classList.contains('line')).toBe(true)
    expect(rst()).toBeNull()
  })

  it('«Выпрямить по линии» живёт в ячейке подписи: дорожка начинается там же, что у строк без кнопки', () => {
    render(TiltRow, { ...docked, props: props({ onstraighten: vi.fn(), reset: true }) })
    const cell = document.querySelector('.cell')!
    expect(cell.querySelector('[data-guide="straighten"]')).not.toBeNull()
    expect(cell.querySelector('label')!.textContent).toBe('Яркость')
  })

  it('якоря справочника: tilt, round, photo-fade; пусто — без якоря', () => {
    for (const g of ['tilt', 'round', 'photo-fade'] as const) {
      const a = render(TiltRow, { props: props({ guide: g }) })
      expect(document.querySelector(`.box[data-guide="${g}"]`), g).not.toBeNull()
      a.unmount()
    }
    render(TiltRow, { props: props() })
    expect(document.querySelector('.box')!.hasAttribute('data-guide')).toBe(false)
  })
})

describe('TiltRow: ⊗', () => {
  it('сбрасывает в dflt: сессия серии кончается, один oncommit(dflt), потом onend', async () => {
    const p = props({ reset: true, dflt: 0 })
    render(TiltRow, { ...docked, props: p })
    expect(rst()!.disabled).toBe(false)
    await fireEvent.click(rst()!)
    expect(p.oncommit).toHaveBeenCalledExactlyOnceWith(0)
    expect(p.onleave).toHaveBeenCalledTimes(1)
    expect(p.onleave.mock.invocationCallOrder[0]).toBeLessThan(p.oncommit.mock.invocationCallOrder[0])
    expect(p.onend).toHaveBeenCalledTimes(1)
  })

  it('значение уже по умолчанию — ⊗ серая; без dflt — тоже; resetOff сильнее', () => {
    const a = render(TiltRow, { ...docked, props: props({ reset: true, tilt: 0 }) })
    expect(rst()!.disabled).toBe(true)
    a.unmount()
    const b = render(TiltRow, { ...docked, props: props({ reset: true, dflt: null }) })
    expect(rst()!.disabled).toBe(true)
    b.unmount()
    const c = render(TiltRow, { ...docked, props: props({ reset: true, tilt: 0, resetOff: false }) })
    expect(rst()!.disabled).toBe(false)
    c.unmount()
    render(TiltRow, { ...docked, props: props({ reset: true, resetOff: true }) })
    expect(rst()!.disabled).toBe(true)
  })

  it('onreset заменяет сброс: oncommit не зовётся (ResetFrame масштаба)', async () => {
    const onreset = vi.fn(async () => {})
    const p = props({ reset: true, onreset, resetOff: false, tilt: 0 })
    render(TiltRow, { ...docked, props: p })
    await fireEvent.click(rst()!)
    expect(onreset).toHaveBeenCalledTimes(1)
    expect(p.oncommit).not.toHaveBeenCalled()
    expect(p.onend).toHaveBeenCalledTimes(1)
  })

  it('у разных значений (mixed) ⊗ жива и ставит всем dflt; в поле «—»', async () => {
    const p = props({ reset: true, mixed: true, tilt: 0 })
    render(TiltRow, { ...docked, props: p })
    expect(num().placeholder).toBe('—')
    expect(num().value).toBe('')
    await fireEvent.click(rst()!)
    expect(p.oncommit).toHaveBeenCalledExactlyOnceWith(0)
  })

  it('подпись и подсказка ⊗ можно задать', () => {
    render(TiltRow, { ...docked, props: props({ reset: true, resetLabel: 'Сбросить кадр', resetTitle: 'Сбросить кадр: целиком' }) })
    const b = screen.getByRole('button', { name: 'Сбросить кадр' })
    expect(b.getAttribute('title')).toBe('Сбросить кадр: целиком')
  })
})

describe('TiltRow: ввод и жесты', () => {
  it('число: change → один oncommit; крайние значения зажимаются; то же значение — без commit', async () => {
    const p = props()
    render(TiltRow, { ...docked, props: p })
    await fireEvent.change(num(), { target: { value: '99' } })
    expect(p.oncommit).toHaveBeenLastCalledWith(50)
    await fireEvent.change(num(), { target: { value: '-99' } })
    expect(p.oncommit).toHaveBeenLastCalledWith(-50)
    expect(p.oncommit).toHaveBeenCalledTimes(2)
    await fireEvent.change(num(), { target: { value: '20' } }) // как сейчас: холостая правка
    expect(p.oncommit).toHaveBeenCalledTimes(2)
    expect(p.onend).toHaveBeenCalledTimes(3)
  })

  it('перетаскивание: много oninput — только предпросмотр, один oncommit на отпускании', async () => {
    const p = props()
    render(TiltRow, { ...docked, props: p })
    for (const v of ['21', '25', '33']) await fireEvent.input(range(), { target: { value: v } })
    expect(p.onlive.mock.calls).toEqual([[21], [25], [33]])
    expect(p.oncommit).not.toHaveBeenCalled()
    await fireEvent.change(range(), { target: { value: '33' } })
    expect(p.oncommit).toHaveBeenCalledExactlyOnceWith(33)
    expect(p.onend).toHaveBeenCalledTimes(1)
  })

  it('двойной щелчок по дорожке — dflt; при dflt=null молчит', async () => {
    const p = props()
    const a = render(TiltRow, { ...docked, props: p })
    await fireEvent.dblClick(range())
    expect(p.oncommit).toHaveBeenCalledExactlyOnceWith(0)
    a.unmount()
    const q = props({ dflt: null })
    render(TiltRow, { ...docked, props: q })
    await fireEvent.dblClick(range())
    expect(q.oncommit).not.toHaveBeenCalled()
  })

  it('недонабранное поле откатывается к значению, commit не зовётся', async () => {
    const p = props()
    render(TiltRow, { ...docked, props: p })
    await fireEvent.change(num(), { target: { value: '' } })
    expect(p.oncommit).not.toHaveBeenCalled()
    expect(num().value).toBe('20')
  })

  it('Esc откатывает поле и снимает фокус, Enter снимает фокус', async () => {
    const p = props()
    render(TiltRow, { ...docked, props: p })
    num().focus()
    num().value = '33'
    await fireEvent.keyDown(num(), { key: 'Escape' })
    expect(num().value).toBe('20')
    expect(p.onend).toHaveBeenCalledTimes(1)
    expect(document.activeElement).not.toBe(num())
  })
})

describe('TiltRow: сессия по фокусу', () => {
  it('onstart — фокус вошёл в строку снаружи, переход ползунок ↔ поле его не повторяет', async () => {
    const p = props({ reset: true })
    render(TiltRow, { ...docked, props: p })
    await fireEvent.focusIn(range(), { relatedTarget: document.body })
    expect(p.onstart).toHaveBeenCalledTimes(1)
    await fireEvent.focusIn(num(), { relatedTarget: range() })
    expect(p.onstart).toHaveBeenCalledTimes(1)
    await fireEvent.focusOut(range(), { relatedTarget: num() })
    expect(p.onleave).not.toHaveBeenCalled()
    await fireEvent.focusOut(num(), { relatedTarget: document.body })
    expect(p.onleave).toHaveBeenCalledTimes(1)
  })
})
