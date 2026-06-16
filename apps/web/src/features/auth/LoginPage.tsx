import { useTranslation } from "react-i18next";

import { PlaceholderPage } from "@shared/ui";

// Phase E wires this up to the Phase B chunk 6 OAuth endpoints
// (/oauth/start/discord, /oauth/start/google). RSO comes once Riot
// approves the production app.
export function LoginPage() {
  const { t } = useTranslation("common");
  return (
    <PlaceholderPage
      title={t("nav.login")}
      description={t("placeholder.login")}
    />
  );
}
