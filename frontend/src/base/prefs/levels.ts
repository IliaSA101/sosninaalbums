// Уровни интерфейса (спеки 010, 011): Простой · Стандартный · Полный · Свой. Уровень прячет
// инструменты, а не данные: сделанное выше видно, печатается и выгружается. Чистая логика; состояние
// и вызов Go — prefs.svelte.ts, набор «Своего» — levelcustom.ts. FEATURES — единственный источник:
// из него строятся меню, мини-панель, клавиши, справка и окна уровней.

// Основа — один из трёх готовых уровней; «Свой» — набор строк поверх основы.
export type Base = 'simple' | 'standard' | 'full'
export type Level = Base | 'custom'

// Столбцы с ✓ в матрице (готовые уровни).
export const LEVELS: readonly Base[] = ['simple', 'standard', 'full']
// Карточки, меню, значок.
export const ALL_LEVELS: readonly Level[] = ['simple', 'standard', 'full', 'custom']

// Набор «Своего» — разница с основой (design §1.1): on — строки сверх основы, off — снятые из основы.
// В списках могут быть чужие id (config.json более новой версии): их не трогаем.
export interface Custom {
  base: Base
  on: string[]
  off: string[]
}

export const LEVEL_TITLE: Record<Level, string> = {
  simple: 'Простой',
  standard: 'Стандартный',
  full: 'Полный',
  custom: 'Свой',
}

// Подсказка уровня в значке шапки.
export const LEVEL_HINT: Record<Level, string> = {
  simple: 'Папки, автосборка, варианты раскладки, отметки, подписи, цвет фона, обложка, экспорт',
  standard: 'Плюс текст, кадр фото, зеркало, зум, группа, фоны и фото на фоне, меню правой кнопки',
  full: 'Плюс ручная раскладка, шаблоны, поворот мест, слои, выравнивание, линейки, плавающие панели',
  custom: 'Ваш набор функций; основа — любой из уровней',
}

// Подсказка «Своего» с названием основы.
export const customHint = (base: Base): string => `Ваш набор, основа — ${LEVEL_TITLE[base]}`

// Пункты карточек (design §5.2), каждый ≤ 26 знаков: в карточке без переноса. У «Своего» четвёртая
// строка зависит от основы — customPoints.
export const LEVEL_POINTS: Record<Level, readonly string[]> = {
  simple: ['Папки и автосборка', 'Варианты раскладки', 'Подписи и цвет фона', 'Обложка', 'Экспорт в JPEG'],
  standard: ['Всё из Простого', 'Текст на развороте', 'Кадр, зеркало, наклон', 'Виньетка в альбоме', 'Фоны и фото на фоне'],
  full: ['Всё из Стандартного', 'Ручная раскладка', 'Шаблоны раскладок', 'Слои и выравнивание', 'Линейки и панели'],
  custom: ['Функции на выбор', 'Основа — любой уровень', 'Нужное включится само'],
}

export const customPoints = (base: Base): string[] => [...LEVEL_POINTS.custom, `Основа сейчас: ${LEVEL_TITLE[base]}`]

export const RANK: Record<Base, number> = { simple: 0, standard: 1, full: 2 }

// Основа из строки Go/config; неизвестное (в том числе custom) — «Простой».
export function normBase(s: unknown): Base {
  return s === 'standard' || s === 'full' ? s : 'simple'
}

// Строка Go/config приводится к уровню; неизвестное — «Простой» (как нормализует Go).
export function normLevel(s: unknown): Level {
  return s === 'custom' ? 'custom' : normBase(s)
}

// Идентификаторы функций (строка таблицы может нести несколько).
export type FeatureId =
  | 'base'
  | 'variants'
  | 'timeline'
  | 'captions'
  | 'bgColor'
  | 'menu'
  | 'turn'
  | 'text'
  | 'photoMenu'
  | 'frame'
  | 'flip'
  | 'adjust'
  | 'tilt'
  | 'zoom'
  | 'names'
  | 'allSpreads'
  | 'groupSelect'
  | 'clipboard' // 099: Ctrl+C, X, V, D
  | 'bgPhoto'
  | 'decor'
  | 'manual'
  | 'rotate'
  | 'groupXf'
  | 'layers'
  | 'align'
  | 'rulers'
  | 'templates'
  | 'textRotate'
  | 'panels'
  | 'marks'
  | 'bins' // 083: подборки фото
  | 'vignette'
  | 'border' // 079: рамочки, строка decor
  | 'photoFade' // 079: прозрачность фото, строка adjust

