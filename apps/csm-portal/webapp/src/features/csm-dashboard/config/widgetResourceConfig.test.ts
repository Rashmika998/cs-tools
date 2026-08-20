// Copyright (c) 2026 WSO2 LLC. (https://www.wso2.com).
//
// WSO2 LLC. licenses this file to you under the Apache License,
// Version 2.0 (the "License"); you may not use this file except
// in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing,
// software distributed under the License is distributed on an
// "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
// KIND, either express or implied.  See the License for the
// specific language governing permissions and limitations
// under the License.

/**
 * Regression coverage for the empty-array filter guard: a DSL entry with
 * `values: []` must leave the corresponding `CasesFilters` field unset
 * rather than setting an explicit empty filter (which the cases list would
 * treat as "match nothing" instead of "no constraint") — see the
 * CodeRabbit finding this closes.
 */

import { describe, expect, it } from "vitest";
import { WIDGET_RESOURCE_CONFIG } from "@features/csm-dashboard/config/widgetResourceConfig";
import { readCasesFiltersFromUrl } from "@features/csm-cases/utils/casesFiltersUrl";

function hrefParams(href: string): URLSearchParams {
  const [, qs] = href.split("?");
  return new URLSearchParams(qs ?? "");
}

describe("WIDGET_RESOURCE_CONFIG.case.buildHref", () => {
  it("omits states/severities/types/products from the href when the DSL entry's values are empty", () => {
    const href = WIDGET_RESOURCE_CONFIG.case.buildHref({
      filters: [
        { field: "state", op: "in", values: [] },
        { field: "severity", op: "in", values: [] },
        { field: "type", op: "in", values: [] },
        { field: "product", op: "in", values: [] },
      ],
    });

    const params = hrefParams(href);
    expect(params.has("states")).toBe(false);
    expect(params.has("severities")).toBe(false);
    expect(params.has("types")).toBe(false);
    expect(params.has("products")).toBe(false);
  });

  it("still sets each field when the DSL entry carries real values", () => {
    const href = WIDGET_RESOURCE_CONFIG.case.buildHref({
      filters: [
        { field: "state", op: "in", values: ["open"] },
        { field: "severity", op: "in", values: ["critical"] },
        { field: "type", op: "in", values: ["case"] },
        { field: "product", op: "in", values: ["API Manager"] },
      ],
    });

    const params = hrefParams(href);
    expect(params.get("states")).toBe("open");
    expect(params.get("types")).toBe("case");
    expect(params.get("products")).toBe("API Manager");
    // Severity is remapped from the dashboard label to the case-list's own
    // S-code, so just assert it was set at all (severity-mapping specifics
    // aren't this fix's concern).
    expect(params.has("severities")).toBe(true);
  });

  it("carries engagementType and workState through to the cases list (previously dropped)", () => {
    // Regression: a case widget filtering by engagementType (e.g. "Engagements
    // In Progress") clicked through to an unfiltered cases list, because this
    // mapping didn't exist at all -- not a translation bug, a missing one.
    const href = WIDGET_RESOURCE_CONFIG.case.buildHref({
      filters: [
        { field: "engagementType", op: "in", values: ["migration", "onboarding"] },
        { field: "workState", op: "in", values: ["paused"] },
      ],
    });

    const params = hrefParams(href);
    expect(params.get("engagementTypes")).toBe("migration,onboarding");
    expect(params.get("workStates")).toBe("paused");
  });

  it("omits engagementTypes/workStates when the DSL entry's values are empty", () => {
    const href = WIDGET_RESOURCE_CONFIG.case.buildHref({
      filters: [
        { field: "engagementType", op: "in", values: [] },
        { field: "workState", op: "in", values: [] },
      ],
    });

    const params = hrefParams(href);
    expect(params.has("engagementTypes")).toBe(false);
    expect(params.has("workStates")).toBe(false);
  });
});

/**
 * Regression: the motivating bug for this whole feature. A widget filtering
 * `creTeam in [<team>]` + `tag notIn [s_dip]` + `state in [...]`
 * clicked through to `/cases?states=...` with the team and tag conditions
 * silently dropped — a tile reading 2 landed on a list of 30 (the org-wide
 * figure). Confirmed live three times before this fix. This suite proves the
 * full round trip end to end: `translateCaseDashboardFilters` ->
 * `casesHref` -> `readCasesFiltersFromUrl` — not just that the href contains
 * the right substring.
 */
