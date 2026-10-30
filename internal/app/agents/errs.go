package agents

import (
	"errors"
	"strings"
)

// agentErr — единственный вид ошибки, текст которой уходит агенту: фиксированные строки без путей
// и без данных альбома (design §3 «Одна точка ответа»). Всё остальное обёртка заменяет на
// errInternal, а подробности пишет в log.txt без путей.
type agentErr struct{ text string }

func (e *agentErr) Error() string { return e.text }

func newAgentErr(text string) *agentErr { return &agentErr{text: text} }

// Список ошибок наружу. Текст читает модель и пересказывает человеку: по-русски, со словами,
// где что включить. errInternal («внутренняя ошибка программы») — в sessions.go.
var (
	errNoAlbum   = newAgentErr("альбом не открыт — откройте или создайте альбом в Sosnina Albums")
	errNoSpread  = newAgentErr("нет такого разворота — номер с 1, «cover» (обложка) или id из list_spreads")
	errNoPhoto   = newAgentErr("нет такого фото — id берите из list_photos")
	errNoImages  = newAgentErr("Картинки агенту выключены: Файл ▸ ИИ-агенты ▸ «+ картинки»")
	errTimeout   = newAgentErr("время вышло — попробуйте ещё раз или запросите меньше")
	errCallBusy  = newAgentErr("занято: идут другие запросы — повторите через секунду")
	errTooBig    = newAgentErr("ответ слишком велик — уменьшите limit")
	errTooBigOne = newAgentErr("ответ слишком велик для одного вызова — запросите другой разворот или другое фото")
	errBadKind   = newAgentErr("inAlbum есть только у фонов, стикеров, рамок и макетов (kind: bg, sticker, frame, look)")
	errPhotoGone = newAgentErr("файл этого фото недоступен (нет на месте или не читается) — состояние видно в list_photos, поле status")
)

// Ошибки правок (149, design §1). Ни в одной нет пути, имени файла и текста агента.
var (
	errNoEdits      = errDenied // правки запретили, пока вопрос висел (WhyOff): то же «Запрещено человеком…» (150)
	errElevated     = newAgentErr("Программа запущена от администратора — правки агентам недоступны: человеку нужно запустить Sosnina Albums обычным способом")
	errEditBusy     = newAgentErr("идёт другая правка или вопрос — повторите через несколько секунд")
	errEditRate     = newAgentErr("слишком много правок за минуту (не больше 20) — подождите")
	errAlbumChanged = newAgentErr("открыт другой альбом — перечитайте get_album")
	errAlbumEdited  = newAgentErr("альбом изменился после вашего чтения — перечитайте get_album")
	errUserBusy     = newAgentErr("человек сейчас правит альбом — повторите через несколько секунд")
	errBackup       = newAgentErr("не удалось сделать резервную копию — правка не сделана")
	errDeclined     = newAgentErr("человек отказал в окне программы — не повторяйте без его новой просьбы в чате")
	errAskTimeout   = newAgentErr("человек не ответил в окне программы — вопрос закрыт; повторите, только если он попросит снова")
	errAskBusy      = newAgentErr("в окне программы уже открыт другой вопрос — подождите ответа человека")
	errAskCooldown  = newAgentErr("человек недавно отказал — вопросов пока нет; не повторяйте без его новой просьбы в чате")
	errAskMuted     = newAgentErr("вопросы агентов отключены (человек отказывал несколько раз): они вернутся сами через несколько минут, " +
		"а раньше — по кнопке «Разрешить вопросы» в Файл ▸ ИИ-агенты")
	errAskOff = newAgentErr("вопросы агентов отключены после трёх отказов: вернутся по кнопке «Разрешить вопросы» " +
		"в Файл ▸ ИИ-агенты или после перезапуска программы")
	errAskGone       = newAgentErr("такого вопроса нет — он закрыт или задан не этим подключением")
	errAskClosed     = newAgentErr("вопрос уже закрыт")
	errClosing       = newAgentErr("программа закрывается")
	errScanning      = newAgentErr("программа ещё читает папки с фото — повторите через минуту")
	errNoFree        = newAgentErr("свободных фото нет: все уже в альбоме, на фоне или отклонены")
	errUndoNotOwn    = newAgentErr("после вашей правки альбом менялся — попросите человека нажать Ctrl+Z")
	errUndoNoOwn     = newAgentErr("в этом подключении агент ещё не правил альбом — отменить прежние правки может человек: Ctrl+Z")
	errCreateNoUndo  = newAgentErr("создание альбома не отменяется: прежний альбом — в Файл ▸ Последние")
	errNotReady      = newAgentErr("программа ещё запускается — повторите через несколько секунд")
	errFolderTooWide = newAgentErr("выберите папку съёмки, а не весь диск, домашнюю или системную папку")
	errFolderOurs    = newAgentErr("это папка самой программы — выберите папку со съёмкой")
	errDraftLocked   = newAgentErr("прежний черновик занят — новый альбом не создан")
	errLate          = newAgentErr("срок вызова вышел — правка не применена, проверьте get_album перед повтором")
	errBadFolder     = newAgentErr("folder_hint длиннее 4096 байт или не UTF-8")
)

