package model

import (
	"strconv"
	"strings"
)

// Копии фото (098 design §2): копия — обычная запись Photos с ID «<ID файла>-<n>», n = 2…MaxCopy.
// Номер живёт в ID, ID файла — часть до последнего «-». Семья — записи с одним BaseID. Инварианты
// «одна запись — одно место» работают по записи: копия для них — другое фото. ID копии (18–19
// знаков с «-») не совпадает ни с ID места (12 hex), ни с ID файла (16 hex).

// MaxCopy — наибольший номер копии: номера 2…MaxCopy, 98 копий одного файла (decisions 098-15).
const MaxCopy = 99

// copyReqTail — хвост запроса копии от интерфейса: «<ID файла>-new» (decisions 098-3).
const copyReqTail = "-new"

// splitCopy — ID файла и номер копии; номер негоден (не число в записи без нулей впереди, < 2,
// > MaxCopy) или его нет — (id, 0).
func splitCopy(id string) (string, int) {
	i := strings.LastIndexByte(id, '-')
	if i <= 0 {
		return id, 0
	}
	tail := id[i+1:]
	if len(tail) == 0 || len(tail) > 2 || tail[0] == '0' {
		return id, 0
	}
	n, err := strconv.Atoi(tail)
	if err != nil || n < 2 || n > MaxCopy || strconv.Itoa(n) != tail {
		return id, 0
	}
	return id[:i], n
}

// BaseID — ID файла записи: «ab…cd-3» → «ab…cd»; без суффикса или с негодным суффиксом — id.
func BaseID(id string) string {
	b, _ := splitCopy(id)
	return b
}

// CopyNo — номер копии в ID; 0 — не копия.
func CopyNo(id string) int {
	_, n := splitCopy(id)
	return n
}

// CopyID — ID записи номер n файла base; n вне 2…MaxCopy — сам base.
func CopyID(base string, n int) string {
	if n < 2 || n > MaxCopy {
		return base
	}
	return base + "-" + strconv.Itoa(n)
}

// CopyReq — запрос копии файла base от интерфейса.
func CopyReq(base string) string { return base + copyReqTail }

// IsCopyReq — id — запрос копии (CopyReq) с непустым ID файла.
func IsCopyReq(id string) bool {
	return len(id) > len(copyReqTail) && strings.HasSuffix(id, copyReqTail)
}

// Family — ID записей семьи BaseID(id), которые есть в Photos, по возрастанию номера (исходная
// первой). Проход по номерам — не больше MaxCopy обращений к карте.
func (p *Project) Family(id string) []string {
	base := BaseID(id)
	var out []string
	for n := 1; n <= MaxCopy; n++ {
		fid := CopyID(base, n)
		if _, ok := p.Photos[fid]; ok {
			out = append(out, fid)
		}
	}
	return out
}

// PlacedFile — какая-нибудь запись семьи id стоит в месте листа (с обложкой): «фото уже в
// альбоме» по файлу, а не по записи (decisions 098-17); фон и стикер не в счёт.
func (p *Project) PlacedFile(id string) bool {
	base := BaseID(id)
	for _, s := range p.Sheets() {
		for _, pid := range s.Photos {
			if BaseID(pid) == base {
				return true
			}
		}
	}
	return false
}

// SetFile — свойства файла r (Path, Alt, W, H, Size, Lib) всем записям семьи id, которые есть в
// Photos; свойства использования не трогает (decisions 098-16).
func (p *Project) SetFile(id string, r PhotoRef) {
	for _, fid := range p.Family(id) {
		c := p.Photos[fid]
		c.Path, c.Alt, c.W, c.H, c.Size, c.Lib = r.Path, r.Alt, r.W, r.H, r.Size, r.Lib
		p.Photos[fid] = c
	}
}
