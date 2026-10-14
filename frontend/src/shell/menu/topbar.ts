// Сводка в шапке: «2 разворота (4 стр.)», у виньетки-листа — «3 листа» (страниц нет).
import { plural } from '../../base/core/plural'
import { unitOf } from '../../base/geom/sheet'
import type { Settings } from '../../base/core/types'

export function countLabel(spreads: number, pages: number, st: Pick<Settings, 'sheet'> | undefined): string {
  const unit = unitOf(st)
  const base = `${spreads} ${plural(spreads, unit.plural)}`
  return st?.sheet ? base : `${base} (${pages} стр.)`
}
