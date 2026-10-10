# Сборка для macOS

Основная платформа программы — Windows, сборку для Mac делают на самом Mac: Wails под macOS собирается только там (нужны системные библиотеки Apple). Ниже — шаги для MacBook (Apple Silicon или Intel), примерно 30–40 минут в первый раз.

## 1. Что поставить один раз

1. Инструменты Apple: в «Терминале» — `xcode-select --install` (откроется окно установки, ~5 мин).
2. Homebrew, если его нет: команда с сайта https://brew.sh.
3. Go, Node.js и git:

   ```bash
   brew install go node git
   ```

   Go нужен версии 1.26 или новее (`go version`), Node — LTS (`node -v`).

4. Wails той же версии, что в проекте (`go.mod`):

   ```bash
   go install github.com/wailsapp/wails/v2/cmd/wails@v2.16.0
   echo 'export PATH="$PATH:$HOME/go/bin"' >> ~/.zshrc && source ~/.zshrc
   wails doctor
   ```

   `wails doctor` должен закончиться строкой «Your system is ready for Wails development!».

## 2. Скачать исходники нужной версии

```bash
git clone https://github.com/IliaSA101/sosninaalbums.git
cd sosninaalbums
git checkout v0.9.0
```

## 3. Собрать

```bash
cd frontend && npm ci && cd ..
wails build -platform darwin/universal -clean
```

`darwin/universal` — одно приложение и для Apple Silicon (M1–M4), и для Intel. Результат: `build/bin/Sosnina Albums.app`.

## 4. Проверить

Приложение не подписано сертификатом Apple, поэтому при первом запуске: правый щелчок по `Sosnina Albums.app` → «Открыть» → «Открыть». Если macOS пишет «повреждено» — снять метку карантина:

```bash
xattr -dr com.apple.quarantine "build/bin/Sosnina Albums.app"
```

Проверка за 2 минуты: окно открылось и в шапке версия; добавить папку с фото — миниатюры видны; «Автосборка»; экспорт одного разворота в пустую папку; закрыть и открыть — альбом на месте.

## 5. Упаковать для скачивания

`ditto` сохраняет внутреннее устройство `.app` (обычный zip из Finder тоже подходит):

```bash
ditto -c -k --keepParent "build/bin/Sosnina Albums.app" SosninaAlbums-0.9.0-macos-universal.zip
```

Файл `SosninaAlbums-0.9.0-macos-universal.zip` передать владельцу репозитория (или, если вас добавили соавтором, выложить самому — шаг 6).

## 6. Выложить в релиз (владелец или соавтор)

Через сайт: Releases → v0.9.0 → Edit → перетащить zip в «Attach binaries» → Update release.

Или командой (GitHub CLI, после `gh auth login`):

```bash
gh release upload v0.9.0 SosninaAlbums-0.9.0-macos-universal.zip -R IliaSA101/sosninaalbums
```

## Если что-то не так

- Ошибка на `npm ci` — проверить `node -v` (нужна LTS) и повторить.
- `wails: command not found` — не добавлен `~/go/bin` в PATH (шаг 1.4).
- Программа запускается, но что-то работает не так, как на Windows: сборка для Mac пока не проверялась — пришлите описание и снимок экрана, исправим в следующей версии.
