package decor

import (
	"github.com/IliaSA101/sosninaalbums/internal/album/model"
	"github.com/IliaSA101/sosninaalbums/internal/decor/gen"
	"github.com/IliaSA101/sosninaalbums/internal/library"
)

// Встроенные стикеры (research §2, Ф2): ключ генератора + цвета по темам. ID — sa:st-<имя>, чтобы
// не совпасть с пресетом фона; в альбом уходят параметры генератора, не ID (решение №99).

func art(kind string, inks ...string) gen.Spec { return gen.Spec{Kind: kind, Inks: inks} }

// stickerPresets — порядок каталога. Темы те же, что у фонов.
var stickerPresets = []preset{
	{"sa:st-star-gold", "basic", "Золотая звезда", "", art("star")},
	{"sa:st-sparkle", "basic", "Искра", "", art("sparkle")},
	{"sa:st-burst", "basic", "Взрыв", "", art("burst")},
	{"sa:st-circle", "basic", "Круг", "", art("circle")},
	{"sa:st-blob", "basic", "Пятно", "", art("blob")},
	{"sa:st-plate-ivory", "basic", "Плашка под текст", "", art("plate")},
	{"sa:st-plate-dark", "basic", "Тёмная плашка", "", art("plate", "#2f3a56", "#e0a93d")},
	{"sa:st-banner", "basic", "Лента-баннер", "", art("banner")},
	{"sa:st-waves", "basic", "Волны", "", art("wave-band")},
	{"sa:st-photo-corner", "basic", "Уголок для фото", "", art("corner")},
	{"sa:st-brush", "basic", "Мазок", "", art("brush-stroke")},
	{"sa:st-arrow", "basic", "Стрелка", "", art("arrow")},

	{"sa:st-heart", "wedding", "Сердце", "", art("heart")},
	{"sa:st-hearts-pair", "wedding", "Пара сердец", "", art("hearts-pair")},
	{"sa:st-rings", "wedding", "Кольца", "", art("rings")},
	{"sa:st-flower", "wedding", "Цветок", "", art("flower")},
	{"sa:st-daisy", "wedding", "Ромашка", "", art("daisy")},
	{"sa:st-leaf", "wedding", "Лист", "", art("leaf")},
	{"sa:st-branch", "wedding", "Ветка", "", art("branch")},
	{"sa:st-arches", "wedding", "Бохо-арки", "", art("arches")},
	{"sa:st-champagne", "wedding", "Шампань", "", art("sparkle", "#e6c98a", "#f6e7c1")},
	{"sa:st-banner-blush", "wedding", "Пудровая лента", "", art("banner", "#e9b8c0", "#c98a96")},

	{"sa:st-balloon", "kids", "Шар", "", art("balloon")},
	{"sa:st-balloons", "kids", "Связка шаров", "", art("balloons")},
	{"sa:st-rainbow", "kids", "Радуга", "", art("rainbow")},
	{"sa:st-sun", "kids", "Солнце", "", art("sun")},
	{"sa:st-cloud", "kids", "Облако", "", art("cloud")},
	{"sa:st-confetti", "kids", "Горсть конфетти", "", art("confetti-pile")},
	{"sa:st-star-blue", "kids", "Голубая звезда", "", art("star", "#6fb7e8")},

	{"sa:st-flower-yellow", "kindergarten", "Жёлтый цветок", "", art("flower", "#f6c445", "#e8607a")},
	{"sa:st-rainbow-soft", "kindergarten", "Нежная радуга", "", art("rainbow", "#f4b6b6", "#f9dcae", "#c5e3c5", "#bcd6ee")},
	{"sa:st-cloud-pink", "kindergarten", "Розовое облако", "", art("cloud", "#f6d5dc", "#fbe9ee")},
	{"sa:st-blob-mint", "kindergarten", "Мятное пятно", "", art("blob", "#bfe3d0")},

	{"sa:st-pencil", "school", "Карандаш", "", art("pencil")},
	{"sa:st-book", "school", "Книга", "", art("book")},
	{"sa:st-apple", "school", "Яблоко", "", art("apple")},
	{"sa:st-arrow-blue", "school", "Синяя стрелка", "", art("arrow", "#5a7fa8")},

	{"sa:st-cap", "grad", "Шапка выпускника", "", art("cap")},
	{"sa:st-banner-navy", "grad", "Тёмная лента", "", art("banner", "#2f3a56", "#1f2a4d")},
	{"sa:st-star-grad", "grad", "Звезда", "", art("star", "#e0a93d")},
	{"sa:st-sparkle-grad", "grad", "Золотая искра", "", art("sparkle", "#e0a93d", "#f3d58a")},

	{"sa:st-bell", "holiday", "Колокольчик", "", art("bell")},
	{"sa:st-burst-gold", "holiday", "Золотой взрыв", "", art("burst", "#e0a93d", "#f6dc9b")},
	{"sa:st-confetti-party", "holiday", "Праздничное конфетти", "", art("confetti-pile", "#e85d75", "#f2b134", "#7cc47f", "#4fb0c6")},
	{"sa:st-balloons-gold", "holiday", "Золотые шары", "", art("balloons", "#e0a93d", "#f3d58a", "#c9a24e")},

	// Фирменные (спека 116): готовые картинки, ключ — gen/art/brand.go; цветов нет.
	{"sa:st-brand-albums-black", ThemeBrand, "sosnina albums, чёрный", "", art("brand-albums-black")},
	{"sa:st-brand-albums-white", ThemeBrand, "sosnina albums, белый", "", art("brand-albums-white")},
	{"sa:st-brand-films-black", ThemeBrand, "sosnina films, чёрный", "", art("brand-films-black")},
	{"sa:st-brand-films-white", ThemeBrand, "sosnina films, белый", "", art("brand-films-white")},
}