describe("WIDGET_RESOURCE_CONFIG.case — previously-dropped fields", () => {
  it("carries creTeam, tag notIn, projectOnboardingStatus, escalation, escalationLevel, projectType, and SLA%/date ranges through to the href", () => {
    const href = WIDGET_RESOURCE_CONFIG.case.buildHref({
      filters: [
        { field: "creTeam", op: "in", values: ["team-abt"] },
        { field: "tag", op: "notIn", values: ["s_dip"] },
        { field: "projectOnboardingStatus", op: "in", values: ["in_progress"] },
        { field: "escalation", op: "isNotEmpty" },
        { field: "escalationLevel", op: "in", values: ["L1"] },
        { field: "projectType", op: "in", values: ["enterprise"] },
        { field: "taskSLABusinessElapsedPercent", op: "gte", values: ["80"] },
        { field: "createdOn", op: "gte", values: ["2026-01-01"] },
      ],
    });
    const parsed = readCasesFiltersFromUrl(hrefParams(href));

    expect(parsed.csTeams).toEqual(["team-abt"]);
    expect(parsed.excludeTags).toEqual(["s_dip"]);
    expect(parsed.tags).toEqual([]); // must NOT be inverted into an inclusion
    expect(parsed.onboardingStatuses).toEqual(["in_progress"]);
    expect(parsed.hasEscalation).toBe(true);
    expect(parsed.escalationLevels).toEqual(["L1"]);
    expect(parsed.projectTypes).toEqual(["enterprise"]);
    expect(parsed.slaElapsedPctGte).toBe(80);
    expect(parsed.createdOnGte).toBe("2026-01-01");
  });

  it("carries sreTeam through to the href the same way creTeam does (CodeRabbit #3801153841/#3801153843)", () => {
    const href = WIDGET_RESOURCE_CONFIG.case.buildHref({
      filters: [{ field: "sreTeam", op: "in", values: ["team-sre-abt"] }],
    });
    const parsed = readCasesFiltersFromUrl(hrefParams(href));

    expect(parsed.sreTeams).toEqual(["team-sre-abt"]);
  });

  it("creTeam and sreTeam survive together on the same widget, independently", () => {
    const href = WIDGET_RESOURCE_CONFIG.case.buildHref({
      filters: [
        { field: "creTeam", op: "in", values: ["team-abt"] },
        { field: "sreTeam", op: "in", values: ["team-sre-abt"] },
      ],
    });
    const parsed = readCasesFiltersFromUrl(hrefParams(href));

    expect(parsed.csTeams).toEqual(["team-abt"]);
    expect(parsed.sreTeams).toEqual(["team-sre-abt"]);
  });

  it("the org-wide-figure regression: team + tag-exclusion + state survive together, unchanged, end to end", () => {
    const href = WIDGET_RESOURCE_CONFIG.case.buildHref({
      filters: [
        { field: "creTeam", op: "in", values: ["team-abt"] },
        { field: "tag", op: "notIn", values: ["s_dip"] },
        { field: "state", op: "in", values: ["open", "work_in_progress"] },
      ],
    });
    const parsed = readCasesFiltersFromUrl(hrefParams(href));

    expect(parsed.csTeams).toEqual(["team-abt"]);
    expect(parsed.excludeTags).toEqual(["s_dip"]);
    expect(parsed.states).toEqual(["open", "work_in_progress"]);
  });

  // Regression: reported live against `abt_overall_open_incident`'s
  // `projectOnboardingStatus notIn ["In-Progress"]` -- the click-through
  // landed on the Cases list with an "Onboarding: In-progress" INCLUDE chip
  // (the exact opposite of the widget's own filter), because the values were
  // read op-blind and dumped into the same field an `in` filter would use.
  // `state` has the identical backend-supported notIn and was audited to
  // have the same latent bug, fixed alongside onboarding status.
  it("projectOnboardingStatus notIn decodes to excludeOnboardingStatuses, never onboardingStatuses (the reported bug)", () => {
    const href = WIDGET_RESOURCE_CONFIG.case.buildHref({
      filters: [
        { field: "projectOnboardingStatus", op: "notIn", values: ["In-Progress"] },
      ],
    });
    const parsed = readCasesFiltersFromUrl(hrefParams(href));

    expect(parsed.excludeOnboardingStatuses).toEqual(["In-Progress"]);
    expect(parsed.onboardingStatuses).toEqual([]);
  });

  it("state notIn decodes to excludeStates, never states", () => {
    const href = WIDGET_RESOURCE_CONFIG.case.buildHref({
      filters: [{ field: "state", op: "notIn", values: ["closed"] }],
    });
    const parsed = readCasesFiltersFromUrl(hrefParams(href));

    expect(parsed.excludeStates).toEqual(["closed"]);
    expect(parsed.states).toEqual([]);
  });

  it("state in and state notIn survive together, independently, on the same widget", () => {
    const href = WIDGET_RESOURCE_CONFIG.case.buildHref({
      filters: [
        { field: "state", op: "in", values: ["open", "work_in_progress"] },
        { field: "state", op: "notIn", values: ["closed"] },
      ],
    });
    const parsed = readCasesFiltersFromUrl(hrefParams(href));

    expect(parsed.states).toEqual(["open", "work_in_progress"]);
    expect(parsed.excludeStates).toEqual(["closed"]);
  });

  it("projectOnboardingStatus in and notIn survive together, independently, on the same widget", () => {
    const href = WIDGET_RESOURCE_CONFIG.case.buildHref({
      filters: [
        { field: "projectOnboardingStatus", op: "in", values: ["completed"] },
        { field: "projectOnboardingStatus", op: "notIn", values: ["In-Progress"] },
      ],
    });
    const parsed = readCasesFiltersFromUrl(hrefParams(href));

    expect(parsed.onboardingStatuses).toEqual(["completed"]);
    expect(parsed.excludeOnboardingStatuses).toEqual(["In-Progress"]);
  });

  it("hasEscalation:false (isEmpty) round-trips distinctly from isNotEmpty", () => {
    const href = WIDGET_RESOURCE_CONFIG.case.buildHref({
      filters: [{ field: "escalation", op: "isEmpty" }],
    });
    const parsed = readCasesFiltersFromUrl(hrefParams(href));
    expect(parsed.hasEscalation).toBe(false);
  });

  it("gte and lte on the same date field both survive independently", () => {
    const href = WIDGET_RESOURCE_CONFIG.case.buildHref({
      filters: [
        { field: "updatedOn", op: "gte", values: ["2026-01-01"] },
        { field: "updatedOn", op: "lte", values: ["2026-06-30"] },
      ],
    });
    const parsed = readCasesFiltersFromUrl(hrefParams(href));
    expect(parsed.updatedOnGte).toBe("2026-01-01");
    expect(parsed.updatedOnLte).toBe("2026-06-30");
  });

  it("`abt_sla_at_risk` (>=80% elapsed) and `abt_sla_violations` (>=100% elapsed) now produce distinct hrefs, each carrying its own threshold", () => {
    // Mirrors the two real widgets' `filters` verbatim (reference/dashboard-config.json,
    // team placeholder already resolved to a concrete groupId — the same
    // shape `DashboardWidgetTile` passes to `buildHref` after
    // `resolveTeamPlaceholder`). Before the data-layer commit these two
    // hrefs were byte-identical because `taskSLABusinessElapsedPercent` was
    // dropped entirely — see the cases-list-advanced-filters task record.
    const teamFilters = [
      { field: "creTeam", op: "in", values: ["22222222-2222-2222-2222-222222222222"] },
      { field: "state", op: "in", values: ["open", "work_in_progress", "waiting_on_wso2"] },
    ];
    const atRiskHref = WIDGET_RESOURCE_CONFIG.case.buildHref({
      filters: [
        ...teamFilters,
        { field: "taskSLABusinessElapsedPercent", op: "gte", values: ["80"] },
      ],
    });
    const violationsHref = WIDGET_RESOURCE_CONFIG.case.buildHref({
      filters: [
        ...teamFilters,
        { field: "taskSLABusinessElapsedPercent", op: "gte", values: ["100"] },
      ],
    });

    expect(atRiskHref).not.toBe(violationsHref);

    const atRiskParsed = readCasesFiltersFromUrl(hrefParams(atRiskHref));
    const violationsParsed = readCasesFiltersFromUrl(hrefParams(violationsHref));
    expect(atRiskParsed.slaElapsedPctGte).toBe(80);
    expect(violationsParsed.slaElapsedPctGte).toBe(100);
    // Both still carry the shared team/state constraints — only the
    // threshold differs.
    expect(atRiskParsed.csTeams).toEqual(violationsParsed.csTeams);
    expect(atRiskParsed.states).toEqual(violationsParsed.states);
  });
});

