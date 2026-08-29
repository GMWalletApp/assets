import { useTranslation } from "react-i18next";
import { AssetIdentity } from "@/components/asset-identity";
import { Badge } from "@/components/ui/badge";
import { type AssetEntry, assetBadge, assetName, assetSecondary, assetSymbol } from "@/lib/assets";

export function AssetCard({ asset, onSelect }: { asset: AssetEntry; onSelect: () => void }) {
  const { t } = useTranslation();
  const badge = assetBadge(asset);
  return (
    <button
      aria-label={`${t("actions.view")} ${assetName(asset)}`}
      className="group flex min-h-36 w-full flex-col items-center rounded-xl border border-border bg-card p-3 text-center shadow-xs transition-[transform,box-shadow,border-color] duration-200 hover:-translate-y-0.5 hover:border-primary/30 hover:shadow-[0_12px_28px_-16px_color-mix(in_oklab,var(--primary)_38%,transparent)] focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2 focus-visible:ring-offset-background sm:min-h-40 sm:p-4"
      type="button"
      onClick={onSelect}
    >
      <AssetIdentity
        aria-hidden="true"
        asset={asset}
        className="size-12 transition-transform duration-200 group-hover:scale-105 sm:size-14"
      />
      <span className="mt-3 block w-full truncate text-sm font-semibold">{assetSymbol(asset)}</span>
      <span className="mt-0.5 block w-full truncate text-xs text-muted-foreground">
        {assetName(asset)}
      </span>
      <Badge className="mt-auto max-w-full font-mono text-[10px]" variant="secondary">
        <span className="truncate">
          {assetSecondary(asset) || t(`assetBadges.${badge}`, { defaultValue: badge })}
        </span>
      </Badge>
    </button>
  );
}
