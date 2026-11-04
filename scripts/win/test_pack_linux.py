"""Тесты pack.py: LICENSE и NOTICE.md в архивах и tar.gz для Linux (спека 101, design §3.3).

Запуск: python -I scripts/win/test_pack_linux.py  или  python -I -m unittest discover -s scripts/win -v
"""

import hashlib
import io
import os
import stat
import sys
import tarfile
import time
import unittest
from unittest import mock
import zipfile

HERE = os.path.dirname(os.path.abspath(__file__))
sys.path.insert(0, HERE)  # python -I не добавляет каталог скрипта

from test_pack import DATE, FOLDER, V, Base, pack  # noqa: E402


class TestLicenseAndLinux(Base):
    def test_license_in_archives(self):
        """101-3: LICENSE и NOTICE.md — внутри каждого архива, дословно; без них check отказывает."""
        paths = {"windows": self.pack_windows()[0], "macos": self.pack_macos()[0]}
        for kind, path in paths.items():
            with self.subTest(kind), zipfile.ZipFile(path) as z:
                self.assertEqual(z.read(FOLDER + "/LICENSE"), b"GNU GENERAL PUBLIC LICENSE\nVersion 3\n")
                self.assertEqual(z.read(FOLDER + "/NOTICE.md"), "Условие 7(b): имя и логотип\n".encode("utf-8"))
                self.assertEqual(z.getinfo(FOLDER + "/LICENSE").external_attr >> 16, stat.S_IFREG | 0o644)
            for gone in ("LICENSE", "NOTICE.md", "THIRD_PARTY_NOTICES.txt"):
                cut = os.path.join(self.dir, kind + "-no-" + gone + ".zip")
                with zipfile.ZipFile(path) as zin, zipfile.ZipFile(cut, "w") as zout:
                    for i in zin.infolist():
                        if i.filename != FOLDER + "/" + gone:
                            zout.writestr(i, zin.read(i))
                self.assertEqual(self.check_code(cut), 1, kind + " без " + gone)
        # tar.gz Linux — те же файлы
        with tarfile.open(self.pack_linux()[0]) as tf:
            self.assertEqual(tf.extractfile(FOLDER + "/LICENSE").read(), b"GNU GENERAL PUBLIC LICENSE\nVersion 3\n")
            self.assertIn(FOLDER + "/NOTICE.md", tf.getnames())

    def test_empty_license_files_are_kept(self):
        """Край: пустые LICENSE/NOTICE.md не отказ упаковщика (проверка текста — не его дело), архив валиден."""
        self.license = self.write("LICENSE", b"")
        self.notice = self.write("NOTICE.md", b"")
        for pk in (self.pack_windows, self.pack_linux):
            path, _ = pk()
            self.assertEqual(self.check_code(path), 0)

    def test_linux_tar_layout(self):
        path, out = self.pack_linux()
        self.assertEqual(os.path.basename(path), "SosninaAlbums-1.2.3-linux-amd64.tar.gz")
        with tarfile.open(path) as tf:
            ms = tf.getmembers()
            self.assertEqual([m.name for m in ms], [
                FOLDER,
                FOLDER + "/sosninaalbums",
                FOLDER + "/sosninaalbums.png",
                FOLDER + "/Как установить (Linux).pdf",
                FOLDER + "/Что нового.txt",
                FOLDER + "/THIRD_PARTY_NOTICES.txt",
                FOLDER + "/LICENSE",
                FOLDER + "/NOTICE.md",
            ])
            self.assertTrue(ms[0].isdir())
            self.assertEqual(ms[0].mode, 0o755)
            for m in ms:
                self.assertEqual((m.uid, m.gid, m.uname, m.gname), (0, 0, "root", "root"))
                self.assertEqual(m.mtime, 1792497600)  # 2026-10-20T12:00:00Z
            by = {m.name: m for m in ms}
            self.assertEqual(by[FOLDER + "/sosninaalbums"].mode, 0o755)
            for n in ("sosninaalbums.png", "Что нового.txt", "LICENSE", "NOTICE.md", "THIRD_PARTY_NOTICES.txt"):
                self.assertEqual(by[FOLDER + "/" + n].mode, 0o644, n)
            self.assertEqual(tf.extractfile(FOLDER + "/sosninaalbums").read(), b"\x7fELF" + b"\x00" * 5000)
            self.assertEqual(tf.extractfile(FOLDER + "/sosninaalbums.png").read(), b"\x89PNG icon")
            self.assertEqual(tf.extractfile(FOLDER + "/Что нового.txt").read()[:3], b"\xef\xbb\xbf")
        self.assertIn(FOLDER + "/LICENSE", out)
        self.assertEqual(self.check_code(path), 0)
        self.assertEqual(self.check_code(path, "--os", "linux", "--version", V), 0)
        self.assertEqual(self.check_code(path, "--version", "9.9.9"), 1)
        self.assertEqual(self.check_code(path, "--os", "windows"), 1)
        # zip под --os linux — отказ
        self.assertEqual(self.check_code(self.pack_windows()[0], "--os", "linux"), 1)

    def test_linux_default_icon_and_docx(self):
        """Без --icon берётся build/appicon.png репозитория; docx вместо pdf — с предупреждением."""
        code, out, err = self.run_pack("linux", "--bin", self.binary, *self.common(self.docx), "--date", DATE)
        self.assertEqual(code, 0, err)
        self.assertIn("Инструкция в архиве — docx", out)
        with tarfile.open(out.strip().splitlines()[-1]) as tf:
            with open(pack.DEFAULT_ICON, "rb") as f:
                self.assertEqual(tf.extractfile(FOLDER + "/sosninaalbums.png").read(), f.read())
            self.assertIn(FOLDER + "/Как установить (Linux).docx", tf.getnames())

    def test_linux_tar_reproducible(self):
        digests = []
        for n in (1, 2):
            self.out = os.path.join(self.dir, "lrun%d" % n)
            with open(self.pack_linux()[0], "rb") as f:
                digests.append(hashlib.sha256(f.read()).hexdigest())
        self.assertEqual(digests[0], digests[1])
        # другое время — другие байты
        self.out = os.path.join(self.dir, "lrun3")
        code, out, _ = self.run_pack("linux", "--bin", self.binary, "--icon", self.icon, *self.common(),
                                     "--date", "2026-10-21T08:00:00+03:00")
        self.assertEqual(code, 0)
        with open(out.strip().splitlines()[-1], "rb") as f:
            self.assertNotEqual(hashlib.sha256(f.read()).hexdigest(), digests[0])

    def test_linux_tar_mtime_is_utc_without_date(self):
        # местное время «UTC+5» (и «UTC-7»): mtime не должен уезжать в будущее или прошлое
        for shift in (5 * 3600, -7 * 3600):
            real = time.time()
            with mock.patch.object(time, "localtime", lambda: time.gmtime(real + shift)):
                code, out, err = self.run_pack("linux", "--bin", self.binary, "--icon", self.icon, *self.common())
            self.assertEqual(code, 0, err)
            with tarfile.open(out.strip().splitlines()[-1]) as tf:
                mtimes = {m.mtime for m in tf.getmembers()}
            self.assertEqual(len(mtimes), 1)  # у всех записей один mtime
            self.assertLess(abs(mtimes.pop() - real), 120, shift)

    def test_zip_date_stays_local_without_date(self):
        # zip хранит местное время DOS — как было
        real = time.time()
        with mock.patch.object(time, "localtime", lambda: time.gmtime(real + 5 * 3600)):
            code, out, err = self.run_pack("windows", "--exe", self.exe, *self.common())
        self.assertEqual(code, 0, err)
        with zipfile.ZipFile(out.strip().splitlines()[-1]) as z:
            y, mo, d, h, mi, _ = z.infolist()[0].date_time
        self.assertEqual((y, mo, d, h, mi), time.gmtime(real + 5 * 3600)[:5])

    def test_linux_missing_inputs(self):
        missing = os.path.join(self.dir, "нет-файла")
        self.assertEqual(self.run_pack("linux", "--bin", missing, *self.common())[0], 1)
        self.assertEqual(self.run_pack("linux", "--bin", self.binary, "--icon", missing, *self.common())[0], 1)
        self.assertEqual(os.listdir(self.out) if os.path.isdir(self.out) else [], [])

    def test_linux_check_rejects_unsafe(self):
        good, _ = self.pack_linux()

        def rebuild(name, edit):
            """Копия good: edit(список (TarInfo, байты)) меняет записи до записи в новый tar.gz."""
            with tarfile.open(good) as tin:
                items = [(m, tin.extractfile(m).read() if m.isfile() else b"") for m in tin.getmembers()]
            items = edit(items) or items
            path = os.path.join(self.dir, name + ".tar.gz")
            with tarfile.open(path, "w:gz") as tout:
                for m, data in items:
                    m.size = len(data) if m.isreg() else 0
                    tout.addfile(m, io.BytesIO(data) if m.isreg() else None)
            return path

        def plain(name, data=b"x"):
            ti = tarfile.TarInfo(name)
            ti.size, ti.mode = len(data), 0o644
            return ti, data

        def link(items):
            ti = tarfile.TarInfo(FOLDER + "/ln")
            ti.type, ti.linkname, ti.mode = tarfile.SYMTYPE, "/etc/passwd", 0o777
            return items + [(ti, b"")]

        def setmode(suffix, mode):
            def edit(items):
                for m, _ in items:
                    if m.name.endswith(suffix):
                        m.mode = mode
            return edit

        cases = {
            "link": link,
            "dotdot": lambda items: items + [plain(FOLDER + "/../evil")],
            "absolute": lambda items: items + [plain("/etc/evil")],
            "extra": lambda items: items + [plain(FOLDER + "/lishnee.txt")],
            "noexec": setmode("/sosninaalbums", 0o644),
            "setuid": setmode("/sosninaalbums", 0o4755),
            "writable": setmode("/LICENSE", 0o666),
            "execlic": setmode("/LICENSE", 0o755),
            "nolicense": lambda items: [it for it in items if not it[0].name.endswith("/LICENSE")],
            "nofolder": lambda items: [it for it in items if it[0].name != FOLDER],
            "twofolders": lambda items: items + [plain("Other/x")],
            "dup": lambda items: items + [items[-1]],
        }
        for name, edit in cases.items():
            with self.subTest(name):
                self.assertEqual(self.check_code(rebuild(name, edit), "--os", "linux"), 1)
        # контроль: пересборка без правок проходит (тест сам не ломает архив)
        self.assertEqual(self.check_code(rebuild("same", lambda items: None), "--os", "linux"), 0)
        junk = self.write("junk.tar.gz", b"\x1f\x8bnot gzip")
        self.assertEqual(self.check_code(junk), 1)
        self.assertEqual(self.check_code(self.write("zzz.tar.gz", b"zzz"), "--os", "linux"), 1)
        self.assertEqual(self.check_code(os.path.join(self.dir, "nope.tar.gz"), "--os", "linux"), 1)


if __name__ == "__main__":
    unittest.main()
