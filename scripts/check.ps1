# Все проверки перед сдачей одной командой: powershell -File scripts/check.ps1 [-Go] [-Ui] [-Race]
# ПРАВЯТСЯ ВМЕСТЕ с двойником scripts/check.sh (Linux, macOS): те же шаги, имена и пороги.
# Без флагов идут и Go, и фронт — параллельно, каждая часть в своём процессе (-Part, служебный) со своим логом;
# логи печатаются по завершении. -Race — Go-тесты с детектором гонок (нужен gcc, cgo; в конце цикла и перед релизом) и govulncheck трёх ОС.
# Работает в Windows PowerShell 5.1 и pwsh 7.
param(
    [switch]$Go, # только Go-шаги
    [switch]$Ui,  # только фронт
    [switch]$Race, # go test -race вместо обычного go test
    [string]$Part = '', # служебный: дочерний прогон одной части из параллельного полного (go или ui) — без общих шагов и итога
    [string]$FailedFile = '' # служебный: куда дочерний прогон пишет имена упавших шагов
)

$ErrorActionPreference = 'Continue'
# Дочерний прогон пишет в файл: кириллицу - в UTF-8 без BOM (родитель читает лог как UTF-8)
if ($Part) { [Console]::OutputEncoding = New-Object System.Text.UTF8Encoding $false }
$root = Split-Path -Parent $PSScriptRoot
Set-Location $root

# GOTMPDIR нужен из-за антивируса, см. README
$env:GOTMPDIR = Join-Path $root '.gotmp'
if (-not (Test-Path $env:GOTMPDIR)) { New-Item -ItemType Directory -Path $env:GOTMPDIR | Out-Null }
# Кэши Go и golangci-lint — в .gotmp основной копии (общие для worktree): пользовательская GOCACHE уводила их мимо проекта (20.10)
$prevEap = $ErrorActionPreference; $ErrorActionPreference = 'Continue'
$common = git -C $root rev-parse --path-format=absolute --git-common-dir 2>$null
$gitOk = ($LASTEXITCODE -eq 0)
$ErrorActionPreference = $prevEap
$main = if ($gitOk -and $common) { Split-Path -Parent $common } else { $root }
$env:GOCACHE = Join-Path (Join-Path $main '.gotmp') 'cache-go'
$env:GOLANGCI_LINT_CACHE = Join-Path (Join-Path $main '.gotmp') 'cache-golangci'

$runGo = $Go -or -not $Ui
$runUi = $Ui -or -not $Go
if ($Part -eq 'go') { $runGo = $true; $runUi = $false }
elseif ($Part -eq 'ui') { $runGo = $false; $runUi = $true }
elseif ($Part) { Write-Host "неизвестная часть: $Part (есть go, ui)" -ForegroundColor Red; exit 2 }
# Полный прогон (обе части, не дочерний) идёт параллельно
$parallel = $runGo -and $runUi -and -not $Part
$failed = New-Object System.Collections.Generic.List[string]
$total = [Diagnostics.Stopwatch]::StartNew()

# Запускает шаг, прячет вывод, печатает одну строку; при провале — хвост вывода.
# $EmptyOut: шаг провален, если вывод непустой (gofmt -l).
# $NoWarn: шаг провален, если в выводе есть предупреждение (правило проекта: ноль warnings).
function Step([string]$Name, [scriptblock]$Cmd, [string]$Dir = $root, [switch]$EmptyOut, [switch]$NoWarn) {
    $sw = [Diagnostics.Stopwatch]::StartNew()
    Push-Location $Dir
    $global:LASTEXITCODE = 0
    # "$_" превращает ErrorRecord из stderr (в 5.1) в обычную строку
    $out = @(& $Cmd 2>&1 | ForEach-Object { "$_" })
    $code = $LASTEXITCODE
    Pop-Location
    $ok = ($code -eq 0)
    if ($EmptyOut -and ($out | Where-Object { $_.Trim() })) { $ok = $false }
    if ($NoWarn -and ($out | Where-Object { $_ -match '(?i)\bwarn(ing)?\b|deprecat|\(!\)' })) { $ok = $false }
    $time = '{0:N1}s' -f $sw.Elapsed.TotalSeconds
    if ($ok) {
        Write-Host "OK   $Name ($time)" -ForegroundColor Green
    } else {
        Write-Host "FAIL $Name ($time)" -ForegroundColor Red
        $out | Select-Object -Last 30 | ForEach-Object { Write-Host "     $_" }
        $failed.Add($Name)
    }
}

