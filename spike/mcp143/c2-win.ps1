# С2 (а) и (б) в Windows (design §4.3): пользователь с кириллическим именем из 20 букв и второй пользователь без прав администратора.
# Файл с BOM (Windows PowerShell 5.1 без BOM читает как ANSI); имя собрано из кодов символов, чтобы не зависеть от кодировки.
# Каждая проверка независима; результаты идут в results.tsv через lib.mjs (record/ingest), при провале — снимок состояния.
param(
  [Parameter(Mandatory = $true)][string]$Console,
  [Parameter(Mandatory = $true)][string]$Out
)
$ErrorActionPreference = 'Continue'
$Lib = Join-Path $PSScriptRoot 'lib.mjs'
$env:SPIKE_OUT = $Out
$Root = 'C:\spike143'

function Rec($id, $st, $d) { & node $Lib record $id $st ([string]$d) | Out-Null; Write-Host "$st $id $d" }
function Check($id, [scriptblock]$b) {
  try { $r = & $b; Rec $id 'OK' (($r | Out-String).Trim()) }
  catch { Rec $id 'FAIL' $_.Exception.Message; & node $Lib snapshot ($id -replace '[^\w.-]', '_') | Out-Null }
}
function Need($cond, $msg) { if (-not $cond) { throw $msg } }

function New-TestUser($name, $pw) {
  $sec = ConvertTo-SecureString $pw -AsPlainText -Force
  try {
    New-LocalUser -Name $name -Password $sec -AccountNeverExpires -PasswordNeverExpires | Out-Null
  } catch {
    # запасной путь: ADSI (тот же NetUserAdd)
    $c = [ADSI]"WinNT://$env:COMPUTERNAME"
    $u = $c.Create('User', $name); $u.SetPassword($pw); $u.SetInfo()
  }
  Add-LocalGroupMember -SID 'S-1-5-32-545' -Member $name -ErrorAction Stop
}

