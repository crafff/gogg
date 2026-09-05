import { useEffect, useMemo, useState } from "react";
import { useTranslation } from "react-i18next";
import { Link, useSearchParams } from "react-router-dom";

import {
  type TftAnalysisCatalogQuery,
  type TftCohort,
  type TftLineupsQuery,
  type TftObservedLineupsQuery,
  type TftWindow,
  useTftAnalysisCatalogQuery,
  useTftLineupsQuery,
  useTftObservedLineupsQuery,
} from "@shared/api";
import {
  Button,
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
  Skeleton,
} from "@shared/ui";

import { TftEntityIcon } from "./components/TftEntityIcon";
import {
  catalogOptions,
  readMinSamples,
  readPreviewMinSamples,
  selectCatalogEntry,
  type TftCatalogSelection,
} from "./lib/analysisFilters";
import { normalizeTftPlatform } from "./lib/platforms";

type CatalogEntry = TftAnalysisCatalogQuery["tftAnalysisCatalog"][number];
type FormalLineup = TftLineupsQuery["tftLineups"]["items"][number];
type ObservedResult = NonNullable<
  TftObservedLineupsQuery["tftObservedLineups"]
>;
type Lineup = FormalLineup | ObservedResult["items"][number];
type LineupEntity = Lineup["coreUnits"][number];
type FacetKind = "core" | "emblem" | "artifact" | "augment";

export function TftAnalysisPage() {
  const { t, i18n } = useTranslation("tft");
  const [searchParams, setSearchParams] = useSearchParams();
  const catalogQuery = useTftAnalysisCatalogQuery();
  const catalog = useMemo(
    () => catalogQuery.data?.tftAnalysisCatalog ?? [],
    [catalogQuery.data?.tftAnalysisCatalog],
  );
  const requested = useMemo(
    () => ({
      platform: searchParams.get("platform")?.toUpperCase(),
      patch: searchParams.get("patch") ?? undefined,
      setNumber: parseOptionalInt(searchParams.get("set")),
      cohort: searchParams.get("cohort")?.toUpperCase(),
      window: searchParams.get("window")?.toUpperCase(),
    }),
    [searchParams],
  );
  const selected = useMemo(
    () => selectCatalogEntry(catalog, requested) as CatalogEntry | null,
    [catalog, requested],
  );
  const locale = i18n.resolvedLanguage === "en-US" ? "en_us" : "zh_cn";
  const minSamples = selected
    ? readMinSamples(searchParams.get("minSamples"))
    : readPreviewMinSamples(searchParams.get("minSamples"));
  const previewPlatform = normalizePreviewPlatform(
    searchParams.get("platform"),
  );

  useEffect(() => {
    if (!selected) return;
    const next = analysisSearchParams(selected, minSamples, searchParams);
    if (next.toString() !== searchParams.toString()) {
      setSearchParams(next, { replace: true });
    }
  }, [minSamples, searchParams, selected, setSearchParams]);

  const lineupsQuery = useTftLineupsQuery(
    {
      filter: {
        platform: selected?.platform ?? "KR",
        patch: selected?.patch ?? "latest",
        setNumber: selected?.setNumber ?? 0,
        cohort: (selected?.cohort ?? "MASTER_PLUS") as TftCohort,
        window: (selected?.window ?? "PATCH") as TftWindow,
        locale,
        minSamples,
        limit: 50,
      },
    },
    { enabled: Boolean(selected) },
  );
  const observedQuery = useTftObservedLineupsQuery(
    {
      filter: {
        platform: previewPlatform,
        locale,
        minSamples,
        limit: 30,
      },
    },
    {
      enabled: !catalogQuery.isPending && !catalogQuery.isError && !selected,
      staleTime: 5 * 60 * 1000,
    },
  );

  function changeSelection(change: Partial<TftCatalogSelection>) {
    if (!selected) return;
    const next = selectCatalogEntry(catalog, { ...selected, ...change });
    if (next)
      setSearchParams(analysisSearchParams(next, minSamples, searchParams));
  }

  function changeMinSamples(value: string) {
    if (!selected) return;
    setSearchParams(
      analysisSearchParams(selected, readMinSamples(value), searchParams),
    );
  }

  if (catalogQuery.isPending) return <AnalysisSkeleton />;

  if (catalogQuery.isError) {
    return (
      <PageState
        title={t("state.catalogError")}
        description={t("state.catalogErrorDescription")}
        action={
          <Button onClick={() => void catalogQuery.refetch()}>
            {t("action.retry")}
          </Button>
        }
      />
    );
  }

  if (!selected) {
    return (
      <ObservedPreview
        query={observedQuery}
        platform={previewPlatform}
        minSamples={minSamples}
        onPlatformChange={(platform) =>
          setSearchParams(
            previewSearchParams(platform, minSamples, searchParams),
          )
        }
        onMinSamplesChange={(value) =>
          setSearchParams(
            previewSearchParams(
              previewPlatform,
              readPreviewMinSamples(value),
              searchParams,
            ),
          )
        }
      />
    );
  }

  const options = catalogOptions(catalog, selected);
  const result = lineupsQuery.data?.tftLineups;

  return (
    <section className="space-y-6">
      <header className="flex flex-col justify-between gap-4 md:flex-row md:items-end">
        <div>
          <p className="text-sm font-medium uppercase tracking-[0.2em] text-accent">
            TFT
          </p>
          <h1 className="mt-1 text-3xl font-semibold text-fg-default">
            {t("analysis.title")}
          </h1>
          <p className="mt-2 max-w-2xl text-sm text-fg-muted">
            {t("analysis.description")}
          </p>
        </div>
        <LinkButton to="/tft/player">{t("action.searchPlayer")}</LinkButton>
      </header>

      <AnalysisFilters
        selected={selected}
        options={options}
        minSamples={minSamples}
        onChange={changeSelection}
        onMinSamplesChange={changeMinSamples}
      />

      {lineupsQuery.isPending && <AnalysisSkeleton compact />}
      {lineupsQuery.isError && (
        <PageState
          title={t("state.lineupsError")}
          description={t("state.lineupsErrorDescription")}
          action={
            <Button onClick={() => void lineupsQuery.refetch()}>
              {t("action.retry")}
            </Button>
          }
        />
      )}

      {result && <CoverageSummary result={result} />}

      {result && result.items.length > 0 && (
        <aside className="rounded-xl border border-border bg-surface-raised px-4 py-3 text-xs text-fg-muted">
          {t("analysis.formalDetailLimit")}
        </aside>
      )}

      {result && result.items.length === 0 && (
        <PageState
          title={t("state.noLineups")}
          description={t("state.noLineupsDescription", { minSamples })}
          action={
            minSamples > 1 ? (
              <Button variant="secondary" onClick={() => changeMinSamples("1")}>
                {t("action.showAllSamples")}
              </Button>
            ) : undefined
          }
        />
      )}

      {result && result.items.length > 0 && (
        <LineupList
          lineups={result.items}
          label={t("analysis.lineupListLabel")}
        />
      )}
    </section>
  );
}

