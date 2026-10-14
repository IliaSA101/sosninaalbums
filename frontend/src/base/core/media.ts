// Адреса картинок из /media. ?v= сбивает кэш WebView2 (immutable): после спеки 005
// превью Adobe-фото пересчитаны в sRGB (v=2), после 006 прозрачные PNG отдаются
// с альфой (v=3), а старые копии лежат в кэше браузера.
const V = 3

export const thumbUrl = (id: string) => `/media/t/${encodeURIComponent(id)}?v=${V}`
export const previewUrl = (id: string) => `/media/p/${encodeURIComponent(id)}?v=${V}`

// Разметка типографии обложки (032): путь Go берёт из проекта, rev только сбивает кэш при смене файла.
export const coverGuideUrl = (rev: number) => `/media/cover-guide?r=${Math.trunc(rev) || 0}`

// Разворот на 17+ фото (027): превью 2048 для каждой ячейки съедает память WebView2, поэтому
// крупное превью грузим только там, где миниатюры 400 px не хватает, и соседей не прогреваем.
export const MANY_PHOTOS = 16
const THUMB_PX = 400

// longSide — длинная сторона картинки на экране, CSS px; нужна ли ячейке крупное превью.
export const wantPreview = (photos: number, longSide: number, dpr: number): boolean =>
  photos <= MANY_PHOTOS || longSide * Math.max(dpr, 1) > THUMB_PX

// Соседние развороты прогреваем, только если они не «многофото».
export const warmable = (photos: number): boolean => photos <= MANY_PHOTOS
