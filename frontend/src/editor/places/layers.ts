// Слои разворота (спека 010 U6): Go считает общий стек мест, фото и текстов (`z` от 1), а интерфейс кладёт
// все слои в один контекст наложения `.content`. Первые 10 индексов оставлены служебным слоям внутри него.
export const LAYER0 = 10

// z-index слоя по `z` из Go; нет `z` (старый ответ, пустой кадр) — самый низ контента.
export const layerIndex = (z?: number): number => LAYER0 + (z ?? 0)

// ID для ArrangeLayers: места/фото, тексты группы и одиночный выделенный текст — без повторов, по порядку.
export function layerTargets(places: readonly string[], texts: readonly string[], text: string | null): string[] {
  return [...new Set([...places, ...texts, ...(text ? [text] : [])])]
}
