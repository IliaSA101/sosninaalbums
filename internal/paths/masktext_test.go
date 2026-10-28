package paths

import (
	"strings"
	"testing"
	"time"

	"github.com/IliaSA101/sosninaalbums/internal/testkit"
)

// Таблица design 092 §2.2: каждая строка с путём даёт «<путь>», остальные не меняются.
func TestMaskText(t *testing.T) {
	cases := []struct{ name, in, want string }{
		{"диск, пробелы в именах", `C:\Users\Иван\Фото свадьба\IMG 1.jpg`, `<путь>`},
		{"диск, нижний регистр", `c:\users\ivan\a.jpg`, `<путь>`},
		{"диск, прямые черты", `D:/Фото/a.jpg`, `<путь>`},
		{"UNC", `\\server\share\a.jpg`, `<путь>`},
		{"длинный путь", `\\?\C:\x\a.jpg`, `<путь>`},
		{"file", `file:///C:/x/a.jpg`, `<путь>`},
		{"file в тексте", `не открылся file:///C:/x/a.jpg"`, `не открылся <путь>"`},
		{"закодированный диск", `C%3A%5CUsers%5Cx`, `<путь>`},
		{"закодированный, прямые", `d%3a%2Ffoto%2Fa.jpg`, `<путь>`},
		{"Mac домашняя", `/Users/ivan/Pictures/a.jpg`, `<путь>`},
		{"Linux домашняя", `/home/u/a`, `<путь>`},
		{"том Mac", `/Volumes/Диск/x`, `<путь>`},
		{"private", `/private/var/folders/ab/x.jpg`, `<путь>`},
		{"var", `/var/folders/ab/x`, `<путь>`},
		{"mnt", `/mnt/c/Users/x`, `<путь>`},
		{"tmp", `/tmp/x.jpg`, `<путь>`},
		{"Unix после пробела", `ошибка /Users/ivan/a.jpg`, `ошибка <путь>`},
		{"Unix после скобки", `at f (/Users/ivan/a.js:1:2)`, `at f (<путь>)`},
		{"Unix после равно", `path=/home/u/a.jpg`, `path=<путь>`},
		{"путь в кавычках, хвост цел", `не открыть "C:\Фото\Иван Петров\a.jpg" — нет доступа`, `не открыть "<путь>" — нет доступа`},
		{"два пути в кавычках", `"C:\a b.jpg" и "D:\c d.jpg"`, `"<путь>" и "<путь>"`},
		{"два пути в скобках стека", "at f (C:\\x\\a.js:1:2)\nat g (D:/y/b.js:3:4)", "at f (<путь>)\nat g (<путь>)"},
		{"путь после двоеточия", `Error: C:\x\a.jpg`, `Error: <путь>`},
		{"адрес окна", `http://wails.localhost/assets/index-ab12.js:1:2345`, `http://wails.localhost/assets/index-ab12.js:1:2345`},
		{"адрес с home внутри", `http://wails.localhost/home/x`, `http://wails.localhost/home/x`},
		{"наш /media", `/media/t/3f9a0123456789ab?v=2`, `/media/t/3f9a0123456789ab?v=2`},
		{"ошибка JS", `TypeError: x is undefined`, `TypeError: x is undefined`},
		{"profile: не file:", `profile: ok`, `profile: ok`},
		// Аудит 092 G2: скобки и апостроф внутри имени не обрывают маску.
		{"круглые скобки в папке", `open D:\Клиенты (2026)\Иванова Анна\IMG_001.jpg`, `open <путь>`},
		{"квадратные скобки в кавычках", `"D:\Свадьба [Петровы]\Анна.jpg"`, `"<путь>"`},
		{"апостроф в имени", `D:\Фото\Мария's wedding\a.jpg`, `<путь>`},
		{"копия (2) в конце", `не открыть D:\Фото\Анна\IMG (2).jpg`, `не открыть <путь>`},
		{"копия (2), точка в конце входит в путь", `D:\Фото\IMG (2).jpg.`, `<путь>`},
		// Было «<путь>), потом»: «(2)» теперь сбалансированная группа внутри пути, скрыта целиком.
		{"скобки перед запятой входят в путь", `D:\Фото\Анна (2), потом`, `<путь>`},
		// Аудит 0.22-sec: скобки и апостроф посреди папки, после пробела.
		{"круглые скобки и пробел после них", `D:\Свадьбы\2026 (июнь) Ивановы Пётр и Анна\IMG_1.jpg`, `<путь>`},
		{"квадратные скобки и пробел после них", `не открыть D:\[VIP] Мария\IMG_1.jpg`, `не открыть <путь>`},
		{"апостроф и пробел после него", `D:\Фото\Анна' Пётр\IMG_1.jpg`, `<путь>`},
		{"апостроф и пробел, путь в апострофах", `'D:\Анна' Пётр\a.jpg' — нет`, `'<путь>' — нет`},
		{"апостроф-притяжательное в апострофах", `'D:\x\Мария's.jpg' и`, `'<путь>' и`},
		{"скобки в имени, путь в скобках стека", `at f (C:\Фото (2)\Анна\a.js:1:2)`, `at f (<путь>)`},
		{"одиночная открывающая скобка", `D:\Фото (Анна\a.jpg`, `<путь>`},
		{"одиночная квадратная открывающая", `D:\Фото [Анна\a.jpg`, `<путь>`},
		{"одиночная закрывающая обрывает путь", `x (D:\Анна\a.jpg) y`, `x (<путь>) y`},
		{"путь в квадратных скобках", `[D:\Анна\a.jpg] ok`, `[<путь>] ok`},
		{"вложенные скобки", `D:\Фото ((Анна))\a.jpg`, `<путь>`},
		{"UNC прямыми чертами", `не открыть //server/share/Иванова Анна/a.jpg`, `не открыть <путь>`},
		{"UNC прямыми чертами в начале", `//server/share/Анна.jpg`, `<путь>`},
		{"UNC прямыми чертами в апострофах", `'//server/Анна/a.jpg' и`, `'<путь>' и`},
		{"корень без диска", `ошибка \Users\Анна\a.jpg`, `ошибка <путь>`},
		{"корень без диска после равно", `path=\Users\Анна\a.jpg`, `path=<путь>`},
		{"корень без диска в кавычках", `"\Users\Анна Иванова\a.jpg" нет`, `"<путь>" нет`},
		{"корень без диска после [", `[\Users\Анна\a.jpg]`, `[<путь>]`},
		{"корень без диска в начале", `\Фото\Анна\a.jpg`, `<путь>`},
		{"Unix после [", `[/Users/anna/a.jpg]`, `[<путь>]`},
		{"комментарий // не путь", `// комментарий и a//b/c`, `// комментарий и a//b/c`},
		{"экранирование не путь", `строка a\nb и \t и \"x\"`, `строка a\nb и \t и \"x\"`},
		{"https с путём не задет", `https://example.com/Users/x`, `https://example.com/Users/x`},
		{"апостроф без пути", `'ok' и 'C:' и 'x\y'`, `'ok' и 'C:' и 'x\y'`},
		{"путь в круглых скобках, стек", `at f (C:\x\Анна\a.js:1:2)`, `at f (<путь>)`},
		{"путь в апострофах", `'C:\Анна Иванова\a.jpg' и 'D:\b.jpg'`, `'<путь>' и '<путь>'`},
		{"стек с пробелом после скобки", `at f (C:\x\a.js:1:2) at g`, `at f (<путь>) at g`},
		{"Unix после двоеточия", `err:/Users/anna/Pictures/x.jpg`, `err:<путь>`},
		{"тильда в начале", `~/Pictures/Анна/x.jpg`, `<путь>`},
		{"тильда после пробела", `ошибка ~/Pictures/Анна/x.jpg`, `ошибка <путь>`},
		{"тильда в слове не путь", `a~/b`, `a~/b`},
		{"скобки без пути", `вызов f(1) [2] it's (ok)`, `вызов f(1) [2] it's (ok)`},
		{"пусто", ``, ``},
		{"уже замаскировано", `<путь>`, `<путь>`},
	}
	for _, c := range cases {
		if got := MaskText(c.in); got != c.want {
			t.Errorf("%s: MaskText(%q) = %q, ждали %q", c.name, c.in, got, c.want)
		}
	}
}

