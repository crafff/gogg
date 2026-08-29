import { useTranslation } from "react-i18next";
import { useParams, useSearchParams } from "react-router-dom";

import {
  useChampionDetailQuery,
  useRegionsQuery,
  useVersionsQuery,
} from "@shared/api";
import { Skeleton } from "@shared/ui";
import { cn } from "@shared/lib/cn";
import {
  RankingsFilters,
  type RankingsFiltersProps,
} from "@features/rankings/components/RankingsFilters";
import {
  mapToTierGroup,
  type Position,
  type RankingsFiltersState,
  type UiTier,
} from "@features/rankings/hooks/useRankingsFilters";
import {
  useGameAssets,
  type GameAssetEntry,
} from "@features/rankings/hooks/useGameAssets";

const DEFAULTS: RankingsFiltersState = {
  position: "",
  tier: "",
  region: "",
  version: "latest",
};

export function ChampionDetailPage() {
  const { championId = "" } = useParams();
  const id = Number(championId);
  const [params, setParams] = useSearchParams();
  const { t, i18n } = useTranslation(["championDetail", "common"]);
  const selected: RankingsFiltersState = {
    position: (params.get("position") ?? "") as Position,
    tier: (params.get("tier") ?? "") as UiTier,
    region: params.get("region") ?? "",
    version: params.get("version") ?? "latest",
  };
  const change = <K extends keyof RankingsFiltersState>(
    key: K,
    value: RankingsFiltersState[K],
  ) => {
    const next = new URLSearchParams(params);
    if (value && value !== DEFAULTS[key]) next.set(key, value);
    else next.delete(key);
    setParams(next);
  };
  const query = useChampionDetailQuery(
    {
      id,
      filter: {
        queueId: 420,
        position: selected.position,
        region: selected.region,
        version: selected.version,
        tierGroup: mapToTierGroup(selected.tier),
      },
    },
    { enabled: Number.isInteger(id) && id > 0 },
  );
  const versions = useVersionsQuery();
  const regions = useRegionsQuery();
  const detail = query.data?.championDetail;
  const assets = useGameAssets(
    detail?.resolvedVersion ??
      (selected.version === "latest" ? null : selected.version),
  );
  const locale = i18n.language.toLowerCase().replace("-", "_");
  const champion = assets.manifest?.champions[String(id)];
  const name =
    champion?.names[locale] ?? detail?.championName ?? `#${championId}`;
  const filterProps: RankingsFiltersProps = {
    selected,
    availableVersions: versions.data?.versions ?? [],
    availableRegions: regions.data?.regions ?? [],
    onPositionChange: (v) => change("position", v),
    onTierChange: (v) => change("tier", v),
    onRegionChange: (v) => change("region", v),
    onVersionChange: (v) => change("version", v),
  };

  if (!Number.isInteger(id) || id <= 0) return <State text={t("invalid")} />;
  return (
    <section className="space-y-6">
      <header className="flex items-center gap-4">
        {champion && (
          <img
            src={`${assets.baseURL}/${champion.image}`}
            alt=""
            className="h-20 w-20 rounded-xl object-cover"
          />
        )}
        <div>
          <p className="text-sm text-fg-muted">{t("eyebrow")}</p>
          <h1 className="text-3xl font-semibold text-fg-default">{name}</h1>
          {detail && (
            <p className="text-sm text-fg-subtle">
              {t("summary", {
                games: detail.games,
                version: detail.resolvedVersion ?? selected.version,
              })}
            </p>
          )}
        </div>
      </header>
      <RankingsFilters {...filterProps} />
      {query.isLoading && (
        <div className="grid gap-4 md:grid-cols-2">
          {Array.from({ length: 6 }, (_, i) => (
            <Skeleton key={i} className="h-40 w-full" />
          ))}
        </div>
      )}
      {query.isError && <State text={t("common:state.error")} />}
      {!query.isLoading && !query.isError && !detail && (
        <State text={t("notFound")} />
      )}
      {detail && (
        <div className="grid gap-4 md:grid-cols-2">
          <BuildCard
            title={t("runes")}
            empty={detail.runeBuilds.length === 0}
            className="md:col-span-2"
          >
            {detail.runeBuilds.map((build, index) => (
              <RuneChoice
                key={index}
                build={build}
                perks={assets.manifest?.perks}
                locale={locale}
                base={assets.baseURL}
              />
            ))}
          </BuildCard>
          <BuildCard
            title={t("spells")}
            empty={detail.summonerSpellBuilds.length === 0}
          >
            {detail.summonerSpellBuilds.map((b, i) => (
              <Choice
                key={i}
                entries={b.ids.map(
                  (x) => assets.manifest?.summonerSpells?.[String(x)],
                )}
                ids={b.ids}
                build={b}
                locale={locale}
                base={assets.baseURL}
              />
            ))}
          </BuildCard>
          <BuildCard
            title={t("starter")}
            empty={detail.starterBuilds.length === 0}
          >
            {detail.starterBuilds.map((b, i) => (
              <Choice
                key={i}
                entries={b.ids.map((x) => assets.manifest?.items?.[String(x)])}
                ids={b.ids}
                build={b}
                locale={locale}
                base={assets.baseURL}
              />
            ))}
          </BuildCard>
          <BuildCard title={t("boots")} empty={detail.bootsBuilds.length === 0}>
            {detail.bootsBuilds.map((b, i) => (
              <Choice
                key={i}
                entries={b.ids.map((x) => assets.manifest?.items?.[String(x)])}
                ids={b.ids}
                build={b}
                locale={locale}
                base={assets.baseURL}
              />
            ))}
          </BuildCard>
          {detail.itemBuilds.map((stage) => (
            <BuildCard
              key={stage.stage}
              title={t("items", { count: stage.stage })}
              empty={stage.builds.length === 0}
            >
              {stage.builds.map((b, i) => (
                <Choice
                  key={i}
                  entries={b.ids.map(
                    (x) => assets.manifest?.items?.[String(x)],
                  )}
                  ids={b.ids}
                  build={b}
                  locale={locale}
                  base={assets.baseURL}
                />
              ))}
            </BuildCard>
          ))}
        </div>
      )}
    </section>
  );
}

