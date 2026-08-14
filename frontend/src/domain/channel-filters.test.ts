import type { VideoChannel } from "../api.js";
import { filterAndSortChannels } from "./channel-filters.js";
import { describe, expect, test } from "vitest";

const baseChannel: VideoChannel = {
  id: 1,
  recorderId: 1,
  recorderCode: "REC-1",
  channelNo: 1,
  channelName: "",
  status: "pending_confirmation",
  thumbnailUrl: "",
  fullImageUrl: "",
  sceneType: "unknown",
  areaType: "",
  areaNumber: "",
  bedLabel: "",
  areaNote: "",
  recognitionAttempts: 0,
};

function channel(patch: Partial<VideoChannel>): VideoChannel {
  return { ...baseChannel, ...patch };
}

describe("channel list filters", () => {
  test("全部按通道号排序", () => {
    expect(
      ids(
        filterAndSortChannels(
          [
            channel({ id: 1, channelNo: 8, areaType: "treatment", areaNumber: "10" }),
            channel({ id: 2, channelNo: 2, areaType: "consultation", areaNumber: "1" }),
            channel({ id: 3, channelNo: 3, areaType: "", areaNote: "前台" }),
          ],
          "all",
        ),
      ),
    ).toEqual([2, 3, 1]);
  });

  test("治疗室包含 VIP治疗室，并按数字排序，无数字排最后", () => {
    expect(
      ids(
        filterAndSortChannels(
          [
            channel({ id: 1, channelNo: 1, areaType: "treatment", areaNumber: "10" }),
            channel({ id: 2, channelNo: 2, areaType: "vip_treatment", areaNumber: "" }),
            channel({ id: 3, channelNo: 3, areaType: "treatment", areaNumber: "2" }),
            channel({ id: 4, channelNo: 4, areaType: "consultation", areaNumber: "1" }),
          ],
          "treatment",
        ),
      ),
    ).toEqual([3, 1, 2]);
  });

  test("前台/候诊区只匹配前台、候诊、等候文本", () => {
    expect(
      ids(
        filterAndSortChannels(
          [
            channel({ id: 1, channelNo: 1, areaType: "", areaNote: "过道" }),
            channel({ id: 2, channelNo: 2, areaType: "", areaNote: "候诊区" }),
            channel({ id: 3, channelNo: 3, areaType: "", areaNote: "等候区" }),
            channel({ id: 4, channelNo: 4, areaType: "", areaNote: "前台" }),
            channel({ id: 5, channelNo: 5, areaType: "", areaNote: "药房" }),
          ],
          "front_waiting",
        ),
      ),
    ).toEqual([2, 3, 4]);
  });

  test("通道/其他是非业务且非前台候诊的兜底组，并按文字排序", () => {
    expect(
      ids(
        filterAndSortChannels(
          [
            channel({ id: 1, channelNo: 4, areaType: "", areaNote: "药房" }),
            channel({ id: 2, channelNo: 2, areaType: "", areaNote: "过道" }),
            channel({ id: 3, channelNo: 3, areaType: "beauty", areaNumber: "1" }),
            channel({ id: 4, channelNo: 1, areaType: "", areaNote: "办公室" }),
            channel({ id: 5, channelNo: 5, areaType: "", areaNote: "前台" }),
          ],
          "passage_other",
        ),
      ),
    ).toEqual([4, 2, 1]);
  });
});

function ids(channels: VideoChannel[]) {
  return channels.map((item) => item.id);
}
