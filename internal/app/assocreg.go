package app

import "github.com/IliaSA101/sosninaalbums/internal/osx"

// assocReg — всё, что связь .salbum (112) берёт у системы: реестр и путь запущенной программы.
// Интерфейс, чтобы тесты app не трогали настоящий реестр: настоящий ставит Run
// (useSystemAssoc), NewApp оставляет nil — связь «не поддерживается».
type assocReg interface {
	Supported() bool
	Read() osx.AssocState
	Write(exe, version string) error
	Remove() error
	DropUserChoice() (bool, error)
	SelfExe() (string, error)
}

// osxAssoc — настоящая связь: функции osx (HKCU на Windows; на других ОС — пустые).
type osxAssoc struct{}

func (osxAssoc) Supported() bool                 { return osx.Supported() }
func (osxAssoc) Read() osx.AssocState            { return osx.ReadAssoc() }
func (osxAssoc) Write(exe, version string) error { return osx.WriteAssoc(exe, version) }
func (osxAssoc) Remove() error                   { return osx.RemoveAssoc() }
func (osxAssoc) DropUserChoice() (bool, error)   { return osx.DropUserChoice() }
func (osxAssoc) SelfExe() (string, error)        { return osx.SelfExe() }

// useSystemAssoc подключает настоящий реестр; зовёт Run до окна, тесты — никогда.
func (a *App) useSystemAssoc() { a.assoc.reg = osxAssoc{} }
