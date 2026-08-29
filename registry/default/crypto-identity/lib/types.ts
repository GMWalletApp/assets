export interface TokenAssetQuery {
  type: "token";
  /** Token symbol or canonical asset name. */
  name: string;
  /** Canonical repository network key, such as `ethereum`, `smartchain`, or `tron`. */
  network: string;
  /** Chain-specific token contract address. */
  contractAddress?: string | null;
  /** @deprecated Assets are always resolved from the compressed catalog. */
  includeCatalog?: boolean;
}

interface CatalogAssetQuery<T extends "exchange" | "swap-provider" | "wallet"> {
  type: T;
  /** Catalog asset key or display name. */
  name: string;
  /** @deprecated Assets are always resolved from the compressed catalog. */
  includeCatalog?: boolean;
}

interface DappAssetQuery {
  type: "dapp";
  /** DApp catalog key or domain name, with or without the png extension. */
  name: string;
  /** @deprecated Assets are always resolved from the compressed catalog. */
  includeCatalog?: boolean;
}

interface NetworkAssetQuery {
  type: "network";
  /** Repository asset key without a file extension. */
  name: string;
}

export type AssetQuery =
  | TokenAssetQuery
  | CatalogAssetQuery<"exchange">
  | CatalogAssetQuery<"swap-provider">
  | CatalogAssetQuery<"wallet">
  | NetworkAssetQuery
  | DappAssetQuery;

export type AssetType = AssetQuery["type"];
type WithoutCatalog<T> = T extends unknown ? Omit<T, "includeCatalog"> : never;

export type CryptoIdentityIcon = WithoutCatalog<AssetQuery>;
