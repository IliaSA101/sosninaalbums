// @vitest-environment happy-dom
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { cleanup, fireEvent, render, screen } from '@testing-library/svelte'
import { flushSync } from 'svelte'

// Пустая лента при поиске по имени (0.26-124): «По «…» ничего не найдено» и «Сбросить поиск» — по образцу «Сбросить фильтр».
vi.mock('../../../wailsjs/runtime/runtime', () => ({ EventsOn: () => () => {} }))
vi.mock('../../../wailsjs/go/app/App', () => ({ Marks: async () => ({}) }))

const { default: Library } = await import('./Library.svelte')
const { store } = await import('../../base/core/store.svelte')
const { nameSearch } = await import('../marks/namefind.svelte')
const { libfilter } = await import('../marks/libfilter.svelte')
const { ownMarks } = await import('../marks/ownmarks.svelte')
const { prefs } = await import('../../base/prefs/prefs.svelte')
const { wire } = await import('../../shell/boot/wire')

wire()

const ph = (name: string) => ({ id: name, path: `D:/a/${name}`, name, w: 100, h: 100 })
const search = (q: string) => {
  nameSearch.type(q)
  nameSearch.commit()
  flushSync()
}

beforeEach(() => {
  prefs.level = 'standard'
  nameSearch.reset()
  libfilter.reset()
  ownMarks.map = {}
  store.loaded = true
  store.filter = 'all'
  store.view = { ...store.view, folders: ['D:/a'] } as never
  store.photos = ['IMG_0798.JPG', 'IMG_0801.JPG'].map(ph) as never
})
afterEach(() => {
  cleanup()
  nameSearch.reset()
})

describe('пустая лента при поиске', () => {
  it('ничего не нашлось — слова с запросом и «Сбросить поиск»; кнопка очищает поиск', async () => {
    render(Library)
    search('zzz')
    expect(screen.getByText(/По «zzz» ничего не найдено/)).toBeTruthy()
    await fireEvent.click(screen.getByText('Сбросить поиск'))
    expect(nameSearch.active).toBe(false)
    expect(screen.queryByText(/ничего не найдено/)).toBeNull()
  })

  it('«Сбросить поиск» сворачивает и пустое поле (п. 4 спеки: пустое поле — кнопка ⌕)', async () => {
    render(Library)
    nameSearch.open = true
    search('zzz')
    await fireEvent.click(screen.getByText('Сбросить поиск'))
    expect(nameSearch.q).toBe('')
    expect(nameSearch.open).toBe(false)
    expect(nameSearch.held).toBe(false)
  })

  it('длинный запрос усечён в тексте', () => {
    render(Library)
    search('x'.repeat(100))
    expect(screen.getByText(new RegExp(`По «${'x'.repeat(60)}…» ничего не найдено`))).toBeTruthy()
  })

  it('поиск находит — подсказки нет', () => {
    render(Library)
    search('798')
    expect(screen.queryByText(/ничего не найдено/)).toBeNull()
  })

  it('фильтр отметок скрыл всё: говорит фильтр, а не поиск (в ленте до поиска пусто)', () => {
    libfilter.minRating = 5
    render(Library)
    search('798')
    expect(screen.queryByText(/ничего не найдено/)).toBeNull()
    expect(screen.getByText(/Ни одно фото не подходит под фильтр/)).toBeTruthy()
  })

  it('поиск пуст, фильтр скрыл всё — «Сбросить фильтр» как раньше', () => {
    libfilter.minRating = 5
    render(Library)
    expect(screen.getByRole('button', { name: 'Сбросить фильтр' })).toBeTruthy()
  })
})
