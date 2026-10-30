import * as Api from '../../../wailsjs/go/app/App'
import { capFirst } from '../words/errtext'
import { keymap } from '../keys/keymap.svelte'

// rev — ревизия альбома, на которую рассчитан тост с «Вернуть»: другая ревизия (Ctrl+Z, любая правка) гасит тост.
// hold (150) — тост правки без вопроса: держится, пока окно не получит фокус, и ещё HOLD_AFTER_MS; смена ревизии у него снимает
// только «Вернуть» (action.undo) и rev. paths — полные пути в тексте: их рисуют в <bdi>, чтобы чужой текст не переворачивал соседей.
export type ToastAction = { label: string; run: () => void; undo?: boolean }
export type Toast = { text: string; kind: 'info' | 'error'; action?: ToastAction; rev?: number; hold?: boolean; paths?: string[] }
export const HOLD_AFTER_MS = 8000

// Что тостам «Вернуть» нужно от Store (структурный тип: цикла импортов со store нет).
export interface ToastHost {
  readonly view: { rev: number }
  readonly delQueue: Promise<unknown>
  notify(text: string, kind?: Toast['kind'], action?: Toast['action'], ms?: number): void
  hold(text: string, o?: HoldOpts): void
  run(p: Promise<unknown>): Promise<boolean>
  goId(id: string): void
  toast: Toast | null
}

export interface HoldOpts {
  action?: ToastAction
  rev?: number
  paths?: string[]
}

// Окно программы сейчас в фокусе. Без document (тесты в node) считаем, что да.
const focused = (): boolean => typeof document === 'undefined' || document.hasFocus()

// Одно сообщение внизу окна и его таймер.
export class Toasts {
  toast = $state<Toast | null>(null)
  private timer = 0
  private offFocus: (() => void) | null = null

  // Снять таймер и ожидание фокуса: тост заменён или обновлён.
  private release() {
    clearTimeout(this.timer)
    this.offFocus?.()
    this.offFocus = null
  }

  // ms — сколько держать тост; без него 8 с с кнопкой и 4 с без (итог подгона по лицам читают дольше — 20 с).
  notify(text: string, kind: Toast['kind'] = 'info', action?: Toast['action'], ms?: number) {
    // Ошибки Go пишутся со строчной буквы.
    this.release()
    this.toast = { text: kind === 'error' ? capFirst(text) : text, kind, action }
    this.timer = window.setTimeout(() => (this.toast = null), ms ?? (action ? 8000 : 4000))
  }

  // Тост правки без вопроса (150, design §7.3): виден, пока окно не получит фокус, и ещё HOLD_AFTER_MS; окно при этом не поднимается.
  // Повторный вызов (склейка правок одного клиента) обновляет текст и ждёт заново. Фокус — событием окна, как panels/folders/autocheck.ts.
  hold(text: string, o: HoldOpts = {}) {
    this.release()
    this.toast = { text, kind: 'info', action: o.action, rev: o.rev, hold: true, paths: o.paths }
    const expire = () => {
      this.timer = window.setTimeout(() => (this.toast = null), HOLD_AFTER_MS)
    }
    if (focused()) return expire()
    const on = () => {
      this.release()
      if (this.toast?.hold) expire()
    }
    window.addEventListener('focus', on)
    this.offFocus = () => window.removeEventListener('focus', on)
  }
}

// Тост с кнопкой «Вернуть» = Undo, как у разреза: безопасно, только если после правки ничего не менялось.
// back — разворот, на который перейти после возврата. at — ревизия, на которую рассчитан тост, если она не нынешняя
// (тост правки агента: view с новой ревизией пришёл раньше события, нынешняя уже может быть дальше — тогда «Вернуть» откажет).
export function undoAction(h: ToastHost, rev: number, back?: string): ToastAction {
  return {
    label: 'Вернуть',
    undo: true,
    run: async () => {
      // Идущее следом удаление ещё может менять альбом: сначала дожидаемся очереди, потом сверяем ревизию.
      await h.delQueue
      if (h.view.rev !== rev) return h.notify(keymap.tip('Уже были другие изменения — используйте «Отменить»', 'undo'))
      if ((await h.run(Api.Undo())) && back) h.goId(back)
    },
  }
}
export function undoToast(h: ToastHost, text: string, back?: string, at?: number) {
  const rev = at ?? h.view.rev
  h.notify(text, 'info', undoAction(h, rev, back))
  if (h.toast) h.toast.rev = rev
}

// Тост «Обложка в стиле «…»» (131 G5): кнопка «Без оформления» — Undo и тот же бросок обычным CreateCover, одним нажатием.
// Не прошёл Undo (или после обложки были другие правки) — ничего не делаем, как «Вернуть».
export function coverLookToast(h: ToastHost & { goCover(): void }, look: string, photoID: string) {
  const rev = h.view.rev
  h.notify(`Обложка в стиле «${look}»`, 'info', {
    label: 'Без оформления',
    run: async () => {
      await h.delQueue
      if (h.view.rev !== rev) return h.notify(keymap.tip('Уже были другие изменения — используйте «Отменить»', 'undo'))
      if (!(await h.run(Api.Undo()))) return
      if (await h.run(Api.CreateCover('', photoID))) h.goCover()
    },
  })
  if (h.toast) h.toast.rev = rev
}

// Итоговый тост правки, добавившей фото (042, 060). notes — заметки с «Вернуть» (отменяют весь шаг);
// named — фото, получившие имя из названия файла: с заметками имена идут в тот же тост, без них у тоста
// своя кнопка «Убрать имена» (снимает только имена, бросок остаётся). lead — прежний простой тост.
export function stepToast(h: ToastHost, lead: string, notes: string[], named: string[]) {
  const names = named.length > 0 ? `Имена взяты из названий файлов: ${named.length}` : ''
  if (notes.length > 0) return undoToast(h, [lead, ...notes, names].filter(Boolean).join('. '))
  if (!names) return lead ? h.notify(lead) : undefined
  const rev = h.view.rev
  h.notify([lead, names].filter(Boolean).join('. '), 'info', {
    label: 'Убрать имена',
    run: async () => {
      // Как у «Вернуть»: после чужой правки кнопка уже не про этот шаг.
      if (h.view.rev !== rev) return h.notify('Уже были другие изменения — имена не тронуты')
      await h.run(Api.ForgetFileNames(named))
    },
  })
  if (h.toast) h.toast.rev = rev
}

// Текст тоста кусками: пути из paths — отдельно (рисуются в <bdi>), остальное как есть. Пути ищутся слева направо, без наложений.
export function toastParts(text: string, paths: readonly string[] | undefined): { t: string; path: boolean }[] {
  const out: { t: string; path: boolean }[] = []
  let rest = text
  while (rest) {
    let at = -1
    let hit = ''
    for (const p of paths ?? []) {
      const i = p ? rest.indexOf(p) : -1
      if (i >= 0 && (at < 0 || i < at || (i === at && p.length > hit.length))) [at, hit] = [i, p]
    }
    if (at < 0) {
      out.push({ t: rest, path: false })
      break
    }
    if (at > 0) out.push({ t: rest.slice(0, at), path: false })
    out.push({ t: hit, path: true })
    rest = rest.slice(at + hit.length)
  }
  return out
}
