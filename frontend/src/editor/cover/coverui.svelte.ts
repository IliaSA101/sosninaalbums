// Состояние обложки только для экрана (спека 032): прозрачность разметки типографии не сохраняется ни в
// проект, ни в localStorage — разметка нужна на время вёрстки.
class CoverUi {
  guideOpacity = $state(0.5)
  // Номер правки, при которой выбрали файл разметки: сбивает кэш картинки (адрес /media/cover-guide один).
  guideRev = $state(0)

  setOpacity(v: number) {
    this.guideOpacity = Number.isFinite(v) ? Math.min(1, Math.max(0.05, v)) : 0.5
  }
}

export const coverUi = new CoverUi()
