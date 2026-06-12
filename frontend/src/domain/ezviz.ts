import type { EzvizAccount } from "../api";

const REGION_NAMES = ["华北", "华东", "华南", "华西"] as const;

type RegionName = (typeof REGION_NAMES)[number];

export function selectableRegionAccounts(accounts: EzvizAccount[]): EzvizAccount[] {
  const selected = new Map<RegionName, EzvizAccount>();
  for (const account of accounts) {
    const region = accountRegion(account);
    if (region && !selected.has(region)) {
      selected.set(region, account);
    }
  }
  return REGION_NAMES.flatMap((region) => {
    const account = selected.get(region);
    return account ? [account] : [];
  });
}

export function displayAccountRegion(account: EzvizAccount): string {
  return accountRegion(account) ?? account.accountName;
}

function accountRegion(account: EzvizAccount): RegionName | "" {
  return REGION_NAMES.find((region) => account.accountName.includes(region)) ?? "";
}