// Причина ошибки Go после пути остаётся: «op путь: причина» → «op <путь>: причина» (qa-0.22).
func TestMaskTextKeepsReason(t *testing.T) {
	cases := []struct{ name, in, want string }{
		{"диск", `open C:\Users\Иван\Фото свадьба\x.jpg: no such file`, `open <путь>: no such file`},
		{"диск, прямые черты", `stat D:/Фото/a.jpg: access is denied`, `stat <путь>: access is denied`},
		{"Unix", `open /home/u/Фото/x.jpg: no such file or directory`, `open <путь>: no such file or directory`},
		{"скобки в имени", `open D:\Фото\Анна\IMG (2).jpg: no such file`, `open <путь>: no such file`},
		{"Unix, скобки в имени", `open /home/u/IMG (2).jpg: no such file`, `open <путь>: no such file`},
		{"UNC", `open \\server\share\Анна\a.jpg: no such file`, `open <путь>: no such file`},
		{"UNC прямыми", `open //server/share/Анна/a.jpg: permission denied`, `open <путь>: permission denied`},
		{"file", `open file:///C:/x/a.jpg: no such file`, `open <путь>: no such file`},
		{"тильда", `open ~/Pictures/a.jpg: no such file`, `open <путь>: no such file`},
		{"корень без диска", `open \Users\Анна\a.jpg: no such file`, `open <путь>: no such file`},
		{"две причины — первая тоже остаётся", `open /home/u/x.jpg: permission denied: повтор`, `open <путь>: permission denied: повтор`},
		{"два пути в одном тексте", "open C:\\a\\x.jpg: no such file\nopen /home/u/y.jpg: is a directory", "open <путь>: no such file\nopen <путь>: is a directory"},
		{"причина с путём — маскируется вся", `rename /home/u/a.jpg: to /home/u/b.jpg`, `rename <путь>`},
		{"двоеточие в имени папки, дальше путь", `open /home/u/Проект: Иванов/a.jpg: no such file`, `open <путь>: no such file`},
		{"двоеточие без пробела внутри пути", `open /home/u/a:b/x.jpg: no such file`, `open <путь>: no such file`},
		{"кириллица в имени папки с двоеточием", `open /home/u/Свадьба: Анна и Пётр: permission denied`, `open <путь>: permission denied`},
		{"Mac, кириллица в имени файла", `open /Users/u/Альбом: Петровы.salbum: no such file`, `open <путь>: no such file`},
		{"слеш в ASCII-причине", `read /home/x/y: input/output error`, `read <путь>: input/output error`},
		{"ASCII-имя с путём после двоеточия", `open /home/u/Proj: Ivanov/a.jpg: no such file`, `open <путь>: no such file`},
		{"пустая причина", `open /home/u/x.jpg: `, `open <путь>`},
		{"без причины", `open /home/u/x.jpg`, `open <путь>`},
		// Аудит 0.23-sec: первое звено причины — текст errno (два слова строчными), иначе это имя.
		{"имя с двоеточием: Smith", `open /home/u/Proj: Smith: permission denied`, `open <путь>: permission denied`},
		{"файл после двоеточия", `open /home/u/a: Smith.jpg`, `open <путь>`},
		{"слово после двоеточия", `open /home/u/Ivanov: wedding`, `open <путь>`},
		{"в кавычках причина вне", `open "/home/u/Фото/x.jpg": no such file`, `open "<путь>": no such file`},
	}
	for _, c := range cases {
		if got := MaskText(c.in); got != c.want {
			t.Errorf("%s: MaskText(%q) = %q, ждали %q", c.name, c.in, got, c.want)
		}
	}
}

