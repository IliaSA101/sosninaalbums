"""Общее для pack.py и pack_linux.py: имена файлов выпуска, ошибка, проверка имени записи (спека 039, 101).

Раскладка и имена живут здесь и в pack.py; остальные скрипты зовут pack.py. Только stdlib.
"""

import re

APP = "sosninaalbums.app"
EXE = "SosninaAlbums.exe"
BIN = "sosninaalbums"  # исполняемый файл Linux
ICON = BIN + ".png"
NOTICES = "THIRD_PARTY_NOTICES.txt"
LICENSE = "LICENSE"
NOTICE_MD = "NOTICE.md"
CHANGES = "Что нового.txt"
GUIDE_STEM = {
    "windows": "Как установить (Windows)",
    "macos": "Как установить (Mac)",
    "linux": "Как установить (Linux)",
}
GUIDE_EXT = (".pdf", ".docx")
MAX_UNPACKED = 1 << 30  # несжатый объём, 1 ГБ
# Файлы с текстами лицензий: в каждом архиве выпуска (п. 4 и 6 GPL — копия лицензии с программой)
LEGAL = (NOTICES, LICENSE, NOTICE_MD)


class PackError(Exception):
    """Ошибка входа или проверки: текст уходит в stderr, код 1."""


def folder_name(version):
    return "Sosnina Albums " + version


def guide_name(osname, ext):
    return GUIDE_STEM[osname] + ext


def check_name(name):
    """Имя записи: относительное, без «..», «.», пустых частей, обратных косых, диска, управляющих."""
    if not name or name.startswith("/") or "\\" in name:
        raise PackError("небезопасное имя записи: %r" % name)
    if any(ord(c) < 32 or c == "\x7f" for c in name):
        raise PackError("управляющий символ в имени записи: %r" % name)
    parts = name.rstrip("/").split("/")
    if any(p in ("", ".", "..") for p in parts) or re.match(r"^[A-Za-z]:", parts[0]):
        raise PackError("небезопасное имя записи: %r" % name)
    return parts
