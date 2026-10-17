// Где стоит мини-панель одиночного фото и есть ли место дуге наклона (063): пишет MiniPanel, читает CellOverlay.
import { NO_ROOM, type ArcRoom, type MiniWhere } from './minipanel'

class MiniSpot {
  photo = $state('') // для какого фото значения; '' — панели одиночного фото нет
  where = $state<MiniWhere>('below')
  lift = $state(0) // px от низа рамки до низа ползунка масштаба (0 — обычные 16 px)
  arc = $state.raw<ArcRoom>(NO_ROOM)
}

export const miniSpot = new MiniSpot()
