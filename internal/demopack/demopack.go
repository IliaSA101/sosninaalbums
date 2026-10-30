// Package demopack — пример набора «Свадьба — синяя акварель» в программе (спека 142): только
// константы. Сам zip лежит рядом для тестов и упаковки выпуска, в exe не встраивается (0.30-142-33):
// стандартные установщики кладут его рядом с программой, компактная докачивает с релиза библиотеки.
package demopack

const (
	ID      = "wedding-navy-watercolor-demo"
	Version = 1
	File    = "wedding-navy-watercolor-demo-1.zip"
	// Size и SHA256 закреплены в коде: файл рядом с exe и ассет релиза принимаются только с ними
	// (страж TestDemoConstMatchesFile сверяет с настоящим zip).
	Size   = 17109459
	SHA256 = "c37f942ca69b278d02a4ab22af317d40c06e4fb160c4ccbd60d4c960b0910ae6"
	// Asset — путь от префикса источника библиотеки: стенд с SA_LIBRARY_URL качает со своего адреса (0.30-142-42).
	Asset   = "wedding-navy-watercolor-demo-v1/" + File
	Title   = "Свадьба — синяя акварель (пример)"
	About   = "Часть набора «Свадьба — синяя акварель» с рамками и макетом альбома. Пример уйдёт сам, когда встанет весь набор"
	License = "CC0-1.0"
	// Счётчики строки «Можно скачать» без каталога; сверяются с манифестом zip.
	Bg, Sticker, Spread, Frame = 4, 7, 4, 2
)

// MaxBudget — бюджет файла в git и выпуске (pre-commit пропускает zip до 20 МиБ).
const MaxBudget = 20 << 20

// Related — наборы каталога, которые вместе покрывают пример (доноры в testdata/demo-v1.keys).
var Related = []string{"wedding-navy-watercolor", "wedding-navy-watercolor-frames", "wedding-navy-watercolor-layouts"}
