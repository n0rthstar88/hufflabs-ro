import os, re, subprocess, sys, tempfile, unittest

HERE = os.path.dirname(os.path.abspath(__file__))
ROOT = os.path.dirname(HERE)
sys.path.insert(0, ROOT)
import gen_mvp as m  # noqa: E402

UP = os.path.join(ROOT, 'upstream')


def entries(text):
    out = {}
    for b in re.split(r'\n  - Id: ', text)[1:]:
        iid = int(b.split('\n', 1)[0])
        out[iid] = dict((k, v) for k, v in re.findall(r'\n    (\w+): (\S+)', b))
    return out


class Tier(unittest.TestCase):
    def test_tier_bounds(self):
        self.assertEqual(m.tier(99), (2, 1, 30))
        self.assertEqual(m.tier(100), (5, 2, 60))
        self.assertEqual(m.tier(149), (5, 2, 60))
        self.assertEqual(m.tier(150), (10, 3.3, 100))


class Generated(unittest.TestCase):
    @classmethod
    def setUpClass(cls):
        out = os.path.join(tempfile.mkdtemp(), 'mob_db.yml')
        subprocess.run([sys.executable, '-I', os.path.join(ROOT, 'gen_mvp.py'), '--upstream', UP, '--out', out],
                       check=True, capture_output=True)
        with open(out) as f:
            cls.gen = entries(f.read())
        with open(os.path.join(UP, 'mob_db.yml')) as f:
            cls.off = entries(f.read())

    def test_only_mvps(self):
        for iid in self.gen:
            self.assertIn('MvpExp', self.off[iid])
        bosses = [i for i, e in self.off.items() if e.get('Class') == 'Boss' and 'MvpExp' not in e]
        self.assertTrue(bosses)
        for iid in bosses:
            self.assertNotIn(iid, self.gen)

    def test_every_mvp_present(self):
        mvps = [i for i, e in self.off.items() if 'MvpExp' in e]
        self.assertEqual(sorted(mvps), sorted(self.gen))

    def test_osiris_low_tier(self):
        osiris = next(i for i, e in self.off.items() if e.get('AegisName') == 'OSIRIS')
        self.assertEqual(int(self.gen[osiris]['Hp']), int(self.off[osiris]['Hp']) * 2)
        self.assertEqual(int(self.gen[osiris]['BaseExp']), int(self.off[osiris]['BaseExp']))
        self.assertEqual(self.gen[osiris]['Level'], self.off[osiris]['Level'])

    def test_top_tier_scaling(self):
        top = next(i for i, e in self.off.items() if 'MvpExp' in e and int(e['Level']) >= 150
                   and int(e['Hp']) * 10 < 2 ** 31 - 1)
        self.assertEqual(int(self.gen[top]['Hp']), int(self.off[top]['Hp']) * 10)
        self.assertEqual(int(self.gen[top]['MvpExp']), round(int(self.off[top]['MvpExp']) * 3.3))
        self.assertEqual(int(self.gen[top]['JobExp']), round(int(self.off[top]['JobExp']) * 3.3))

    def test_hp_capped(self):
        for e in self.gen.values():
            self.assertLessEqual(int(e['Hp']), 2 ** 31 - 1)


if __name__ == '__main__':
    unittest.main()
