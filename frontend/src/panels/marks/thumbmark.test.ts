import { describe, expect, it } from 'vitest'
import type { Mark } from '../../base/core/types'
import { markTitle, peekTip, stars, thumbTitle } from './thumbmark'

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
  it('подборки дописываются в конец (083)', () => {
    expect(markTitle('a.jpg', { rating: 4, label: '', flag: 0 }, ['Обязательно', 'Для родителей'])).toBe(
      'a.jpg · ★★★★ · Подборки: Обязательно, Для родителей',
    )
    expect(markTitle('a.jpg', none, ['Запас'])).toBe('a.jpg · Подборки: Запас')
    expect(markTitle('a.jpg', none, [])).toBe('a.jpg')
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

describe('подсказка о крупном просмотре (128-9, 0.26-128-16)', () => {
  it('вторая строка title: жест и клавиша (Т4)', () => {
    expect(peekTip(false, 'Z')).toBe('Ctrl+правый щелчок — крупно (Z)')
    expect(thumbTitle('a.jpg', none, [], peekTip(false, 'Z'))).toBe('a.jpg\nCtrl+правый щелчок — крупно (Z)')
    expect(thumbTitle('a.jpg', { rating: 4, label: '', flag: 0 }, [], peekTip(false, 'Z'))).toBe(
      'a.jpg · ★★★★\nCtrl+правый щелчок — крупно (Z)',
    )
  })
  it('на Mac — ⌘', () => {
    expect(peekTip(true, 'Z')).toBe('⌘+правый щелчок — крупно (Z)')
  })
  it('клавиша снята — без скобок', () => {
    expect(peekTip(false, '')).toBe('Ctrl+правый щелчок — крупно')
    expect(thumbTitle('a.jpg', none, [], peekTip(false, ''))).toBe('a.jpg\nCtrl+правый щелчок — крупно')
  })
})
