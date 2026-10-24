import { afterEach, describe, expect, it } from 'vitest'
import { setMacForTest } from '../../base/core/platform'
import { catalogView } from '../../base/keys/keymap.fixture'
import { askText, assignedText, phrase, plural, q, removedText, resetAllText, resetOneText, shadowNote } from './keywords'

afterEach(() => setMacForTest(null))
const e = (id: string) => catalogView.commands.find((x) => x.id === id)!

describe('тексты окна клавиш', () => {
  it('«Клавиша B» для одиночной буквы; с модификатором и длинные имена — как есть', () => {
    expect(phrase('KeyB', false)).toBe('Клавиша B')
    expect(phrase('Digit7', false)).toBe('Клавиша 7')
    expect(phrase('Mod+KeyD', false)).toBe('Ctrl+D')
    expect(phrase('Delete', false)).toBe('Del')
    expect(phrase('F5', false)).toBe('F5')
    expect(phrase('Mod+KeyD', true)).toBe('⌘D')
  })
  it('вопрос «Заменить?» называет обе команды и скрытую на уровне', () => {
    expect(askText('Mod+KeyD', 'Дублировать', true, false)).toBe(
      'Ctrl+D уже у «Дублировать». Если заменить, у «Дублировать» этой клавиши не будет.',
    )
    expect(askText('Mod+KeyD', 'Линейки', false, false)).toContain('«Линейки» (скрыта на этом уровне)')
  })
  it('статусы: назначено, с заменой, убрано, сброс всех', () => {
    expect(assignedText('KeyB', 'Во весь экран', null, false)).toBe('B — «Во весь экран»')
    expect(assignedText('Mod+KeyD', 'Дублировать', { label: 'Линейки', visible: false }, false)).toBe(
      'Ctrl+D — «Дублировать» (у «Линейки» (скрыта на этом уровне) больше нет)',
    )
    expect(removedText('KeyF', 'Во весь экран', false)).toBe('F убрана у «Во весь экран»')
    expect(resetAllText(7)).toBe('Все клавиши — как было (своих было 7)')
  })
  it('перекрытие: у кого область главнее, та и сработает; если главнее наша — об этом', () => {
    // «Звёзды: убрать» (лента, ранг 2) и «Вписать» (везде, ранг 1)
    expect(shadowNote(e('fitZoom'), e('markRating0'), 'Digit0', 2, 1, true, false)).toBe(
      'В ленте фото 0 — «Звёзды: убрать», там сработает она',
    )
    expect(shadowNote(e('markRating0'), e('fitZoom'), 'Digit0', 1, 2, true, false)).toBe(
      '0 есть и у «Вписать разворот в окно» (везде); в ленте фото, где действует «Звёзды: убрать», сработает она',
    )
    expect(shadowNote(e('fitZoom'), e('markRating0'), 'Digit0', 2, 1, false, false)).toContain('(скрыта на этом уровне)')
  })
  it('вложенные кавычки в имени команды: внутренние «…» → „…“, внешние остаются', () => {
    const flag = 'Флажок «отобрано»'
    expect(q(flag)).toBe('«Флажок „отобрано“»')
    expect(q('Дублировать')).toBe('«Дублировать»')
    const texts = [
      assignedText('KeyP', flag, { label: flag, visible: true }, false),
      removedText('KeyP', flag, false),
      resetOneText(flag),
      askText('KeyP', flag, true, false),
      shadowNote(e('fitZoom'), { ...e('markRating0'), label: flag }, 'Digit0', 2, 1, true, false),
    ]
    for (const t of texts) expect(t, t).not.toMatch(/«[^»]*«/)
    expect(texts[0]).toBe('P — «Флажок „отобрано“» (у «Флажок „отобрано“» больше нет)')
  })
  it('начальный фокус окна: у поиска нет data-autofocus — Modal фокусирует само окно, первая клавиша идёт на карту', () => {
    const files = import.meta.glob('./KeysDialog.svelte', { query: '?raw', import: 'default', eager: true }) as Record<string, string>
    const src = Object.values(files)[0]
    expect(src).toContain('class="q"')
    expect(src).not.toContain('data-autofocus')
  })
  it('склонение «команда»', () => {
    expect([1, 2, 5, 11, 21].map((n) => plural(n, 'команда', 'команды', 'команд'))).toEqual([
      'команда',
      'команды',
      'команд',
      'команд',
      'команда',
    ])
  })
})