# Размер файлов и порог папок — scripts/limits.mjs (единственный источник пределов, 0.13.1): Go — файл кода 800 строк,
# тест 1200, счётчика файлов в папке нет; фронт — .ts 600 строк / 30 КБ, .svelte 600, тест 900, область ≤ 20 файлов
# кода без тестов; подпапок ≤ 20. Пустой вывод — чисто; иначе провал со списком.
function CheckSizes {
    node scripts/limits.mjs sizes
    $global:LASTEXITCODE = 0
}

function CheckFolders {
    node scripts/limits.mjs folders
    $global:LASTEXITCODE = 0
}

# Json-вывод go test и vitest: по нему test-baseline сверяет имена тестов (с подтестами) и покрытие без второго прогона.
$goJson = Join-Path $env:GOTMPDIR 'go-test.json'
$uiJson = Join-Path $env:GOTMPDIR 'vitest.json'
# Только свои файлы: дочерние прогоны частей идут одновременно и чужой json не трогают.
if ($runGo) { Remove-Item -LiteralPath $goJson -ErrorAction SilentlyContinue }
if ($runGo -and (Test-Path (Join-Path $root 'api/albums/go.mod'))) {
    # 146 (да владельца 2026-10-29): модуль контракта api/albums — свой go.mod; корневые vet, test и lint его не видят.
    $api = Join-Path $root 'api/albums'
    Step 'api/albums: go vet' { go vet ./... } -Dir $api
    Step 'api/albums: golangci-lint' { node (Join-Path $root 'scripts/heavy.mjs') $gclPath run --allow-parallel-runners ./... } -Dir $api
    if ($Race) { Step 'api/albums: go test -race' { go test -race ./... } -Dir $api } else { Step 'api/albums: go test' { go test ./... } -Dir $api }
}
if ($runUi) { Remove-Item -LiteralPath $uiJson -ErrorAction SilentlyContinue }

# Версия actionlint — из задания workflows в ci.yml (go run ...@версия, ставить не нужно).
$alWant = ''
$ciYml = Join-Path $root '.github\workflows\ci.yml'
if (Test-Path -LiteralPath $ciYml) {
    $m = [regex]::Match((Get-Content -Raw -LiteralPath $ciYml), 'rhysd/actionlint/cmd/actionlint@(v[0-9.]+)')
    if ($m.Success) { $alWant = $m.Groups[1].Value }
}
# Кэши prettier и eslint - в .gotmp (не в дереве)
$prettierCache = Join-Path $env:GOTMPDIR 'prettier.cache'
$eslintCache = (Join-Path $env:GOTMPDIR 'cache-eslint') + '/'

