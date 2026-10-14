# test_winzip.ps1 - проверки winzip.psm1 и связки с shots.ps1 / win-shots.yml без Pester (спека 039, design §4).
# Запуск: powershell -NoProfile -ExecutionPolicy Bypass -File scripts/win/test_winzip.ps1  (Windows PowerShell 5.1).
# Код возврата: 0 - всё прошло, 1 - есть провалы. Рабочего стола не трогает (окна Проводника - только на раннере).
# Отдельный файл, а не раздел test_winlib.ps1: тот упёрся в 30 КБ. Разбор, BOM и размер самих файлов проверяет test_winlib.ps1.
# Файл в UTF-8 с BOM.
$ErrorActionPreference = 'Stop'
$here = Split-Path -Parent $MyInvocation.MyCommand.Path
$root = Split-Path -Parent (Split-Path -Parent $here)
Import-Module (Join-Path $here 'winlib.psm1') -Force -DisableNameChecking
Import-Module (Join-Path $here 'winzip.psm1') -Force -DisableNameChecking
Add-Type -AssemblyName System.IO.Compression.FileSystem

$script:Fail = 0
$script:Pass = 0
# Cond - строго [bool]: массив или строка - провал, а не молчаливое «истина»
function Check([string]$Name, $Cond, [string]$Detail = '') {
    if ($Cond -is [bool] -and $Cond) { $script:Pass++; Write-Host "ok    $Name" }
    elseif ($Cond -is [bool]) { $script:Fail++; Write-Host "FAIL  $Name  $Detail" }
    else { $script:Fail++; Write-Host "FAIL  $Name  условие не bool ($($Cond.GetType().Name))" }
}
function Same([string]$Name, $Got, $Want) { Check $Name ("$Got" -ceq "$Want") "получено '$Got', ожидалось '$Want'" }
function Section([string]$Title) { Write-Host "-- $Title" }

$tmp = Join-Path ([IO.Path]::GetTempPath()) ('sa-winzip-' + [Guid]::NewGuid().ToString('N'))
New-Item -ItemType Directory -Force -Path $tmp | Out-Null

