#!/usr/bin/env bash
# Все проверки перед сдачей одной командой — двойник scripts/check.ps1 для Linux и macOS:
#   scripts/check.sh [--go] [--ui] [--race]      (принимаются и -Go, -Ui, -Race, как у .ps1)
# Без флагов идут и Go, и фронт. --race — Go-тесты с детектором гонок (gcc, cgo; в конце цикла и перед релизом) и govulncheck.
# ПРАВЯТСЯ ВМЕСТЕ: шаги, их имена и пороги — как в check.ps1. Отличия только платформенные: окружение — scripts/env.sh;
# на Linux лишний шаг «go vet windows» (файлы *_windows.go иначе здесь не компилирует никто, CI — только на windows-latest).
set -u

go_only=0 ui_only=0 race=0
for a in "$@"; do
  case "$a" in
    --go | -go | -Go) go_only=1 ;;
    --ui | -ui | -Ui) ui_only=1 ;;
    --race | -race | -Race) race=1 ;;
    *) echo "неизвестный флаг: $a (есть --go, --ui, --race)" >&2; exit 2 ;;
  esac
done
run_go=0 run_ui=0
{ [ $go_only = 1 ] || [ $ui_only = 0 ]; } && run_go=1
{ [ $ui_only = 1 ] || [ $go_only = 0 ]; } && run_ui=1

root=$(CDPATH='' cd -- "$(dirname -- "$0")/.." && pwd)
cd "$root" || exit 1
. "$root/scripts/env.sh"
# Тег webkit2_41 нужен Go-пакетам, которые тянут окно Wails (main, internal/app): без него cgo ищет WebKitGTK 4.0.
[ -n "$wails_tags" ] && export GOFLAGS="${GOFLAGS:+$GOFLAGS }-tags=$wails_tags"

if [ -t 1 ]; then red=$'\e[31m' green=$'\e[32m' yellow=$'\e[33m' off=$'\e[0m'; else red='' green='' yellow='' off=''; fi
failed=()
now() { date +%s.%N; }
total_start=$(now)

# Запускает шаг, прячет вывод, печатает одну строку; при провале — хвост вывода.
# --empty-out: шаг провален, если вывод непустой (gofmt -l). --no-warn: провален, если в выводе предупреждение.
# --dir <папка>: где запускать.
step() {
  local name=$1 empty=0 nowarn=0 dir=$root
  shift
  while :; do
    case "${1:-}" in
      --empty-out) empty=1; shift ;;
      --no-warn) nowarn=1; shift ;;
      --dir) dir=$2; shift 2 ;;
      *) break ;;
    esac
  done
  local start out code ok=1
  start=$(now)
  out=$(cd "$dir" && "$@" 2>&1)
  code=$?
  [ $code -ne 0 ] && ok=0
  [ $empty = 1 ] && [ -n "$(printf '%s' "$out" | tr -d '[:space:]')" ] && ok=0
  [ $nowarn = 1 ] && printf '%s\n' "$out" | grep -Eiq '\bwarn(ing)?\b|deprecat|\(!\)' && ok=0
  local t
  t=$(awk -v a="$start" -v b="$(now)" 'BEGIN { printf "%.1fs", b - a }')
  if [ $ok = 1 ]; then
    echo "${green}OK   $name ($t)${off}"
  else
    echo "${red}FAIL $name ($t)${off}"
    printf '%s\n' "$out" | tail -n 30 | sed 's/^/     /'
    failed+=("$name")
  fi
}

# Пустой вывод — чисто (как CheckSizes/CheckFolders в .ps1: код выхода не важен, важен список).
check_sizes() { node scripts/limits.mjs sizes; return 0; }
check_folders() { node scripts/limits.mjs folders; return 0; }
# gofmt — того же тулчейна, что go (GOTOOLCHAIN): системный gofmt другой версии форматирует иначе.
go_fmt() {
  local files
  files=$(git ls-files --cached --others --exclude-standard -- '*.go')
  [ -z "$files" ] && return 0
  printf '%s\n' "$files" | xargs "$(go env GOROOT)/bin/gofmt" -l
}
# Версия golangci-lint — как в CI: разные версии дают разные замечания.
gcl_run() {
  local have
  have=$(golangci-lint version 2>/dev/null | sed -n 's/.*version \([0-9.]*\).*/v\1/p')
  if [ -n "$gcl_want" ] && [ "$have" != "$gcl_want" ]; then
    echo "golangci-lint $have, а в CI $gcl_want: go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@$gcl_want (GOBIN — папка инструментов проекта)"
    return 1
  fi
  node scripts/heavy.mjs "$(command -v golangci-lint)" run --allow-parallel-runners . ./internal/...
}
node_version() {
  [ -z "$node_want" ] || [ "$(node -v)" = "v$node_want" ] || { echo "node $(node -v), а в CI v$node_want: nvm install $node_want"; return 1; }
}

