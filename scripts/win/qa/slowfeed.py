"""Медленная лента обновлений для win-qa (спека 118, Г10): раздаёт папку на 127.0.0.1 с ограничением скорости.

Нужна, чтобы успеть нажать «Отменить» во время скачивания: python -m http.server отдаёт установщик за доли секунды.
Запуск: python slowfeed.py <папка ленты> <порт> <байт в секунду>. Обрыв соединения клиентом (отмена) — не ошибка.
"""

import functools
import sys
import time
from http.server import SimpleHTTPRequestHandler, ThreadingHTTPServer

CHUNK = 64 * 1024


def make_handler(rate):
    class Slow(SimpleHTTPRequestHandler):
        def copyfile(self, source, outputfile):
            # порциями с паузой: скорость ≈ rate байт/с
            try:
                while True:
                    buf = source.read(CHUNK)
                    if not buf:
                        break
                    outputfile.write(buf)
                    outputfile.flush()
                    time.sleep(len(buf) / rate)
            except (BrokenPipeError, ConnectionResetError, ConnectionAbortedError):
                pass

        def log_message(self, *args):
            pass

    return Slow


def main():
    root, port, rate = sys.argv[1], int(sys.argv[2]), max(1, int(sys.argv[3]))
    handler = functools.partial(make_handler(rate), directory=root)
    ThreadingHTTPServer(("127.0.0.1", port), handler).serve_forever()


if __name__ == "__main__":
    main()
