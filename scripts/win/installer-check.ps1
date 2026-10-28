# installer-check.ps1 - проверка установщика на раннере windows-latest (спека 111, design §8.1; задание win-check в release.yml).
# Запуск: pwsh scripts/win/installer-check.ps1 -Setup <setup X.Y.Z> -Next <setup X.Y.(Z+1), тот же exe> -Version X.Y.Z [-Standard <стандартный setup X.Y.Z с примером>] [-Summary <файл>] [-Force]
# Ставит и удаляет программу по-настоящему: на машине с уже стоящей программой или config.json отказывает без -Force.
# Код выхода: 0 - всё прошло, 1 - есть провалы (список в сводке и в конце вывода). Коды установщика - installer.nsi.
# Windows PowerShell 5.1 и pwsh 7: файл в UTF-8 с BOM; без Invoke-Expression и сети.
param(
    [Parameter(Mandatory = $true)][string]$Setup,
    [Parameter(Mandatory = $true)][string]$Next,
    [Parameter(Mandatory = $true)][ValidatePattern('^\d+\.\d+\.\d+$')][string]$Version,
    [string]$Standard = '',
    [string]$Summary = 'installer-check-summary.md',
    [switch]$Force
)
$ErrorActionPreference = 'Stop'

$parts = $Version.Split('.')
$NextVersion = '{0}.{1}.{2}' -f $parts[0], $parts[1], ([int]$parts[2] + 1)
$Inst = Join-Path $env:LOCALAPPDATA 'Programs\Sosnina Albums'
$ExePath = Join-Path $Inst 'SosninaAlbums.exe'
$UninstExe = Join-Path $Inst 'uninstall.exe'
$DataRoaming = Join-Path $env:APPDATA 'SosninaAlbums'
$DataLocal = Join-Path $env:LOCALAPPDATA 'SosninaAlbums'
$Config = Join-Path $DataRoaming 'config.json'
$Lnk = Join-Path $env:APPDATA 'Microsoft\Windows\Start Menu\Programs\Sosnina Albums.lnk'
$DesktopLnk = Join-Path ([Environment]::GetFolderPath('Desktop')) 'Sosnina Albums.lnk'
$KUninst = 'Software\Microsoft\Windows\CurrentVersion\Uninstall\SosninaAlbums'
$KApp = 'Software\SosninaAlbums'
$KProg = 'Software\Classes\SosninaAlbums.Album'
$KExt = 'Software\Classes\.salbum'

$script:Fail = 0
$script:Rows = New-Object System.Collections.ArrayList

function Check([string]$Name, $Cond, [string]$Detail = '') {
    $ok = ($Cond -is [bool]) -and $Cond
    if ($ok) { Write-Host "ok    $Name" } else { $script:Fail++; Write-Host "FAIL  $Name  $Detail" }
    [void]$script:Rows.Add([pscustomobject]@{ Name = $Name; Ok = $ok; Detail = $Detail })
}
function Same([string]$Name, $Got, $Want) { Check $Name ("$Got" -ceq "$Want") "получено '$Got', ожидалось '$Want'" }
function Section([string]$Title) { Write-Host "-- $Title" }

# Значение ключа HKCU (имя '' - значение по умолчанию); $null - ключа нет
function Get-Reg([string]$Key, [string]$Name = '') {
    $k = Get-Item -LiteralPath "HKCU:\$Key" -ErrorAction SilentlyContinue
    if (-not $k) { return $null }
    return $k.GetValue($Name)
}
function Test-RegKey([string]$Key) { return [bool](Test-Path -LiteralPath "HKCU:\$Key") }

# Запуск установщика без ожидания потомков (Start-Process -Wait ждёт и программу, запущенную /relaunch)
function Start-Setup([string]$Exe, [string]$Arguments) {
    $psi = New-Object System.Diagnostics.ProcessStartInfo
    $psi.FileName = $Exe
    $psi.Arguments = $Arguments
    $psi.UseShellExecute = $false
    return [System.Diagnostics.Process]::Start($psi)
}
# Код выхода; -1 - не уложился в срок (процесс снят)
function Invoke-Setup([string]$Exe, [string]$Arguments, [int]$TimeoutSec = 180) {
    $p = Start-Setup $Exe $Arguments
    if (-not $p.WaitForExit($TimeoutSec * 1000)) {
        try { $p.Kill() } catch { }
        return -1
    }
    return $p.ExitCode
}

