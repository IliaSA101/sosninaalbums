// Адреса картинок из /media. ?v= сбивает кэш WebView2 (immutable): после спеки 005
// превью Adobe-фото пересчитаны в sRGB (v=2), после 006 прозрачные PNG отдаются
// с альфой (v=3), а старые копии лежат в кэше браузера.
const V = 3

export const thumbUrl = (id: string) => `/media/t/${encodeURIComponent(id)}?v=${V}`
export const previewUrl = (id: string) => `/media/p/${encodeURIComponent(id)}?v=${V}`
