// Справочник функций (спека 033): модель статьи и общий список. Тексты — articles-a.ts, articles-e.ts, articles-d.ts (раздел 4), articles-b.ts, articles-b2.ts, articles-c.ts, articles-c2.ts, articles-f.ts и articles-g.ts;
// уровни берутся из FEATURES (levels.ts), здесь они не дублируются. Правка проекта из справочника запрещена.
import type { Base, FeatureId } from '../../base/prefs/levels'
import type { SectionName } from '../../base/prefs/view.svelte'
import type { ActId } from '../../editor/photomenu/photoacts'
import { PART_A } from './articles-a'
import { PART_E } from './articles-e'
import { PART_D } from './articles-d'
import { PART_B } from './articles-b'
import { PART_B2 } from './articles-b2'
import { PART_C } from './articles-c'
import { PART_C2 } from './articles-c2'
import { PART_F } from './articles-f'
import { PART_G } from './articles-g'

export type GroupId = 'start' | 'photos' | 'spreads' | 'onspread' | 'text' | 'cover' | 'view' | 'manual'

// Разделы списка в порядке показа; каждая статья — ровно в одном (по group). Разделов «Новое в …» нет (решение 0.11 №54).
export const GUIDE_GROUPS: { id: GroupId; title: string }[] = [
  { id: 'start', title: 'Начало и файлы' },
  { id: 'photos', title: 'Фото и лента' },
  { id: 'spreads', title: 'Развороты и раскладки' },
  { id: 'onspread', title: 'Фото на развороте' },
  { id: 'text', title: 'Текст и подписи' },
  { id: 'cover', title: 'Обложка и виньетка' },
  { id: 'view', title: 'Вид и холст' },
  { id: 'manual', title: 'Ручная раскладка' },
]

// Состояние, без которого показать нечего (show.ts): что делаем, если его нет.
export type Need =
  | 'spread' // есть текущий разворот (не обложка)
  | 'cover' // текущий лист — обложка
  | 'vignette' // текущий лист — виньетка
  | 'photo' // выделено фото
  | 'text' // выделен текст
  | 'grid' // разворот в «Ручной»
  | 'auto' // разворот в «Авто» (не виньетка): есть «Все раскладки»
  | 'notvig' // текущий разворот — не виньетка (и не обложка): есть «Сделать виньеткой»
  | 'libsel' // в ленте фото выделено фото: появляется полоса отметок
  | 'card' // карточка выделенного видна (у холста или в панели деталей)
  | 'idle' // ничего не выделено: видна карточка листа («Фон», «Авто / Ручная»…)
  | 'gallery' // открыта галерея раскладок
  | 'canvas' // холст вписан, не увеличен
  | `details:${SectionName}` // раздел панели деталей раскрыт

// Якорь места: ui:<data-guide> или menu:<id верхнего пункта меню>.
export type SpotKey = `ui:${string}` | `menu:${string}`

// Одно из мест функции (решение 133): своя кнопка «Показать …» в карточке. Место с level есть только с этого
// уровня и выше: на более низком вместо кнопки серая строка «на «Полном»: <place>». needs — состояние для
// этого места; без него берётся needs статьи.
export interface Place {
  label: string // текст кнопки: «Показать в меню»
  spot: SpotKey
  level?: Base
  place?: string // имя места для строки про уровень («лента действий слева»); обязательно вместе с level
  needs?: Need[]
  miss?: string // подсказка в тосте, если места на экране нет, а создать его показом нельзя (проект не правим)
}

export interface Article {
  id: string // kebab, уникальный
  title: string // ≤ 40 знаков
  group: GroupId
  feature: FeatureId // строка уровня (можно also-id: 'bgColor', 'names', 'allSpreads')
  what: string // что делает, ≤ 220 знаков
  where?: string // где, словами интерфейса
  words?: string // синонимы через пробел, строчными
  keys?: string[] // action строки HOTKEYS или её show у справочной строки
  menu?: string[] // id верхних пунктов меню: строка «Меню: Файл ▸ Экспорт…» строится из buildMenu
  how?: string // для жестов и клавиш: как сделать
  see?: string[] // «См. также»: id других статей, не своя, ≤ 4 (щелчок в карточке выбирает статью)
  acts?: ActId[] // действия фото и стикера (ACTS), о которых статья: страж полноты и индекс поиска
  spot?: SpotKey // что мигает: одно место (кнопка «Показать»)
  spots?: Place[] // несколько мест (шапка и меню, лента и меню): вместо spot, от двух штук
  needs?: Need[]
  // Места нет в разметке, а создать его показом нельзя (проект не правим): мигает запасное место с подсказкой;
  // без spot — тост «Название: подсказка». Без miss тост строится из where/how.
  miss?: { spot?: `ui:${string}`; hint: string }
}

export const ARTICLES: Article[] = [...PART_A, ...PART_E, ...PART_D, ...PART_B, ...PART_B2, ...PART_C, ...PART_C2, ...PART_F, ...PART_G]

// Места статьи в порядке показа: spots, иначе единственное spot с кнопкой «Показать»; без обоих (жест, клавиша) — пусто.
export const placesOf = (a: Article): Place[] => a.spots ?? (a.spot ? [{ label: 'Показать', spot: a.spot }] : [])

const BY_ID = new Map(ARTICLES.map((a) => [a.id, a]))
export const articleById = (id: string): Article | undefined => BY_ID.get(id)
