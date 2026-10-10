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

# Размер файлов: больше 600 строк или 30 КБ — резать по смыслу (см. docs/BACKLOG.md).
# Список из git (учитывает .gitignore: node_modules, dist, .gotmp, build/bin не попадают).
function CheckSizes {
    $skip = '(^|/)(wailsjs|build|docs/private|docs/refs)/|\.d\.ts$'
    git ls-files --cached --others --exclude-standard |
        Where-Object { $_ -match '\.(go|ts|js|svelte|css|ps1|py)$' -and $_ -notmatch $skip -and (Test-Path -LiteralPath $_) } |
        ForEach-Object {
            $f = Get-Item -LiteralPath $_
            $lines = @(Get-Content -LiteralPath $f.FullName).Count
            if ($lines -gt 600 -or $f.Length -gt 30KB) { "$_ ($lines строк, $([math]::Round($f.Length / 1KB, 1)) КБ)" }
        }
    # Пустой вывод — всё чисто; иначе провал с именами файлов.
    $global:LASTEXITCODE = 0
}

# Порог папок (022): не больше 20 файлов .go/.ts/.svelte (с тестами) на папку Go и на область frontend/src.
# Исключения - scripts/folder-exceptions.txt (строки "папка потолок # причина").
function CheckFolders {
    # Порог по уровням (022, docs/specs/0.8/022-structure/design.md §4): файлов кода прямо в папке ≤ 20 (ключ — папка
    # файла, для Go и фронта одинаково); подпапок у любой папки ≤ 20 (по всем путям репозитория, docs тоже — решение
    # владельца 2026-10-09); код фронта — только frontend/src/<файл> или frontend/src/<группа>/<область>/<файл>.
    # Исключения — scripts/folder-exceptions.txt: «<папка> N» — файлы, «<папка>/ N» — подпапки.
    $skip = '(^|/)(wailsjs|testdata|node_modules)/|\.d\.ts$'
    $maxFiles = @{}
    $maxDirs = @{}
    $exc = Join-Path $root 'scripts/folder-exceptions.txt'
    if (Test-Path -LiteralPath $exc) {
        foreach ($line in Get-Content -LiteralPath $exc) {
            if ($line -match '^\s*([^\s#]+)/\s+(\d+)') { $maxDirs[$Matches[1]] = [int]$Matches[2] }
            elseif ($line -match '^\s*([^\s#]+)\s+(\d+)') { $maxFiles[$Matches[1]] = [int]$Matches[2] }
        }
    }
    $all = @(git ls-files --cached --others --exclude-standard | Where-Object { Test-Path -LiteralPath $_ })
    $code = @($all | Where-Object { $_ -match '\.(go|ts|svelte)$' -and $_ -notmatch $skip })

    $code | ForEach-Object { if ($_.Contains('/')) { $_.Substring(0, $_.LastIndexOf('/')) } else { '.' } } |
        Group-Object |
        ForEach-Object {
            $lim = 20
            if ($maxFiles.ContainsKey($_.Name)) { $lim = $maxFiles[$_.Name] }
            if ($_.Count -gt $lim) { "$($_.Name): $($_.Count) files (max $lim)" }
        }

    $kids = @{}
    foreach ($f in $all) {
        $parts = $f.Split('/')
        $parent = '.'
        for ($i = 0; $i -lt $parts.Length - 1; $i++) {
            if (-not $kids.ContainsKey($parent)) { $kids[$parent] = @{} }
            $kids[$parent][$parts[$i]] = 1
            $parent = if ($i -eq 0) { $parts[0] } else { "$parent/$($parts[$i])" }
        }
    }
    foreach ($p in ($kids.Keys | Sort-Object)) {
        $lim = 20
        if ($maxDirs.ContainsKey($p)) { $lim = $maxDirs[$p] }
        if ($kids[$p].Count -gt $lim) { "${p}: $($kids[$p].Count) folders (max $lim)" }
    }

    $code | Where-Object { $_ -match '^frontend/src/' -and $_.Split('/').Length -notin 3, 5 } |
        ForEach-Object { $_.Substring(0, $_.LastIndexOf('/')) } |
        Sort-Object -Unique |
        ForEach-Object { "${_}: code outside frontend/src/<group>/<area>" }
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
    Step 'go vet' { go vet ./... }
    # golangci-lint ставится через go install — лежит в GOPATH\bin, которого может не быть в PATH
    $gcl = Get-Command golangci-lint -ErrorAction SilentlyContinue
    $gclPath = if ($gcl) { $gcl.Source } else { Join-Path (go env GOPATH) 'bin\golangci-lint.exe' }
    # Проверки веток идут параллельно в фоне (решение владельца 2026-10-09): у каждой копии свой .gotmp и json-вывод,
    # общий только замок golangci-lint — без флага второй прогон отказывается работать.
    Step 'golangci-lint' { node scripts/heavy.mjs $gclPath run --allow-parallel-runners ./... }
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
            Step 'govulncheck' { & $gvcPath ./... }
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
