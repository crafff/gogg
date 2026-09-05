import { useId, useState } from "react";
import { useTranslation } from "react-i18next";

import type { SummonerQuery } from "@shared/api";

import { LocalAssetImage } from "./LocalAssetImage";

export type Match = NonNullable<
  SummonerQuery["summoner"]
>["history"]["items"][number];
type Participant = Match["participants"][number];

const RANK_TIER_KEYS = {
  IRON: "rank.tier.IRON",
  BRONZE: "rank.tier.BRONZE",
  SILVER: "rank.tier.SILVER",
  GOLD: "rank.tier.GOLD",
  PLATINUM: "rank.tier.PLATINUM",
  EMERALD: "rank.tier.EMERALD",
  DIAMOND: "rank.tier.DIAMOND",
  MASTER: "rank.tier.MASTER",
  GRANDMASTER: "rank.tier.GRANDMASTER",
  CHALLENGER: "rank.tier.CHALLENGER",
  UNRANKED: "rank.tier.UNRANKED",
} as const;

export function SummonerMatchCard({ match }: { match: Match }) {
  const { t, i18n } = useTranslation("summoner");
  const [expanded, setExpanded] = useState(false);
  const detailsID = useId();
  const patch = match.version.split(".").slice(0, 2).join(".");
  const assetBase = `/game-assets/${patch}`;
  const minutes = Math.floor(match.durationSeconds / 60);
  const seconds = match.durationSeconds % 60;
  const playedAt = new Intl.DateTimeFormat(i18n.language, {
    month: "short",
    day: "numeric",
    hour: "2-digit",
    minute: "2-digit",
  }).format(new Date(match.gameStartTime));

  return (
    <article
      className={`overflow-hidden rounded-lg border bg-surface-raised shadow-card ${
        match.win
          ? "border-l-4 border-l-sky-500"
          : "border-l-4 border-l-red-500"
      } border-y-border border-r-border`}
    >
      <button
        type="button"
        aria-expanded={expanded}
        aria-controls={detailsID}
        aria-label={
          expanded
            ? t("match.hideParticipants")
            : t("match.showParticipants", { count: match.participants.length })
        }
        onClick={() => setExpanded((value) => !value)}
        className="group relative block w-full text-left transition-colors hover:bg-surface-overlay focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-inset focus-visible:ring-accent"
      >
        <div className="grid gap-4 p-4 pb-11 lg:grid-cols-[9rem_minmax(15rem,1fr)_minmax(16rem,1.25fr)] lg:items-center lg:pb-4 lg:pr-36">
          <div>
            <p className="font-medium text-fg-default">
              {t(`queue.${match.queue}`)}
            </p>
            <p
              className={
                match.win ? "text-sm text-sky-400" : "text-sm text-red-400"
              }
            >
              {match.win ? t("match.victory") : t("match.defeat")}
              {match.earlySurrender
                ? ` · ${t("match.earlySurrender")}`
                : match.surrender
                  ? ` · ${t("match.surrender")}`
                  : ""}
            </p>
            {match.averageTier ? (
              <p className="mt-1 text-xs text-fg-muted">
                <span className="text-fg-subtle">{t("match.averageRank")}</span>{" "}
                <span className="font-medium text-fg-default">
                  ≈{" "}
                  <TierLabel
                    tier={match.averageTier}
                    division={match.averageDivision}
                  />
                </span>{" "}
                ·{" "}
                {t("match.rankCoverage", {
                  count: match.tierCoverage,
                  total: match.participants.length,
                })}
              </p>
            ) : (
              <p className="mt-1 text-xs text-fg-subtle">
                {t("match.averageRankPending")}
              </p>
            )}
            <p className="mt-1 text-xs text-fg-subtle">
              {playedAt} · {minutes}:{String(seconds).padStart(2, "0")}
            </p>
          </div>

          <div className="flex items-center gap-3">
            <LocalAssetImage
              src={`${assetBase}/champions/${match.championId}.png`}
              alt={match.championName}
              fallback={match.championName.slice(0, 1)}
              loading="lazy"
              className="h-16 w-16 rounded-lg object-cover"
            />
            <div>
              <p className="font-medium text-fg-default">
                {match.championName}{" "}
                <span className="text-xs text-fg-subtle">
                  Lv. {match.championLevel}
                </span>
              </p>
              <p className="text-lg font-semibold text-fg-default">
                {match.kills} /{" "}
                <span className="text-red-400">{match.deaths}</span> /{" "}
                {match.assists}
              </p>
              <p className="text-xs text-fg-muted">
                {t("match.kda", { value: match.kda.toFixed(2) })} ·{" "}
                {match.position || t("match.unknownPosition")}
              </p>
            </div>
            <IconColumn
              ids={match.summonerSpellIds.slice(0, 2)}
              path="spells"
              base={assetBase}
            />
            <IconColumn
              ids={[match.perkIds[0], match.secondaryStyleId].filter(
                (id): id is number => Boolean(id),
              )}
              path="perks"
              base={assetBase}
            />
          </div>

          <div className="space-y-3">
            <div className="grid grid-cols-2 gap-x-5 gap-y-1 text-xs text-fg-muted sm:grid-cols-4 lg:grid-cols-2 xl:grid-cols-4">
              <Metric
                label={t("match.cs")}
                value={`${match.minionsKilled} (${match.csPerMinute.toFixed(1)})`}
              />
              <Metric
                label={t("match.gold")}
                value={match.goldEarned.toLocaleString()}
              />
              <Metric
                label={t("match.damage")}
                value={match.damageToChampions.toLocaleString()}
              />
              <Metric
                label={t("match.vision")}
                value={match.visionScore.toLocaleString()}
              />
            </div>
            <div className="flex min-h-9 flex-wrap gap-1">
              {match.itemIds.filter(Boolean).map((id, index) => (
                <LocalAssetImage
                  key={`${id}-${index}`}
                  src={`${assetBase}/items/${id}.png`}
                  alt=""
                  loading="lazy"
                  className="h-8 w-8 rounded object-cover"
                />
              ))}
            </div>
          </div>
        </div>
        <span className="absolute bottom-3 right-4 flex items-center gap-1 text-xs font-medium text-accent">
          {expanded
            ? t("match.hideParticipants")
            : t("match.showParticipants", { count: match.participants.length })}
          <Chevron expanded={expanded} />
        </span>
      </button>
      {expanded && (
        <MatchParticipants
          id={detailsID}
          participants={match.participants}
          assetBase={assetBase}
        />
      )}
    </article>
  );
}

