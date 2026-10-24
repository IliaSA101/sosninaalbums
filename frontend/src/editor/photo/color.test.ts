import { describe, expect, it } from 'vitest'
import { colorBadge, profileLine } from './color'

describe('colorBadge', () => {
  it('Adobe RGB — нейтральный aRGB с пояснением про пересчёт', () => {
    const b = colorBadge({ color: 'adobe', profile: 'Adobe RGB (1998)' })
    expect(b).toMatchObject({ label: 'aRGB', level: 'info' })
    expect(b?.title).toContain('пересчитано в sRGB')
  })
  it('119: p3 с именем Xiaomi — значок «P3», info, подсказка с настоящим именем', () => {
    const b = colorBadge({ color: 'p3', profile: 'sRGB EOTF with DCI-P3 Color Gamut' })
    expect(b).toMatchObject({ label: 'P3', level: 'info' })
    expect(b?.title).toBe('sRGB EOTF with DCI-P3 Color Gamut: при экспорте будет пересчитано в sRGB')
  })
  it('119: icc — значок «ICC», info; p3 и icc без имени — запасные имена', () => {
    expect(colorBadge({ color: 'icc', profile: 'ProPhoto RGB' })).toMatchObject({ label: 'ICC', level: 'info' })
    expect(colorBadge({ color: 'p3' })?.title.startsWith('Display P3:')).toBe(true)
    expect(colorBadge({ color: 'icc' })?.title.startsWith('RGB-профиль:')).toBe(true)
  })
  it('другой профиль — предупреждение с именем', () => {
    const b = colorBadge({ color: 'other', profile: 'Display P3' })
    expect(b).toMatchObject({ label: '!', level: 'warn' })
    expect(b?.title.startsWith('Display P3:')).toBe(true)
    expect(b?.title).toContain('Пересохраните фото в sRGB')
  })
  it('без имени профиля — запасной текст', () => {
    expect(colorBadge({ color: 'other' })?.title.startsWith('неизвестный профиль:')).toBe(true)
  })
  it('sRGB, пусто и нет фото — без значка', () => {
    expect(colorBadge({ color: 'srgb' })).toBeNull()
    expect(colorBadge({})).toBeNull()
    expect(colorBadge(undefined)).toBeNull()
  })
})

describe('profileLine', () => {
  it('Adobe RGB — строка с именем и пояснением', () => {
    const l = profileLine({ color: 'adobe', profile: 'Adobe RGB (1998)' })
    expect(l).toMatchObject({ text: 'Профиль: Adobe RGB (1998)', level: 'info' })
    expect(l?.title).toContain('пересчитано в sRGB')
  })
  it('119: p3 и icc — строка с именем, уровень info', () => {
    const p3 = profileLine({ color: 'p3', profile: 'sRGB EOTF with DCI-P3 Color Gamut' })
    expect(p3).toMatchObject({ text: 'Профиль: sRGB EOTF with DCI-P3 Color Gamut', level: 'info' })
    expect(p3?.title).toContain('пересчитано в sRGB')
    expect(profileLine({ color: 'icc', profile: 'ProPhoto RGB' })).toMatchObject({ text: 'Профиль: ProPhoto RGB', level: 'info' })
    expect(profileLine({ color: 'p3' })?.text).toBe('Профиль: Display P3')
    expect(profileLine({ color: 'icc' })?.text).toBe('Профиль: RGB-профиль')
  })
  it('другой профиль без имени — запасной текст, предупреждение', () => {
    expect(profileLine({ color: 'other' })).toMatchObject({ text: 'Профиль: неизвестный', level: 'warn' })
  })
  it('sRGB, пусто и нет фото — строки нет', () => {
    expect(profileLine({ color: 'srgb' })).toBeNull()
    expect(profileLine({})).toBeNull()
    expect(profileLine(undefined)).toBeNull()
  })
})
