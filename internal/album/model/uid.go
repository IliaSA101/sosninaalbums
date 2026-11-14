package model

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"path/filepath"

	"github.com/IliaSA101/sosninaalbums/internal/paths"
)

// Отпечаток проекта (146 Т4, решение 0.32-146-T4-9): случайный идентификатор в файле альбома.
// Запись о ссылке (shares.json) помнит его: другой проект на том же пути (заменили файл,
// восстановили чужую копию) несёт другой отпечаток, и его страницы по прежней ссылке не уходят.
// Это не данные вёрстки: потеря поля (старая программа его выбросит) даёт только вопрос о новой
// ссылке, поэтому номер формата не меняется (salbum-format.md «Отпечаток»).

// uidBytes — длина отпечатка: 128 бит.
const uidBytes = 16

// NewUID — новый отпечаток: 32 знака hex из crypto/rand.
func NewUID() string {
	var b [uidBytes]byte
	_, _ = rand.Read(b[:]) // crypto/rand с Go 1.24 не возвращает ошибку
	return hex.EncodeToString(b[:])
}

// GoodUID — строка годится в отпечаток: ровно 32 знака [0-9a-f]. Отпечаток из файла — чужие
// данные: негодный считается отсутствующим.
func GoodUID(s string) bool { return len(s) == 2*uidBytes && lowerHex(s) }

// lowerHex — все знаки s из [0-9a-f].
func lowerHex(s string) bool {
	for i := 0; i < len(s); i++ {
		if c := s[i]; (c < '0' || c > '9') && (c < 'a' || c > 'f') {
			return false
		}
	}
	return true
}

// Печать отпечатка (дополнения T4-9 после аудита): запись о ссылке пути действует только для
// отпечатка, выданного по этому пути. Печать ставится при выдаче отпечатка (album/editor_uid.go),
// обычная запись переносит её как есть. Копия файла в Проводнике (шаблон, скопированный клиенту)
// несёт печать пути исходника: при выгрузке и показе ссылки запись пути для неё чужая, выгрузка
// выдаёт новый отпечаток. Сам путь в файл не пишется — только хэш.

// uidTagBytes — длина печати: 64 бита, 16 знаков hex.
const uidTagBytes = 8

// UIDTag — печать отпечатка uid к файлу path: хэш отпечатка и пути, свёрнутого как ключ записи о
// ссылке (Clean, paths.Fold). Отпечаток в хэше — соль: печати двух альбомов одной папки разные.
func UIDTag(uid, path string) string {
	if !GoodUID(uid) || path == "" {
		return ""
	}
	sum := sha256.Sum256([]byte(uid + "\x00" + paths.Fold(filepath.Clean(path))))
	return hex.EncodeToString(sum[:uidTagBytes])
}

// GoodUIDTag — строка годится в печать: ровно 16 знаков [0-9a-f]. Печать из файла — чужие данные.
func GoodUIDTag(s string) bool { return len(s) == 2*uidTagBytes && lowerHex(s) }

// UIDAt — печать отпечатка из файла (Load); "" — не было или негодная. В JSON снимков не идёт.
func (p *Project) UIDAt() string { return p.uidAt }

// SetUIDAt — печать для файла: Load кладёт прочитанную, Editor — ту, что пойдёт в запись.
func (p *Project) SetUIDAt(s string) { p.uidAt = s }

// Sealed — печать p — пути path (отпечаток выдан по нему): это отпечаток альбома, а не копии
// чужого файла. Нет печати (файл ранней сборки 0.32), путь в другом написании (8.3,
// символическая ссылка) — false: лишний вопрос о новой ссылке вместо страниц в чужую ссылку.
func (p *Project) Sealed(path string) bool {
	return p.uidAt != "" && p.uidAt == UIDTag(p.UID, path)
}
