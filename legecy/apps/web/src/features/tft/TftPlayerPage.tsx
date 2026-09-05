import { useEffect, useMemo, useRef, useState } from "react";
import { useTranslation } from "react-i18next";
import { Link, useParams } from "react-router-dom";
import { useQueryClient } from "@tanstack/react-query";

import {
  type TftLookupJobQuery,
  type TftMatchHistoryQuery,
  useRefreshTftPlayerMutation,
  useTftLookupJobQuery,
  useTftMatchHistoryQuery,
} from "@shared/api";
import {
  asGraphQLRequestError,
  type GraphQLRequestError,
} from "@shared/api/fetcher";
import {
  Button,
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
  Skeleton,
} from "@shared/ui";

import { TftMatchCard } from "./components/TftMatchCard";
import { TftPlayerSearchForm } from "./components/TftPlayerSearchForm";
import { normalizeTftPlatform } from "./lib/platforms";

type HistoryResult = NonNullable<TftMatchHistoryQuery["tftMatchHistory"]>;
type Match = HistoryResult["matches"][number];
type LookupJob = NonNullable<TftLookupJobQuery["tftLookupJob"]>;

const TERMINAL_STATUSES = new Set(["COMPLETED", "PARTIAL", "FAILED"]);
const QUEUE_OPTIONS = ["ALL", "1100", "1090", "1160", "1130"] as const;

