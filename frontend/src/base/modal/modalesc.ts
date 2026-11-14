// Esc в окне (074): окно может забрать Esc себе («Esc — отмена записи сочетания»), тогда оно остаётся открытым.
// Без onescape или при ответе false Esc закрывает окно, как прежде.
export function escCloses(onescape?: () => boolean): boolean {
  return onescape?.() !== true
}
