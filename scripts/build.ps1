# Сборка выпуска: powershell -File scripts/build.ps1
# wails build -clean, затем exe переименовывается в «SosninaAlbums-<версия>.exe» — версия из wails.json (productVersion),
# ПРАВЯТСЯ ВМЕСТЕ с двойником scripts/build.sh (Linux, macOS; там же exe для Windows кросс-сборкой).
# чтобы у Марины и в GitHub Release было видно, какая это сборка. Работает в Windows PowerShell 5.1 и pwsh 7.
$ErrorActionPreference = 'Stop'
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

$ver = (Get-Content (Join-Path $root 'wails.json') -Raw -Encoding UTF8 | ConvertFrom-Json).info.productVersion
if (-not $ver) { throw 'нет info.productVersion в wails.json' }

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
