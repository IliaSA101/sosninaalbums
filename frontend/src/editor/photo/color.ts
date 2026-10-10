import type { Photo } from '../../base/core/types'

// Значок цветового профиля на миниатюре и ячейке. null — профиль sRGB или неизвестен.
export interface ColorBadge {
  label: string
  title: string
  level: 'info' | 'warn' // info — пересчитаем сами, warn — цвета в печати исказятся
}

export function colorBadge(p: Pick<Photo, 'color' | 'profile'> | undefined): ColorBadge | null {
  if (!p) return null
  if (p.color === 'adobe') {
    const name = p.profile || 'Adobe RGB (1998)'
    return { label: 'aRGB', title: `${name}: при экспорте будет пересчитано в sRGB`, level: 'info' }
  }
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
export function profileLine(
  p: Pick<Photo, 'color' | 'profile'> | undefined,
): { text: string; title: string; level: ColorBadge['level'] } | null {
  const b = colorBadge(p)
  if (!b || !p) return null
  const name = p.profile || (p.color === 'adobe' ? 'Adobe RGB (1998)' : 'неизвестный')
  return { text: `Профиль: ${name}`, title: b.title, level: b.level }
}
