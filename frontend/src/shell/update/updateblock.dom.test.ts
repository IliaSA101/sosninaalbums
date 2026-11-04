// @vitest-environment happy-dom
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { cleanup, fireEvent, render, screen } from '@testing-library/svelte'
import { flushSync } from 'svelte'

// Go подменён: считаем вызовы, ответы задаёт тест.
const go = vi.hoisted(() => ({
  UpdateInfo: vi.fn(),
  CheckUpdate: vi.fn(),
  DownloadUpdate: vi.fn(),
  CancelUpdate: vi.fn(),
  InstallUpdate: vi.fn(),
  OpenUpdatePage: vi.fn(),
  SetUpdateCheck: vi.fn(),
}))
vi.mock('../../../wailsjs/runtime/runtime', () => ({ EventsOn: () => () => {} }))
vi.mock('../../../wailsjs/go/app/App', () => go)

const { default: UpdateBlock } = await import('./UpdateBlock.svelte')
const { updateUi } = await import('./update.svelte')
const { NO_UPDATE } = await import('./updateapi')
const { store } = await import('../../base/core/store.svelte')

import type { UpdateInfo } from './updateapi'
const set = (p: Partial<UpdateInfo>) => {
  updateUi.info = { ...NO_UPDATE, mode: 'install', ...p }
  flushSync()
}
const btn = (name: string | RegExp) => screen.getByRole('button', { name })
const tick = () => new Promise((r) => setTimeout(r, 0))

beforeEach(() => {
  for (const f of Object.values(go)) f.mockReset().mockResolvedValue(undefined)
  go.CheckUpdate.mockResolvedValue({ ...NO_UPDATE, phase: 'latest', mode: 'install' })
  store.toast = null
  updateUi.info = { ...NO_UPDATE, mode: 'install' }
})
afterEach(() => cleanup())

