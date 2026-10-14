package prefs

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"slices"
	"time"

	"github.com/IliaSA101/sosninaalbums/internal/album/model"
	"github.com/IliaSA101/sosninaalbums/internal/library"
	"github.com/IliaSA101/sosninaalbums/internal/paths"
)

// Templates — шаблоны сеток в памяти и состояние templates.json. Без своего замка:
// вызывающий держит его (App.tplMu). Списки не меняются на месте: правка заменяет
// срез целиком, поэтому копию среза можно отдавать наружу без замка.
type Templates struct {
	list   []model.Template
	bad    bool // файл есть, но не разобран: перед первой записью уходит в .bad-<время>
	cut    bool // файл прочитан не целиком (зажим отбросил шаблоны): перед первой записью — тоже в .bad
	unread bool // файл есть, но не открылся (замок антивируса): перед записью перечитать
	newer  bool // файл новее программы: читаем, но не пишем
}

// TemplatesPath — templates.json в папке данных dir.
func TemplatesPath(dir string) string { return filepath.Join(dir, "templates.json") }

// openTplFile — os.Open; подменяется в тестах (замок на файле иначе не изобразить).
var openTplFile = os.Open

// LoadTemplates читает templates.json при старте. Большой или битый файл — пустой
// список и отметка bad (его не затрут, а уберут в .bad); прочитан не целиком — отметка
// cut (то же); версия новее — только чтение; не открылся или не прочитался — пустой
// список, файл перечитает Store.
func LoadTemplates(dir string) Templates {
	var t Templates
	if err := t.read(dir); err != nil {
		t.unread = true
		log.Printf("templates.json не прочитан (%v): перечитаем перед первой записью", paths.NoPath(err))
	}
	return t
}

// read читает файл в t. Ошибка — только если файл есть, но не открылся или не
// прочитался: такой файл может быть исправным, в .bad его не уводим.
func (t *Templates) read(dir string) error {
	f, err := openTplFile(TemplatesPath(dir))
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return err
	}
	defer func() { _ = f.Close() }()
	data, err := io.ReadAll(io.LimitReader(f, model.MaxTemplatesFile+1))
	if err != nil {
		return err
	}
	list, ver, err := model.ParseTemplates(data)
	if err != nil {
		t.bad = true
		log.Printf("templates.json не разобран (%v): будет сохранён как .bad перед первой записью", err)
		return nil
	}
	t.list = list
	if n := inFile(data); len(list) < n {
		t.cut = true
		log.Printf("templates.json: прочитано шаблонов %d из %d, файл будет сохранён как .bad перед первой записью", len(list), n)
	}
	if ver > model.TemplatesVersion {
		t.newer = true
		log.Printf("templates.json версии %d новее программы: шаблоны только для чтения", ver)
	}
	return nil
}

// List — текущий список; менять на месте нельзя.
func (t *Templates) List() []model.Template { return t.list }

// Newer — файл записан более новой версией: шаблоны только для чтения.
func (t *Templates) Newer() bool { return t.newer }

// Store пишет новый список в templates.json папки dir и только потом принимает его в
// память. Файл новее программы не трогается; не разобранный сначала уходит в
// templates.bad-<время>.json (через rename) — без этого запись не делается; так же
// прочитанный не целиком (cut): иначе отброшенные зажимом шаблоны пропали бы молча. Данные
// больше MaxTemplatesFile не пишутся (model.ErrTemplatesFull): файл и память прежние.
func (t *Templates) Store(dir string, list []model.Template, rename func(from, to string) error) error {
	if t.unread {
		var err error
		if list, err = t.reread(dir, list); err != nil {
			return err
		}
	}
	if t.newer {
		return model.ErrTemplatesNewer
	}
	// В памяти — то, что ляжет в файл: после перезапуска шаблоны те же до бита.
	list = model.RoundTemplates(list)
	data, err := model.MarshalTemplates(list)
	if err != nil {
		return err
	}
	if len(data) > model.MaxTemplatesFile {
		// Такой файл при следующем чтении ушёл бы в .bad: не пишем, память прежняя.
		log.Printf("templates.json: %d байт больше предела, запись отменена", len(data))
		return model.ErrTemplatesFull
	}
	if t.bad || t.cut {
		if err := moveBad(dir, rename); err != nil {
			return fmt.Errorf("не удалось убрать повреждённый templates.json: %w", err)
		}
		t.bad, t.cut = false, false
	}
	if err := library.WriteFileAtomic(TemplatesPath(dir), data); err != nil {
		log.Printf("templates.json: %v", paths.NoPath(err))
		return fmt.Errorf("не удалось сохранить шаблоны: %w", err)
	}
	t.list = list
	return nil
}

// reread — второе чтение файла, не открывшегося при старте. Память была пуста, поэтому
// list — только добавленные шаблоны: они встают после прочитанных из файла. Снова не
// открылся — ошибка, файл не трогаем.
func (t *Templates) reread(dir string, list []model.Template) ([]model.Template, error) {
	var re Templates
	if err := re.read(dir); err != nil {
		log.Printf("templates.json снова не прочитан: %v", paths.NoPath(err))
		return nil, fmt.Errorf("не удалось прочитать templates.json: %w", err)
	}
	*t = re
	merged := append(slices.Clone(re.list), list...)
	if len(merged) > model.MaxTemplates {
		return nil, model.ErrTooManyTpl
	}
	return merged, nil
}

// inFile — шаблонов в файле до зажима; не разобрать — 0.
func inFile(data []byte) int {
	var f struct {
		Templates []json.RawMessage `json:"templates"`
	}
	if json.Unmarshal(data, &f) != nil {
		return 0
	}
	return len(f.Templates)
}

// moveBad переименовывает templates.json в templates.bad-<время>.json (при занятом
// имени — с номером); нет файла — ничего не делает.
func moveBad(dir string, rename func(from, to string) error) error {
	src := TemplatesPath(dir)
	if _, err := os.Stat(src); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return err
	}
	stamp := time.Now().Format("20060102-150405")
	for i := 0; ; i++ {
		name := "templates.bad-" + stamp
		if i > 0 {
			name = fmt.Sprintf("%s-%d", name, i)
		}
		dst := filepath.Join(dir, name+".json")
		if _, err := os.Lstat(dst); err == nil {
			continue
		}
		log.Printf("templates.json → %s", filepath.Base(dst))
		return rename(src, dst)
	}
}
