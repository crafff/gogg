import i18n from "i18next";
import LanguageDetector from "i18next-browser-languagedetector";
import { initReactI18next } from "react-i18next";

import {
  DEFAULT_LANGUAGE,
  NAMESPACES,
  SUPPORTED_LANGUAGES,
  resources,
} from "./resources";

// Single-shot init: chunk 2 ships all bundles eagerly because the
// payload is tiny (~2 kB total). Once chunk 4 adds champion/summoner
// namespaces we'll split to `i18next-http-backend` + lazy loading.
if (!i18n.isInitialized) {
  void i18n
    .use(LanguageDetector)
    .use(initReactI18next)
    .init({
      resources,
      ns: [...NAMESPACES],
      defaultNS: "common",
      fallbackLng: DEFAULT_LANGUAGE,
      supportedLngs: [...SUPPORTED_LANGUAGES],
      load: "currentOnly",
      interpolation: { escapeValue: false },
      detection: {
        order: ["localStorage", "navigator", "htmlTag"],
        caches: ["localStorage"],
        lookupLocalStorage: "gogg.lang",
      },
      returnNull: false,
    });
}

export { i18n };
export { SUPPORTED_LANGUAGES, DEFAULT_LANGUAGE, NAMESPACES } from "./resources";
export type { SupportedLanguage, Namespace } from "./resources";
