// @vitest-environment happy-dom
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { cleanup, fireEvent, render, screen } from '@testing-library/svelte'
import { flushSync, tick } from 'svelte'
import MenuList from './MenuList.svelte'
import type { MenuNode } from './menulist'

// happy-dom не считает раскладку: размеры списка и якорей подставляем сами.
const LIST_W = 200
const LIST_H = 300
let rects: Record<string, { left: number; top: number; width: number; height: number }> = {}

beforeEach(() => {
  vi.spyOn(HTMLElement.prototype, 'clientWidth', 'get').mockReturnValue(LIST_W)
  vi.spyOn(HTMLElement.prototype, 'clientHeight', 'get').mockReturnValue(LIST_H)
  vi.spyOn(HTMLElement.prototype, 'getBoundingClientRect').mockImplementation(function (this: HTMLElement) {
    const r = rects[this.dataset.item ?? ''] ?? { left: 0, top: 0, width: LIST_W, height: 28 }
    return { ...r, right: r.left + r.width, bottom: r.top + r.height, x: r.left, y: r.top, toJSON: () => ({}) }
  })
  window.innerWidth = 1000
  window.innerHeight = 700
  rects = {}
})
afterEach(() => {
  cleanup()
  vi.restoreAllMocks()
})

const items: MenuNode[] = [
  { id: 'preview', label: 'Просмотр' },
  {
    id: 'mirror',
    label: 'Зеркало',
    sep: true,
    sub: [
      { id: 'flipH', label: 'Зеркало ↔', mark: 'check', checked: true },
      { id: 'flipV', label: 'Зеркало ↕', mark: 'check', checked: false },
    ],
  },
  {
    id: 'frame',
    label: 'Кадр',
    sub: [
      { id: 'contain', label: 'Целиком', mark: 'radio', checked: true },
      { id: 'cover', label: 'С обрезкой', mark: 'radio', checked: false },
    ],
  },
  { id: 'gray', label: 'Ч/б', disabled: true },
  { id: 'delete', label: 'Удалить', sep: true, danger: true, hint: 'Del' },
]

function open(over: Partial<{ items: MenuNode[]; at: { x: number; y: number } }> = {}) {
  const onpick = vi.fn()
  const onclose = vi.fn()
  render(MenuList, { items: over.items ?? items, at: over.at ?? { x: 100, y: 100 }, onpick, onclose })
  flushSync()
  return { onpick, onclose }
}
const btn = (name: string) => screen.getByRole('menuitem', { name: new RegExp(name) })
const menus = () => [...document.body.querySelectorAll<HTMLElement>('.drop')]

