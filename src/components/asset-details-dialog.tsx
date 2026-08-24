import { Check, Copy, ExternalLink } from "lucide-react";
import { AssetIdentity } from "@/components/asset-identity";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { ScrollArea } from "@/components/ui/scroll-area";
import { Separator } from "@/components/ui/separator";
import { useCopyFeedback } from "@/hooks/use-copy-feedback";
import {
  type AssetEntry,
  assetBadge,
  assetLogoUrl,
  assetName,
  assetSecondary,
  assetSymbol,
} from "@/lib/assets";

export function AssetDetailsDialog({
  asset,
  onOpenChange,
}: {
  asset: AssetEntry | null;
  onOpenChange: (open: boolean) => void;
}) {
  if (!asset) {
    return null;
  }

  const rows = assetRows(asset);
  return (
    <Dialog open onOpenChange={onOpenChange}>
      <DialogContent className="gap-0 overflow-hidden p-0 sm:max-w-2xl">
        <DialogHeader className="flex-row items-center gap-4 border-b p-5 text-left">
          <AssetIdentity aria-hidden="true" asset={asset} className="size-14" />
          <div className="min-w-0">
            <DialogTitle className="truncate text-xl">{assetName(asset)}</DialogTitle>
            <DialogDescription className="mt-1 truncate">
              {assetSymbol(asset)} · {assetSecondary(asset)}
            </DialogDescription>
          </div>
        </DialogHeader>
        <ScrollArea className="max-h-[65vh]">
          <div className="space-y-5 p-5">
            <div className="flex flex-wrap gap-2">
              <Badge>{assetBadge(asset)}</Badge>
              {"token" in asset ? <Badge variant="outline">{asset.token.status}</Badge> : null}
              {"token" in asset && asset.token.hot ? <Badge variant="secondary">hot</Badge> : null}
            </div>
            <Separator />
            <div className="grid gap-3">
              {rows.map((row) => (
                <DetailRow key={row.label} {...row} />
              ))}
            </div>
          </div>
        </ScrollArea>
      </DialogContent>
    </Dialog>
  );
}

interface Detail {
  href?: string | undefined;
  label: string;
  value: string;
}

function assetRows(asset: AssetEntry): Detail[] {
  const currentLogo = assetLogoUrl(asset);
  if ("token" in asset) {
    return [
      { label: "type", value: asset.token.type },
      { label: "decimals", value: String(asset.token.decimals) },
      { label: "assetId", value: asset.token.assetId },
      { label: "address", value: asset.token.address || "-" },
      { label: "logoURI", value: asset.token.logoURI ?? "-", href: asset.token.logoURI },
      { label: "resolved URL", value: currentLogo ?? "-", href: currentLogo },
    ];
  }
  return [
    { label: "id", value: asset.item.id },
    { label: "name", value: asset.item.name },
    { label: "type", value: asset.item.type ?? asset.category },
    { label: "logoURI", value: asset.item.logoURI, href: asset.item.logoURI },
    { label: "resolved URL", value: currentLogo ?? "-", href: currentLogo },
  ];
}

function DetailRow({ href, label, value }: Detail) {
  const { copied, copy } = useCopyFeedback(value);

  return (
    <div className="grid gap-2 rounded-lg bg-muted/70 p-3 md:grid-cols-[7rem_minmax(0,1fr)_auto] md:items-start">
      <span className="text-xs font-semibold text-muted-foreground">{label}</span>
      {href ? (
        <a
          className="flex min-w-0 items-start gap-1 break-all font-mono text-xs text-primary hover:underline"
          href={href}
          rel="noreferrer"
          target="_blank"
        >
          {value}
          <ExternalLink className="mt-0.5 size-3 shrink-0" />
        </a>
      ) : (
        <span className="min-w-0 break-all font-mono text-xs">{value}</span>
      )}
      <Button aria-label={`复制 ${label}`} size="icon-xs" variant="ghost" onClick={copy}>
        {copied ? <Check /> : <Copy />}
      </Button>
    </div>
  );
}
