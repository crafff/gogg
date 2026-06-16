import { useTranslation } from "react-i18next";

import { SUPPORTED_LANGUAGES, type SupportedLanguage } from "./resources";

// Minimal switcher in chunk 2 — the styled Radix-based variant lands
// alongside the global header in chunk 5. Kept native <select> here so
// it has zero component dependencies and the i18n round-trip can be
// verified independently of the design system.
export function LanguageSwitcher() {
  const { i18n, t } = useTranslation("common");
  const current = (SUPPORTED_LANGUAGES as readonly string[]).includes(
    i18n.resolvedLanguage ?? "",
  )
    ? (i18n.resolvedLanguage as SupportedLanguage)
    : "zh-CN";

  return (
    <label className="inline-flex items-center gap-2 text-xs text-fg-subtle">
      <span>{t("language.label")}</span>
      <select
        aria-label={t("language.label")}
        className="rounded border border-border-default bg-surface-raised px-2 py-1 text-xs text-fg-default"
        value={current}
        onChange={(e) => {
          void i18n.changeLanguage(e.target.value);
        }}
      >
        {SUPPORTED_LANGUAGES.map((lang) => (
          <option key={lang} value={lang}>
            {t(`language.${lang}` as const)}
          </option>
        ))}
      </select>
    </label>
  );
}
