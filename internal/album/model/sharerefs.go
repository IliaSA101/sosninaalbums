package model

import (
	"crypto/sha256"
	"encoding/hex"
	"slices"
)

// Привязки проекта к выгруженным альбомам (167, design §2): в файле лежит не адрес и не id
// (id — ключ доступа к листалке, файл уходит коллегам и в шаблоны), а ref — хэш от uid проекта и
// id альбома. Найти по нему альбом может только владелец через свой список на сервисе; uid в
// хэше — соль. Привязки — не вёрстка: правкой не являются, в JSON снимков не идут (Clone их
// теряет — снимки несут их отдельно), а потеря поля старой программой даёт только поиск заново.

// MaxShareRefs — сколько привязок хранит проект (новая — первой, лишние отбрасываются).
const MaxShareRefs = 4

// RawShareRefs — предел массива shareRefs в чужом файле: длиннее — поле отброшено целиком и
// дальше не разбирается (ни времени, ни памяти на мусор).
const RawShareRefs = 64

// linkRefBytes — длина ref: 128 бит, 32 знака hex.
const linkRefBytes = 16

// LinkRef — привязка проекта uid к альбому id: hex(SHA-256("sa-link" NUL uid NUL id)[:16]). Без
// адреса сервиса (167-14): смена домена поиск не ломает. "" — uid или id негодны.
func LinkRef(uid, id string) string {
	if !GoodUID(uid) || id == "" {
		return ""
	}
	sum := sha256.Sum256([]byte("sa-link\x00" + uid + "\x00" + id))
	return hex.EncodeToString(sum[:linkRefBytes])
}

// GoodLinkRef — строка годится в привязку: ровно 32 знака [0-9a-f]. Привязка из файла — чужие
// данные.
func GoodLinkRef(s string) bool { return len(s) == 2*linkRefBytes && lowerHex(s) }

// AddLinkRef — список refs с ref первым: дубль переезжает вперёд, лишнее сверх MaxShareRefs
// отбрасывается; негодный ref список не меняет. Возвращает новый срез, refs не правится.
func AddLinkRef(refs []string, ref string) []string {
	if !GoodLinkRef(ref) {
		return refs
	}
	out := make([]string, 0, min(len(refs)+1, MaxShareRefs))
	out = append(out, ref)
	for _, r := range refs {
		if len(out) == MaxShareRefs {
			break
		}
		if r != ref {
			out = append(out, r)
		}
	}
	return out
}

// CleanLinkRefs — привязки refs после проверки: негодные и повторы выброшены, не больше
// MaxShareRefs, порядок прежний. Массив длиннее RawShareRefs отбрасывается целиком. Всегда
// новый срез; нет годных — nil.
func CleanLinkRefs(refs []string) []string {
	if len(refs) > RawShareRefs {
		return nil
	}
	var out []string
	for _, r := range refs {
		if len(out) == MaxShareRefs {
			break
		}
		if GoodLinkRef(r) && !slices.Contains(out, r) {
			out = append(out, r)
		}
	}
	return out
}

// ShareRefs — привязки проекта, прочитанные из файла или положенные Editor перед записью (копия).
// Поле вне JSON: снимки отмены и холостую правку не трогает.
func (p *Project) ShareRefs() []string { return slices.Clone(p.shareRefs) }

// SetShareRefs — привязки для записи (Load кладёт прочитанные, Editor — свои); негодное
// отбрасывается (CleanLinkRefs).
func (p *Project) SetShareRefs(refs []string) { p.shareRefs = CleanLinkRefs(refs) }
