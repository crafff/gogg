import { useTranslation } from "react-i18next";

import { Tag } from "@shared/ui";
import { cn } from "@shared/lib/cn";
import type { ChampionRankingsQuery } from "@shared/api";
import type { GameAssetManifest } from "../hooks/useGameAssets";

export type RankingRow =
  ChampionRankingsQuery["championRankings"]["items"][number];

export interface RankingsTableProps {
  items: ReadonlyArray<RankingRow>;
  assets?: GameAssetManifest | null;
  assetBaseURL?: string;
}

/**
 * Dense scrollable table rendering one champion per row. Visual
 * details kept minimal in chunk 4 — champion portraits + tier
 * badges land in chunk 5 once the static asset pipeline is wired up.
 */
export function RankingsTable({
  items,
  assets,
  assetBaseURL = "",
}: RankingsTableProps) {
  const { t } = useTranslation(["rankings", "common"]);

  return (
    <div className="overflow-hidden rounded-lg border border-border bg-surface-raised">
      <table className="w-full text-left text-sm">
        <thead className="bg-surface-overlay/60 text-xs uppercase tracking-wide text-fg-subtle">
          <tr>
            <Th className="w-12 text-center">{t("column.rank")}</Th>
            <Th>{t("column.champion")}</Th>
            <Th>{t("filter.position")}</Th>
            <Th className="text-right">{t("column.winrate")}</Th>
            <Th className="text-right">{t("column.pickrate")}</Th>
            <Th className="text-right">{t("column.banrate")}</Th>
            <Th className="text-right">{t("column.kda")}</Th>
            <Th className="text-right">{t("column.games")}</Th>
          </tr>
        </thead>
        <tbody className="divide-y divide-border">
          {items.map((row, index) => (
            <Row
              key={`${row.championId}-${row.teamPosition.join(",")}`}
              row={row}
              index={index}
              assets={assets}
              assetBaseURL={assetBaseURL}
            />
          ))}
        </tbody>
      </table>
    </div>
  );
}

function Th({
  children,
  className,
}: React.HTMLAttributes<HTMLTableCellElement>) {
  return <th className={cn("px-3 py-2 font-medium", className)}>{children}</th>;
}

function Row({
  row,
  index,
  assets,
  assetBaseURL,
}: {
  row: RankingRow;
  index: number;
  assets?: GameAssetManifest | null;
  assetBaseURL: string;
}) {
  const { t, i18n } = useTranslation("rankings");
  const champion = assets?.champions[String(row.championId)];
  const locale = i18n.language.toLowerCase().replace("-", "_");
  const championName = champion?.names[locale] ?? row.championName;

  return (
    <tr className="hover:bg-surface-overlay/40">
      <td className="px-3 py-2 text-center font-mono text-fg-muted">
        {index + 1}
      </td>
      <td className="px-3 py-2 font-medium text-fg-default">
        <div className="flex items-center gap-2">
          {champion && (
            <img
              src={`${assetBaseURL}/${champion.image}`}
              alt=""
              loading="lazy"
              className="h-8 w-8 rounded object-cover"
            />
          )}
          <span>{championName}</span>
        </div>
      </td>
      <td className="px-3 py-2">
        <div className="flex flex-wrap gap-1">
          {row.teamPosition.map((pos) => (
            <Tag key={pos} size="sm">
              {assets?.positions?.[pos.toLowerCase()] && (
                <img
                  src={`${assetBaseURL}/${assets.positions[pos.toLowerCase()]}`}
                  alt=""
                  className="mr-1 inline-block h-3.5 w-3.5"
                />
              )}
              {t(`position.${pos}` as const, { defaultValue: pos })}
            </Tag>
          ))}
        </div>
      </td>
      <PercentCell value={row.winRate} />
      <PercentCell value={row.pickRate} />
      <PercentCell value={row.banRate} />
      <td className="px-3 py-2 text-right font-mono">{row.kda.toFixed(2)}</td>
      <td className="px-3 py-2 text-right font-mono text-fg-muted">
        {row.games.toLocaleString()}
      </td>
    </tr>
  );
}

function PercentCell({ value }: { value: number }) {
  // The rankings API already returns percentage points (for example 53.1).
  return (
    <td className="px-3 py-2 text-right font-mono">{value.toFixed(1)}%</td>
  );
}