export function TftPlayerPage() {
  const { t, i18n } = useTranslation("tft");
  const queryClient = useQueryClient();
  const params = useParams<{
    platform: string;
    gameName: string;
    tagLine: string;
  }>();
  const platform = normalizeTftPlatform(params.platform);
  const gameName = params.gameName;
  const tagLine = params.tagLine;
  const hasIdentity = Boolean(params.platform && gameName && tagLine);
  const identity = useMemo(
    () => ({ platform, gameName: gameName ?? "", tagLine: tagLine ?? "" }),
    [platform, gameName, tagLine],
  );
  const identityKey = `${platform}/${gameName ?? ""}#${tagLine ?? ""}`;
  const locale = i18n.resolvedLanguage === "en-US" ? "en_us" : "zh_cn";
  const [queue, setQueue] = useState<(typeof QUEUE_OPTIONS)[number]>("ALL");
  const [after, setAfter] = useState<string | null>(null);
  const [matches, setMatches] = useState<Match[]>([]);
  const [jobID, setJobID] = useState<string | null>(null);
  const [notice, setNotice] = useState("");
  const [retryAt, setRetryAt] = useState<number | null>(null);
  const [clock, setClock] = useState(Date.now());
  const autoRefreshKey = useRef("");

  useEffect(() => {
    setQueue("ALL");
    setAfter(null);
    setMatches([]);
    setJobID(null);
    setNotice("");
    setRetryAt(null);
    autoRefreshKey.current = "";
  }, [identityKey]);

  useEffect(() => {
    // Entity names and icon paths are localized server-side. A language
    // change must restart pagination so match-id de-duplication cannot retain
    // entities from the previous locale.
    setAfter(null);
    setMatches([]);
  }, [locale]);

  const historyQuery = useTftMatchHistoryQuery(
    {
      input: {
        ...identity,
        first: 20,
        after,
        locale,
        queueId: queue === "ALL" ? 0 : Number(queue),
      },
    },
    { enabled: hasIdentity },
  );
  const result = historyQuery.data?.tftMatchHistory;

  useEffect(() => {
    const page = historyQuery.data?.tftMatchHistory?.matches;
    if (!page) return;
    setMatches((current) => {
      if (!after) return page;
      const known = new Set(current.map((match) => match.matchId));
      return [...current, ...page.filter((match) => !known.has(match.matchId))];
    });
  }, [after, historyQuery.data]);

  const refreshMutation = useRefreshTftPlayerMutation<GraphQLRequestError>({
    onMutate() {
      setNotice("");
    },
    onSuccess(data) {
      setRetryAt(null);
      const job = data.refreshTFTPlayer.job;
      if (job) {
        setJobID(job.id);
      } else if (data.refreshTFTPlayer.fresh) {
        setNotice(t("player.refresh.alreadyFresh"));
      }
    },
    onError(error) {
      if (error.code === "RATE_LIMITED" && error.retryAfterSeconds) {
        setRetryAt(Date.now() + error.retryAfterSeconds * 1_000);
        setClock(Date.now());
      } else {
        setRetryAt(null);
      }
    },
  });
  const retrySeconds = retryAt
    ? Math.max(0, Math.ceil((retryAt - clock) / 1_000))
    : 0;

  useEffect(() => {
    if (!retryAt) return;
    const timer = window.setInterval(() => setClock(Date.now()), 1_000);
    return () => window.clearInterval(timer);
  }, [retryAt]);

  useEffect(() => {
    if (!retryAt || retrySeconds > 0) return;
    setRetryAt(null);
    refreshMutation.reset();
  }, [refreshMutation, retryAt, retrySeconds]);

  function refresh() {
    if (!hasIdentity || refreshMutation.isPending || jobID || retrySeconds > 0)
      return;
    setNotice("");
    refreshMutation.mutate({ identity });
  }

  useEffect(() => {
    if (
      !hasIdentity ||
      historyQuery.isPending ||
      historyQuery.isError ||
      autoRefreshKey.current === identityKey ||
      (result && !result.profile.isStale)
    ) {
      return;
    }
    autoRefreshKey.current = identityKey;
    refreshMutation.mutate({ identity });
  }, [
    hasIdentity,
    historyQuery.isError,
    historyQuery.isPending,
    identity,
    identityKey,
    refreshMutation,
    result,
  ]);

  const jobQuery = useTftLookupJobQuery(
    { id: jobID ?? "" },
    { enabled: Boolean(jobID), refetchInterval: jobID ? 2_000 : false },
  );
  const job = jobQuery.data?.tftLookupJob;

  useEffect(() => {
    if (!job || !TERMINAL_STATUSES.has(job.status)) return;
    if (job.status === "FAILED") {
      const key = jobFailureKey(job.errorCode);
      setNotice(t(key, { platform: job.platform }));
    } else {
      setNotice(
        job.status === "PARTIAL"
          ? t("player.refresh.partial")
          : t("player.refresh.complete"),
      );
    }
    setJobID(null);
    setAfter(null);
    setMatches([]);
    void queryClient.invalidateQueries({ queryKey: ["TFTMatchHistory"] });
  }, [job, queryClient, t]);

  useEffect(() => {
    if (!jobID || !jobQuery.isSuccess || job !== null) return;
    setJobID(null);
    setNotice(t("player.refresh.jobExpired"));
    void queryClient.invalidateQueries({ queryKey: ["TFTMatchHistory"] });
  }, [job, jobID, jobQuery.isSuccess, queryClient, t]);

  function changeQueue(value: string) {
    setQueue(value as (typeof QUEUE_OPTIONS)[number]);
    setAfter(null);
    setMatches([]);
  }

  if (!hasIdentity) {
    return (
      <section className="py-10 sm:py-20">
        <div className="mx-auto mb-8 max-w-2xl text-center">
          <p className="text-sm font-medium uppercase tracking-[0.2em] text-accent">
            TFT
          </p>
          <h1 className="mt-2 text-3xl font-semibold text-fg-default sm:text-4xl">
            {t("player.title")}
          </h1>
          <p className="mt-3 text-fg-muted">{t("player.description")}</p>
        </div>
        <TftPlayerSearchForm />
        <p className="mx-auto mt-4 max-w-2xl text-center text-xs text-fg-subtle">
          {t("player.search.supported")}
        </p>
        <p className="mt-8 text-center">
          <Link className="text-sm text-accent hover:underline" to="/tft">
            {t("action.viewLineups")}
          </Link>
        </p>
      </section>
    );
  }

  return (
    <section className="space-y-5">
      <TftPlayerSearchForm
        compact
        initialPlatform={platform}
        initialRiotID={`${gameName}#${tagLine}`}
      />

      {result && (
        <header className="rounded-xl border border-border bg-surface-raised p-5">
          <div className="flex flex-col justify-between gap-4 sm:flex-row sm:items-center">
            <div>
              <p className="text-xs font-medium uppercase tracking-widest text-accent">
                {result.profile.platform} · TFT
              </p>
              <h1 className="mt-1 text-2xl font-semibold text-fg-default">
                {result.profile.gameName}
                <span className="text-fg-subtle">
                  #{result.profile.tagLine}
                </span>
              </h1>
              <p className="mt-1 text-xs text-fg-subtle">
                {t("player.profile.updated", {
                  value: formatDate(
                    result.profile.lastRefreshedAt,
                    i18n.resolvedLanguage,
                  ),
                })}
              </p>
            </div>
            <Button
              variant="secondary"
              disabled={
                Boolean(jobID) || refreshMutation.isPending || retrySeconds > 0
              }
              onClick={refresh}
            >
              {jobID || refreshMutation.isPending
                ? t("player.refresh.refreshing")
                : t("player.refresh.action")}
            </Button>
          </div>
        </header>
      )}

      {(jobID || notice || refreshMutation.isError) && (
        <RefreshStatus
          job={job}
          notice={notice}
          error={refreshMutation.error}
          retrySeconds={retrySeconds}
        />
      )}

      {historyQuery.isPending && !result && <HistorySkeleton />}
      {historyQuery.isError && !result && (
        <PageState
          title={t("player.state.error")}
          description={t("player.state.errorDescription")}
          action={
            <Button onClick={() => void historyQuery.refetch()}>
              {t("action.retry")}
            </Button>
          }
        />
      )}
      {!historyQuery.isPending &&
        !result &&
        (jobID || refreshMutation.isPending) && (
          <PageState
            title={t("player.state.fetchingFirst")}
            description={t("player.state.fetchingFirstDescription")}
          />
        )}
      {!historyQuery.isPending &&
        !result &&
        !jobID &&
        !refreshMutation.isPending &&
        !historyQuery.isError && (
          <PageState
            title={t("player.state.notFound")}
            description={notice || t("player.state.notFoundDescription")}
          />
        )}

      {result && (
        <>
          <div className="flex flex-col justify-between gap-3 sm:flex-row sm:items-end">
            <div>
              <h2 className="text-xl font-semibold text-fg-default">
                {t("player.history.title")}
              </h2>
              <p className="text-xs text-fg-subtle">
                {t("player.history.scope")}
              </p>
            </div>
            <Select value={queue} onValueChange={changeQueue}>
              <SelectTrigger
                className="w-full sm:w-48"
                aria-label={t("player.history.queueFilter")}
              >
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                {QUEUE_OPTIONS.map((value) => (
                  <SelectItem key={value} value={value}>
                    {t(`player.queue.${value}`)}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
          </div>

          {matches.length === 0 && !historyQuery.isFetching && (
            <PageState
              title={t("player.state.empty")}
              description={t("player.state.emptyDescription")}
            />
          )}
          <div className="space-y-3">
            {matches.map((match) => (
              <TftMatchCard key={match.matchId} match={match} />
            ))}
          </div>
          {historyQuery.isFetching && matches.length > 0 && (
            <Skeleton className="h-36 w-full" />
          )}
          {result.pageInfo.hasNextPage && result.pageInfo.endCursor && (
            <div className="text-center">
              <Button
                variant="secondary"
                disabled={historyQuery.isFetching}
                onClick={() => setAfter(result.pageInfo.endCursor)}
              >
                {historyQuery.isFetching
                  ? t("player.history.loadingMore")
                  : t("player.history.loadMore")}
              </Button>
            </div>
          )}
          {!result.pageInfo.hasNextPage && matches.length > 0 && (
            <p className="text-center text-xs text-fg-subtle">
              {t("player.history.end")}
            </p>
          )}
        </>
      )}
    </section>
  );
}

function RefreshStatus({
  job,
  notice,
  error,
  retrySeconds,
}: {
  job: LookupJob | null | undefined;
  notice: string;
  error: unknown;
  retrySeconds: number;
}) {
  const { t } = useTranslation("tft");
  if (job) {
    const total = Math.max(
      job.scannedCount,
      job.fetchedCount + job.failedCount,
      1,
    );
    const progress = Math.min(
      100,
      Math.round(((job.fetchedCount + job.failedCount) / total) * 100),
    );
    return (
      <aside
        className="rounded-xl border border-border-accent bg-accent-subtle p-3"
        aria-live="polite"
      >
        <div className="flex justify-between gap-3 text-sm text-fg-default">
          <span>{t(`player.stage.${job.stage}`)}</span>
          <span>
            {t("player.refresh.progress", {
              scanned: job.scannedCount,
              fetched: job.fetchedCount,
            })}
          </span>
        </div>
        <div className="mt-2 h-1.5 overflow-hidden rounded bg-surface-sunken">
          <div
            className="h-full bg-accent transition-all"
            style={{ width: `${progress}%` }}
          />
        </div>
      </aside>
    );
  }
  const requestError = asGraphQLRequestError(error);
  let text = notice;
  if (requestError?.code === "RATE_LIMITED") {
    text = t("player.refresh.rateLimited", { seconds: retrySeconds });
  } else if (requestError?.code === "NETWORK_ERROR") {
    text = t("player.refresh.networkError");
  } else if (requestError?.code === "SERVICE_UNAVAILABLE") {
    text = t("player.refresh.serviceUnavailable");
  } else if (error) {
    text = t("player.refresh.failed");
  }
  return (
    <p
      className={`rounded-xl border p-3 text-sm ${error ? "border-red-500/50 text-red-300" : "border-border text-fg-muted"}`}
      role={error ? "alert" : "status"}
    >
      {text}
    </p>
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
    <div className="rounded-xl border border-border bg-surface-raised px-6 py-12 text-center">
      <h2 className="text-lg font-semibold text-fg-default">{title}</h2>
      <p className="mx-auto mt-2 max-w-xl text-sm text-fg-muted">
        {description}
      </p>
      {action && <div className="mt-4">{action}</div>}
    </div>
  );
}

function HistorySkeleton() {
  return (
    <div className="space-y-3">
      <Skeleton className="h-28 w-full" />
      <Skeleton className="h-48 w-full" />
      <Skeleton className="h-48 w-full" />
    </div>
  );
}

function jobFailureKey(errorCode: string | null | undefined) {
  switch (errorCode) {
    case "NOT_FOUND":
      return "player.refresh.notFound" as const;
    case "NOT_FOUND_IN_REGION":
      return "player.refresh.notFoundInPlatform" as const;
    case "PROFILE_NOT_READY":
      return "player.refresh.profileUnavailable" as const;
    default:
      return "player.refresh.failed" as const;
  }
}

function formatDate(value: string | null | undefined, locale?: string) {
  if (!value) return "—";
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return value;
  return new Intl.DateTimeFormat(locale, {
    dateStyle: "medium",
    timeStyle: "short",
  }).format(date);
}
