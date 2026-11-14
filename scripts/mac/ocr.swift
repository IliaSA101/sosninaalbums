// Распознавание текста на снимке экрана (Vision) — запасной путь, если кнопку не видно в доступности
// (029 design §3.3, прогон 2: «Все равно открыть» в «Настройках»).
// Сборка один раз: swiftc -O ocr.swift -o ocr; запасной путь — swift ocr.swift …
//   ocr find <png> <надпись>…  — «x y w h<TAB>строка» в точках главного экрана (верх-лево), самые
//                                 короткие строки первыми (кнопка, а не абзац с её названием);
//   ocr click <x> <y>          — щелчок мыши в точку (точки экрана).
// Подпись сравнивается по «содержит»: ё=е, неразрывный пробел=пробел, без учёта регистра.
import CoreGraphics
import Foundation
import ImageIO
import Vision

func norm(_ s: String) -> String {
    return s.replacingOccurrences(of: "ё", with: "е").replacingOccurrences(of: "Ё", with: "е")
        .replacingOccurrences(of: "\u{00A0}", with: " ").replacingOccurrences(of: "\u{202F}", with: " ")
        .lowercased()
}

func fail(_ msg: String, _ code: Int32) -> Never {
    FileHandle.standardError.write((msg + "\n").data(using: .utf8)!)
    exit(code)
}

let args = CommandLine.arguments

if args.count == 4, args[1] == "click", let x = Double(args[2]), let y = Double(args[3]) {
    let p = CGPoint(x: x, y: y)
    CGEvent(
        mouseEventSource: nil, mouseType: .mouseMoved, mouseCursorPosition: p, mouseButton: .left
    )?.post(tap: .cghidEventTap)
    usleep(400_000)
    for t in [CGEventType.leftMouseDown, CGEventType.leftMouseUp] {
        CGEvent(mouseEventSource: nil, mouseType: t, mouseCursorPosition: p, mouseButton: .left)?
            .post(tap: .cghidEventTap)
        usleep(80_000)
    }
    exit(0)
}

guard args.count >= 4, args[1] == "find" else {
    fail("использование: ocr find <png> <надпись>… | ocr click <x> <y>", 2)
}

let url = URL(fileURLWithPath: args[2]) as CFURL
guard let src = CGImageSourceCreateWithURL(url, nil),
    let img = CGImageSourceCreateImageAtIndex(src, 0, nil)
else {
    fail("снимок не открылся: \(args[2])", 1)
}
let wanted = args[3...].map(norm)

// Снимок — в пикселях, экран — в точках: масштаб Retina из ширины снимка
let screenW = Double(CGDisplayBounds(CGMainDisplayID()).width)
let scale = screenW > 0 ? Double(img.width) / screenW : 1.0
let imgW = Double(img.width)
let imgH = Double(img.height)

let req = VNRecognizeTextRequest()
req.recognitionLevel = .accurate
req.recognitionLanguages = ["ru-RU", "en-US"]
req.usesLanguageCorrection = false
do {
    try VNImageRequestHandler(cgImage: img, options: [:]).perform([req])
} catch {
    fail("Vision: \(error)", 1)
}

var hits: [(len: Int, line: String)] = []
for obs in req.results ?? [] {
    guard let cand = obs.topCandidates(1).first else { continue }
    let text = cand.string
    // Нормализация заменяет знак на знак, поэтому позиции в обеих строках совпадают
    let nt = norm(text)
    for w in wanted {
        guard let r = nt.range(of: w) else { continue }
        let lo = text.index(text.startIndex, offsetBy: nt.distance(from: nt.startIndex, to: r.lowerBound))
        let hi = text.index(lo, offsetBy: w.count)
        // Рамка подстроки; если Vision её не даёт — рамка всей строки
        let bb = (try? cand.boundingBox(for: lo..<hi))?.boundingBox ?? obs.boundingBox
        let x = bb.minX * imgW / scale
        let y = (1 - bb.maxY) * imgH / scale
        let bw = bb.width * imgW / scale
        let bh = bb.height * imgH / scale
        let line = "\(Int(x.rounded())) \(Int(y.rounded())) \(Int(bw.rounded())) \(Int(bh.rounded()))\t\(text)"
        hits.append((text.count, line))
        break
    }
}
hits.sort { $0.len < $1.len }
for h in hits { print(h.line) }