// Тома и службы Linux: /run/media, /media/<имя>, /srv; наши /media/… (ID, не пути) целы.
func TestMaskTextLinuxMounts(t *testing.T) {
	cases := []struct{ name, in, want string }{
		{"run/media", `/run/media/ilya/ДИСК/Фото/a.jpg`, `<путь>`},
		{"media, том пользователя", `open /media/ilya/Фото Анны/a.jpg: no such file`, `open <путь>: no such file`},
		{"media, два знака", `/media/ab/x`, `<путь>`},
		{"media в начале и после скобки", `at f (/media/ilya/x.js:1:2)`, `at f (<путь>)`},
		{"srv", `open /srv/albums/Анна/a.jpg: permission denied`, `open <путь>: permission denied`},
		{"srv в кавычках", `"/srv/data/a b.jpg" нет`, `"<путь>" нет`},
		{"run/media, регистр", `/RUN/Media/u/x`, `<путь>`},
		{"наш /media/t", `/media/t/3f9a0123456789ab?v=2`, `/media/t/3f9a0123456789ab?v=2`},
		{"наш /media/p", `/media/p/ab/1?px=2`, `/media/p/ab/1?px=2`},
		{"наш /media/f", `/media/f/ab/1`, `/media/f/ab/1`},
		{"наш /media/font", `/media/font/Name`, `/media/font/Name`},
		{"наш /media/gen", `/media/gen/frame?x=1`, `/media/gen/frame?x=1`},
		{"наш /media/lib", `/media/lib/t/3f9a?v=1`, `/media/lib/t/3f9a?v=1`},
		{"наш /media/tpl", `/media/tpl/sa:tpl-1?px=2`, `/media/tpl/sa:tpl-1?px=2`},
		{"наш /media/cover-guide", `/media/cover-guide/1`, `/media/cover-guide/1`},
		{"наш /media/spread-guide", `/media/spread-guide/1`, `/media/spread-guide/1`},
		{"наш адрес в тексте", `GET /media/spread-guide/1 404`, `GET /media/spread-guide/1 404`},
		{"сегмент из одного знака", `/media/x/file`, `/media/x/file`},
		{"media без хвоста", `/media/ilya`, `/media/ilya`},
		{"адрес с media внутри", `http://wails.localhost/media/ilya/x`, `http://wails.localhost/media/ilya/x`},
		{"наш и том в одной строке", `/media/t/ab и /media/ilya/Диск/a.jpg`, `/media/t/ab и <путь>`},
		// Аудит 0.23-sec: сетевые и прочие корни Linux, любой абсолютный путь из двух частей.
		{"gvfs", `open /run/user/1000/gvfs/smb-share:server=nas,share=photos/Свадьба Ивановых/a.jpg: no such file`, `open <путь>: no such file`},
		{"data", `/data/Свадьба Ивановых/a.jpg`, `<путь>`},
		{"opt и root", "open /opt/albums/Анна/a.jpg\nи /root/Фото/b.jpg", "open <путь>\nи <путь>"},
		{"nfs, net, storage", "/nfs/x/a.jpg\n/net/host/b.jpg\n/storage/emulated/c.jpg", "<путь>\n<путь>\n<путь>"},
		{"произвольный корень в кавычках", `"/export/Иванова Анна/a.jpg" нет`, `"<путь>" нет`},
		{"произвольный корень в апострофах", `'/data/Анна Иванова/a.jpg' и 'D:\b.jpg'`, `'<путь>' и '<путь>'`},
		{"наш адрес в апострофах цел", `'/media/t/3f9a' и '/media/x/y'`, `'/media/t/3f9a' и '/media/x/y'`},
		{"том media в апострофах", `'/media/ilya/Диск/a.jpg' нет`, `'<путь>' нет`},
		{"одна часть не путь", `ответ /ok и /api`, `ответ /ok и /api`},
	}
	for _, c := range cases {
		if got := MaskText(c.in); got != c.want {
			t.Errorf("%s: MaskText(%q) = %q, ждали %q", c.name, c.in, got, c.want)
		}
	}
}

// Крайние входы: мегабайт без путей и мегабайт путей маскируются за разумное время (регулярные
// выражения RE2 без возвратов) и не оставляют имени.
func TestMaskTextBig(t *testing.T) {
	start := time.Now()
	plain := strings.Repeat("abc def ", 1<<17)
	if MaskText(plain) != plain {
		t.Error("текст без путей изменён")
	}
	got := MaskText(strings.Repeat(`"C:\Иван\a.jpg" `, 1<<16))
	if strings.Contains(got, "Иван") {
		t.Error("имя осталось")
	}
	if d := time.Since(start); d > testkit.Limit(5*time.Second) {
		t.Errorf("слишком долго: %v", d)
	}
}
