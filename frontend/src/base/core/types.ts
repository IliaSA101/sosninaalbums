// Точка входа типов: данные Go — в ../types/*, здесь же типы только интерфейса.
import type { BackdropScope } from '../types/sheet'

export type * from '../types/marks'
export type * from '../types/sheet'
export type * from '../types/layouts'
export type * from '../types/app'
export type * from '../types/print'

// Что тащим мышью.
export type Payload = { kind: 'photos'; ids: string[]; fromSpread?: string } | { kind: 'spread'; id: string; index: number }

// Куда отпускаем.
export type Target =
  | { kind: 'cell'; spread: string; index: number; photo: string; side: 'before' | 'after' | 'center' }
  // at — доля ширины листа под курсором 0…1 (есть только у самого листа на холсте; у соседа в ряду нет)
  | { kind: 'spread'; spread: string; at?: number }
  | { kind: 'mini'; spread: string; index: number; side: 'before' | 'after' }
  // Миниатюра фото в ленте альбома (Timeline): вставка до или после неё.
  | { kind: 'chip'; spread: string; index: number; photo: string; side: 'before' | 'after' }
  | { kind: 'new'; at: number }
  | { kind: 'library' }
  // Место разворота-сетки (пустое или занятое): сторон нет, всё место — «центр».
  | { kind: 'place'; spread: string; place: string; photo: string | null }
  // Зона «Фото фона» в карточке разворота (спека 009): фото встаёт на фон области.
  // many — тянут несколько фото: зона не подсвечивается, бросок объясняет тостом.
  | { kind: 'backdrop'; spread: string; scope: BackdropScope; many?: boolean }
  // Плитка обложки в ленте (032): бросок фото создаёт обложку; несколько фото — тост «только одно».
  | { kind: 'cover'; many?: boolean }
  // Кнопка «Подборки» в полосе ленты (083): бросок фото из ленты добавляет их в подборку.
  | { kind: 'bin' }

// Вкладка галереи под разворотом: раскладки «Авто» или сетки (спека 007).
export type GalleryTab = 'layouts' | 'grids'

// Зона последнего нажатия мыши (store.zone): Ctrl+A и Del действуют по ней.
export type Zone = 'canvas' | 'library'