function ObservedPreview({
  query,
  platform,
  minSamples,
  onPlatformChange,
  onMinSamplesChange,
}: {
  query: {
    data?: TftObservedLineupsQuery;
    isPending: boolean;
    isError: boolean;
    refetch: () => Promise<unknown>;
  };
  platform: string;
  minSamples: number;
  onPlatformChange: (value: string) => void;
  onMinSamplesChange: (value: string) => void;
}) {
  const { t } = useTranslation("tft");
  const result = query.data?.tftObservedLineups;

  if (query.isPending) {
    return (
      <section className="space-y-6">
        <ObservedHeader />
        <AnalysisSkeleton compact />
      </section>
    );
  }
  if (query.isError) {
    return (
      <section className="space-y-6">
        <ObservedHeader />
        <PageState
          title={t("preview.error")}
          description={t("preview.errorDescription")}
          action={
            <Button onClick={() => void query.refetch()}>
              {t("action.retry")}
            </Button>
          }
        />
      </section>
    );
  }
  if (!result) {
    return (
      <PageState
        title={t("state.notPublished")}
        description={t("state.notPublishedDescription")}
        action={
          <LinkButton to="/tft/player">{t("action.searchPlayer")}</LinkButton>
        }
      />
    );
  }

  const platforms = ["GLOBAL", ...result.platforms];
  return (
    <section className="space-y-6">
      <ObservedHeader />

      <aside className="rounded-xl border border-amber-400/40 bg-amber-400/10 p-4">
        <div className="flex flex-wrap items-center gap-2">
          <span className="rounded-full bg-amber-400/20 px-2.5 py-1 text-xs font-semibold text-amber-200">
            {t("preview.badge")}
          </span>
          <span className="text-xs text-fg-subtle">
            {t("preview.run", { runId: result.runId })}
          </span>
        </div>
        <h2 className="mt-3 text-base font-semibold text-fg-default">
          {t("preview.title")}
        </h2>
        <p className="mt-1 text-sm text-fg-muted">{t("preview.description")}</p>
        <p className="mt-2 text-xs text-fg-subtle">
          {t("preview.provenance", {
            version: result.rawGameVersions.join(", "),
            catalogSource: result.catalogSnapshot.source,
            catalogPatch: result.catalogSnapshot.patch,
            catalogRevision: result.catalogSnapshot.revision.slice(0, 8),
            assetSource: result.assetSnapshot?.source ?? t("preview.noAssets"),
            assetPatch: result.assetSnapshot?.patch ?? t("preview.noAssets"),
            assetRevision:
              result.assetSnapshot?.revision.slice(0, 8) ??
              t("preview.noAssets"),
          })}
        </p>
        <p className="mt-2 text-xs text-fg-subtle">
          {t("lineup.starStrengthDescription")}
        </p>
      </aside>

      <div className="grid gap-3 rounded-xl border border-border bg-surface-raised p-4 sm:grid-cols-2">
        <FilterSelect
          label={t("filter.platform")}
          value={platform}
          options={platforms.map((value) => ({
            value,
            label: value === "GLOBAL" ? t("preview.global") : value,
          }))}
          onChange={onPlatformChange}
        />
        <label className="space-y-1 text-xs text-fg-muted">
          <span>{t("filter.minSamples")}</span>
          <input
            type="number"
            min={20}
            max={100000}
            value={minSamples}
            onChange={(event) => onMinSamplesChange(event.target.value)}
            className="h-9 w-full rounded border border-border-default bg-surface-sunken px-3 text-sm text-fg-default focus-visible:outline-none focus-visible:shadow-focus-ring"
          />
        </label>
      </div>

      <ObservedCoverageSummary result={result} />

      {result.items.length === 0 && (
        <PageState
          title={t("state.noLineups")}
          description={t("preview.noLineupsDescription", { minSamples })}
          action={
            minSamples > 20 ? (
              <Button
                variant="secondary"
                onClick={() => onMinSamplesChange("20")}
              >
                {t("action.showAllSamples")}
              </Button>
            ) : undefined
          }
        />
      )}

      {result.items.length > 0 && (
        <LineupList
          lineups={result.items}
          label={t("preview.lineupListLabel")}
        />
      )}
    </section>
  );
}

function ObservedHeader() {
  const { t } = useTranslation("tft");
  return (
    <header className="flex flex-col justify-between gap-4 md:flex-row md:items-end">
      <div>
        <p className="text-sm font-medium uppercase tracking-[0.2em] text-accent">
          TFT
        </p>
        <h1 className="mt-1 text-3xl font-semibold text-fg-default">
          {t("preview.pageTitle")}
        </h1>
        <p className="mt-2 max-w-2xl text-sm text-fg-muted">
          {t("preview.pageDescription")}
        </p>
      </div>
      <LinkButton to="/tft/player">{t("action.searchPlayer")}</LinkButton>
    </header>
  );
}

function AnalysisFilters({
  selected,
  options,
  minSamples,
  onChange,
  onMinSamplesChange,
}: {
  selected: TftCatalogSelection;
  options: ReturnType<typeof catalogOptions>;
  minSamples: number;
  onChange: (change: Partial<TftCatalogSelection>) => void;
  onMinSamplesChange: (value: string) => void;
}) {
  const { t } = useTranslation("tft");
  return (
    <div className="grid gap-3 rounded-xl border border-border bg-surface-raised p-4 sm:grid-cols-2 lg:grid-cols-6">
      <FilterSelect
        label={t("filter.platform")}
        value={selected.platform}
        options={options.platforms.map((value) => ({ value, label: value }))}
        onChange={(platform) => onChange({ platform })}
      />
      <FilterSelect
        label={t("filter.patch")}
        value={selected.patch}
        options={options.patches.map((value) => ({ value, label: value }))}
        onChange={(patch) => onChange({ patch })}
      />
      <FilterSelect
        label={t("filter.set")}
        value={String(selected.setNumber)}
        options={options.sets.map((value) => ({
          value: String(value),
          label: t("filter.setValue", { value }),
        }))}
        onChange={(setNumber) => onChange({ setNumber: Number(setNumber) })}
      />
      <FilterSelect
        label={t("filter.cohort")}
        value={selected.cohort}
        options={options.cohorts.map((value) => ({
          value,
          label:
            value === "DIAMOND" ? t("cohort.DIAMOND") : t("cohort.MASTER_PLUS"),
        }))}
        onChange={(cohort) => onChange({ cohort })}
      />
      <FilterSelect
        label={t("filter.window")}
        value={selected.window}
        options={options.windows.map((value) => ({
          value,
          label:
            value === "THREE_DAYS" ? t("window.THREE_DAYS") : t("window.PATCH"),
        }))}
        onChange={(window) => onChange({ window })}
      />
      <label className="space-y-1 text-xs text-fg-muted">
        <span>{t("filter.minSamples")}</span>
        <input
          type="number"
          min={1}
          max={100000}
          value={minSamples}
          onChange={(event) => onMinSamplesChange(event.target.value)}
          className="h-9 w-full rounded border border-border-default bg-surface-sunken px-3 text-sm text-fg-default focus-visible:outline-none focus-visible:shadow-focus-ring"
        />
      </label>
    </div>
  );
}

