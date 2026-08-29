import { useEffect, useMemo, useRef, useState } from "react";
import { useTranslation } from "react-i18next";
import { useParams } from "react-router-dom";
import { useQueryClient } from "@tanstack/react-query";

import {
  type SummonerQuery,
  type SummonerLookupJobQuery,
  type SummonerQueueFilter,
  useRefreshSummonerMutation,
  useSummonerLookupJobQuery,
  useSummonerQuery,
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
import {
  asGraphQLRequestError,
  type GraphQLRequestError,
} from "@shared/api/fetcher";

import { SummonerMatchCard } from "./components/SummonerMatchCard";
import { SummonerProfileHeader } from "./components/SummonerProfileHeader";
import { SummonerSearchForm } from "./components/SummonerSearchForm";

type Match = NonNullable<SummonerQuery["summoner"]>["history"]["items"][number];
type LookupJob = NonNullable<SummonerLookupJobQuery["summonerLookupJob"]>;
const TERMINAL_STATUSES = new Set(["COMPLETED", "PARTIAL", "FAILED"]);
const QUEUED_STALL_MS = 2 * 60_000;
const RUNNING_STALL_MS = 20 * 60_000;

export function SummonerPage() {
  const { t } = useTranslation("summoner");
  const queryClient = useQueryClient();
  const params = useParams<{
    region: string;
    gameName: string;
    tagLine: string;
    name: string;
  }>();
  const region = params.region?.toUpperCase();
  const gameName = params.gameName;
  const tagLine = params.tagLine;
  const hasIdentity = Boolean(region && gameName && tagLine);
  const identity = useMemo(
    () => ({
      region: region ?? "KR",
      gameName: gameName ?? "",
      tagLine: tagLine ?? "",
    }),
    [region, gameName, tagLine],
  );
  const identityKey = `${identity.region}/${identity.gameName}#${identity.tagLine}`;
  const [queue, setQueue] = useState<SummonerQueueFilter>("ALL");
  const [after, setAfter] = useState<string | null>(null);
  const [matches, setMatches] = useState<Match[]>([]);
  const [jobID, setJobID] = useState<string | null>(null);
  const [jobNotice, setJobNotice] = useState("");
  const [retryAt, setRetryAt] = useState<number | null>(null);
  const [clock, setClock] = useState(Date.now());
  const autoRefreshKey = useRef("");

  useEffect(() => {
    setQueue("ALL");
    setAfter(null);
    setMatches([]);
    setJobID(null);
    setJobNotice("");
    setRetryAt(null);
    autoRefreshKey.current = "";
  }, [identityKey]);

  const summonerQuery = useSummonerQuery(
    { identity, history: { queue, first: 20, after } },
    { enabled: hasIdentity },
  );
  const result = summonerQuery.data?.summoner;

  useEffect(() => {
    const pageItems = summonerQuery.data?.summoner?.history.items;
    if (!pageItems) return;
    setMatches((current) => {
      if (!after) return pageItems;
      const known = new Set(current.map((match) => match.matchId));
      return [
        ...current,
        ...pageItems.filter((match) => !known.has(match.matchId)),
      ];
    });
  }, [after, summonerQuery.data]);

  const refreshMutation = useRefreshSummonerMutation<GraphQLRequestError>({
    onMutate() {
      setJobNotice("");
    },
    onSuccess(data) {
      setRetryAt(null);
      const job = data.refreshSummoner.job;
      if (job) {
        setJobID(job.id);
        setJobNotice("");
      } else if (data.refreshSummoner.fresh) {
        setJobNotice(t("refresh.alreadyFresh"));
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
    setJobNotice("");
    refreshMutation.mutate({ identity });
  }

  useEffect(() => {
    if (
      !hasIdentity ||
      summonerQuery.isPending ||
      summonerQuery.isError ||
      autoRefreshKey.current === identityKey ||
      (result && !result.profile.isStale)
    ) {
      return;
    }
    autoRefreshKey.current = identityKey;
    refreshMutation.mutate({ identity });
  }, [
    hasIdentity,
    identity,
    identityKey,
    refreshMutation,
    result,
    summonerQuery.isError,
    summonerQuery.isPending,
  ]);

  const jobQuery = useSummonerLookupJobQuery(
    { id: jobID ?? "" },
    { enabled: Boolean(jobID), refetchInterval: jobID ? 2_000 : false },
  );
  const job = jobQuery.data?.summonerLookupJob;

  useEffect(() => {
    if (!job || !TERMINAL_STATUSES.has(job.status)) return;
    if (job.status === "FAILED") {
      if (job.errorCode === "NOT_FOUND") {
        setJobNotice(t("refresh.notFound"));
      } else if (job.errorCode === "NOT_FOUND_IN_REGION") {
        setJobNotice(t("refresh.notFoundInRegion", { region: job.region }));
      } else if (
        job.errorCode === "SUMMONER_PROFILE_NOT_READY" ||
        job.errorCode === "RANK_PROFILE_NOT_READY"
      ) {
        setJobNotice(t("refresh.profileUnavailable"));
      } else {
        setJobNotice(t("refresh.failed"));
      }
      setJobID(null);
      // Account resolution may already have repaired a stale Riot ID before a
      // later profile request failed. Re-read the local cache without clearing
      // the currently displayed matches.
      void queryClient.invalidateQueries({ queryKey: ["Summoner"] });
      return;
    } else {
      setJobNotice(
        job.status === "PARTIAL" ? t("refresh.partial") : t("refresh.complete"),
      );
    }
    setJobID(null);
    setAfter(null);
    setMatches([]);
    void queryClient.invalidateQueries({ queryKey: ["Summoner"] });
  }, [job, queryClient, t]);

  useEffect(() => {
    if (!jobID || !jobQuery.isSuccess || job !== null) return;
    setJobID(null);
    setJobNotice(t("refresh.jobExpired"));
    void queryClient.invalidateQueries({ queryKey: ["Summoner"] });
  }, [job, jobID, jobQuery.isSuccess, queryClient, t]);

  function changeQueue(value: string) {
    setQueue(value as SummonerQueueFilter);
    setAfter(null);
    setMatches([]);
  }

  if (!hasIdentity) {
    return (
      <section className="py-10 sm:py-20">
        <div className="mx-auto mb-8 max-w-2xl text-center">
          <p className="text-sm font-medium uppercase tracking-widest text-accent">
            GOGG
          </p>
          <h1 className="mt-2 text-3xl font-semibold text-fg-default sm:text-4xl">
            {t("title")}
          </h1>
          <p className="mt-3 text-fg-muted">{t("description")}</p>
        </div>
        <SummonerSearchForm />
        <p className="mx-auto mt-4 max-w-2xl text-center text-xs text-fg-subtle">
          {t("search.supported")}
        </p>
      </section>
    );
  }

  return (
    <section className="space-y-5">
      <SummonerSearchForm
        compact
        initialRegion={region}
        initialRiotID={`${gameName}#${tagLine}`}
      />

      {result && (
        <SummonerProfileHeader
          profile={result.profile}
          ranks={result.ranks}
          refreshing={Boolean(jobID) || refreshMutation.isPending}
          refreshDisabled={retrySeconds > 0}
          onRefresh={refresh}
        />
      )}

      {(jobID || jobNotice || refreshMutation.isError) && (
        <RefreshStatus
          job={job}
          notice={jobNotice}
          mutationError={refreshMutation.error}
          retrySeconds={retrySeconds}
        />
      )}

      {summonerQuery.isPending && !result && (
        <div className="space-y-3">
          <Skeleton className="h-44 w-full" />
          <Skeleton className="h-36 w-full" />
          <Skeleton className="h-36 w-full" />
        </div>
      )}

      {summonerQuery.isError && !result && <State text={t("state.error")} />}

      {!summonerQuery.isPending &&
        !result &&
        (jobID || refreshMutation.isPending) && (
          <State text={t("state.fetchingFirst")} />
        )}

      {!summonerQuery.isPending &&
        !result &&
        !jobID &&
        !refreshMutation.isPending && (
          <State text={jobNotice || t("state.notFound")} />
        )}

      {result && (
        <>
          <div className="flex flex-col justify-between gap-3 sm:flex-row sm:items-center">
            <div>
              <h2 className="text-xl font-semibold text-fg-default">
                {t("history.title")}
              </h2>
              <p className="text-xs text-fg-subtle">{t("history.scope")}</p>
            </div>
            <Select value={queue} onValueChange={changeQueue}>
              <SelectTrigger
                className="w-full sm:w-48"
                aria-label={t("history.queueFilter")}
              >
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                {(
                  [
                    "ALL",
                    "SWIFTPLAY",
                    "DRAFT_PICK",
                    "RANKED_SOLO",
                    "RANKED_FLEX",
                  ] as const
                ).map((value) => (
                  <SelectItem key={value} value={value}>
                    {t(`queue.${value}`)}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
          </div>

          {matches.length === 0 && !summonerQuery.isFetching && (
            <State text={t("history.empty")} />
          )}
          <div className="space-y-3">
            {matches.map((match) => (
              <SummonerMatchCard key={match.matchId} match={match} />
            ))}
          </div>
          {summonerQuery.isFetching && matches.length > 0 && (
            <Skeleton className="h-28 w-full" />
          )}
          {result.history.pageInfo.hasNextPage &&
            result.history.pageInfo.endCursor && (
              <div className="text-center">
                <Button
                  variant="secondary"
                  disabled={summonerQuery.isFetching}
                  onClick={() => setAfter(result.history.pageInfo.endCursor)}
                >
                  {summonerQuery.isFetching
                    ? t("history.loadingMore")
                    : t("history.loadMore")}
                </Button>
              </div>
            )}
          {!result.history.pageInfo.hasNextPage && matches.length > 0 && (
            <p className="text-center text-xs text-fg-subtle">
              {t("history.end")}
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
  mutationError,
  retrySeconds,
}: {
  job: LookupJob | null | undefined;
  notice: string;
  mutationError: unknown;
  retrySeconds: number;
}) {
  const { t } = useTranslation("summoner");
  const [now, setNow] = useState(Date.now());
  useEffect(() => {
    if (!job) return;
    const timer = window.setInterval(() => setNow(Date.now()), 10_000);
    return () => window.clearInterval(timer);
  }, [job]);
  if (job) {
    const progress = Math.min(100, job.scannedCount || 0);
    const idleFor = now - new Date(job.updatedAt).getTime();
    const stalled =
      (job.status === "QUEUED" && idleFor >= QUEUED_STALL_MS) ||
      (job.status === "RUNNING" && idleFor >= RUNNING_STALL_MS);
    return (
      <aside
        className={`rounded-lg border p-3 ${stalled ? "border-amber-500/50 bg-amber-500/10" : "border-border-accent bg-accent-subtle"}`}
        aria-live="polite"
      >
        <div className="flex justify-between gap-3 text-sm text-fg-default">
          <span>
            {stalled
              ? t(
                  job.status === "QUEUED"
                    ? "refresh.queuedStalled"
                    : "refresh.runningStalled",
                )
              : t(`stage.${job.stage}`)}
          </span>
          <span>
            {t("refresh.progress", {
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
  const requestError = asGraphQLRequestError(mutationError);
  let errorText = "";
  if (requestError?.code === "RATE_LIMITED") {
    const key =
      requestError.rateLimitScope === "IP"
        ? "refresh.rateLimitedIP"
        : requestError.rateLimitScope === "IDENTITY"
          ? "refresh.rateLimitedIdentity"
          : "refresh.rateLimited";
    errorText = t(key, { seconds: retrySeconds });
  } else if (requestError?.code === "NETWORK_ERROR") {
    errorText = t("refresh.networkError");
  } else if (requestError?.code === "SERVICE_UNAVAILABLE") {
    errorText = t("refresh.serviceUnavailable");
  } else if (mutationError) {
    errorText = t("refresh.failed");
  }
  return (
    <p
      className={`rounded-lg border p-3 text-sm ${mutationError ? "border-red-500/50 text-red-300" : "border-border text-fg-muted"}`}
      role={mutationError ? "alert" : "status"}
    >
      {errorText || notice}
    </p>
  );
}

function State({ text }: { text: string }) {
  return (
    <p className="rounded-lg border border-border bg-surface-raised py-12 text-center text-fg-muted">
      {text}
    </p>
  );
}
