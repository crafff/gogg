import { useTranslation } from "react-i18next";

import type { SummonerQuery } from "@shared/api";
import { Button } from "@shared/ui";

import { LocalAssetImage } from "./LocalAssetImage";

type SummonerResult = NonNullable<SummonerQuery["summoner"]>;

interface SummonerProfileHeaderProps {
  profile: SummonerResult["profile"];
  ranks: SummonerResult["ranks"];
  refreshing: boolean;
  refreshDisabled: boolean;
  onRefresh: () => void;
}

export function SummonerProfileHeader({
  profile,
  ranks,
  refreshing,
  refreshDisabled,
  onRefresh,
}: SummonerProfileHeaderProps) {
  const { t, i18n } = useTranslation("summoner");
  const rankByQueue = new Map(ranks.map((rank) => [rank.queueType, rank]));
  const refreshedAt = profile.lastRefreshedAt
    ? new Intl.DateTimeFormat(i18n.language, {
        dateStyle: "medium",
        timeStyle: "short",
      }).format(new Date(profile.lastRefreshedAt))
    : t("profile.never");

  return (
    <header className="rounded-xl border border-border bg-surface-raised p-4 shadow-card sm:p-6">
      <div className="flex flex-col gap-5 sm:flex-row sm:items-center">
        <LocalAssetImage
          src={`/game-assets/profile-icons/${profile.profileIconId}.jpg`}
          alt={profile.gameName}
          fallback={profile.gameName.slice(0, 1).toUpperCase()}
          className="h-20 w-20 rounded-xl border border-border-strong object-cover"
        />
        <div className="min-w-0 flex-1">
          <h1 className="truncate text-2xl font-semibold text-fg-default sm:text-3xl">
            {profile.gameName}
            <span className="font-normal text-fg-muted">
              #{profile.tagLine}
            </span>
          </h1>
          <p className="mt-1 text-sm text-fg-muted">
            {profile.region} ·{" "}
            {t("profile.level", { level: profile.summonerLevel })}
          </p>
          <p className="mt-1 text-xs text-fg-subtle">
            {t("profile.updated", { time: refreshedAt })}
          </p>
        </div>
        <Button
          variant="secondary"
          onClick={onRefresh}
          disabled={refreshing || refreshDisabled}
          className="shrink-0"
        >
          {refreshing ? t("refresh.running") : t("refresh.button")}
        </Button>
      </div>
      <div className="mt-5 grid gap-3 sm:grid-cols-2">
        <RankCard
          label={t("rank.solo")}
          rank={rankByQueue.get("RANKED_SOLO_5x5")}
        />
        <RankCard
          label={t("rank.flex")}
          rank={rankByQueue.get("RANKED_FLEX_SR")}
        />
      </div>
    </header>
  );
}

function RankCard({
  label,
  rank,
}: {
  label: string;
  rank: SummonerResult["ranks"][number] | undefined;
}) {
  const { t } = useTranslation("summoner");
  return (
    <div className="rounded-lg border border-border bg-surface-overlay/50 p-3">
      <p className="text-xs font-medium uppercase tracking-wide text-fg-muted">
        {label}
      </p>
      {rank ? (
        <div className="mt-1 flex flex-wrap items-baseline justify-between gap-2">
          <p className="font-semibold capitalize text-fg-default">
            {rank.tier.toLowerCase()} {rank.division} · {rank.leaguePoints} LP
          </p>
          <p className="text-xs text-fg-muted">
            {t("rank.record", {
              wins: rank.wins,
              losses: rank.losses,
              winRate: rank.winRate.toFixed(1),
            })}
          </p>
        </div>
      ) : (
        <p className="mt-1 font-medium text-fg-subtle">{t("rank.unranked")}</p>
      )}
    </div>
  );
}
