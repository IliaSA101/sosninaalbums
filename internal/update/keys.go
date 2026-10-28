package update

// Keys — открытые ключи подписи выпусков, сырые 32 байта ed25519 в base64: текущий и запасной
// (design §5.3; отдельная пара, не ключ библиотеки). Закрытые — у владельца вне git.
var Keys = [...]string{
	"D3obGCPx1ud3g1NZAejMrWxxykBTlLQjru75UxmUyYs=", // текущий: docs/private/release-key.pem (23.10.2026)
	"ZSmrmbpNgRv+lMTbYOTS/29fHJR804u+isdDZWJjTaE=", // запасной: docs/private/release-key-2.pem
}
