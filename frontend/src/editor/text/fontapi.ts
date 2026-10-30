// Методы Go для своих шрифтов (082, design §8) одним модулем: тесты подменяют его целиком.
// ImportFonts — диалог (закрыли — пустой итог), ImportFontPaths — то же без диалога (QA из браузера),
// DropFonts — откат начертаний, не открытых окном (Go удаляет только свежие этого сеанса), HideFont — ×.
export { DropFonts, HideFont, ImportFontPaths, ImportFonts } from '../../../wailsjs/go/app/App'
