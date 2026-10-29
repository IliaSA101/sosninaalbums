// Ролики справочника F1 (спека 140, решение 0.30-140-15): какие статьи с роликом, программа знает и без сети.
// Запись делает съёмка (Т6): id статьи → секунды и хеш текста статьи на момент съёмки (страж videos.guard.test.ts).
// Строка «Ролик» видна и без каталога: показываем встроенный список вместе с кэшем каталога (GuideVideos), чтобы справочник работал без сети.
export const GUIDE_VIDEOS: Record<string, { seconds: number; hash: string }> = {
  autobuild: { seconds: 24, hash: '5e29d4fe' },
  manual: { seconds: 30, hash: '467b653a' },
  variants: { seconds: 24, hash: 'f153cae8' },
  vignette: { seconds: 28, hash: '44db7e7f' },
  cover: { seconds: 33, hash: '094624e9' },
  'bg-photo': { seconds: 26, hash: '9592699e' },
  looks: { seconds: 21, hash: 'c8ee119c' },
  export: { seconds: 30, hash: '896a9ece' },
}
