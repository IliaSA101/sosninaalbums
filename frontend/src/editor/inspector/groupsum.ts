// Группы карточек (спека 124, 0.26-124-28): чистая логика в одном месте — какие группы есть на уровне и что пишет
// свёрнутая группа. Видимость решает тот же prefs.can, что и тело карточки; пустая строка значения — «писать нечего».
import type { FeatureId } from '../../base/prefs/levels'
import type { GroupId } from '../../base/prefs/cardgroups.svelte'
import type { ShapeKey } from '../../base/geom/shape'
import { signed } from '../photoprops/adjustrows'

// Какие функции дают группе строки; хватает любой из них. Пусто — строки есть на любом уровне («данные, а не инструменты»).
// Список совпадает с гейтами тела (InspectorFrame, InspectorSpread): правка тела — правка здесь.
export const GROUP_NEEDS: Record<GroupId, readonly FeatureId[]> = {
  'photo.frame': ['frame', 'turn', 'tilt', 'flip'],
  'photo.tone': ['adjust', 'photoFade'],
  'photo.shape': ['frame', 'border'],
  'photo.name': ['captions'],
  'photo.place': ['manual', 'layers'],
  'photo.about': [],
  'spread.vig': [],
  'spread.bg': ['bgColor', 'bgPhoto'],
  'spread.caps': ['captions'],
  'cover.layout': [],
  'cover.spine': [],
  'cover.bg': ['bgColor', 'bgPhoto'],
  'cover.caps': ['captions'],
  'cover.size': [],
}

// Есть ли у группы хоть одна строка на текущем уровне; нет — заголовок не рисуется (иначе пустое тело).
export function groupShown(id: GroupId, can: (f: FeatureId) => boolean): boolean {
  const needs = GROUP_NEEDS[id]
  return needs.length === 0 || needs.some(can)
}

// Тащат фото — группа с зоной «Фото фона» раскрыта на время переноса, без записи в память (0.26-124-23).
export const dropOpen = (d: { active: boolean; payload: { kind: string } | null }): boolean => d.active && d.payload?.kind === 'photos'

// Число с запятой и минусом U+2212, не больше одного знака после запятой: «2,5», «−3», «195,7».
export function num(v: number): string {
  const r = Math.round(v * 10) / 10
  return (r < 0 ? '−' : '') + String(Math.abs(r)).replace('.', ',')
}

const join = (parts: (string | false | undefined)[]): string => parts.filter(Boolean).join(' · ')

// «Кадр»: «120 % · с обрезкой · 2,5°»; по умолчанию «100 %»; «Вписано целиком» — «целиком».
export function frameSum(v: { pct: number; contain: boolean; tilt: number }): string {
  const scale = v.contain ? 'целиком' : `${v.pct} %`
  return join([scale, !v.contain && v.pct > 100 && 'с обрезкой', v.tilt !== 0 && `${num(v.tilt)}°`])
}

// «Цвет и тон»: «ч/б · +10 · −5 · 20 %» (только не по умолчанию) или «цвет».
export function toneSum(v: { gray: boolean; bright: number; contrast: number; fade: number }): string {
  return (
    join([v.gray && 'ч/б', v.bright !== 0 && signed(v.bright), v.contrast !== 0 && signed(v.contrast), v.fade > 0 && `${v.fade} %`]) ||
    'цвет'
  )
}

const SHAPE_NAME: Record<ShapeKey, string> = { rect: 'прямоугольник', round: 'скруглённый', oval: 'круг' }

// «Форма и рамочка»: «круг · Паспарту» или «прямоугольник · без рамочки». border — имя рамочки, пусто — нет.
export const shapeSum = (v: { shape: ShapeKey; border: string }): string => join([SHAPE_NAME[v.shape], v.border || 'без рамочки'])

// «Имя на фото»: «Иванов Иван» или «нет».
export const nameSum = (v: { last: string; first: string; patr: string }): string =>
  [v.last, v.first, v.patr]
    .map((s) => s.trim())
    .filter(Boolean)
    .join(' ') || 'нет'

// «Место»: размер места в мм.
export const placeSum = (v: { w: number; h: number }): string => `${num(v.w)} × ${num(v.h)} мм`

// «О снимке»: размер в px; нет данных — пусто.
export const aboutSum = (v: { w: number; h: number }): string => (v.w > 0 && v.h > 0 ? `${v.w}×${v.h} px` : '')

// «Фон» листа и обложки: чем занят.
export type BgKind = 'album' | 'color' | 'photo' | 'pattern'
const BG_TEXT: Record<BgKind, string> = { album: 'как в альбоме', color: 'свой цвет', photo: '+ фото фона', pattern: 'узор' }
export const bgSum = (k: BgKind): string => BG_TEXT[k]

// Чем занят фон листа: фото главнее узора, узор — своего цвета. Фон альбома листу не принадлежит (ownBg отфильтрован вызывающим).
export const bgKind = (v: { photo: boolean; pattern: boolean; color: boolean }): BgKind =>
  v.photo ? 'photo' : v.pattern ? 'pattern' : v.color ? 'color' : 'album'

// «Подписи» листа и обложки: «к каждому фото» / «свои» / «нет».
export type CapsKind = 'each' | 'own' | 'none'
const CAPS_TEXT: Record<CapsKind, string> = { each: 'к каждому фото', own: 'свои', none: 'нет' }
export const capsSum = (k: CapsKind): string => CAPS_TEXT[k]

// Подписи листа: общий шаблон «к каждому фото» (стиль включён и шаблон не пуст) / свои (стиль без шаблона или подписи вручную) / нет.
export const capsKind = (v: { styled: boolean; template: boolean; own: number }): CapsKind =>
  v.styled && v.template ? 'each' : v.styled || v.own > 0 ? 'own' : 'none'

// «Корешок»: «надпись · плашка» / «без плашки».
export const spineSum = (v: { text: boolean; plate: boolean }): string => join([v.text && 'надпись', v.plate ? 'плашка' : 'без плашки'])

// «Размеры для типографии»: формат и строка корешка.
export const sizeSum = (preset: string, spineLine: string): string => join([preset, spineLine])