// Строка матрицы = единица переключения в «Своём»; её идентификатор — главный id.
export type RowId = FeatureId

export interface Feature {
  id: RowId // главный; остальные — в also
  also?: FeatureId[]
  group: 'Основа' | 'Фото и текст' | 'Холст' | 'Ручная раскладка'
  title: string // строка окна уровней
  short: string // краткое имя: подсказки, тосты, «Включено вместе: …»
  min: Base // с какого уровня входит в основу
  needs?: RowId[] // опоры: без них функция мертва (design §2.2)
}

// Уровни вложены: функция с min доступна и на всех более высоких. Строки делим там, где есть опора
// или фотографу есть смысл выбирать (design §2.1); id функций не менялись — гейты в компонентах прежние.
export const FEATURES: Feature[] = [
  {
    id: 'base',
    group: 'Основа',
    title: 'Папки, лента, перенос и обмен, автосборка, обложка, экспорт с окном, отмена, «Найти пропавшие», панель деталей',
    short: 'Основа',
    min: 'simple',
  },
  { id: 'menu', group: 'Основа', title: 'Строка меню, стартовое окно, окно уровней', short: 'Строка меню', min: 'simple' },
  { id: 'variants', group: 'Основа', title: 'Варианты раскладки, раскладки страниц', short: 'Варианты раскладки', min: 'simple' },
  { id: 'timeline', group: 'Основа', title: 'Разрез и склейка, крупный просмотр', short: 'Разрез и склейка', min: 'simple' },
  {
    id: 'captions',
    also: ['bgColor'],
    group: 'Основа',
    title: 'Подписи, цвет фона и палитра',
    short: 'Подписи и цвет фона',
    min: 'simple',
  },
  { id: 'marks', group: 'Основа', title: 'Звёзды, метки, флажки и фильтр ленты', short: 'Отметки фото', min: 'simple' },
  {
    id: 'bins',
    group: 'Основа',
    title: 'Подборки фото: кнопка в ленте, B, перетаскивание',
    short: 'Подборки фото',
    min: 'standard',
  },
  { id: 'turn', group: 'Фото и текст', title: 'Мини-панель: Подпись · ↻ 90° · Удалить', short: 'Поворот фото на 90°', min: 'simple' },
  { id: 'text', group: 'Фото и текст', title: 'Свободный текст', short: 'Свободный текст', min: 'standard' },
  { id: 'photoMenu', group: 'Фото и текст', title: 'Меню по правой кнопке', short: 'Меню по правой кнопке', min: 'standard' },
  { id: 'frame', group: 'Фото и текст', title: 'Кадр (✥, ползунок), «Целиком / С обрезкой», форма места', short: 'Кадр', min: 'standard' },
  { id: 'flip', group: 'Фото и текст', title: 'Зеркало фото', short: 'Зеркало фото', min: 'standard' },
  {
    id: 'adjust',
    also: ['photoFade'],
    group: 'Фото и текст',
    title: 'Ч/б, яркость, контраст и прозрачность фото',
    short: 'Ч/б, тон и прозрачность фото',
    min: 'standard',
  },
  { id: 'tilt', group: 'Фото и текст', title: 'Наклон фото', short: 'Наклон фото', min: 'standard' },
  { id: 'bgPhoto', group: 'Фото и текст', title: 'Фото на фоне и режим фона', short: 'Фото на фоне', min: 'standard' },
  {
    id: 'decor',
    also: ['border'],
    group: 'Фото и текст',
    title: 'Фоны, стикеры и рамочки из библиотеки',
    short: 'Фоны, стикеры и рамочки',
    min: 'standard',
    needs: ['bgPhoto'],
  },
  { id: 'vignette', group: 'Фото и текст', title: 'Виньетка: лист учителей и учеников', short: 'Виньетка', min: 'standard' },
  {
    id: 'textRotate',
    group: 'Фото и текст',
    title: 'Поворот текста, кегль ручкой',
    short: 'Поворот текста',
    min: 'full',
    needs: ['text'],
  },
  {
    id: 'zoom',
    also: ['names', 'allSpreads'],
    group: 'Холст',
    title: 'Зум, названия разворотов, «Все развороты»',
    short: 'Зум и названия',
    min: 'standard',
  },
  { id: 'groupSelect', group: 'Холст', title: 'Выделение рамкой, Ctrl+A, удаление группы', short: 'Выделение группы', min: 'standard' },
  {
    id: 'clipboard',
    group: 'Холст',
    title: 'Копировать и вставить (Ctrl+C, X, V, D)',
    short: 'Копировать и вставить',
    min: 'standard',
  },
  { id: 'rulers', group: 'Холст', title: 'Линейки', short: 'Линейки', min: 'full' },
  { id: 'panels', group: 'Холст', title: 'Плавающие панели', short: 'Плавающие панели', min: 'full', needs: ['manual'] },
  {
    id: 'manual',
    group: 'Ручная раскладка',
    title: 'Ручная раскладка: места, ручки, X/Y/Ш/В, привязка',
    short: 'Ручная раскладка',
    min: 'full',
  },
  { id: 'rotate', group: 'Ручная раскладка', title: 'Поворот мест', short: 'Поворот мест', min: 'full', needs: ['manual'] },
  { id: 'layers', group: 'Ручная раскладка', title: 'Слои', short: 'Слои', min: 'full', needs: ['manual'] },
  { id: 'align', group: 'Ручная раскладка', title: 'Выравнивание', short: 'Выравнивание', min: 'full', needs: ['manual'] },
  {
    id: 'templates',
    group: 'Ручная раскладка',
    title: 'Шаблоны, копирование и отражение раскладки',
    short: 'Шаблоны',
    min: 'full',
    needs: ['manual'],
  },
  {
    id: 'groupXf',
    group: 'Ручная раскладка',
    title: 'Масштаб и поворот группы',
    short: 'Масштаб и поворот группы',
    min: 'full',
    needs: ['manual', 'groupSelect'],
  },
]

