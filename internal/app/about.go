package app

import (
	"errors"
	"net/url"

	"github.com/IliaSA101/sosninaalbums/internal/app/appkit"
)

// Адреса окна «О программе» (101). Зашиты здесь: интерфейс передаёт только id, как у OpenPrintSite.
const (
	telegramURL = "https://t.me/Ilya_Sosnin"
	marinaURL   = "https://sosnina-films.ru/contacts"
	// authorsURL — «Об авторах» в окне «Наборы» (129 §6): README библиотеки, раздел «Авторы» (#авторы).
	authorsURL = "https://github.com/IliaSA101/sosninaalbums-library#%D0%B0%D0%B2%D1%82%D0%BE%D1%80%D1%8B"
)

// errUnknownLink — id ссылки вне белого списка linkURL.
var errUnknownLink = errors.New("неизвестная ссылка")

// linkURL — адрес по id (source, license, telegram, marina, authors, bug, idea, videos, download); ver — версия программы.
// Ссылки на файлы идут на метку своей версии (v<ver>): код и текст лицензии те же, что в
// запущенной сборке; у dev метки нет — берётся основная ветка. false — id неизвестен.
func linkURL(id, ver string) (string, bool) {
	tagged := ver != "" && ver != "dev"
	switch id {
	case "source":
		if tagged {
			return appkit.RepoURL + "/tree/" + url.PathEscape("v"+ver), true
		}
		return appkit.RepoURL, true
	case "license":
		if tagged {
			return appkit.RepoURL + "/blob/" + url.PathEscape("v"+ver) + "/NOTICE.md", true
		}
		return appkit.RepoURL + "/blob/main/NOTICE.md", true
	case "telegram":
		return telegramURL, true
	case "marina":
		return marinaURL, true
	case "authors":
		return authorsURL, true
	case "bug":
		q := url.Values{"template": {"bug.yml"}}
		if tagged {
			q.Set("version", ver) // поле «Версия» формы уже заполнено
		}
		return appkit.RepoURL + "/issues/new?" + q.Encode(), true
	case "idea":
		return appkit.RepoURL + "/issues/new?template=idea.yml", true
	case "download":
		// Страница последнего выпуска: он откроет любой более старый файл и несёт свежие правки (112).
		return appkit.RepoURL + "/releases/latest", true
	case "videos":
		// Всегда main: новые ролики появляются на странице без новой версии программы (113).
		return appkit.RepoURL + "/blob/main/docs/VIDEOS.md", true
	}
	return "", false
}

// OpenLink открывает в браузере ссылку окна «О программе» по id: source, license, telegram,
// marina, bug, idea, videos, download. Адрес берётся только из linkURL — от интерфейса его не принимаем (иначе страница
// в окне открывала бы что угодно). Неизвестный id — ошибка, браузер не трогаем.
func (a *App) OpenLink(id string) error {
	u, ok := linkURL(id, appVersion(wailsJSON))
	if !ok {
		return errUnknownLink
	}
	openURL(a, u)
	return nil
}
