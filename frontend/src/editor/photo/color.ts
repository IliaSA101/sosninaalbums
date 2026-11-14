import type { Photo } from '../../base/core/types'

// Значок цветового профиля на миниатюре и ячейке. null — профиль sRGB или неизвестен.
export interface ColorBadge {
  label: string
  title: string
  level: 'info' | 'warn' // info — пересчитаем сами, warn — цвета в печати исказятся
}

type ColorPhoto = Pick<Photo, 'color' | 'profile'>

// Надпись значка и запасное имя профиля для классов, которые Go пересчитывает в sRGB сам (005, 119).
// Надпись зависит от класса, а не от имени: у Xiaomi профиль зовётся «sRGB EOTF with DCI-P3 Color Gamut».
const CONVERTED = {
  adobe: { label: 'aRGB', name: 'Adobe RGB (1998)' },
  p3: { label: 'P3', name: 'Display P3' },
  icc: { label: 'ICC', name: 'RGB-профиль' },
} as const

const converted = (c: Photo['color']) => (c === 'adobe' || c === 'p3' || c === 'icc' ? CONVERTED[c] : null)

export function colorBadge(p: ColorPhoto | undefined): ColorBadge | null {
  if (!p) return null
  const k = converted(p.color)
  if (k) return { label: k.label, title: `${p.profile || k.name}: при экспорте будет пересчитано в sRGB`, level: 'info' }
  if (p.color === 'other') {
    const name = p.profile || 'неизвестный профиль'
    return {
      label: '!',
      title: `${name}: программа не пересчитывает этот профиль, цвета в печати исказятся. Пересохраните фото в sRGB`,
      level: 'warn',
    }
  }
  return null
}

// Строка профиля в карточке кадра: только если фото не sRGB; пояснение — в подсказке (как у значка).
export function profileLine(p: ColorPhoto | undefined): { text: string; title: string; level: ColorBadge['level'] } | null {
  const b = colorBadge(p)
  if (!b || !p) return null
  const name = p.profile || converted(p.color)?.name || 'неизвестный'
  return { text: `Профиль: ${name}`, title: b.title, level: b.level }
}
