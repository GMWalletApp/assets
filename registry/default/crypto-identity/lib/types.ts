export interface TokenAssetQuery {
  type: "token";
  /** Token symbol or asset name. Matching is case-insensitive. */
  name: string;
  /** Network name or common alias. Casing and separators are normalized internally. */
  network: string;
  /** Chain-specific token contract address. Matching is case-insensitive where applicable. */
  contractAddress?: string | null;
  /** @deprecated Assets are always resolved from the compressed catalog. */
  includeCatalog?: boolean;
}

interface CatalogAssetQuery<T extends "exchange" | "swap-provider" | "wallet"> {
  type: T;
  /** Catalog ID or display name. Casing and separators are normalized internally. */
  name: string;
  /** @deprecated Assets are always resolved from the compressed catalog. */
  includeCatalog?: boolean;
}

interface DappAssetQuery {
  type: "dapp";
  /** DApp ID or domain, with or without dots, separators, casing, or the PNG suffix. */
  name: string;
  /** @deprecated Assets are always resolved from the compressed catalog. */
  includeCatalog?: boolean;
}

interface NetworkAssetQuery {
  type: "network";
  /** Network name or common alias. Casing and separators are normalized internally. */
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