// lightStickers — стикеры почти белого цвета: на светлой шашечке плитки их не видно, плитка рисует
// тёмную (Item.Dark, спека 116). Список ведёт TestStickerDarkMatchesPixels по рисунку.
var lightStickers = map[string]bool{
	"sa:st-plate-ivory":        true,
	"sa:st-brand-albums-white": true,
	"sa:st-brand-films-white":  true,
}

// Sticker — что ставит щелчок по плитке стикера (AddSticker и ReplaceSticker в App): встроенный —
// параметры генератора, свой — фото с ключом библиотеки. Aspect — ширина/высота первой вставки.
type Sticker struct {
	Art    *gen.Spec       // встроенный; nil — свой
	Photo  model.PhotoInfo // свой: ID = LibPhotoID(Lib), Path, Lib, W, H, Size
	Aspect float64
}

// Sticker — разбор ID плитки стикера; путь свой — из индекса, не от интерфейса.
func (s *Store) Sticker(id string) (Sticker, error) {
	if p, ok := findPreset(stickerPresets, id); ok {
		if !gen.Known(p.spec.Kind) {
			return Sticker{}, ErrNotFound
		}
		sp := gen.Clamp(p.spec)
		rs, _ := gen.Resolve(sp)
		return Sticker{Art: &sp, Aspect: rs.Aspect}, nil
	}
	if !model.ValidLibKey(id) {
		return Sticker{}, ErrNotFound
	}
	s.mu.RLock()
	e := s.entryOf(KindSticker, id)
	path, ok := s.files[KindSticker][id]
	ok = ok && (e != nil || s.inPack(KindSticker, id))
	s.mu.RUnlock()
	if !ok {
		return Sticker{}, ErrNotFound
	}
	info, err := library.ReadInfo(path)
	if err != nil || info.W <= 0 || info.H <= 0 {
		return Sticker{}, ErrNotFound
	}
	return Sticker{
		Photo:  model.PhotoInfo{ID: model.LibPhotoID(id), Path: path, Lib: id, W: info.W, H: info.H, Size: info.Size},
		Aspect: float64(info.W) / float64(info.H),
	}, nil
}

// presetsOf — встроенные плитки вида.
func presetsOf(kind string) []preset {
	switch kind {
	case KindSticker:
		return stickerPresets
	case KindSpread, KindFrame: // встроенные развороты — spreadBuiltins, рамочки — frameBuiltins
		return nil
	}
	return bgPresets
}
