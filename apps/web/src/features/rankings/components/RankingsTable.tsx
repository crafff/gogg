import { useMemo, useState } from "react";
import { useTranslation } from "react-i18next";

import { Tag } from "@shared/ui";
import { cn } from "@shared/lib/cn";
import type { GameAssetManifest } from "../hooks/useGameAssets";
import {
  sortRankings,
  type RankingRow,
  type SortDirection,
  type SortKey,
} from "./rankingSort";

export type { RankingRow } from "./rankingSort";

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
  const [sortKey, setSortKey] = useState<SortKey>("composite");
  const [direction, setDirection] = useState<SortDirection>("desc");
  const sortedItems = useMemo(
    () => sortRankings(items, sortKey, direction),
    [items, sortKey, direction],
  );

  const selectSort = (next: SortKey) => {
    if (next === sortKey) {
      setDirection((current) => (current === "desc" ? "asc" : "desc"));
      return;
    }
    setSortKey(next);
    setDirection("desc");
  };

  return (
    <div className="overflow-hidden rounded-lg border border-border bg-surface-raised">
      <div className="flex items-center justify-end gap-2 border-b border-border px-3 py-2 text-sm">
        <label htmlFor="rankings-sort" className="text-fg-muted">
          {t("sort.label")}
        </label>
        <select
          id="rankings-sort"
          value={sortKey}
          onChange={(event) => selectSort(event.target.value as SortKey)}
          className="rounded border border-border bg-surface-overlay px-2 py-1 text-fg-default"
        >
          {(
            [
              "composite",
              "winRate",
              "pickRate",
              "banRate",
              "kda",
              "games",
            ] as const
          ).map((key) => (
            <option key={key} value={key}>
              {t(`sort.${key}`)}
            </option>
          ))}
        </select>
        <button
          type="button"
          onClick={() =>
            setDirection((current) => (current === "desc" ? "asc" : "desc"))
          }
          className="rounded border border-border px-2 py-1 text-fg-muted"
          aria-label={t("sort.direction")}
        >
          {direction === "desc" ? "↓" : "↑"}
        </button>
      </div>
      <table className="w-full text-left text-sm">
        <thead className="bg-surface-overlay/60 text-xs uppercase tracking-wide text-fg-subtle">
          <tr>
            <Th className="w-12 text-center">{t("column.rank")}</Th>
            <Th>{t("column.champion")}</Th>
            <Th>{t("filter.position")}</Th>
            <SortableTh
              label={t("column.winrate")}
              sort="winRate"
              active={sortKey}
              direction={direction}
              onSort={selectSort}
            />
            <SortableTh
              label={t("column.pickrate")}
              sort="pickRate"
              active={sortKey}
              direction={direction}
              onSort={selectSort}
            />
            <SortableTh
              label={t("column.banrate")}
              sort="banRate"
              active={sortKey}
              direction={direction}
              onSort={selectSort}
            />
            <SortableTh
              label={t("column.kda")}
              sort="kda"
              active={sortKey}
              direction={direction}
              onSort={selectSort}
            />
            <SortableTh
              label={t("column.games")}
              sort="games"
              active={sortKey}
              direction={direction}
              onSort={selectSort}
            />
          </tr>
        </thead>
        <tbody className="divide-y divide-border">
          {sortedItems.map((row, index) => (
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

function SortableTh({
  label,
  sort,
  active,
  direction,
  onSort,
}: {
  label: string;
  sort: SortKey;
  active: SortKey;
  direction: SortDirection;
  onSort: (key: SortKey) => void;
}) {
  return (
    <th
      className="px-3 py-2 text-right font-medium"
      aria-sort={
        active === sort
          ? direction === "desc"
            ? "descending"
            : "ascending"
          : "none"
      }
    >
      <button
        type="button"
        className="whitespace-nowrap hover:text-fg-default"
        onClick={() => onSort(sort)}
      >
        {label} {active === sort ? (direction === "desc" ? "↓" : "↑") : ""}
      </button>
    </th>
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
