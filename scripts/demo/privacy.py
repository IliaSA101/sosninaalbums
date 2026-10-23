"""Проверка приватности кадра (102-9, design §7): личное не должно попасть в ролик.

На каждой метке журнала сценарий собирает текст страницы и подписи видимых узлов (`COLLECT_JS`), а `scan` ищет в них
домашний каталог, имя пользователя и типовые пути. Находка — запись провалена (код 3 в demo.py). Чистая часть
(`scan`, `findings_text`) работает без браузера. Распознавания по картинке нет (tesseract нет): ловим текст страницы
раньше, чем он окажется в кадре.
"""

from __future__ import annotations

import getpass
import re
from pathlib import Path

# Текст страницы и атрибуты видимых в окне узлов: title, aria-label, alt, placeholder, value полей ввода.
COLLECT_JS = """
() => {
  const vw = window.innerWidth, vh = window.innerHeight
  const out = [document.body ? document.body.innerText : '', document.title || '']
  for (const el of document.querySelectorAll('[title],[aria-label],[alt],[placeholder],input,textarea')) {
    const r = el.getBoundingClientRect()
    if (r.width <= 0 || r.height <= 0 || r.right <= 0 || r.bottom <= 0 || r.left >= vw || r.top >= vh) continue
    for (const a of ['title', 'aria-label', 'alt', 'placeholder']) {
      const v = el.getAttribute(a)
      if (v) out.push(v)
    }
    if ('value' in el && typeof el.value === 'string' && el.value) out.push(el.value)
  }
  return out
}
"""

# Путеподобное — подстрокой; слова (stand, имя) — только целыми словами: «понимаю» не содержит «stand» по ошибке.
_SUBSTR = ("/home/", "/media/", "/run/media/", ".gotmp", "SosninaAlbums-stand", "/Users/")
_DRIVE = re.compile(r"[A-Za-z]:\\")


def _word(w: str) -> re.Pattern[str]:
    return re.compile(r"(?<![0-9A-Za-zА-Яа-яЁё_])" + re.escape(w) + r"(?![0-9A-Za-zА-Яа-яЁё_])", re.IGNORECASE)


def _context(text: str, i: int, n: int) -> str:
    s = text[max(0, i - 15) : i + n + 25]
    return " ".join(s.split())


def scan(texts: list[str], home: str, user: str, extra: tuple[str, ...] = ()) -> list[str]:
    """Находки: строки «что: контекст» (без повторов). Пусто — чисто. Пустые home и user не ищутся (не «любой текст»)."""
    found: list[str] = []
    seen: set[tuple[str, str]] = set()

    def add(kind: str, text: str, i: int, n: int) -> None:
        ctx = _context(text, i, n)
        if (kind, ctx) not in seen:
            seen.add((kind, ctx))
            found.append(f"{kind}: {ctx}")

    home = home.strip().rstrip("/\\")
    patterns = [(s, s) for s in (*_SUBSTR, *extra) if s]
    if len(home) > 1:  # «/» не домашний каталог
        patterns.append((home, "домашний каталог"))
    for text in texts:
        low = text.lower()
        for needle, label in patterns:
            start = 0
            while (i := low.find(needle.lower(), start)) >= 0:
                add(label if label != needle else f"путь {needle}", text, i, len(needle))
                start = i + len(needle)
        for m in _DRIVE.finditer(text):
            add("диск Windows", text, m.start(), len(m.group()))
        for word, label in (("stand", "стенд"), (user.strip(), "имя пользователя")):
            if not word:
                continue
            for m in _word(word).finditer(text):
                add(label, text, m.start(), len(m.group()))
    return found


def findings_text(mark: str, found: list[str]) -> str:
    """Строки для privacy.txt: метка и находки (пусто, если чисто)."""
    return "".join(f"[{mark}] {f}\n" for f in found)


def current_identity() -> tuple[str, str]:
    """(домашний каталог, имя пользователя) этой машины для scan."""
    return str(Path.home()), getpass.getuser()
