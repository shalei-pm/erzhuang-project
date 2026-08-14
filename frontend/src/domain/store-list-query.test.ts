import { normalizeCityFilter, storeListSearchParams } from "./store-list-query.js";

assertEqual(normalizeCityFilter("all"), "");
assertEqual(normalizeCityFilter(" 上海 "), "上海");

const allParams = storeListSearchParams({ query: " 正大 ", cityFilter: "all", page: 2, pageSize: 20 });
assertEqual(allParams.toString(), "q=%E6%AD%A3%E5%A4%A7&page=2&page_size=20");

const cityParams = storeListSearchParams({ query: "", cityFilter: "上海", page: 1, pageSize: 20 });
assertEqual(cityParams.toString(), "q=&page=1&page_size=20&city=%E4%B8%8A%E6%B5%B7");

console.log("store-list-query tests passed");

function assertEqual(actual: string, expected: string) {
  if (actual !== expected) {
    throw new Error(`expected ${expected}, got ${actual}`);
  }
}
