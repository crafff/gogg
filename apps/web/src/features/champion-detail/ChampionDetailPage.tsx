import { useTranslation } from "react-i18next";
import { Link, useParams, useSearchParams } from "react-router-dom";

import {
  type ChampionWinFactorsQuery,
  useChampionDetailQuery,
  useChampionWinFactorsQuery,
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
  const factorsView = params.get("view") === "factors";
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
  const versions = useVersionsQuery();
  const regions = useRegionsQuery();
  const resolvedFilterVersion =
    selected.version === "latest"
      ? (versions.data?.versions[0] ?? "")
      : selected.version;
  const versionReady = resolvedFilterVersion !== "";
  const query = useChampionDetailQuery(
    {
      id,
      filter: {
        queueId: 420,
        position: selected.position,
        region: selected.region,
        version: resolvedFilterVersion || "latest",
        tierGroup: mapToTierGroup(selected.tier),
      },
    },
    {
      enabled: Number.isInteger(id) && id > 0 && !factorsView && versionReady,
    },
  );
  const factorsQuery = useChampionWinFactorsQuery(
    {
      id,
      filter: {
        queueId: 420,
        position: selected.position,
        region: selected.region,
        version: resolvedFilterVersion || "latest",
        tierGroup: mapToTierGroup(selected.tier),
      },
    },
    {
      enabled:
        Number.isInteger(id) &&
        id > 0 &&
        factorsView &&
        versionReady &&
        selected.position !== "",
    },
  );
  const detail = query.data?.championDetail;
  const factors = factorsQuery.data?.championWinFactors;
  const assets = useGameAssets(
    detail?.resolvedVersion ??
      factors?.resolvedVersion ??
      (resolvedFilterVersion || null),
  );
  const locale = i18n.language.toLowerCase().replace("-", "_");
  const champion = assets.manifest?.champions[String(id)];
  const name =
    champion?.names[locale] ??
    detail?.championName ??
    factors?.championName ??
    `#${championId}`;
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
          {factors && (
            <p className="text-sm text-fg-subtle">
              {t("factorSummary", {
                games: factors.sampleGames,
                players: factors.samplePlayers,
                version: factors.resolvedVersion,
              })}
            </p>
          )}
        </div>
      </header>
      <RankingsFilters {...filterProps} />
      <nav
        aria-label={t("viewsLabel")}
        className="flex gap-2 border-b border-border"
      >
        <ViewLink params={params} view="overview" active={!factorsView}>
          {t("overviewView")}
        </ViewLink>
        <ViewLink params={params} view="factors" active={factorsView}>
          {t("factorsView")}
        </ViewLink>
      </nav>
      {!factorsView && query.isLoading && (
        <div className="grid gap-4 md:grid-cols-2">
          {Array.from({ length: 6 }, (_, i) => (
            <Skeleton key={i} className="h-40 w-full" />
          ))}
        </div>
      )}
      {!factorsView && query.isError && (
        <State text={t("common:state.error")} />
      )}
      {!factorsView && !query.isLoading && !query.isError && !detail && (
        <State text={t("notFound")} />
      )}
      {!factorsView && detail && (
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
      {factorsView && selected.position === "" && (
        <State text={t("choosePosition")} />
      )}
      {factorsView && selected.position !== "" && factorsQuery.isLoading && (
        <div className="grid gap-4 lg:grid-cols-2">
          {Array.from({ length: 4 }, (_, index) => (
            <Skeleton key={index} className="h-80 w-full" />
          ))}
        </div>
      )}
      {factorsView && factorsQuery.isError && (
        <State text={t("common:state.error")} />
      )}
      {factorsView &&
        selected.position !== "" &&
        !factorsQuery.isLoading &&
        !factorsQuery.isError &&
        !factors && <State text={t("notFound")} />}
      {factorsView && factors && factors.availability !== "AVAILABLE" && (
        <State text={t(unavailableTranslationKey(factors.unavailableReason))} />
      )}
      {factorsView && factors?.availability === "AVAILABLE" && (
        <WinFactors result={factors} />
      )}
    </section>
  );
}

function ViewLink({
  params,
  view,
  active,
  children,
}: {
  params: URLSearchParams;
  view: "overview" | "factors";
  active: boolean;
  children: React.ReactNode;
}) {
  const next = new URLSearchParams(params);
  if (view === "factors") next.set("view", "factors");
  else next.delete("view");
  return (
    <Link
      to={{ search: next.size > 0 ? `?${next.toString()}` : "" }}
      aria-current={active ? "page" : undefined}
      className={cn(
        "border-b-2 px-3 py-2 text-sm font-medium",
        active
          ? "border-accent text-fg-default"
          : "border-transparent text-fg-muted hover:text-fg-default",
      )}
    >
      {children}
    </Link>
  );
}

type WinFactorsResult = NonNullable<
  ChampionWinFactorsQuery["championWinFactors"]
>;
export type WinFactor = WinFactorsResult["factors"][number];

export function WinFactors({ result }: { result: WinFactorsResult }) {
  const { t } = useTranslation("championDetail");
  return (
    <div className="space-y-4">
      <div className="rounded-lg border border-border bg-surface-raised p-4 text-sm text-fg-muted">
        <p>{t("observationalNotice")}</p>
        <p className="mt-2 text-xs text-fg-subtle">
          {t("factorCoverage", {
            games: result.sampleGames,
            players: result.samplePlayers,
            region: result.regionScope,
            tier: result.tierGroup,
            version: result.resolvedVersion,
          })}
        </p>
        {result.dataThrough && (
          <p className="mt-1 text-xs text-fg-subtle">
            {t("dataThrough", {
              date: new Intl.DateTimeFormat(undefined, {
                dateStyle: "medium",
              }).format(new Date(result.dataThrough)),
            })}
          </p>
        )}
      </div>
      <div className="grid gap-4 xl:grid-cols-2">
        {result.factors.map((factor) => (
          <WinFactorCard key={factor.metricKey} factor={factor} />
        ))}
      </div>
    </div>
  );
}

export function WinFactorCard({ factor }: { factor: WinFactor }) {
  const { t } = useTranslation("championDetail");
  return (
    <article className="overflow-hidden rounded-lg border border-border bg-surface-raised">
      <header className="space-y-2 border-b border-border p-4">
        <div className="flex items-start justify-between gap-3">
          <div>
            <p className="text-xs uppercase tracking-wide text-fg-subtle">
              {t("observedEvidence")}
            </p>
            <h2 className="font-semibold text-fg-default">
              {t(metricTranslationKey(factor.metricKey))}
            </h2>
          </div>
          <span className="rounded-full border border-border px-2 py-1 text-xs text-fg-muted">
            {factor.startMinute}–{factor.endMinute}m
          </span>
        </div>
        <dl className="grid grid-cols-3 gap-2 text-center">
          {(["p50", "p70", "p90"] as const).map((key) => (
            <div key={key} className="rounded bg-surface-overlay p-2">
              <dt className="text-xs uppercase text-fg-subtle">{key}</dt>
              <dd className="font-medium text-fg-default">
                {formatMetricValue(factor[key], factor.unit)}
              </dd>
            </div>
          ))}
        </dl>
      </header>
      <div className="overflow-x-auto">
        <table className="w-full text-left text-xs">
          <caption className="sr-only">
            {t("bucketCaption", {
              metric: t(metricTranslationKey(factor.metricKey)),
            })}
          </caption>
          <thead className="text-fg-subtle">
            <tr>
              <th scope="col" className="px-3 py-2">
                {t("range")}
              </th>
              <th scope="col" className="px-3 py-2">
                {t("winRate")}
              </th>
              <th scope="col" className="px-3 py-2">
                {t("delta")}
              </th>
              <th scope="col" className="px-3 py-2">
                {t("sampleGames")}
              </th>
              <th scope="col" className="px-3 py-2">
                {t("samplePlayers")}
              </th>
            </tr>
          </thead>
          <tbody>
            {factor.buckets.map((bucket) => (
              <tr key={bucket.ordinal} className="border-t border-border">
                <td className="px-3 py-2 text-fg-default">
                  {formatMetricValue(bucket.lowerBound, factor.unit)}–
                  {formatMetricValue(bucket.upperBound, factor.unit)}
                </td>
                <td className="px-3 py-2 text-fg-muted">
                  {bucket.observedWinRate.toFixed(1)}%
                </td>
                <td className="px-3 py-2 font-medium text-fg-default">
                  {bucket.observedWinRateDelta >= 0 ? "+" : ""}
                  {bucket.observedWinRateDelta.toFixed(1)} pp
                </td>
                <td className="px-3 py-2 text-fg-muted">{bucket.games}</td>
                <td className="px-3 py-2 text-fg-muted">
                  {bucket.samplePlayers}
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
    </article>
  );
}

function formatMetricValue(value: number, unit: string) {
  if (unit === "DAMAGE") return Math.round(value).toLocaleString();
  return Number.isInteger(value) ? String(value) : value.toFixed(1);
}

const metricTranslationKeys = {
  JUNGLE_CS_10: "metrics.JUNGLE_CS_10",
  JUNGLE_CS_GAIN_10_15: "metrics.JUNGLE_CS_GAIN_10_15",
  DAMAGE_TO_CHAMPIONS_10: "metrics.DAMAGE_TO_CHAMPIONS_10",
  DAMAGE_TO_CHAMPIONS_GAIN_10_15: "metrics.DAMAGE_TO_CHAMPIONS_GAIN_10_15",
} as const;

function metricTranslationKey(metric: string) {
  return (
    metricTranslationKeys[metric as keyof typeof metricTranslationKeys] ??
    "metrics.DAMAGE_TO_CHAMPIONS_10"
  );
}

const unavailableTranslationKeys = {
  UNSUPPORTED_POSITION: "unavailable.UNSUPPORTED_POSITION",
  NOT_PUBLISHED: "unavailable.NOT_PUBLISHED",
  MIN_GAMES: "unavailable.MIN_GAMES",
  MIN_PLAYERS: "unavailable.MIN_PLAYERS",
  SPARSE_BUCKETS: "unavailable.SPARSE_BUCKETS",
} as const;

function unavailableTranslationKey(reason: string | null | undefined) {
  if (reason && reason in unavailableTranslationKeys) {
    return unavailableTranslationKeys[
      reason as keyof typeof unavailableTranslationKeys
    ];
  }
  return "unavailable.UNKNOWN" as const;
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