function FilterSelect({
  label,
  value,
  options,
  onChange,
}: {
  label: string;
  value: string;
  options: { value: string; label: string }[];
  onChange: (value: string) => void;
}) {
  return (
    <div className="space-y-1 text-xs text-fg-muted">
      <span>{label}</span>
      <Select value={value} onValueChange={onChange}>
        <SelectTrigger className="w-full" aria-label={label}>
          <SelectValue />
        </SelectTrigger>
        <SelectContent>
          {options.map((option) => (
            <SelectItem key={option.value} value={option.value}>
              {option.label}
            </SelectItem>
          ))}
        </SelectContent>
      </Select>
    </div>
  );
}

function CoverageSummary({
  result,
}: {
  result: TftLineupsQuery["tftLineups"];
}) {
  const { t, i18n } = useTranslation("tft");
  const count = new Intl.NumberFormat(i18n.resolvedLanguage).format;
  return (
    <aside className="grid gap-3 rounded-xl border border-border-accent bg-accent-subtle p-4 sm:grid-cols-2 lg:grid-cols-5">
      <SummaryMetric
        label={t("coverage.matches")}
        value={count(result.sourceMatches)}
      />
      <SummaryMetric
        label={t("coverage.participants")}
        value={count(result.sourceParticipants)}
      />
      <SummaryMetric
        label={t("coverage.exactLineups")}
        value={count(result.coverage.exactLineups)}
      />
      <SummaryMetric
        label={t("coverage.familyThreshold")}
        value={count(result.coverage.familyThreshold)}
      />
      <SummaryMetric
        label={t("coverage.publishedAt")}
        value={formatDate(result.publishedAt, i18n.resolvedLanguage)}
      />
      <p className="text-xs text-fg-muted sm:col-span-2 lg:col-span-5">
        {t("coverage.window", {
          start: formatDate(result.coverage.windowStart, i18n.resolvedLanguage),
          end: formatDate(result.coverage.windowEnd, i18n.resolvedLanguage),
          version: result.algorithmVersion,
        })}
      </p>
    </aside>
  );
}

function ObservedCoverageSummary({ result }: { result: ObservedResult }) {
  const { t, i18n } = useTranslation("tft");
  const count = new Intl.NumberFormat(i18n.resolvedLanguage).format;
  return (
    <aside className="grid gap-3 rounded-xl border border-border-accent bg-accent-subtle p-4 sm:grid-cols-2 lg:grid-cols-5">
      <SummaryMetric
        label={t("coverage.matches")}
        value={count(result.sourceMatches)}
      />
      <SummaryMetric
        label={t("preview.usableParticipants")}
        value={count(result.usableParticipants)}
      />
      <SummaryMetric
        label={t("coverage.exactLineups")}
        value={count(result.exactLineups)}
      />
      <SummaryMetric
        label={t("filter.set")}
        value={t("filter.setValue", { value: result.setNumber })}
      />
      <SummaryMetric
        label={t("filter.patch")}
        value={result.patch ?? t("preview.unknownPatch")}
      />
      <p className="text-xs text-fg-muted sm:col-span-2 lg:col-span-5">
        {t("preview.window", {
          start: formatDate(result.windowStart, i18n.resolvedLanguage),
          end: formatDate(result.windowEnd, i18n.resolvedLanguage),
          participants: count(result.sourceParticipants),
          version: result.algorithmVersion,
        })}
      </p>
    </aside>
  );
}

function SummaryMetric({ label, value }: { label: string; value: string }) {
  return (
    <div>
      <p className="text-xs text-fg-muted">{label}</p>
      <p className="mt-1 text-lg font-semibold text-fg-default">{value}</p>
    </div>
  );
}

function LineupList({ lineups, label }: { lineups: Lineup[]; label: string }) {
  const { t } = useTranslation("tft");
  const [searchParams, setSearchParams] = useSearchParams();
  const facets = useMemo(() => collectLineupFacets(lineups), [lineups]);
  const active = {
    core: searchParams.get("core"),
    emblem: searchParams.get("emblem"),
    artifact: searchParams.get("artifact"),
    augment: searchParams.get("augment"),
  };
  const expanded = searchParams.get("lineup");

  useEffect(() => {
    const next = new URLSearchParams(searchParams);
    let changed = false;
    for (const kind of ["core", "emblem", "artifact", "augment"] as const) {
      const value = next.get(kind);
      if (value && !facets[kind].some((entity) => entity.id === value)) {
        next.delete(kind);
        changed = true;
      }
    }
    const lineupID = next.get("lineup");
    if (lineupID && !lineups.some((lineup) => lineup.id === lineupID)) {
      next.delete("lineup");
      changed = true;
    }
    if (changed) setSearchParams(next, { replace: true });
  }, [facets, lineups, searchParams, setSearchParams]);

  const filtered = lineups.filter((lineup) =>
    Object.entries(active).every(([kind, value]) => {
      if (!value) return true;
      if (kind === "core") {
        return lineup.unitItems.some(
          (unit) => unit.isCore && unit.unit.id === value,
        );
      }
      return lineup.signals.some(
        (signal) =>
          signal.kind === kind.toUpperCase() && signal.entity.id === value,
      );
    }),
  );

  function updateParam(key: FacetKind | "lineup", value: string | null) {
    const next = new URLSearchParams(searchParams);
    if (value && next.get(key) !== value) next.set(key, value);
    else next.delete(key);
    if (key !== "lineup") next.delete("lineup");
    setSearchParams(next);
  }

  function clearFilters() {
    const next = new URLSearchParams(searchParams);
    for (const key of ["core", "emblem", "artifact", "augment"]) {
      next.delete(key);
    }
    next.delete("lineup");
    setSearchParams(next);
  }

  const hasFilters = Object.values(active).some(Boolean);
  return (
    <div className="space-y-3">
      <LineupFacetBar
        facets={facets}
        active={active}
        onToggle={(kind, id) => updateParam(kind, id)}
        onClear={clearFilters}
      />
      {filtered.length === 0 ? (
        <PageState
          title={t("state.noFilteredLineups")}
          description={t("state.noFilteredLineupsDescription")}
          action={
            hasFilters ? (
              <Button variant="secondary" onClick={clearFilters}>
                {t("action.clearFilters")}
              </Button>
            ) : undefined
          }
        />
      ) : (
        <ol className="space-y-3" aria-label={label}>
          {filtered.map((lineup) => (
            <li key={lineup.id}>
              <LineupCard
                lineup={lineup}
                rank={lineups.findIndex((item) => item.id === lineup.id) + 1}
                expanded={expanded === lineup.id}
                active={active}
                onToggleFacet={(kind, id) => updateParam(kind, id)}
                onToggleDetails={() =>
                  updateParam(
                    "lineup",
                    expanded === lineup.id ? null : lineup.id,
                  )
                }
              />
            </li>
          ))}
        </ol>
      )}
    </div>
  );
}

