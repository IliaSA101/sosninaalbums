Sosnina Albums — программа вёрстки свадебных фотоальбомов: выбираете папки с фото, раскладываете по разворотам вручную или автосборкой, выгружаете JPEG 300 dpi.

## Что нового в {{VERSION}}

{{CHANGES}}

Полный список — в [CHANGELOG.md](https://github.com/IliaSA101/sosninaalbums/blob/v{{VERSION}}/CHANGELOG.md).

## Установка (Windows 10/11)

Скачайте `SosninaAlbums-{{VERSION}}-windows-x64.zip` и распакуйте: откройте его в «Загрузках» и нажмите «Извлечь все» → «Извлечь». Не запускайте программу прямо из архива. В распакованной папке `Sosnina Albums {{VERSION}}` — `SosninaAlbums.exe`, инструкция «Как установить (Windows)», «Что нового» и лицензии. Установка не нужна: запустите `SosninaAlbums.exe`, папку можно держать где угодно.

Программа не подписана сертификатом, поэтому при первом запуске Windows может показать окно SmartScreen: нажмите «Подробнее» → «Выполнить в любом случае». Антивирус может спросить разрешение — это ложное срабатывание на новые программы.

Нужен WebView2 (в Windows 10/11 обычно уже есть; если нет — программа сама предложит его скачать).

Подробная инструкция со снимками — [docs/install/guide.md](https://github.com/IliaSA101/sosninaalbums/blob/v{{VERSION}}/docs/install/guide.md); то же файлом — `SosninaAlbums-{{VERSION}}-install-guide-windows-ru.pdf` (или `.docx`) ниже, её копия лежит и в архиве.

## macOS

Скачайте `SosninaAlbums-{{VERSION}}-macos-universal.zip`. Safari распаковывает его сам, в других браузерах — двойной щелчок по архиву. Появится папка `Sosnina Albums {{VERSION}}`: в ней `sosninaalbums.app`, инструкция «Как установить (Mac)», «Что нового» и лицензии. Перенесите `sosninaalbums.app` в «Программы» (левая колонка Finder). Сборка универсальная: Apple Silicon (M1 и новее) и Intel.

Приложение не подписано сертификатом Apple, поэтому при первом запуске macOS его заблокирует: правый щелчок → «Открыть» → «Открыть»; в macOS 15 и новее — «Системные настройки» → «Конфиденциальность и безопасность» → «Все равно открыть». Если macOS пишет, что приложение «повреждено», — в «Терминале»:

```bash
xattr -dr com.apple.quarantine /Applications/sosninaalbums.app
```

Те же шаги со снимками для каждой версии macOS — в [инструкции](https://github.com/IliaSA101/sosninaalbums/blob/v{{VERSION}}/docs/install/guide.md).

Собрать самому — [docs/BUILD-macOS.md](https://github.com/IliaSA101/sosninaalbums/blob/v{{VERSION}}/docs/BUILD-macOS.md).

## Linux

Ubuntu 22.04 и новее, Linux Mint 21+, Debian 12+: скачайте `SosninaAlbums-{{VERSION}}-linux-amd64.deb` и откройте двойным щелчком (или `sudo apt install ./SosninaAlbums-{{VERSION}}-linux-amd64.deb`). Пакет сам поставит GTK 3 и WebKitGTK 4.1, программа появится в меню «Графика» со значком. Обновление — новая `.deb` поверх, удаление — `sudo apt remove sosninaalbums`.

Другие дистрибутивы: `SosninaAlbums-{{VERSION}}-linux-amd64.tar.gz` — распакуйте и запустите `sosninaalbums` из папки `Sosnina Albums {{VERSION}}` (в системе должны быть GTK 3 и WebKitGTK 4.1). Внутри — инструкция «Как установить (Linux)», «Что нового» и лицензии.

---

Инструкции для каждой системы (`SosninaAlbums-{{VERSION}}-install-guide-…`, docx и pdf) лежат отдельными файлами ниже: их можно прочитать до скачивания архива.

Лицензия — GNU GPL v3 (только версия 3) с дополнительным условием об имени и авторстве, см. `LICENSE` и `NOTICE.md` (версии до 0.22.x выпускались под MIT); сторонние компоненты — `THIRD_PARTY_NOTICES.txt` (лежит и в архивах).