type ChoiceBuild = {
  games: number;
  eligibleGames: number;
  pickRate: number;
  winRate: number;
};
type RuneBuild = ChoiceBuild & {
  primaryStyleId: number;
  secondaryStyleId: number;
  perkIds: number[];
  statShardIds: number[];
};

export function RuneChoice({
  build,
  perks,
  locale,
  base,
}: {
  build: RuneBuild;
  perks: Record<string, GameAssetEntry> | undefined;
  locale: string;
  base: string;
}) {
  const { t } = useTranslation("championDetail");
  return (
    <ChoiceRow build={build}>
      <div className="flex min-w-0 flex-wrap gap-2">
        <RuneBranch
          label={t("primaryRunes")}
          styleID={build.primaryStyleId}
          ids={build.perkIds.slice(0, 4)}
          entries={perks}
          locale={locale}
          base={base}
          className="basis-[18rem]"
        />
        <RuneBranch
          label={t("secondaryRunes")}
          styleID={build.secondaryStyleId}
          ids={build.perkIds.slice(4)}
          entries={perks}
          locale={locale}
          base={base}
          className="basis-[14rem]"
        />
        <RuneShardGroup
          label={t("statShards")}
          ids={build.statShardIds}
          entries={perks}
          locale={locale}
          base={base}
        />
      </div>
    </ChoiceRow>
  );
}

function RuneBranch({
  label,
  styleID,
  ids,
  entries,
  locale,
  base,
  className,
}: {
  label: string;
  styleID: number;
  ids: number[];
  entries: Record<string, GameAssetEntry> | undefined;
  locale: string;
  base: string;
  className?: string;
}) {
  const style = entries?.[String(styleID)];
  const styleName = style?.names[locale] ?? String(styleID);
  return (
    <div
      role="group"
      aria-label={`${label} ${styleName}`}
      className={cn(
        "flex min-w-0 flex-1 flex-wrap items-center gap-3 rounded-lg border border-border bg-surface-overlay/40 px-3 py-2",
        className,
      )}
    >
      <div className="flex shrink-0 items-center gap-2 border-r border-border-strong pr-3">
        <AssetIcon
          id={styleID}
          entry={style}
          locale={locale}
          base={base}
          className="h-8 w-8"
        />
        <div className="leading-tight">
          <p className="text-[10px] text-fg-subtle">{label}</p>
          <p className="whitespace-nowrap text-xs font-medium text-fg-muted">
            {styleName}
          </p>
        </div>
      </div>
      <div className="flex min-w-0 flex-1 flex-wrap items-center gap-2">
        {ids.map((id, index) => (
          <AssetIcon
            key={`${id}-${index}`}
            id={id}
            entry={entries?.[String(id)]}
            locale={locale}
            base={base}
          />
        ))}
      </div>
    </div>
  );
}