# Процессы программы из установленной папки
function Get-AppProc {
    return @(Get-Process -Name 'SosninaAlbums' -ErrorAction SilentlyContinue | Where-Object {
        try { $_.Path -ieq $ExePath } catch { $false }
    })
}
function Wait-AppProc([int]$TimeoutSec) {
    $sw = [System.Diagnostics.Stopwatch]::StartNew()
    do {
        $list = Get-AppProc
        if ($list.Count -gt 0) { return $list }
        Start-Sleep -Milliseconds 500
    } while ($sw.Elapsed.TotalSeconds -lt $TimeoutSec)
    return @()
}
# Закрыть по PID вместе с дочерними (WebView2); чужие процессы не трогаем
function Stop-App {
    foreach ($p in (Get-AppProc)) {
        try { & taskkill.exe /PID $p.Id /T /F | Out-Null } catch { Write-Host "taskkill: $($_.Exception.Message)" }
    }
    $sw = [System.Diagnostics.Stopwatch]::StartNew()
    while ((Get-AppProc).Count -gt 0 -and $sw.Elapsed.TotalSeconds -lt 20) { Start-Sleep -Milliseconds 300 }
}

# Манифест установщика пишется в ресурсы открытым XML: ищем строку в байтах
function Get-ManifestLevel([string]$Path) {
    $text = [System.Text.Encoding]::GetEncoding('iso-8859-1').GetString([System.IO.File]::ReadAllBytes($Path))
    $m = [regex]::Match($text, 'requestedExecutionLevel\s+level="([A-Za-z]+)"')
    if ($m.Success) { return $m.Groups[1].Value }
    return ''
}

# Читать log.txt при запущенной программе (она держит файл на запись)
function Read-Shared([string]$Path) {
    if (-not (Test-Path -LiteralPath $Path)) { return '' }
    $fs = New-Object System.IO.FileStream($Path, 'Open', 'Read', 'ReadWrite')
    try {
        $sr = New-Object System.IO.StreamReader($fs, [System.Text.Encoding]::UTF8)
        return $sr.ReadToEnd()
    } finally { $fs.Dispose() }
}

function Write-Summary {
    $lines = @('### Установщик (win-check)', '', "Версия $Version, следующая $NextVersion.", '', '| Проверка | Итог |', '| --- | --- |')
    foreach ($r in $script:Rows) {
        $mark = 'ok'
        if (-not $r.Ok) { $mark = 'ПРОВАЛ ' + ($r.Detail -replace '\|', '/') }
        $lines += ('| {0} | {1} |' -f ($r.Name -replace '\|', '/'), $mark)
    }
    [System.IO.File]::WriteAllText($Summary, (($lines -join "`n") + "`n"), (New-Object System.Text.UTF8Encoding($false)))
}

