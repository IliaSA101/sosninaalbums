"""Тесты installer.sh и installer.nsi (спека 111, design §6): сборка makensis -WX, воспроизводимость, контракт скрипта.

Запуск: python -I -m unittest discover -s scripts/win -v. Без makensis сборочные тесты пропускаются;
в CI (переменная CI) его отсутствие — провал: иначе проверка молча ничего бы не проверяла.
"""

import fnmatch
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


# картинки мастера (make_wizard_bmp.py): installer.sh берёт их рядом с собой
WIZARD_BMPS = ("wizard.bmp", "wizard-100.bmp", "wizard-125.bmp", "wizard-150.bmp", "wizard-175.bmp", "header.bmp")
HAVE_7Z = shutil.which("7z") is not None


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
        waits = re.findall(r'MessageBox MB_RETRYCANCEL[^\n]*', self.nsi)
        self.assertEqual(len(waits), 2, waits)  # установщик и деинсталлятор
        for wait in waits:
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

    def func(self, name):
        i = self.nsi.index("Function %s\n" % name)
        return self.nsi[i:self.nsi.index("FunctionEnd", i)]

    def test_wait_unlocked_only_mutex_means_open(self):
        # 0.31-143-32: программа открыта — только по замку Wails; занятый exe без замка (живой посредник --mcp)
        # не ведёт в «busy» (60 с и код 5), а даёт 5 с и выход в замену
        wait = self.func("WaitUnlocked")
        self.assertIn("OpenMutexW", wait)
        after_open = wait[wait.index('FileOpen $R6'):wait.index("  busy:")]
        self.assertNotIn("Goto busy", after_open)
        self.assertNotRegex(after_open, r"\b(MessageBox|Quit)\b")  # ни окна в тихом режиме, ни ложного кода 5
        self.assertIn("IntOp $R9 $R9 + 1", after_open)
        self.assertRegex(after_open, r"\$\{If\} \$R9 >= 10\s+Goto done")
        self.assertRegex(after_open, r"Sleep 500\s+Goto loop")  # до 5 с ждём: программа дописывает выход
        self.assertIn("Goto busy", wait[:wait.index("FileOpen")])  # замок по-прежнему ведёт в ожидание и вопрос

    def test_replace_busy_old_renamed_with_tick(self):
        # занятый .old (образ живого посредника) уходит в .old-<тики>, а не валит замену кодом 6
        rep = self.func("ReplaceExe")
        mask = rep.index('Delete "$INSTDIR\\${EXE}.old*"')
        tick = rep.index("GetTickCount")
        ren = rep.index('Rename "$INSTDIR\\${EXE}.old" "$INSTDIR\\${EXE}.old-$R2"')
        mv = rep.index('Rename "$INSTDIR\\${EXE}" "$INSTDIR\\${EXE}.old"')
        self.assertTrue(mask < tick < ren < mv)
        self.assertIn("IfFileExists", rep[mask:tick])
        self.assertIn("Push $R2", rep)
        self.assertIn("Pop $R2", rep)
        # в конце установки хвосты тоже стираются маской
        sec = self.nsi[self.nsi.index('Section "main"'):self.nsi.index('Section "Uninstall"')]
        self.assertIn('Delete "$INSTDIR\\${EXE}.old*"', sec)

    def test_uninstall_busy_exe_asks_retry(self):
        # занятый exe или .old* (живой посредник, в том числе после обновления): «Повтор» как в WaitUnlocked;
        # «Отмена»/тихо - файл остаётся, остальное удаляется
        un = self.nsi[self.nsi.index('Section "Uninstall"'):]
        top = un.index("delexe:")
        last = un.index('Delete "$INSTDIR\\uninstall.exe"')
        loop = un[top:last]
        ask = re.search(r"MessageBox MB_RETRYCANCEL[^\n]*", un).group(0)
        self.assertIn("/SD IDCANCEL IDRETRY delexe", ask)
        self.assertIn("«Повтор»", ask)
        self.assertIn("Claude Desktop", ask)
        self.assertIn(ask, loop)
        # цикл стирает и exe, и .old*, и проверяет оба: иначе занятый .old навсегда останется в папке
        self.assertRegex(loop, r'Delete "\$INSTDIR\\\$\{EXE\}"')
        self.assertRegex(loop, r'Delete "\$INSTDIR\\\$\{EXE\}\.old\*"')
        self.assertRegex(loop, r'IfFileExists "\$INSTDIR\\\$\{EXE\}" ')
        self.assertRegex(loop, r'FindFirst \$R3 \$R4 "\$INSTDIR\\\$\{EXE\}\.old\*"')
        self.assertLess(loop.index("FindFirst"), loop.index("MessageBox"))
        self.assertLess(loop.index('{EXE}.old*"'), loop.index("FindFirst"))
        # вопрос не прерывает удаление: Quit/Abort в деинсталляторе нет, uninstall.exe и реестр - после него
        self.assertNotRegex(un, r"\b(Quit|Abort)\b")
        self.assertLess(last, un.index("DeleteRegKey HKCU"))

    def test_no_delete_on_reboot(self):
        # отложенное удаление требует прав администратора, установщик ставит без них
        code = "\n".join(ln for ln in self.nsi.splitlines() if not ln.lstrip().startswith(";"))
        self.assertNotIn("REBOOTOK", code)
        self.assertNotIn("MOVEFILE_DELAY_UNTIL_REBOOT", code)

    def test_wizard_bmp(self):
        # картинки мастера: 2x (200 % и выше) и по одной точно под высоту контрола на 100/125/150/175 %
        # (314/386/458/555 px, измерено на снимках CI; ширина — по пропорции 164x314), 57x57 в 2x в шапке; BMP 24 бита
        sizes = (("wizard.bmp", (328, 628)), ("wizard-100.bmp", (164, 314)), ("wizard-125.bmp", (202, 386)),
                 ("wizard-150.bmp", (239, 458)), ("wizard-175.bmp", (290, 555)), ("header.bmp", (114, 114)))
        self.assertEqual([n for n, _ in sizes], list(WIZARD_BMPS))
        for name, size in sizes:
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

    def test_wizard_for_dpi(self):
        # подмена картинки по системному DPI: после MUI2 (.onGUIInit), пороги 96/120/144/168 по возрастанию,
        # на каждый масштаб своя картинка; выше 168 и при сбое чтения DPI остаётся 2x от MUI2
        self.assertIn("!define MUI_CUSTOMFUNCTION_GUIINIT WizardForDpi", self.nsi)
        body = self.nsi[self.nsi.index("Function WizardForDpi"):]
        body = body[:body.index("FunctionEnd")]
        self.assertIn("gdi32::GetDeviceCaps(p r0, i 90)", body)  # 90 = LOGPIXELSY
        self.assertIn("user32::ReleaseDC", body)
        picks = re.findall(r'\$1 <= (\d+)\n\s+File "/oname=\$PLUGINSDIR\\modern-wizard.bmp" "\$\{WIZARD_DPI\}/(wizard-\d+)\.bmp"',
                           body)
        self.assertEqual(picks, [("96", "wizard-100"), ("120", "wizard-125"), ("144", "wizard-150"),
                                 ("168", "wizard-175")])
        self.assertEqual(body.count("!pragma verifyloadimage"), 4)
        self.assertNotIn("${Else}\n", body)
        self.assertIn("!ifndef WIZARD_DPI", self.nsi)

    def test_set_date_save_first(self):
        # SetDateSave off — первая команда после Unicode true: ни одна картинка и exe не попадут в установщик с mtime
        lines = [ln for ln in self.nsi.splitlines() if ln.strip() and not ln.lstrip().startswith(";")]
        self.assertEqual(lines[0], "Unicode true")
        self.assertEqual(lines[1], "SetDateSave off")

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

    def test_demo_block_order(self):
        # стандартная редакция (0.30-142-44): пример — после замены exe, затем рабочая папка возвращается в $INSTDIR
        # (её наследуют CreateShortcut и Exec); без DEMO_SRC блока нет вовсе
        i0 = self.nsi.index("!ifdef DEMO_SRC")
        blk = self.nsi[i0:self.nsi.index("!endif", i0)]
        self.assertLess(self.nsi.index("Call ReplaceExe"), i0)
        self.assertLess(i0, self.nsi.index("WriteUninstaller"))
        self.assertLess(i0, self.nsi.index("CreateShortcut"))
        self.assertLess(blk.index('SetOutPath "$INSTDIR\\data"'), blk.index('File "${DEMO_SRC}"'))
        self.assertLess(blk.index('File "${DEMO_SRC}"'), blk.index('SetOutPath "$INSTDIR"'))
        self.assertNotRegex(self.nsi, r"!ifndef DEMO_SRC\s*\n\s*!error")  # DEMO_SRC необязателен

    def test_uninstall_demo_mask_and_order(self):
        # маска по префиксу (v2 не оставит сиротой v1; имя через -D при пропуске уронило бы makensis -WX);
        # RMDir data без /r — до RMDir $INSTDIR, иначе папка программы останется непустой
        un = self.nsi[self.nsi.index('Section "Uninstall"'):]
        mask = un.index('Delete "$INSTDIR\\data\\wedding-navy-watercolor-demo-*.zip"')
        rd = un.index('RMDir "$INSTDIR\\data"\n')
        inst = un.index('\n  RMDir "$INSTDIR"\n')
        self.assertLess(mask, rd)
        self.assertLess(rd, inst)
        self.assertNotIn("!ifdef", un)  # удаление безусловное, не зависит от DEMO_SRC

    @unittest.skipUnless(HAVE_PIL, "нет Pillow")
    def test_wizard_bmp_reproducible(self):
        # скрипт картинок детерминирован: два прогона в разные папки — те же байты всех шести файлов,
        # размеры совпадают с файлами в git (байты в git зависят от версии Pillow, размеры — нет)
        outs = []
        with tempfile.TemporaryDirectory() as d:
            for n in ("a", "b"):
                out = os.path.join(d, n)
                subprocess.run([sys.executable, "-I", os.path.join(HERE, "make_wizard_bmp.py"), out], check=True)
                self.assertEqual(sorted(os.listdir(out)), sorted(WIZARD_BMPS))
                got = {}
                for name in WIZARD_BMPS:
                    with open(os.path.join(out, name), "rb") as f:
                        got[name] = f.read()
                outs.append(got)
        self.assertEqual(outs[0], outs[1])
        for name in WIZARD_BMPS:
            self.assertEqual(len(outs[0][name]), os.path.getsize(os.path.join(HERE, name)), name)


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
    def test_reproducible_across_bitmap_mtime(self):
        # MUI2 и WizardForDpi кладут wizard*.bmp и header.bmp командой File в .onGUIInit; без SetDateSave off в начале скрипта
        # их mtime (момент git checkout) попадал в установщик, и сверка release-sign.mjs с CI не сходилась (0.25.0)
        self.assertTrue(HAVE_MAKENSIS, "в CI makensis обязателен (apt install nsis)")
        a = self.build("a")
        # копия скрипта с картинками: installer.sh берёт их по своему расположению
        win = os.path.join(self.dir, "repo", "scripts", "win")
        os.makedirs(win)
        for name in ("installer.sh", "installer.nsi", *WIZARD_BMPS):
            shutil.copyfile(os.path.join(HERE, name), os.path.join(win, name))
        for name in WIZARD_BMPS:
            os.utime(os.path.join(win, name), (1_000_000_000, 1_000_000_000))  # 2001 год
        out = os.path.join(self.dir, "b")
        p = subprocess.run([BASH, os.path.join(win, "installer.sh"), "--exe", self.exe, "--version", "1.2.3",
                            "--out", out, "--icon", os.path.join(HERE, "..", "..", "build", "windows", "icon.ico")],
                           capture_output=True, text=True, encoding="utf-8")
        self.assertEqual(p.returncode, 0, p.stderr)
        b = p.stdout.strip().splitlines()[-1]
        with open(a, "rb") as fa, open(b, "rb") as fb:
            self.assertEqual(fa.read(), fb.read(), "mtime картинок мастера не должен влиять на байты установщика")

    @unittest.skipUnless(HAVE_MAKENSIS or os.environ.get("CI"), "нет makensis")
    @unittest.skipUnless(HAVE_7Z, "нет 7z: содержимое установщика не прочитать")
    def test_all_wizard_images_inside(self):
        # в установщике лежат все пять картинок мастера (2x и четыре под DPI), по одной на ветку WizardForDpi, и шапка
        self.assertTrue(HAVE_MAKENSIS, "в CI makensis обязателен (apt install nsis)")
        setup = self.build("a")
        p = subprocess.run(["7z", "l", "-slt", setup], capture_output=True, text=True, encoding="utf-8")
        self.assertEqual(p.returncode, 0, p.stderr)
        sizes = {}
        for block in p.stdout.split("\n\n"):
            fields = dict(ln.split(" = ", 1) for ln in block.splitlines() if " = " in ln)
            if fields.get("Path", "").startswith("$PLUGINSDIR"):
                sizes.setdefault(fields["Path"], []).append(int(fields["Size"]))
        want = sorted(os.path.getsize(os.path.join(HERE, n)) for n in WIZARD_BMPS[:5])
        self.assertEqual(sorted(sizes["$PLUGINSDIR/modern-wizard.bmp"]), want)
        self.assertEqual(sizes["$PLUGINSDIR/modern-header.bmp"], [os.path.getsize(os.path.join(HERE, "header.bmp"))])

    def test_missing_dpi_image(self):
        # нет любой из картинок под DPI — установщик не собирается, не молча без неё (до makensis)
        for gone in WIZARD_BMPS[1:5]:
            win = os.path.join(self.dir, "repo-" + gone, "scripts", "win")
            os.makedirs(win)
            for name in ("installer.sh", "installer.nsi", *WIZARD_BMPS):
                if name != gone:
                    shutil.copyfile(os.path.join(HERE, name), os.path.join(win, name))
            p = subprocess.run([BASH, os.path.join(win, "installer.sh"), "--exe", self.exe, "--version", "1.2.3",
                                "--out", os.path.join(self.dir, "o-" + gone), "--icon", self.exe],
                               capture_output=True, text=True, encoding="utf-8")
            self.assertEqual(p.returncode, 1, gone)
            self.assertIn(gone, p.stderr)

    @unittest.skipUnless(HAVE_MAKENSIS or os.environ.get("CI"), "нет makensis")
    def test_versions_and_empty_exe(self):
        self.assertTrue(HAVE_MAKENSIS, "в CI makensis обязателен (apt install nsis)")
        cur = self.build("cur", "1.2.3")
        nxt = self.build("nxt", "1.2.4")  # пробный установщик следующей версии для win-check
        with open(cur, "rb") as f1, open(nxt, "rb") as f2:
            self.assertNotEqual(f1.read(), f2.read())
        # пустой exe (ci.yml проверяет сам скрипт без сборки программы) собирается так же, без предупреждений
        self.build("empty", "0.0.1", exe=self.empty)

    def test_demo_and_standard_only_together(self):
        # редакция — пара ключей; одиночный не собирает ничего (до makensis)
        out = os.path.join(self.dir, "o")
        zp = os.path.join(self.dir, "demo.zip")
        with open(zp, "wb") as f:
            f.write(b"PK")
        base = ("--exe", self.exe, "--version", "1.2.3", "--out", out)
        self.assertEqual(self.run_sh(*base, "--demo", zp)[0], 2)
        self.assertEqual(self.run_sh(*base, "--standard")[0], 2)
        code, _, err = self.run_sh(*base, "--demo", os.path.join(self.dir, "none.zip"), "--standard")
        self.assertEqual(code, 1, err)
        self.assertFalse(os.path.exists(out))

    def test_standard_name_outside_sign_mask(self):
        # release-sign.mjs ищет «*-windows-x64-setup.exe» и SETUP_RE: стандартный установщик под них попасть не должен
        name = "SosninaAlbums-1.2.3-windows-x64-standard-setup.exe"
        self.assertFalse(fnmatch.fnmatch(name, "*-windows-x64-setup.exe"))
        self.assertTrue(fnmatch.fnmatch("SosninaAlbums-1.2.3-windows-x64-setup.exe", "*-windows-x64-setup.exe"))
        with open(SH, encoding="utf-8") as f:
            self.assertIn("-windows-x64-$suffix.exe", f.read())

    @unittest.skipUnless(HAVE_MAKENSIS or os.environ.get("CI"), "нет makensis")
    def test_standard_build_wx_and_demo_inside(self):
        self.assertTrue(HAVE_MAKENSIS, "в CI makensis обязателен (apt install nsis)")
        zp = os.path.join(self.dir, "wedding-navy-watercolor-demo-1.zip")
        with open(zp, "wb") as f:
            f.write(b"PK\x03\x04" + os.urandom(200_000))  # несжимаемый: размер установщика растёт на размер zip
        compact = self.build("c")
        out = os.path.join(self.dir, "s")
        args = ("--exe", self.exe, "--version", "1.2.3", "--out", out, "--demo", zp, "--standard")
        code, stdout, err = self.run_sh(*args)
        self.assertEqual(code, 0, err)
        std = stdout.strip().splitlines()[-1]
        self.assertEqual(os.path.basename(std), "SosninaAlbums-1.2.3-windows-x64-standard-setup.exe")
        self.assertGreater(os.path.getsize(std), os.path.getsize(compact) + 190_000)
        code2, stdout2, _ = self.run_sh(*args)
        self.assertEqual(code2, 0)
        with open(std, "rb") as f1, open(stdout2.strip().splitlines()[-1], "rb") as f2:
            self.assertEqual(f1.read(), f2.read(), "стандартная сборка воспроизводима побайтно")
        with open(compact, "rb") as f:
            self.assertEqual(f.read()[:2], b"MZ")


if __name__ == "__main__":
    unittest.main()
