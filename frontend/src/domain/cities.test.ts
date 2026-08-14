import { CITY_OPTIONS } from "./cities.js";

assertIncludes(CITY_OPTIONS, "济南");
assertEqual(
  CITY_OPTIONS.join(","),
  [
    "北京",
    "长沙",
    "成都",
    "重庆",
    "东莞",
    "佛山",
    "广州",
    "杭州",
    "合肥",
    "济南",
    "昆明",
    "南京",
    "宁波",
    "青岛",
    "上海",
    "深圳",
    "苏州",
    "天津",
    "武汉",
    "西安",
    "郑州",
  ].join(","),
);

console.log("cities tests passed");

function assertIncludes(values: string[], expected: string) {
  if (!values.includes(expected)) {
    throw new Error(`expected city list to include ${expected}`);
  }
}

function assertEqual(actual: string, expected: string) {
  if (actual !== expected) {
    throw new Error(`expected ${expected}, got ${actual}`);
  }
}
