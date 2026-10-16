import { describe, expect, it } from 'vitest'
import { scanFraction, scanText } from './scan'

const sc = (o: Partial<{ active: boolean; walking: boolean; found: number; read: number }>) => ({
  active: true,
  walking: false,
  found: 0,
  read: 0,
  ...o,
})

describe('scanText', () => {
  it('обход: только найдено', () => {
    expect(scanText(sc({ walking: true, found: 42 }))).toBe('Ищу фото… найдено 42')
  })
  it('чтение: N из M', () => {
    expect(scanText(sc({ found: 700, read: 120 }))).toBe('Читаю папку… 120 из 700')
  })
})

describe('scanFraction', () => {
  it('при обходе и без файлов — null', () => {
    expect(scanFraction(sc({ walking: true, found: 10 }))).toBeNull()
    expect(scanFraction(sc({ found: 0 }))).toBeNull()
  })
  it('доля зажата в 0…1', () => {
    expect(scanFraction(sc({ found: 200, read: 50 }))).toBe(0.25)
    expect(scanFraction(sc({ found: 10, read: 15 }))).toBe(1)
  })
})
