package model

import (
	"errors"
	"fmt"
	"strings"

	"github.com/IliaSA101/sosninaalbums/internal/library"
)

// Постановка копий (098 design §2.2): интерфейс просит копию строкой «<ID файла>-new» в прежних
// методах; тело правки первым шагом зовёт ResolveCopies и дальше видит обычную незанятую запись.
// Номер копии выдаётся внутри той же правки — гонки нет; отказ правки — откат и записи копии.

var (
	// ErrCopyLib — копия своего из библиотеки или записи без пути: таких копий не бывает (098-15).
	ErrCopyLib = errors.New("у своего элемента библиотеки копий не бывает")
	// ErrCopies — все номера копий файла заняты (098-15).
	ErrCopies = fmt.Errorf("слишком много копий одного фото (%d)", MaxCopy-1)
)

// ReqBase — ID файла из запроса копии (CopyReq); не запрос — id как есть.
func ReqBase(id string) string {
	if !IsCopyReq(id) {
		return id
	}
	return strings.TrimSuffix(id, copyReqTail)
}

// FileOf — ID файла записи или запроса копии: BaseID, у запроса — ReqBase.
func FileOf(id string) string { return BaseID(ReqBase(id)) }

// AddCopy — новая запись «<IDFor(ref.Path)>-<n>» (наименьший свободный n) со свойствами ref как
// есть; никуда не ставит. ID файла — по пути, а не по ID источника: годится и для записи из
// снимка (099), и для переноса между альбомами. Свойства файла — как у семьи, если она есть
// (снимок мог устареть после перепривязки). Своё из библиотеки или пустой путь — ErrCopyLib,
// номеров нет — ErrCopies.
func AddCopy(p *Project, ref PhotoRef) (string, error) {
	if ref.Lib != "" || ref.Path == "" {
		return "", ErrCopyLib
	}
	base := library.IDFor(ref.Path)
	if fam := p.Family(base); len(fam) > 0 {
		f := p.Photos[fam[0]]
		ref.Path, ref.Alt, ref.W, ref.H, ref.Size, ref.Lib = f.Path, f.Alt, f.W, f.H, f.Size, f.Lib
	}
	for n := 2; n <= MaxCopy; n++ {
		id := CopyID(base, n)
		if _, taken := p.Photos[id]; !taken {
			p.Photos[id] = ref
			return id, nil
		}
	}
	return "", ErrCopies
}

// FitFile — свойства файла записи id (Path, Alt, W, H, Size, Lib) — как у другой записи её семьи,
// если такая есть: запись из снимка буфера могла устареть (другая запись семьи получила новые
// W/H или перепривязку), а FamilyInvariant требует общих свойств файла. Нет записи id, своё из
// библиотеки или семья из одной записи — ничего.
func (p *Project) FitFile(id string) {
	r, ok := p.Photos[id]
	if !ok || r.Lib != "" {
		return
	}
	for _, fid := range p.Family(id) {
		if fid == id {
			continue
		}
		f := p.Photos[fid]
		r.Path, r.Alt, r.W, r.H, r.Size, r.Lib = f.Path, f.Alt, f.W, f.H, f.Size, f.Lib
		p.Photos[id] = r
		return
	}
}

// ResolveCopies — запросы копий в infos → настоящие ID; остальное как есть (новый срез). Запрос
// файла, чья исходная запись нигде не стоит и не взята этим же броском, — сама исходная запись
// (ещё одна запись семьи); иначе — новая копия: свойства файла, ч/б и тон, зеркало и имя — от
// исходной записи (нет её — от записи семьи с наименьшим номером), кадр, подпись, прозрачность
// и свой кадр — свои, пустые (decisions 098-2). Своё из библиотеки — исходная запись (перенос).
func ResolveCopies(p *Project, infos []PhotoInfo) ([]PhotoInfo, error) {
	out := make([]PhotoInfo, len(infos))
	taken := make(map[string]bool, len(infos)) // записи, уже взятые этим броском
	for _, ph := range infos {
		if !IsCopyReq(ph.ID) {
			taken[ph.ID] = true
		}
	}
	for i, ph := range infos {
		out[i] = ph
		if !IsCopyReq(ph.ID) {
			continue
		}
		base := ReqBase(ph.ID)
		fam := p.Family(base)
		lib := ph.Lib != "" || len(fam) > 0 && p.Photos[fam[0]].Lib != ""
		if lib || !taken[base] && !p.Placed(base) {
			out[i].ID, taken[base] = base, true
			continue
		}
		src, ok := p.Photos[base]
		if !ok && len(fam) > 0 {
			src = p.Photos[fam[0]]
		} else if !ok {
			src = PhotoRef{Path: ph.Path, W: ph.W, H: ph.H, Size: ph.Size}
		}
		id, err := AddCopy(p, inherit(src))
		if err != nil {
			return nil, err
		}
		out[i].ID, taken[id] = id, true
	}
	return out, nil
}

// inherit — что копия берёт у записи (decisions 098-2): свойства файла, ч/б и тон, зеркало, имя
// (глубокие копии); кадр, подпись, прозрачность и свой кадр — нет.
func inherit(r PhotoRef) PhotoRef {
	c := r.Copy()
	return PhotoRef{Path: c.Path, Alt: c.Alt, W: c.W, H: c.H, Size: c.Size, Lib: c.Lib,
		FlipH: c.FlipH, FlipV: c.FlipV, Adj: c.Adj, Person: c.Person}
}

// PlacedFiles — ID файлов (BaseID), какая-нибудь запись которых стоит в месте листа (с обложкой):
// набор для PlacedFile по многим фото сразу — один проход по листам.
func (p *Project) PlacedFiles() map[string]bool {
	out := map[string]bool{}
	for _, s := range p.Sheets() {
		for _, id := range s.Photos {
			out[BaseID(id)] = true
		}
	}
	return out
}
