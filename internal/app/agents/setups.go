package agents

import (
	"bytes"
	"encoding/json"
	"path/filepath"
	"strings"
	"unicode"
	"unicode/utf8"
)

// Тексты настройки клиентов агентов (design §5): готовы к вставке, путь exe — из os.Executable.
// Секретов в них нет — программа открывает сокет в папке данных, а не порт.

// Идентификаторы клиентов в Setup.ID.
const (
	SetupClaudeDesktop = "claude-desktop"
	SetupClaudeCode    = "claude-code"
	SetupOpenCode      = "opencode"
	SetupOther         = "other"
)

// serverName — имя сервера в настройках клиентов.
const serverName = "sosnina-albums"

// Setup — один клиент в «Как подключить». Теги — те же, что у app.AgentSetup (TestDTOPairs).
type Setup struct {
	ID    string `json:"id"`
	Title string `json:"title"`
	Text  string `json:"text"` // готово к копированию
	Note  string `json:"note"` // где лежит файл настройки или как вставить команду
	File  bool   `json:"file"` // есть «Сохранить файл для Claude Desktop…» (только Windows и Mac)
}

// Setups — тексты настройки для программы exe на ОС goos ("windows", "darwin", "linux"). Пустой
// exe — пустой список (путь неизвестен или мёртв, см. ExeHint).
func Setups(exe, goos string) []Setup {
	if exe == "" {
		return []Setup{}
	}
	desktopNote := "Файл: ~/.config/Claude/claude_desktop_config.json (формат mcpServers)"
	switch goos {
	case "windows":
		desktopNote = `Файл: %APPDATA%\Claude\claude_desktop_config.json`
	case "darwin":
		desktopNote = "Файл: ~/Library/Application Support/Claude/claude_desktop_config.json"
	}
	codeNote := "Выполните в терминале"
	if goos == "windows" {
		codeNote = "Команда для PowerShell"
	}
	list := []Setup{
		{ID: SetupClaudeDesktop, Title: "Claude Desktop и Cowork", Text: desktopJSON(exe), Note: desktopNote,
			File: goos == "windows" || goos == "darwin"},
		{ID: SetupOpenCode, Title: "OpenCode", Text: openCodeJSON(exe), Note: "Файл: opencode.json (проекта) или ~/.config/opencode/opencode.json"},
	}
	q, why := quoteCmd(exe, goos)
	if why != "" {
		list[0].Note += ". Команды для терминала не показаны: " + why
		return list
	}
	otherNote := "Команда запуска сервера по stdio, без аргументов, кроме --mcp"
	otherText := q + " --mcp"
	if goos == "windows" {
		// Строка в кавычках в начале оператора PowerShell — выражение, и --mcp после неё ошибка
		// разбора; путь в кавычках вызывает оператор & (0.31-143-83).
		otherNote = "Команда для PowerShell: запуск сервера по stdio, без аргументов, кроме --mcp"
		otherText = "& " + otherText
	}
	// Порядок в окне: Claude Desktop, Claude Code, OpenCode, другой клиент.
	return []Setup{list[0],
		{ID: SetupClaudeCode, Title: "Claude Code", Text: "claude mcp add " + serverName + " -- " + q + " --mcp", Note: codeNote},
		list[1],
		{ID: SetupOther, Title: "Другой клиент (команда)", Text: otherText, Note: otherNote},
	}
}

// psQuotes — знаки, которые PowerShell считает одинарной кавычкой: ' и типографские U+2018…U+201B
// (автозамена Word и мессенджеров ставит их в имени папки). Любой из них внутри строки закрывает
// её, пока не удвоен.
const psQuotes = "'\u2018\u2019\u201A\u201B"

