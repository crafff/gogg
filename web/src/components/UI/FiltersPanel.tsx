import { useEffect, useRef, useState } from "react";
import type { Position, Tier } from "@shared-types";
import styles from "./FiltersPanel.module.css";

interface FiltersPanelProps {
  position: Position;
  onPositionChange: (position: Position) => void;
  tier: Tier;
  onTierChange: (tier: Tier) => void;
  region: string;
  onRegionChange: (region: string) => void;
  version: string;
  onVersionChange: (version: string) => void;
  availableRegions: string[];
  availableVersions: string[];
}

export function FiltersPanel({
  position, onPositionChange,
  tier, onTierChange,
  region, onRegionChange,
  version, onVersionChange,
  availableRegions,
  availableVersions,
}: FiltersPanelProps) {
  const positionOptions: Array<{ value: Position; label: string }> = [
    { value: "", label: "全部位置" },
    { value: "TOP", label: "上单" },
    { value: "JUNGLE", label: "打野" },
    { value: "MIDDLE", label: "中路" },
    { value: "BOTTOM", label: "下路" },
    { value: "UTILITY", label: "辅助" },
  ];

  const tierOptions: Array<{ value: Tier; label: string }> = [
    { value: "", label: "全部段位" },
    { value: "challenger", label: "王者" },
    { value: "grandmaster_plus", label: "宗师+" },
    { value: "grandmaster", label: "宗师" },
    { value: "master_plus", label: "大师+" },
    { value: "master", label: "大师" },
  ];

  const posActiveIndex = Math.max(
    0,
    positionOptions.findIndex((o) => o.value === position)
  );
  const tierActiveIndex = Math.max(
    0,
    tierOptions.findIndex((o) => o.value === tier)
  );

  const posButtonRefs = useRef<Array<HTMLButtonElement | null>>([]);
  const tierButtonRefs = useRef<Array<HTMLButtonElement | null>>([]);
  const [posSlider, setPosSlider] = useState({ left: 0, width: 0, ready: false });
  const [tierSlider, setTierSlider] = useState({ left: 0, width: 0, ready: false });

  useEffect(() => {
    const update = () => {
      const btn = posButtonRefs.current[posActiveIndex];
      if (btn) setPosSlider({ left: btn.offsetLeft, width: btn.offsetWidth, ready: true });
    };
    update();
    window.addEventListener("resize", update);
    return () => window.removeEventListener("resize", update);
  }, [posActiveIndex]);

  useEffect(() => {
    const update = () => {
      const btn = tierButtonRefs.current[tierActiveIndex];
      if (btn) setTierSlider({ left: btn.offsetLeft, width: btn.offsetWidth, ready: true });
    };
    update();
    window.addEventListener("resize", update);
    return () => window.removeEventListener("resize", update);
  }, [tierActiveIndex]);

  return (
    <div className={styles.panel}>
      {/* Position */}
      <div className={styles.filterRow}>
        <div className={styles.chipBar}>
          <div className={styles.chipGroup} style={{ gridTemplateColumns: `repeat(${positionOptions.length}, minmax(84px, 1fr))` }}>
            <div
              className={styles.slider}
              style={{ transform: `translateX(${posSlider.left}px)`, width: `${posSlider.width}px`, opacity: posSlider.ready ? 1 : 0 }}
            />
            {positionOptions.map(({ value, label }, i) => (
              <button
                key={value || "ALL_POS"}
                type="button"
                ref={(el) => { posButtonRefs.current[i] = el; }}
                onClick={() => onPositionChange(value)}
                className={`${styles.chip} ${position === value ? styles.active : ""}`}
              >
                {label}
              </button>
            ))}
          </div>
        </div>
      </div>

      {/* Tier */}
      <div className={styles.filterRow}>
        <div className={styles.chipBar}>
          <div className={styles.chipGroup} style={{ gridTemplateColumns: `repeat(${tierOptions.length}, minmax(84px, 1fr))` }}>
            <div
              className={styles.slider}
              style={{ transform: `translateX(${tierSlider.left}px)`, width: `${tierSlider.width}px`, opacity: tierSlider.ready ? 1 : 0 }}
            />
            {tierOptions.map(({ value, label }, i) => (
              <button
                key={value || "ALL_TIER"}
                type="button"
                ref={(el) => { tierButtonRefs.current[i] = el; }}
                onClick={() => onTierChange(value)}
                className={`${styles.chip} ${tier === value ? styles.active : ""}`}
              >
                {label}
              </button>
            ))}
          </div>
        </div>
      </div>

      {/* Region + Version */}
      <div className={styles.filterRow}>
        <div className={styles.selectGroup}>
          <label className={styles.selectLabel}>地区</label>
          <select
            className={styles.select}
            value={region}
            onChange={(e) => onRegionChange(e.target.value)}
          >
            <option value="">全部地区</option>
            {availableRegions.map((r) => (
              <option key={r} value={r}>{r}</option>
            ))}
          </select>
        </div>

        <div className={styles.selectGroup}>
          <label className={styles.selectLabel}>版本</label>
          <select
            className={styles.select}
            value={version}
            onChange={(e) => onVersionChange(e.target.value)}
          >
            <option value="latest">最新版本</option>
            {availableVersions.map((v) => (
              <option key={v} value={v}>{v}</option>
            ))}
          </select>
        </div>
      </div>
    </div>
  );
}
