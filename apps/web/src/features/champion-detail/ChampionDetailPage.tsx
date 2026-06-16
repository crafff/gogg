import { useTranslation } from "react-i18next";
import { useParams } from "react-router-dom";

import { PlaceholderPage } from "@shared/ui";

// Phase E adds the real champion detail page (mv_champion_detail
// aggregations: core items, runes, matchups, build paths). For now
// chunk 5 only proves the route + URL param plumbing works.
export function ChampionDetailPage() {
  const { t } = useTranslation("common");
  const { championId } = useParams<{ championId: string }>();

  return (
    <PlaceholderPage
      title={t("nav.champion")}
      description={t("placeholder.championDetail")}
      context={championId ? `championId = ${championId}` : undefined}
    />
  );
}
