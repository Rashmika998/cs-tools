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

import { describe, expect, it } from "vitest";
import type { CasesFilters } from "@features/csm-cases/components/CasesFilterBar";
import { DEFAULT_CASES_FILTERS } from "@features/csm-cases/utils/casesFiltersUrl";
import { buildCaseSearchFilters } from "./caseSearchPayload";

function filterOf(filters: CasesFilters, field: string) {
  return (buildCaseSearchFilters(filters, "", undefined).filters ?? []).filter(
    (f) => f.field === field,
  );
}

describe("buildCaseSearchFilters — new advanced-filter fields", () => {
  it("emits creTeam op:in for csTeams", () => {
    const filters: CasesFilters = { ...DEFAULT_CASES_FILTERS, csTeams: ["team-a"] };
    expect(filterOf(filters, "creTeam")).toEqual([
      { field: "creTeam", op: "in", values: ["team-a"] },
    ]);
  });

  it("emits sreTeam op:in for sreTeams, independently of creTeam", () => {
    const filters: CasesFilters = {
      ...DEFAULT_CASES_FILTERS,
      csTeams: ["team-a"],
      sreTeams: ["team-sre-b"],
    };
    expect(filterOf(filters, "creTeam")).toEqual([
      { field: "creTeam", op: "in", values: ["team-a"] },
    ]);
    expect(filterOf(filters, "sreTeam")).toEqual([
      { field: "sreTeam", op: "in", values: ["team-sre-b"] },
    ]);
  });

  it("emits tag op:in and tag op:notIn as two independent entries", () => {
    const filters: CasesFilters = {
      ...DEFAULT_CASES_FILTERS,
      tags: ["patch"],
      excludeTags: ["s_dip"],
    };
    const tagEntries = filterOf(filters, "tag");
    expect(tagEntries).toEqual([
      { field: "tag", op: "in", values: ["patch"] },
      { field: "tag", op: "notIn", values: ["s_dip"] },
    ]);
  });

  it("does NOT invert excludeTags into an `in` entry", () => {
    // Regression: the equivalent bug in widgetPreviewUrl.ts inverted `tag
    // notIn` into `tag in` because it dropped the op. This asserts the
    // payload builder never produces an `in` entry when only excludeTags is
    // set.
    const filters: CasesFilters = { ...DEFAULT_CASES_FILTERS, excludeTags: ["s_dip"] };
    const tagEntries = filterOf(filters, "tag");
    expect(tagEntries).toEqual([{ field: "tag", op: "notIn", values: ["s_dip"] }]);
    expect(tagEntries?.some((e) => e.op === "in")).toBe(false);
  });

  it("emits projectOnboardingStatus op:in", () => {
    const filters: CasesFilters = {
      ...DEFAULT_CASES_FILTERS,
      onboardingStatuses: ["in_progress"],
    };
    expect(filterOf(filters, "projectOnboardingStatus")).toEqual([
      { field: "projectOnboardingStatus", op: "in", values: ["in_progress"] },
    ]);
  });

  it("emits state op:in and state op:notIn as two independent entries", () => {
    const filters: CasesFilters = {
      ...DEFAULT_CASES_FILTERS,
      states: ["open"],
      excludeStates: ["closed"],
    };
    expect(filterOf(filters, "state")).toEqual([
      { field: "state", op: "in", values: ["open"] },
      { field: "state", op: "notIn", values: ["closed"] },
    ]);
  });

  it("does NOT invert excludeStates into an `in` entry", () => {
    const filters: CasesFilters = { ...DEFAULT_CASES_FILTERS, excludeStates: ["closed"] };
    const stateEntries = filterOf(filters, "state");
    expect(stateEntries).toEqual([{ field: "state", op: "notIn", values: ["closed"] }]);
    expect(stateEntries?.some((e) => e.op === "in")).toBe(false);
  });

  it("emits projectOnboardingStatus op:in and op:notIn as two independent entries", () => {
    const filters: CasesFilters = {
      ...DEFAULT_CASES_FILTERS,
      onboardingStatuses: ["completed"],
      excludeOnboardingStatuses: ["In-Progress"],
    };
    expect(filterOf(filters, "projectOnboardingStatus")).toEqual([
      { field: "projectOnboardingStatus", op: "in", values: ["completed"] },
      { field: "projectOnboardingStatus", op: "notIn", values: ["In-Progress"] },
    ]);
  });

  it("does NOT invert excludeOnboardingStatuses into an `in` entry (the reported bug)", () => {
    // Regression: reported live against a dashboard widget's
    // `projectOnboardingStatus notIn ["In-Progress"]` -- the click-through
    // showed an "Onboarding: In-progress" INCLUDE chip, the exact opposite
    // of the widget's own filter. This asserts the payload builder itself
    // never produces an `in` entry when only excludeOnboardingStatuses is set
    // (see widgetResourceConfig.test.ts for the click-through-side fix).
    const filters: CasesFilters = {
      ...DEFAULT_CASES_FILTERS,
      excludeOnboardingStatuses: ["In-Progress"],
    };
    const entries = filterOf(filters, "projectOnboardingStatus");
    expect(entries).toEqual([
      { field: "projectOnboardingStatus", op: "notIn", values: ["In-Progress"] },
    ]);
    expect(entries?.some((e) => e.op === "in")).toBe(false);
  });

  it("emits taskSLABusinessElapsedPercent gte and lte as separate entries", () => {
    const filters: CasesFilters = {
      ...DEFAULT_CASES_FILTERS,
      slaElapsedPctGte: 50,
      slaElapsedPctLte: 100,
    };
    expect(filterOf(filters, "taskSLABusinessElapsedPercent")).toEqual([
      { field: "taskSLABusinessElapsedPercent", op: "gte", values: ["50"] },
      { field: "taskSLABusinessElapsedPercent", op: "lte", values: ["100"] },
    ]);
  });

  it("emits escalation isNotEmpty with no `values` when hasEscalation is true", () => {
    const filters: CasesFilters = { ...DEFAULT_CASES_FILTERS, hasEscalation: true };
    expect(filterOf(filters, "escalation")).toEqual([
      { field: "escalation", op: "isNotEmpty" },
    ]);
  });

  it("emits escalation isEmpty with no `values` when hasEscalation is false", () => {
    const filters: CasesFilters = { ...DEFAULT_CASES_FILTERS, hasEscalation: false };
    expect(filterOf(filters, "escalation")).toEqual([{ field: "escalation", op: "isEmpty" }]);
  });

  it("omits escalation entirely when hasEscalation is null (unfiltered)", () => {
    const filters: CasesFilters = { ...DEFAULT_CASES_FILTERS, hasEscalation: null };
    expect(filterOf(filters, "escalation")).toEqual([]);
  });

  it("emits escalationLevel and projectType op:in", () => {
    const filters: CasesFilters = {
      ...DEFAULT_CASES_FILTERS,
      escalationLevels: ["L1"],
      projectTypes: ["enterprise"],
    };
    expect(filterOf(filters, "escalationLevel")).toEqual([
      { field: "escalationLevel", op: "in", values: ["L1"] },
    ]);
    expect(filterOf(filters, "projectType")).toEqual([
      { field: "projectType", op: "in", values: ["enterprise"] },
    ]);
  });

  it("emits createdOn/updatedOn/closedOn gte+lte as independent entries per field", () => {
    const filters: CasesFilters = {
      ...DEFAULT_CASES_FILTERS,
      createdOnGte: "2026-01-01",
      createdOnLte: "2026-03-31",
      updatedOnGte: "2026-02-01",
      closedOnLte: "2026-06-30",
    };
    expect(filterOf(filters, "createdOn")).toEqual([
      { field: "createdOn", op: "gte", values: ["2026-01-01"] },
      { field: "createdOn", op: "lte", values: ["2026-03-31"] },
    ]);
    expect(filterOf(filters, "updatedOn")).toEqual([
      { field: "updatedOn", op: "gte", values: ["2026-02-01"] },
    ]);
    expect(filterOf(filters, "closedOn")).toEqual([
      { field: "closedOn", op: "lte", values: ["2026-06-30"] },
    ]);
  });

  it("emits nothing beyond the default filters when all new fields are unset", () => {
    const result = buildCaseSearchFilters(DEFAULT_CASES_FILTERS, "", undefined);
    expect(result.filters).toBeUndefined();
  });
});

