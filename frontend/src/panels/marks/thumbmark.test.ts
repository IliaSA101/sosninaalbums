import { describe, expect, it } from 'vitest'
import type { Mark } from '../../base/core/types'
import { markTitle, stars } from './thumbmark'

const none: Mark = { rating: 0, label: '', flag: 0 }

describe('stars', () => {
  it('по оценке и с ограничением', () => {
    expect(stars(0)).toBe('')
    expect(stars(3)).toBe('★★★')
    expect(stars(9)).toBe('★★★★★')
    expect(stars(-1)).toBe('')
  })
})

describe('markTitle', () => {
  it('без отметок — одно имя', () => {
    expect(markTitle('a.jpg', none)).toBe('a.jpg')
  })
  it('оценка, метка, флажок', () => {
    expect(markTitle('a.jpg', { rating: 4, label: 'red', flag: 1 })).toBe('a.jpg · ★★★★ · метка: красная · отобрано')
    expect(markTitle('a.jpg', { ...none, flag: -1 })).toBe('a.jpg · отклонено (снять: выделите и нажмите U)')
  })
  it('отклонено в Lightroom — без подсказки про U', () => {
    const lr: Mark = { rating: 0, label: '', flag: -1 }
    expect(markTitle('a.jpg', { ...none, lr })).toBe('a.jpg · в Lightroom: отклонено')
  })
  it('незнакомая метка — её текст в одну строку', () => {
    const m: Mark = { ...none, label: 'other', labelText: 'Клиенту\n <b>x</b>' }
    expect(markTitle('a.jpg', m)).toBe('a.jpg · метка: Клиенту <b>x</b>')
    expect(markTitle('a.jpg', { ...none, label: 'other' })).toBe('a.jpg · метка: своя метка')
  })
  it('своя расходится с файлом — «в Lightroom»', () => {
    const lr: Mark = { rating: 5, label: 'green', flag: 0 }
    expect(markTitle('a.jpg', { rating: 2, label: 'green', flag: 0, lr })).toBe('a.jpg · ★★ · метка: зелёная · в Lightroom: ★★★★★')
    expect(markTitle('a.jpg', { rating: 0, label: '', flag: 0, lr: { rating: 3, label: 'red', flag: 1 } })).toBe(
      'a.jpg · в Lightroom: ★★★, метка: красная, отобрано',
    )
    expect(markTitle('a.jpg', { rating: 3, label: '', flag: 0, lr: { rating: 3, label: 'red', flag: 0 } })).toBe(
      'a.jpg · ★★★ · в Lightroom: метка: красная',
    )
  })
})