# Json-вывод go test и vitest: по нему test-baseline сверяет имена тестов и покрытие без второго прогона.
go_json="$GOTMPDIR/go-test.json"
ui_json="$GOTMPDIR/vitest.json"
rm -f "$go_json" "$ui_json"

if [ $run_go = 1 ]; then
  # Только файлы из git: .gotmp в .gitignore, worktree агентов в .claude — отдельные репозитории.
  step 'gofmt' --empty-out go_fmt
  # Пакеты — `. ./internal/...`, не `./...`: тот заходит в frontend/node_modules.
  step 'go vet' go vet . ./internal/...
  # e2e-тест L1 (тег e2ebig) иначе не собирает никто. -tags в команде заменяет теги из GOFLAGS — webkit2_41 повторяем.
  step 'go vet e2ebig' go vet -tags "e2ebig${wails_tags:+,$wails_tags}" ./internal/app
  if [ "$(go env GOOS)" != windows ]; then
    # Файлы *_windows.go (пути, диски, проводник) на Linux не компилируются вовсе — проверяем их сборку под Windows.
    step 'go vet windows' env GOOS=windows GOFLAGS= go vet . ./internal/...
  fi
  step 'golangci-lint' gcl_run
  if [ $race = 1 ]; then
    step 'go test -race' env CGO_ENABLED=1 node scripts/heavy.mjs node scripts/test-baseline.mjs --run-go --race --out "$go_json"
    # Уязвимости зависимостей и stdlib; govulncheck нет — пропуск с подсказкой (как в .ps1).
    if command -v govulncheck >/dev/null 2>&1; then
      step 'govulncheck' govulncheck . ./internal/...
    else
      echo "${yellow}govulncheck не найден (go install golang.org/x/vuln/cmd/govulncheck@v1.8.0) - шаг пропущен${off}"
    fi
  else
    step 'go test' node scripts/heavy.mjs node scripts/test-baseline.mjs --run-go --out "$go_json"
  fi
fi
if [ $run_ui = 1 ]; then
  fe="$root/frontend"
  step 'версия node' node_version
  step 'prettier' --dir "$fe" npx prettier --check . --log-level warn
  step 'eslint' --dir "$fe" npx eslint . --max-warnings 0
  step 'svelte-check' --dir "$fe" npx svelte-check --output human --fail-on-warnings
  step 'vitest' --no-warn --dir "$fe" npm test -- --reporter=default --reporter=json "--outputFile.json=$ui_json"
  step 'npm build' --no-warn --dir "$fe" npm run build
  step 'контракт биндингов' node scripts/bindings-contract.mjs --check
fi

# Эталон силы тестов: число тестов и покрытие не падают (testdata/test-baseline.json, обновляется только --write).
baseline=(scripts/test-baseline.mjs --check)
[ $run_go = 1 ] && [ $run_ui = 0 ] && baseline+=(--go)
[ $run_ui = 1 ] && [ $run_go = 0 ] && baseline+=(--ui)
[ $run_go = 1 ] && [ -f "$go_json" ] && baseline+=(--go-json "$go_json")
[ $run_ui = 1 ] && [ -f "$ui_json" ] && baseline+=(--ui-json "$ui_json")
step 'test baseline' node scripts/heavy.mjs node "${baseline[@]}"

step 'размер файлов' --empty-out check_sizes
step 'folder limit' --empty-out check_folders
# Комментарии по-русски (CLAUDE.md «Правила»): новые английские фразы в диффе ветки к dev — провал.
step 'язык комментариев' node scripts/comment-lang.mjs

t=$(awk -v a="$total_start" -v b="$(now)" 'BEGIN { printf "%.1fs", b - a }')
if [ ${#failed[@]} -gt 0 ]; then
  echo "${red}ИТОГО: упало - $(IFS=,; echo "${failed[*]}" | sed 's/,/, /g') ($t)${off}"
  exit 1
fi
echo "${green}ИТОГО: всё чисто ($t)${off}"
exit 0