// A typed case number / WSO2 case id must go through as an exact-match field
// filter, not the free-text `searchQuery` scan. `searchQuery` is a CONTAINS/OR
// scan that also covers the description upstream, so an exact case number
// matched other cases merely *mentioning* it -- searching one case number
// surfaced a different case entirely.
describe("buildCaseSearchFilters — exact case-number / WSO2-id search", () => {
  it("routes a CS case number to an exact `number` filter, not searchQuery", () => {
    const result = buildCaseSearchFilters(DEFAULT_CASES_FILTERS, "CS0346083", undefined);

    expect(result.searchQuery).toBeUndefined();
    expect(result.filters).toEqual([
      { field: "number", op: "eq", values: ["CS0346083"] },
    ]);
  });

  it("routes a WSO2 case id to an exact `internalId` filter, not searchQuery", () => {
    const result = buildCaseSearchFilters(
      DEFAULT_CASES_FILTERS,
      "AXACOLPATRIASUB-484",
      undefined,
    );

    expect(result.searchQuery).toBeUndefined();
    expect(result.filters).toEqual([
      { field: "internalId", op: "eq", values: ["AXACOLPATRIASUB-484"] },
    ]);
  });

  it("still uses free-text searchQuery for anything that isn't an identifier", () => {
    const result = buildCaseSearchFilters(DEFAULT_CASES_FILTERS, "printer jam", undefined);

    expect(result.searchQuery).toBe("printer jam");
    expect(result.filters).toBeUndefined();
  });

  it("treats a partial/malformed case number as free text, so typing stays usable", () => {
    // Mid-typing (6 digits) and an over-long 8-digit string are both free text.
    expect(
      buildCaseSearchFilters(DEFAULT_CASES_FILTERS, "CS034608", undefined).searchQuery,
    ).toBe("CS034608");
    expect(
      buildCaseSearchFilters(DEFAULT_CASES_FILTERS, "CS03460834", undefined).searchQuery,
    ).toBe("CS03460834");
  });

  it("combines the exact identifier filter with the other active filters", () => {
    const result = buildCaseSearchFilters(
      { ...DEFAULT_CASES_FILTERS, caseTypes: ["case"] },
      "CS0346083",
      undefined,
    );

    expect(result.searchQuery).toBeUndefined();
    expect(result.filters).toEqual([
      { field: "type", op: "in", values: ["case"] },
      { field: "number", op: "eq", values: ["CS0346083"] },
    ]);
  });

  it("forceFreeText opts an identifier query back into the searchQuery scan", () => {
    // The cases list runs this leg alongside the exact one (see useGetCsmCases),
    // so a case that only *mentions* the number stays findable.
    const result = buildCaseSearchFilters(
      DEFAULT_CASES_FILTERS,
      "CS0346083",
      undefined,
      { forceFreeText: true },
    );

    expect(result.searchQuery).toBe("CS0346083");
    expect(result.filters).toBeUndefined();
  });

  it("emits no search filter at all for an empty query", () => {
    const result = buildCaseSearchFilters(DEFAULT_CASES_FILTERS, "", undefined);

    expect(result.searchQuery).toBeUndefined();
    expect(result.filters).toBeUndefined();
  });
});

