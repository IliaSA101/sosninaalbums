"""Тесты installer.sh и installer.nsi (спека 111, design §6): сборка makensis -WX, воспроизводимость, контракт скрипта.

Запуск: python -I -m unittest discover -s scripts/win -v. Без makensis сборочные тесты пропускаются;
в CI (переменная CI) его отсутствие — провал: иначе проверка молча ничего бы не проверяла.
"""

import os
import re
import shutil
import struct
import subprocess
import sys
import tempfile
import unittest

HERE = os.path.dirname(os.path.abspath(__file__))
SH = os.path.join(HERE, "installer.sh")
NSI = os.path.join(HERE, "installer.nsi")
HAVE_MAKENSIS = shutil.which("makensis") is not None
BASH = shutil.which("bash")
try:
    import PIL  # noqa: F401
    HAVE_PIL = True
except ImportError:
    HAVE_PIL = False


def read_nsi():
    with open(NSI, encoding="utf-8") as f:
        return f.read()


class TestScript(unittest.TestCase):
    """Контракт installer.nsi без сборки: то, что win-check потом проверяет на Windows."""

    def setUp(self):
        self.nsi = read_nsi()

    def test_exit_codes(self):
        # коды 3, 4, 5, 6, 64 — контракт с installer-check.ps1 и программой (design §6)
        for code in (3, 4, 5, 6, 64):
            self.assertRegex(self.nsi, r"SetErrorLevel %d\b" % code)

    def test_user_level_no_admin(self):
        self.assertIn("RequestExecutionLevel user", self.nsi)
        self.assertNotIn("MUI_PAGE_DIRECTORY", self.nsi)  # папка постоянная, выбора нет

    def test_recursive_delete_only_updates_folder(self):
        # RMDir /r по $INSTDIR стёр бы чужую папку при /D=: удаление строго поимённо
        rec = [ln for ln in self.nsi.splitlines() if re.match(r"\s*RMDir /r\b", ln)]
        self.assertEqual(len(rec), 1, rec)
        self.assertIn("SosninaAlbums\\Updates", rec[0])
        self.assertNotIn("$INSTDIR", rec[0])

    def test_registry_contract(self):
        # ключи и ProgID — общий контракт со спекой 112
        for needle in ('"SosninaAlbums.Album"', "AppVersion", "InstallDir", '"Version"', "OpenWithProgids",
                       "QuietUninstallString", 'shell\\open\\command'):
            self.assertIn(needle, self.nsi)
        self.assertNotIn("HKLM", self.nsi)  # без администратора — только HKCU

    def test_replace_before_registry(self):
        # версия в реестре — только после успешной замены exe (сбой замены: прежняя версия цела)
        self.assertLess(self.nsi.index("Call ReplaceExe"), self.nsi.index('"DisplayVersion" "${VERSION}"'))
        for needle in ("${EXE}.old", "Abort"):
            self.assertIn(needle, self.nsi)

    def test_assoc_respects_user_choice(self):
        # тихое обновление не перетирает выбор пользователя: отметка AssocOff и чужой ProgID
        self.assertIn('"AssocOff"', self.nsi)
        write = self.nsi.index('WriteRegStr HKCU "Software\\Classes\\.salbum" "" "${PROGID}"')
        self.assertLess(self.nsi.index('"AssocOff"'), write)
        self.assertLess(self.nsi.index('ReadRegStr $R0 HKCU "Software\\Classes\\.salbum"'), write)

    def test_mutex_matches_instance_id(self):
        # замок Wails строится из instanceID: смена значения без правки установщика сломает ожидание закрытия
        mutex = re.search(r'!define MUTEX "wails-app-([0-9a-f-]{36})sim"', self.nsi)
        self.assertIsNotNone(mutex)
        with open(os.path.join(HERE, "..", "..", "internal", "app", "app_test.go"), encoding="utf-8") as f:
            self.assertIn('"%s"' % mutex.group(1), f.read())

    def test_wizard_texts(self):
        # первая страница: свой текст (что ставится, куда, без администратора), не стандартный текст NSIS
        self.assertIn("%LOCALAPPDATA%\\Programs\\${PRODUCT}", self.nsi)
        self.assertIn("Права администратора не нужны", self.nsi)
        self.assertIn('!define MUI_WELCOMEPAGE_TEXT "$WelcomeText"', self.nsi)
        self.assertIn('!define MUI_WELCOMEFINISHPAGE_BITMAP "${WIZARD}"', self.nsi)

    def test_same_version_text(self):
        # повторный запуск той же версии: текст первой страницы говорит, что она уже стоит; тихо — код 4 как прежде
        same = self.nsi.index("${ElseIf} $R3 == 0")
        tail = self.nsi[same:self.nsi.index("FunctionEnd", same)]
        self.assertIn("SetErrorLevel 4", tail)
        self.assertLess(tail.index("SetErrorLevel 4"), tail.index("StrCpy $WelcomeText"))
        self.assertIn("уже установлена", tail)
        self.assertIn("«Установить» — переустановит её (настройки и альбомы не тронутся)", tail)

    def test_retry_button_name(self):
        # кнопка системного MessageBox называется «Повтор», не «Повторить»
        wait = re.search(r'MessageBox MB_RETRYCANCEL[^\n]*', self.nsi).group(0)
        self.assertIn("«Повтор»", wait)
        self.assertNotIn("«Повторить»", self.nsi)

    def test_silent_wait_gives_code_5(self):
        # /S при открытой программе: 120 шагов по 0,5 с, затем MessageBox молча отвечает «Отмена» (/SD IDCANCEL)
        # и Quit с кодом 5 — без окна и без зависания
        self.assertIn("StrCpy $R8 120", self.nsi)
        self.assertIn("Sleep 500", self.nsi)
        wait = re.search(r"MessageBox MB_RETRYCANCEL[^\n]*", self.nsi)
        self.assertIn("/SD IDCANCEL IDRETRY retry", wait.group(0))
        after = self.nsi[wait.end():]
        self.assertRegex(after[:60], r"\s*SetErrorLevel 5\s+Quit")

    def test_wizard_bmp(self):
        # картинки мастера в 2x (при DPI 125-200 % NSIS сжимает, а не растягивает): BMP 24 бита,
        # пропорции 1x: 164x314 слева на первой и последней страницах, 57x57 справа в шапке
        for name, size in (("wizard.bmp", (164 * 2, 314 * 2)), ("header.bmp", (57 * 2, 57 * 2))):
            path = os.path.join(HERE, name)
            self.assertLess(os.path.getsize(path), 700_000, name)
            with open(path, "rb") as f:
                head = f.read(30)
            self.assertEqual(head[:2], b"BM")
            width, height, planes, bits = struct.unpack("<iiHH", head[18:30])
            self.assertEqual((width, abs(height), bits), (*size, 24), name)

    def test_hidpi_images(self):
        # чёткость при масштабе экрана: программа DPI-aware, картинки сжимаются с сохранением пропорций,
        # справа в шапке — своя картинка вместо маленького значка установщика
        for needle in ("ManifestDPIAware true",
                       "!define MUI_WELCOMEFINISHPAGE_BITMAP_STRETCH AspectFitHeight",
                       "!define MUI_HEADERIMAGE\n",
                       "!define MUI_HEADERIMAGE_RIGHT",
                       '!define MUI_HEADERIMAGE_BITMAP "${HEADER}"',
                       "!define MUI_HEADERIMAGE_BITMAP_STRETCH AspectFitHeight"):
            self.assertIn(needle, self.nsi)

    def test_no_straight_quotes_in_visible_text(self):
        # стандартные русские строки NSIS пишут «Готово», «Удалить» в прямых кавычках: страницы мастера и
        # удаления, которые мы показываем, берут свои тексты в «ёлочках»
        for name in ("MUI_FINISHPAGE_TEXT", "MUI_UNCONFIRMPAGE_TEXT_TOP"):
            line = re.search(r"^!define %s (.*)$" % name, self.nsi, re.M)
            self.assertIsNotNone(line, name)
            self.assertNotIn('$\\"', line.group(1), name)
            self.assertRegex(line.group(1), "«[^»]+»", name)
        # а в самом скрипте видимый текст не содержит экранированных кавычек
        self.assertNotIn('$\\"', self.nsi)
        # какие страницы показываем: если добавится страница со стандартным текстом с кавычками
        # (лицензия, папка, меню Пуск), её текст надо переопределить — тест напомнит
        for page in ("MUI_PAGE_LICENSE", "MUI_PAGE_COMPONENTS", "MUI_PAGE_STARTMENU", "MUI_UNPAGE_WELCOME",
                     "MUI_UNPAGE_FINISH"):
            self.assertNotIn(page, self.nsi, page + ": переопределите её русские тексты с прямыми кавычками")

    def test_uninstall_removes_empty_assoc_keys(self):
        # после удаления не остаётся пустых HKCU\Software\Classes\.salbum и OpenWithProgids; чужое не трогаем
        un = self.nsi[self.nsi.index('Section "Uninstall"'):]
        drop = un.index('DeleteRegValue HKCU "Software\\Classes\\.salbum\\OpenWithProgids" "${PROGID}"')
        inner = un.index('DeleteRegKey /ifempty HKCU "Software\\Classes\\.salbum\\OpenWithProgids"')
        outer = un.index('DeleteRegKey /ifempty HKCU "Software\\Classes\\.salbum"\n')
        self.assertLess(drop, inner)
        self.assertLess(inner, outer)  # сначала вложенный, иначе .salbum не пуст
        # безусловного удаления этих ключей нет
        self.assertNotRegex(un, r'DeleteRegKey HKCU "Software\\Classes\\\.salbum')

    @unittest.skipUnless(HAVE_PIL, "нет Pillow")
    def test_wizard_bmp_reproducible(self):
        # скрипт картинок детерминирован: два прогона — те же байты (обе картинки)
        outs = []
        with tempfile.TemporaryDirectory() as d:
            for n in ("a", "b"):
                paths = [os.path.join(d, n + ".bmp"), os.path.join(d, n + "-header.bmp")]
                subprocess.run([sys.executable, "-I", os.path.join(HERE, "make_wizard_bmp.py"), *paths], check=True)
                got = []
                for p in paths:
                    with open(p, "rb") as f:
                        got.append(f.read())
                outs.append(got)
        self.assertEqual(outs[0], outs[1])
        for data, name in zip(outs[0], ("wizard.bmp", "header.bmp")):
            self.assertEqual(len(data), os.path.getsize(os.path.join(HERE, name)), name)


