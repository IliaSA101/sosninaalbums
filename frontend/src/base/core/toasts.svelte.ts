import * as Api from '../../../wailsjs/go/app/App'
import { capFirst } from '../words/errtext'
import { keymap } from '../keys/keymap.svelte'

// rev — ревизия альбома, на которую рассчитан тост с «Вернуть»: другая ревизия (Ctrl+Z, любая правка) гасит тост.
export type Toast = { text: string; kind: 'info' | 'error'; action?: { label: string; run: () => void }; rev?: number }

// Что тостам «Вернуть» нужно от Store (структурный тип: цикла импортов со store нет).
export interface ToastHost {
  readonly view: { rev: number }
  readonly delQueue: Promise<unknown>
  notify(text: string, kind?: Toast['kind'], action?: Toast['action'], ms?: number): void
  run(p: Promise<unknown>): Promise<boolean>
  goId(id: string): void
  toast: Toast | null
}

// Одно сообщение внизу окна и его таймер.
export class Toasts {
  toast = $state<Toast | null>(null)
  private timer = 0

  // ms — сколько держать тост; без него 8 с с кнопкой и 4 с без (итог подгона по лицам читают дольше — 20 с).
  notify(text: string, kind: Toast['kind'] = 'info', action?: Toast['action'], ms?: number) {
    // Ошибки Go пишутся со строчной буквы.
    this.toast = { text: kind === 'error' ? capFirst(text) : text, kind, action }
    clearTimeout(this.timer)
    this.timer = window.setTimeout(() => (this.toast = null), ms ?? (action ? 8000 : 4000))
  }
}

// Тост с кнопкой «Вернуть» = Undo, как у разреза: безопасно, только если после правки ничего не менялось.
// back — разворот, на который перейти после возврата.
export function undoToast(h: ToastHost, text: string, back?: string) {
  const rev = h.view.rev
  h.notify(text, 'info', {
    label: 'Вернуть',
    run: async () => {
      // Идущее следом удаление ещё может менять альбом: сначала дожидаемся очереди, потом сверяем ревизию.
      await h.delQueue
      if (h.view.rev !== rev) return h.notify(keymap.tip('Уже были другие изменения — используйте «Отменить»', 'undo'))
      if ((await h.run(Api.Undo())) && back) h.goId(back)
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
