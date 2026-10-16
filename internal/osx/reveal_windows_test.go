package osx

import "testing"

// Проводник получает командную строку целиком: запятая в пути не должна ломать /select.
// Поле CmdLine есть только у SysProcAttr под Windows — поэтому тест в _windows-файле.
func TestExplorerSelectAttr(t *testing.T) {
	attr := explorerSelectAttr(`C:\a, b\c.jpg`)
	if want := `explorer /select,"C:\a, b\c.jpg"`; attr.CmdLine != want {
		t.Fatalf("CmdLine %q, ждали %q", attr.CmdLine, want)
	}
}

// «Открыть папку»: путь в кавычках, запятая без пробела («Иванов,Петрова») не режет его.
func TestExplorerOpenAttr(t *testing.T) {
	for _, dir := range []string{`D:\Альбомы\Иванов,Петрова`, `C:\a b`, `D:\`} {
		if attr, want := explorerOpenAttr(dir), `explorer "`+dir+`"`; attr.CmdLine != want {
			t.Errorf("CmdLine %q, ждали %q", attr.CmdLine, want)
		}
	}
}
