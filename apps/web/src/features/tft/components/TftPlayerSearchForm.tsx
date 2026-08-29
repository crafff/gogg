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

import { normalizeTftPlatform, TFT_PLATFORMS } from "../lib/platforms";
import { parseRiotID, tftPlayerPath } from "../lib/playerIdentity";

interface TftPlayerSearchFormProps {
  initialPlatform?: string;
  initialRiotID?: string;
  compact?: boolean;
}

const PLATFORM_STORAGE_KEY = "gogg:tft-platform";

export function TftPlayerSearchForm({
  initialPlatform,
  initialRiotID = "",
  compact = false,
}: TftPlayerSearchFormProps) {
  const { t } = useTranslation("tft");
  const navigate = useNavigate();
  const [platform, setPlatform] = useState(() =>
    preferredPlatform(initialPlatform),
  );
  const [riotID, setRiotID] = useState(initialRiotID);
  const [error, setError] = useState("");

  useEffect(() => {
    setPlatform(preferredPlatform(initialPlatform));
    setRiotID(initialRiotID);
    setError("");
  }, [initialPlatform, initialRiotID]);

  function submit(event: React.FormEvent<HTMLFormElement>) {
    event.preventDefault();
    const identity = parseRiotID(riotID);
    if (!identity) {
      setError(t("player.search.invalid"));
      return;
    }
    setError("");
    rememberPlatform(platform);
    navigate(tftPlayerPath(platform, identity.gameName, identity.tagLine));
  }

  function changePlatform(value: string) {
    const normalized = normalizeTftPlatform(value);
    setPlatform(normalized);
    rememberPlatform(normalized);
  }

  return (
    <form
      className={compact ? "space-y-1" : "mx-auto max-w-2xl space-y-2"}
      onSubmit={submit}
      aria-label={t("player.search.label")}
    >
      <div className="flex flex-col gap-2 sm:flex-row">
        <Select value={platform} onValueChange={changePlatform}>
          <SelectTrigger
            className="w-full sm:w-28"
            aria-label={t("player.search.platform")}
          >
            <SelectValue />
          </SelectTrigger>
          <SelectContent>
            {TFT_PLATFORMS.map((value) => (
              <SelectItem key={value} value={value}>
                {value}
              </SelectItem>
            ))}
          </SelectContent>
        </Select>
        <label className="sr-only" htmlFor="tft-player-riot-id">
          {t("player.search.riotID")}
        </label>
        <input
          id="tft-player-riot-id"
          value={riotID}
          onChange={(event) => setRiotID(event.target.value)}
          placeholder={t("player.search.placeholder")}
          autoComplete="off"
          className="h-9 min-w-0 flex-1 rounded border border-border-default bg-surface-raised px-3 text-sm text-fg-default placeholder:text-fg-subtle focus-visible:outline-none focus-visible:shadow-focus-ring"
        />
        <Button type="submit" className="shrink-0">
          {t("player.search.submit")}
        </Button>
      </div>
      {error && (
        <p className="text-sm text-danger" role="alert">
          {error}
        </p>
      )}
    </form>
  );
}

function preferredPlatform(initialPlatform?: string) {
  if (initialPlatform) return normalizeTftPlatform(initialPlatform);
  try {
    return normalizeTftPlatform(
      window.localStorage.getItem(PLATFORM_STORAGE_KEY),
    );
  } catch {
    return "KR";
  }
}

function rememberPlatform(platform: string) {
  try {
    window.localStorage.setItem(
      PLATFORM_STORAGE_KEY,
      normalizeTftPlatform(platform),
    );
  } catch {
    // Search remains usable when browser storage is unavailable.
  }
}
