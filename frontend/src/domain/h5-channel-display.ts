import type { H5MonitorChannel } from "./h5-types";
import { channelMappingTargetLabel } from "./channel-mapping-target";

const businessAreaLabels: Record<string, string> = {
  consultation: "面诊室",
  treatment: "治疗室",
  vip_treatment: "VIP治疗室",
  beauty: "美容室",
};

const sceneLabels: Record<string, string> = {
  front_desk: "前台",
  waiting_area: "候诊区",
  corridor: "走廊",
  passage: "通道",
  hall: "大厅",
  entrance: "门口",
  storage: "库房",
  pharmacy: "药房",
  machine_room: "机房",
};

export interface H5ChannelDisplayText {
  title: string;
  subtitle: string;
}

export interface H5VisibleRowsInput {
  containerWidth: number;
  viewportWidth: number;
  visibleCount: number;
  totalCount: number;
}

const desktopMinColumnWidth = 160;
const desktopColumnGap = 22;
const mobileBreakpoint = 640;
const mobileColumnCount = 3;
const initialVisibleRows = 4;
const loadMoreRows = 2;

export function h5ChannelDisplayText(channel: H5MonitorChannel): H5ChannelDisplayText {
  return {
    title: h5ChannelTitle(channel),
    subtitle: `通道${channel.channel_no}`,
  };
}

export function h5InitialVisibleCount(containerWidth: number, viewportWidth: number): number {
  return h5CameraColumnCount(containerWidth, viewportWidth) * initialVisibleRows;
}

export function h5NextVisibleCount(input: H5VisibleRowsInput): number {
  const columns = h5CameraColumnCount(input.containerWidth, input.viewportWidth);
  const nextCount = input.visibleCount + columns * loadMoreRows;
  return Math.min(input.totalCount, nextCount);
}

export function h5CameraColumnCount(containerWidth: number, viewportWidth: number): number {
  if (viewportWidth <= mobileBreakpoint) return mobileColumnCount;
  if (containerWidth <= 0) return mobileColumnCount;
  return Math.max(1, Math.floor((containerWidth + desktopColumnGap) / (desktopMinColumnWidth + desktopColumnGap)));
}

export function h5ChannelTitle(channel: H5MonitorChannel): string {
  const businessLabel = businessAreaLabels[channel.area_type];
  if (businessLabel) {
    const label = channelMappingTargetLabel(
      {
        areaType: channel.area_type,
        areaNumber: channel.area_number,
        bedLabel: channel.bed_label,
        areaNote: channel.area_note,
      },
      { includeSingleBedSuffix: true },
    );
    return label || businessLabel;
  }

  const note = channel.area_note.trim();
  if (note) return note;

  const sceneLabel = sceneLabels[channel.scene_type];
  if (sceneLabel) return sceneLabel;

  const name = channel.channel_name.trim();
  return name || `通道${channel.channel_no}`;
}
