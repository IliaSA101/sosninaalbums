# Общее окружение двойников для Linux и macOS (check.sh, build.sh, stand.sh): подключается через `. scripts/env.sh`.
# В .ps1 этот блок повторён в каждом скрипте; здесь — один файл. Правится вместе с блоком кэшей в .ps1.
# Ждёт переменную root (корень рабочей копии); задаёт main, GOTMPDIR, кэши Go, GOTOOLCHAIN, PATH к Node и инструментам.

# Кэши Go и golangci-lint — в .gotmp основной копии (общие для worktree), как в .ps1 и scripts/goenv.mjs.
# GOTMPDIR на Linux антивирусу не нужен, но держим тот же .gotmp: туда смотрят temp-clean и логи стендов.
common=$(git -C "$root" rev-parse --path-format=absolute --git-common-dir 2>/dev/null) && main=$(dirname "$common") || main=$root
export GOTMPDIR="$root/.gotmp"
mkdir -p "$GOTMPDIR"
export GOCACHE="$main/.gotmp/cache-go"
export GOLANGCI_LINT_CACHE="$main/.gotmp/cache-golangci"

# Тулчейн — ровно из go.mod (toolchain), как в CI: машина с более новым Go иначе проверяла бы и собирала своим,
# и сборки двух машин не сошлись бы побайтно.
tc=$(awk '$1 == "toolchain" { print $2; exit }' "$root/go.mod")
[ -n "$tc" ] && export GOTOOLCHAIN=$tc

# Версии Node и golangci-lint — из CI (.github/workflows/ci.yml): локальная проверка той же версией, что и CI.
ci="$root/.github/workflows/ci.yml"
node_want=$(sed -n 's/.*node-version: *"\{0,1\}\([0-9.]*\)"\{0,1\}.*/\1/p' "$ci" | head -n 1)
gcl_want=$(awk '/golangci-lint-action/ { f = 1 } f && $1 == "version:" { print $2; exit }' "$ci")
# Node нужной версии из nvm — впереди PATH (у nvm по умолчанию может стоять другая, например у соседнего проекта).
if [ -n "$node_want" ] && [ "$(node -v 2>/dev/null)" != "v$node_want" ] && [ -d "$HOME/.nvm/versions/node/v$node_want/bin" ]; then
  PATH="$HOME/.nvm/versions/node/v$node_want/bin:$PATH"
fi

# Инструменты Go проекта (wails, golangci-lint, govulncheck): SA_TOOLS, затем папка sosninaalbums-tools рядом
# с основной копией, затем GOBIN и GOPATH/bin. Своя папка, а не ~/go/bin: там версии соседних проектов.
for d in "${SA_TOOLS:-}" "$(dirname "$main")/sosninaalbums-tools" "${GOBIN:-}" "$(go env GOPATH 2>/dev/null)/bin"; do
  if [ -n "$d" ] && [ -d "$d" ]; then PATH="$PATH:$d"; tools=${tools:-$d}; fi
done
# Папка инструментов проекта — раньше общих: иначе golangci-lint из ~/go/bin соседа перекрыл бы свой.
[ -n "${tools:-}" ] && PATH="$tools:$PATH"
export PATH

# Wails v2 на Linux собирается с WebKitGTK 4.1 только с тегом webkit2_41 (4.0 на Ubuntu 24.04 нет); на macOS тег не нужен.
wails_tags=''
[ "$(uname -s)" = Linux ] && wails_tags=webkit2_41
