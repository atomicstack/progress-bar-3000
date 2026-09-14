"""check that animation reels demonstrate distinct real cli events."""

import importlib.util
from pathlib import Path
import unittest


spec = importlib.util.spec_from_file_location('record_demos', Path(__file__).with_name('record-demos.py'))
recorder = importlib.util.module_from_spec(spec)
spec.loader.exec_module(recorder)


class AnimationReelTests(unittest.TestCase):
    def demos(self):
        self.assertTrue(hasattr(recorder, 'animation_demos'), 'individual animation reels are missing')
        return recorder.animation_demos()

    def test_reels_use_explicit_granular_truecolor(self):
        demos = self.demos()
        self.assertEqual(len(demos), 8)
        for name, (_, _, terminals, _, duration) in demos.items():
            with self.subTest(name=name):
                self.assertTrue(name.startswith('animation-'))
                self.assertGreaterEqual(duration, 8)
                self.assertLessEqual(duration, 12)
                for _, flags, _ in terminals:
                    self.assertEqual(flags[flags.index('--style') + 1], 'granular')
                    self.assertEqual(flags[flags.index('--color-mode') + 1], 'truecolor')

    def test_ambient_motion_holds_before_filling(self):
        for name, (_, _, _, update, duration) in self.demos().items():
            if name in ('animation-milestone-ripple', 'animation-phase-transition'):
                continue
            with self.subTest(name=name):
                self.assertEqual(update(0, 0, duration), '@value 68.00\n')
                self.assertEqual(update(3, 0, duration), '@value 68.00\n')
                self.assertIn('@value 100.00\n', update(duration - 1, 0, duration))

    def test_milestones_are_discrete_and_separated(self):
        _, _, _, update, duration = self.demos()['animation-milestone-ripple']
        events = [(frame / recorder.FPS, update(frame / recorder.FPS, 0, duration))
                  for frame in range(round(duration * recorder.FPS))]
        events = [(elapsed, payload) for elapsed, payload in events if payload]
        self.assertEqual([payload for _, payload in events],
                         [f'@value {value}\n' for value in (10, 25, 50, 75, 100)])
        self.assertTrue(all(b[0] - a[0] >= 1.5 for a, b in zip(events, events[1:])))

    def test_phase_transitions_keep_value_steady(self):
        _, _, _, update, duration = self.demos()['animation-phase-transition']
        events = [update(frame / recorder.FPS, 0, duration)
                  for frame in range(round(duration * recorder.FPS))]
        events = [payload for payload in events if payload]
        self.assertTrue(all(event.startswith('{') and event.count('\n') == 1 for event in events),
                        'phase updates must be atomic json events')
        self.assertEqual([recorder.json.loads(event) for event in events],
                         [{'type': 'value', 'value': 68, 'phase': phase}
                          for phase in ('fetch', 'build', 'test', 'package')])

    def test_aurora_poster_shows_filled_colour(self):
        path = recorder.ROOT / 'assets/demos/animation-aurora.png'
        with recorder.Image.open(path) as image:
            pixel = image.getpixel((recorder.PAD + 10 * recorder.CELL_W, 118))
            self.assertGreater(max(pixel), 80, 'poster captured the empty track before fill settled')


if __name__ == '__main__':
    unittest.main()
