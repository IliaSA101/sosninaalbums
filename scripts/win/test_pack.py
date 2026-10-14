"""Тесты pack.py (спека 039, design §2.5): синтетические exe и .app-zip, без настоящих сборок.

Запуск: python -I scripts/win/test_pack.py  или  python -I -m unittest discover -s scripts/win -v
"""

import contextlib
import hashlib
import io
import os
import stat
import subprocess
import sys
import tempfile
import unittest
import warnings
import zipfile

HERE = os.path.dirname(os.path.abspath(__file__))
sys.path.insert(0, HERE)  # python -I не добавляет каталог скрипта

import pack  # noqa: E402

V = "1.2.3"
FOLDER = "Sosnina Albums " + V
DATE = "2026-10-20T12:00:00Z"
CHANGELOG = """# История изменений

## 1.2.4 — не выпущена

### Добавлено

- Чужая версия.

## 1.2.3 — 2026-10-20

### Добавлено

- Новая **функция** с `кодом` и [ссылкой](https://example.com/a).

### Исправлено

- Вторая строка.

## 1.2.2

- Старое.
"""


def mode(n):
    return (stat.S_IFREG | n) << 16


def make_app_zip(path, extra=(), skip=(), app="sosninaalbums.app"):
    """Zip «как от ditto»: права в external_attr, create_system 3, __MACOSX с AppleDouble, ссылка."""
    with zipfile.ZipFile(path, "w", zipfile.ZIP_DEFLATED) as z:
        def add(name, data, attr, ctype=zipfile.ZIP_DEFLATED):
            if name in skip:
                return
            zi = zipfile.ZipInfo("x", (2026, 10, 1, 9, 30, 0))
            zi.filename = name  # мимо замены «\» на «/» в конструкторе (Windows)
            zi.create_system = 3
            zi.external_attr = attr
            zi.compress_type = ctype
            z.writestr(zi, data)
        add(app + "/", b"", (stat.S_IFDIR | 0o755) << 16 | 0x10, zipfile.ZIP_STORED)
        add(app + "/Contents/Info.plist", b"<plist/>", mode(0o644))
        add(app + "/Contents/MacOS/sosninaalbums", b"\xcf\xfa\xed\xfe" * 100, mode(0o755))
        add(app + "/Contents/Resources/icon.icns", b"icns", mode(0o644))
        add(app + "/Contents/Frameworks/lib.dylib", b"dy", mode(0o755))
        add(app + "/Contents/Frameworks/lib-current.dylib", b"lib.dylib", (stat.S_IFLNK | 0o755) << 16)
        add(app + "/Contents/_CodeSignature/CodeResources", b"sig", mode(0o644))
        add("__MACOSX/", b"", (stat.S_IFDIR | 0o755) << 16 | 0x10, zipfile.ZIP_STORED)
        add("__MACOSX/._" + app, b"apple", mode(0o644))
        add("__MACOSX/" + app + "/._x", b"apple", mode(0o644))
        for name, data, attr in extra:
            add(name, data, attr)


class Base(unittest.TestCase):
    def setUp(self):
        self._tmp = tempfile.TemporaryDirectory()
        self.addCleanup(self._tmp.cleanup)
        self.dir = self._tmp.name
        self.out = os.path.join(self.dir, "out")
        self.exe = self.write("SosninaAlbums.exe", b"MZ" + b"\x00" * 5000)
        self.pdf = self.write("guide.pdf", b"%PDF-1.7 pdf")
        self.docx = self.write("guide.docx", b"PK docx")
        self.log = self.write("CHANGELOG.md", CHANGELOG.encode("utf-8"))
        self.notices = self.write("THIRD_PARTY_NOTICES.txt", "Уведомления\n".encode("utf-8"))
        self.appzip = os.path.join(self.dir, "app.zip")
        make_app_zip(self.appzip)

    def write(self, name, data):
        p = os.path.join(self.dir, name)
        with open(p, "wb") as f:
            f.write(data)
        return p

    def run_pack(self, *argv):
        """Запуск main в процессе: (код, stdout, stderr)."""
        out, err = io.StringIO(), io.StringIO()
        with contextlib.redirect_stdout(out), contextlib.redirect_stderr(err):
            code = pack.main(list(argv))
        return code, out.getvalue(), err.getvalue()

    def common(self, guide=None):
        return ["--version", V, "--guide", guide or self.pdf, "--changelog", self.log,
                "--notices", self.notices, "--out", self.out]

    def pack_windows(self, *extra, guide=None):
        code, out, err = self.run_pack("windows", "--exe", self.exe, *self.common(guide), "--date", DATE, *extra)
        self.assertEqual(code, 0, err)
        return out.strip().splitlines()[-1], out

    def pack_macos(self, *extra, guide=None):
        code, out, err = self.run_pack("macos", "--app-zip", self.appzip, *self.common(guide), "--date", DATE, *extra)
        self.assertEqual(code, 0, err)
        return out.strip().splitlines()[-1], out

    def check_code(self, path, *extra):
        return self.run_pack("check", path, *extra)[0]


