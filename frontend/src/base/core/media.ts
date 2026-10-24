// Адреса картинок из /media. ?v= сбивает кэш WebView2 (immutable): после спеки 005
// превью Adobe-фото пересчитаны в sRGB (v=2), после 006 прозрачные PNG отдаются
// с альфой (v=3), а старые копии лежат в кэше браузера.
// 119: превью P3 и других профилей пересчитаны в sRGB (v=4).
const V = 4

// retry — повторная загрузка после сверки папок (088): новая метка сбивает кэш неудачного запроса; путь для Go прежний.
export const thumbUrl = (id: string, retry = 0) => `/media/t/${encodeURIComponent(id)}?v=${V}${retry > 0 ? `&r=${retry}` : ''}`
export const previewUrl = (id: string) => `/media/p/${encodeURIComponent(id)}?v=${V}`

// Разметка типографии обложки (032) и разворотов (066): путь Go берёт из проекта, rev только сбивает кэш
// при смене файла. PSD Go отдаёт PNG ≤ 4096 px.
export const coverGuideUrl = (rev: number) => `/media/cover-guide?r=${Math.trunc(rev) || 0}`
export const spreadGuideUrl = (rev: number) => `/media/spread-guide?r=${Math.trunc(rev) || 0}`

// Разворот на 17+ фото (027): превью 2048 для каждой ячейки съедает память WebView2, поэтому
// крупное превью грузим только там, где миниатюры 400 px не хватает, и соседей не прогреваем.
export const MANY_PHOTOS = 16
const THUMB_PX = 400

// longSide — длинная сторона картинки на экране, CSS px; нужна ли ячейке крупное превью.
export const wantPreview = (photos: number, longSide: number, dpr: number): boolean =>
  photos <= MANY_PHOTOS || longSide * Math.max(dpr, 1) > THUMB_PX

// Соседние развороты прогреваем, только если они не «многофото».
export const warmable = (photos: number): boolean => photos <= MANY_PHOTOS
