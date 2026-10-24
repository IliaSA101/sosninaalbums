//go:build !windows

package osx

import "testing"

// Вне Windows связь не поддерживается: пакетные функции — no-op и ничего не создают.
func TestAssocOther(t *testing.T) {
	if Supported() {
		t.Error("Supported() должен быть false")
	}
	if ReadAssoc() != (AssocState{}) {
		t.Error("ReadAssoc не пуст")
	}
	if err := WriteAssoc("/opt/x/sosninaalbums", "0.25.0"); err != nil {
		t.Errorf("WriteAssoc: %v", err)
	}
	if ReadAssoc() != (AssocState{}) {
		t.Error("WriteAssoc что-то записал")
	}
	if d, err := DropUserChoice(); d || err != nil {
		t.Errorf("DropUserChoice: %v %v", d, err)
	}
	if err := RemoveAssoc(); err != nil {
		t.Errorf("RemoveAssoc: %v", err)
	}
	AllowForeground() // не должно падать
}
