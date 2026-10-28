import { describe, expect, it } from 'vitest'
import { folderName } from './names'

describe('folderName', () => {
  it('Windows и Unix, хвостовой разделитель', () => {
    expect(folderName('D:\\Photos\\Wedding')).toBe('Wedding')
    expect(folderName('/home/u/Свадьба/')).toBe('Свадьба')
  })
  it('пустая строка остаётся как есть', () => {
    expect(folderName('')).toBe('')
  })
})
