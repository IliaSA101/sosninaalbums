// Куски текста статей по уровню (080-9), по образцу {key:…|…} из base/keys/keytext.ts:
//   {can:adjust| и «Ч/б развороты»} — кусок только при включённой функции (уровень или «Свой»), иначе опускается целиком.
// Кусок начинается с разделителя (пробел, «;»), чтобы после выпадения не оставались двойные пробелы и « ;».
// Порядок: сначала метки клавиш ({key} внутри куска разрешён), потом уровни; вложенный {can} нельзя (страж).
import type { FeatureId } from '../../base/prefs/levels'

const MARK = /\{can:([A-Za-z0-9]+)\|([^}]*)\}/g
// Начало метки — для стража: id берётся и у куска с вложенной {key:…} (в нём есть «}»).
const HEAD = /\{can:([A-Za-z0-9]+)\|/g

export function fillLevels(text: string, can: (id: FeatureId) => boolean): string {
  return text.replace(MARK, (_m, id: string, body: string) => (can(id as FeatureId) ? body : ''))
}

// Id функций, названных метками текста (страж: каждый — FeatureId).
export function levelMarks(text: string): string[] {
  return [...text.matchAll(HEAD)].map((m) => m[1])
}

// Куски, внутри которых есть ещё одна {can:} (страж: вложенность не поддерживается). {key:…} убираем заранее.
export function nestedLevels(text: string): string[] {
  const plain = text.replace(/\{key:[A-Za-z0-9]+(?:\|[^}]*)?\}/g, '')
  return [...plain.matchAll(MARK)].map((m) => m[2]).filter((body) => body.includes('{can:'))
}
