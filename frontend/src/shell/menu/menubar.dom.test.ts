// @vitest-environment happy-dom
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { cleanup, fireEvent, render, screen } from '@testing-library/svelte'
import { flushSync, tick } from 'svelte'

// Строка меню (124 U1): пять меню, ←/→ проходят их по кругу, «Показать в меню» справочника открывает «Разворот».
// Положение списков (геометрию) happy-dom не считает — это за QA на стенде.
vi.mock('../../../wailsjs/runtime/runtime', () => ({ EventsOn: () => () => {} }))
vi.mock('../../../wailsjs/go/app/App', () => ({ RecentProjects: vi.fn(async () => []) }))

const { default: MenuBar } = await import('./MenuBar.svelte')
const { menuUi } = await import('./menuui.svelte')
const { prefs } = await import('../../base/prefs/prefs.svelte')
const { dropCatalog, useCatalog } = await import('../../base/keys/keymap.fixture')

const NAMES = ['Файл', 'Правка', 'Разворот', 'Вид', 'Справка']
const bar = () => screen.getAllByRole('menuitem', { name: /^(Файл|Правка|Разворот|Вид|Справка)$/ })
const open = () => bar().filter((b) => b.getAttribute('aria-expanded') === 'true')
const items = () => [...document.querySelectorAll<HTMLElement>('[role=menu] [data-item]')].map((b) => b.dataset.item)

// Стрелка на окне: MenuBar слушает keydown в фазе захвата на window.
async function arrow(key: 'ArrowLeft' | 'ArrowRight' | 'ArrowDown') {
  window.dispatchEvent(new KeyboardEvent('keydown', { key, bubbles: true, cancelable: true }))
  flushSync()
  await tick()
}

beforeEach(() => {
  useCatalog()
  prefs.level = 'full'
  menuUi.openId = null
  menuUi.req = null
})
afterEach(() => {
  cleanup()
  dropCatalog()
  menuUi.openId = null
  menuUi.req = null
  prefs.level = 'simple'
})

describe('строка меню', () => {
  it('«Файл · Правка · Разворот · Вид · Справка»', () => {
    render(MenuBar)
    expect(bar().map((b) => b.textContent)).toEqual(NAMES)
  })

  it('щелчок по «Разворот» открывает 11 пунктов; повторный щелчок закрывает', async () => {
    render(MenuBar)
    await fireEvent.click(screen.getByRole('menuitem', { name: 'Разворот' }))
    expect(items()).toEqual([
      ...['addText', 'addSticker', 'bgGallery', 'borderGallery', 'copyDecor', 'spreadTpl'],
      ...['renameSpread', 'pasteNames', 'varyLayouts', 'makeVignette', 'vigSetup'],
    ])
    await fireEvent.click(screen.getByRole('menuitem', { name: 'Разворот' }))
    expect(items()).toEqual([])
  })

  it('«Правка» после переноса — 14 пунктов верхнего уровня', async () => {
    render(MenuBar)
    await fireEvent.click(screen.getByRole('menuitem', { name: 'Правка' }))
    expect(items()).toHaveLength(14)
    expect(items()).not.toContain('bgGallery')
  })

  it('→ проходит все пять меню по кругу и возвращается к первому', async () => {
    render(MenuBar)
    await fireEvent.click(bar()[0])
    const seen = [open()[0].textContent]
    for (let i = 0; i < 5; i++) {
      // Первый пункт «Вид» — «Уровень ▸»: → на нём входит в подменю, как в любом меню; сначала шаг вниз.
      if (seen[i] === 'Вид') await arrow('ArrowDown')
      await arrow('ArrowRight')
      seen.push(open()[0].textContent)
    }
    expect(seen).toEqual([...NAMES, 'Файл'])
  })

  it('← идёт назад и с «Файла» уходит в «Справку»', async () => {
    render(MenuBar)
    await fireEvent.click(bar()[0])
    await arrow('ArrowLeft')
    expect(open().map((b) => b.textContent)).toEqual(['Справка'])
    await arrow('ArrowLeft')
    expect(open().map((b) => b.textContent)).toEqual(['Вид'])
    await arrow('ArrowLeft')
    expect(open().map((b) => b.textContent)).toEqual(['Разворот'])
  })

  it('открыто одно меню: наведение на соседа переключает', async () => {
    render(MenuBar)
    await fireEvent.click(bar()[1])
    await fireEvent.pointerEnter(bar()[2])
    expect(open().map((b) => b.textContent)).toEqual(['Разворот'])
  })

  it('справочник просит «Разворот» — оно открывается без фокуса на пункте', async () => {
    render(MenuBar)
    menuUi.ask('spread')
    flushSync()
    await tick()
    expect(open().map((b) => b.textContent)).toEqual(['Разворот'])
    expect(items()).toContain('bgGallery')
  })

  it('на «Простом» «Разворот» остаётся в строке, ← → не теряются', async () => {
    prefs.level = 'simple'
    render(MenuBar)
    expect(bar().map((b) => b.textContent)).toEqual(NAMES)
    await fireEvent.click(bar()[1])
    await arrow('ArrowRight')
    expect(open().map((b) => b.textContent)).toEqual(['Разворот'])
  })
})
