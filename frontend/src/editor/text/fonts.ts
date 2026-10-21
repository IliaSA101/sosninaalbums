// Шрифты текстовых блоков (спека 006, design §5.1). Go отдаёт State.fonts, файлы лежат
// в /media/font/<file>. FontFace создаём из кода, а не @font-face в CSS: Vite пытается
// разрешить url(/media/…) при сборке и пишет предупреждение.
import type { FontFamily, FontImport, TextStyle } from '../../base/core/types'
import { DropFonts } from './fontapi'

// Go не делает подстановок GSUB, поэтому лигатуры и calt выключены и на экране (иначе ширины разойдутся).
export const FEATURE_SETTINGS = '"liga" 0, "clig" 0, "calt" 0'
// Для style="" у SVG-текста; font-synthesis: none — без поддельного жирного и курсива.
export const TEXT_RENDER_CSS = `font-synthesis:none;font-variant-ligatures:none;font-feature-settings:${FEATURE_SETTINGS};white-space:pre`

export interface FaceSpec {
  family: string
  source: string
  weight: '400' | '700'
  style: 'normal' | 'italic'
}

export const fontUrl = (file: string) => `url(/media/font/${encodeURIComponent(file)})`

// Описания всех начертаний (чистая часть, под тест).
export function faceSpecs(fonts: FontFamily[]): FaceSpec[] {
  return fonts.flatMap((f) =>
    f.faces.map((face): FaceSpec => ({
      family: f.css,
      source: fontUrl(face.file),
      weight: face.bold ? '700' : '400',
      style: face.italic ? 'italic' : 'normal',
    })),
  )
}

// Уже зарегистрированные адреса: событие `fonts` шлёт полный список, дублировать начертания в document.fonts нельзя.
const registered = new Set<string>()
export const isRegistered = (file: string) => registered.has(fontUrl(file))

// Забыть зарегистрированное и проверенное (для тестов; в программе не нужно).
export function resetFonts() {
  registered.clear()
  fate.clear()
  told.clear()
}

// Последний список шрифтов: слушатель отказов ищет в нём упавшее семейство по имени css.
let latest: FontFamily[] = []
let watching = false
const told = new Set<string>()

function watchFailures(notify: (text: string) => void) {
  if (watching || typeof document === 'undefined' || !document.fonts?.addEventListener) return
  watching = true
  document.fonts.addEventListener('loadingerror', (e) => {
    for (const face of (e as FontFaceSetLoadEvent).fontfaces) {
      const text = failedText(face.family, latest, told)
      if (text) notify(text)
    }
  })
}

// Регистрирует новые начертания в document.fonts, но не грузит: браузер возьмёт файл при первом
// использовании (текст на холсте, образец в открытом списке). Гарнитур 33, файлов 62 —
// грузить всё на старте незачем. Возвращает число вновь зарегистрированных. notify — тост об отказе окна при
// обычном использовании своего шрифта (082-14), слушатель ставится один раз.
export function loadFonts(fonts: FontFamily[], notify?: (text: string) => void): number {
  latest = fonts
  if (notify) watchFailures(notify)
  let n = 0
  for (const s of faceSpecs(fonts)) {
    if (registered.has(s.source)) continue
    try {
      document.fonts.add(new FontFace(s.family, s.source, { weight: s.weight, style: s.style, featureSettings: FEATURE_SETTINGS }))
      registered.add(s.source)
      n++
    } catch {
      // Битое описание одного файла не мешает остальным.
    }
  }
  return n
}

// Группы списка шрифтов: id из Go (FontFamily.group) и подпись. Порядок — порядок в списке.
export const GROUPS: readonly (readonly [string, string])[] = [
  ['my', 'Мои шрифты'],
  ['script', 'Рукописные'],
  ['kids', 'Детские и округлые'],
  ['serif', 'С засечками'],
  ['sans', 'Без засечек'],
  ['display', 'Декоративные'],
]

export interface FontGroup {
  id: string
  title: string
  fonts: FontFamily[]
}

