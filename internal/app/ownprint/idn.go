package ownprint

import (
	"strings"
	"unicode"
	"unicode/utf8"

	"golang.org/x/net/idna"

	"github.com/IliaSA101/sosninaalbums/internal/album/model/printprof"
)

// Кириллический адрес в окне «Своя типография» (123 Б3, решение 0.26-123-52): пользователь вводит
// https://фабрика.рф/…, Go переводит хост в xn--… до printprof.CheckURL и пишет в файл только его;
// при открытии окна хост показан Юникодом, если метка безопасна (printprof.DisplayHost). Разбор
// файла остаётся строгим: кириллический хост там по-прежнему отказ.

// maxIDNInput — вход длиннее не переводится: printprof.CheckURL отвергнет его по длине (300 знаков).
const maxIDNInput = 300

// idnProfile — строгий перевод хоста для просмотра: отображение UTS 46, Bidi, ContextJ, только
// LDH для ASCII, дефисы по правилам, длина меток и имени DNS; не переходный (ß остаётся ß).
// Наружу — только IDNProfile() (разностная проверка показа в тестах App): переменную не подменить.
var idnProfile = idna.New(idna.MapForLookup(), idna.Transitional(false), idna.BidiRule(), idna.CheckJoiners(true),
	idna.CheckHyphens(true), idna.StrictDomainName(true), idna.VerifyDNSLength(true), idna.ValidateLabels(true))

// IDNProfile — профиль строгого перевода хоста (только чтение).
func IDNProfile() *idna.Profile { return idnProfile }

// idnBadHost — текст под полем, если хост не перевёлся в доменное имя (подробности x/net не нужны).
const idnBadHost = "Хост не разбирается как доменное имя: только буквы, цифры и «-» между точками"

// SplitHost делит адрес https://хост[:порт]/хвост на части; ok=false — не https, есть имя
// пользователя, IP-литерал в скобках или пустой хост: такой адрес оставляет проверке printprof.
// Экспортирована для тестов App (фаззинг IDNURL).
func SplitHost(s string) (pre, host, post string, ok bool) {
	const scheme = "https://"
	if !strings.HasPrefix(s, scheme) {
		return "", "", "", false
	}
	rest := s[len(scheme):]
	end := strings.IndexAny(rest, "/?#")
	if end < 0 {
		end = len(rest)
	}
	auth, tail := rest[:end], rest[end:]
	if auth == "" || strings.ContainsAny(auth, "@[]") {
		return "", "", "", false
	}
	host = auth
	if i := strings.LastIndexByte(auth, ':'); i >= 0 {
		host = auth[:i]
		tail = auth[i:] + tail // порт остаётся как есть: «:443» проверит printprof
	}
	return scheme, host, tail, host != ""
}

// IDNURL — адрес с хостом в ASCII: кириллица и прочий Юникод в хосте → xn--…. Адрес без
// не-ASCII в хосте возвращается как есть (его судит printprof.CheckURL). Второй результат — русский текст
// под полем адреса: хост не разбирается, смесь алфавитов в метке, двойник латиницы.
func IDNURL(s string) (string, string) {
	s = strings.TrimSpace(s)
	if utf8.RuneCountInString(s) > maxIDNInput || !utf8.ValidString(s) {
		return s, ""
	}
	pre, host, post, ok := SplitHost(s)
	if !ok || isASCII(host) {
		return s, ""
	}
	// UTS 46 молча выбрасывает «нулевую ширину» и мягкий перенос: адрес с невидимкой стал бы чужим сайтом.
	if strings.ContainsFunc(host, func(r rune) bool { return unicode.In(r, unicode.Cc, unicode.Cf, unicode.Z) }) {
		return s, idnBadHost
	}
	ascii, err := idnProfile.ToASCII(host)
	uni := ""
	if err == nil {
		uni, err = idnProfile.ToUnicode(ascii) // обратно: метки, как их увидит человек
	}
	if err != nil || !isASCII(ascii) {
		return s, idnBadHost
	}
	for _, l := range strings.Split(uni, ".") {
		if why := printprof.LabelProblem(l); why != "" {
			return s, upperFirst(why)
		}
	}
	return pre + ascii + post, ""
}

// IDNShow — адрес для окна: хост из xn--… в Юникоде, если метка безопасна; прочее как есть.
func IDNShow(s string) string {
	pre, host, post, ok := SplitHost(s)
	if !ok || !strings.Contains(strings.ToLower(host), "xn--") {
		return s
	}
	return pre + printprof.DisplayHost(host) + post
}

func isASCII(s string) bool {
	for i := 0; i < len(s); i++ {
		if s[i] >= utf8.RuneSelf {
			return false
		}
	}
	return true
}