// Строки окна уровней по группам, в порядке первого появления группы в таблице.
export function featureGroups(): { title: Feature['group']; rows: Feature[] }[] {
  const out: { title: Feature['group']; rows: Feature[] }[] = []
  for (const f of FEATURES) {
    const g = out.find((x) => x.title === f.group)
    if (g) g.rows.push(f)
    else out.push({ title: f.group, rows: [f] })
  }
  return out
}

const MIN = new Map<FeatureId, Base>(FEATURES.flatMap((f) => [f.id, ...(f.also ?? [])].map((id) => [id, f.min] as const)))

// Строка таблицы, в которую входит id (главный или из also): единица переключения в «Своём».
const ROW_OF = new Map<FeatureId, RowId>(FEATURES.flatMap((f) => [f.id, ...(f.also ?? [])].map((id) => [id, f.id] as const)))
export const rowOf = (id: FeatureId): RowId => ROW_OF.get(id) ?? id

// Нижний уровень, с которого функция входит в основу.
export const minLevel = (id: FeatureId): Base => MIN.get(id) ?? 'simple'

// Доступность на готовом уровне. 'custom' без набора — как «Простой» (набор считает levelcustom.features).
export const can = (id: FeatureId, level: Level): boolean => RANK[level === 'custom' ? 'simple' : level] >= RANK[minLevel(id)]

// Объекты, которые остаются на развороте при любом уровне (вопрос 5): текст, готовая подпись,
// пустое место и места «Ручной», стикер (048: ниже «Стандартного» виден и печатается, но не выделяется).
export type ObjectKind = 'text' | 'caption' | 'place' | 'sticker'
export const OBJECT_FEATURE: Record<ObjectKind, RowId> = { text: 'text', caption: 'captions', place: 'manual', sticker: 'decor' }

// Какой уровень нужен, чтобы править объект; null — править можно (или уровня достаточно).
// Для 'custom' набор не знаем — см. levelcustom.blockerOf.
export function blocker(kind: ObjectKind, level: Level): Base | null {
  const f = OBJECT_FEATURE[kind]
  return can(f, level) ? null : minLevel(f)
}

// Текст тоста при щелчке по такому объекту.
export const blockedText = (need: Base): string => `Править — в уровне «${LEVEL_TITLE[need]}»`

// То же в «Своём»: называем функцию, которую надо включить.
export const blockedRowText = (short: string): string => `Править — включите «${short}»`
