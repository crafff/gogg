import sys
import unittest
from pathlib import Path

import pandas as pd

sys.path.insert(0, str(Path(__file__).resolve().parents[1]))
from features import FEATURES, add_features  # noqa: E402


class FeatureTest(unittest.TestCase):
    def test_constructs_tempo_and_enemy_gaps_without_mutating_input(self):
        stems = (
            "gold", "current_gold", "jungle_cs", "lane_cs", "level", "xp",
            "time_enemy_cc", "wards_placed", "wards_killed", "dmg_total",
            "dmg_to_champs", "dmg_magic_champs", "dmg_phys_champs",
            "dmg_true_champs", "dmg_taken",
        )
        row = {f"{stem}_10": 100 for stem in stems}
        row.update({f"{stem}_15": 300 for stem in stems})
        for stem in (
            "gold", "current_gold", "jungle_cs", "lane_cs", "level", "xp",
            "time_enemy_cc", "wards_placed", "wards_killed", "dmg_total",
            "dmg_to_champs", "dmg_taken",
        ):
            row[f"enemy_{stem}_15"] = 250
        row.update({
            "gold_10": 3000, "gold_15": 5000, "enemy_gold_15": 4600,
            "dmg_taken_10": 1200, "dmg_taken_15": 2800,
            "pos_x_10": 0, "pos_y_10": 0, "pos_x_15": 3, "pos_y_15": 4,
            "first_purchase_ms": 60000, "last_purchase_ms": 600000,
            "item_purchases_15": 8, "distinct_items_15": 6,
            "item_undos_15": 0, "items_sold_15": 1,
            "q_points_15": 5, "w_points_15": 1, "e_points_15": 4, "r_points_15": 2,
        })
        source = pd.DataFrame([row])
        result = add_features(source)
        self.assertEqual(result.loc[0, "gold_gain_10_15"], 2000)
        self.assertEqual(result.loc[0, "gold_vs_enemy_jungle_15"], 400)
        self.assertEqual(result.loc[0, "dmg_taken_gain_10_15"], 1600)
        self.assertEqual(result.loc[0, "position_displacement_10_15"], 5)
        self.assertEqual(result.loc[0, "first_purchase_minute"], 1)
        self.assertEqual(result.loc[0, "e_priority_over_q_15"], 0)
        self.assertEqual(result.loc[0, "w_extra_points_15"], 0)
        self.assertNotIn("gold_gain_10_15", source.columns)
        self.assertFalse(set(FEATURES) - set(result.columns))


if __name__ == "__main__":
    unittest.main()