// quoteCmd — путь в одинарных кавычках: в двойных bash раскрывает $(…), обратные кавычки и $VAR, а
// PowerShell — $ и обратную кавычку. Windows (PowerShell): каждая кавычка из psQuotes внутри пути
// удваивается. Unix: апостроф внутри пути закрывает строку, ставится как \' и открывает строку
// заново (sh, bash, zsh и fish читают так одинаково); обратная косая черта в одинарных кавычках
// у fish — экранирование, у sh — обычный знак, поэтому для такого пути команду не
// даём, остаются JSON-настройки (0.31-143-81). Команды нет и для пути с управляющими или
// невидимыми знаками (Cc, Cf, не-UTF-8) на любой ОС: при вставке в терминал без bracketed paste
// Ctrl-U стирает открывающую кавычку, а перевод строки отправляет остаток на выполнение
// (0.31-143-82). why — причина отказа для пояснения, "" — команда дана.
func quoteCmd(exe, goos string) (cmd, why string) {
	if !utf8.ValidString(exe) || strings.ContainsFunc(exe, func(r rune) bool { return unicode.IsControl(r) || unicode.Is(unicode.Cf, r) }) {
		return "", "в пути есть управляющие или невидимые знаки, их нельзя безопасно вставить в терминал"
	}
	if goos == "windows" {
		var b strings.Builder
		b.WriteByte('\'')
		for _, r := range exe {
			b.WriteRune(r)
			if strings.ContainsRune(psQuotes, r) {
				b.WriteRune(r)
			}
		}
		b.WriteByte('\'')
		return b.String(), ""
	}
	if strings.Contains(exe, `\`) {
		return "", "в пути есть обратная косая черта, а оболочки читают её по-разному"
	}
	return "'" + strings.ReplaceAll(exe, "'", `'\''`) + "'", ""
}

// indentJSON — JSON с отступом без экранирования <, >, & (путь читает человек).
func indentJSON(v any) string {
	var b bytes.Buffer
	e := json.NewEncoder(&b)
	e.SetEscapeHTML(false)
	e.SetIndent("", "  ")
	if err := e.Encode(v); err != nil { // типы ниже простые: ошибки нет
		return ""
	}
	return strings.TrimRight(b.String(), "\n")
}

func desktopJSON(exe string) string {
	type srv struct {
		Command string   `json:"command"`
		Args    []string `json:"args"`
	}
	return indentJSON(map[string]any{"mcpServers": map[string]srv{serverName: {Command: exe, Args: []string{"--mcp"}}}})
}

func openCodeJSON(exe string) string {
	type srv struct {
		Type    string   `json:"type"`
		Command []string `json:"command"`
		Enabled bool     `json:"enabled"`
	}
	return indentJSON(map[string]any{"mcp": map[string]srv{serverName: {Type: "local", Command: []string{exe, "--mcp"}, Enabled: true}}})
}

// inWindowsTemp — путь exe лежит во временной папке Windows. Сравнивается и с длинной записью
// (в %TMP% бывает 8.3: C:\Users\IVANOV~1\…, а os.Executable отдаёт длинный путь), и по элементу
// Temp<N>_*.zip, в который Проводник распаковывает открытый zip, где бы ни лежала Temp.
func inWindowsTemp(exe, tmpDir string) bool {
	if hasZipTemp(exe) {
		return true
	}
	exes := []string{exe, longPathName(exe)}
	dirs := []string{tmpDir, longPathName(tmpDir)}
	for _, e := range exes {
		for _, d := range dirs {
			if e != "" && d != "" && strings.HasPrefix(strings.ToLower(e), strings.ToLower(filepath.Clean(d))+`\`) {
				return true
			}
		}
	}
	return false
}

// hasZipTemp — в пути есть папка вида Temp<цифры>_<имя>.zip (регистр не важен).
func hasZipTemp(path string) bool {
	for _, el := range strings.Split(strings.ToLower(path), `\`) {
		rest, ok := strings.CutPrefix(el, "temp")
		if !ok {
			continue
		}
		name, ok := strings.CutPrefix(strings.TrimLeft(rest, "0123456789"), "_")
		if ok && len(name) > len(".zip") && strings.HasSuffix(name, ".zip") {
			return true
		}
	}
	return false
}

// ExeHint — предупреждение про путь программы: "" — путь годится. Путь бывает временным: Mac —
// App Translocation (новый при каждом запуске, если .app не в «Программах»), Windows — exe из zip
// без распаковки (под папкой Temp), прочее — exe удалён или заменён (regular сообщает, что на
// этом месте обычный файл). Настройка с таким путём сломается при следующем запуске.
func ExeHint(exe, goos, tmpDir string, regular func(path string) bool) string {
	dead := exe == "" || !filepath.IsAbs(exe)
	if !dead {
		switch {
		case goos == "darwin" && strings.Contains(exe, "/AppTranslocation/"):
			dead = true
		case goos == "windows" && inWindowsTemp(exe, tmpDir):
			dead = true
		case !regular(exe):
			dead = true
		}
	}
	if !dead {
		return ""
	}
	return "Перенесите программу в постоянную папку (Mac — в «Программы», Windows — распакуйте zip) и откройте окно снова."
}

// XDGHint — на Linux папка настроек задана переменной XDG_CONFIG_HOME не там, где ищет посредник
// запасным путём (~/.config): клиент без этой переменной может не найти программу. Пусто — всё
// в порядке (или не Linux).
func XDGHint(goos, xdg, home string) string {
	if goos != "linux" || xdg == "" || home == "" || filepath.Clean(xdg) == filepath.Join(home, ".config") {
		return ""
	}
	return "Агенты могут не найти программу: папка настроек задана переменной XDG_CONFIG_HOME."
}
