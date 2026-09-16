import { expect, it } from "vitest";
import { t1BIPatch } from "./DigitalTwin";

it("maps optional visit segments into the digital twin chart", () => {
  const patch = t1BIPatch({
    tenant_id: 10001,
    begin_day: "2026-08-17",
    end_day: "2026-09-15",
    fetched_at: "2026-09-16T01:00:00Z",
    trends: [{
      date: "2026-09-15",
      visit_all: 55,
      visit_front_desk: 3,
      visit_no_consult: 34,
      visit_need_consult: 18,
      stay_all: 55,
      wait_all: 8,
      upgrade_all: 12,
      redemption_all: 680,
      service_point_all: 2.4,
    }],
  });

  expect(patch.trends?.[0]).toMatchObject({ visitAll: 55, frontDesk: 3, noConsult: 34, consult: 18 });
});
