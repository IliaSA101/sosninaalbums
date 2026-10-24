#!/usr/bin/env bash
# Все проверки перед сдачей одной командой — двойник scripts/check.ps1 для Linux и macOS:
#   scripts/check.sh [--go] [--ui] [--race]      (принимаются и -Go, -Ui, -Race, как у .ps1)
# Без флагов идут и Go, и фронт — параллельно, каждая часть в своём процессе (--part, служебный) со своим логом;
# логи печатаются по завершении. --race — Go-тесты с детектором гонок (gcc, cgo; в конце цикла и перед релизом) и govulncheck трёх ОС.
# ПРАВЯТСЯ ВМЕСТЕ: шаги, их имена и пороги — как в check.ps1. Отличия только платформенные: окружение — scripts/env.sh;
# на Linux лишний шаг «go vet windows» (файлы *_windows.go иначе здесь не компилирует никто, CI — только на windows-latest).
set -u

go_only=0 ui_only=0 race=0 part=''
while [ $# -gt 0 ]; do
  case "$1" in
    --go | -go | -Go) go_only=1 ;;
    --ui | -ui | -Ui) ui_only=1 ;;
    --race | -race | -Race) race=1 ;;
    # Служебный: дочерний прогон одной части из параллельного полного (go или ui) — без общих шагов и итога.
    --part) part=${2:-}; shift ;;
    *) echo "неизвестный флаг: $1 (есть --go, --ui, --race)" >&2; exit 2 ;;
  esac
  shift
done
case "$part" in
  go) go_only=1 ui_only=0 ;;
  ui) ui_only=1 go_only=0 ;;
  '') ;;
  *) echo "неизвестная часть: $part (есть go, ui)" >&2; exit 2 ;;
esac
run_go=0 run_ui=0
{ [ $go_only = 1 ] || [ $ui_only = 0 ]; } && run_go=1
{ [ $ui_only = 1 ] || [ $go_only = 0 ]; } && run_ui=1

root=$(CDPATH='' cd -- "$(dirname -- "$0")/.." && pwd)
cd "$root" || exit 1
. "$root/scripts/env.sh"
# Тег webkit2_41 нужен Go-пакетам, которые тянут окно Wails (main, internal/app): без него cgo ищет WebKitGTK 4.0.
[ -n "$wails_tags" ] && export GOFLAGS="${GOFLAGS:+$GOFLAGS }-tags=$wails_tags"

# SA_CHECK_COLOR: дочерний прогон пишет в файл, но цвет нужен — родитель сидит в терминале.
if [ -t 1 ] || [ -n "${SA_CHECK_COLOR:-}" ]; then red=$'\e[31m' green=$'\e[32m' yellow=$'\e[33m' off=$'\e[0m'; else red='' green='' yellow='' off=''; fi
failed=()
# Запуск без разделения: полный прогон (обе части, не дочерний) идёт параллельно.
parallel=0
{ [ $run_go = 1 ] && [ $run_ui = 1 ] && [ -z "$part" ]; } && parallel=1
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
# Линтер под другую ОС: CI линтует только Windows-сборку (*_windows.go иначе видны только в CI), локально — только свою.
gcl_cross() { ( export GOOS=$1 GOFLAGS=; gcl_run ); }
node_version() {
  [ -z "$node_want" ] || [ "$(node -v)" = "v$node_want" ] || { echo "node $(node -v), а в CI v$node_want: nvm install $node_want"; return 1; }
}

# Json-вывод go test и vitest: по нему test-baseline сверяет имена тестов и покрытие без второго прогона.
go_json="$GOTMPDIR/go-test.json"
ui_json="$GOTMPDIR/vitest.json"
# Только свои файлы: дочерние прогоны частей идут одновременно и чужой json не трогают.
[ $run_go = 1 ] && rm -f "$go_json"
[ $run_ui = 1 ] && rm -f "$ui_json"

# Полный прогон: Go и фронт — два дочерних процесса (как scripts/precheck.mjs), вывод каждого — в свой лог, печать по завершении.
# Общие шаги (размер, папки, язык) — после; код возврата общий.
if [ $parallel = 1 ]; then
  [ -t 1 ] && export SA_CHECK_COLOR=1
  extra=()
  [ $race = 1 ] && extra+=(--race)
  logs=()
  pids=()
  for p in go ui; do
    logs+=("$GOTMPDIR/check-$p.log")
    SA_CHECK_FAILED="$GOTMPDIR/check-$p.failed" "$root/scripts/check.sh" --part "$p" ${extra[@]+"${extra[@]}"} >"$GOTMPDIR/check-$p.log" 2>&1 &
    pids+=($!)
  done
  trap 'kill "${pids[@]}" 2>/dev/null; exit 130' INT TERM
  codes=()
  for i in 0 1; do wait "${pids[$i]}"; codes+=($?); done
  trap - INT TERM
  for i in 0 1; do
    cat "${logs[$i]}"
    p=$([ "$i" = 0 ] && echo go || echo ui)
    if [ -s "$GOTMPDIR/check-$p.failed" ]; then
      while IFS= read -r n; do failed+=("$n"); done <"$GOTMPDIR/check-$p.failed"
    elif [ "${codes[$i]}" != 0 ]; then
      failed+=("$p (аварийно)")
    fi
  done
  run_go=0 run_ui=0
