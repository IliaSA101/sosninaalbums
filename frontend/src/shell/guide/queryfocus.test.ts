import { describe, expect, it } from 'vitest'
import { selectQuery } from './queryfocus'

describe('поле поиска справочника', () => {
  it('при открытии выделяет весь прошлый запрос — сразу и после фокуса окна', async () => {
    let n = 0
    selectQuery({ select: () => void n++ })
    expect(n).toBe(1) // сразу
    await new Promise((r) => setTimeout(r, 0))
    expect(n).toBe(2) // повторно, когда Modal уже поставил фокус
  })
})
