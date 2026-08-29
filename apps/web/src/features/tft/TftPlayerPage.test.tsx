import "@shared/i18n";

import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { act, render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { MemoryRouter, Route, Routes } from "react-router-dom";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { i18n } from "@shared/i18n";

import { TftPlayerPage } from "./TftPlayerPage";

const apiMocks = vi.hoisted(() => ({
  history: vi.fn(),
  refresh: vi.fn(),
  job: vi.fn(),
}));

vi.mock("@shared/api", () => ({
  useTftMatchHistoryQuery: apiMocks.history,
  useRefreshTftPlayerMutation: apiMocks.refresh,
  useTftLookupJobQuery: apiMocks.job,
}));

interface RefreshPayload {
  refreshTFTPlayer: {
    fresh: boolean;
    reused: boolean;
    job: LookupJob | null;
  };
}

interface RefreshCallbacks {
  onMutate?: () => void;
  onSuccess?: (data: RefreshPayload) => void;
}

interface LookupJob {
  id: string;
  platform: string;
  gameName: string;
  tagLine: string;
  status: "QUEUED" | "RUNNING" | "COMPLETED" | "PARTIAL" | "FAILED";
  stage:
    | "QUEUED"
    | "RESOLVE_ACCOUNT"
    | "FETCH_MATCH_IDS"
    | "FETCH_MATCHES"
    | "FINALIZE";
  scannedCount: number;
  fetchedCount: number;
  failedCount: number;
  errorCode: string | null;
  createdAt: string;
  updatedAt: string;
  completedAt: string | null;
}

const NO_HISTORY = queryResult({ tftMatchHistory: null });
const NO_JOB = jobResult(null, false);

beforeEach(async () => {
  apiMocks.history.mockReset();
  apiMocks.refresh.mockReset();
  apiMocks.job.mockReset();
  await i18n.changeLanguage("en-US");
});

afterEach(() => {
  vi.restoreAllMocks();
});

describe("TftPlayerPage state machine", () => {
  it("automatically starts a refresh for an identity missing from local history", async () => {
    const mutate = vi.fn();
    apiMocks.history.mockReturnValue(NO_HISTORY);
    apiMocks.refresh.mockReturnValue(mutationResult(mutate));
    apiMocks.job.mockReturnValue(NO_JOB);

    renderPage();

    await waitFor(() => {
      expect(mutate).toHaveBeenCalledTimes(1);
    });
    expect(mutate).toHaveBeenCalledWith({
      identity: { platform: "KR", gameName: "Test Player", tagLine: "KR1" },
    });
  });

  it("polls a running job, then invalidates and re-reads history after completion", async () => {
    let activeHistory = NO_HISTORY;
    let activeJob: LookupJob = lookupJob("RUNNING", "FETCH_MATCHES");
    const refreshedHistory = queryResult({
      tftMatchHistory: historyPage(
        [match("fresh-match", "fresh result")],
        false,
        null,
      ),
    });
    const client = createClient();
    const invalidate = vi.spyOn(client, "invalidateQueries");

    apiMocks.history.mockImplementation(() => activeHistory);
    apiMocks.refresh.mockImplementation((rawCallbacks) => {
      const callbacks = rawCallbacks as RefreshCallbacks;
      return mutationResult((variables: unknown) => {
        callbacks.onMutate?.();
        callbacks.onSuccess?.({
          refreshTFTPlayer: { fresh: false, reused: false, job: activeJob },
        });
        return variables;
      });
    });
    apiMocks.job.mockImplementation((_variables, rawOptions) => {
      const options = rawOptions as { enabled?: boolean };
      return jobResult(
        options.enabled ? activeJob : null,
        Boolean(options.enabled),
      );
    });

    const view = renderPage(client);

    await waitFor(() => {
      expect(screen.getByText("Fetching match details")).toBeInTheDocument();
    });
    expect(
      apiMocks.job.mock.calls.some(
        ([, options]) =>
          (options as { refetchInterval?: number | false }).refetchInterval ===
          2_000,
      ),
    ).toBe(true);

    activeJob = lookupJob("COMPLETED", "FINALIZE");
    view.rerender(pageTree(client));

    await waitFor(() => {
      expect(invalidate).toHaveBeenCalledWith({
        queryKey: ["TFTMatchHistory"],
      });
    });

    activeHistory = refreshedHistory;
    view.rerender(pageTree(client));
    await waitFor(() => {
      expect(screen.getByText("fresh result")).toBeInTheDocument();
    });
    expect(
      apiMocks.history.mock.calls.some(([variables]) => {
        const input = (variables as { input: { after: string | null } }).input;
        return input.after === null;
      }),
    ).toBe(true);
  });

  it("merges cursor pages without retaining the duplicate boundary match", async () => {
    const firstPage = queryResult({
      tftMatchHistory: historyPage(
        [match("match-a", "result a"), match("match-b", "result b")],
        true,
        "cursor-1",
      ),
    });
    const secondPage = queryResult({
      tftMatchHistory: historyPage(
        [match("match-b", "result b"), match("match-c", "result c")],
        false,
        null,
      ),
    });
    apiMocks.history.mockImplementation((rawVariables) => {
      const variables = rawVariables as { input: { after: string | null } };
      return variables.input.after === "cursor-1" ? secondPage : firstPage;
    });
    apiMocks.refresh.mockReturnValue(mutationResult(vi.fn()));
    apiMocks.job.mockReturnValue(NO_JOB);

    renderPage();
    const user = userEvent.setup();
    await user.click(await screen.findByRole("button", { name: "Load more" }));

    await waitFor(() => {
      expect(screen.getByText("result c")).toBeInTheDocument();
    });
    expect(screen.getAllByText("result b")).toHaveLength(1);
    expect(screen.getByText("result a")).toBeInTheDocument();
    expect(screen.getByText("result c")).toBeInTheDocument();
  });

  it("drops prior pages and re-queries the first page when locale changes", async () => {
    const englishFirst = queryResult({
      tftMatchHistory: historyPage(
        [match("match-a", "English first")],
        true,
        "english-cursor",
      ),
    });
    const englishSecond = queryResult({
      tftMatchHistory: historyPage(
        [match("match-b", "English second")],
        false,
        null,
      ),
    });
    const chineseFirst = queryResult({
      tftMatchHistory: historyPage([match("match-a", "中文首页")], false, null),
    });
    apiMocks.history.mockImplementation((rawVariables) => {
      const variables = rawVariables as {
        input: { after: string | null; locale: string };
      };
      if (variables.input.locale === "zh_cn") return chineseFirst;
      return variables.input.after === "english-cursor"
        ? englishSecond
        : englishFirst;
    });
    apiMocks.refresh.mockReturnValue(mutationResult(vi.fn()));
    apiMocks.job.mockReturnValue(NO_JOB);

    renderPage();
    const user = userEvent.setup();
    await user.click(await screen.findByRole("button", { name: "Load more" }));
    expect(await screen.findByText("English second")).toBeInTheDocument();

    await act(async () => {
      await i18n.changeLanguage("zh-CN");
    });

    await waitFor(() => {
      expect(screen.getByText("中文首页")).toBeInTheDocument();
      expect(screen.queryByText("English second")).not.toBeInTheDocument();
      expect(screen.queryByText("English first")).not.toBeInTheDocument();
    });
    expect(
      apiMocks.history.mock.calls.some(([variables]) => {
        const input = (
          variables as { input: { after: string | null; locale: string } }
        ).input;
        return input.locale === "zh_cn" && input.after === null;
      }),
    ).toBe(true);
  });
});

function renderPage(client = createClient()) {
  return render(pageTree(client));
}

function pageTree(client: QueryClient) {
  return (
    <QueryClientProvider client={client}>
      <MemoryRouter initialEntries={["/tft/player/KR/Test%20Player/KR1"]}>
        <Routes>
          <Route
            path="/tft/player/:platform/:gameName/:tagLine"
            element={<TftPlayerPage />}
          />
        </Routes>
      </MemoryRouter>
    </QueryClientProvider>
  );
}

function createClient() {
  return new QueryClient({ defaultOptions: { queries: { retry: false } } });
}

function mutationResult(mutate: (variables: unknown) => unknown) {
  return {
    mutate,
    reset: vi.fn(),
    isPending: false,
    isError: false,
    error: null,
  };
}

function queryResult(data: unknown) {
  return {
    data,
    isPending: false,
    isError: false,
    isFetching: false,
    refetch: vi.fn(),
  };
}

function jobResult(job: LookupJob | null, isSuccess: boolean) {
  return {
    data: isSuccess ? { tftLookupJob: job } : undefined,
    isSuccess,
  };
}

function lookupJob(
  status: LookupJob["status"],
  stage: LookupJob["stage"],
): LookupJob {
  return {
    id: "job-1",
    platform: "KR",
    gameName: "Test Player",
    tagLine: "KR1",
    status,
    stage,
    scannedCount: 20,
    fetchedCount: status === "COMPLETED" ? 20 : 5,
    failedCount: 0,
    errorCode: null,
    createdAt: "2026-08-29T00:00:00Z",
    updatedAt: "2026-08-29T00:01:00Z",
    completedAt: status === "COMPLETED" ? "2026-08-29T00:02:00Z" : null,
  };
}

function historyPage(
  matches: ReturnType<typeof match>[],
  hasNextPage: boolean,
  endCursor: string | null,
) {
  return {
    profile: {
      platform: "KR",
      gameName: "Test Player",
      tagLine: "KR1",
      lastRefreshedAt: "2026-08-29T00:00:00Z",
      isStale: false,
    },
    matches,
    pageInfo: { endCursor, hasNextPage, returned: matches.length },
  };
}

function match(matchId: string, endOfGameResult: string) {
  return {
    matchId,
    platform: "KR",
    queueId: 1100,
    patch: "16.17",
    gameVersion: "16.17.1",
    gameDatetime: "2026-08-29T00:00:00Z",
    gameLengthSeconds: 1_800,
    setNumber: 15,
    tftGameType: "standard",
    endOfGameResult,
    participant: {
      puuid: "puuid-1",
      gameName: "Test Player",
      tagLine: "KR1",
      isCurrentPlayer: true,
      placement: 2,
      level: 9,
      goldLeft: 10,
      lastRound: 35,
      playersEliminated: 2,
      totalDamageToPlayers: 120,
      augments: [],
      traits: [],
      units: [],
    },
  };
}
