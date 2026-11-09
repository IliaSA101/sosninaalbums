# winqa-dpi.psm1 - масштаб экрана на лету для снимков мастера на 125-200 % (спека 118, design §4, решение 118-7).
# DisplayConfigGetDeviceInfo/SetDeviceInfo с типами -3/-4 не документированы Microsoft (так работает "Параметры - Дисплей"):
# отказ - не сбой, а ответ пробы, пункт тогда уходит на ноутбук. Windows PowerShell 5.1, файл в UTF-8 с BOM.

if (-not ('SaQa.Dpi' -as [type])) {
    Add-Type -TypeDefinition @'
using System;
using System.Collections.Generic;
using System.Runtime.InteropServices;
namespace SaQa {
    [StructLayout(LayoutKind.Sequential, CharSet = CharSet.Unicode)]
    public struct DM {
        [MarshalAs(UnmanagedType.ByValTStr, SizeConst = 32)] public string dmDeviceName;
        public short dmSpecVersion; public short dmDriverVersion; public short dmSize; public short dmDriverExtra;
        public int dmFields;
        public int dmPositionX; public int dmPositionY; public int dmDisplayOrientation; public int dmDisplayFixedOutput;
        public short dmColor; public short dmDuplex; public short dmYResolution; public short dmTTOption; public short dmCollate;
        [MarshalAs(UnmanagedType.ByValTStr, SizeConst = 32)] public string dmFormName;
        public short dmLogPixels;
        public int dmBitsPerPel; public int dmPelsWidth; public int dmPelsHeight; public int dmDisplayFlags; public int dmDisplayFrequency;
        public int dmICMMethod; public int dmICMIntent; public int dmMediaType; public int dmDitherType;
        public int dmReserved1; public int dmReserved2; public int dmPanningWidth; public int dmPanningHeight;
    }
    public static class Dpi {
        [DllImport("user32.dll")] static extern int GetDisplayConfigBufferSizes(uint flags, out uint numPaths, out uint numModes);
        [DllImport("user32.dll")] static extern int QueryDisplayConfig(uint flags, ref uint numPaths, byte[] paths, ref uint numModes, byte[] modes, IntPtr topology);
        [DllImport("user32.dll", EntryPoint = "DisplayConfigGetDeviceInfo")] static extern int GetInfo(byte[] packet);
        [DllImport("user32.dll", EntryPoint = "DisplayConfigSetDeviceInfo")] static extern int SetInfo(byte[] packet);
        [DllImport("user32.dll")] public static extern uint GetDpiForWindow(IntPtr hwnd);
        [DllImport("user32.dll")] public static extern uint GetDpiForSystem();
        [DllImport("user32.dll", CharSet = CharSet.Unicode, EntryPoint = "EnumDisplaySettingsW")] static extern bool EnumDisplaySettings(string dev, int mode, ref DM dm);

        static readonly int[] Steps = { 100, 125, 150, 175, 200, 225, 250, 300, 350, 400, 450, 500 };

        // Первый активный путь: LUID адаптера (8 байт) и id источника; DISPLAYCONFIG_PATH_INFO - 72 байта, MODE_INFO - 64
        static bool Source(out byte[] adapter, out uint id) {
            adapter = new byte[8]; id = 0;
            uint np, nm;
            if (GetDisplayConfigBufferSizes(2, out np, out nm) != 0 || np == 0) return false;   // QDC_ONLY_ACTIVE_PATHS
            var paths = new byte[72 * np]; var modes = new byte[64 * nm];
            if (QueryDisplayConfig(2, ref np, paths, ref nm, modes, IntPtr.Zero) != 0 || np == 0) return false;
            Array.Copy(paths, 0, adapter, 0, 8);
            id = BitConverter.ToUInt32(paths, 8);
            return true;
        }
        // Заголовок DISPLAYCONFIG_DEVICE_INFO_HEADER: тип, размер, LUID, id источника (20 байт)
        static byte[] Packet(int type, int size, byte[] adapter, uint id) {
            var p = new byte[size];
            BitConverter.GetBytes(type).CopyTo(p, 0);
            BitConverter.GetBytes((uint)size).CopyTo(p, 4);
            Array.Copy(adapter, 0, p, 8, 8);
            BitConverter.GetBytes(id).CopyTo(p, 16);
            return p;
        }
        // {рекомендуемый, текущий, максимальный} в процентах или null. Относительные шаги считаются от рекомендуемого: рек. = -min
        public static int[] Get() {
            byte[] ad; uint id;
            if (!Source(out ad, out id)) return null;
            var p = Packet(-3, 32, ad, id);
            if (GetInfo(p) != 0) return null;
            int min = BitConverter.ToInt32(p, 20), cur = BitConverter.ToInt32(p, 24), max = BitConverter.ToInt32(p, 28);
            int rec = -min;
            if (rec < 0 || rec + max >= Steps.Length || rec + cur < 0 || rec + cur >= Steps.Length) return null;
            return new int[] { Steps[rec], Steps[rec + cur], Steps[rec + max] };
        }
        // 0 - применено; -1 нет источника; -2 нет такого шага у экрана; иначе код DisplayConfigSetDeviceInfo
        public static int Set(int percent) {
            byte[] ad; uint id;
            if (!Source(out ad, out id)) return -1;
            var cur = Get();
            if (cur == null) return -1;
            int idx = Array.IndexOf(Steps, percent), rec = Array.IndexOf(Steps, cur[0]), max = Array.IndexOf(Steps, cur[2]);
            if (idx < 0 || idx > max) return -2;
            var p = Packet(-4, 24, ad, id);
            BitConverter.GetBytes(idx - rec).CopyTo(p, 20);
            return SetInfo(p);
        }
        // Режимы экрана "ШxВ" без повторов
        public static string[] Modes() {
            var set = new List<string>();
            for (int i = 0; i < 400; i++) {
                var dm = new DM();
                dm.dmSize = (short)Marshal.SizeOf(typeof(DM));
                if (!EnumDisplaySettings(null, i, ref dm)) break;
                string s = dm.dmPelsWidth + "x" + dm.dmPelsHeight;
                if (!set.Contains(s)) set.Add(s);
            }
            return set.ToArray();
        }
    }
}
'@
}

