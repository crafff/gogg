import { useTranslation } from "react-i18next";

import { Tag } from "@shared/ui";
import { cn } from "@shared/lib/cn";
import type { ChampionRankingsQuery } from "@shared/api";

export type RankingRow =
  ChampionRankingsQuery["championRankings"]["items"][number];

export interface RankingsTableProps {
  items: ReadonlyArray<RankingRow>;
}

/**
 * Dense scrollable table rendering one champion per row. Visual
 * details kept minimal in chunk 4 — champion portraits + tier
 * badges land in chunk 5 once the static asset pipeline is wired up.
 */
export function RankingsTable({ items }: RankingsTableProps) {
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

function Row({ row, index }: { row: RankingRow; index: number }) {
  const { t } = useTranslation("rankings");

  return (
    <tr className="hover:bg-surface-overlay/40">
      <td className="px-3 py-2 text-center font-mono text-fg-muted">
        {index + 1}
      </td>
      <td className="px-3 py-2 font-medium text-fg-default">
        {row.championName}
      </td>
      <td className="px-3 py-2">
        <div className="flex flex-wrap gap-1">
          {row.teamPosition.map((pos) => (
            <Tag key={pos} size="sm">
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
  // value is a fraction in [0, 1]; format as 53.1% to match legacy.
  return (
    <td className="px-3 py-2 text-right font-mono">
      {(value * 100).toFixed(1)}%
    </td>
  );
}