// Гарнитуры по группам в порядке GROUPS, внутри — как пришли из Go. Пустые группы пропускаются;
// гарнитура с незнакомой группой уходит в последнюю, чтобы её можно было выбрать. Скрытые (×, 082) не показываются,
// кроме текущей (current): иначе стоящий у текста шрифт пропал бы из списка.
export function groupFonts(fonts: FontFamily[], current = ''): FontGroup[] {
  const known = new Set(GROUPS.map(([id]) => id))
  const last = GROUPS[GROUPS.length - 1][0]
  const shown = fonts.filter((f) => !f.hidden || f.id === current)
  return GROUPS.map(([id, title]) => ({
    id,
    title,
    fonts: shown.filter((f) => (known.has(f.group) ? f.group : last) === id),
  })).filter((g) => g.fonts.length > 0)
}

// Образец в списке шрифтов; у шрифта без кириллицы — латиницей (082).
export const SAMPLE = 'Анна и Павел'
export const SAMPLE_LAT = 'Anna & Paul'
export const sampleOf = (f: Pick<FontFamily, 'noCyr'>) => (f.noCyr ? SAMPLE_LAT : SAMPLE)

// Клавиши навигации по списку: новая активная строка или null, если клавиша не навигационная.
export function moveActive(key: string, active: number, n: number): number | null {
  if (n <= 0) return null
  switch (key) {
    case 'ArrowDown':
      return (active + 1) % n
    case 'ArrowUp':
      return (active - 1 + n) % n
    case 'Home':
      return 0
    case 'End':
      return n - 1
    default:
      return null
  }
}

// Переход по набранным буквам: индекс первой гарнитуры, чьё имя начинается с query (без регистра).
// Если текущая уже подходит — остаётся на ней. Нет совпадения — -1.
export function findByPrefix(names: string[], query: string, from: number): number {
  const q = query.toLowerCase()
  if (!q) return -1
  const hit = (i: number) => names[i]?.toLowerCase().startsWith(q) ?? false
  return hit(from) ? from : names.findIndex((_, i) => hit(i))
}

// = text.DefaultFont в Go: запасная гарнитура для знака, которого нет в выбранной (решение №66).
export const DEFAULT_FONT = 'montserrat'

export interface FontCss {
  family: string // значение font-family (с запасным семейством)
  weight: 400 | 700
  style: 'normal' | 'italic'
}

// CSS-шрифт для стиля блока. font берётся только из таблицы State.fonts (текст из файла
// проекта недоверенный); неизвестный id (нет шрифта на этом компьютере, 082-7) — Montserrat с Ж и К из стиля,
// как в Go. Жирный и курсив — только если у семейства есть такое начертание (у Marck Script их нет).
// Следом — Montserrat: браузер берёт знак, которого нет в шрифте (№, ₽), оттуда же, откуда Go в JPEG,
// а не из системного.
export function fontCss(st: Pick<TextStyle, 'font' | 'bold' | 'italic'>, fonts: FontFamily[]): FontCss {
  const f = fonts.find((x) => x.id === st.font) ?? fonts.find((x) => x.id === DEFAULT_FONT)
  if (!f) return { family: 'sans-serif', weight: 400, style: 'normal' }
  const bold = !!st.bold && f.faces.some((x) => x.bold)
  const italic = !!st.italic && f.faces.some((x) => x.italic)
  const name = (x: FontFamily) => `"${x.css.replace(/["\\]/g, '')}"`
  const fb = f.id !== DEFAULT_FONT ? fonts.find((x) => x.id === DEFAULT_FONT) : undefined
  const family = [name(f), ...(fb ? [name(fb)] : []), 'sans-serif'].join(', ')
  return { family, weight: bold ? 700 : 400, style: italic ? 'italic' : 'normal' }
}