/**
 * service_request / security_report_analysis / announcement / engagement:
 * additional case-table resourceTypes (see `BeWidgetResourceType`), all
 * routing to the same /cases/search endpoint and response shape as `case`
 * (only the click-through destination differs per type).
 */
describe("WIDGET_RESOURCE_CONFIG — case-table resourceTypes beyond `case`", () => {
  it("all route to /cases/search and read the cases[] items key, same as case", () => {
    for (const type of [
      "service_request",
      "security_report_analysis",
      "announcement",
      "engagement",
    ] as const) {
      expect(WIDGET_RESOURCE_CONFIG[type].searchEndpoint).toBe("/cases/search");
      expect(WIDGET_RESOURCE_CONFIG[type].itemsKey).toBe("cases");
    }
  });

  it("service_request's buildHref lands on the operations service-requests tab with translated filters", () => {
    const href = WIDGET_RESOURCE_CONFIG.service_request.buildHref({
      filters: [{ field: "state", op: "in", values: ["open"] }],
    });
    expect(href.startsWith("/operations?")).toBe(true);
    const params = hrefParams(href);
    expect(params.get("tab")).toBe("service_requests");
    expect(params.get("states")).toBe("open");
  });

  it("security_report_analysis's buildHref lands on the security center security-reports tab with translated filters", () => {
    const href = WIDGET_RESOURCE_CONFIG.security_report_analysis.buildHref({
      filters: [{ field: "state", op: "in", values: ["open"] }],
    });
    expect(href.startsWith("/security-center?")).toBe(true);
    const params = hrefParams(href);
    expect(params.get("tab")).toBe("security_reports");
    expect(params.get("states")).toBe("open");
  });

  it("engagement's buildHref lands on /engagements with translated filters", () => {
    const href = WIDGET_RESOURCE_CONFIG.engagement.buildHref({
      filters: [{ field: "state", op: "in", values: ["open"] }],
    });
    expect(href.startsWith("/engagements?")).toBe(true);
    const params = hrefParams(href);
    expect(params.get("states")).toBe("open");
  });

  it("announcement's buildHref is the unfiltered /announcements page (no URL filter scheme exists there yet)", () => {
    expect(
      WIDGET_RESOURCE_CONFIG.announcement.buildHref({
        filters: [{ field: "state", op: "in", values: ["open"] }],
      }),
    ).toBe("/announcements");
  });

  // Regression test: incident_task has no dedicated list route of its own
  // (only ever viewed as part of its parent incident), so its buildHref used
  // to fall back to the plain, unfiltered incidents tab -- silently dropping
  // this widget's own filters and landing the user on an unrelated result
  // set. It must route through the generic dashboard-widget preview page
  // instead, which is filter-aware, using the widgetId/displayName context
  // DashboardWidgetTile passes at call time.
  it("incident_task's buildHref routes to the widget preview page with widget context, not the unfiltered incidents tab", () => {
    const href = WIDGET_RESOURCE_CONFIG.incident_task.buildHref(
      { assignmentGroupIds: ["grp-1"] },
      { widgetId: "widget-42", displayName: "My Incident Tasks" },
    );
    expect(href.startsWith("/dashboard/preview/incident-tasks?")).toBe(true);
    const params = hrefParams(href);
    expect(params.get("w")).toBe("widget-42");
    expect(params.get("n")).toBe("My Incident Tasks");
  });

  it("each of the four has its own distinct icon from `case` and from each other", () => {
    const icons = [
      WIDGET_RESOURCE_CONFIG.case.icon,
      WIDGET_RESOURCE_CONFIG.service_request.icon,
      WIDGET_RESOURCE_CONFIG.security_report_analysis.icon,
      WIDGET_RESOURCE_CONFIG.announcement.icon,
      WIDGET_RESOURCE_CONFIG.engagement.icon,
    ];
    expect(new Set(icons).size).toBe(icons.length);
  });
});
