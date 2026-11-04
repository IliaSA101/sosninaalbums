import { describe, expect, it } from 'vitest'
import { throughNext, type ThroughEv } from './minipanel'

// Часть А 070 (QA 0.14.1 №1): панель «насквозь» не залипает.
describe('throughNext', () => {
  const none = { shiftKey: false, ctrlKey: false, altKey: false, metaKey: false }
  const ev = (type: 'keydown' | 'keyup' | 'pointermove' | 'pointerdown' | 'pointerover', mods: Partial<typeof none> = {}): ThroughEv => ({
    type,
    ...none,
    ...mods,
  })
  // Прежняя схема (до 070): признак менялся только по keydown/keyup/pointermove, остальное его не трогало.
  const old = (prev: boolean, e: ThroughEv) =>
    e.type === 'keydown' || e.type === 'keyup' || e.type === 'pointermove' ? throughNext(e) : e.type === 'blur' ? false : prev

  it('воспроизведение: keyup Alt потерян, панель появилась под неподвижным курсором — старый признак залипал', () => {
    const stream = [ev('keydown', { altKey: true }), ev('pointerdown'), ev('pointerover')] // pointermove нет: курсор стоит
    expect(stream.reduce(old, false)).toBe(true) // панель полупрозрачна, первый щелчок по кнопке уходит в фото
    expect(throughNext(stream[2])).toBe(false) // теперь событие указателя без модификаторов снимает признак
  })
  it('модификатор отпущен вне окна: первое событие указателя без него — сброс', () => {
    expect(throughNext(ev('keydown', { ctrlKey: true }))).toBe(true)
    for (const t of ['pointermove', 'pointerdown', 'pointerover'] as const) expect(throughNext(ev(t))).toBe(false)
  })
  it('модификатор действительно зажат — пропускаем (Shift/Ctrl/Alt/Meta)', () => {
    for (const k of Object.keys(none) as (keyof typeof none)[])
      for (const t of ['pointermove', 'pointerdown', 'pointerover', 'keydown'] as const)
        expect(throughNext(ev(t, { [k]: true }))).toBe(true)
  })
  it('keyup одного модификатора при втором зажатом — остаёмся «насквозь»; все отпущены — сброс', () => {
    expect(throughNext(ev('keyup', { shiftKey: true }))).toBe(true)
    expect(throughNext(ev('keyup'))).toBe(false)
  })
  it('blur и скрытие окна — сброс', () => {
    expect(throughNext({ type: 'blur' })).toBe(false)
    expect(throughNext({ type: 'hidden' })).toBe(false)
  })
  it('новое положение панели или другое выделенное — сброс по последнему указателю', () => {
    expect(throughNext({ type: 'place' })).toBe(false) // указателя ещё не было
    expect(throughNext({ type: 'place', lastPointer: none })).toBe(false) // залипший keydown без указателя — снят
    // Shift+щелчок по соседу: панель переехала, Shift всё ещё зажат — следующий Shift+щелчок идёт насквозь
    expect(throughNext({ type: 'place', lastPointer: { ...none, shiftKey: true } })).toBe(true)
  })
})