// Ошибки выгрузки и открытия (152). Без путей и имён файлов.
var (
	errProgBusy   = newAgentErr("программа занята выгрузкой по ссылке — повторите через несколько секунд")
	errStopped    = newAgentErr("человек остановил действие в окне программы — не повторяйте без его новой просьбы в чате")
	errNoRecent   = newAgentErr("такого альбома нет в последних — ключ берите из list_albums или вызовите open_album без ключа")
	errOpenGone   = newAgentErr("файла этого альбома нет на месте — вызовите open_album без ключа, человек выберет файл сам")
	errOpenRemote = newAgentErr("этот альбом лежит не на этом компьютере — вызовите open_album без ключа, человек выберет файл сам")
	errOpenFailed = newAgentErr("альбом не открылся (файл повреждён, чужой или недоступен) — причину видит человек в программе")
	errExporting  = newAgentErr("идёт выгрузка альбома — повторите, когда она закончится")
	errBadExport  = newAgentErr("неверные настройки выгрузки: диапазон, качество, dpi или типография не подходят")
	errOpenNewer  = newAgentErr("альбом сохранён более новой версией программы — открыть его нельзя, человек видит окно с подробностями")
)

// Ошибки ступени 3б (151, design §1.8). Тексты без путей и без данных альбома.
var (
	errNoCover      = newAgentErr("у этого альбома нет обложки")
	errNoDecor      = newAgentErr("нет такого фона или стикера — id берите из list_decor")
	errNoProfile    = newAgentErr("нет такой типографии или формата — смотрите list_print_profiles")
	errStickerLimit = newAgentErr("на листе или в альбоме нет места для стольких стикеров")
	errBadPage      = newAgentErr("на этой странице листа нет фото или сетка мала для ids: offset должен быть меньше total, " +
		"а grid × grid — не меньше числа ids")
)

// badArgs — отказ схемы входа: в тексте только то, что прислал сам агент (имена полей, значения).
func badArgs(detail string) *agentErr {
	r := []rune(detail)
	if len(r) > 300 {
		detail = string(r[:300]) + "…"
	}
	return newAgentErr("неверные аргументы вызова: " + detail)
}

// toAgent приводит ошибку к тому, что можно показать агенту; прочее — errInternal. Второй
// результат — исходная ошибка для log.txt (nil, если она своя).
func toAgent(err error) (out *agentErr, orig error) {
	var ae *agentErr
	if errors.As(err, &ae) {
		return ae, nil
	}
	return errInternal, err
}

// errCoverNotTarget — «cover» там, где обложка не цель правки: честный текст вместо «нет такого
// разворота — … «cover»…», который предлагал бы только что отвергнутое. Слова — по смыслу инструмента.
func errCoverNotTarget(tool string) *agentErr {
	switch tool {
	case "set_background":
		return newAgentErr("на обложку фон этим инструментом не ставится — номер разворота с 1 или id из list_spreads")
	case "vary_layouts":
		return newAgentErr("у обложки нет раскладки для перебора — номер разворота с 1 или id из list_spreads")
	}
	return newAgentErr("обложка здесь не цель — номер разворота с 1 или id из list_spreads")
}

// errNoSpreadFlat — неизвестный разворот там, где обложка не цель: без «cover» в подсказке.
var errNoSpreadFlat = newAgentErr("нет такого разворота — номер с 1 или id из list_spreads")

// noSpreadFor — «нет такого разворота» по смыслу инструмента: coverOK — обложка допустима
// (get_spread, show_spread), тогда подсказка с «cover»; иначе errNoSpreadFlat. Один источник текстов.
func noSpreadFor(coverOK bool) *agentErr {
	if coverOK {
		return errNoSpread
	}
	return errNoSpreadFlat
}

// isCoverRef — ссылка агента на обложку («cover» без учёта регистра).
func isCoverRef(ref string) bool { return strings.EqualFold(strings.TrimSpace(ref), "cover") }
