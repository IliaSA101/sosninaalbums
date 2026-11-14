// @vitest-environment happy-dom
import { afterEach, describe, expect, it, vi } from 'vitest'
import { cleanup, fireEvent, render, screen } from '@testing-library/svelte'
import { flushSync } from 'svelte'

// Пропорция портрета на общем Seg (grow): щелчок меняет форму сессии, нажата текущая.
vi.mock('../../../wailsjs/runtime/runtime', () => ({ EventsOn: () => () => {} }))
vi.mock('../../../wailsjs/go/app/App', () => ({}))
vi.mock('./vigact', () => ({
  previewVignette: async () => ({ spread: null, error: '' }),
  vigChoices: async () => [],
  applySetup: async () => true,
}))

const { default: VigDialog } = await import('./VigDialog.svelte')
const { vigDialog } = await import('./vigsession.svelte')
const { store } = await import('../../base/core/store.svelte')

const vig = { tpl: 't', teachers: 1, pupils: 24, ratio: '3:4', manual: false, max: 64 }
const sp = { id: 's1', kind: 'vignette', grid: true, photos: [], places: [], rects: [], w: 420, h: 297, vig } as never

afterEach(() => {
  cleanup()
  vigDialog.close()
})

describe('VigDialog: пропорция портрета', () => {
  it('нажата текущая; щелчок по другой переносит нажатие', async () => {
    store.view = { rev: 1, folders: [], spreads: [sp], settings: {} } as never
    vigDialog.open('s1')
    render(VigDialog)
    flushSync()
    const group = screen.getByRole('group', { name: 'Пропорция портрета' })
    expect(Array.from(group.querySelectorAll('button')).map((b) => b.textContent?.trim())).toEqual(['2:3', '3:4', '4:5', '1:1'])
    expect(screen.getByRole('button', { name: '3:4' }).getAttribute('aria-pressed')).toBe('true')
    await fireEvent.click(screen.getByRole('button', { name: '4:5' }))
    flushSync()
    expect(screen.getByRole('button', { name: '4:5' }).getAttribute('aria-pressed')).toBe('true')
    expect(screen.getByRole('button', { name: '3:4' }).getAttribute('aria-pressed')).toBe('false')
  })
})
