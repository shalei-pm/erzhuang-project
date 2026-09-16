# Digital twin kit provenance

Copied on 2026-09-09 from the user-owned local project
`/Users/sylar/Documents/ChatGPT/快医美数字孪生` (data kit v1).
Only runtime HTML/CSS/JS and type definitions were migrated. No source-project
files, backups, screenshots or git history were modified or imported.

The React host supplies authenticated institution and camera metadata. Demo
figures remain synthetic and are explicitly labelled. The source startup.js is
not run. An optional externalCameraDialog flag delegates camera presentation to
the existing Erzhuang player without changing the original standalone behavior.

Region snapshots support two consultation-segment measurements:

- `noConsultation`: number of customers who do not require consultation.
- `consultationRequired`: number of customers who require consultation.

The dashboard displays these measurements for reception, waiting and treatment.
Use `null` only when a value has never been obtained; the dashboard renders it
as `—` without a unit. A successful response containing `0` is a valid
measurement and renders as `0 人`.

Polling failures must not replace the last successful measurement with `null`.
Keep the existing value and mark only the affected fields:

- Add overview field names to `staleOverview`.
- Add region field names to each region's `staleFields`.
- Remove a field from the stale list after its next successful refresh.

Stale measurements keep their previous value and show a quiet amber status dot.
If there is no previous value, the dashboard keeps `—` and shows the same dot.

The fifth BI slot alternates every 7 seconds between `核销客单价` and
`人均服务点数`. Both charts keep the same three customer segments and may
also be selected manually with the two chart indicators.

The arrival chart is titled `到院人次` and uses `人次` as its unit. When the
upstream payload provides all three explicit paths (`visit_front_desk`,
`visit_no_consult`, and `visit_need_consult`), the chart renders a true stacked
bar for `前台签到 / 非面诊 / 面诊`. If any path is missing, that date renders
only the `全部顾客` total bar. Never derive front-desk visits by subtracting
the other paths from the total.