@unittest.skipUnless(BASH, "нет bash")
class TestInstallerSh(unittest.TestCase):
    def setUp(self):
        self._tmp = tempfile.TemporaryDirectory()
        self.addCleanup(self._tmp.cleanup)
        self.dir = self._tmp.name
        self.exe = os.path.join(self.dir, "SosninaAlbums.exe")
        with open(self.exe, "wb") as f:
            f.write(b"MZ" + bytes(range(256)) * 400)
        self.empty = os.path.join(self.dir, "empty.exe")
        open(self.empty, "wb").close()

    def run_sh(self, *argv):
        p = subprocess.run([BASH, SH, *argv], capture_output=True, text=True, encoding="utf-8")
        return p.returncode, p.stdout, p.stderr

    def build(self, name, version="1.2.3", exe=None):
        out = os.path.join(self.dir, name)
        code, stdout, err = self.run_sh("--exe", exe or self.exe, "--version", version, "--out", out)
        self.assertEqual(code, 0, err)
        path = stdout.strip().splitlines()[-1]
        self.assertEqual(os.path.basename(path), "SosninaAlbums-%s-windows-x64-setup.exe" % version)
        return path

    def test_bad_arguments(self):
        out = os.path.join(self.dir, "o")
        self.assertEqual(self.run_sh()[0], 2)
        self.assertEqual(self.run_sh("--bogus")[0], 2)
        for bad in ("1.2", "1.2.3.4", "1.2.x", "1.2.3-dev", "../1.2.3"):
            code, _, err = self.run_sh("--exe", self.exe, "--version", bad, "--out", out)
            self.assertEqual(code, 2, bad + " " + err)
        self.assertFalse(os.path.exists(out))  # при отказе ничего не создано
        code, _, err = self.run_sh("--exe", os.path.join(self.dir, "none.exe"), "--version", "1.2.3", "--out", out)
        self.assertEqual(code, 1, err)
        code, _, err = self.run_sh("--exe", self.exe, "--version", "1.2.3", "--out", out,
                                   "--icon", os.path.join(self.dir, "none.ico"))
        self.assertEqual(code, 1, err)

    @unittest.skipUnless(HAVE_MAKENSIS or os.environ.get("CI"), "нет makensis")
    def test_build_wx_and_reproducible(self):
        self.assertTrue(HAVE_MAKENSIS, "в CI makensis обязателен (apt install nsis)")
        a = self.build("a")
        b = self.build("b")
        with open(a, "rb") as fa, open(b, "rb") as fb:
            da, db = fa.read(), fb.read()
        self.assertEqual(da[:2], b"MZ")
        self.assertEqual(da, db, "два прогона на одних входах должны совпасть побайтно")
        # манифест установщика: без запроса повышения (UAC), DPI
        self.assertIn(b'requestedExecutionLevel level="asInvoker"', da)
        self.assertIn(b"dpiAware", da)

    @unittest.skipUnless(HAVE_MAKENSIS or os.environ.get("CI"), "нет makensis")
    def test_reproducible_across_mtime_and_path(self):
        # release-sign.mjs пересобирает установщик из метки и сверяет с CI побайтно: время и путь exe там другие.
        # Без SetDateSave off mtime файла попадал в установщик и байты расходились
        self.assertTrue(HAVE_MAKENSIS, "в CI makensis обязателен (apt install nsis)")
        a = self.build("a")
        other = os.path.join(self.dir, "elsewhere", "SosninaAlbums.exe")
        os.makedirs(os.path.dirname(other))
        shutil.copyfile(self.exe, other)
        os.utime(other, (1_000_000_000, 1_000_000_000))  # 2001 год
        b = self.build("b", exe=other)
        with open(a, "rb") as fa, open(b, "rb") as fb:
            self.assertEqual(fa.read(), fb.read(), "mtime и путь exe не должны влиять на байты установщика")

    @unittest.skipUnless(HAVE_MAKENSIS or os.environ.get("CI"), "нет makensis")
    def test_versions_and_empty_exe(self):
        self.assertTrue(HAVE_MAKENSIS, "в CI makensis обязателен (apt install nsis)")
        cur = self.build("cur", "1.2.3")
        nxt = self.build("nxt", "1.2.4")  # пробный установщик следующей версии для win-check
        with open(cur, "rb") as f1, open(nxt, "rb") as f2:
            self.assertNotEqual(f1.read(), f2.read())
        # пустой exe (ci.yml проверяет сам скрипт без сборки программы) собирается так же, без предупреждений
        self.build("empty", "0.0.1", exe=self.empty)


if __name__ == "__main__":
    unittest.main()
