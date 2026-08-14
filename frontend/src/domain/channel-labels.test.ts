import { channelSceneLabel } from "./channel-labels.js";

assertEqual(channelSceneLabel("unknown"), "其他区域");
assertEqual(channelSceneLabel("machine_room"), "机房");
assertEqual(channelSceneLabel("front_desk"), "前台");
assertEqual(channelSceneLabel("treatment"), "治疗室");

console.log("channel-labels tests passed");

function assertEqual(actual: string, expected: string) {
  if (actual !== expected) {
    throw new Error(`expected ${expected}, got ${actual}`);
  }
}
