import type { ChampionRankingItem } from "@shared-types";
import styles from "./RankingsTable.module.css";

type PositionIconTheme = "default" | "red" | "light";

// Quick switch for icon style comparison: "default" | "red" | "light"
const POSITION_ICON_THEME: PositionIconTheme = "light";

const championImageModules = import.meta.glob(
  "/src/assets/images/champions/square/*.png",
  { eager: true, import: "default" }
) as Record<string, string>;

const positionIconModules = import.meta.glob(
  "/src/assets/positions/svg/position-*.svg",
  { eager: true, import: "default" }
) as Record<string, string>;

function normalizeChampionName(value: string): string {
  return value.toLowerCase().replace(/[^a-z0-9]/g, "");
}

const championImageMap = new Map<string, string>(
  Object.entries(championImageModules).map(([filePath, imageUrl]) => {
    const fileName = filePath.split("/").pop() ?? "";
    const baseName = fileName.replace(/\.png$/i, "");
    return [normalizeChampionName(baseName), imageUrl];
  })
);

interface RankingsTableProps {
  items: ChampionRankingItem[];
}

function formatRate(value: number): string {
  return `${value.toFixed(1)}%`;
}

function formatPositions(values: string[]): string {
  const map: Record<string, string> = {
    TOP: "上单",
    JUNGLE: "打野",
    MIDDLE: "中路",
    BOTTOM: "下路",
    UTILITY: "辅助",
  };

  if (values.length === 0) {
    return "-";
  }

  return values.map((value) => map[value] ?? value).join(" / ");
}

function toPositionIconBase(position: string): string | null {
  const map: Record<string, string> = {
    TOP: "top",
    JUNGLE: "jungle",
    MIDDLE: "middle",
    BOTTOM: "bottom",
    UTILITY: "utility",
  };

  return map[position] ?? null;
}

function getPositionIconUrl(position: string, theme: PositionIconTheme): string | null {
  const base = toPositionIconBase(position);
  if (!base) {
    return null;
  }

  const suffix = theme === "default" ? "" : `-${theme}`;
  const path = `/src/assets/positions/svg/position-${base}${suffix}.svg`;
  return positionIconModules[path] ?? null;
}

function getChampionImageUrl(championName: string): string | null {
  const normalized = normalizeChampionName(championName);
  return championImageMap.get(normalized) ?? null;
}

function getChampionInitial(championName: string): string {
  const trimmed = championName.trim();
  return trimmed.length > 0 ? trimmed[0].toUpperCase() : "?";
}

export function RankingsTable({ items }: RankingsTableProps) {
  if (items.length === 0) {
    return null;
  }
  const rows = items;

  return (
    <div className={styles.wrapper}>
      <table className={styles.table}>
        <colgroup>
          <col className={styles.rankCol} />
          <col className={styles.positionCol} />
          <col className={styles.heroCol} />
          <col className={styles.rateCol} />
          <col className={styles.rateCol} />
          <col className={styles.rateCol} />
        </colgroup>
        <thead>
          <tr>
            <th className={styles.rankHeader}>排名</th>
            <th className={styles.positionHeader}>位置</th>
            <th className={styles.heroHeader}>英雄</th>
            <th className={styles.rateHeader}>胜率</th>
            <th className={styles.rateHeader}>登场率</th>
            <th className={styles.rateHeader}>BAN率</th>
          </tr>
        </thead>
        <tbody>
          {rows.map((row, index) => {
            const championImageUrl = getChampionImageUrl(row.championName);

            return (
              <tr key={`${row.championId}-${index}`}>
                <td className={styles.rankCell}>{index + 1}</td>
                <td className={styles.positionCell}>
                  {row.teamPosition.length > 0 ? (
                    <span className={styles.positionIcons}>
                      {row.teamPosition.map((position) => {
                        const iconUrl = getPositionIconUrl(position, POSITION_ICON_THEME);

                        if (!iconUrl) {
                          return (
                            <span key={`${row.championId}-${position}`} className={styles.positionFallback}>
                              {formatPositions([position])}
                            </span>
                          );
                        }

                        return (
                          <img
                            key={`${row.championId}-${position}`}
                            src={iconUrl}
                            alt={formatPositions([position])}
                            title={formatPositions([position])}
                            className={styles.positionIcon}
                            loading="lazy"
                          />
                        );
                      })}
                    </span>
                  ) : (
                    <span className={styles.positionFallback}>-</span>
                  )}
                </td>
                <td className={styles.nameCell}>
                  <span className={styles.championCell}>
                    {championImageUrl ? (
                      <img
                        src={championImageUrl}
                        alt={row.championName}
                        className={styles.championAvatar}
                        loading="lazy"
                      />
                    ) : (
                      <span className={styles.championFallback}>
                        {getChampionInitial(row.championName)}
                      </span>
                    )}
                    <span>{row.championName}</span>
                  </span>
                </td>
                <td>{formatRate(row.winRate)}</td>
                <td>{formatRate(row.pickRate)}</td>
                <td>{formatRate(row.banRate)}</td>
              </tr>
            );
          })}
        </tbody>
      </table>
    </div>
  );
}