function MatchParticipants({
  id,
  participants,
  assetBase,
}: {
  id: string;
  participants: Participant[];
  assetBase: string;
}) {
  const { t } = useTranslation("summoner");
  const teamIDs = [
    ...new Set(participants.map((participant) => participant.teamId)),
  ].sort((left, right) => left - right);

  return (
    <div
      id={id}
      role="region"
      aria-label={t("match.participantDetails", { count: participants.length })}
      className="border-t border-border bg-surface-overlay/40 p-3 sm:p-4"
    >
      {participants.length === 0 ? (
        <p className="py-3 text-center text-sm text-fg-subtle">
          {t("match.participantsUnavailable")}
        </p>
      ) : (
        <div className="space-y-4">
          {teamIDs.map((teamID) => {
            const team = participants.filter(
              (participant) => participant.teamId === teamID,
            );
            return (
              <ParticipantTeam
                key={teamID}
                teamID={teamID}
                participants={team}
                assetBase={assetBase}
              />
            );
          })}
        </div>
      )}
    </div>
  );
}

function ParticipantTeam({
  teamID,
  participants,
  assetBase,
}: {
  teamID: number;
  participants: Participant[];
  assetBase: string;
}) {
  const { t } = useTranslation("summoner");
  const won = participants[0]?.win ?? false;
  const teamName =
    teamID === 100
      ? t("match.blueTeam")
      : teamID === 200
        ? t("match.redTeam")
        : t("match.team", { id: teamID });

  return (
    <section className="overflow-hidden rounded-lg border border-border bg-surface-raised">
      <h3
        className={`border-b border-border px-3 py-2 text-sm font-semibold ${won ? "text-sky-400" : "text-red-400"}`}
      >
        {teamName} · {won ? t("match.victory") : t("match.defeat")}
      </h3>
      <div className="overflow-x-auto">
        <table className="w-full min-w-[55rem] text-xs">
          <thead className="bg-surface-overlay text-left text-fg-subtle">
            <tr>
              <th scope="col" className="px-3 py-2 font-medium">
                {t("match.player")}
              </th>
              <th scope="col" className="px-2 py-2 text-center font-medium">
                KDA
              </th>
              <th scope="col" className="px-2 py-2 text-center font-medium">
                {t("match.rank")}
              </th>
              <th scope="col" className="px-2 py-2 text-right font-medium">
                {t("match.cs")}
              </th>
              <th scope="col" className="px-2 py-2 text-right font-medium">
                {t("match.gold")}
              </th>
              <th scope="col" className="px-2 py-2 text-right font-medium">
                {t("match.damage")}
              </th>
              <th scope="col" className="px-2 py-2 text-right font-medium">
                {t("match.vision")}
              </th>
              <th scope="col" className="px-3 py-2 font-medium">
                {t("match.items")}
              </th>
            </tr>
          </thead>
          <tbody className="divide-y divide-border">
            {participants.map((participant) => (
              <ParticipantRow
                key={participant.participantId}
                participant={participant}
                assetBase={assetBase}
              />
            ))}
          </tbody>
        </table>
      </div>
    </section>
  );
}

