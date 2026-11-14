// Тост «диск снова ответил» (событие drive:back, 157): drive — буква без двоеточия, photos — сколько фото нашлось.
// Лежит в base/, потому что подписка на событие — в store (base не импортирует panels). «Фото» после числа не
// склоняется, поэтому форма одна.
export function driveBackText(drive: string, photos: number): string {
  return `Диск ${drive}: снова доступен — найдено ${photos} фото`
}