function LineupFacetBar({
  facets,
  active,
  onToggle,
  onClear,
}: {
  facets: Record<FacetKind, LineupEntity[]>;
  active: Record<FacetKind, string | null>;
  onToggle: (kind: FacetKind, id: string | null) => void;
  onClear: () => void;
}) {
  const { t } = useTranslation("tft");
  const groups = (["core", "emblem", "artifact", "augment"] as const).filter(
    (kind) => facets[kind].length > 0,
  );
  if (groups.length === 0) return null;
  return (
    <aside
      className="space-y-3 rounded-xl border border-border bg-surface-raised p-3"
      aria-label={t("filter.lineupFacets")}
    >
      <div className="flex items-center justify-between gap-3">
        <div>
          <p className="text-xs font-semibold uppercase tracking-wide text-fg-muted">
            {t("filter.quickFilters")}
          </p>
          <p className="mt-0.5 text-[10px] text-fg-subtle">
            {t("filter.quickFiltersScope")}
          </p>
        </div>
        {Object.values(active).some(Boolean) && (
          <Button variant="ghost" size="sm" onClick={onClear}>
            {t("action.clearFilters")}
          </Button>
        )}
      </div>
      {groups.map((kind) => (
        <div
          key={kind}
          className="grid grid-cols-[4.5rem_minmax(0,1fr)] items-start gap-2"
        >
          <span className="pt-1 text-[11px] text-fg-subtle">
            {t(`filter.${kind}`)}
          </span>
          <div
            className={
              kind === "core"
                ? "flex min-w-0 flex-wrap gap-1.5"
                : "flex min-w-0 gap-1.5 overflow-x-auto pb-1"
            }
          >
            {facets[kind].map((entity) => {
              const selected = active[kind] === entity.id;
              return (
                <button
                  key={entity.id}
                  type="button"
                  aria-pressed={selected}
                  aria-label={t("filter.filterByEntity", {
                    category: t(`filter.${kind}`),
                    entity: entity.name || entity.id,
                  })}
                  title={entity.name || entity.id}
                  onClick={() => onToggle(kind, selected ? null : entity.id)}
                  className={`flex shrink-0 items-center rounded-md border p-1 text-[11px] transition focus-visible:outline-none focus-visible:shadow-focus-ring ${
                    selected
                      ? "border-accent bg-accent-subtle text-accent"
                      : kind === "core"
                        ? `${unitCostClass(entity.cost)} text-fg-muted hover:brightness-110`
                        : "border-border bg-surface-sunken text-fg-muted hover:border-border-strong"
                  }`}
                >
                  <TftEntityIcon
                    entityId={entity.id}
                    name={entity.name}
                    iconUrl={entity.iconUrl}
                    size="sm"
                  />
                </button>
              );
            })}
          </div>
        </div>
      ))}
    </aside>
  );
}

function LineupCard({
  lineup,
  rank,
  expanded,
  active,
  onToggleFacet,
  onToggleDetails,
}: {
  lineup: Lineup;
  rank: number;
  expanded: boolean;
  active: Record<FacetKind, string | null>;
  onToggleFacet: (kind: FacetKind, id: string | null) => void;
  onToggleDetails: () => void;
}) {
  const { t, i18n } = useTranslation("tft");
  const percent = (value: number) =>
    new Intl.NumberFormat(i18n.resolvedLanguage, {
      style: "percent",
      maximumFractionDigits: 1,
    }).format(value);
  const decimal = (value: number) =>
    new Intl.NumberFormat(i18n.resolvedLanguage, {
      maximumFractionDigits: 2,
    }).format(value);
  const unitAnalysis = new Map(
    lineup.unitItems.map((item) => [item.unit.id, item]),
  );
  return (
    <article className="overflow-hidden rounded-xl border border-border bg-surface-raised shadow-card">
      <div className="px-3 py-3">
        <div className="flex min-w-0 items-center gap-2">
          <p className="w-10 shrink-0 text-center text-xs font-semibold uppercase tracking-wide text-accent md:w-12">
            #{rank}
          </p>
          <div
            className="flex min-w-0 flex-1 items-start gap-2 overflow-x-auto pb-1"
            aria-label={t("lineup.summaryUnits")}
          >
            {lineup.coreUnits.map((unit) => {
              const analysis = unitAnalysis.get(unit.id);
              const threeStarSignal = lineup.signals.find(
                (signal) =>
                  signal.kind === "THREE_STAR" &&
                  signal.entity.id === unit.id,
              );
              const content = (
                <>
                  <span className="flex h-4 w-[3.25rem] items-center justify-center gap-0.5 text-[8px] font-semibold leading-none">
                    {analysis?.isCore && (
                      <span className="rounded bg-accent-subtle px-1 py-0.5 text-accent">
                        {t("lineup.coreBadge")}
                      </span>
                    )}
                    {threeStarSignal && (
                      <span
                        className="tracking-[-0.08em] text-[#d98cff]"
                        title={t("lineup.signalEvidence", {
                          samples: threeStarSignal.sampleSize,
                          rate: percent(threeStarSignal.rate),
                        })}
                        aria-label={t("lineup.threeStarBadge", {
                          entity: unit.name || unit.id,
                        })}
                      >
                        <span aria-hidden="true">★★★</span>
                      </span>
                    )}
                  </span>
                  <span
                    className={`relative inline-flex rounded-md border-2 ${unitCostClass(unit.cost)} ${
                      analysis?.isCore
                        ? "ring-2 ring-accent ring-offset-1 ring-offset-surface-raised"
                        : ""
                    }`}
                  >
                    <TftEntityIcon
                      entityId={unit.id}
                      name={unit.name}
                      iconUrl={unit.iconUrl}
                      size="lg"
                    />
                  </span>
                  <span className="sr-only">{unit.name || unit.id}</span>
                  <span className="mt-0.5 flex h-4 w-[3.25rem] justify-center gap-0.5">
                    {analysis?.commonItems.slice(0, 3).map(({ entity }) => (
                      <TftEntityIcon
                        key={entity.id}
                        entityId={entity.id}
                        name={entity.name}
                        iconUrl={entity.iconUrl}
                        size="xs"
                      />
                    ))}
                  </span>
                </>
              );
              return (
                <div
                  key={unit.id}
                  className="w-[3.25rem] shrink-0 text-center"
                  title={unit.name || unit.id}
                >
                  {analysis?.isCore ? (
                    <button
                      type="button"
                      aria-pressed={active.core === unit.id}
                      aria-label={t("filter.filterByCore", {
                        entity: unit.name || unit.id,
                      })}
                      onClick={() =>
                        onToggleFacet(
                          "core",
                          active.core === unit.id ? null : unit.id,
                        )
                      }
                      className="block w-full rounded-md focus-visible:outline-none focus-visible:shadow-focus-ring"
                    >
                      {content}
                    </button>
                  ) : (
                    content
                  )}
                </div>
              );
            })}
          </div>
          {lineup.unitItems.length === 0 && lineup.commonItems.length > 0 && (
            <div className="flex shrink-0 gap-1" aria-label={t("lineup.items")}>
              {lineup.commonItems.slice(0, 3).map(({ entity }) => (
                <TftEntityIcon
                  key={entity.id}
                  entityId={entity.id}
                  name={entity.name}
                  iconUrl={entity.iconUrl}
                  size="sm"
                />
              ))}
            </div>
          )}
        </div>
        <div className="mt-2 flex flex-col gap-2 border-t border-border pt-2 sm:flex-row sm:items-center">
          {lineup.signals.length > 0 && (
            <div className="min-w-0 sm:flex-1">
              <SignalBadges
                lineup={lineup}
                active={active}
                onToggle={onToggleFacet}
              />
            </div>
          )}
          <dl className="grid w-full grid-cols-3 divide-x divide-border rounded-lg bg-surface-sunken px-1 py-1.5 text-center sm:ml-auto sm:w-64 sm:shrink-0">
            <CompactMetric
              label={t("metric.avgPlacement")}
              value={decimal(lineup.metrics.avgPlacement)}
            />
            <CompactMetric
              label={t("metric.top4Rate")}
              value={percent(lineup.metrics.top4Rate)}
            />
            <CompactMetric
              label={t("metric.samples")}
              value={String(lineup.metrics.sampleSize)}
            />
          </dl>
          <div className="flex items-center justify-end gap-1">
            <CopyTeamCodeButton code={lineup.teamCode} />
            <Button
              variant="ghost"
              size="sm"
              aria-expanded={expanded}
              aria-controls={`lineup-details-${lineup.id}`}
              onClick={onToggleDetails}
              className="shrink-0"
            >
              {expanded ? t("action.hideDetails") : t("action.showDetails")}
              <span aria-hidden="true">{expanded ? "▲" : "▼"}</span>
            </Button>
          </div>
        </div>
      </div>
      {expanded && (
        <LineupDetails lineup={lineup} id={`lineup-details-${lineup.id}`} />
      )}
    </article>
  );
}