try {
    Section 'Исходные данные'
    foreach ($f in @($Setup, $Next)) { if (-not (Test-Path -LiteralPath $f -PathType Leaf)) { throw "нет файла установщика: $f" } }
    if (-not $Force) {
        if (Test-Path -LiteralPath $Inst) { throw "папка $Inst уже есть: проверка её удалит, запустите с -Force, если это можно" }
        if (Test-Path -LiteralPath $Config) { throw "$Config уже есть: проверка пишет туда метку, запустите с -Force, если это можно" }
    }
    Same 'версия установщика в свойствах файла' ((Get-Item -LiteralPath $Setup).VersionInfo.ProductVersion) $Version
    Same 'версия пробного установщика в свойствах файла' ((Get-Item -LiteralPath $Next).VersionInfo.ProductVersion) $NextVersion
    # 5а. Без запроса повышения (UAC): установщик в имени setup иначе получил бы окно запроса
    Same 'манифест установщика: asInvoker' (Get-ManifestLevel $Setup) 'asInvoker'
    Same 'манифест пробного установщика: asInvoker' (Get-ManifestLevel $Next) 'asInvoker'

    Section 'Метка пользователя'
    New-Item -ItemType Directory -Force -Path $DataRoaming | Out-Null
    [System.IO.File]::WriteAllText($Config, '{"level":"simple"}', (New-Object System.Text.UTF8Encoding($false)))
    $mark = [System.IO.File]::ReadAllText($Config)

    Section 'Тихая установка'
    Same 'setup /S - код 0' (Invoke-Setup $Setup '/S') 0
    Check 'программа и uninstall.exe на месте' ((Test-Path -LiteralPath $ExePath -PathType Leaf) -and (Test-Path -LiteralPath $UninstExe -PathType Leaf))
    Check 'exe не пустой' ((Test-Path -LiteralPath $ExePath) -and ((Get-Item -LiteralPath $ExePath).Length -gt 0))
    Check 'нет недописанного .exe.new' (-not (Test-Path -LiteralPath "$ExePath.new"))
    Same 'HKCU InstallDir' (Get-Reg $KApp 'InstallDir') $Inst
    Same 'HKCU Version' (Get-Reg $KApp 'Version') $Version
    Same 'Приложения: DisplayVersion' (Get-Reg $KUninst 'DisplayVersion') $Version
    Same 'Приложения: DisplayName' (Get-Reg $KUninst 'DisplayName') 'Sosnina Albums'
    Same 'Приложения: InstallLocation' (Get-Reg $KUninst 'InstallLocation') $Inst
    Same 'Приложения: UninstallString' (Get-Reg $KUninst 'UninstallString') ('"' + $UninstExe + '"')
    Same 'Приложения: QuietUninstallString' (Get-Reg $KUninst 'QuietUninstallString') ('"' + $UninstExe + '" /S')
    Check 'Приложения: издатель и значок' ((-not [string]::IsNullOrEmpty((Get-Reg $KUninst 'Publisher'))) -and ((Get-Reg $KUninst 'DisplayIcon') -like "*SosninaAlbums.exe*"))
    Same 'ассоциация: .salbum -> ProgID' (Get-Reg $KExt '') 'SosninaAlbums.Album'
    Check 'ассоциация: OpenWithProgids' (Test-RegKey "$KExt\OpenWithProgids")
    Same 'ProgID: AppVersion' (Get-Reg $KProg 'AppVersion') $Version
    Same 'ProgID: команда открытия' (Get-Reg "$KProg\shell\open\command" '') ('"' + $ExePath + '" "%1"')
    Check 'ярлык в Пуске' (Test-Path -LiteralPath $Lnk)
    Check 'на рабочем столе ярлыка нет (тихо)' (-not (Test-Path -LiteralPath $DesktopLnk))

    Section 'Запуск установленной'
    $logPath = Join-Path $DataRoaming 'log.txt'
    $want = "запуск, версия $Version"
    # строка могла остаться от прежнего запуска (-Force на машине разработчика): ждём новую
    $before = [regex]::Matches((Read-Shared $logPath), [regex]::Escape($want)).Count
    [void](Start-Process -FilePath $ExePath -PassThru)
    $sw = [System.Diagnostics.Stopwatch]::StartNew()
    $seen = $false
    do {
        Start-Sleep -Seconds 1
        $seen = [regex]::Matches((Read-Shared $logPath), [regex]::Escape($want)).Count -gt $before
    } while (-not $seen -and $sw.Elapsed.TotalSeconds -lt 30)
    Check "log.txt: «$want»" $seen
    Stop-App
    Check 'программа закрыта' ((Get-AppProc).Count -eq 0)

    Section 'Сбой замены exe'
    # Хеш прежнего exe и версия: после отказа установщика ничего не должно измениться
    $hashBefore = (Get-FileHash -LiteralPath $ExePath -Algorithm SHA256).Hash
    # а) exe открыт без права переименования (FileShare.Read, не Delete): замка Wails нет, ждём 5 с и пробуем заменить -
    # 5 с повторов, затем откат. (Занятый .old больше не причина кода 6: он уходит в .old-<тики>, 0.31-143-32)
    $lockExe = [System.IO.File]::Open($ExePath, 'Open', 'Read', 'Read')
    try { $code6 = Invoke-Setup $Next '/S' } finally { $lockExe.Dispose() }
    Same 'next /S, когда старый exe не переименовать - код 6' $code6 6
    Same 'прежний exe цел (хеш)' ((Get-FileHash -LiteralPath $ExePath -Algorithm SHA256).Hash) $hashBefore
    Same 'версия в Приложениях прежняя' (Get-Reg $KUninst 'DisplayVersion') $Version
    Same 'ProgID: AppVersion прежний' (Get-Reg $KProg 'AppVersion') $Version
    Check 'нет недописанного .exe.new' (-not (Test-Path -LiteralPath "$ExePath.new"))
    # б) .exe.new открыт без разрешения удалять: установщик не может записать новую копию
    $lockNew = [System.IO.File]::Open("$ExePath.new", 'Create', 'ReadWrite', 'None')
    $code = Invoke-Setup $Next '/S'
    $lockNew.Dispose()
    Remove-Item -LiteralPath "$ExePath.new" -Force -ErrorAction SilentlyContinue
    Check 'next /S при занятом .exe.new - сбой (код не 0)' ($code -ne 0) "код $code"
    Same 'после сбоя exe цел (хеш)' ((Get-FileHash -LiteralPath $ExePath -Algorithm SHA256).Hash) $hashBefore
    Same 'после сбоя версия прежняя' (Get-Reg $KUninst 'DisplayVersion') $Version
    Check 'после сбоя ярлык Пуска на месте' (Test-Path -LiteralPath $Lnk)

    Section 'Обновление поверх занятого exe'
    # Занятый .old (образ живого посредника --mcp; здесь папка - её Delete не стирает): уходит в .old-<тики>, замена проходит
    $oldDir = "$ExePath.old"
    New-Item -ItemType Directory -Force -Path $oldDir | Out-Null
    [System.IO.File]::WriteAllText((Join-Path $oldDir 'x'), 'x')
    $lock = [System.IO.File]::Open($ExePath, 'Open', 'Read', 'Read')
    $sw = [System.Diagnostics.Stopwatch]::StartNew()
    $p = Start-Setup $Next '/S'
    Start-Sleep -Seconds 4
    $lock.Dispose()
    $done = $p.WaitForExit(180000)
    Check 'установщик дождался освобождения exe (>= 4 с)' ($done -and $sw.Elapsed.TotalSeconds -ge 3.9) ("{0:N1} с" -f $sw.Elapsed.TotalSeconds)
    if ($done) { Same 'next /S поверх - код 0' $p.ExitCode 0 } else { try { $p.Kill() } catch { }; Check 'next /S поверх завершился' $false }
    Same 'DisplayVersion обновлён' (Get-Reg $KUninst 'DisplayVersion') $NextVersion
    Same 'ProgID: AppVersion обновлён' (Get-Reg $KProg 'AppVersion') $NextVersion
    Check 'занятый .exe.old переименован в .exe.old-<тики>' ((-not (Test-Path -LiteralPath $oldDir)) -and (@(Get-ChildItem -LiteralPath $Inst -Filter 'SosninaAlbums.exe.old-*' -Force).Count -eq 1))
    Remove-Item -LiteralPath (Join-Path $Inst 'SosninaAlbums.exe.old-*') -Recurse -Force -ErrorAction SilentlyContinue

    Section 'Откат и та же версия'
    Same 'setup /S поверх новее - код 3' (Invoke-Setup $Setup '/S') 3
    Same 'после отказа версия прежняя' (Get-Reg $KUninst 'DisplayVersion') $NextVersion
    Same 'next /S той же версии - код 4' (Invoke-Setup $Next '/S') 4
    Same 'setup /S /relaunch поверх новее - код 3' (Invoke-Setup $Setup '/S /relaunch') 3
    $running = Wait-AppProc 30
    Check 'при отказе /relaunch открыл прежнюю программу' ($running.Count -gt 0)
    Stop-App

    Section 'Удаление'
    New-Item -ItemType Directory -Force -Path (Join-Path $DataLocal 'Updates') | Out-Null
    [System.IO.File]::WriteAllText((Join-Path $DataLocal 'Updates\old.part'), 'x')
    New-Item -ItemType Directory -Force -Path (Join-Path $DataLocal 'thumbs') | Out-Null
    [System.IO.File]::WriteAllText((Join-Path $DataLocal 'thumbs\keep.txt'), 'x')
    # Программа между меткой и удалением законно дописывает в config.json свои поля (assoc, спека 112): сверяем с состоянием перед удалением
    $mark = [System.IO.File]::ReadAllText($Config)
    Check 'config.json перед удалением: level=simple' ($mark -match '"level"\s*:\s*"simple"')
    Same 'uninstall.exe /S - код 0' (Invoke-Setup $UninstExe "/S _?=$Inst") 0
    Check 'exe удалён' (-not (Test-Path -LiteralPath $ExePath))
    Check 'в папке остался только сам uninstall.exe' (@(Get-ChildItem -LiteralPath $Inst -Force).Count -eq 1)
    Check 'ярлык Пуска удалён' (-not (Test-Path -LiteralPath $Lnk))
    Check 'ключ Приложений удалён' (-not (Test-RegKey $KUninst))
    Check 'ключ SosninaAlbums удалён' (-not (Test-RegKey $KApp))
    Check 'ProgID удалён' (-not (Test-RegKey $KProg))
    Check 'ассоциация .salbum снята' ($null -eq (Get-Reg $KExt ''))
    Check 'пустые ключи .salbum и OpenWithProgids убраны' (-not (Test-RegKey $KExt))
    Check 'скачанные обновления удалены' (-not (Test-Path -LiteralPath (Join-Path $DataLocal 'Updates')))
    Check 'миниатюры остались' (Test-Path -LiteralPath (Join-Path $DataLocal 'thumbs\keep.txt'))
    Check 'config.json пользователя цел' ((Test-Path -LiteralPath $Config) -and ([System.IO.File]::ReadAllText($Config) -ceq $mark))
    # Саму копию uninstall.exe удаляет проверка: при _?= NSIS её не трогает
    Remove-Item -LiteralPath $UninstExe -Force -ErrorAction SilentlyContinue
    Remove-Item -LiteralPath $Inst -Force -ErrorAction SilentlyContinue

    Section 'Связь .salbum: чужой ProgID и отметка AssocOff'
    # а) расширение у другой программы: установщик его не перетирает и свой ProgID не пишет
    New-Item -Path "HKCU:\$KExt" -Force | Out-Null
    Set-Item -LiteralPath "HKCU:\$KExt" -Value 'Other.Album'
    Same 'setup /S при чужой связи - код 0' (Invoke-Setup $Setup '/S') 0
    Same 'чужой ProgID .salbum не перетёрт' (Get-Reg $KExt '') 'Other.Album'
    Check 'наш ProgID не записан' (-not (Test-RegKey $KProg))
    Check 'ярлык Пуска на месте' (Test-Path -LiteralPath $Lnk)
    Same 'uninstall.exe /S (чужая связь) - код 0' (Invoke-Setup $UninstExe "/S _?=$Inst") 0
    Same 'чужой ProgID пережил удаление' (Get-Reg $KExt '') 'Other.Album'
    Remove-Item -LiteralPath "HKCU:\$KExt" -Recurse -Force -ErrorAction SilentlyContinue
    Remove-Item -LiteralPath $UninstExe -Force -ErrorAction SilentlyContinue
    Remove-Item -LiteralPath $Inst -Force -ErrorAction SilentlyContinue
    # б) пользователь выключил связь (AssocOff=1): тихая установка её не возвращает
    New-Item -Path "HKCU:\$KApp" -Force | Out-Null
    New-ItemProperty -LiteralPath "HKCU:\$KApp" -Name 'AssocOff' -Value '1' -PropertyType String -Force | Out-Null
    Same 'setup /S при AssocOff - код 0' (Invoke-Setup $Setup '/S') 0
    Check 'связь .salbum не записана' ($null -eq (Get-Reg $KExt ''))
    Check 'наш ProgID не записан (AssocOff)' (-not (Test-RegKey $KProg))
    Same 'Version записана' (Get-Reg $KApp 'Version') $Version
    Same 'uninstall.exe /S (AssocOff) - код 0' (Invoke-Setup $UninstExe "/S _?=$Inst") 0
    Check 'отметка AssocOff убрана вместе с ключом' (-not (Test-RegKey $KApp))
    Remove-Item -LiteralPath $UninstExe -Force -ErrorAction SilentlyContinue
    Remove-Item -LiteralPath $Inst -Force -ErrorAction SilentlyContinue

    if ($Standard) {
        Section 'Стандартная редакция: пример в data\ (0.30-142-43)'
        if (-not (Test-Path -LiteralPath $Standard -PathType Leaf)) { throw "нет файла стандартного установщика: $Standard" }
        Same 'версия стандартного установщика в свойствах файла' ((Get-Item -LiteralPath $Standard).VersionInfo.ProductVersion) $Version
        Same 'манифест стандартного установщика: asInvoker' (Get-ManifestLevel $Standard) 'asInvoker'
        $DataDir = Join-Path $Inst 'data'
        # Имя примера не зашито: маска по префиксу, как у деинсталлятора (v2 примера подойдёт)
        function Get-Demo { return @(Get-ChildItem -LiteralPath $DataDir -Filter 'wedding-navy-watercolor-demo-*.zip' -File -ErrorAction SilentlyContinue) }

        # (2) стандартный X и его собственное удаление
        Same 'standard /S - код 0' (Invoke-Setup $Standard '/S') 0
        Check 'программа и uninstall.exe на месте (стандартная)' ((Test-Path -LiteralPath $ExePath -PathType Leaf) -and (Test-Path -LiteralPath $UninstExe -PathType Leaf))
        Same 'пример в data\ - один файл' (Get-Demo).Count 1
        $demoSize = 0
        $demoHash = ''
        if ((Get-Demo).Count -eq 1) {
            $demoSize = (Get-Demo)[0].Length
            $demoHash = (Get-FileHash -LiteralPath (Get-Demo)[0].FullName -Algorithm SHA256).Hash
        }
        Check 'пример не пустой' ($demoSize -gt 0)
        Same 'Приложения: DisplayVersion (стандартная)' (Get-Reg $KUninst 'DisplayVersion') $Version
        Check 'EstimatedSize считает data\' ([int](Get-Reg $KUninst 'EstimatedSize') -ge [int][math]::Floor($demoSize / 1024)) "EstimatedSize=$(Get-Reg $KUninst 'EstimatedSize') КБ, пример $demoSize байт"
        # ярлык открывает программу из папки установки, не из data\ (SetOutPath возвращён)
        $wsh = New-Object -ComObject WScript.Shell
        Same 'ярлык Пуска: рабочая папка - папка программы' ($wsh.CreateShortcut($Lnk).WorkingDirectory) $Inst
        Same 'standard uninstall.exe /S - код 0' (Invoke-Setup $UninstExe "/S _?=$Inst") 0
        Check 'пример и data\ удалены' (-not (Test-Path -LiteralPath $DataDir))
        Check 'в папке остался только сам uninstall.exe (стандартная)' (@(Get-ChildItem -LiteralPath $Inst -Force).Count -eq 1)
        Check 'ключ Приложений удалён (стандартная)' (-not (Test-RegKey $KUninst))
        Remove-Item -LiteralPath $UninstExe -Force -ErrorAction SilentlyContinue
        Remove-Item -LiteralPath $Inst -Force -ErrorAction SilentlyContinue

        # (1) стандартный X -> компактный X+1: data\ цел, uninstall.exe от X+1 чистит data\ и папку.
        # Той же версии /S вышел бы с кодом 4 и ничего не поставил (installer.nsi) - поэтому именно Next
        Same 'standard /S (до обновления) - код 0' (Invoke-Setup $Standard '/S') 0
        Same 'next (компактный X+1) /S поверх стандартной - код 0' (Invoke-Setup $Next '/S') 0
        Same 'Приложения: DisplayVersion обновлён (X+1)' (Get-Reg $KUninst 'DisplayVersion') $NextVersion
        Same 'пример цел после компактного X+1 - один файл' (Get-Demo).Count 1
        if ((Get-Demo).Count -eq 1) {
            Same 'пример цел после компактного X+1 - хеш' ((Get-FileHash -LiteralPath (Get-Demo)[0].FullName -Algorithm SHA256).Hash) $demoHash
        }
        Check 'EstimatedSize с data\ после компактного X+1' ([int](Get-Reg $KUninst 'EstimatedSize') -ge [int][math]::Floor($demoSize / 1024)) "EstimatedSize=$(Get-Reg $KUninst 'EstimatedSize') КБ"
        Same 'next uninstall.exe /S - код 0' (Invoke-Setup $UninstExe "/S _?=$Inst") 0
        Check 'uninstall.exe от X+1 удалил пример и data\' (-not (Test-Path -LiteralPath $DataDir))
        Check 'в папке остался только сам uninstall.exe (после X+1)' (@(Get-ChildItem -LiteralPath $Inst -Force).Count -eq 1)
        Remove-Item -LiteralPath $UninstExe -Force -ErrorAction SilentlyContinue
        Remove-Item -LiteralPath $Inst -Force -ErrorAction SilentlyContinue
    }
} catch {
    $script:Fail++
    Write-Host "FAIL  проверка прервана: $($_.Exception.Message)"
    [void]$script:Rows.Add([pscustomobject]@{ Name = 'проверка прервана'; Ok = $false; Detail = $_.Exception.Message })
} finally {
    Stop-App
    Write-Summary
}

if ($script:Fail -gt 0) { Write-Host "installer-check: провалов $($script:Fail)"; exit 1 }
Write-Host 'installer-check: OK'
exit 0
