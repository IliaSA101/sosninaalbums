// «Был черновик»: несохранённый альбом с содержимым (U4). Go молча кладёт такой в резервные копии при «Новом»,
// фотограф должен узнать, где он. Пустой черновик и сохранённый альбом (путь есть — всё в файле) не в счёт.

export interface DraftView {
  path: string
  folders: readonly unknown[]
  spreads: readonly unknown[]
}

export const DRAFT_NOTE = 'Прежний черновик — в резервных копиях'

export function hadDraft(v: DraftView): boolean {
  return v.path === '' && (v.spreads.length > 0 || v.folders.length > 0)
}