function LineupDetails({ lineup, id }: { lineup: Lineup; id: string }) {
  const { t, i18n } = useTranslation("tft");
  const percent = (value: number) =>
    new Intl.NumberFormat(i18n.resolvedLanguage, {
      style: "percent",
      maximumFractionDigits: 1,
    }).format(value);
  return (
    <div id={id} className="border-t border-border bg-surface-sunken/40 p-4">
      <dl className="grid grid-cols-3 gap-2 text-center sm:grid-cols-6">
        <Metric
          label={t("metric.samples")}
          value={String(lineup.metrics.sampleSize)}
        />
        <Metric
          label={t("metric.pickRate")}
          value={percent(lineup.metrics.pickRate)}
        />
        <Metric
          label={t("metric.firstRate")}
          value={percent(lineup.metrics.firstRate)}
        />
        <Metric
          label={t("metric.top4Rate")}
          value={percent(lineup.metrics.top4Rate)}
        />
        <Metric
          label={t("metric.contestedRate")}
          value={percent(lineup.metrics.contestedRate)}
        />
        <Metric
          label={t("metric.lobbies")}
          value={String(lineup.metrics.lobbyCount)}
        />
      </dl>
      {lineup.signals.length > 0 && (
        <SignalEvidenceList signals={lineup.signals} />
      )}
      {lineup.unitItems.length > 0 && (
        <UnitItemsSection units={lineup.unitItems} />
      )}
      {lineup.starCompositions.length > 0 ? (
        <StarCompositionSection
          compositions={lineup.starCompositions}
          knownSamples={lineup.starCompositionKnownSamples}
          unknownSamples={lineup.starCompositionUnknownSamples}
          coverage={lineup.starCompositionCoverage}
        />
      ) : lineup.starLevels.length > 0 ? (
        <StarStrengthSection levels={lineup.starLevels} />
      ) : null}
      {(lineup.commonItems.length > 0 ||
        lineup.commonAugments.length > 0 ||
        lineup.commonTraits.length > 0) && (
        <div className="mt-4 grid gap-3 sm:grid-cols-3">
          <EntityCounts
            title={t("lineup.items")}
            entities={lineup.commonItems}
          />
          <EntityCounts
            title={t("lineup.augments")}
            entities={lineup.commonAugments}
          />
          <EntityCounts
            title={t("lineup.traits")}
            entities={lineup.commonTraits}
          />
        </div>
      )}
    </div>
  );
}

function SignalBadges({
  lineup,
  active,
  onToggle,
}: {
  lineup: Lineup;
  active: Record<FacetKind, string | null>;
  onToggle: (kind: FacetKind, id: string | null) => void;
}) {
  const { t, i18n } = useTranslation("tft");
  if (lineup.signals.length === 0) return null;
  const percent = (value: number) =>
    new Intl.NumberFormat(i18n.resolvedLanguage, {
      style: "percent",
      maximumFractionDigits: 0,
    }).format(value);
  return (
    <div className="flex flex-wrap gap-1">
      {summarySignals(lineup.signals).map((signal) => {
        const facet: FacetKind | null =
          signal.kind === "EMBLEM"
            ? "emblem"
            : signal.kind === "ARTIFACT"
              ? "artifact"
              : signal.kind === "AUGMENT"
                ? "augment"
                : null;
        const label = t(`lineup.signal.${signal.kind}`, {
          entity: signal.entity.name || signal.entity.id,
          rate: percent(signal.rate),
        });
        const classes = `rounded-full border px-2 py-1 text-[10px] font-medium ${signalClass(signal.kind)} ${
          facet && active[facet] === signal.entity.id
            ? "ring-2 ring-accent"
            : ""
        }`;
        return facet ? (
          <button
            key={`${signal.kind}:${signal.entity.id}:${signal.holderUnit?.id ?? ""}`}
            type="button"
            className={`${classes} focus-visible:outline-none focus-visible:shadow-focus-ring`}
            aria-pressed={active[facet] === signal.entity.id}
            title={t("lineup.signalEvidence", {
              samples: signal.sampleSize,
              rate: percent(signal.rate),
            })}
            onClick={() =>
              onToggle(
                facet,
                active[facet] === signal.entity.id ? null : signal.entity.id,
              )
            }
          >
            {label}
          </button>
        ) : (
          <span
            key={`${signal.kind}:${signal.entity.id}:${signal.holderUnit?.id ?? ""}`}
            className={classes}
            title={t("lineup.signalEvidence", {
              samples: signal.sampleSize,
              rate: percent(signal.rate),
            })}
          >
            {label}
          </span>
        );
      })}
    </div>
  );
}

