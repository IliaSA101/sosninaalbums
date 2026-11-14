package assoc

import (
	"strings"

	"github.com/IliaSA101/sosninaalbums/internal/osx"
	"github.com/IliaSA101/sosninaalbums/internal/paths"
)

// План связи: чистые решения без реестра и файлов (Exists приходит снаружи).

// planIn — вход plan: что в реестре и кто этот запуск.
type planIn struct {
	St     osx.AssocState
	Exe    string // exe этого запуска (Registry.SelfExe)
	Ver    string // версия этого запуска
	Exists func(path string) bool
}

// planAct — решение plan.
type planAct struct {
	Write   bool // записать связь на этот exe
	Foreign bool // расширение у чужой программы: не трогаем (в лог)
	Drop    bool // удалить наш прежний выбор «Открыть с помощью → всегда»
}

// plan — чистое решение для связи, уже ставшей нашей (согласие дано или связь наша).
//
//	ProgID пуст или наш: пусто или exe из команды пропал → писать; моя версия новее → писать;
//	  равны: я установлена, они нет → писать; иначе не писать.
//	ProgID чужой → не писать.
//	UserChoice наш (userChoiceOurs) → удалить.
func plan(in planIn) planAct {
	st := in.St
	act := planAct{Drop: userChoiceOurs(st, in.Exists)}
	if st.ProgID != "" && !st.ProgIDIsOurs() {
		act.Foreign = true
		return act
	}
	theirs := theirVersion(st)
	switch {
	case st.ProgID == "" || st.Exe == "" || !in.Exists(st.Exe):
		act.Write = true
	case verKnown(in.Ver) && !verKnown(theirs):
		// update.Newer для неразборной версии даёт false, а здесь неизвестная версия ниже любой:
		// связь без внятной версии (запись чужим инструментом, «dev») забирает любая настоящая.
		act.Write = true
	case cmpVer(in.Ver, theirs) > 0:
		act.Write = true
	case cmpVer(in.Ver, theirs) == 0 && isInstalled(in.Exe, st) && !isInstalled(st.Exe, st):
		act.Write = true
	}
	return act
}

// userChoiceOurs — прежний выбор пользователя ведёт в нашу программу и мешает связи HKCU
// (Windows слушает UserChoice раньше). Applications\sosninaalbums*.exe — всегда наш (случай
// «Открыть с помощью → всегда» на старой версии). Наш ProgID Windows разбирает по команде
// связи, поэтому он мешает, только когда команда мертва: exe пропал. Чужой выбор — не наш.
func userChoiceOurs(st osx.AssocState, exists func(string) bool) bool {
	if st.UserChoice == "" || !st.UserChoiceIsOurs() {
		return false
	}
	if strings.EqualFold(st.UserChoice, osx.AssocProgID) {
		return st.Exe == "" || !exists(st.Exe)
	}
	return true
}

// isInstalled — exe — установленная копия (InstallDir\SosninaAlbums.exe, раздел пишет установщик).
func isInstalled(exe string, st osx.AssocState) bool {
	inst := st.InstalledExe()
	return inst != "" && exe != "" && paths.SamePath(exe, inst)
}

// theirVersion — версия записавшего связь: AppVersion; нет её, а exe установленный — версия
// установки; иначе "" (неизвестна).
func theirVersion(st osx.AssocState) string {
	if st.AppVersion != "" {
		return st.AppVersion
	}
	if isInstalled(st.Exe, st) {
		return st.InstallVersion
	}
	return ""
}
