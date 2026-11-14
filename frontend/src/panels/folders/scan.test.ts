import { describe, expect, it } from 'vitest'
import { exportsText, scanFraction, scanText } from './scan'

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

describe('exportsText (121)', () => {
  const t = (n: number) => exportsText(n, 'ФА25')
  it('склонение: 1, 2, 5, 11, 21, 82 файла', () => {
    expect(t(1)).toBe('В папке «ФА25» 1 файл выгрузки Sosnina Albums — в ленту не взят: это разворот, а не фото')
    expect(t(2)).toContain('2 файла выгрузки')
    expect(t(5)).toContain('5 файлов выгрузки')
    expect(t(11)).toContain('11 файлов выгрузки')
    expect(t(21)).toContain('21 файл выгрузки')
    expect(t(82)).toBe('В папке «ФА25» 82 файла выгрузки Sosnina Albums — в ленту не взяты: это развороты, а не фото')
  })
  it('имя папки — в «ёлочках»', () => {
    expect(exportsText(3, 'Свадьба 2026')).toContain('В папке «Свадьба 2026» 3 файла')
  })
})
