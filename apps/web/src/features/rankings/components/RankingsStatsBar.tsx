import { Trans, useTranslation } from "react-i18next";

import { Tag } from "@shared/ui";

export interface RankingsStatsBarProps {
  totalMatches: number;
  /** Echoed version from the resolved query — null when "latest" was used and the API didn't echo. */
  resolvedVersion: string | null;
  /** Region from the committed filter, used when the response doesn't echo. Empty = all regions. */
  region: string;
}

/**
 * Single-line stats bar above the rankings table — context for the
 * numbers below. Renders nothing when the query hasn't matched any
 * games yet (loading or empty result).
 */
export function RankingsStatsBar({
  totalMatches,
  resolvedVersion,
  region,
}: RankingsStatsBarProps) {
  const { t } = useTranslation("rankings");

  if (totalMatches <= 0) return null;

  return (
    <p className="text-sm text-fg-muted">
      <Trans
        ns="rankings"
        i18nKey="subtitle"
        values={{ count: totalMatches.toLocaleString() }}
        components={{ strong: <strong className="text-fg-default" /> }}
      />
      {resolvedVersion && (
        <>
          <span className="mx-2 text-fg-subtle">·</span>
          <Tag tone="accent">{resolvedVersion}</Tag>
        </>
      )}
      {region && (
        <>
          <span className="mx-2 text-fg-subtle">·</span>
          <Tag>{region}</Tag>
        </>
      )}
      {!region && (
        <>
          <span className="mx-2 text-fg-subtle">·</span>
          <Tag>{t("filter.all")}</Tag>
        </>
      )}
    </p>
  );
}