function ParticipantRow({
  participant,
  assetBase,
}: {
  participant: Participant;
  assetBase: string;
}) {
  const { t } = useTranslation("summoner");
  const riotID = participant.gameName
    ? `${participant.gameName}${participant.tagLine ? `#${participant.tagLine}` : ""}`
    : t("match.playerNumber", { id: participant.participantId });
  const perkIDs = [participant.perkIds[0], participant.secondaryStyleId].filter(
    (id): id is number => Boolean(id),
  );

  return (
    <tr
      className={participant.isCurrentPlayer ? "bg-accent-subtle" : undefined}
    >
      <th scope="row" className="px-3 py-2 text-left font-normal">
        <div className="flex min-w-52 items-center gap-2">
          <LocalAssetImage
            src={`${assetBase}/champions/${participant.championId}.png`}
            alt={participant.championName}
            fallback={participant.championName.slice(0, 1)}
            loading="lazy"
            className="h-10 w-10 rounded object-cover"
          />
          <IconColumn
            ids={participant.summonerSpellIds.slice(0, 2)}
            path="spells"
            base={assetBase}
            size="small"
          />
          <IconColumn
            ids={perkIDs}
            path="perks"
            base={assetBase}
            size="small"
          />
          <div className="min-w-0">
            <p className="truncate font-medium text-fg-default">
              {riotID}
              {participant.isCurrentPlayer && (
                <span className="ml-1 rounded bg-accent px-1 py-0.5 text-[10px] text-fg-inverse">
                  {t("match.you")}
                </span>
              )}
            </p>
            <p className="truncate text-fg-subtle">
              {participant.championName} · Lv. {participant.championLevel} ·{" "}
              {participant.position || t("match.unknownPosition")}
            </p>
          </div>
        </div>
      </th>
      <td className="whitespace-nowrap px-2 py-2 text-center text-fg-default">
        {participant.kills} /{" "}
        <span className="text-red-400">{participant.deaths}</span> /{" "}
        {participant.assists}
        <span className="block text-[10px] text-fg-subtle">
          {participant.kda.toFixed(2)}
        </span>
      </td>
      <RankCell rank={participant.rank} />
      <NumberCell value={participant.minionsKilled} />
      <NumberCell value={participant.goldEarned} />
      <NumberCell value={participant.damageToChampions} />
      <NumberCell value={participant.visionScore} />
      <td className="px-3 py-2">
        <div className="flex min-w-44 gap-1">
          {participant.itemIds.filter(Boolean).map((id, index) => (
            <LocalAssetImage
              key={`${id}-${index}`}
              src={`${assetBase}/items/${id}.png`}
              alt=""
              loading="lazy"
              className="h-6 w-6 rounded object-cover"
            />
          ))}
        </div>
      </td>
    </tr>
  );
}

function RankCell({ rank }: { rank: Participant["rank"] }) {
  const { t } = useTranslation("summoner");
  if (!rank) {
    return (
      <td className="whitespace-nowrap px-2 py-2 text-center text-fg-subtle">
        {t("rank.pending")}
      </td>
    );
  }
  if (rank.tier === "UNRANKED") {
    return (
      <td className="whitespace-nowrap px-2 py-2 text-center text-fg-muted">
        {t("rank.unranked")}
      </td>
    );
  }
  const snapshotTitle =
    rank.snapshotDeltaHours == null
      ? undefined
      : t("rank.snapshotDelta", { hours: rank.snapshotDeltaHours });
  return (
    <td
      className="whitespace-nowrap px-2 py-2 text-center text-fg-default"
      title={snapshotTitle}
    >
      <span>
        ≈ <TierLabel tier={rank.tier} division={rank.division} />
      </span>
      {rank.leaguePoints != null && (
        <span className="block text-[10px] text-fg-subtle">
          {rank.leaguePoints} LP
        </span>
      )}
    </td>
  );
}

function TierLabel({
  tier,
  division,
}: {
  tier: string;
  division?: string | null;
}) {
  const { t } = useTranslation("summoner");
  const key = RANK_TIER_KEYS[tier as keyof typeof RANK_TIER_KEYS];
  return (
    <>
      {key ? t(key) : tier}
      {division ? ` ${division}` : ""}
    </>
  );
}

function NumberCell({ value }: { value: number }) {
  return (
    <td className="whitespace-nowrap px-2 py-2 text-right text-fg-muted">
      {value.toLocaleString()}
    </td>
  );
}

function Chevron({ expanded }: { expanded: boolean }) {
  return (
    <svg
      aria-hidden="true"
      viewBox="0 0 20 20"
      fill="none"
      className={`h-4 w-4 transition-transform ${expanded ? "rotate-180" : ""}`}
    >
      <path
        d="m5 7.5 5 5 5-5"
        stroke="currentColor"
        strokeWidth="1.75"
        strokeLinecap="round"
        strokeLinejoin="round"
      />
    </svg>
  );
}

function IconColumn({
  ids,
  path,
  base,
  size = "normal",
}: {
  ids: number[];
  path: string;
  base: string;
  size?: "normal" | "small";
}) {
  return (
    <div className="grid shrink-0 grid-cols-1 gap-1">
      {ids.map((id, index) => (
        <LocalAssetImage
          key={`${id}-${index}`}
          src={`${base}/${path}/${id}.png`}
          alt=""
          loading="lazy"
          className={`${size === "small" ? "h-4 w-4" : "h-7 w-7"} rounded object-cover`}
        />
      ))}
    </div>
  );
}

function Metric({ label, value }: { label: string; value: string }) {
  return (
    <p>
      <span className="text-fg-subtle">{label}</span> {value}
    </p>
  );
}
