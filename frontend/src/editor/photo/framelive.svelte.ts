// Предпросмотр ползунков кадра (карточка справа, InspectorFrame; ползунок на ячейке): пока ползунок
// тянут, Cell этого фото рисует кадр отсюда; SetFrame / SetFrameAngle уходит в Go при отпускании.
// tilt — тянут наклон, pan — двигают кадр за ✥: на ячейке показывается сетка третей (по ней выравнивают горизонт).
// ✥ живёт в CellOverlay вне ячейки, поэтому сдвиг кадра тоже идёт через это состояние.
// angle — живой угол дуги наклона (059, 063): строка «Наклон» карточки показывает его на ходу, пока тянут или жмут стрелки.
import type { Frame } from '../../base/core/types'

class FrameLive {
  cur = $state<{ id: string; frame: Frame; tilt: boolean; pan: boolean; angle: number | null } | null>(null)

  set(id: string, frame: Frame, tilt = false, pan = false, angle: number | null = null) {
    this.cur = { id, frame, tilt, pan, angle }
  }

  clear() {
    this.cur = null
  }

  of(id: string): Frame | null {
    return this.cur?.id === id ? this.cur.frame : null
  }

  // Живой угол дуги этого фото (°) или null — тогда карточка берёт значение из View.
  angleOf(id: string): number | null {
    return this.cur?.id === id ? this.cur.angle : null
  }

  // Наклон этого фото тянут ползунком: нужна сетка.
  tilting(id: string): boolean {
    return this.cur?.id === id && this.cur.tilt
  }

  // Кадр этого фото двигают за ✥ (кнопка зажата).
  panning(id: string): boolean {
    return this.cur?.id === id && this.cur.pan
  }
}

export const framelive = new FrameLive()
