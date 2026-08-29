import zhCNCommon from "./locales/zh-CN/common.json";
import zhCNRankings from "./locales/zh-CN/rankings.json";
import zhCNChampionDetail from "./locales/zh-CN/championDetail.json";
import zhCNSummoner from "./locales/zh-CN/summoner.json";
import zhCNTft from "./locales/zh-CN/tft.json";
import enUSCommon from "./locales/en-US/common.json";
import enUSRankings from "./locales/en-US/rankings.json";
import enUSChampionDetail from "./locales/en-US/championDetail.json";
import enUSSummoner from "./locales/en-US/summoner.json";
import enUSTft from "./locales/en-US/tft.json";

export const SUPPORTED_LANGUAGES = ["zh-CN", "en-US"] as const;
export type SupportedLanguage = (typeof SUPPORTED_LANGUAGES)[number];

export const DEFAULT_LANGUAGE: SupportedLanguage = "zh-CN";

export const NAMESPACES = [
  "common",
  "rankings",
  "championDetail",
  "summoner",
  "tft",
] as const;
export type Namespace = (typeof NAMESPACES)[number];

export const resources = {
  "zh-CN": {
    common: zhCNCommon,
    rankings: zhCNRankings,
    championDetail: zhCNChampionDetail,
    summoner: zhCNSummoner,
    tft: zhCNTft,
  },
  "en-US": {
    common: enUSCommon,
    rankings: enUSRankings,
    championDetail: enUSChampionDetail,
    summoner: enUSSummoner,
    tft: enUSTft,
  },
} as const;