fi

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
  # Workflow: синтаксис, выражения и run: через shellcheck, как задание workflows в ci.yml (0.25.0: SC2015, SC2086 нашлись только в CI).
  # Нет инструмента — жёлтая строка, не провал. actionlint ставить не нужно: go run ...@версия из ci.yml.
  if [ -z "$actionlint_want" ]; then
    echo "${yellow}версия actionlint не найдена в ci.yml - шаг actionlint пропущен${off}"
  elif command -v shellcheck >/dev/null 2>&1; then
    step 'actionlint' env GOFLAGS= go run "github.com/rhysd/actionlint/cmd/actionlint@$actionlint_want"
  else
    step 'actionlint (без shellcheck)' env GOFLAGS= go run "github.com/rhysd/actionlint/cmd/actionlint@$actionlint_want" -shellcheck=
    echo "${yellow}shellcheck не найден (sudo apt install shellcheck; в CI 0.9.0) - run: в workflow не проверены${off}"
  fi
  step 'golangci-lint' gcl_run
  [ "$(go env GOOS)" != windows ] && step 'golangci-lint windows' gcl_cross windows
  if [ $race = 1 ]; then
    step 'go test -race' env CGO_ENABLED=1 node scripts/heavy.mjs node scripts/test-baseline.mjs --run-go --race --out "$go_json"
    # Уязвимости зависимостей и stdlib — по трём ОС, как в CI (анализ учитывает теги сборки); govulncheck нет — пропуск с подсказкой (как в .ps1).
    if command -v govulncheck >/dev/null 2>&1; then
      for os in windows darwin linux; do step "govulncheck $os" env GOOS=$os GOFLAGS= govulncheck . ./internal/...; done
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
  # Кэши prettier и eslint — в .gotmp (env.sh), не в дереве.
  step 'prettier' --dir "$fe" npx prettier --check . --cache --cache-location "$prettier_cache" --log-level warn
  step 'eslint' --dir "$fe" npx eslint . --max-warnings 0 --cache --cache-location "$eslint_cache/"
  step 'svelte-check' --dir "$fe" npx svelte-check --output human --fail-on-warnings
  step 'vitest' --no-warn --dir "$fe" npm test -- --reporter=default --reporter=json "--outputFile.json=$ui_json"
  step 'npm build' --no-warn --dir "$fe" npm run build
  step 'контракт биндингов' node scripts/bindings-contract.mjs --check
  # Тесты скриптов — на публичном дереве (без publish-exclude), как их видит CI main: в 0.25.0 test_qakit ждал docs/qa, которого там нет.
  step 'тесты скриптов' node scripts/script-tests.mjs --public
fi

# Эталон силы тестов: число тестов и покрытие не падают (testdata/test-baseline.json, обновляется только --write).
# В параллельном полном прогоне (родитель) обе части уже сверены своими дочерними процессами.
if [ $parallel = 0 ]; then
  baseline=(scripts/test-baseline.mjs --check)
  [ $run_go = 1 ] && [ $run_ui = 0 ] && baseline+=(--go)
  [ $run_ui = 1 ] && [ $run_go = 0 ] && baseline+=(--ui)
  [ $run_go = 1 ] && [ -f "$go_json" ] && baseline+=(--go-json "$go_json")
  [ $run_ui = 1 ] && [ -f "$ui_json" ] && baseline+=(--ui-json "$ui_json")
  step 'test baseline' node scripts/heavy.mjs node "${baseline[@]}"
fi

# Дочерний прогон части: общие шаги и итог — в родителе; имена упавших шагов — ему через файл.
if [ -n "$part" ]; then
  : >"${SA_CHECK_FAILED:-/dev/null}"
  [ ${#failed[@]} -gt 0 ] && printf '%s\n' "${failed[@]}" >"${SA_CHECK_FAILED:-/dev/null}"
  [ ${#failed[@]} -gt 0 ] && exit 1
  exit 0
fi

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
