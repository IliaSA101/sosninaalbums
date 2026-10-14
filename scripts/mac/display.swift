// Разрешение главного экрана раннера (029): снимки 1024×768 мелкие и размытые в docx.
// Сборка один раз: swiftc -O display.swift -o display; запасной путь — swift display.swift …
//   display list                   — все режимы, включая дубли низкого разрешения:
//                                    «ширина высота пикс-ширина пикс-высота Гц hidpi|-»;
//   display cur                    — текущий режим в том же виде;
//   display set <ш> <в> [hidpi|any] — выставить режим на сессию; hidpi — только с пикселями плотнее
//                                    точек (Retina), any — любой (HiDPI предпочтительнее при равенстве).
// Код 0 — режим выставлен, 3 — такого режима у раннера нет, 4 — система отказала.
import CoreGraphics
import Foundation

func fail(_ msg: String, _ code: Int32) -> Never {
    FileHandle.standardError.write((msg + "\n").data(using: .utf8)!)
    exit(code)
}

func describe(_ m: CGDisplayMode) -> String {
    let hidpi = m.pixelWidth > m.width ? "hidpi" : "-"
    return "\(m.width) \(m.height) \(m.pixelWidth) \(m.pixelHeight) \(Int(m.refreshRate)) \(hidpi)"
}

let id = CGMainDisplayID()
let opts = [kCGDisplayShowDuplicateLowResolutionModes: kCFBooleanTrue] as CFDictionary
let all: [CGDisplayMode] = CGDisplayCopyAllDisplayModes(id, opts) ?? []
let args = CommandLine.arguments

guard args.count >= 2 else {
    fail("использование: display list | cur | set <ш> <в> [hidpi|any]", 2)
}

switch args[1] {
case "list":
    for m in all {
        print(describe(m) + (m.isUsableForDesktopGUI() ? "" : " unusable"))
    }
case "cur":
    guard let m = CGDisplayCopyDisplayMode(id) else { fail("нет текущего режима", 1) }
    print(describe(m))
case "set":
    guard args.count >= 4, let w = Int(args[2]), let h = Int(args[3]) else {
        fail("использование: display set <ш> <в> [hidpi|any]", 2)
    }
    let wantHi = args.count >= 5 && args[4] == "hidpi"
    var pick = all.filter { $0.width == w && $0.height == h && $0.isUsableForDesktopGUI() }
    if wantHi { pick = pick.filter { $0.pixelWidth > $0.width } }
    // Плотнее пикселями — выше; затем частота
    pick.sort {
        if $0.pixelWidth != $1.pixelWidth { return $0.pixelWidth > $1.pixelWidth }
        return $0.refreshRate > $1.refreshRate
    }
    guard let mode = pick.first else { fail("режима \(w)x\(h)\(wantHi ? " hidpi" : "") нет", 3) }
    var cfg: CGDisplayConfigRef?
    guard CGBeginDisplayConfiguration(&cfg) == .success else { fail("CGBeginDisplayConfiguration", 4) }
    guard CGConfigureDisplayWithDisplayMode(cfg, id, mode, nil) == .success else {
        CGCancelDisplayConfiguration(cfg)
        fail("CGConfigureDisplayWithDisplayMode", 4)
    }
    guard CGCompleteDisplayConfiguration(cfg, .forSession) == .success else {
        fail("CGCompleteDisplayConfiguration", 4)
    }
    print("выставлен: " + describe(mode))
default:
    fail("неизвестная команда \(args[1])", 2)
}