describe('MenuList', () => {
  it('рисует пункты, линии и клавишу справа; список уходит в body', () => {
    open()
    expect(menus()).toHaveLength(1)
    expect(menus()[0].parentElement).toBe(document.body)
    expect(screen.getAllByRole('menuitem')).toHaveLength(5)
    expect(document.querySelectorAll('[role=separator]')).toHaveLength(2)
    expect(btn('Удалить').textContent).toContain('Del')
    expect(btn('Зеркало').textContent).toContain('▸')
  })

  it('наведение на ▸ открывает подменю справа, на обычный пункт — закрывает', async () => {
    open()
    rects.mirror = { left: 100, top: 130, width: LIST_W, height: 28 }
    await fireEvent.pointerEnter(btn('Зеркало'))
    expect(menus()).toHaveLength(2)
    expect(parseFloat(menus()[1].style.left)).toBe(300) // правый край пункта
    expect(parseFloat(menus()[1].style.top)).toBe(130)
    await fireEvent.pointerEnter(btn('Кадр'))
    expect(menus()).toHaveLength(2) // другое подменю — одно
    expect(screen.getByRole('menuitemradio', { name: /Целиком/ })).toBeTruthy()
    await fireEvent.pointerEnter(btn('Просмотр'))
    expect(menus()).toHaveLength(1)
  })

  it('у правого края окна подменю слева', async () => {
    open({ at: { x: 790, y: 100 } })
    rects.mirror = { left: 790, top: 130, width: LIST_W, height: 28 }
    await fireEvent.pointerEnter(btn('Зеркало'))
    expect(menus()).toHaveLength(2)
    expect(parseFloat(menus()[1].style.left)).toBe(590) // левее пункта на ширину подменю
  })

  it('наведение на пункт самого подменю его не закрывает', async () => {
    open()
    await fireEvent.pointerEnter(btn('Зеркало'))
    await fireEvent.pointerEnter(screen.getByRole('menuitemcheckbox', { name: /↔/ }))
    expect(menus()).toHaveLength(2)
  })

  it('щелчок по ▸ открывает и закрывает подменю', async () => {
    const { onpick } = open()
    await fireEvent.click(btn('Кадр'), { detail: 1 })
    expect(menus()).toHaveLength(2)
    await fireEvent.click(btn('Кадр'), { detail: 1 })
    expect(menus()).toHaveLength(1)
    expect(onpick).not.toHaveBeenCalled()
  })

  it('выбор: onpick получает пункт без подменю, серый не выбирается', async () => {
    const { onpick } = open()
    await fireEvent.click(btn('Удалить'))
    expect(onpick).toHaveBeenCalledWith(expect.objectContaining({ id: 'delete' }))
    onpick.mockClear()
    await fireEvent.click(btn('Ч/б'))
    expect(onpick).not.toHaveBeenCalled()
    await fireEvent.pointerEnter(btn('Кадр'))
    await fireEvent.click(screen.getByRole('menuitemradio', { name: /Целиком/ })) // уже отмеченный — тоже onpick
    expect(onpick).toHaveBeenCalledWith(expect.objectContaining({ id: 'contain' }))
  })

  it('✓ и •: role menuitemcheckbox/menuitemradio, aria-checked', async () => {
    open()
    await fireEvent.pointerEnter(btn('Зеркало'))
    const h = screen.getByRole('menuitemcheckbox', { name: /↔/ })
    const v = screen.getByRole('menuitemcheckbox', { name: /↕/ })
    expect([h.getAttribute('aria-checked'), v.getAttribute('aria-checked')]).toEqual(['true', 'false'])
    expect(h.querySelector('.mark')?.textContent).toBe('✓')
    expect(v.querySelector('.mark')?.textContent).toBe('')
    await fireEvent.pointerEnter(btn('Кадр'))
    const c = screen.getByRole('menuitemradio', { name: /Целиком/ })
    expect(c.getAttribute('aria-checked')).toBe('true')
    expect(c.querySelector('.mark')?.textContent).toBe('•')
    expect(btn('Просмотр').getAttribute('aria-checked')).toBeNull()
  })

  it('первый доступный пункт получает фокус при открытии', async () => {
    open()
    await tick()
    await tick()
    expect(document.activeElement).toBe(btn('Просмотр'))
  })

  it('↓ ↑ идут по доступным, серый пропускается; → входит в подменю, ← возвращает, Esc закрывает всё', async () => {
    const { onclose } = open()
    await tick()
    await tick()
    await fireEvent.keyDown(window, { key: 'ArrowDown' })
    expect(document.activeElement).toBe(btn('Зеркало'))
    await fireEvent.keyDown(window, { key: 'ArrowRight' })
    await tick()
    await tick()
    expect(menus()).toHaveLength(2)
    expect(document.activeElement).toBe(screen.getByRole('menuitemcheckbox', { name: /↔/ }))
    await fireEvent.keyDown(window, { key: 'ArrowDown' })
    expect(document.activeElement).toBe(screen.getByRole('menuitemcheckbox', { name: /↕/ }))
    await fireEvent.keyDown(window, { key: 'ArrowLeft' })
    expect(menus()).toHaveLength(1)
    expect(document.activeElement).toBe(btn('Зеркало'))
    await fireEvent.keyDown(window, { key: 'ArrowDown' })
    await fireEvent.keyDown(window, { key: 'ArrowDown' }) // Кадр → Ч/б серый → Удалить
    expect(document.activeElement).toBe(btn('Удалить'))
    await fireEvent.keyDown(window, { key: 'ArrowDown' }) // по кругу
    expect(document.activeElement).toBe(btn('Просмотр'))
    expect(onclose).not.toHaveBeenCalled()
    await fireEvent.keyDown(window, { key: 'Escape' })
    expect(onclose).toHaveBeenCalledTimes(1)
  })

  it('наведение переносит пункт клавиатуры: ↓/↑ идут от наведённого, в том числе с подменю и в подменю', async () => {
    open()
    await tick()
    await tick()
    await fireEvent.pointerEnter(btn('Зеркало')) // пункт с подменю
    await tick()
    await tick()
    expect(document.activeElement).toBe(btn('Зеркало'))
    await fireEvent.keyDown(window, { key: 'ArrowDown' })
    expect(document.activeElement).toBe(btn('Кадр'))
    await fireEvent.pointerEnter(btn('Удалить'))
    await fireEvent.keyDown(window, { key: 'ArrowUp' }) // серый «Ч/б» пропускается
    expect(document.activeElement).toBe(btn('Кадр'))
    await fireEvent.pointerEnter(btn('Зеркало'))
    await tick()
    await tick()
    await fireEvent.pointerEnter(screen.getByRole('menuitemcheckbox', { name: /↕/ })) // пункт подменю
    await fireEvent.keyDown(window, { key: 'ArrowUp' })
    expect(document.activeElement).toBe(screen.getByRole('menuitemcheckbox', { name: /↔/ }))
  })

  it('подсвечен ровно один текущий пункт: наведение и клавиши делят его (QA: двойная подсветка)', async () => {
    open()
    await tick()
    await tick()
    const lit = () => [...document.body.querySelectorAll('button.cur')].map((b) => b.textContent)
    expect(lit()).toEqual([expect.stringContaining('Просмотр')])
    await fireEvent.pointerEnter(btn('Зеркало'))
    await tick()
    await tick()
    expect(lit()).toEqual([expect.stringContaining('Зеркало')])
    await fireEvent.keyDown(window, { key: 'ArrowDown' }) // курсор остался над «Зеркалом», текущий — «Кадр»
    expect(lit()).toEqual([expect.stringContaining('Кадр')])
    await fireEvent.keyDown(window, { key: 'ArrowRight' }) // вход в подменю: текущий — в нём, а не в основном списке
    await tick()
    await tick()
    expect(lit()).toEqual([expect.stringContaining('Целиком')])
    await fireEvent.pointerEnter(btn('Просмотр')) // уход из подменю: текущий снова один, в основном списке
    expect(lit()).toEqual([expect.stringContaining('Просмотр')])
    await fireEvent.pointerEnter(btn('Ч/б')) // серый текущим не становится
    expect(lit()).toEqual([expect.stringContaining('Просмотр')])
  })

  it('наведение на серый пункт фокус не переносит', async () => {
    open()
    await tick()
    await tick()
    await fireEvent.pointerEnter(btn('Ч/б'))
    expect(document.activeElement).toBe(btn('Просмотр'))
  })

  it('→ на обычном пункте ничего не делает', async () => {
    open()
    await tick()
    await tick()
    await fireEvent.keyDown(window, { key: 'ArrowRight' })
    expect(menus()).toHaveLength(1)
  })

  it('щелчок мимо, resize и blur закрывают; щелчок по кнопке-владельцу — нет', async () => {
    const owner = document.createElement('button')
    document.body.appendChild(owner)
    const onclose = vi.fn()
    render(MenuList, { items, at: { x: 1, y: 1 }, keep: owner, onpick: vi.fn(), onclose })
    flushSync()
    await fireEvent.pointerDown(owner)
    expect(onclose).not.toHaveBeenCalled()
    await fireEvent.pointerDown(btn('Просмотр'))
    expect(onclose).not.toHaveBeenCalled()
    await fireEvent.pointerDown(document.body)
    expect(onclose).toHaveBeenCalledTimes(1)
    await fireEvent(window, new Event('resize'))
    await fireEvent(window, new Event('blur'))
    expect(onclose).toHaveBeenCalledTimes(3)
    owner.remove()
  })

  it('пока открыт, горячие клавиши приложения не срабатывают', async () => {
    open()
    const app = vi.fn()
    window.addEventListener('keydown', app) // bubble-фаза, как у обработчика горячих клавиш приложения
    await fireEvent.keyDown(document.body, { key: 'h' })
    await fireEvent.keyDown(document.body, { key: 'Delete' })
    expect(app).not.toHaveBeenCalled()
    window.removeEventListener('keydown', app)
  })

  it('заметка — серая строка без кнопки, фокус и стрелки её обходят', async () => {
    const { onpick } = open({
      items: [
        { id: 'a', label: 'Первый' },
        { id: 'n', label: 'Всё остальное — правым щелчком', kind: 'note' },
        { id: 'b', label: 'Второй' },
      ],
    })
    await tick()
    await tick()
    expect(screen.getAllByRole('menuitem')).toHaveLength(2)
    expect(screen.getByText(/правым щелчком/).tagName).toBe('DIV')
    await fireEvent.keyDown(window, { key: 'ArrowDown' })
    expect(document.activeElement).toBe(btn('Второй'))
    await fireEvent.click(screen.getByText(/правым щелчком/))
    expect(onpick).not.toHaveBeenCalled()
  })

  it('у кнопки: сбоку по side, у края — в другую сторону', () => {
    const btnBox = { left: 300, top: 120, width: 34, height: 22 }
    render(MenuList, { items, at: btnBox, side: 'left', onpick: vi.fn(), onclose: vi.fn() })
    flushSync()
    expect(parseFloat(menus()[0].style.left)).toBe(100) // слева от кнопки на ширину списка
    expect(parseFloat(menus()[0].style.top)).toBe(120)
  })

  it('размонтирование убирает список из body', () => {
    open()
    cleanup()
    expect(menus()).toHaveLength(0)
  })
})

