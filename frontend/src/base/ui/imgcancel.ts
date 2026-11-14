// Для `{@attach cancelOnUnmount}` на <img> с миниатюрой: при снятии узла убирает src.
// Удаление <img> из DOM запрос в очереди браузера не отменяет (замер Chromium: 20 из 20 дошли до сервера,
// у каждого error), а снятый src отменяет ещё не отправленные — плитки, прокрученные мимо, не грузятся зря.
export function cancelOnUnmount(img: HTMLImageElement): () => void {
  return () => img.removeAttribute('src')
}