describe("buildCaseSearchFilters — workState only applies when state is exactly work_in_progress", () => {
  it("emits workState when work_in_progress is the sole selected state", () => {
    const filters: CasesFilters = {
      ...DEFAULT_CASES_FILTERS,
      states: ["work_in_progress"],
      workStates: ["ongoing", "paused"],
    };

    expect(filterOf(filters, "workState")).toEqual([
      { field: "workState", op: "in", values: ["ongoing", "paused"] },
    ]);
  });

  // Regression guard: a stale `workStates` value reaching this builder any
  // way other than the filter bar's own onChange (a saved view, a
  // dashboard/pinned-view URL that predates the exact-match fix, a future
  // caller) must never silently narrow results to just in-progress/paused
  // cases once another state is also selected.
  it("drops workState from the payload when another state is selected alongside work_in_progress", () => {
    const filters: CasesFilters = {
      ...DEFAULT_CASES_FILTERS,
      states: ["work_in_progress", "open"],
      workStates: ["ongoing", "paused"],
    };

    expect(filterOf(filters, "workState")).toEqual([]);
    // The state filter itself still applies normally.
    expect(filterOf(filters, "state")).toEqual([
      { field: "state", op: "in", values: ["work_in_progress", "open"] },
    ]);
  });

  it("drops workState from the payload when no state is selected", () => {
    const filters: CasesFilters = {
      ...DEFAULT_CASES_FILTERS,
      states: [],
      workStates: ["ongoing"],
    };

    expect(filterOf(filters, "workState")).toEqual([]);
  });
});
