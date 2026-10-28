// Вызовы Go обновления (111, design §3): настоящие биндинги, тип ответа — app.UpdateInfo.
// phase: idle | checking | latest | available | downloading | ready | error;
// mode: install (установленная Windows) | setup (переносная Windows) | page (Mac, Linux, «нужна новая программа»).
import * as UpdateGo from '../../../wailsjs/go/app/App'
import type { app } from '../../../wailsjs/go/models'

export { UpdateGo }
export type UpdateInfo = app.UpdateInfo

// update:progress — в биндинги не попадает (событие, не тип метода).
export interface UpdateProgress {
  done: number
  total: number
}

// Состояние до первого ответа Go: ничего не предлагаем.
export const NO_UPDATE: UpdateInfo = {
  phase: 'idle',
  mode: 'page',
  version: '',
  notes: [],
  done: 0,
  total: 0,
  error: '',
  atStart: true,
  checked: 0,
  just: '',
  failed: '',
}