class TestPack(Base):
    def test_windows_layout(self):
        path, out = self.pack_windows()
        self.assertEqual(os.path.basename(path), "SosninaAlbums-1.2.3-windows-x64.zip")
        with zipfile.ZipFile(path) as z:
            self.assertEqual(z.namelist(), [
                FOLDER + "/SosninaAlbums.exe",
                FOLDER + "/Как установить (Windows).pdf",
                FOLDER + "/Что нового.txt",
                FOLDER + "/THIRD_PARTY_NOTICES.txt",
            ])
            self.assertEqual(z.read(FOLDER + "/SosninaAlbums.exe"), b"MZ" + b"\x00" * 5000)
            self.assertEqual(z.read(FOLDER + "/Как установить (Windows).pdf"), b"%PDF-1.7 pdf")
            self.assertEqual(z.read(FOLDER + "/THIRD_PARTY_NOTICES.txt"), "Уведомления\n".encode("utf-8"))
            for i in z.infolist():
                self.assertEqual(i.create_system, 3)
                self.assertEqual(i.compress_type, zipfile.ZIP_DEFLATED)
                self.assertEqual(i.date_time, (2026, 10, 20, 12, 0, 0))
            self.assertEqual(z.getinfo(FOLDER + "/SosninaAlbums.exe").external_attr >> 16, stat.S_IFREG | 0o755)
            self.assertEqual(z.getinfo(FOLDER + "/Что нового.txt").external_attr >> 16, stat.S_IFREG | 0o644)
        # stdout: список записей, sha256, затем путь последней строкой
        lines = out.strip().splitlines()
        self.assertIn(FOLDER + "/SosninaAlbums.exe", out)
        with open(path, "rb") as f:
            self.assertIn("sha256  " + hashlib.sha256(f.read()).hexdigest(), lines[-2])
        self.assertEqual(self.check_code(path), 0)
        self.assertEqual(self.check_code(path, "--os", "windows", "--version", V), 0)

    def test_macos_layout_and_check(self):
        path, _ = self.pack_macos()
        self.assertEqual(os.path.basename(path), "SosninaAlbums-1.2.3-macos-universal.zip")
        with zipfile.ZipFile(path) as z:
            names = z.namelist()
            for n in ("Что нового.txt", "Как установить (Mac).pdf", "THIRD_PARTY_NOTICES.txt"):
                self.assertIn(FOLDER + "/" + n, names)
            self.assertIn(FOLDER + "/sosninaalbums.app/Contents/Info.plist", names)
            self.assertEqual(z.read(FOLDER + "/Что нового.txt")[:3], b"\xef\xbb\xbf")
            self.assertNotIn(b"\r", z.read(FOLDER + "/Что нового.txt"))
        self.assertEqual(self.check_code(path), 0)
        self.assertEqual(self.check_code(path, "--os", "macos", "--version", V), 0)
        # чужие ОС и версия не проходят
        self.assertEqual(self.check_code(path, "--os", "windows"), 1)
        self.assertEqual(self.check_code(path, "--version", "9.9.9"), 1)

    def test_macos_keeps_modes(self):
        path, _ = self.pack_macos()
        base = FOLDER + "/sosninaalbums.app/Contents/"
        with zipfile.ZipFile(path) as z:
            binary = z.getinfo(base + "MacOS/sosninaalbums")
            self.assertEqual(binary.external_attr >> 16, stat.S_IFREG | 0o755)
            self.assertEqual(binary.create_system, 3)
            plist = z.getinfo(base + "Info.plist")
            self.assertEqual(plist.external_attr >> 16, stat.S_IFREG | 0o644)
            self.assertEqual(plist.create_system, 3)
            # ссылка осталась ссылкой с той же целью (без распаковки данные не менялись)
            link = z.getinfo(base + "Frameworks/lib-current.dylib")
            self.assertTrue(stat.S_ISLNK(link.external_attr >> 16))
            self.assertEqual(z.read(link), b"lib.dylib")
            self.assertEqual(z.read(binary), b"\xcf\xfa\xed\xfe" * 100)
            # папка осталась папкой, подпись на месте
            d = z.getinfo(FOLDER + "/sosninaalbums.app/")
            self.assertTrue(stat.S_ISDIR(d.external_attr >> 16))
            self.assertIn(base + "_CodeSignature/CodeResources", z.namelist())
            # дата исходных записей сохраняется без --date
        path2, _ = self.run_pack_nodate()
        with zipfile.ZipFile(path2) as z:
            self.assertEqual(z.getinfo(FOLDER + "/sosninaalbums.app/Contents/Info.plist").date_time,
                             (2026, 10, 1, 9, 30, 0))

    def run_pack_nodate(self):
        out2 = os.path.join(self.dir, "out2")
        code, out, err = self.run_pack("macos", "--app-zip", self.appzip, *self.common()[:-1], out2)
        self.assertEqual(code, 0, err)
        return out.strip().splitlines()[-1], out

    def test_macos_rsrc_prefix(self):
        path, _ = self.pack_macos()
        with zipfile.ZipFile(path) as z:
            names = z.namelist()
        self.assertIn("__MACOSX/Sosnina Albums 1.2.3/sosninaalbums.app/._x", names)
        self.assertIn("__MACOSX/Sosnina Albums 1.2.3/._sosninaalbums.app", names)
        self.assertIn("__MACOSX/", names)
        self.assertFalse([n for n in names if n.startswith("__MACOSX/sosninaalbums.app")])
        # AppleDouble вне папки выпуска — отказ check
        bad = os.path.join(self.dir, "badrsrc.zip")
        with zipfile.ZipFile(path) as zin, zipfile.ZipFile(bad, "w") as zout:
            for i in zin.infolist():
                zout.writestr(i, zin.read(i))
            zi = zipfile.ZipInfo("__MACOSX/stray")
            zi.create_system, zi.external_attr = 3, mode(0o644)
            zout.writestr(zi, b"x")
        self.assertEqual(self.check_code(bad), 1)

    def test_rejects_unsafe_entries(self):
        cases = {
            "абсолютный путь": [("/abs", b"x", mode(0o644))],
            "точки в пути": [("sosninaalbums.app/a/../../b", b"x", mode(0o644))],
            "диск": [("C:/x", b"x", mode(0o644))],
            "ссылка наружу": [("sosninaalbums.app/Contents/ln", b"../../../etc", (stat.S_IFLNK | 0o755) << 16)],
            "абсолютная ссылка": [("sosninaalbums.app/Contents/ln2", b"/etc/passwd", (stat.S_IFLNK | 0o755) << 16)],
            "второй .app": [("other.app/Contents/Info.plist", b"x", mode(0o644))],
            "файл в корне": [("readme.txt", b"x", mode(0o644))],
            "повтор": [("sosninaalbums.app/Contents/Info.plist", b"y", mode(0o644))],
        }
        for label, extra in cases.items():
            with self.subTest(label):
                src = os.path.join(self.dir, "bad.zip")
                with warnings.catch_warnings():
                    warnings.simplefilter("ignore")  # zipfile предупреждает о повторе имени — он здесь и нужен
                    make_app_zip(src, extra)
                code, out, err = self.run_pack("macos", "--app-zip", src, *self.common())
                self.assertEqual(code, 1, ascii(label) + " " + ascii(out))
                self.assertTrue(err.strip(), label)
                self.assertFalse(os.path.exists(os.path.join(self.out, "SosninaAlbums-1.2.3-macos-universal.zip")), label)
                self.assertFalse(os.path.exists(os.path.join(self.out, "SosninaAlbums-1.2.3-macos-universal.zip.part")))
        # допустимая ссылка внутри .app принимается (граница: цель в соседней папке)
        src = os.path.join(self.dir, "ok.zip")
        make_app_zip(src, [("sosninaalbums.app/Contents/Frameworks/up", b"../Resources/icon.icns", (stat.S_IFLNK | 0o755) << 16)])
        self.assertEqual(self.run_pack("macos", "--app-zip", src, *self.common())[0], 0)

    def test_check_name_rules(self):
        # обратная косая в zip-записи на Windows подменяется самим zipfile, поэтому правило — прямым вызовом
        for bad in ("a\\b", "/abs", "a/../b", "a//b", "./a", "C:/x", "a\x00b", ""):
            with self.subTest(bad), self.assertRaises(pack.PackError):
                pack.check_name(bad)
        self.assertEqual(pack.check_name("a/b/"), ["a", "b"])
        self.assertEqual(pack.check_name("Как установить (Mac).pdf"), ["Как установить (Mac).pdf"])

    def test_rejects_app_without_binary(self):
        for label, skip in (("нет plist", ("sosninaalbums.app/Contents/Info.plist",)),
                            ("нет бинаря", ("sosninaalbums.app/Contents/MacOS/sosninaalbums",))):
            with self.subTest(label):
                src = os.path.join(self.dir, "x.zip")
                make_app_zip(src, skip=skip)
                self.assertEqual(self.run_pack("macos", "--app-zip", src, *self.common())[0], 1)
        # бинарь без бита исполнения
        src = os.path.join(self.dir, "noexec.zip")
        make_app_zip(src, [("sosninaalbums.app/Contents/MacOS/tool", b"x", mode(0o644))])
        self.assertEqual(self.run_pack("macos", "--app-zip", src, *self.common())[0], 1)
        # не-zip на входе
        junk = self.write("junk.zip", b"not a zip")
        self.assertEqual(self.run_pack("macos", "--app-zip", junk, *self.common())[0], 1)

    def test_rejects_missing_inputs(self):
        missing = os.path.join(self.dir, "нет-файла")
        base = self.common()
        self.assertEqual(self.run_pack("windows", "--exe", missing, *base)[0], 1)
        self.assertEqual(self.run_pack("macos", "--app-zip", missing, *base)[0], 1)
        for flag in ("--guide", "--changelog", "--notices"):
            argv = list(self.common())
            argv[argv.index(flag) + 1] = missing
            code, _, err = self.run_pack("windows", "--exe", self.exe, *argv)
            self.assertEqual(code, 1, flag)
            self.assertIn("нет входного файла", err)
        # пустой exe не отказ, а «guide» неверного типа — отказ
        txt = self.write("guide.txt", b"x")
        self.assertEqual(self.run_pack("windows", "--exe", self.exe, *self.common(txt))[0], 1)
        # после отказов в out нет архива и .part
        self.assertEqual(os.listdir(self.out) if os.path.isdir(self.out) else [], [])

    def test_bad_arguments(self):
        err = io.StringIO()
        with contextlib.redirect_stderr(err):
            self.assertEqual(pack.main([]), 2)
            self.assertEqual(pack.main(["windows"]), 2)
            self.assertEqual(pack.main(["windows", "--exe", self.exe, "--version", "../x", "--guide", self.pdf,
                                        "--changelog", self.log, "--notices", self.notices, "--out", self.out]), 2)
            self.assertEqual(pack.main(["windows", "--exe", self.exe, *self.common(), "--date", "вчера"]), 2)
            self.assertEqual(pack.main(["check", "z.zip", "--os", "linux"]), 2)

    def test_utf8_flag_on_cyrillic(self):
        for path in (self.pack_windows()[0], self.pack_macos()[0]):
            with zipfile.ZipFile(path) as z:
                found = 0
                for i in z.infolist():
                    if any(ord(c) > 127 for c in i.filename):
                        found += 1
                        self.assertTrue(i.flag_bits & 0x800, i.filename)
                    else:
                        self.assertFalse(i.flag_bits & 0x800, i.filename)
                self.assertGreaterEqual(found, 2)  # «Как установить…» и «Что нового.txt»
            # имя читается обратно как есть
            with zipfile.ZipFile(path) as z:
                self.assertIn(FOLDER + "/Что нового.txt", z.namelist())

    def test_changes_txt(self):
        path, _ = self.pack_windows()
        with zipfile.ZipFile(path) as z:
            raw = z.read(FOLDER + "/Что нового.txt")
        self.assertEqual(raw[:3], b"\xef\xbb\xbf")
        text = raw[3:].decode("utf-8")
        self.assertTrue(text.endswith("\r\n"))
        self.assertNotIn("\n", text.replace("\r\n", ""))  # только CRLF
        lines = text.split("\r\n")
        self.assertEqual(lines[0], "Sosnina Albums 1.2.3 — что нового")
        self.assertEqual(lines[1], "")
        self.assertIn("Добавлено:", lines)
        self.assertIn("Исправлено:", lines)
        self.assertIn("- Новая функция с кодом и ссылкой (https://example.com/a).", lines)
        self.assertIn("- Вторая строка.", lines)
        self.assertNotIn("Чужая версия", text)
        self.assertNotIn("Старое", text)
        for bad in ("**", "`", "###", "]("):
            self.assertNotIn(bad, text)
        self.assertEqual(lines[-2], "https://github.com/IliaSA101/sosninaalbums/blob/v1.2.3/CHANGELOG.md")
        # Mac — LF
        macpath, _ = self.pack_macos()
        with zipfile.ZipFile(macpath) as z:
            mac = z.read(FOLDER + "/Что нового.txt")[3:].decode("utf-8")
        self.assertNotIn("\r", mac)
        self.assertEqual(mac.replace("\n", "\r\n"), text)

    def test_changes_section_edges(self):
        # раздел до конца файла без следующего «## » и версия — точное совпадение, а не префикс
        self.log = self.write("CHANGELOG.md", "## 1.2.30\n- не та\n\n## 1.2.3\n- последняя\n".encode("utf-8"))
        path, _ = self.pack_windows()
        with zipfile.ZipFile(path) as z:
            text = z.read(FOLDER + "/Что нового.txt").decode("utf-8-sig")
        self.assertIn("- последняя", text)
        self.assertNotIn("не та", text)
        # пустой раздел — как отсутствующий
        self.log = self.write("CHANGELOG.md", "## 1.2.3\n\n\n## 1.2.2\n- x\n".encode("utf-8"))
        _, out = self.pack_windows()
        self.assertIn("::warning::", out)

    def test_changes_missing_section(self):
        self.log = self.write("CHANGELOG.md", "# Журнал\n\n## 0.1.0\n- старое\n".encode("utf-8"))
        path, out = self.pack_windows()
        self.assertIn("::warning::", out)
        with zipfile.ZipFile(path) as z:
            text = z.read(FOLDER + "/Что нового.txt").decode("utf-8-sig")
        self.assertEqual(text.splitlines(), [
            "Sosnina Albums 1.2.3 — что нового",
            "https://github.com/IliaSA101/sosninaalbums/blob/v1.2.3/CHANGELOG.md",
        ])
        self.assertEqual(self.check_code(path), 0)
        # пустой CHANGELOG — то же
        self.log = self.write("CHANGELOG.md", b"")
        self.pack_windows()

    def test_guide_docx_fallback(self):
        path, out = self.pack_windows(guide=self.docx)
        self.assertIn("::warning::", out)
        with zipfile.ZipFile(path) as z:
            self.assertIn(FOLDER + "/Как установить (Windows).docx", z.namelist())
            self.assertNotIn(FOLDER + "/Как установить (Windows).pdf", z.namelist())
        self.assertEqual(self.check_code(path), 0)
        mac, out = self.pack_macos(guide=self.docx)
        self.assertIn("::warning::", out)
        with zipfile.ZipFile(mac) as z:
            self.assertIn(FOLDER + "/Как установить (Mac).docx", z.namelist())
        self.assertEqual(self.check_code(mac), 0)
        # pdf — без предупреждения
        _, out = self.pack_windows()
        self.assertNotIn("::warning::", out)

    def test_check_packaged_vs_bare(self):
        packaged, _ = self.pack_windows()
        self.assertEqual(self.check_code(packaged), 0)
        # «голый» Windows: exe в корне
        bare_exe = os.path.join(self.dir, "bare.zip")
        with zipfile.ZipFile(bare_exe, "w") as z:
            z.writestr("SosninaAlbums.exe", b"MZ")
        self.assertEqual(self.check_code(bare_exe), 3)
        self.assertEqual(self.check_code(bare_exe, "--os", "windows"), 3)
        self.assertEqual(self.check_code(bare_exe, "--os", "macos"), 1)
        # «голый» Mac: .app в корне (+ __MACOSX)
        self.assertEqual(self.check_code(self.appzip), 3)
        self.assertEqual(self.check_code(self.appzip, "--os", "macos"), 3)
        # третье: чужие файлы, лишний элемент, два .app, не zip, пустой
        extra = os.path.join(self.dir, "extra.zip")
        with zipfile.ZipFile(packaged) as zin, zipfile.ZipFile(extra, "w") as zout:
            for i in zin.infolist():
                zout.writestr(i, zin.read(i))
            zout.writestr(FOLDER + "/lishnee.txt", b"x")
        self.assertEqual(self.check_code(extra), 1)
        noguide = os.path.join(self.dir, "noguide.zip")
        with zipfile.ZipFile(packaged) as zin, zipfile.ZipFile(noguide, "w") as zout:
            for i in zin.infolist():
                if "Как установить" not in i.filename:
                    zout.writestr(i, zin.read(i))
        self.assertEqual(self.check_code(noguide), 1)
        two = os.path.join(self.dir, "two.zip")
        with zipfile.ZipFile(two, "w") as z:
            z.writestr("a/x", b"1")
            z.writestr("b/y", b"2")
        self.assertEqual(self.check_code(two), 1)
        self.assertEqual(self.check_code(self.write("junk.zip", b"zzz")), 1)
        empty = os.path.join(self.dir, "empty.zip")
        zipfile.ZipFile(empty, "w").close()
        self.assertEqual(self.check_code(empty), 1)
        self.assertEqual(self.check_code(os.path.join(self.dir, "nope.zip")), 1)
        # Mac без бита исполнения у бинаря в упакованном архиве
        mac, _ = self.pack_macos()
        noexec = os.path.join(self.dir, "noexec.zip")
        with zipfile.ZipFile(mac) as zin, zipfile.ZipFile(noexec, "w") as zout:
            for i in zin.infolist():
                if i.filename.endswith("MacOS/sosninaalbums"):
                    i.external_attr = mode(0o644)
                zout.writestr(i, zin.read(i))
        self.assertEqual(self.check_code(noexec), 1)
        # Mac-архив с чужим AppleDouble-ссылкой наружу
        evil = os.path.join(self.dir, "evil.zip")
        with zipfile.ZipFile(mac) as zin, zipfile.ZipFile(evil, "w") as zout:
            for i in zin.infolist():
                zout.writestr(i, zin.read(i))
            zi = zipfile.ZipInfo(FOLDER + "/sosninaalbums.app/Contents/ln")
            zi.create_system, zi.external_attr = 3, (stat.S_IFLNK | 0o755) << 16
            zout.writestr(zi, b"../../../x")
        self.assertEqual(self.check_code(evil), 1)

    def test_deterministic(self):
        for kind in ("windows", "macos"):
            with self.subTest(kind):
                digests = []
                for n in (1, 2):
                    self.out = os.path.join(self.dir, "run%d" % n)
                    path, _ = (self.pack_windows if kind == "windows" else self.pack_macos)()
                    with open(path, "rb") as f:
                        digests.append(hashlib.sha256(f.read()).hexdigest())
                self.assertEqual(digests[0], digests[1])
        # другое время — другие байты (дата реально влияет)
        self.out = os.path.join(self.dir, "run3")
        path, _ = self.pack_windows()
        self.out = os.path.join(self.dir, "run4")
        code, out, _ = self.run_pack("windows", "--exe", self.exe, *self.common(), "--date", "2026-10-21T08:00:00+03:00")
        self.assertEqual(code, 0)
        with open(path, "rb") as a, open(out.strip().splitlines()[-1], "rb") as b:
            self.assertNotEqual(a.read(), b.read())

    def test_overwrites_existing_archive(self):
        path, _ = self.pack_windows()
        with open(path, "wb") as f:
            f.write(b"old")
        path2, _ = self.pack_windows()
        self.assertEqual(path, path2)
        self.assertEqual(self.check_code(path2), 0)
        self.assertEqual(sorted(os.listdir(self.out)), ["SosninaAlbums-1.2.3-windows-x64.zip"])


class TestCli(Base):
    """Реальный запуск `python -I pack.py` (как в workflow): кодировка stdout, коды выхода."""

    def run_cli(self, *argv):
        r = subprocess.run([sys.executable, "-I", os.path.join(HERE, "pack.py"), *argv],
                           capture_output=True, encoding="utf-8")
        return r.returncode, r.stdout, r.stderr

    def test_cli_roundtrip(self):
        code, out, err = self.run_cli("windows", "--exe", self.exe, *self.common(), "--date", DATE)
        self.assertEqual(code, 0, err)
        path = out.strip().splitlines()[-1]
        self.assertTrue(os.path.isfile(path))
        self.assertIn("Что нового.txt", out)  # кириллица в stdout не падает
        self.assertEqual(self.run_cli("check", path, "--version", V)[0], 0)
        self.assertEqual(self.run_cli("check", path, "--version", "0.0.1")[0], 1)
        self.assertEqual(self.run_cli("windows")[0], 2)


if __name__ == "__main__":
    unittest.main()
