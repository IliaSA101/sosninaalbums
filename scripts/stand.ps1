# Стенд QA — своя копия wails dev рядом с основной (34115): powershell -ExecutionPolicy Bypass -File scripts/stand.ps1 -Name b -Port 34116
# Запускать из корня рабочей копии (ветка до слияния); одна рабочая копия — один стенд (wails dev
# пересобирает exe и биндинги в своей папке). SA_STAND даёт программе свой замок второго экземпляра
# и свои папки настроек, WebView2, кэша миниатюр и копий (internal/osx DataName). Vite берёт свободный
# порт сам (5173, 5174, …), wails dev находит его по выводу (frontend:dev:serverUrl "auto").
# Работает до остановки wails dev; вывод — в .gotmp/wails-dev-<имя>.log. Работает в Windows PowerShell 5.1 и pwsh 7.
param(
    [Parameter(Mandatory = $true)][string]$Name, # латиница и цифры, до 16 знаков (как проверяет Go)
    [int]$Port = 34116                           # порт wails dev: QA — http://localhost:<порт>
)

$ErrorActionPreference = 'Stop'
$root = (Get-Location).Path

# Имя входит в имена папок данных: то же правило, что osx.ParseStand, — отказ раньше сборки.
if ($Name -notmatch '^[A-Za-z0-9]{1,16}$') {
    throw "Имя стенда «$Name»: только латиница и цифры, до 16 знаков"
}
$Name = $Name.ToLowerInvariant()
if ($Port -lt 1024 -or $Port -gt 65535) { throw "Порт $Port вне 1024–65535" }
if ($Port -eq 34115) { throw 'Порт 34115 — у основного стенда, возьмите 34116 и выше' }
if (-not (Test-Path (Join-Path $root 'wails.json'))) { throw "Нет wails.json в $root — запускать из корня рабочей копии" }
if (-not (Test-Path (Join-Path $root 'frontend/node_modules'))) {
    throw 'Нет frontend/node_modules: в рабочей копии агента — junction на основное дерево (см. CLAUDE.md, Worktree)'
}
if (-not (Get-Command wails -ErrorAction SilentlyContinue)) { throw 'wails не найден в PATH' }

# Занятый порт: wails dev упал бы позже и с невнятной ошибкой.
$probe = [System.Net.Sockets.TcpListener]::new([System.Net.IPAddress]::Loopback, $Port)
try { $probe.Start() } catch { throw "Порт $Port занят — другой стенд или программа" } finally { $probe.Stop() }

# GOTMPDIR — из-за антивируса (README): exe собираются только в .gotmp рабочей копии.
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
$env:SA_STAND = $Name
$log = Join-Path $env:GOTMPDIR "wails-dev-$Name.log"
Set-Content -LiteralPath $log -Value "стенд $Name, http://localhost:$Port, $root" -Encoding UTF8

Write-Host "Стенд $Name`: http://localhost:$Port, лог $log"
# stderr wails dev — обычные строки лога, а не ошибки скрипта; Add-Content — UTF-8 построчно
# (перенаправление > в 5.1 пишет UTF-16 и держит файл до конца).
$ErrorActionPreference = 'Continue'
& wails dev -devserver "localhost:$Port" 2>&1 | ForEach-Object { Add-Content -LiteralPath $log -Value "$_" -Encoding UTF8 }
exit $LASTEXITCODE
