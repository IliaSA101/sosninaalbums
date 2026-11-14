// Хеш текста статьи F1 для GUIDE_VIDEOS (спека 140, design §6): FNV-1a 32 по сырым `title\nwhat\nwhere\nhow`,
// без подстановки клавиш (клавиши у пользователя свои). Тот же расчёт — scripts/guide-videos/articlehash.mjs.
import type { Article } from './articles'

// FNV-1a 32 по байтам UTF-8; Math.imul держит умножение в 32 битах.
export function fnv1a32(s: string): number {
  let h = 0x811c9dc5
  for (const b of new TextEncoder().encode(s)) h = Math.imul(h ^ b, 0x01000193) >>> 0
  return h
}

// Пустое поле — пустая строка; 8 знаков hex.
export function articleHash(a: Partial<Pick<Article, 'what' | 'where' | 'how'>> & Pick<Article, 'title'>): string {
  return fnv1a32([a.title, a.what ?? '', a.where ?? '', a.how ?? ''].join('\n'))
    .toString(16)
    .padStart(8, '0')
}