# Полный прогон: Go и фронт - два дочерних процесса (как scripts/precheck.mjs), вывод каждого - в свой лог, печать по завершении.
# Общие шаги (размер, папки, язык) - после; код возврата общий.
if ($parallel) {
    $psExe = (Get-Process -Id $PID).Path
    $procs = @{}
    foreach ($p in 'go', 'ui') {
        $ff = Join-Path $env:GOTMPDIR "check-$p.failed"
        Remove-Item -LiteralPath $ff -ErrorAction SilentlyContinue
        # Пути в кавычках: Start-Process в 5.1 склеивает аргументы пробелами как есть
        $childArgs = @('-NoProfile', '-ExecutionPolicy', 'Bypass', '-File', "`"$PSCommandPath`"", '-Part', $p, '-FailedFile', "`"$ff`"")
        if ($Race) { $childArgs += '-Race' }
        $spArgs = @{
            FilePath               = $psExe
            ArgumentList           = $childArgs
            NoNewWindow            = $true
            PassThru               = $true
            RedirectStandardOutput = Join-Path $env:GOTMPDIR "check-$p.log"
            RedirectStandardError  = Join-Path $env:GOTMPDIR "check-$p.err"
        }
        $procs[$p] = Start-Process @spArgs
    }
    try {
        foreach ($p in 'go', 'ui') { $procs[$p].WaitForExit() }
    } finally {
        # Прервали родителя (Ctrl-C) - дочерние не оставляем
        foreach ($p in 'go', 'ui') { if (-not $procs[$p].HasExited) { Stop-Process -Id $procs[$p].Id -ErrorAction SilentlyContinue } }
    }
    foreach ($p in 'go', 'ui') {
        foreach ($line in (Get-Content -LiteralPath (Join-Path $env:GOTMPDIR "check-$p.log") -Encoding UTF8)) {
            if ($line -match '^OK ') { Write-Host $line -ForegroundColor Green }
            elseif ($line -match '^FAIL ') { Write-Host $line -ForegroundColor Red }
            elseif ($line -match '^govulncheck не найден|^shellcheck не найден|^версия actionlint') { Write-Host $line -ForegroundColor Yellow }
            else { Write-Host $line }
        }
        $errFile = Join-Path $env:GOTMPDIR "check-$p.err"
        if ((Test-Path -LiteralPath $errFile) -and (Get-Item -LiteralPath $errFile).Length -gt 0) {
            Get-Content -LiteralPath $errFile -Encoding UTF8 | ForEach-Object { Write-Host $_ -ForegroundColor Red }
        }
        $ff = Join-Path $env:GOTMPDIR "check-$p.failed"
        $names = @()
        if (Test-Path -LiteralPath $ff) { $names = @(Get-Content -LiteralPath $ff -Encoding UTF8 | Where-Object { $_.Trim() }) }
        if ($names.Count -gt 0) { foreach ($n in $names) { $failed.Add($n) } }
        elseif ($procs[$p].ExitCode -ne 0) { $failed.Add("$p (аварийно)") }
    }
    $runGo = $false; $runUi = $false
}

if ($runGo) {
    # Только файлы из git (как в CheckSizes): .gotmp в .gitignore, worktree агентов в .claude — отдельные репозитории.
    # Иначе недописанный файл агента роняет gofmt кодом выхода, а фильтр вывода этого не спасает.
    Step 'gofmt' { gofmt -l (git ls-files --cached --others --exclude-standard -- '*.go') } -EmptyOut
    # Пакеты — `. ./internal/...`, не `./...`: тот заходит в frontend/node_modules (flatted/golang)
    Step 'go vet' { go vet . ./internal/... }
    # e2e-тест L1 (тег e2ebig) иначе не собирает никто: при правке App он тихо перестал бы компилироваться (ревью 0.21, владелец 2026-10-20)
    Step 'go vet e2ebig' { go vet -tags e2ebig ./internal/app }
    # golangci-lint ставится через go install — лежит в GOPATH\bin, которого может не быть в PATH
    $gcl = Get-Command golangci-lint -ErrorAction SilentlyContinue
    $gclPath = if ($gcl) { $gcl.Source } else { Join-Path (go env GOPATH) 'bin\golangci-lint.exe' }
    # Проверки веток идут параллельно в фоне (решение владельца 2026-10-09): у каждой копии свой .gotmp и json-вывод,
    # общий только замок golangci-lint — без флага второй прогон отказывается работать.
    Step 'golangci-lint' { node scripts/heavy.mjs $gclPath run --allow-parallel-runners . ./internal/... }
    # CI линтует только Windows-сборку; Linux-сборку здесь (на Windows) не видит никто, кроме Ubuntu-машины. cgo при кросс-сборке выключен.
    $env:GOOS = 'linux'; $env:CGO_ENABLED = '0'
    Step 'golangci-lint linux' { node scripts/heavy.mjs $gclPath run --allow-parallel-runners . ./internal/... }
    $env:GOOS = $null; $env:CGO_ENABLED = $null
    # Workflow: синтаксис, выражения и run: через shellcheck, как задание workflows в ci.yml (0.25.0: SC2015, SC2086 нашлись только в CI).
    # Нет инструмента - жёлтая строка, не провал. actionlint ставить не нужно: go run ...@версия из ci.yml.
    if (-not $alWant) {
        Write-Host 'версия actionlint не найдена в ci.yml - шаг actionlint пропущен' -ForegroundColor Yellow
    } elseif (Get-Command shellcheck -ErrorAction SilentlyContinue) {
        Step 'actionlint' { go run "github.com/rhysd/actionlint/cmd/actionlint@$alWant" }
    } else {
        Step 'actionlint (без shellcheck)' { go run "github.com/rhysd/actionlint/cmd/actionlint@$alWant" '-shellcheck=' }
        Write-Host 'shellcheck не найден (winget install koalaman.shellcheck; в CI 0.9.0) - run: в workflow не проверены' -ForegroundColor Yellow
    }
    if ($Race) {
        # gcc от winget (WinLibs) может не попасть в PATH старого окна — ищем сами
        if (-not (Get-Command gcc -ErrorAction SilentlyContinue)) {
            $gcc = Get-ChildItem "$env:LOCALAPPDATA\Microsoft\WinGet\Packages\BrechtSanders.WinLibs*\mingw64\bin" -ErrorAction SilentlyContinue | Select-Object -First 1
            if ($gcc) { $env:PATH = "$($gcc.FullName);$env:PATH" }
        }
        $env:CGO_ENABLED = '1'
        Step 'go test -race' { node scripts/heavy.mjs node scripts/test-baseline.mjs --run-go --race --out $goJson }
        $env:CGO_ENABLED = $null
        # Уязвимости зависимостей и stdlib: govulncheck лежит в GOPATH/bin; нет - пропуск
        $gvc = Get-Command govulncheck -ErrorAction SilentlyContinue
        $gvcPath = if ($gvc) { $gvc.Source } else { Join-Path (go env GOPATH) 'bin\govulncheck.exe' }
        if (Test-Path $gvcPath) {
            # По трём ОС, как в CI (анализ учитывает теги сборки)
            foreach ($os in 'windows', 'darwin', 'linux') {
                $env:GOOS = $os
                Step "govulncheck $os" { & $gvcPath . ./internal/... }
            }
            $env:GOOS = $null
        } else {
            Write-Host 'govulncheck не найден (go install golang.org/x/vuln/cmd/govulncheck@latest) - шаг пропущен' -ForegroundColor Yellow
        }
    } else {
        Step 'go test' { node scripts/heavy.mjs node scripts/test-baseline.mjs --run-go --out $goJson }
    }
}
if ($runUi) {
    $fe = Join-Path $root 'frontend'
    Step 'prettier' { npx prettier --check . --cache --cache-location $prettierCache --log-level warn } $fe
    Step 'eslint' { npx eslint . --max-warnings 0 --cache --cache-location $eslintCache } $fe
    Step 'svelte-check' { npx svelte-check --output human --fail-on-warnings } $fe
    Step 'vitest' { npm test -- --reporter=default --reporter=json "--outputFile.json=$uiJson" } $fe -NoWarn
    Step 'npm build' { npm run build } $fe -NoWarn
    Step 'контракт биндингов' { node scripts/bindings-contract.mjs --check } $root
    # Тесты скриптов - на публичном дереве (без publish-exclude), как их видит CI main: в 0.25.0 test_qakit ждал docs/qa, которого там нет.
    Step 'тесты скриптов' { node scripts/script-tests.mjs --public } $root
}

# Эталон силы тестов: число тестов и покрытие не падают (testdata/test-baseline.json, обновляется только --write).
# В параллельном полном прогоне (родитель) обе части уже сверены своими дочерними процессами.
if (-not $parallel) {
    Step 'test baseline' {
        $a = @('scripts/test-baseline.mjs', '--check')
        if ($runGo -and -not $runUi) { $a += '--go' }
        if ($runUi -and -not $runGo) { $a += '--ui' }
        if ($runGo -and (Test-Path -LiteralPath $goJson)) { $a += @('--go-json', $goJson) }
        if ($runUi -and (Test-Path -LiteralPath $uiJson)) { $a += @('--ui-json', $uiJson) }
        # heavy.mjs: тяжёлые проверки — не больше SA_HEAVY_SLOTS одновременно на машину (очередь, scripts/heavy.mjs)
        node scripts/heavy.mjs node @a
    }
}

# Дочерний прогон части: общие шаги и итог - в родителе; имена упавших шагов - ему через файл.
if ($Part) {
    if ($FailedFile -and $failed.Count -gt 0) { Set-Content -LiteralPath $FailedFile -Value $failed.ToArray() -Encoding UTF8 }
    if ($failed.Count -gt 0) { exit 1 }
    exit 0
}

Step 'размер файлов' { CheckSizes } -EmptyOut
Step 'folder limit' { CheckFolders } -EmptyOut
# Комментарии по-русски (CLAUDE.md «Правила»): новые английские фразы в диффе ветки к dev — провал (да владельца 17.10)
Step 'язык комментариев' { node scripts/comment-lang.mjs }

$time = '{0:N1}s' -f $total.Elapsed.TotalSeconds
if ($failed.Count -gt 0) {
    Write-Host "ИТОГО: упало - $($failed -join ', ') ($time)" -ForegroundColor Red
    exit 1
}
Write-Host "ИТОГО: всё чисто ($time)" -ForegroundColor Green
exit 0