function SignalEvidenceList({ signals }: { signals: Lineup["signals"] }) {
  const { t, i18n } = useTranslation("tft");
  const percent = (value: number) =>
    new Intl.NumberFormat(i18n.resolvedLanguage, {
      style: "percent",
      maximumFractionDigits: 0,
    }).format(value);
  return (
    <section className="mt-4 rounded-lg border border-border bg-surface-raised p-3">
      <h3 className="text-xs font-medium text-fg-muted">
        {t("lineup.signalDetails")}
      </h3>
      <p className="mt-1 text-[11px] text-fg-subtle">
        {t("lineup.signalDisclaimer")}
      </p>
      <ul className="mt-2 grid gap-2 sm:grid-cols-2">
        {signals.map((signal) => (
          <li
            key={`${signal.kind}:${signal.entity.id}:${signal.holderUnit?.id ?? ""}`}
            className="flex items-center gap-2 rounded border border-border bg-surface-sunken p-2 text-xs"
          >
            <TftEntityIcon
              entityId={signal.entity.id}
              name={signal.entity.name}
              iconUrl={signal.entity.iconUrl}
              size="sm"
            />
            <span className="min-w-0 flex-1 text-fg-muted">
              <span className="block truncate font-medium">
                {t(`lineup.signal.${signal.kind}`, {
                  entity: signal.entity.name || signal.entity.id,
                  rate: percent(signal.rate),
                })}
              </span>
              {signal.holderUnit && (
                <span className="block truncate text-[10px] text-fg-subtle">
                  {t("lineup.signalHolder", {
                    unit: signal.holderUnit.name || signal.holderUnit.id,
                  })}
                </span>
              )}
            </span>
            <span className="shrink-0 font-mono text-[10px] text-fg-subtle">
              n={signal.sampleSize}
            </span>
          </li>
        ))}
      </ul>
    </section>
  );
}

function summarySignals(signals: Lineup["signals"]) {
  const kinds = ["EMBLEM", "ARTIFACT", "AUGMENT", "THREE_STAR"] as const;
  return kinds.flatMap((kind) => {
    const strongest = signals
      .filter((signal) => signal.kind === kind)
      .sort((left, right) => right.rate - left.rate)[0];
    return strongest ? [strongest] : [];
  });
}

function CopyTeamCodeButton({ code }: { code: string | null | undefined }) {
  const { t } = useTranslation("tft");
  const [state, setState] = useState<"idle" | "copied" | "error">("idle");
  async function copy() {
    if (!code) return;
    if (!navigator.clipboard) {
      setState("error");
      return;
    }
    try {
      await navigator.clipboard.writeText(code);
      setState("copied");
      window.setTimeout(() => setState("idle"), 1800);
    } catch {
      setState("error");
    }
  }
  if (!code) {
    return (
      <span
        role="status"
        aria-label={t("action.teamCodeUnavailable")}
        className="shrink-0 rounded border border-border bg-surface-sunken px-2.5 py-1.5 text-xs text-fg-subtle"
      >
        {t("action.teamCodeUnavailableShort")}
      </span>
    );
  }
  return (
    <div className="shrink-0">
      <Button
        variant="secondary"
        size="sm"
        onClick={() => void copy()}
        title={t("action.copyTeamCodeDescription")}
      >
        {state === "copied"
          ? t("action.copied")
          : state === "error"
            ? t("action.copyFailed")
            : t("action.copyTeamCode")}
      </Button>
      <span className="sr-only" aria-live="polite">
        {state === "copied"
          ? t("action.copied")
          : state === "error"
            ? t("action.copyFailed")
            : ""}
      </span>
    </div>
  );
}

function CompactMetric({ label, value }: { label: string; value: string }) {
  return (
    <div className="min-w-12">
      <dt className="text-[9px] uppercase tracking-wide text-fg-subtle">
        {label}
      </dt>
      <dd className="text-sm font-semibold text-fg-default">{value}</dd>
    </div>
  );
}

function collectLineupFacets(
  lineups: Lineup[],
): Record<FacetKind, LineupEntity[]> {
  const maps: Record<FacetKind, Map<string, LineupEntity>> = {
    core: new Map(),
    emblem: new Map(),
    artifact: new Map(),
    augment: new Map(),
  };
  for (const lineup of lineups) {
    for (const unit of lineup.unitItems) {
      if (unit.isCore) maps.core.set(unit.unit.id, unit.unit);
    }
    for (const signal of lineup.signals) {
      const kind = signal.kind.toLowerCase() as FacetKind;
      if (kind in maps) maps[kind].set(signal.entity.id, signal.entity);
    }
  }
  return {
    core: [...maps.core.values()].sort(
      (left, right) =>
        (left.cost ?? Number.MAX_SAFE_INTEGER) -
          (right.cost ?? Number.MAX_SAFE_INTEGER) ||
        left.id.localeCompare(right.id),
    ),
    emblem: [...maps.emblem.values()],
    artifact: [...maps.artifact.values()],
    augment: [...maps.augment.values()],
  };
}

function unitCostClass(cost: number | null | undefined) {
  switch (cost) {
    case 1:
      return "border-slate-400 bg-slate-400/10";
    case 2:
      return "border-emerald-400 bg-emerald-400/10";
    case 3:
      return "border-sky-400 bg-sky-400/10";
    case 4:
      return "border-violet-400 bg-violet-400/10";
    case 5:
      return "border-amber-300 bg-amber-300/10";
    case 6:
      return "border-rose-400 bg-rose-400/10";
    default:
      return "border-border-strong bg-surface-sunken";
  }
}

function signalClass(kind: string) {
  switch (kind) {
    case "EMBLEM":
      return "border-emerald-400/50 bg-emerald-400/10 text-emerald-200";
    case "ARTIFACT":
      return "border-amber-300/50 bg-amber-300/10 text-amber-100";
    case "THREE_STAR":
      return "border-fuchsia-400/50 bg-fuchsia-400/10 text-fuchsia-200";
    default:
      return "border-sky-400/50 bg-sky-400/10 text-sky-200";
  }
}

