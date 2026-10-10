package salbum

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"

	"github.com/IliaSA101/sosninaalbums/internal/album/model"
)

func writeAtomic(path string, data []byte) error {
	tmp := path + ".tmp"
	// Заранее лежащий .tmp (жёсткая ссылка, чужой файл) убираем, а не пишем сквозь него:
	// иначе запись пошла бы в чужой файл по ссылке.
	if fi, err := os.Lstat(tmp); err == nil && fi.IsDir() {
		return fmt.Errorf("на месте временного файла %s лежит папка", tmp)
	}
	if err := os.Remove(tmp); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	if err := writeSynced(tmp, data); err != nil {
		_ = os.Remove(tmp) // уже есть ошибка записи; остаток .tmp безвреден
		return err
	}
	if err := os.Rename(tmp, path); err != nil {
		_ = os.Remove(tmp) // уже есть ошибка записи; остаток .tmp безвреден
		return err
	}
	return nil
}

// writeSynced пишет файл и сбрасывает его на диск до переименования: иначе после
// сбоя питания вместо проекта может остаться пустой файл. O_EXCL — не открывать
// чужой файл, подложенный на место path.
func writeSynced(path string, data []byte) error {
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
	if err != nil {
		return err
	}
	if _, err := f.Write(data); err != nil {
		_ = f.Close() // уже есть ошибка записи
		return err
	}
	if err := f.Sync(); err != nil {
		_ = f.Close() // уже есть ошибка записи
		return err
	}
	return f.Close()
}

// backupPath — копия файла старой версии: <путь>.v<from>.bak (from — версия на чтении).
func backupPath(path string, from int) string { return fmt.Sprintf("%s.v%d.bak", path, from) }

// BackupOld копирует файл старой версии перед первой записью поверх него, чтобы
// можно было вернуться на прежнюю программу. Копия уже есть — не трогаем;
// исходника нет или он уже новой версии — копировать нечего.
func BackupOld(path string, from int) error {
	bak := backupPath(path, from)
	_, err := os.Lstat(bak)
	if err == nil {
		return nil
	}
	if !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	data, err := ReadLimited(path)
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	var head struct {
		Version int `json:"version"`
	}
	if json.Unmarshal(data, &head) == nil && head.Version >= model.FormatVersion {
		return nil
	}
	return writeAtomic(bak, data)
}
