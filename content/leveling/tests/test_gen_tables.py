import os, sys, unittest, tempfile, subprocess, re

HERE = os.path.dirname(os.path.abspath(__file__))
ROOT = os.path.dirname(HERE)
sys.path.insert(0, ROOT)
import gen_tables as g  # noqa: E402

UP = os.path.join(ROOT, 'upstream')


class SoftCurve(unittest.TestCase):
    def setUp(self):
        # official steps for levels 1..274 (drop the sentinel at 275)
        self.base = g.read_group_steps(os.path.join(UP, 'job_exp.yml'), 'MaxBaseLevel', 'BaseExp', 'Dragon_Knight')[:274]
        self.target = 10 ** 13

    def test_keeps_official_below_keep_through(self):
        c = g.soft_curve(self.base, 210, 500, self.target, (275, 500))
        self.assertEqual(c[:209], self.base[:209])

    def test_length_is_one_step_per_level_below_max(self):
        c = g.soft_curve(self.base, 210, 500, self.target, (275, 500))
        self.assertEqual(len(c), 499)

    def test_window_sum_hits_target(self):
        c = g.soft_curve(self.base, 210, 500, self.target, (275, 500))
        got = sum(c[274:499])
        self.assertLess(abs(got - self.target) / self.target, 0.005)

    def test_monotonic_from_keep_through(self):
        c = g.soft_curve(self.base, 210, 500, self.target, (275, 500))
        for a, b in zip(c[208:], c[209:]):
            self.assertLessEqual(a, b)

    def test_impossible_target_raises(self):
        with self.assertRaises(ValueError):
            g.soft_curve(self.base, 210, 500, 1, (275, 500))


class StatPoints(unittest.TestCase):
    def test_reach_target(self):
        official = list(range(48, 48 + 275 * 3, 3))  # 275 cumulative entries
        pts = g.stat_points(official, 500, 387)
        self.assertEqual(len(pts), 500)
        self.assertEqual(pts[:275], official)
        self.assertEqual(pts[499] - pts[274], 387 * 225)

    def test_per_level_constant(self):
        self.assertEqual(g.STAT_POINTS_PER_LEVEL, 387)


class XpBudget(unittest.TestCase):
    def test_rate_uses_top_20_non_boss(self):
        base, job = g.xp_per_hour_1x(os.path.join(UP, 'mob_db.yml'))
        self.assertGreater(base, 10 ** 8)
        self.assertGreater(job, 10 ** 8)


class Generated(unittest.TestCase):
    @classmethod
    def setUpClass(cls):
        cls.out = tempfile.mkdtemp()
        subprocess.run([sys.executable, '-I', os.path.join(ROOT, 'gen_tables.py'), '--upstream', UP, '--out', cls.out], check=True, capture_output=True)
        cls.jexp = open(os.path.join(cls.out, 'job_stats.yml')).read()

    def test_only_final_classes_raised(self):
        groups = re.split(r'\n  - Jobs:', self.jexp)[1:]
        raised = [grp for grp in groups if 'MaxBaseLevel: 500' in grp or 'MaxJobLevel: 250' in grp]
        self.assertEqual(len(raised), 2)
        for grp in raised:
            jobs = re.findall(r'\n      (\w+): true', '\n' + grp.split('\n    Max')[0])
            self.assertEqual(sorted(jobs), sorted(g.FINAL_CLASSES))
        self.assertEqual(len(g.FINAL_CLASSES), 25)

    def test_base_table_has_500_levels(self):
        grp = [x for x in re.split(r'\n  - Jobs:', self.jexp) if 'MaxBaseLevel: 500' in x][0]
        self.assertEqual(len(re.findall(r'- Level: ', grp)), 500)

    def test_job_table_has_250_levels(self):
        grp = [x for x in re.split(r'\n  - Jobs:', self.jexp) if 'MaxJobLevel: 250' in x][0]
        self.assertEqual(len(re.findall(r'- Level: ', grp)), 250)

    def test_statpoint_has_500_levels(self):
        sp = open(os.path.join(self.out, 'statpoint.yml')).read()
        self.assertEqual(len(re.findall(r'- Level: ', sp)), 500)

    def test_statpoint_low_levels_match_official(self):
        sp = open(os.path.join(self.out, 'statpoint.yml')).read()
        off = open(os.path.join(UP, 'statpoint.yml')).read()
        first = lambda t: re.search(r'- Level: 1\n    Points: (\d+)', t).group(1)
        self.assertEqual(first(sp), first(off))
        self.assertEqual(re.search(r'- Level: 275\n    Points: (\d+)', sp).group(1),
                         re.search(r'- Level: 275\n    Points: (\d+)', off).group(1))

    def test_trait_points_stay_official_above_275(self):
        sp = open(os.path.join(self.out, 'statpoint.yml')).read()
        rows = re.findall(r'- Level: (\d+)\n    Points: \d+(?:\n    TraitPoints: (\d+))?', sp)
        traits = [int(t or 0) for _, t in rows]
        self.assertEqual(len(traits), 500)
        self.assertEqual(set(traits[275:]), {traits[274]})


if __name__ == '__main__':
    unittest.main()
