#!/usr/bin/env python3
"""record the real cli in rgb pseudo-terminals and save lossless animated pngs."""

import argparse
import codecs
import fcntl
import os
from pathlib import Path
import pty
import select
import signal
import socket
import struct
import subprocess
import tempfile
import termios
import time

from PIL import Image, ImageDraw, ImageFont
import pyte

ROOT = Path(__file__).resolve().parents[1]
COLS, ROWS, FPS = 76, 5, 15
CELL_W, CELL_H, PAD = 12, 26, 28
BG, FG = '#10141f', '#dbe5f5'


def send(path, payload):
    with socket.socket(socket.AF_UNIX) as client:
        client.settimeout(2)
        client.connect(str(path))
        client.sendall(payload.encode())
        client.shutdown(socket.SHUT_WR)


class Terminal:
    def __init__(self, path, flags):
        self.path = path
        self.master, slave = pty.openpty()
        fcntl.ioctl(slave, termios.TIOCSWINSZ, struct.pack('HHHH', ROWS, COLS, 0, 0))
        self.screen = pyte.Screen(COLS, ROWS)
        self.stream = pyte.Stream(self.screen)
        self.decoder = codecs.getincrementaldecoder('utf-8')('replace')
        self.raw = bytearray()
        env = dict(os.environ, TERM='xterm-256color', COLORTERM='truecolor')
        env.pop('NO_COLOR', None)
        self.process = subprocess.Popen(
            [str(ROOT / 'progress-bar-3000'), '--socket-path', str(path),
             '--color-mode', 'truecolor', '--fps', '30', '--width', '54',
             '--format', '%p %{percent}', *flags],
            stdin=slave, stdout=slave, stderr=slave, env=env, start_new_session=True)
        os.close(slave)

    def read(self):
        while select.select([self.master], [], [], 0)[0]:
            try:
                chunk = os.read(self.master, 65536)
            except OSError:
                break
            if not chunk:
                break
            self.raw.extend(chunk)
            self.stream.feed(self.decoder.decode(chunk))

    def close(self):
        if self.process.poll() is None:
            # this is the exact child owned by this object, never a name lookup.
            self.process.send_signal(signal.SIGINT)
            try:
                self.process.wait(timeout=3)
            except subprocess.TimeoutExpired:
                self.process.kill()
                self.process.wait()
        os.close(self.master)


def colour(value, fallback):
    if value == 'default':
        return fallback
    if len(value) == 6 and all(c in '0123456789abcdef' for c in value.lower()):
        return '#' + value
    return {'white': FG, 'black': BG, 'blue': '#0087ff', 'brightblack': '#65718a'}.get(value, fallback)


def paint_row(draw, screen, row, y, font, bold):
    for x in range(COLS):
        cell = screen.buffer[row][x]
        left = PAD + x * CELL_W
        fg, bg = colour(cell.fg, FG), colour(cell.bg, BG)
        if cell.reverse:
            fg, bg = bg, fg
        draw.rectangle((left, y, left + CELL_W - 1, y + CELL_H - 1), fill=bg)
        if cell.data and cell.data != ' ':
            draw.text((left, y - 1), cell.data, font=bold if cell.bold else font, fill=fg)


