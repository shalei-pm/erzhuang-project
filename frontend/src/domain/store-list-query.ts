export type StoreListQueryInput = {
  query: string;
  cityFilter: string;
  page: number;
  pageSize: number;
};

export function normalizeCityFilter(cityFilter: string) {
  const city = cityFilter.trim();
  return city === "all" ? "" : city;
}

export function storeListSearchParams(input: StoreListQueryInput) {
  const search = new URLSearchParams({
    q: input.query.trim(),
    page: String(input.page),
    page_size: String(input.pageSize),
  });
  const city = normalizeCityFilter(input.cityFilter);
  if (city) {
    search.set("city", city);
  }
  return search;
}
