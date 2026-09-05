import { useTranslation } from "react-i18next";
import type { TFunction } from "i18next";

import type { TftMatchHistoryQuery } from "@shared/api";
import { cn } from "@shared/lib/cn";

import { TftEntityIcon } from "./TftEntityIcon";

type Match = NonNullable<
  TftMatchHistoryQuery["tftMatchHistory"]
>["matches"][number];

export function TftMatchCard({ match }: { match: Match }) {
  const { t, i18n } = useTranslation("tft");
  const participant = match.participant;
  const won = participant.placement === 1;
  const top4 = participant.placement <= 4;

  return (
    <article
      className={cn(
        "overflow-hidden rounded-xl border bg-surface-raised shadow-card",
        won
          ? "border-accent"
          : top4
            ? "border-tier-diamond/50"
            : "border-border",
      )}
    >
      <div className="grid gap-4 p-4 md:grid-cols-[7rem_minmax(0,1fr)_12rem]">
        <header>
          <p
            className={cn(
              "text-3xl font-semibold",
              won
                ? "text-accent"
                : top4
                  ? "text-tier-diamond"
                  : "text-fg-muted",
            )}
          >
            {t("player.match.placement", { value: participant.placement })}
          </p>
          <p className="mt-1 text-xs text-fg-muted">
            {queueLabel(match.queueId, t)}
          </p>
          <p className="text-xs text-fg-subtle">
            {formatDate(match.gameDatetime, i18n.resolvedLanguage)}
          </p>
          <p className="mt-1 text-[11px] text-fg-subtle">
            {t("player.match.patchSet", {
              patch: match.patch,
              set: match.setNumber,
            })}
          </p>
        </header>

        <div className="min-w-0 space-y-3">
          <ul
            className="flex flex-wrap gap-2"
            aria-label={t("player.match.units")}
          >
            {participant.units.map((unit, index) => (
              <li key={`${unit.entity.id}-${index}`} className="relative">
                <TftEntityIcon
                  entityId={unit.entity.id}
                  name={unit.entity.name}
                  iconUrl={unit.entity.iconUrl}
                  size="lg"
                  className={cn(unit.tier >= 3 && "border-accent")}
                />
                <span className="absolute -bottom-1 -right-1 rounded bg-surface-sunken px-1 text-[9px] text-accent">
                  {"★".repeat(Math.max(1, Math.min(unit.tier, 3)))}
                </span>
                {unit.items.length > 0 && (
                  <span className="absolute -left-1 -top-1 flex">
                    {unit.items.slice(0, 3).map((item, itemIndex) => (
                      <TftEntityIcon
                        key={`${item.id}-${itemIndex}`}
                        entityId={item.id}
                        name={item.name}
                        iconUrl={item.iconUrl}
                        size="sm"
                        className="-mr-3 h-5 w-5 rounded-full"
                      />
                    ))}
                  </span>
                )}
              </li>
            ))}
          </ul>

          <div className="flex flex-wrap gap-1.5">
            {participant.traits
              .filter((trait) => trait.style > 0 || trait.tierCurrent > 0)
              .map((trait) => (
                <span
                  key={trait.entity.id}
                  className="inline-flex items-center gap-1 rounded-full border border-border px-2 py-1 text-[11px] text-fg-muted"
                >
                  <TftEntityIcon
                    entityId={trait.entity.id}
                    name={trait.entity.name}
                    iconUrl={trait.entity.iconUrl}
                    size="sm"
                    className="h-4 w-4 border-0"
                  />
                  {trait.entity.name || trait.entity.id} {trait.numUnits}
                </span>
              ))}
          </div>

          {participant.augments.length > 0 && (
            <div
              className="flex flex-wrap gap-2"
              aria-label={t("player.match.augments")}
            >
              {participant.augments.map((augment) => (
                <span
                  key={augment.id}
                  className="inline-flex items-center gap-1.5 text-xs text-fg-muted"
                >
                  <TftEntityIcon
                    entityId={augment.id}
                    name={augment.name}
                    iconUrl={augment.iconUrl}
                    size="sm"
                  />
                  <span className="max-w-32 truncate">
                    {augment.name || augment.id}
                  </span>
                </span>
              ))}
            </div>
          )}
        </div>

        <dl className="grid grid-cols-3 gap-2 text-center md:grid-cols-2">
          <Stat label={t("player.match.level")} value={participant.level} />
          <Stat label={t("player.match.gold")} value={participant.goldLeft} />
          <Stat label={t("player.match.round")} value={participant.lastRound} />
          <Stat
            label={t("player.match.eliminations")}
            value={participant.playersEliminated}
          />
          <Stat
            label={t("player.match.damage")}
            value={participant.totalDamageToPlayers}
          />
          <Stat
            label={t("player.match.duration")}
            value={formatDuration(match.gameLengthSeconds)}
          />
        </dl>
      </div>
      {match.endOfGameResult && (
        <p className="border-t border-border px-4 py-2 text-xs text-fg-subtle">
          {match.endOfGameResult}
        </p>
      )}
    </article>
  );
}

function Stat({ label, value }: { label: string; value: string | number }) {
  return (
    <div className="rounded bg-surface-sunken p-2">
      <dt className="text-[10px] uppercase text-fg-subtle">{label}</dt>
      <dd className="mt-0.5 text-sm font-medium text-fg-default">{value}</dd>
    </div>
  );
}

function formatDate(value: string, locale?: string) {
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return value;
  return new Intl.DateTimeFormat(locale, {
    dateStyle: "medium",
    timeStyle: "short",
  }).format(date);
}

function formatDuration(seconds: number) {
  const minutes = Math.max(0, Math.floor(seconds / 60));
  const remainder = Math.max(0, Math.floor(seconds % 60));
  return `${minutes}:${String(remainder).padStart(2, "0")}`;
}

function queueLabel(queueId: number, t: TFunction<"tft">) {
  switch (queueId) {
    case 1100:
      return t("player.queue.1100");
    case 1090:
      return t("player.queue.1090");
    case 1160:
      return t("player.queue.1160");
    case 1130:
      return t("player.queue.1130");
    default:
      return String(queueId);
  }
}