function UnitItemsSection({ units }: { units: Lineup["unitItems"] }) {
  const { t, i18n } = useTranslation("tft");
  const percent = (value: number) =>
    new Intl.NumberFormat(i18n.resolvedLanguage, {
      style: "percent",
      maximumFractionDigits: 0,
    }).format(value);
  const decimal = (value: number) =>
    new Intl.NumberFormat(i18n.resolvedLanguage, {
      maximumFractionDigits: 2,
    }).format(value);
  const ordered = [...units].sort(
    (left, right) =>
      (left.coreRank ?? Number.MAX_SAFE_INTEGER) -
        (right.coreRank ?? Number.MAX_SAFE_INTEGER) ||
      right.itemInvestmentRate - left.itemInvestmentRate ||
      left.unit.id.localeCompare(right.unit.id),
  );
  return (
    <section className="mt-4">
      <h3 className="mb-2 text-xs font-medium text-fg-muted">
        {t("lineup.unitAnalysis")}
      </h3>
      <ul className="grid gap-2 sm:grid-cols-2">
        {ordered.map((analysis) => {
          const { unit, commonItems } = analysis;
          const unitName = unit.name || unit.id;
          return (
            <li
              key={unit.id}
              className={`min-w-0 rounded-lg border bg-surface-sunken p-2 ${
                analysis.isCore ? "border-amber-400/50" : "border-border"
              }`}
              aria-label={t("lineup.unitItemsLabel", { unit: unitName })}
            >
              <div className="flex items-center gap-2">
                <TftEntityIcon
                  entityId={unit.id}
                  name={unit.name}
                  iconUrl={unit.iconUrl}
                  size="md"
                />
                <div className="min-w-0 flex-1">
                  <p className="flex items-center gap-1 truncate text-xs font-medium text-fg-muted">
                    <span className="truncate">{unitName}</span>
                    {analysis.isCore && (
                      <span className="shrink-0 rounded bg-amber-400/20 px-1.5 py-0.5 text-[10px] font-semibold text-amber-200">
                        {t("lineup.coreUnit", { rank: analysis.coreRank })}
                      </span>
                    )}
                  </p>
                  <p className="mt-0.5 text-[10px] text-fg-subtle">
                    {t("lineup.itemInvestment", {
                      average: decimal(analysis.averageItems),
                      fullRate: percent(analysis.threeItemRate),
                    })}
                  </p>
                  <p className="mt-0.5 text-[10px] text-fg-subtle">
                    {t("lineup.unitStarCoverage", {
                      known: analysis.knownStarSamples,
                      unknown: analysis.unknownStarSamples,
                      coverage: percent(analysis.starCoverage),
                    })}
                  </p>
                </div>
              </div>
              {commonItems.length > 0 && (
                <ul className="mt-1 flex flex-wrap gap-1.5">
                  {commonItems.map(({ entity, rate }) => (
                    <li
                      key={entity.id}
                      className="flex items-center gap-1 text-[10px] text-fg-subtle"
                      title={`${entity.name || entity.id} · ${percent(rate)}`}
                    >
                      <TftEntityIcon
                        entityId={entity.id}
                        name={entity.name}
                        iconUrl={entity.iconUrl}
                        size="sm"
                      />
                      <span className="max-w-24 truncate">
                        {entity.name || entity.id}
                      </span>
                      <span className="font-mono">{percent(rate)}</span>
                    </li>
                  ))}
                </ul>
              )}
              <ul className="mt-2 flex flex-wrap gap-1.5">
                {analysis.starDistribution.map((level) => {
                  const strength =
                    level.avgPlacement == null
                      ? t("lineup.smallSample", { samples: level.sampleSize })
                      : t("lineup.starBucketStrength", {
                          placement: decimal(level.avgPlacement),
                          top4: percent(level.top4Rate ?? 0),
                        });
                  return (
                    <li
                      key={level.stars}
                      className="rounded border border-border bg-surface-raised px-1.5 py-1 text-[10px] text-fg-subtle"
                      aria-label={t("lineup.unitStarDistributionLabel", {
                        unit: unitName,
                        stars: level.stars,
                        rate: percent(level.rate),
                        samples: level.sampleSize,
                        strength,
                      })}
                    >
                      <span className="font-semibold text-fg-muted">
                        {level.stars}★
                      </span>{" "}
                      {percent(level.rate)} · {strength}
                    </li>
                  );
                })}
              </ul>
            </li>
          );
        })}
      </ul>
    </section>
  );
}