// Доступны ли Ж и К (у гарнитуры есть такое начертание).
export function styleSupport(fontId: string, fonts: FontFamily[]): { bold: boolean; italic: boolean } {
  const f = fonts.find((x) => x.id === fontId)
  return { bold: !!f?.faces.some((x) => x.bold), italic: !!f?.faces.some((x) => x.italic) }
}

// pt → мм (кегль в Go и в SVG разворота — в мм).
export const ptToMm = (pt: number) => (pt * 25.4) / 72

const COLOR_RE = /^#[0-9a-fA-F]{6}$/
// Цвет из файла проекта попадает в style="": только #rrggbb, иначе запасной.
export const safeColor = (c: string, fallback = '#000000') => (COLOR_RE.test(c) ? c : fallback)

// Длина строки для SVG `textLength` (082-9): ширина из Go, чтобы центр и края совпали с выгрузкой для любого шрифта
// (кернинг кириллицы у своих файлов Go не читает, §7.1). Нет ширины (старый ответ Go, пустая строка) — как раньше.
export const lineLen = (w: number | undefined): number | undefined => (w !== undefined && Number.isFinite(w) && w > 0 ? w : undefined)

// style="" строки SVG-текста: размер в единицах viewBox (мм разворота).
export function svgTextCss(st: TextStyle, fonts: FontFamily[]): string {
  const f = fontCss(st, fonts)
  return `font-family:${f.family};font-weight:${f.weight};font-style:${f.style};font-size:${ptToMm(st.size)}px;fill:${safeColor(st.color)};${TEXT_RENDER_CSS}`
}

// style="" редактора на странице: те же шрифт, кегль (px = мм × pxPerMm), цвет и выравнивание,
// межстрочный 1.2 — как в Go (internal/text).
export function editorCss(st: TextStyle, fonts: FontFamily[], pxPerMm: number): string {
  const f = fontCss(st, fonts)
  const align = st.align === 'left' || st.align === 'right' ? st.align : 'center'
  return `font-family:${f.family};font-weight:${f.weight};font-style:${f.style};font-size:${ptToMm(st.size) * pxPerMm}px;line-height:1.2;color:${safeColor(st.color)};text-align:${align}`
}

// --- проверка окном программы (082-14) ---

// Судьба начертания: true — окно его не открыло и Go откатил копию. Один результат на файл: проверку зовут и загрузка
// из списка, и событие `fonts` (открыт собранный альбом) — вторая не ждёт заново и не откатывает дважды.
const fate = new Map<string, Promise<boolean>>()

// Открывается ли файл окном: FontFace.load() идёт через тот же разбор OTS, что и текст на холсте. Имя семейства
// служебное, в document.fonts проверочное лицо не попадает.
const openable = (file: string) =>
  new FontFace('SA check', fontUrl(file)).load().then(
    () => true,
    () => false,
  )

// Проверяет начертания (ключи '<16 hex>.ttf|otf'): отказавшие уходят в DropFonts одним вызовом (Go удалит только
// свежие этого сеанса). Возвращает откатанные. Все открылись — Go не зовётся. Ошибка DropFonts идёт наверх.
export async function checkFaces(files: string[]): Promise<string[]> {
  const uniq = [...new Set(files)]
  const fresh = uniq.filter((f) => !fate.has(f))
  if (fresh.length > 0) {
    const batch = (async () => {
      const ok = await Promise.all(fresh.map(openable))
      const bad = fresh.filter((_, i) => !ok[i])
      return new Set(bad.length > 0 ? await DropFonts(bad) : [])
    })()
    for (const f of fresh)
      fate.set(
        f,
        batch.then((gone) => gone.has(f)),
      )
    // Откатанное и неудавшееся проверяется заново при следующей загрузке того же файла; открывшееся помним.
    batch.then(
      (gone) => gone.forEach((f) => fate.delete(f)),
      () => fresh.forEach((f) => fate.delete(f)),
    )
  }
  const gone = await Promise.all(uniq.map(async (f) => ((await fate.get(f)) ? f : '')))
  return gone.filter(Boolean)
}

