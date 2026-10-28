// Причина, по которой плитки «Можно скачать» серые (144): идёт скачивание (своё или из окна «Наборы») либо библиотека
// «только чтение». Пусто — плитки живые. Одно правило для «Фонов…» и «Макетов альбома».
import { READ_ONLY_HINT } from '../decor/decor'
import { BUSY_HINT } from '../../shell/packs/packs'

export const getOffHint = (net: { updating: boolean; progress: unknown }, readOnly: boolean): string =>
  net.updating || net.progress ? BUSY_HINT : readOnly ? READ_ONLY_HINT : ''