try {
    Section 'Выбор файлов выпуска'
    $v = 'SosninaAlbums-0.11.0-windows-x64'
    Same 'выпуск с архивом: zip раньше exe' (Select-ReleaseAsset @("$v.exe", "$v.zip", 'SosninaAlbums-0.11.0-macos-universal.zip')).Kind 'zip'
    Same 'выпуск 0.10: только exe' (Select-ReleaseAsset @("$v.exe", 'THIRD_PARTY_NOTICES.txt')).Kind 'exe'
    Same 'два архива: первый' (Select-ReleaseAsset @("$v.zip", 'SosninaAlbums-0.10.0-windows-x64.zip')).Name "$v.zip"
    Same 'только Mac и чужие имена: ничего' (Select-ReleaseAsset @('SosninaAlbums-0.11.0-macos-universal.zip', 'evil-windows-x64.zip', 'SosninaAlbums-1.2-windows-x64.zip')).Name ''
    Same 'пустой список: ничего' (Select-ReleaseAsset @()).Kind ''
    $g = 'SosninaAlbums-0.10.0-install-guide'
    Same 'инструкция: pdf своего варианта раньше docx и общей' (Select-GuideAsset @("$g-ru.pdf", "$g-windows-ru.docx", "$g-windows-ru.pdf")) "$g-windows-ru.pdf"
    Same 'инструкция: нет pdf - docx своего варианта' (Select-GuideAsset @("$g-ru.pdf", "$g-windows-ru.docx")) "$g-windows-ru.docx"
    Same 'инструкция: нет своей - общая' (Select-GuideAsset @("$g-ru.docx", "$g-ru.pdf")) "$g-ru.pdf"
    Same 'инструкция: вариант macos не берётся, пустой список - пусто' ((Select-GuideAsset @('SosninaAlbums-0.11.0-install-guide-macos-ru.pdf')) + (Select-GuideAsset @())) ''

    Section 'Раскладка и подписи'
    $lay = Get-ZipLayout "C:\Users\x\Downloads\$v.zip"
    Same 'раскладка: папка распаковки, папка выпуска, версия' "$($lay.Folder)|$($lay.Inner)|$($lay.Version)|$($lay.Ok)" "$v|Sosnina Albums 0.11.0|0.11.0|True"
    $odd = Get-ZipLayout 'my.zip'
    Same 'раскладка: нестандартное имя' "$($odd.Folder)|$($odd.Inner)|$($odd.Ok)" 'my||False'
    Same 'подпись обрезается до 80 знаков, короткая не меняется' ((Format-Caption ('я' * 90)).Length + (Format-Caption 'ок').Length) 82

    Section 'Метка Zone.Identifier'
    $zr = Join-Path $tmp 'zone'
    New-Item -ItemType Directory -Force -Path (Join-Path $zr 'a\b') | Out-Null
    $f1 = Join-Path $zr 'one.txt'; $f2 = Join-Path $zr 'a\b\two.txt'
    Set-Content -LiteralPath $f1 -Value 'x'; Set-Content -LiteralPath $f2 -Value 'y'
    $zt = Test-ZoneTree $zr
    Check 'без меток дерево не годно, оба файла в списке' ((-not $zt.Ok) -and $zt.Total -eq 2 -and $zt.Missing.Count -eq 2)
    Set-Zone $f1 'https://example.test/'
    $zt = Test-ZoneTree $zr
    Check 'один файл без метки - он в списке (путь от корня)' ((-not $zt.Ok) -and $zt.Missing.Count -eq 1 -and $zt.Missing[0] -ceq 'a\b\two.txt')
    Same 'ZoneId читается' (Get-ZoneId $f1) '3'
    Check 'ZoneId: файла или метки нет - null' (($null -eq (Get-ZoneId $f2)) -and ($null -eq (Get-ZoneId (Join-Path $tmp 'none.txt'))))
    Set-Content -LiteralPath $f2 -Stream Zone.Identifier -Value "[ZoneTransfer]`r`nZoneId=2"
    Check 'ZoneId=2 не считается меткой из интернета' (-not (Test-ZoneTree $zr).Ok)
    Set-Zone $f2 'https://example.test/'
    Check 'метка у всех файлов - годно' ([bool](Test-ZoneTree $zr).Ok)
    $emp = Join-Path $tmp 'empty'
    New-Item -ItemType Directory -Force -Path $emp | Out-Null
    Check 'пустая папка и папки нет - не годно, без исключения' ((-not (Test-ZoneTree $emp).Ok) -and (-not (Test-ZoneTree (Join-Path $tmp 'none')).Ok))

    Section 'Запись архива'
    $zp = Join-Path $tmp 'e.zip'
    $za = [IO.Compression.ZipFile]::Open($zp, 'Create')
    $sw = New-Object IO.StreamWriter($za.CreateEntry('Sosnina Albums 1.0.0/SosninaAlbums.exe').Open())
    $sw.Write('12345'); $sw.Dispose(); $za.Dispose()
    Same 'размер записи архива по имени файла' (Get-ZipEntryLength $zp 'SosninaAlbums.exe') '5'
    Same 'записи или архива нет: -1' ((Get-ZipEntryLength $zp 'x.exe') + (Get-ZipEntryLength (Join-Path $tmp 'no.zip') 'a')) '-2'

    Section 'win-shots.yml и shots.ps1'
    $yml = Join-Path $root '.github\workflows\win-shots.yml'
    $y = [IO.File]::ReadAllText($yml, [Text.Encoding]::UTF8)
    # run-блоки задания shots идут в Windows PowerShell 5.1, файл без BOM - ANSI: кириллица в них ломает команды
    $inRun = $false; $runIndent = 0; $nonAscii = @(); $runBlocks = 0
    foreach ($ln in ($y.Substring($y.IndexOf("`n  shots:")) -split '\r?\n')) {
        if ($inRun -and $ln.Trim() -and ($ln.Length - $ln.TrimStart().Length) -le $runIndent) { $inRun = $false }
        if ($inRun -and $ln -cmatch '[^\x00-\x7F]') { $nonAscii += $ln.Trim() }
        if ($ln -match '^(\s*)(- )?run:\s*\|') { $inRun = $true; $runBlocks++; $runIndent = $Matches[1].Length + $(if ($Matches[2]) { 2 } else { 0 }) }
    }
    Check 'run-блоки задания shots найдены и только ASCII' ($runBlocks -ge 4 -and $nonAscii.Count -eq 0) ("блоков $runBlocks; " + ($nonAscii -join ' // '))
    Check 'pack.py вызывается по контракту design §2' (($y -match 'pack\.py" check \S+ --os windows') -and ($y -match 'pack\.py" windows --exe \S+ --version \S+ --guide .+ --changelog \S+ --notices \S+ --out \S+'))
    Check 'архив уходит в shots.ps1 как ZipPath' ($y.Contains('ZipPath = $env:ZIP') -and -not $y.Contains('ExePath'))
    Check 'права workflow: contents: read' ($y -match '(?m)^permissions:\s*\r?\n\s+contents: read')
    $shotsText = [IO.File]::ReadAllText((Join-Path $here 'shots.ps1'), [Text.Encoding]::UTF8)
    $zipText = [IO.File]::ReadAllText((Join-Path $here 'winzip.psm1'), [Text.Encoding]::UTF8)
    Check 'shots.ps1: вход ZipPath вместо ExePath, главы под Invoke-Chapter' ($shotsText.Contains('[string]$ZipPath') -and -not $shotsText.Contains('$ExePath') -and $shotsText.Contains("Invoke-Chapter 'Распакуйте архив'") -and $shotsText.Contains('Invoke-ZipFirstRun') -and $shotsText.Contains('Invoke-ZipFromArchive'))
    $shots = @('win-08-zip-downloads', 'win-09-zip-extract-all', 'win-10-extract-dialog', 'win-11-extracted', 'win-12-run-from-zip')
    Check 'winzip.psm1: стоп-кадры win-08..win-12 по design §4.2' (@($shots | Where-Object { -not $zipText.Contains($_) }).Count -eq 0)
    # Expand-Archive метку Zone.Identifier на извлечённые файлы не переносит - SmartScreen пропал бы (research §1.1)
    $bad = @(Get-ChildItem -LiteralPath $here -File | Where-Object { $_.Extension -in '.ps1', '.psm1' -and $_.Name -ne 'test_winzip.ps1' } |
            Where-Object { [IO.File]::ReadAllText($_.FullName, [Text.Encoding]::UTF8) -match 'Expand-Archive\s+-' } | ForEach-Object { $_.Name })
    Check 'распаковка не через Expand-Archive' ($bad.Count -eq 0) ($bad -join ', ')
} finally {
    Remove-Item -LiteralPath $tmp -Recurse -Force -ErrorAction SilentlyContinue
}

Write-Host ("`nпрошло {0}, провалов {1}" -f $script:Pass, $script:Fail)
if ($script:Fail -gt 0) { exit 1 }
exit 0
