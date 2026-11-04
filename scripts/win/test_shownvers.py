"""Таблица случаев Get-QaShownVers (test_winqa.ps1) совпадает с Go-тестом TestShownVers по литералам."""
import re
import unittest
from pathlib import Path

ROOT = Path(__file__).resolve().parents[2]


def go_cases():
    src = (ROOT / "internal/app/assoc/version_test.go").read_text(encoding="utf-8")
    body = src.split("func TestShownVers", 1)[1].split("for _, c := range cases", 1)[0]
    return re.findall(r'\{"([^"]*)", "([^"]*)", "([^"]*)", "([^"]*)"\}', body)


def ps_cases():
    src = (ROOT / "scripts/win/qa/test_winqa.ps1").read_text(encoding="utf-8")
    body = src.split("Section 'Get-QaShownVers'", 1)[1].split("Section 'Test-QaTourLog'", 1)[0]
    return re.findall(r"@\('([^']*)', '([^']*)', '([^']*)', '([^']*)'\)", body)


class ShownVersTable(unittest.TestCase):
    def test_tables_match(self):
        go, ps = go_cases(), ps_cases()
        self.assertTrue(go)
        self.assertEqual(go, ps)


if __name__ == "__main__":
    unittest.main()
