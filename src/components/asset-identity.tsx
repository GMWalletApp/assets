import { ImageOff } from "lucide-react";
import type { ComponentProps } from "react";
import { type AssetEntry, assetCornerIcon, assetIcon, identityBaseUrl } from "@/lib/assets";
import { CryptoIdentity } from "../../registry/default/crypto-identity";

interface AssetIdentityProps extends Omit<ComponentProps<typeof CryptoIdentity>, "icon"> {
  asset: AssetEntry;
}

export function AssetIdentity({ asset, ...props }: AssetIdentityProps) {
  return (
    <CryptoIdentity
      cornerIcon={assetCornerIcon(asset)}
      fallback={<ImageOff className="size-5 text-muted-foreground" />}
      icon={assetIcon(asset)}
      preferredBaseUrl={identityBaseUrl()}
      {...props}
    />
  );
}