describe('значки пунктов (0.25-menu-icons)', () => {
  it('ни у одного пункта нет значка — места под значок нет, столбец отметок как был', () => {
    open()
    expect(document.body.querySelector('.ico')).toBeNull()
    expect(document.body.querySelector('button.iconed')).toBeNull()
    expect(document.body.querySelector('.mark')).not.toBeNull()
  })
  it('есть значки — рисуются у пунктов, у пункта без значка пустое место того же размера, отметок нет', () => {
    open({
      items: [
        { id: 'a', label: 'Название', icon: 'text' },
        { id: 'b', label: 'Без значка' },
        { id: 'c', label: 'Удалить', icon: 'spread-delete', danger: true },
      ],
    })
    const slots = [...document.body.querySelectorAll('.drop button')].map((b) => b.querySelector('.ico'))
    expect(slots.every((s) => s)).toBe(true)
    expect(slots.map((s) => !!s!.querySelector('svg'))).toEqual([true, false, true])
    expect(document.body.querySelector('.mark')).toBeNull()
    expect(document.body.querySelector('button.iconed')).not.toBeNull()
  })
  it('значки и отметки вместе — оба столбца', () => {
    open({ items: [{ id: 'a', label: 'А', icon: 'text', mark: 'check', checked: true }] })
    expect(document.body.querySelector('.mark')?.textContent).toBe('✓')
    expect(document.body.querySelector('.ico svg')).not.toBeNull()
    expect(document.body.querySelector('button.iconed')).toBeNull()
  })
})
