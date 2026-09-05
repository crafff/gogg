import { useEffect, useState } from "react";
import { useTranslation } from "react-i18next";
import { useNavigate } from "react-router-dom";

import {
  Button,
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@shared/ui";

interface SummonerSearchFormProps {
  initialRegion?: string;
  initialRiotID?: string;
  compact?: boolean;
}

const REGION_STORAGE_KEY = "gogg:summoner-region";
const RECENT_SUMMONERS_STORAGE_KEY = "gogg:recent-summoners";
const MAX_RECENT_SUMMONERS = 5;

interface RecentSummoner {
  region: "KR" | "NA1";
  gameName: string;
  tagLine: string;
}

export function SummonerSearchForm({
  initialRegion,
  initialRiotID = "",
  compact = false,
}: SummonerSearchFormProps) {
  const { t } = useTranslation("summoner");
  const navigate = useNavigate();
  const [region, setRegion] = useState(() => preferredRegion(initialRegion));
  const [riotID, setRiotID] = useState(initialRiotID);
  const [error, setError] = useState("");
  const [recentSummoners, setRecentSummoners] = useState(loadRecentSummoners);

  useEffect(() => {
    setRegion(preferredRegion(initialRegion));
    setRiotID(initialRiotID);
    setError("");
  }, [initialRegion, initialRiotID]);

  function changeRegion(value: string) {
    setRegion(value);
    rememberRegion(value);
  }

  function submit(event: React.FormEvent<HTMLFormElement>) {
    event.preventDefault();
    const value = riotID.trim();
    const separator = value.lastIndexOf("#");
    const gameName = value.slice(0, separator).trim();
    const tagLine = value.slice(separator + 1).trim();
    if (separator <= 0 || !gameName || !tagLine) {
      setError(t("search.invalid"));
      return;
    }
    setError("");
    openSummoner({ region: normalizeRegion(region), gameName, tagLine });
  }

  function openSummoner(summoner: RecentSummoner) {
    setRegion(summoner.region);
    setRiotID(`${summoner.gameName}#${summoner.tagLine}`);
    setError("");
    rememberRegion(summoner.region);
    setRecentSummoners(rememberRecentSummoner(summoner));
    navigate(summonerPath(summoner));
  }

  function clearRecentSummoners() {
    forgetRecentSummoners();
    setRecentSummoners([]);
  }

  return (
    <form
      className={compact ? "space-y-1" : "mx-auto max-w-2xl space-y-2"}
      onSubmit={submit}
      aria-label={t("search.label")}
    >
      <div className="flex flex-col gap-2 sm:flex-row">
        <Select value={region} onValueChange={changeRegion}>
          <SelectTrigger
            className="w-full sm:w-28"
            aria-label={t("search.region")}
          >
            <SelectValue />
          </SelectTrigger>
          <SelectContent>
            <SelectItem value="KR">KR</SelectItem>
            <SelectItem value="NA1">NA1</SelectItem>
          </SelectContent>
        </Select>
        <label className="sr-only" htmlFor="summoner-riot-id">
          {t("search.riotID")}
        </label>
        <input
          id="summoner-riot-id"
          value={riotID}
          onChange={(event) => setRiotID(event.target.value)}
          placeholder={t("search.placeholder")}
          autoComplete="off"
          className="h-9 min-w-0 flex-1 rounded border border-border-default bg-surface-raised px-3 text-sm text-fg-default placeholder:text-fg-subtle focus-visible:outline-none focus-visible:shadow-focus-ring"
        />
        <Button type="submit" className="shrink-0">
          {t("search.submit")}
        </Button>
      </div>
      {error && (
        <p className="text-sm text-red-400" role="alert">
          {error}
        </p>
      )}
      {recentSummoners.length > 0 && (
        <div className="flex items-center gap-2 overflow-x-auto pt-1 text-xs">
          <span className="shrink-0 text-fg-subtle">{t("search.recent")}</span>
          <ul
            className="flex min-w-0 items-center gap-1"
            aria-label={t("search.recent")}
          >
            {recentSummoners.map((summoner) => (
              <li key={recentSummonerKey(summoner)}>
                <button
                  type="button"
                  onClick={() => openSummoner(summoner)}
                  className="whitespace-nowrap rounded-full border border-border-default bg-surface-raised px-2 py-1 text-fg-muted transition-colors hover:border-border-strong hover:bg-surface-overlay hover:text-fg-default focus-visible:outline-none focus-visible:shadow-focus-ring"
                >
                  <span className="text-fg-subtle">{summoner.region}</span>{" "}
                  {summoner.gameName}#{summoner.tagLine}
                </button>
              </li>
            ))}
          </ul>
          <button
            type="button"
            onClick={clearRecentSummoners}
            className="ml-auto shrink-0 whitespace-nowrap text-fg-subtle hover:text-fg-default focus-visible:outline-none focus-visible:underline"
          >
            {t("search.clearRecent")}
          </button>
        </div>
      )}
    </form>
  );
}

function preferredRegion(initialRegion?: string): string {
  if (initialRegion) return initialRegion.toUpperCase();
  try {
    const stored = window.localStorage
      .getItem(REGION_STORAGE_KEY)
      ?.toUpperCase();
    if (stored === "KR" || stored === "NA1") return stored;
  } catch {
    // Storage can be disabled by browser privacy settings.
  }
  return "KR";
}

function rememberRegion(region: string) {
  try {
    window.localStorage.setItem(REGION_STORAGE_KEY, region);
  } catch {
    // Searching still works when browser storage is unavailable.
  }
}

function normalizeRegion(region: string): RecentSummoner["region"] {
  return region.toUpperCase() === "NA1" ? "NA1" : "KR";
}

function summonerPath(summoner: RecentSummoner) {
  return `/summoner/${summoner.region}/${encodeURIComponent(summoner.gameName)}/${encodeURIComponent(summoner.tagLine)}`;
}

function recentSummonerKey(summoner: RecentSummoner) {
  return `${summoner.region}/${summoner.gameName.toLowerCase()}#${summoner.tagLine.toUpperCase()}`;
}

function loadRecentSummoners(): RecentSummoner[] {
  try {
    const stored: unknown = JSON.parse(
      window.localStorage.getItem(RECENT_SUMMONERS_STORAGE_KEY) ?? "[]",
    );
    if (!Array.isArray(stored)) return [];
    return stored
      .filter(isRecentSummoner)
      .slice(0, MAX_RECENT_SUMMONERS)
      .map((summoner) => ({
        ...summoner,
        region: normalizeRegion(summoner.region),
      }));
  } catch {
    return [];
  }
}

function isRecentSummoner(value: unknown): value is RecentSummoner {
  if (!value || typeof value !== "object") return false;
  const candidate = value as Partial<RecentSummoner>;
  return (
    (candidate.region === "KR" || candidate.region === "NA1") &&
    typeof candidate.gameName === "string" &&
    candidate.gameName.trim().length > 0 &&
    typeof candidate.tagLine === "string" &&
    candidate.tagLine.trim().length > 0
  );
}

function rememberRecentSummoner(summoner: RecentSummoner): RecentSummoner[] {
  const key = recentSummonerKey(summoner);
  const recent = [
    summoner,
    ...loadRecentSummoners().filter(
      (candidate) => recentSummonerKey(candidate) !== key,
    ),
  ].slice(0, MAX_RECENT_SUMMONERS);
  try {
    window.localStorage.setItem(
      RECENT_SUMMONERS_STORAGE_KEY,
      JSON.stringify(recent),
    );
  } catch {
    // The in-memory list still works for this page when storage is unavailable.
  }
  return recent;
}

function forgetRecentSummoners() {
  try {
    window.localStorage.removeItem(RECENT_SUMMONERS_STORAGE_KEY);
  } catch {
    // Clearing the in-memory list is still useful when storage is unavailable.
  }
}
