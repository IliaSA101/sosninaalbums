// Действие `use:seenIf={on}`: выбранная плитка не должна прятаться за краем прокрутки (окна «Развороты» и «Макеты альбома»).
export function seenIf(el: HTMLElement, on: boolean) {
  if (on) el.scrollIntoView?.({ block: 'nearest' })
  return { update: (v: boolean) => v && el.scrollIntoView?.({ block: 'nearest' }) }
}