# Запуск spikec.exe от имени пользователя; вывод потомка — в файл общей папки.
function Invoke-AsUser($name, $pw, $tag, $argLine) {
  $o = "$Root\run\$tag.out"; $d = "$Root\run\$tag.done"; $bat = "$Root\run\$tag.cmd"
  Remove-Item $o, $d -ErrorAction SilentlyContinue
  Set-Content -Path $bat -Encoding ASCII -Value @('@echo off', "`"$Root\bin\spikec.exe`" $argLine > `"$o`" 2>&1", "echo %errorlevel% > `"$d`"")
  $method = 'Start-Process -Credential'
  $cred = New-Object System.Management.Automation.PSCredential(".\$name", (ConvertTo-SecureString $pw -AsPlainText -Force))
  try {
    $p = Start-Process -FilePath 'cmd.exe' -ArgumentList "/c `"$bat`"" -Credential $cred -LoadUserProfile -WorkingDirectory $Root -WindowStyle Hidden -PassThru
    $null = $p.WaitForExit(60000)
  } catch { $method = "Start-Process failed: $($_.Exception.Message)" }
  if (-not (Test-Path $d)) {
    $method += ' -> scheduled task'
    $tn = "spike143-$tag"
    try {
      $act = New-ScheduledTaskAction -Execute $bat
      Register-ScheduledTask -TaskName $tn -Action $act -User $name -Password $pw -RunLevel Limited -Force | Out-Null
      Start-ScheduledTask -TaskName $tn
      for ($i = 0; $i -lt 120 -and -not (Test-Path $d); $i++) { Start-Sleep -Milliseconds 500 }
    } catch { $method += " failed: $($_.Exception.Message)" }
    finally { Unregister-ScheduledTask -TaskName $tn -Confirm:$false -ErrorAction SilentlyContinue }
  }
  $text = ''; $code = $null
  if (Test-Path $o) { $text = Get-Content -Raw -Encoding UTF8 $o }
  if (Test-Path $d) { $code = [int](Get-Content $d -Raw).Trim() }
  New-Item -ItemType Directory -Force "$Out\user-runs" | Out-Null
  [IO.File]::WriteAllText("$Out\user-runs\$tag.txt", "method: $method`nexit: $code`n$text", (New-Object Text.UTF8Encoding $false))
  [pscustomobject]@{ Method = $method; Out = $text; Code = $code; File = "$Out\user-runs\$tag.txt" }
}

$cyr = -join ([char[]](0x422, 0x435, 0x441, 0x442, 0x43E, 0x432, 0x44B, 0x439, 0x43F, 0x43E, 0x43B, 0x44C, 0x437, 0x43E, 0x432, 0x430, 0x442, 0x435, 0x43B, 0x44C))
$other = 'spikeother'
$pw1 = 'Sp!ke143-' + [guid]::NewGuid().ToString('N').Substring(0, 12)
$pw2 = 'Sp!ke143-' + [guid]::NewGuid().ToString('N').Substring(0, 12)
$serve = $null
try {
  New-Item -ItemType Directory -Force "$Root\bin", "$Root\run" | Out-Null
  Copy-Item $Console "$Root\bin\spikec.exe" -Force
  & icacls $Root /grant '*S-1-5-32-545:(OI)(CI)RX' | Out-Null
  & icacls "$Root\run" /grant '*S-1-1-0:(OI)(CI)M' | Out-Null
  Rec 'c2win/env' 'INFO' ("runner=$env:USERNAME OS=" + [Environment]::OSVersion.VersionString + " seclogon=" + (Get-Service seclogon).Status + " name_len=" + $cyr.Length)

  # (а) кириллический пользователь: слушать и подключаться по пути профиля с не-ASCII буквами
  Check 'c2a/user-created' { New-TestUser $cyr $pw1; "user of $($cyr.Length) letters created, in group Users" }
  Check 'c2a/selftest-as-user' {
    $r = Invoke-AsUser $cyr $pw1 'a1' "--selftest --out $Root\run"
    & node $Lib ingest c2a $r.File | Out-Null
    Need ($null -ne $r.Code) "child did not finish ($($r.Method))"
    Need ($r.Out.Contains($cyr)) "socket path does not contain the Cyrillic profile name (APPDATA of another user?) [$($r.Method)]"
    Need ($r.Out -match "RESULT`tlisten`tOK") "no listen OK"
    Need ($r.Out -match "RESULT`tconnect-sdk`tOK") "no connect-sdk OK"
    Need ($r.Out -match "RESULT`tclose-removes-file`tOK") "no close-removes-file OK"
    Need ($r.Code -eq 0) "exit code $($r.Code)"
    "started via $($r.Method)"
  }

  # (б) другой пользователь без прав администратора: сокет владельца должен отказать в доступе
  Check 'c2b/user-created' { New-TestUser $other $pw2; "user $other created, in group Users" }
  $ready = "$Root\run\readyb"
  Remove-Item $ready -ErrorAction SilentlyContinue
  $serve = Start-Process -FilePath "$Root\bin\spikec.exe" -ArgumentList "--serve --ttl 150 --ready `"$ready`" --out `"$Root\run`"" -WindowStyle Hidden -PassThru
  $sock = $null
  Check 'c2b/owner-listens' {
    for ($i = 0; $i -lt 100 -and -not (Test-Path $ready); $i++) { Start-Sleep -Milliseconds 100 }
    Need (Test-Path $ready) 'listener did not start'
    $script:sock = (Get-Content $ready -Raw).Trim().Split(' ')[1]
    "listening on $script:sock"
  }
  if ($sock) {
    Check 'c2b/acl-dump' { (& icacls (Split-Path $sock) | Out-String) -replace "\s+", ' ' }
    Check 'c2b/owner-connects-control' {
      $o = & "$Root\bin\spikec.exe" --clientcheck --sock $sock --out $Root\run | Out-String
      Need ($LASTEXITCODE -eq 0) "the owner itself cannot connect: $o"
      'owner connects (control: a refusal below is not an absent socket)'
    }
    Check 'c2b/other-user-denied' {
      $r = Invoke-AsUser $other $pw2 'b1' "--clientcheck --sock `"$sock`" --out $Root\run"
      & node $Lib ingest c2b/clientcheck $r.File | Out-Null
      Need ($null -ne $r.Code) "child did not finish ($($r.Method))"
      Need ($r.Out -notmatch 'CONNECTED') "ANOTHER USER CONNECTED to the owner's socket: $($r.Out)"
      Need ($r.Code -eq 3 -and $r.Out -match 'DENIED') "unexpected result, code $($r.Code): $($r.Out)"
      "refused: " + (($r.Out -split "`n" | Where-Object { $_ -match 'DENIED' }) -join ' ')
    }
  }
} catch {
  Rec 'c2win/crash' 'FAIL' $_.Exception.Message
  & node $Lib snapshot 'c2win-crash' | Out-Null
} finally {
  if ($serve) { Stop-Process -Id $serve.Id -Force -ErrorAction SilentlyContinue }
  foreach ($u in @($cyr, $other)) { Remove-LocalUser -Name $u -ErrorAction SilentlyContinue }
  Copy-Item "$Root\run" "$Out\run-folder" -Recurse -Force -ErrorAction SilentlyContinue
  exit 0
}
