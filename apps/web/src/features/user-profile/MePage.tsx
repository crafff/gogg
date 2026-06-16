import { useTranslation } from "react-i18next";

import { PlaceholderPage } from "@shared/ui";

// Phase E: bound summoner, favorites, OAuth identities. Currently a
// placeholder that lights up once /me's JWT-protected query lands.
export function MePage() {
  const { t } = useTranslation("common");
  return (
    <PlaceholderPage title={t("nav.me")} description={t("placeholder.me")} />
  );
}
