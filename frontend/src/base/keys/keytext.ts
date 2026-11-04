// Клавиши команд в готовых текстах (074-16): статьи справочника и подсказки не пишут «Ctrl+Z» руками,
// а ставят метку, чтобы после переназначения текст не врал.
//   {key:undo}          — сочетание команды; клавиши нет — «без клавиши»
//   {key:undo| (%)}     — то же в шаблоне, % — сочетание; клавиши нет — шаблон опускается целиком
const MARK = /\{key:([A-Za-z0-9]+)(?:\|([^}]*))?\}/g

export function fillKeys(text: string, hint: (id: string) => string): string {
  return text.replace(MARK, (_m, id: string, tpl: string | undefined) => {
    const h = hint(id)
    if (tpl === undefined) return h || 'без клавиши'
    return h ? tpl.replace('%', h) : ''
  })
}

// Id команд, названных метками текста (страж: каждый есть в каталоге).
export function keyMarks(text: string): string[] {
  return [...text.matchAll(MARK)].map((m) => m[1])
}
