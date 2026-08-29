import { useEffect, useMemo } from "react";
import { useTranslation } from "react-i18next";
import { Link, useSearchParams } from "react-router-dom";

import {
  type TftAnalysisCatalogQuery,
  type TftCohort,
  type TftLineupsQuery,
  type TftWindow,
  useTftAnalysisCatalogQuery,
  useTftLineupsQuery,
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
  selectCatalogEntry,
  type TftCatalogSelection,
} from "./lib/analysisFilters";

type CatalogEntry = TftAnalysisCatalogQuery["tftAnalysisCatalog"][number];
type Lineup = TftLineupsQuery["tftLineups"]["items"][number];

export function TftAnalysisPage() {
  const { t, i18n } = useTranslation("tft");
  const [searchParams, setSearchParams] = useSearchParams();
  const catalogQuery = useTftAnalysisCatalogQuery();
  const catalog = useMemo(
    () => catalogQuery.data?.tftAnalysisCatalog ?? [],
    [catalogQuery.data?.tftAnalysisCatalog],
  );
  const minSamples = readMinSamples(searchParams.get("minSamples"));
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

  useEffect(() => {
    if (!selected) return;
    const next = analysisSearchParams(selected, minSamples);
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

  function changeSelection(change: Partial<TftCatalogSelection>) {
    if (!selected) return;
    const next = selectCatalogEntry(catalog, { ...selected, ...change });
    if (next) setSearchParams(analysisSearchParams(next, minSamples));
  }

  function changeMinSamples(value: string) {
    if (!selected) return;
    setSearchParams(analysisSearchParams(selected, readMinSamples(value)));
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
      <PageState
        title={t("state.notPublished")}
        description={t("state.notPublishedDescription")}
        action={
          <LinkButton to="/tft/player">{t("action.searchPlayer")}</LinkButton>
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
        <ol
          className="grid gap-4 lg:grid-cols-2"
          aria-label={t("analysis.lineupListLabel")}
        >
          {result.items.map((lineup, index) => (
            <li key={lineup.id}>
              <LineupCard lineup={lineup} rank={index + 1} />
            </li>
          ))}
        </ol>
      )}
    </section>
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

function SummaryMetric({ label, value }: { label: string; value: string }) {
  return (
    <div>
      <p className="text-xs text-fg-muted">{label}</p>
      <p className="mt-1 text-lg font-semibold text-fg-default">{value}</p>
    </div>
  );
}

function LineupCard({ lineup, rank }: { lineup: Lineup; rank: number }) {
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
    <article className="h-full rounded-xl border border-border bg-surface-raised p-4 shadow-card">
      <header className="flex items-start justify-between gap-4">
        <div>
          <p className="text-xs font-medium uppercase tracking-widest text-accent">
            {t("lineup.rank", { rank })}
          </p>
          <h2 className="mt-1 text-base font-semibold text-fg-default">
            {lineup.coreUnits.map((unit) => unit.name || unit.id).join(" · ")}
          </h2>
        </div>
        <div className="text-right">
          <p className="text-2xl font-semibold text-fg-default">
            {decimal(lineup.metrics.avgPlacement)}
          </p>
          <p className="text-xs text-fg-subtle">{t("metric.avgPlacement")}</p>
        </div>
      </header>

      <div
        className="mt-4 flex flex-wrap gap-2"
        aria-label={t("lineup.coreUnits")}
      >
        {lineup.coreUnits.map((unit) => (
          <div
            key={unit.id}
            className="flex items-center gap-2 rounded-lg border border-border bg-surface-sunken p-1.5 pr-2"
          >
            <TftEntityIcon
              entityId={unit.id}
              name={unit.name}
              iconUrl={unit.iconUrl}
              size="lg"
            />
            <span className="max-w-24 truncate text-xs text-fg-muted">
              {unit.name || unit.id}
            </span>
          </div>
        ))}
      </div>

      <dl className="mt-4 grid grid-cols-3 gap-2 text-center sm:grid-cols-6 lg:grid-cols-3 xl:grid-cols-6">
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

      <div className="mt-4 grid gap-3 sm:grid-cols-3">
        <EntityCounts title={t("lineup.items")} entities={lineup.commonItems} />
        <EntityCounts
          title={t("lineup.augments")}
          entities={lineup.commonAugments}
        />
        <EntityCounts
          title={t("lineup.traits")}
          entities={lineup.commonTraits}
        />
      </div>
    </article>
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
) {
  return new URLSearchParams({
    platform: selection.platform,
    patch: selection.patch,
    set: String(selection.setNumber),
    cohort: selection.cohort,
    window: selection.window,
    minSamples: String(minSamples),
  });
}

function formatDate(value: string, locale?: string) {
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return value;
  return new Intl.DateTimeFormat(locale, {
    dateStyle: "medium",
    timeStyle: "short",
  }).format(date);
}
