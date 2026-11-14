import { describe, expect, it, vi } from 'vitest'

// Go подменён: список копий, замена альбома копией, ошибка по имени «bad».
const list = [
  { name: 'a.salbum', time: new Date(2026, 9, 9, 15, 30).getTime(), size: 131072, spreads: 8, photos: 120 },
  { name: 'b.salbum', time: new Date(2026, 9, 8, 9, 12).getTime(), size: 2048, spreads: 2, photos: 10 },
]
const restored = { rev: 5, folders: [], spreads: [{ id: 'x' }] }

vi.mock('../../../wailsjs/runtime/runtime', () => ({ EventsOn: () => () => {} }))
vi.mock('../../../wailsjs/go/app/App', () => ({
  Backups: async () => list,
  ShowBackups: async () => {},
  RestoreBackup: async (name: string) => {
    if (name === 'bad') throw 'копия не найдена'
    return restored
  },
}))
vi.mock('../../editor/text/fonts', () => ({ loadFonts: async () => {} }))
vi.stubGlobal('window', globalThis)

const { backupsUi } = await import('./backups.svelte')
const { store } = await import('../../base/core/store.svelte')

describe('backupsUi', () => {
  it('show читает список и открывает окно', async () => {
    await backupsUi.show()
    expect(backupsUi.open).toBe(true)
    expect(backupsUi.items.map((b) => b.name)).toEqual(['a.salbum', 'b.salbum'])
    expect(backupsUi.loading).toBe(false)
  })

  it('«Восстановить»: окно закрывается, альбом заменён, тост с временем копии и «Вернуть»', async () => {
    await backupsUi.show()
    await backupsUi.recover(list[0])
    expect(backupsUi.open).toBe(false)
    expect(store.view.rev).toBe(5)
    expect(store.toast?.text).toBe('Альбом восстановлен из копии от 15:30')
    expect(store.toast?.action?.label).toBe('Вернуть')
  })

  it('копия совпала с альбомом (rev тот же): тост без «Вернуть»', async () => {
    store.toast = null
    await backupsUi.show()
    await backupsUi.recover(list[0])
    const toast = store.toast as { text: string; action?: unknown } | null
    expect(toast?.text).toBe('Копия совпадает с альбомом')
    expect(toast?.action).toBeUndefined()
  })

  it('ошибка Go остаётся в окне, а не в тосте', async () => {
    await backupsUi.show()
    await backupsUi.recover({ ...list[0], name: 'bad' })
    expect(backupsUi.open).toBe(true)
    expect(backupsUi.error).toBe('Копия не найдена')
    expect(backupsUi.busy).toBe(false)
  })
})