function RuneShardGroup({
  label,
  ids,
  entries,
  locale,
  base,
}: {
  label: string;
  ids: number[];
  entries: Record<string, GameAssetEntry> | undefined;
  locale: string;
  base: string;
}) {
  return (
    <div
      role="group"
      aria-label={label}
      className="flex min-w-0 flex-1 basis-[12rem] flex-wrap items-center gap-3 rounded-lg border border-border bg-surface-overlay/40 px-3 py-2"
    >
      <p className="shrink-0 text-xs font-medium text-fg-muted">{label}</p>
      <div className="flex items-center gap-2">
        {ids.map((id, index) => (
          <AssetIcon
            key={`${id}-${index}`}
            id={id}
            entry={entries?.[String(id)]}
            locale={locale}
            base={base}
          />
        ))}
      </div>
    </div>
  );
}

function Choice({
  entries,
  ids,
  build,
  locale,
  base,
}: {
  entries: (GameAssetEntry | undefined)[];
  ids: number[];
  build: ChoiceBuild;
  locale: string;
  base: string;
}) {
  return (
    <ChoiceRow build={build}>
      <div className="flex min-w-0 flex-wrap items-center gap-2">
        {ids.map((id, index) => (
          <AssetIcon
            key={`${id}-${index}`}
            id={id}
            entry={entries[index]}
            locale={locale}
            base={base}
          />
        ))}
      </div>
    </ChoiceRow>
  );
}

function AssetIcon({
  id,
  entry,
  locale,
  base,
  className,
}: {
  id: number;
  entry: GameAssetEntry | undefined;
  locale: string;
  base: string;
  className?: string;
}) {
  const name = entry?.names[locale] ?? String(id);
  const size = cn("h-9 w-9", className);
  return entry ? (
    <img
      src={`${base}/${entry.image}`}
      title={name}
      alt={name}
      className={cn(size, "rounded object-cover")}
    />
  ) : (
    <span
      className={cn(
        size,
        "grid place-items-center rounded bg-surface-overlay text-[10px] text-fg-muted",
      )}
    >
      {id}
    </span>
  );
}

function ChoiceRow({
  build,
  children,
}: {
  build: ChoiceBuild;
  children: React.ReactNode;
}) {
  const { t } = useTranslation("championDetail");
  return (
    <div className="grid gap-3 border-t border-border py-4 first:border-0 lg:grid-cols-[minmax(0,1fr)_auto] lg:items-center">
      {children}
      <div className="min-w-40 justify-self-start whitespace-nowrap text-xs text-fg-muted lg:justify-self-end lg:text-right">
        <div>
          {t("rates", {
            pick: build.pickRate.toFixed(1),
            win: build.winRate.toFixed(1),
          })}
        </div>
        <div className="mt-1">
          {t("sample", { games: build.games, total: build.eligibleGames })}
        </div>
      </div>
    </div>
  );
}

function BuildCard({
  title,
  empty,
  children,
  className,
}: {
  title: string;
  empty: boolean;
  children: React.ReactNode;
  className?: string;
}) {
  const { t } = useTranslation("championDetail");
  return (
    <article
      className={cn(
        "rounded-lg border border-border bg-surface-raised p-4",
        className,
      )}
    >
      <h2 className="font-semibold text-fg-default">{title}</h2>
      {empty ? (
        <p className="py-8 text-center text-sm text-fg-muted">{t("empty")}</p>
      ) : (
        children
      )}
    </article>
  );
}
function State({ text }: { text: string }) {
  return (
    <p className="rounded-lg border border-border bg-surface-raised py-12 text-center text-fg-muted">
      {text}
    </p>
  );
}
