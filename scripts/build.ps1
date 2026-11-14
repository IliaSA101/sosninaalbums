# Сборка выпуска: powershell -File scripts/build.ps1
# wails build -clean, затем exe переименовывается в «SosninaAlbums-<версия>.exe» — версия из wails.json (productVersion),
# ПРАВЯТСЯ ВМЕСТЕ с двойником scripts/build.sh (Linux, macOS; там же exe для Windows кросс-сборкой).
# чтобы у Марины и в GitHub Release было видно, какая это сборка. Работает в Windows PowerShell 5.1 и pwsh 7.
# -Installer: ещё и установщик build/bin/SosninaAlbums-<версия>-windows-x64-setup.exe (нужны makensis и bash из Git for Windows;
# scripts/win/installer.sh, спека 111). Выпускной установщик собирает CI.
# -Standard (вместе с -Installer): ещё и стандартный установщик с примером набора (спека 142, installer.sh --demo ... --standard).
param([switch]$Installer, [switch]$Standard)
$ErrorActionPreference = 'Stop'
$root = Split-Path -Parent $PSScriptRoot
Set-Location $root
if ($Standard -and -not $Installer) { throw '-Standard — вместе с -Installer' }
# Zip примера набора: один на репозиторий (internal/demopack); ищем до сборки, чтобы не падать на последнем шаге
$demo = $null
if ($Standard) {
    $zips = @(Get-ChildItem -Path (Join-Path $root 'internal/demopack') -Filter '*.zip' -File)
    if ($zips.Count -ne 1) { throw "в internal/demopack ждали ровно один zip примера, найдено: $($zips.Count)" }
    $demo = $zips[0].FullName -replace '\\', '/'
}

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

$ver = (Get-Content (Join-Path $root 'wails.json') -Raw -Encoding UTF8 | ConvertFrom-Json).info.productVersion
if (-not $ver) { throw 'нет info.productVersion в wails.json' }

# CGO выключен, как в кросс-сборке build.sh и CI: иначе в exe другой build ID и пометка CGO, и независимая пересборка
# установщика (release-sign.mjs) не сойдётся побайтно
$env:CGO_ENABLED = '0'
wails build -clean -trimpath
if ($LASTEXITCODE -ne 0) { throw "wails build: код $LASTEXITCODE" }

$bin = Join-Path $root 'build/bin'
$out = Join-Path $bin "SosninaAlbums-$ver.exe"
Move-Item -Force (Join-Path $bin 'sosninaalbums.exe') $out

# Защита от утечки личных путей: -trimpath должен вычистить их из exe. Ищем байтовые строки в latin1-представлении.
# Образцы берутся из окружения сборщика, а не пишутся в скрипт: так в репозиторий не попадает ни имя, ни путь.
$text = [Text.Encoding]::GetEncoding('iso-8859-1').GetString([IO.File]::ReadAllBytes($out))
$rootFwd = $root -replace '\\', '/'
foreach ($s in @("Users\$env:USERNAME", "Users/$env:USERNAME", $root, $rootFwd)) {
    if ($text.Contains($s)) { throw "в exe найдена строка '$s' — личные данные в сборке, публиковать нельзя" }
}
Write-Host "Готово: $out"

if ($Installer) {
    $bash = Get-Command bash -ErrorAction SilentlyContinue
    if (-not $bash) { throw 'для -Installer нужен bash (Git for Windows) и makensis в PATH' }
    # bash из Git for Windows путей с обратными косыми не разбирает
    & $bash.Source ((Join-Path $root 'scripts/win/installer.sh') -replace '\\', '/') --exe ($out -replace '\\', '/') --version $ver --out ($bin -replace '\\', '/')
    if ($LASTEXITCODE -ne 0) { throw "installer.sh: код $LASTEXITCODE" }
    if ($Standard) {
        & $bash.Source ((Join-Path $root 'scripts/win/installer.sh') -replace '\\', '/') --exe ($out -replace '\\', '/') --version $ver --out ($bin -replace '\\', '/') --demo $demo --standard
        if ($LASTEXITCODE -ne 0) { throw "installer.sh --standard: код $LASTEXITCODE" }
    }
}
