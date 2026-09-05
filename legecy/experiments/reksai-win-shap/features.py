"""Feature definitions kept separate so they can be tested without ML packages."""

FEATURES = [
    "gold_15", "jungle_cs_15", "lane_cs_15", "level_15", "xp_15",
    "current_gold_15", "unspent_gold_ratio_15", "time_enemy_cc_15",
    "dmg_total_15", "dmg_to_champs_15", "dmg_magic_champs_15",
    "dmg_phys_champs_15", "dmg_true_champs_15", "dmg_taken_15",
    "wards_placed_15", "wards_killed_15", "position_displacement_10_15",
    "gold_gain_10_15", "current_gold_gain_10_15", "jungle_cs_gain_10_15",
    "lane_cs_gain_10_15", "level_gain_10_15", "xp_gain_10_15",
    "time_enemy_cc_gain_10_15", "wards_placed_gain_10_15", "wards_killed_gain_10_15",
    "dmg_total_gain_10_15", "dmg_to_champs_gain_10_15",
    "dmg_magic_champs_gain_10_15", "dmg_phys_champs_gain_10_15",
    "dmg_true_champs_gain_10_15", "dmg_taken_gain_10_15",
    "gold_vs_enemy_jungle_15", "jungle_cs_vs_enemy_jungle_15",
    "current_gold_vs_enemy_jungle_15", "lane_cs_vs_enemy_jungle_15",
    "level_vs_enemy_jungle_15", "xp_vs_enemy_jungle_15", "time_enemy_cc_vs_enemy_jungle_15",
    "wards_placed_vs_enemy_jungle_15", "wards_killed_vs_enemy_jungle_15",
    "dmg_total_vs_enemy_jungle_15", "dmg_to_champs_vs_enemy_jungle_15",
    "dmg_taken_vs_enemy_jungle_15",
    "item_purchases_15", "distinct_items_15", "item_undos_15", "items_sold_15",
    "first_purchase_minute", "last_purchase_minute",
    "q_points_15", "w_points_15", "e_points_15", "r_points_15",
]


def add_features(frame):
    result = frame.copy()
    for stem in (
        "gold", "current_gold", "jungle_cs", "lane_cs", "level", "xp",
        "time_enemy_cc", "wards_placed", "wards_killed", "dmg_total",
        "dmg_to_champs", "dmg_magic_champs", "dmg_phys_champs",
        "dmg_true_champs", "dmg_taken",
    ):
        result[f"{stem}_gain_10_15"] = result[f"{stem}_15"] - result[f"{stem}_10"]
    for stem in (
        "gold", "current_gold", "jungle_cs", "lane_cs", "level", "xp",
        "time_enemy_cc", "wards_placed", "wards_killed", "dmg_total",
        "dmg_to_champs", "dmg_taken",
    ):
        result[f"{stem}_vs_enemy_jungle_15"] = (
            result[f"{stem}_15"] - result[f"enemy_{stem}_15"]
        )
    result["unspent_gold_ratio_15"] = result["current_gold_15"] / result["gold_15"].clip(lower=1)
    result["position_displacement_10_15"] = (
        (result["pos_x_15"] - result["pos_x_10"]) ** 2
        + (result["pos_y_15"] - result["pos_y_10"]) ** 2
    ) ** 0.5
    result["first_purchase_minute"] = result["first_purchase_ms"] / 60000
    result["last_purchase_minute"] = result["last_purchase_ms"] / 60000
    result["e_priority_over_q_15"] = (result["e_points_15"] > result["q_points_15"]).astype(int)
    result["w_extra_points_15"] = (result["w_points_15"] > 1).astype(int)
    return result
