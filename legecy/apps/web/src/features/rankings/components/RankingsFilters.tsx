import { useTranslation } from "react-i18next";

import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@shared/ui";
import { cn } from "@shared/lib/cn";

import type {
  Position,
  RankingsFiltersState,
  UiTier,
} from "../hooks/useRankingsFilters";

const POSITIONS: Position[] = [
  "",
  "TOP",
  "JUNGLE",
  "MIDDLE",
  "BOTTOM",
  "UTILITY",
];
const TIERS: UiTier[] = [
  "",
  "challenger",
  "grandmaster_plus",
  "grandmaster",
  "master_plus",
  "master",
];

export interface RankingsFiltersProps {
  selected: RankingsFiltersState;
  availableVersions: string[];
  availableRegions: string[];
  onPositionChange: (next: Position) => void;
  onTierChange: (next: UiTier) => void;
  onRegionChange: (next: string) => void;
  onVersionChange: (next: string) => void;
}

/**
 * Filter row for the rankings page. The animated sliding indicator
 * from the legacy FiltersPanel is intentionally NOT ported here —
 * keeping this component declarative for the first cut. The slider
 * is a chunk 6 polish item.
 */
export function RankingsFilters({
  selected,
  availableVersions,
  availableRegions,
  onPositionChange,
  onTierChange,
  onRegionChange,
  onVersionChange,
}: RankingsFiltersProps) {
  const { t } = useTranslation(["rankings", "common"]);

  return (
    <div className="space-y-3">
      <SegmentRow
        label={t("filter.position")}
        values={POSITIONS}
        active={selected.position}
        onChange={onPositionChange}
        renderLabel={(value) =>
          value === "" ? t("filter.all") : t(`position.${value}` as const)
        }
      />
      <SegmentRow
        label={t("filter.tier")}
        values={TIERS}
        active={selected.tier}
        onChange={onTierChange}
        renderLabel={(value) =>
          value === "" ? t("filter.all") : t(`tier.${value}` as const)
        }
      />
      <div className="flex flex-wrap gap-3">
        <DropdownField
          label={t("filter.region")}
          value={selected.region}
          placeholder={t("filter.all")}
          options={[
            { value: "", label: t("filter.all") },
            ...availableRegions.map((r) => ({ value: r, label: r })),
          ]}
          onChange={onRegionChange}
        />
        <DropdownField
          label={t("filter.version")}
          value={selected.version}
          placeholder={t("filter.latest")}
          options={[
            { value: "latest", label: t("filter.latest") },
            ...availableVersions.map((v) => ({ value: v, label: v })),
          ]}
          onChange={onVersionChange}
        />
      </div>
    </div>
  );
}

interface SegmentRowProps<T extends string> {
  label: string;
  values: readonly T[];
  active: T;
  renderLabel: (value: T) => string;
  onChange: (value: T) => void;
}

function SegmentRow<T extends string>({
  label,
  values,
  active,
  renderLabel,
  onChange,
}: SegmentRowProps<T>) {
  return (
    <div className="flex flex-wrap items-center gap-2 text-sm">
      <span className="w-14 shrink-0 text-fg-subtle">{label}</span>
      <div className="flex flex-wrap gap-1 rounded-lg border border-border bg-surface-raised p-1">
        {values.map((value) => {
          const isActive = value === active;
          return (
            <button
              key={value || "__all__"}
              type="button"
              onClick={() => onChange(value)}
              aria-pressed={isActive}
              className={cn(
                "rounded px-3 py-1 text-xs font-medium transition",
                "focus-visible:outline-none focus-visible:shadow-focus-ring",
                isActive
                  ? "bg-accent text-fg-inverse shadow-sm"
                  : "text-fg-muted hover:bg-surface-overlay hover:text-fg-default",
              )}
            >
              {renderLabel(value)}
            </button>
          );
        })}
      </div>
    </div>
  );
}

interface DropdownFieldProps {
  label: string;
  value: string;
  placeholder: string;
  options: ReadonlyArray<{ value: string; label: string }>;
  onChange: (value: string) => void;
}

// Radix Select bans the empty string as an Item value (it conflicts
// with the "no selection" sentinel internally), so we encode the
// "all" / "no filter" choice as ALL_SENTINEL and round-trip it at the
// component boundary.
const ALL_SENTINEL = "__all__";

function DropdownField({
  label,
  value,
  placeholder,
  options,
  onChange,
}: DropdownFieldProps) {
  const radixValue = value === "" ? ALL_SENTINEL : value;
  return (
    <label className="inline-flex items-center gap-2 text-sm">
      <span className="text-fg-subtle">{label}</span>
      <Select
        value={radixValue}
        onValueChange={(next) => onChange(next === ALL_SENTINEL ? "" : next)}
      >
        <SelectTrigger className="w-32" aria-label={label}>
          <SelectValue placeholder={placeholder} />
        </SelectTrigger>
        <SelectContent>
          {options.map((opt) => (
            <SelectItem
              key={opt.value || ALL_SENTINEL}
              value={opt.value || ALL_SENTINEL}
            >
              {opt.label}
            </SelectItem>
          ))}
        </SelectContent>
      </Select>
    </label>
  );
}
