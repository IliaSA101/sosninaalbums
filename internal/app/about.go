package app

import (
	"errors"
	"net/url"
)

// Адреса окна «О программе» (101). Зашиты здесь: интерфейс передаёт только id, как у OpenFsfera.
const (
	repoURL     = "https://github.com/IliaSA101/sosninaalbums"
	telegramURL = "https://t.me/Ilya_Sosnin"
	marinaURL   = "https://sosnina-films.ru/contacts"
)

// errUnknownLink — id ссылки вне белого списка linkURL.
var errUnknownLink = errors.New("неизвестная ссылка")

// linkURL — адрес по id (source, license, telegram, marina, bug, idea); ver — версия программы.
// Ссылки на файлы идут на метку своей версии (v<ver>): код и текст лицензии те же, что в
// запущенной сборке; у dev метки нет — берётся основная ветка. false — id неизвестен.
func linkURL(id, ver string) (string, bool) {
	tagged := ver != "" && ver != "dev"
	switch id {
	case "source":
		if tagged {
			return repoURL + "/tree/" + url.PathEscape("v"+ver), true
		}
		return repoURL, true
	case "license":
		if tagged {
			return repoURL + "/blob/" + url.PathEscape("v"+ver) + "/NOTICE.md", true
		}
		return repoURL + "/blob/main/NOTICE.md", true
	case "telegram":
		return telegramURL, true
	case "marina":
		return marinaURL, true
	case "bug":
		q := url.Values{"template": {"bug.yml"}}
		if tagged {
			q.Set("version", ver) // поле «Версия» формы уже заполнено
		}
		return repoURL + "/issues/new?" + q.Encode(), true
	case "idea":
		return repoURL + "/issues/new?template=idea.yml", true
	}
	return "", false
}

// OpenLink открывает в браузере ссылку окна «О программе» по id: source, license, telegram,
// marina, bug, idea. Адрес берётся только из linkURL — от интерфейса его не принимаем (иначе страница
// в окне открывала бы что угодно). Неизвестный id — ошибка, браузер не трогаем.
func (a *App) OpenLink(id string) error {
	u, ok := linkURL(id, appVersion(wailsJSON))
	if !ok {
		return errUnknownLink
	}
	openURL(a, u)
	return nil
}
