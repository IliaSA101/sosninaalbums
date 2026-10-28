// Каталог действий над фото (спека 108): слово, значок и функция уровня записаны один раз.
// Его читают дерево меню фото (photomenu.ts) и столбец мини-панели (places/minipanel.ts, список «⋯»).
import type { IconName } from '../../base/ui/Icon.svelte'
import type { FeatureId } from '../../base/prefs/levels'

export type ActId =
  | 'preview'
  | 'clipCopy'
  | 'clipPaste'
  | 'turnCw'
  | 'turnCcw'
  | 'straighten'
  | 'tilt'
  | 'flipH'
  | 'flipV'
  | 'fit'
  | 'contain'
  | 'cover'
  | 'fitPhoto'
  | 'resetFrame'
  | 'faceRef'
  | 'rect'
  | 'rounded'
  | 'oval'
  | 'border'
  | 'gray'
  | 'backdrop'
  | 'layer'
  | 'caption'
  | 'name'
  | 'swap'
  | 'same'
  | 'more'
  | 'explorer'
  | 'copy'
  | 'uses'
  | 'relink'
  | 'replace'
  | 'undraft'
  | 'delete'

export interface Act {
  label: string // слово в меню и в списке «⋯»
  icon?: IconName // значок столбца
  feature?: FeatureId // функция уровня (levels.ts); нет — есть всегда, пока есть само меню или панель
  cmd?: string // команда реестра: клавиша справа в меню (keymap.hint)
}

export const ACTS: Record<ActId, Act> = {
  preview: { label: 'Крупный просмотр', icon: 'preview', cmd: 'photoPreview' },
  clipCopy: { label: 'Копировать', icon: 'copy', cmd: 'copyItems' },
  clipPaste: { label: 'Вставить', icon: 'paste', cmd: 'pasteItems' },
  turnCw: { label: 'Повернуть ↻ 90°', icon: 'turn-cw', feature: 'turn', cmd: 'turnCw' },
  turnCcw: { label: 'Повернуть ↺ 90°', icon: 'turn-ccw', feature: 'turn', cmd: 'turnCcw' },
  straighten: { label: 'Выпрямить по линии', icon: 'level', feature: 'tilt', cmd: 'straighten' },
  tilt: { label: 'Наклон в рамке', icon: 'tilt', feature: 'tilt' },
  flipH: { label: 'Зеркало ↔', icon: 'flip-h', feature: 'flip', cmd: 'flipH' },
  flipV: { label: 'Зеркало ↕', icon: 'flip-v', feature: 'flip', cmd: 'flipV' },
  fit: { label: 'Целиком / с обрезкой', icon: 'contain', feature: 'frame' },
  contain: { label: 'Целиком', icon: 'contain', feature: 'frame' },
  cover: { label: 'С обрезкой', icon: 'cover', feature: 'frame' },
  fitPhoto: { label: 'Подогнать место под фото', icon: 'fit-photo', feature: 'frame' },
  resetFrame: { label: 'Сбросить кадр', icon: 'reset', feature: 'frame' },
  faceRef: { label: 'Остальные — как это лицо', icon: 'vignette', feature: 'vignette' },
  rect: { label: 'Прямоугольное', icon: 'shape-rect', feature: 'frame' },
  rounded: { label: 'Скруглённое', icon: 'shape-round', feature: 'frame' },
  oval: { label: 'Круглое', icon: 'shape-oval', feature: 'frame' },
  border: { label: 'Рамочка…', icon: 'frame', feature: 'border' },
  gray: { label: 'Сделать ч/б', icon: 'gray', feature: 'adjust', cmd: 'gray' },
  backdrop: { label: 'На фон', icon: 'image', feature: 'bgPhoto' },
  layer: { label: 'Слой', icon: 'layers', feature: 'layers' },
  caption: { label: 'Подпись', icon: 'caption', feature: 'captions' },
  name: { label: 'Имя под фото', icon: 'caption', feature: 'captions' },
  swap: { label: 'Поменять местами', icon: 'swap', feature: 'manual' },
  same: { label: 'Одинаковый размер', icon: 'same-size', feature: 'manual' },
  more: { label: 'Ещё', icon: 'more' },
  explorer: { label: 'Показать в папке', icon: 'folder' }, // слово по ОС подставляет дерево меню (revealLabel)
  copy: { label: 'Копировать имя файла', icon: 'copy-name' },
  uses: { label: 'Где ещё стоит', icon: 'locate' },
  relink: { label: 'Перепривязать…', icon: 'link' },
  replace: { label: 'Заменить…', icon: 'replace' },
  undraft: { label: 'Сделать обычной надписью' }, // 131: заготовка макета → обычный текст (Go снимает признак и [ ])
  delete: { label: 'Удалить', icon: 'trash' },
}

export const actOk = (id: ActId, can: (f: FeatureId) => boolean): boolean => {
  const f = ACTS[id].feature
  return !f || can(f)
}

// Подменю меню фото: название группы (пункты и порядок — в photomenu.ts).
export type GroupId = 'turn' | 'mirror' | 'frame' | 'shape' | 'file'
export const GROUP_LABEL: Record<GroupId, string> = {
  turn: 'Повернуть',
  mirror: 'Зеркало',
  frame: 'Кадр',
  shape: 'Форма',
  file: 'Файл',
}

// Значок группы = значок её главного действия (как у кнопки мини-панели и правой панели).
export const GROUP_ICON: Record<GroupId, IconName> = {
  turn: 'turn-cw',
  mirror: 'flip-h',
  frame: 'contain',
  shape: 'shape-round',
  file: 'folder',
}
