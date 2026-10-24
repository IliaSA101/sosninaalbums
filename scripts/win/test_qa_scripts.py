"""Статические проверки сценариев win-qa и installer-check (доправка 118, прогоны 3-4): ошибки, найденные на раннере."""

import os
import re
import unittest

HERE = os.path.dirname(os.path.abspath(__file__))


def read(*parts):
    with open(os.path.join(HERE, *parts), encoding="utf-8-sig") as f:
        return f.read()


class QaInstall(unittest.TestCase):
    def setUp(self):
        self.text = read("qa", "qa-install.ps1")

    def test_no_nsis2_copy_name(self):
        # NSIS 3: копия деинсталлятора - ~nsuXXX.tmp\Un.exe, процесс Un; Au_ был у NSIS 2
        self.assertNotIn("'Au_'", self.text)
        self.assertIn("-ProcNames @('Un')", self.text)

    def test_v9_starts_setup_before_wizard(self):
        m = re.search(r"Start-FromMenu 30\s+Start-QaVideo 'В9'(.*?)Invoke-QaWizard[^\n]*-Id 'В9'", self.text, re.S)
        self.assertIsNotNone(m, "перед Invoke-QaWizard В9 ожидается Start-Setup")
        self.assertIn("Start-Setup $setup0", m.group(1))

    def test_v6_v7_wait_app_window(self):
        self.assertGreaterEqual(self.text.count("Wait-QaAppWindow"), 2)


class WizardPress(unittest.TestCase):
    def test_foreground_before_press(self):
        text = read("qa", "winqa-wizard.psm1")
        body = text[text.index("function Invoke-WizTarget"):text.index("function Invoke-WizPress")]
        self.assertIn("Set-WizForeground", body)
        self.assertNotIn("-Mouse", body)
        wiz = text[text.index("function Invoke-QaWizard"):]
        self.assertEqual(wiz.count("Invoke-WizPress"), 3)
        self.assertNotIn("(Invoke-WizTarget", wiz)

    def test_e4_prefers_running_app(self):
        text = read("qa", "qa-update.ps1")
        e4 = text[text.index("Invoke-QaItem 'Е4'"):text.index("Invoke-QaItem 'Е5'")]
        self.assertLess(e4.index("Get-AppProcs"), e4.index("Start-StandCmd"))


class InstallerCheck(unittest.TestCase):
    def test_mark_reread_before_uninstall(self):
        text = read("installer-check.ps1")
        uninstall = text.index("Invoke-Setup $UninstExe")
        reread = text.rfind("$mark = [System.IO.File]::ReadAllText($Config)", 0, uninstall)
        self.assertGreater(reread, text.rfind("Stop-App", 0, uninstall), "метка читается после последнего запуска программы")
        self.assertGreater(reread, text.index("Section 'Удаление'"))


class Shots(unittest.TestCase):
    def test_save_shot_sets_dpi_awareness(self):
        text = read("qa", "winqa.psm1")
        body = text[text.index("function Save-QaShot"):]
        body = body[: body.index("\n}\n")]
        self.assertIn("Enable-QaDpiAware", body)
        self.assertIn("SetProcessDpiAwarenessContext(new System.IntPtr(-4))", text)
        self.assertIn("Enable-QaDpiAware", read("qa", "run.ps1"))


class WizardFallback(unittest.TestCase):
    def test_e3_has_video_and_blocks_e4(self):
        text = read("qa", "qa-update.ps1")
        self.assertIn("Start-QaVideo 'Е3'", text)
        self.assertIn("Stop-QaNewProcs -Name $name1", text)
        self.assertRegex(text, r"WizOk\)\s*\{\s*Add-QaResult -Id 'Е4' -Status skip")

    def test_wizard_uses_win32_fallback(self):
        text = read("qa", "winqa-wizard.psm1")
        self.assertIn("Write-QaUiaDiag", text)
        # класс SaQa.Wiz перенесён в winqa.psm1 (его зовёт и блок update)
        self.assertIn("0x00F5", read("qa", "winqa.psm1"))  # BM_CLICK
        self.assertNotIn("Add-Type", text)

    def test_wizard_press_text_rechecks_page(self):
        text = read("qa", "winqa-wizard.psm1")
        self.assertIn("страница сменилась с задержкой", text)


class OpenBlock(unittest.TestCase):
    def test_a12_close_scoped_to_tour_card(self):
        text = read("qa", "qa-open-a.ps1")
        a12 = text[text.index("Invoke-QaItem 'А12'"):text.index("Invoke-QaItem 'А13'")]
        self.assertNotIn("Invoke-QaButton $id 'Закрыть'", a12)
        self.assertIn("Find-QaButtonNear", a12)
        self.assertIn("^Видео о программе", a12[a12.index("$gone"):])

    def test_open_about_brings_window_front(self):
        mod = read("qa", "winqa-update.psm1")
        fn = mod[mod.index("function Open-QaAbout"):]
        self.assertLess(fn.index("Set-QaAppForeground"), fn.index("^Справка"))
        self.assertIn("AboutWhy", fn)
        text = read("qa", "qa-update.ps1")
        e4 = text[text.index("Invoke-QaItem 'Е4'"):text.index("Invoke-QaItem 'Е5'")]
        self.assertIn("Save-QaShot -Id 'Е4'", e4)

    def test_app_foreground_without_alt(self):
        # одиночный Alt вешает окно программы (модальный цикл меню), Alt-приём - только для мастера
        mod = read("qa", "winqa-update.psm1")
        fg = mod[mod.index("function Set-QaAppForeground"):mod.index("function Open-QaAbout")]
        self.assertNotIn("]::Foreground(", fg)
        self.assertIn("ForegroundNoKey", fg)
        fn = mod[mod.index("function Open-QaAbout"):mod.index("function Wait-QaNewApp")]
        self.assertGreaterEqual(fn.count("Step-Qa"), 5)
        base = read("qa", "winqa.psm1")
        nokey = base[base.index("ForegroundNoKey(long"):base.index("TopWindows(int[]")]
        self.assertNotIn("keybd_event", nokey)

    def test_modules_do_not_touch_foreign_qa_state(self):
        # $script:Qa живёт в области winqa.psm1; в других модулях он $null
        import glob, os
        for f in glob.glob(os.path.join(os.path.dirname(__file__), "qa", "*.psm1")):
            if os.path.basename(f) == "winqa.psm1":
                continue
            with open(f, encoding="utf-8-sig") as h:
                self.assertNotIn("$script:Qa.", h.read(), f)

    def test_d3_waits_for_same_version_text(self):
        text = read("qa", "qa-open-d.ps1")
        self.assertIn("сохранён более новой версией программы", text)

    def test_d_prep_hides_edge_first_run(self):
        text = read("qa", "qa-open-d.ps1")
        self.assertIn("HideFirstRunExperience", text)
        self.assertLess(text.index("HideFirstRunExperience"), text.index("Invoke-QaItem 'Д2'"))


if __name__ == "__main__":
    unittest.main()
