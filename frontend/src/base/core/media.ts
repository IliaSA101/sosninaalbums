// Адреса картинок из /media. ?v= сбивает кэш WebView2 (immutable): после спеки 005
// превью Adobe-фото пересчитаны в sRGB (v=2), после 006 прозрачные PNG отдаются
// с альфой (v=3), а старые копии лежат в кэше браузера.
// 119: превью P3 и других профилей пересчитаны в sRGB (v=4).
// 125: то же для PNG с профилем (v=5): плитки, ушедшие в кэш без пересчёта, не должны остаться.
const V = 5

// retry — повторная загрузка после сверки папок (088): новая метка сбивает кэш неудачного запроса; путь для Go прежний.
export const thumbUrl = (id: string, retry = 0) => `/media/t/${encodeURIComponent(id)}?v=${V}${retry > 0 ? `&r=${retry}` : ''}`
export const previewUrl = (id: string) => `/media/p/${encodeURIComponent(id)}?v=${V}`
// Оригинал файла как есть (128, ступень 2): окно крупного просмотра при зуме выше 1:1 по превью. Go отдаёт без кэша
// и игнорирует ?v=; 404 и 415 (профиль не sRGB, больше 100 Мп или 200 МБ, битый) UI видит как error у <img>.
export const originalUrl = (id: string) => `/media/o/${encodeURIComponent(id)}?v=${V}`

// Ролик справочника F1 (140): Go отдаёт файл целиком, 200 (design §5, 0.30-140-19). В <video src> адрес не годится
// (WebKitGTK не играет через схему Wails) — окно берёт его fetch и играет Blob.
export const guideVideoUrl = (id: string) => `/media/guide-video/${encodeURIComponent(id)}`

// Разметка типографии обложки (032) и разворотов (066): путь Go берёт из проекта, rev только сбивает кэш
// при смене файла. PSD Go отдаёт PNG ≤ 4096 px.
export const coverGuideUrl = (rev: number) => `/media/cover-guide?r=${Math.trunc(rev) || 0}`
export const spreadGuideUrl = (rev: number) => `/media/spread-guide?r=${Math.trunc(rev) || 0}`

// Макеты альбома (131): шаг i (0 — обложка, 1…24), ширина px, пропорция картинки r, bg — только фон шага («Фоны по серии»),
// front — только лицевая сторона обложки (плитка карточки; r — пропорция стороны, только при i = 0).
// Ключ — ровно 16 строчных hex файла макета, иначе адреса нет; Go пускает только ключи из библиотеки, остальное — 404. Свой счётчик версий: ?v=1.
const LOOK_V = 1
export function lookUrl(key: string, o: { i: number; px: number; r: number; bg?: boolean; front?: boolean }): string {
  if (!/^[0-9a-f]{16}$/.test(key)) return '' // не ключ — картинку не просим (Go ответил бы 404)
  const r = Math.round(o.r * 10000) / 10000 || 1
  return `/media/look/${encodeURIComponent(key)}?v=${LOOK_V}&i=${Math.trunc(o.i) || 0}&px=${Math.trunc(o.px) || 0}&r=${r}&bg=${o.bg ? 1 : 0}${o.front ? '&f=1' : ''}`
}

// Разворот на 17+ фото (027): превью 2048 для каждой ячейки съедает память WebView2, поэтому
// крупное превью грузим только там, где миниатюры 400 px не хватает, и соседей не прогреваем.
export const MANY_PHOTOS = 16
const THUMB_PX = 400

// longSide — длинная сторона картинки на экране, CSS px; нужна ли ячейке крупное превью.
export const wantPreview = (photos: number, longSide: number, dpr: number): boolean =>
  photos <= MANY_PHOTOS || longSide * Math.max(dpr, 1) > THUMB_PX

// Соседние развороты прогреваем, только если они не «многофото».
export const warmable = (photos: number): boolean => photos <= MANY_PHOTOS
