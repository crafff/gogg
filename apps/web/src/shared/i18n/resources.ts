import zhCNCommon from "./locales/zh-CN/common.json";
import zhCNRankings from "./locales/zh-CN/rankings.json";
import enUSCommon from "./locales/en-US/common.json";
import enUSRankings from "./locales/en-US/rankings.json";

export const SUPPORTED_LANGUAGES = ["zh-CN", "en-US"] as const;
export type SupportedLanguage = (typeof SUPPORTED_LANGUAGES)[number];

export const DEFAULT_LANGUAGE: SupportedLanguage = "zh-CN";

export const NAMESPACES = ["common", "rankings"] as const;
export type Namespace = (typeof NAMESPACES)[number];

export const resources = {
  "zh-CN": {
    common: zhCNCommon,
    rankings: zhCNRankings,
  },
  "en-US": {
    common: enUSCommon,
    rankings: enUSRankings,
  },
} as const;