# Текущий масштаб: Ok, Rec/Cur/Max в процентах; Ok=$false - API отказал (на этой ВМ масштаб на лету не сменить)
function Get-QaDpiState {
    $r = $null
    try { $r = [SaQa.Dpi]::Get() } catch { return [pscustomobject]@{ Ok = $false; Rec = 0; Cur = 0; Max = 0; Why = $_.Exception.Message } }
    if (-not $r) { return [pscustomobject]@{ Ok = $false; Rec = 0; Cur = 0; Max = 0; Why = 'DisplayConfig не ответил' } }
    return [pscustomobject]@{ Ok = $true; Rec = $r[0]; Cur = $r[1]; Max = $r[2]; Why = '' }
}

# Включает "исправлять размытость" (новые процессы берут текущий DPI без повторного входа) и ставит масштаб.
# Результат: Ok и Why. Ok только если API принял и масштаб стал нужным
function Set-QaDpi([int]$Percent) {
    New-Item -Path 'HKCU:\Control Panel\Desktop' -Force | Out-Null
    Set-ItemProperty -Path 'HKCU:\Control Panel\Desktop' -Name 'EnablePerProcessSystemDPI' -Value 1 -Type DWord
    $code = [SaQa.Dpi]::Set($Percent)
    if ($code -ne 0) { return [pscustomobject]@{ Ok = $false; Why = "SetDeviceInfo: код $code" } }
    Start-Sleep -Seconds 2
    $now = Get-QaDpiState
    if (-not $now.Ok -or $now.Cur -ne $Percent) { return [pscustomobject]@{ Ok = $false; Why = "после установки масштаб $($now.Cur) %" } }
    return [pscustomobject]@{ Ok = $true; Why = '' }
}

# DPI окна по дескриптору UIA (NativeWindowHandle); 0 - окна нет
function Get-QaWindowDpi($El) {
    try {
        $h = [int]$El.Current.NativeWindowHandle
        if ($h -eq 0) { return 0 }
        return [int][SaQa.Dpi]::GetDpiForWindow([IntPtr]$h)
    } catch { return 0 }
}

# Системный DPI в свежем процессе powershell (в своём он закеширован при старте)
function Get-QaFreshSystemDpi {
    $code = '$t = Add-Type -Name N -Namespace S -PassThru -MemberDefinition ''[DllImport("user32.dll")] public static extern bool SetProcessDPIAware(); [DllImport("user32.dll")] public static extern uint GetDpiForSystem();''; [void]$t::SetProcessDPIAware(); $t::GetDpiForSystem()'
    $enc = [Convert]::ToBase64String([System.Text.Encoding]::Unicode.GetBytes($code))
    $out = & powershell.exe -NoProfile -ExecutionPolicy Bypass -EncodedCommand $enc 2>$null
    $n = 0
    if ([int]::TryParse((@($out)[-1]), [ref]$n)) { return $n }
    return 0
}

function Get-QaDisplayModes { return @([SaQa.Dpi]::Modes()) }