describe('UpdateBlock: кнопки зовут Go', () => {
  it('idle: «Проверить обновления» → CheckUpdate, затем «Установлена последняя версия»', async () => {
    render(UpdateBlock)
    await fireEvent.click(btn('Проверить обновления'))
    await tick()
    expect(go.CheckUpdate).toHaveBeenCalledTimes(1)
    expect(screen.getByRole('status').textContent).toBe('Установлена последняя версия')
  })

  it('пока идёт проверка — «Проверяем…», кнопка неактивна, повторный щелчок не зовёт Go', async () => {
    let done!: (i: UpdateInfo) => void
    go.CheckUpdate.mockReturnValue(new Promise((r) => (done = r)))
    render(UpdateBlock)
    await fireEvent.click(btn('Проверить обновления'))
    expect(screen.getByRole('status').textContent).toBe('Проверяем…')
    const b = btn('Проверить обновления') as HTMLButtonElement
    expect(b.disabled).toBe(true)
    await fireEvent.click(b)
    expect(go.CheckUpdate).toHaveBeenCalledTimes(1)
    done({ ...NO_UPDATE, phase: 'available', mode: 'install', version: '0.25.1' })
    await tick()
    expect(screen.getByRole('status').textContent).toBe('Вышла версия 0.25.1')
  })

  it('available: «Обновить» → DownloadUpdate; ход — по update:progress; «Отменить» → CancelUpdate', async () => {
    let done!: (i: UpdateInfo) => void
    go.DownloadUpdate.mockReturnValue(new Promise((r) => (done = r)))
    render(UpdateBlock)
    set({ phase: 'available', version: '0.25.1' })
    await fireEvent.click(btn('Обновить'))
    expect(go.DownloadUpdate).toHaveBeenCalledTimes(1)
    expect(screen.getByRole('status').textContent).toBe('Скачиваем обновление…')
    updateUi.progressed({ done: 3 * 1048576, total: 8 * 1048576 })
    flushSync()
    expect(screen.getByRole('status').textContent).toBe('Скачиваем обновление — 3 из 8 МБ')
    expect((screen.getByRole('progressbar') as HTMLProgressElement).value).toBe(38)
    await fireEvent.click(btn('Отменить'))
    expect(go.CancelUpdate).toHaveBeenCalledTimes(1)
    expect(screen.getByRole('status').textContent).toBe('Отменяем…')
    // Go оборвал скачивание и вернул прежнее предложение.
    done({ ...NO_UPDATE, phase: 'available', mode: 'install', version: '0.25.1' })
    await tick()
    expect(screen.getByRole('status').textContent).toBe('Вышла версия 0.25.1')
  })

  it('ready: «Перезапустить и обновить» → InstallUpdate, второй щелчок не повторяет', async () => {
    render(UpdateBlock)
    set({ phase: 'ready', version: '0.25.1' })
    await fireEvent.click(btn('Перезапустить и обновить'))
    await fireEvent.click(btn('Перезапустить и обновить'))
    expect(go.InstallUpdate).toHaveBeenCalledTimes(1)
  })

  it('page: «Скачать» → OpenUpdatePage; «Страница выпуска» на ошибке → тоже', async () => {
    render(UpdateBlock)
    set({ phase: 'available', mode: 'page', version: '0.25.1' })
    await fireEvent.click(btn('Скачать'))
    expect(go.OpenUpdatePage).toHaveBeenCalledTimes(1)
    set({ phase: 'error', error: 'Нет связи' })
    await fireEvent.click(btn('Страница выпуска'))
    expect(go.OpenUpdatePage).toHaveBeenCalledTimes(2)
  })

  it('флажок «Проверять при запуске» → SetUpdateCheck; ошибка Go возвращает прежнее и даёт тост', async () => {
    render(UpdateBlock)
    const box = screen.getByRole('checkbox', { name: /Проверять при запуске/ }) as HTMLInputElement
    expect(box.checked).toBe(true)
    go.SetUpdateCheck.mockResolvedValueOnce({ ...NO_UPDATE, atStart: false })
    await fireEvent.click(box)
    await tick()
    expect(go.SetUpdateCheck).toHaveBeenLastCalledWith(false)
    expect(box.checked).toBe(false)
    go.SetUpdateCheck.mockRejectedValueOnce('не записалось')
    await fireEvent.click(box)
    await tick()
    expect(go.SetUpdateCheck).toHaveBeenLastCalledWith(true)
    expect(box.checked).toBe(false)
    expect(store.toast?.text).toBe('Не записалось')
  })

  // Go отвечает на сетевой отказ и отказ проверки без reject: phase 'error' и слова в поле error.
  it('ошибка проверки — строкой в блоке; «Повторить» снова зовёт CheckUpdate', async () => {
    go.CheckUpdate.mockResolvedValueOnce({ ...NO_UPDATE, phase: 'error', mode: 'install', error: 'Нет связи с github.com' })
    render(UpdateBlock)
    await fireEvent.click(btn('Проверить обновления'))
    await tick()
    expect(screen.getByRole('status').textContent).toBe('Нет связи с github.com')
    await fireEvent.click(btn('Повторить'))
    await tick()
    expect(go.CheckUpdate).toHaveBeenCalledTimes(2)
    expect(screen.getByRole('status').textContent).toBe('Установлена последняя версия')
  })

  it('ошибка скачивания (версия известна): «Повторить» снова зовёт DownloadUpdate', async () => {
    go.DownloadUpdate.mockResolvedValueOnce({
      ...NO_UPDATE,
      phase: 'error',
      mode: 'install',
      version: '0.25.1',
      error: 'Обновление не прошло проверку — скачайте установщик со страницы выпуска',
    })
    render(UpdateBlock)
    set({ phase: 'available', version: '0.25.1' })
    await fireEvent.click(btn('Обновить'))
    await tick()
    expect(screen.getByRole('status').textContent).toContain('Обновление не прошло проверку')
    go.DownloadUpdate.mockResolvedValueOnce({ ...NO_UPDATE, phase: 'ready', mode: 'install', version: '0.25.1' })
    await fireEvent.click(btn('Повторить'))
    await tick()
    expect(go.DownloadUpdate).toHaveBeenCalledTimes(2)
    expect(btn('Перезапустить и обновить')).toBeTruthy()
  })

  it('reject (неверный порядок вызовов) — тоже строкой в блоке, с заглавной буквы', async () => {
    go.DownloadUpdate.mockRejectedValueOnce('нет предложения об обновлении — сначала проверьте обновления')
    render(UpdateBlock)
    set({ phase: 'available', version: '0.25.1' })
    await fireEvent.click(btn('Обновить'))
    await tick()
    expect(screen.getByRole('status').textContent).toBe('Нет предложения об обновлении — сначала проверьте обновления')
  })

  it('available: рядом с «Обновить» постоянная «Проверить обновления» (первая в ряду) → CheckUpdate', async () => {
    render(UpdateBlock)
    set({ phase: 'available', version: '0.25.1' })
    const names = screen.getAllByRole('button').map((b) => b.textContent?.trim())
    expect(names.slice(0, 2)).toEqual(['Проверить обновления', 'Обновить'])
    await fireEvent.click(btn('Проверить обновления'))
    expect(go.CheckUpdate).toHaveBeenCalledTimes(1)
  })

  it('«Повторить» после ошибки проверки зовёт CheckUpdate, после ошибки скачивания — DownloadUpdate', async () => {
    render(UpdateBlock)
    set({ phase: 'available', version: '0.25.1' })
    go.CheckUpdate.mockResolvedValueOnce({ ...NO_UPDATE, phase: 'error', mode: 'install', version: '0.25.1', error: 'нет связи' })
    await fireEvent.click(btn('Проверить обновления'))
    await tick()
    expect(screen.getByRole('status').textContent).toBe('Нет связи')
    await fireEvent.click(btn('Повторить'))
    expect(go.CheckUpdate).toHaveBeenCalledTimes(2)
    expect(go.DownloadUpdate).not.toHaveBeenCalled()
    set({ phase: 'available', version: '0.25.1' })
    go.DownloadUpdate.mockResolvedValueOnce({ ...NO_UPDATE, phase: 'error', mode: 'install', version: '0.25.1', error: 'обрыв' })
    await fireEvent.click(btn('Обновить'))
    await tick()
    await fireEvent.click(btn('Повторить'))
    expect(go.DownloadUpdate).toHaveBeenCalledTimes(2)
  })

  it('«Что нового ▸» раскрывает строки из файла выпуска текстом; свёртывается вторым щелчком', async () => {
    render(UpdateBlock)
    set({ phase: 'available', version: '0.25.1', notes: ['Установщик', '<b>не разметка</b>'] })
    expect(screen.queryByRole('list')).toBeNull()
    await fireEvent.click(btn(/Что нового/))
    const items = screen.getAllByRole('listitem').map((li) => li.textContent)
    expect(items).toEqual(['Установщик', '<b>не разметка</b>'])
    await fireEvent.click(btn(/Что нового/))
    expect(screen.queryByRole('list')).toBeNull()
  })

  it('setup: подсказка «Поставьте установщиком…»; install — без неё', () => {
    render(UpdateBlock)
    set({ phase: 'latest', mode: 'setup' })
    expect(document.body.textContent).toContain('Поставьте установщиком — дальше программа будет обновляться сама')
    set({ phase: 'latest', mode: 'install' })
    expect(document.body.textContent).not.toContain('Поставьте установщиком')
  })
})
