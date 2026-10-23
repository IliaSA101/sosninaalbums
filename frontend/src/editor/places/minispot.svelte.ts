// Где стоит мини-панель одиночного фото и есть ли место дуге наклона (063): пишет MiniPanel, читает CellOverlay.
import { NO_ROOM, type ArcRoom, type MiniWhere } from './minipanel'

class MiniSpot {
  photo = $state('') // для какого фото значения; '' — панели одиночного фото нет
  where = $state<MiniWhere>('below')
  lift = $state(0) // px от низа рамки до низа ползунка масштаба (0 — обычные 16 px)
  arc = $state.raw<ArcRoom>(NO_ROOM)

  set(photo: string, where: MiniWhere, lift: number, arc: ArcRoom) {
    this.photo = photo
    this.where = where
    this.lift = lift
    const a = this.arc
    if (arc.mode !== a.mode || arc.vx !== a.vx || arc.vy !== a.vy || arc.vw !== a.vw) this.arc = arc // тот же объект — читатели не гоняются
  }

  clear() {
    this.photo = ''
    this.where = 'below'
    this.lift = 0
    this.arc = NO_ROOM
  }
}

export const miniSpot = new MiniSpot()
