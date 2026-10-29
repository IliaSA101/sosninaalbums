// Бегунок масштаба (спека 016, design §2): три состояния и переходы. Чистое ядро без таймеров и Svelte.
// closed — скрыт; peek — «подсказка» после смены масштаба не с бегунка (гаснет сама); open — открыт
// щелчком по бейджу или взят рукой (закрывают Esc, щелчок мимо, ещё один щелчок по бейджу).
export type ZoomUiState = 'closed' | 'peek' | 'open'
export type ZoomUiEvent = 'pulse' | 'expire' | 'grab' | 'toggle' | 'open' | 'close'

export const PEEK_MS = 1500 // сколько подсказка живёт после последней смены масштаба

export function zoomUiNext(s: ZoomUiState, e: ZoomUiEvent): ZoomUiState {
  switch (e) {
    case 'pulse':
      return s === 'open' ? s : 'peek'
    case 'expire':
      return s === 'peek' ? 'closed' : s
    case 'grab':
      return 'open' // взяли ручку или щёлкнули по дорожке
    case 'toggle':
      return s === 'open' ? 'closed' : 'open' // щелчок по бейджу: подсказка закрепляется
    case 'open':
      return 'open'
    case 'close':
      return 'closed'
  }
}
