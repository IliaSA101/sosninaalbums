# Все проверки перед сдачей одной командой: powershell -File scripts/check.ps1 [-Go] [-Ui] [-Race]
# Без флагов идут и Go, и фронт. -Race — Go-тесты с детектором гонок (нужен gcc, cgo; в конце цикла и перед релизом). Работает в Windows PowerShell 5.1 и pwsh 7.
param(
    [switch]$Go, # только Go-шаги
    [switch]$Ui,  # только фронт
    [switch]$Race # go test -race вместо обычного go test
)

$ErrorActionPreference = 'Continue'
$root = Split-Path -Parent $PSScriptRoot
Set-Location $root

# GOTMPDIR нужен из-за антивируса, см. README
$env:GOTMPDIR = Join-Path $root '.gotmp'
if (-not (Test-Path $env:GOTMPDIR)) { New-Item -ItemType Directory -Path $env:GOTMPDIR | Out-Null }

$runGo = $Go -or -not $Ui
$runUi = $Ui -or -not $Go
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
Remove-Item -LiteralPath $goJson, $uiJson -ErrorAction SilentlyContinue

if ($runGo) {
    # Только файлы из git (как в CheckSizes): .gotmp в .gitignore, worktree агентов в .claude — отдельные репозитории.
    # Иначе недописанный файл агента роняет gofmt кодом выхода, а фильтр вывода этого не спасает.
    Step 'gofmt' { gofmt -l (git ls-files --cached --others --exclude-standard -- '*.go') } -EmptyOut
    # Пакеты — `. ./internal/...`, не `./...`: тот заходит в frontend/node_modules (flatted/golang)
    Step 'go vet' { go vet . ./internal/... }
    # golangci-lint ставится через go install — лежит в GOPATH\bin, которого может не быть в PATH
    $gcl = Get-Command golangci-lint -ErrorAction SilentlyContinue
    $gclPath = if ($gcl) { $gcl.Source } else { Join-Path (go env GOPATH) 'bin\golangci-lint.exe' }
    # Проверки веток идут параллельно в фоне (решение владельца 2026-10-09): у каждой копии свой .gotmp и json-вывод,
    # общий только замок golangci-lint — без флага второй прогон отказывается работать.
    Step 'golangci-lint' { node scripts/heavy.mjs $gclPath run --allow-parallel-runners . ./internal/... }
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
            Step 'govulncheck' { & $gvcPath . ./internal/... }
        } else {
            Write-Host 'govulncheck не найден (go install golang.org/x/vuln/cmd/govulncheck@latest) - шаг пропущен' -ForegroundColor Yellow
        }
    } else {
        Step 'go test' { node scripts/heavy.mjs node scripts/test-baseline.mjs --run-go --out $goJson }
    }
}
if ($runUi) {
    $fe = Join-Path $root 'frontend'
    Step 'prettier' { npx prettier --check . --log-level warn } $fe
    Step 'eslint' { npx eslint . --max-warnings 0 } $fe
    Step 'svelte-check' { npx svelte-check --output human --fail-on-warnings } $fe
    Step 'vitest' { npm test -- --reporter=default --reporter=json "--outputFile.json=$uiJson" } $fe -NoWarn
    Step 'npm build' { npm run build } $fe -NoWarn
    Step 'контракт биндингов' { node scripts/bindings-contract.mjs --check } $root
}

# Эталон силы тестов: число тестов и покрытие не падают (testdata/test-baseline.json, обновляется только --write)
Step 'test baseline' {
    $a = @('scripts/test-baseline.mjs', '--check')
    if ($runGo -and -not $runUi) { $a += '--go' }
    if ($runUi -and -not $runGo) { $a += '--ui' }
    if ($runGo -and (Test-Path -LiteralPath $goJson)) { $a += @('--go-json', $goJson) }
    if ($runUi -and (Test-Path -LiteralPath $uiJson)) { $a += @('--ui-json', $uiJson) }
    # heavy.mjs: тяжёлые проверки — не больше SA_HEAVY_SLOTS одновременно на машину (очередь, scripts/heavy.mjs)
    node scripts/heavy.mjs node @a
}

Step 'размер файлов' { CheckSizes } -EmptyOut
Step 'folder limit' { CheckFolders } -EmptyOut

$time = '{0:N1}s' -f $total.Elapsed.TotalSeconds
if ($failed.Count -gt 0) {
    Write-Host "ИТОГО: упало - $($failed -join ', ') ($time)" -ForegroundColor Red
    exit 1
}
Write-Host "ИТОГО: всё чисто ($time)" -ForegroundColor Green
exit 0