function StarCompositionSection({
  compositions,
  knownSamples,
  unknownSamples,
  coverage,
}: {
  compositions: Lineup["starCompositions"];
  knownSamples: number;
  unknownSamples: number;
  coverage: number;
}) {
  const { t, i18n } = useTranslation("tft");
  const percent = (value: number) =>
    new Intl.NumberFormat(i18n.resolvedLanguage, {
      style: "percent",
      maximumFractionDigits: 1,
    }).format(value);
  const decimal = (value: number) =>
    new Intl.NumberFormat(i18n.resolvedLanguage, {
      maximumFractionDigits: 2,
    }).format(value);
  const stable = compositions.filter((item) => item.sampleSize >= 5);
  if (stable.length === 0) return null;
  return (
    <section className="mt-4">
      <h3 className="mb-1 text-xs font-medium text-fg-muted">
        {t("lineup.starComposition")}
      </h3>
      <p className="mb-2 text-[10px] text-fg-subtle">
        {t("lineup.starCompositionCoverage", {
          known: knownSamples,
          unknown: unknownSamples,
          coverage: percent(coverage),
        })}
      </p>
      <div
        className="overflow-x-auto rounded-lg border border-border"
        role="region"
        aria-label={t("lineup.starComposition")}
        tabIndex={0}
      >
        <table className="w-full min-w-[38rem] text-left text-xs">
          <caption className="sr-only">{t("lineup.starComposition")}</caption>
          <thead className="bg-surface-sunken text-[10px] uppercase tracking-wide text-fg-subtle">
            <tr>
              <th scope="col" className="px-2 py-2">
                {t("lineup.starDistribution")}
              </th>
              <th scope="col" className="px-2 py-2 text-right">
                {t("lineup.totalStarsLabel")}
              </th>
              <th scope="col" className="px-2 py-2 text-right">
                {t("metric.samples")}
              </th>
              <th scope="col" className="px-2 py-2 text-right">
                {t("metric.sampleShare")}
              </th>
              <th scope="col" className="px-2 py-2 text-right">
                {t("metric.avgPlacement")}
              </th>
              <th scope="col" className="px-2 py-2 text-right">
                {t("metric.firstRate")}
              </th>
              <th scope="col" className="px-2 py-2 text-right">
                {t("metric.top4Rate")}
              </th>
            </tr>
          </thead>
          <tbody className="divide-y divide-border">
            {stable.map((composition) => (
              <tr
                key={composition.levels
                  .map((level) => `${level.stars}:${level.unitCount}`)
                  .join("|")}
              >
                <th
                  scope="row"
                  className="whitespace-nowrap px-2 py-2 font-medium text-fg-muted"
                >
                  {composition.levels
                    .map((level) => `${level.stars}★×${level.unitCount}`)
                    .join(" · ")}
                </th>
                <td className="px-2 py-2 text-right text-fg-default">
                  {composition.totalStars}
                </td>
                <td className="px-2 py-2 text-right text-fg-default">
                  {composition.sampleSize}
                </td>
                <td className="px-2 py-2 text-right text-fg-default">
                  {percent(composition.rate)}
                </td>
                <td className="px-2 py-2 text-right text-fg-default">
                  {composition.avgPlacement == null
                    ? "—"
                    : decimal(composition.avgPlacement)}
                </td>
                <td className="px-2 py-2 text-right text-fg-default">
                  {composition.firstRate == null
                    ? "—"
                    : percent(composition.firstRate)}
                </td>
                <td className="px-2 py-2 text-right text-fg-default">
                  {composition.top4Rate == null
                    ? "—"
                    : percent(composition.top4Rate)}
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
    </section>
  );
}

function StarStrengthSection({ levels }: { levels: Lineup["starLevels"] }) {
  const { t, i18n } = useTranslation("tft");
  const percent = (value: number) =>
    new Intl.NumberFormat(i18n.resolvedLanguage, {
      style: "percent",
      maximumFractionDigits: 1,
    }).format(value);
  const decimal = (value: number) =>
    new Intl.NumberFormat(i18n.resolvedLanguage, {
      maximumFractionDigits: 2,
    }).format(value);
  return (
    <section className="mt-4">
      <h3 className="mb-2 text-xs font-medium text-fg-muted">
        {t("lineup.starStrength")}
      </h3>
      <div
        className="overflow-x-auto rounded-lg border border-border"
        role="region"
        aria-label={t("lineup.starStrength")}
        tabIndex={0}
      >
        <table className="w-full min-w-[34rem] text-left text-xs">
          <caption className="sr-only">{t("lineup.starStrength")}</caption>
          <thead className="bg-surface-sunken text-[10px] uppercase tracking-wide text-fg-subtle">
            <tr>
              <th scope="col" className="px-2 py-2">
                {t("lineup.starStrength")}
              </th>
              <th scope="col" className="px-2 py-2 text-right">
                {t("metric.samples")}
              </th>
              <th scope="col" className="px-2 py-2 text-right">
                {t("metric.sampleShare")}
              </th>
              <th scope="col" className="px-2 py-2 text-right">
                {t("metric.avgPlacement")}
              </th>
              <th scope="col" className="px-2 py-2 text-right">
                {t("metric.firstRate")}
              </th>
              <th scope="col" className="px-2 py-2 text-right">
                {t("metric.top4Rate")}
              </th>
            </tr>
          </thead>
          <tbody className="divide-y divide-border">
            {levels.map((level) => (
              <tr key={level.totalStars}>
                <th
                  scope="row"
                  className="whitespace-nowrap px-2 py-2 font-medium text-fg-muted"
                >
                  {t("lineup.totalStars", { value: level.totalStars })}
                </th>
                <td className="px-2 py-2 text-right text-fg-default">
                  {level.sampleSize}
                </td>
                <td className="px-2 py-2 text-right text-fg-default">
                  {percent(level.rate)}
                </td>
                <td className="px-2 py-2 text-right text-fg-default">
                  {decimal(level.avgPlacement)}
                </td>
                <td className="px-2 py-2 text-right text-fg-default">
                  {percent(level.firstRate)}
                </td>
                <td className="px-2 py-2 text-right text-fg-default">
                  {percent(level.top4Rate)}
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
    </section>
  );
}

function Metric({ label, value }: { label: string; value: string }) {
  return (
    <div className="rounded bg-surface-sunken px-2 py-2">
      <dt className="text-[10px] uppercase tracking-wide text-fg-subtle">
        {label}
      </dt>
      <dd className="mt-0.5 text-sm font-medium text-fg-default">{value}</dd>
    </div>
  );
}

function EntityCounts({
  title,
  entities,
}: {
  title: string;
  entities: Lineup["commonItems"];
}) {
  const { i18n } = useTranslation("tft");
  return (
    <div>
      <h3 className="mb-2 text-xs font-medium text-fg-muted">{title}</h3>
      <ul className="space-y-1.5">
        {entities.slice(0, 5).map(({ entity, rate }) => (
          <li key={entity.id} className="flex items-center gap-2 text-xs">
            <TftEntityIcon
              entityId={entity.id}
              name={entity.name}
              iconUrl={entity.iconUrl}
              size="sm"
            />
            <span className="min-w-0 flex-1 truncate text-fg-muted">
              {entity.name || entity.id}
            </span>
            <span className="font-mono text-fg-subtle">
              {new Intl.NumberFormat(i18n.resolvedLanguage, {
                style: "percent",
                maximumFractionDigits: 0,
              }).format(rate)}
            </span>
          </li>
        ))}
      </ul>
    </div>
  );
}

function PageState({
  title,
  description,
  action,
}: {
  title: string;
  description: string;
  action?: React.ReactNode;
}) {
  return (
    <section className="rounded-xl border border-border bg-surface-raised px-6 py-14 text-center">
      <h1 className="text-xl font-semibold text-fg-default">{title}</h1>
      <p className="mx-auto mt-2 max-w-xl text-sm text-fg-muted">
        {description}
      </p>
      {action && <div className="mt-5">{action}</div>}
    </section>
  );
}

function LinkButton({
  to,
  children,
}: {
  to: string;
  children: React.ReactNode;
}) {
  return (
    <Link
      to={to}
      className="inline-flex h-9 items-center justify-center rounded bg-accent px-4 text-sm font-medium text-fg-inverse transition hover:bg-accent-hover focus-visible:outline-none focus-visible:shadow-focus-ring"
    >
      {children}
    </Link>
  );
}

function AnalysisSkeleton({ compact = false }: { compact?: boolean }) {
  return (
    <div className="space-y-4" aria-label="loading">
      {!compact && <Skeleton className="h-20 w-full" />}
      <Skeleton className="h-24 w-full" />
      <div className="grid gap-4 lg:grid-cols-2">
        <Skeleton className="h-96 w-full" />
        <Skeleton className="h-96 w-full" />
      </div>
    </div>
  );
}

function parseOptionalInt(value: string | null) {
  if (!value) return undefined;
  const parsed = Number(value);
  return Number.isInteger(parsed) ? parsed : undefined;
}

function analysisSearchParams(
  selection: TftCatalogSelection,
  minSamples: number,
  current?: URLSearchParams,
) {
  const next = retainedLineupParams(current);
  next.set("platform", selection.platform);
  next.set("patch", selection.patch);
  next.set("set", String(selection.setNumber));
  next.set("cohort", selection.cohort);
  next.set("window", selection.window);
  next.set("minSamples", String(minSamples));
  return next;
}

function previewSearchParams(
  platform: string,
  minSamples: number,
  current?: URLSearchParams,
) {
  const next = retainedLineupParams(current);
  next.set("platform", platform);
  next.set("minSamples", String(minSamples));
  return next;
}

function retainedLineupParams(current?: URLSearchParams) {
  const next = new URLSearchParams();
  if (!current) return next;
  for (const key of ["core", "emblem", "artifact", "augment", "lineup"]) {
    const value = current.get(key);
    if (value) next.set(key, value);
  }
  return next;
}

function normalizePreviewPlatform(value: string | null) {
  return value?.trim().toUpperCase() === "GLOBAL"
    ? "GLOBAL"
    : normalizeTftPlatform(value);
}

function formatDate(value: string, locale?: string) {
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return value;
  return new Intl.DateTimeFormat(locale, {
    dateStyle: "medium",
    timeStyle: "short",
  }).format(date);
}
