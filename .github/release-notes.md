Sosnina Albums — программа вёрстки свадебных фотоальбомов: выбираете папки с фото, раскладываете по разворотам вручную или автосборкой, выгружаете JPEG 300 dpi.

## Что нового в {{VERSION}}

{{CHANGES}}

Полный список — в [CHANGELOG.md](https://github.com/IliaSA101/sosninaalbums/blob/v{{VERSION}}/CHANGELOG.md).

## Установка (Windows 10/11)

Скачайте `SosninaAlbums-{{VERSION}}-windows-x64.exe` и запустите — установка не нужна.

Файл не подписан сертификатом, поэтому при первом запуске Windows может показать окно SmartScreen: нажмите «Подробнее» → «Выполнить в любом случае». Антивирус может спросить разрешение — это ложное срабатывание на новые программы.

Нужен WebView2 (в Windows 10/11 обычно уже есть; если нет — программа сама предложит его скачать).

## macOS

Скачайте `SosninaAlbums-{{VERSION}}-macos-universal.zip`, распакуйте и перенесите `sosninaalbums.app` в «Программы». Сборка универсальная: Apple Silicon (M1 и новее) и Intel.

Приложение не подписано сертификатом Apple, поэтому при первом запуске macOS его заблокирует: правый щелчок → «Открыть» → «Открыть»; в macOS 15 и новее — «Системные настройки» → «Конфиденциальность и безопасность» → «Всё равно открыть». Если macOS пишет, что приложение «повреждено», — в «Терминале»:

```bash
xattr -dr com.apple.quarantine /Applications/sosninaalbums.app
```

Собрать самому — [docs/BUILD-macOS.md](https://github.com/IliaSA101/sosninaalbums/blob/v{{VERSION}}/docs/BUILD-macOS.md).

---

Лицензия — MIT; сторонние компоненты — `THIRD_PARTY_NOTICES.txt`.