// Файлы начертаний своих гарнитур (group 'my').
export const ownFaceFiles = (fonts: FontFamily[]): string[] =>
  fonts.filter((f) => f.group === 'my').flatMap((f) => f.faces.map((x) => x.file))

// Событие `fonts` после старта: зарегистрировать новое и проверить свежие свои начертания (альбом открыт вместе
// с папкой «Шрифты», 082-10). Откат тихий: гарнитуры не станет в списке, полоса «Нет шрифтов» останется.
export function loadAndCheck(fonts: FontFamily[], notify?: (text: string) => void) {
  const fresh = ownFaceFiles(fonts).filter((f) => !isRegistered(f))
  loadFonts(fonts, notify)
  if (fresh.length > 0) checkFaces(fresh).catch(() => {})
}

export const REFUSED = 'окно программы не смогло открыть этот шрифт'

// Итог загрузки после проверки окном: откатанные начертания — пропуски с причиной; гарнитура, у которой не осталось
// ни одного, не считается добавленной (её не ставят тексту). Гонка: событие `fonts` → loadAndCheck → DropFonts может
// прийти раньше ответа ImportFonts, и в списке гарнитуры уже нет. Поэтому откатанную отличает FontAdded.files (файлы,
// записанные этой загрузкой), а не список: все её файлы в dropped и в списке не осталось других её начертаний.
// fonts — список (или его геттер: читается после проверки, когда событие уже могло прийти). Файл, чью гарнитуру
// не определить, — пропуск под общим именем.
export async function settleImport(
  imp: FontImport,
  fonts: FontFamily[] | (() => FontFamily[]),
): Promise<FontImport & { dropped: string[] }> {
  const dropped = await checkFaces(imp.faces)
  if (dropped.length === 0) return { ...imp, dropped }
  const list = typeof fonts === 'function' ? fonts() : fonts
  const rest = new Set(dropped)
  const skipped = [...imp.skipped]
  const added = imp.added.filter((a) => {
    const fam = list.find((x) => x.id === a.id)
    const own = a.files ?? []
    // Остались ли у гарнитуры живые начертания (старые, не из этой загрузки).
    const alive = fam?.faces.some((x) => !rest.has(x.file)) ?? false
    // files пусто (загрузка ничего не писала или старый ответ) — как раньше, по списку.
    const gone = own.length > 0 ? own.every((f) => rest.has(f)) && !alive : !!fam && !alive
    if (!gone) return true
    skipped.push({ name: a.name, why: REFUSED })
    own.forEach((f) => rest.delete(f))
    fam?.faces.forEach((x) => rest.delete(x.file))
    return false
  })
  for (const file of rest) {
    const owner = imp.added.find((a) => a.files?.includes(file))
    const fam = list.find((x) => x.faces.some((y) => y.file === file))
    skipped.push({ name: owner?.name ?? fam?.name ?? 'Шрифт', why: REFUSED })
  }
  return { ...imp, added, skipped, dropped }
}

// Хвост тоста загрузки: гарнитуры без русских букв («нет русских букв: Lato Script»); нет таких — ''.
export function noCyrTail(added: FontImport['added']): string {
  const names = added.filter((a) => a.noCyr).map((a) => a.name)
  return names.length > 0 ? `нет русских букв: ${names.join(', ')}` : ''
}

// Своя гарнитура не открылась при обычной регистрации (не свежая загрузка, откатывать нечего, 082-14): тост
// по каждой гарнитуре не больше раза за сеанс (told). family — имя упавшего FontFace (css гарнитуры, браузер
// может вернуть его в кавычках); не своя или уже сообщали — ''.
export function failedText(family: string, fonts: FontFamily[], told: Set<string>): string {
  const css = family.replace(/^["']|["']$/g, '')
  const f = fonts.find((x) => x.group === 'my' && x.css === css)
  if (!f || told.has(f.id)) return ''
  told.add(f.id)
  return `Шрифт «${f.name}» не открылся в окне программы: на холсте вместо него Montserrat. Уберите его (×) и загрузите файл заново.`
}
