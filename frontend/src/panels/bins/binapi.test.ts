import { describe, expect, it, vi } from 'vitest'

// Мост к Go: аргументы доходят до методов App как есть, ответ — вид; ошибка Go не глотается.
const calls: unknown[][] = []
vi.mock('../../../wailsjs/go/app/App', () => {
  const rec =
    (name: string) =>
    async (...a: unknown[]) => {
      calls.push([name, ...a])
      if (a[0] === 'boom') throw new Error('Такая подборка уже есть')
      return { rev: 7, bins: [] }
    }
  return { NewBin: rec('NewBin'), RenameBin: rec('RenameBin'), DeleteBin: rec('DeleteBin'), SetBin: rec('SetBin') }
})
import { DeleteBin, NewBin, RenameBin, SetBin } from './binapi'

describe('binapi', () => {
  it('зовёт методы Go с теми же аргументами и отдаёт вид', async () => {
    calls.length = 0
    expect((await NewBin('Семья', ['a', 'b'])).rev).toBe(7)
    await RenameBin('b1', 'Друзья')
    await SetBin('b1', ['a'], false)
    await DeleteBin('b1')
    expect(calls).toEqual([
      ['NewBin', 'Семья', ['a', 'b']],
      ['RenameBin', 'b1', 'Друзья'],
      ['SetBin', 'b1', ['a'], false],
      ['DeleteBin', 'b1'],
    ])
  })

  it('ошибка Go доходит до вызывающего', async () => {
    await expect(NewBin('boom', [])).rejects.toThrow('Такая подборка уже есть')
  })
})
