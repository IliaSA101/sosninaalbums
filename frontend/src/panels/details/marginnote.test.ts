import { describe, expect, it, vi } from 'vitest'
import { marginNoteAsker } from './marginnote'

// Предупреждение о полях у реза: пусто/непусто, гонка ответов, нечисло, сбой Go.
const defer = () => {
  let resolve!: (t: string) => void
  let reject!: (e: unknown) => void
  const p = new Promise<string>((res, rej) => ((resolve = res), (reject = rej)))
  return { p, resolve, reject }
}
const tick = () => new Promise((r) => setTimeout(r, 0))

describe('marginNoteAsker', () => {
  it('шлёт числа и показывает ответ; пустой ответ гасит строку', async () => {
    const out: string[] = []
    const call = vi.fn().mockResolvedValueOnce('Край фото на линии реза').mockResolvedValueOnce('')
    const a = marginNoteAsker(call, (t) => out.push(t))
    a.ask('3', '3')
    await tick()
    expect(call).toHaveBeenCalledWith(3, 3)
    a.ask(5, undefined) // обрез не задан — 0
    await tick()
    expect(call).toHaveBeenLastCalledWith(5, 0)
    expect(out).toEqual(['Край фото на линии реза', ''])
  })
  it('гонка: поздний ответ раннего вызова не затирает последний', async () => {
    const first = defer()
    const second = defer()
    const out: string[] = []
    const call = vi.fn().mockReturnValueOnce(first.p).mockReturnValueOnce(second.p)
    const a = marginNoteAsker(call, (t) => out.push(t))
    a.ask(3, 3)
    a.ask(10, 3)
    second.resolve('')
    await tick()
    first.resolve('устаревшее')
    await tick()
    expect(out).toEqual([''])
  })
  it('нечисло или пустое поле: Go не зовём, строка гаснет, прежний ответ не возвращается', async () => {
    const first = defer()
    const out: string[] = []
    const call = vi.fn().mockReturnValueOnce(first.p)
    const a = marginNoteAsker(call, (t) => out.push(t))
    a.ask(3, 3)
    a.ask('', 3)
    a.ask(NaN, 3)
    first.resolve('поздний')
    await tick()
    expect(call).toHaveBeenCalledTimes(1)
    expect(out).toEqual(['', ''])
  })
  it('сбой Go гасит строку, но не устаревший вызов', async () => {
    const first = defer()
    const second = defer()
    const out: string[] = []
    const call = vi.fn().mockReturnValueOnce(first.p).mockReturnValueOnce(second.p)
    const a = marginNoteAsker(call, (t) => out.push(t))
    a.ask(3, 3)
    a.ask(4, 3)
    first.reject(new Error('x'))
    await tick()
    expect(out).toEqual([])
    second.reject(new Error('y'))
    await tick()
    expect(out).toEqual([''])
  })
})