def record(name, title, subtitle, specs, update, duration, font_path):
    font = ImageFont.truetype(font_path, 20)
    try:
        bold = ImageFont.truetype(font_path, 20, index=1)
    except OSError:
        bold = font
    small = ImageFont.truetype(font_path, 16)
    lines = sum(rows + (1 if label else 0) + 1 for label, _, rows in specs)
    height = 116 + lines * CELL_H + 20
    frames = []
    with tempfile.TemporaryDirectory(prefix='pb3-', dir='/tmp') as temp:
        terminals = []
        try:
            for i, (_, flags, _) in enumerate(specs):
                terminals.append(Terminal(Path(temp) / f'{i}.sock', flags))
            deadline = time.monotonic() + 5
            while not all(t.path.exists() for t in terminals):
                if time.monotonic() > deadline or any(t.process.poll() is not None for t in terminals):
                    raise RuntimeError('renderer failed to open its socket')
                time.sleep(.02)
            for t in terminals:
                send(t.path, '@set-phases fetch,build,test,package\n@set-total 100\n@value 0\n@phase-name fetch\n')
            # wait for a real initialized frame so the animation poster is useful.
            deadline = time.monotonic() + 5
            while True:
                for terminal in terminals:
                    terminal.read()
                ready = all(
                    any('%' in row for row in terminal.screen.display)
                    and (rows == 1 or any('fetch' in row for row in terminal.screen.display))
                    for terminal, (_, _, rows) in zip(terminals, specs)
                )
                if ready:
                    break
                if time.monotonic() > deadline:
                    raise RuntimeError('renderer did not display its initialized frame')
                time.sleep(.03)
            started = time.monotonic()
            count = round(duration * FPS)
            for frame in range(count):
                target = started + frame / FPS
                time.sleep(max(0, target - time.monotonic()))
                elapsed = frame / FPS
                for i, terminal in enumerate(terminals):
                    payload = update(elapsed, i, duration)
                    if payload:
                        send(terminal.path, payload)
                time.sleep(.015)
                canvas = Image.new('RGB', (COLS * CELL_W + PAD * 2, height), BG)
                draw = ImageDraw.Draw(canvas)
                draw.text((PAD, 20), title, font=bold, fill=FG)
                draw.text((PAD, 55), subtitle, font=small, fill='#91a1bc')
                y = 105
                for terminal, (label, _, rows) in zip(terminals, specs):
                    terminal.read()
                    if label:
                        draw.text((PAD, y), label, font=small, fill='#91a1bc')
                        y += CELL_H
                    for row in range(rows):
                        paint_row(draw, terminal.screen, row, y, font, bold)
                        y += CELL_H
                    y += CELL_H
                frames.append(canvas)
            for terminal in terminals:
                if b'\x1b[38;2;' not in terminal.raw:
                    raise RuntimeError('recording did not receive 24-bit rgb output')
                if not any('%' in row for row in terminal.screen.display):
                    raise RuntimeError('recording has no visible progress bar')
            out = ROOT / 'assets' / 'demos' / f'{name}.png'
            out.parent.mkdir(parents=True, exist_ok=True)
            frames[0].save(out, save_all=True, append_images=frames[1:],
                           duration=round(1000 / FPS), loop=0, disposal=0, blend=0)
            with Image.open(out) as result:
                assert result.is_animated and result.mode == 'RGB'
                assert result.n_frames > 30
            print(f'recorded {out.relative_to(ROOT)}: {len(frames)} rgb frames')
        finally:
            for terminal in terminals:
                terminal.close()


def progress(elapsed, _, duration):
    value = min(100, max(0, (elapsed - .4) / (duration - 1.5) * 100))
    phase = ['fetch', 'build', 'test', 'package'][min(3, int(value / 25))]
    return f'@value {value:.2f}\n'


def phased(elapsed, _, duration):
    frame = round(elapsed * FPS)
    interval = round((duration - 2) * FPS / 4)
    if frame % interval:
        return ''
    step = min(4, frame // interval)
    phase = ['fetch', 'build', 'test', 'package', 'package'][step]
    label = ['fetching objects', 'compiling modules', 'checking results', 'packing artifacts', 'done'][step]
    return (f'@set-total 4\n@value {step}\n@phase-name {phase}\n'
            f'@label {label}\n@meta objects={step * 600}\n')


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--font', default='/System/Library/Fonts/Menlo.ttc', help='monospace font file')
    parser.add_argument('--only', choices=['phases', 'styles', 'animations', 'socket'])
    args = parser.parse_args()
    demos = {
        'phases': ('progress-bar-3000', 'smooth progress · highlighted phase plan · 24-bit rgb',
                   [('', ['--tint-animation', 'cycle', '--detail-format', '%{phases}'], 2)], phased, 8),
        'styles': ('choose your texture', 'the same progress, rendered with different fill and track styles',
                   [(style, ['--style', style, '--bg-style', 'shade-light'], 1)
                    for style in ['plain', 'block', 'granular', 'shaded', 'gradient-block', 'gradient-granular', 'gradient-shaded']], progress, 7),
        'animations': ('colour in motion', 'pulse · shimmer · cycle — a held value makes each tint easy to see',
                       [(tint, ['--tint-animation', tint, '--gradient-start', '#ff70d2', '--gradient-end', '#00d8ff'], 1)
                        for tint in ['pulse', 'shimmer', 'cycle']], lambda *_: '@value 68\n', 8),
        'socket': ('one renderer, many updates', 'unix socket input · live labels · custom metadata',
                   [('', ['--tint-animation', 'shimmer', '--gradient-start', '#76ffbd', '--gradient-end', '#0087ff',
                          '--detail-format', '%{phases}', '--detail-format', '%{label}',
                          '--detail-format', 'objects: %{meta:objects} / 2400'], 4)], phased, 8),
    }
    for name, demo in demos.items():
        if args.only is None or args.only == name:
            record(name, *demo, args.font)


if __name__ == '__main__':
    main()
