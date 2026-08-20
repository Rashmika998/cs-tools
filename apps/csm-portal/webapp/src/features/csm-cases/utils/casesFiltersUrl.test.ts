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
import {
  casesHref,
  DEFAULT_CASES_FILTERS,
  readCasesFiltersFromUrl,
  writeCasesFiltersToUrl,
} from "./casesFiltersUrl";

describe("readCasesFiltersFromUrl", () => {
  it("returns the defaults for an empty query string", () => {
    expect(readCasesFiltersFromUrl(new URLSearchParams())).toEqual(
      DEFAULT_CASES_FILTERS,
    );
  });

  it("parses a fully-populated query string", () => {
    const params = new URLSearchParams(
      "search=timeout&severities=S0,S2&states=open,work_in_progress,closed&types=case,engagement&assignees=alice@example.com,@me&workStates=ongoing,paused&projects=apim&products=API%20Manager,Asgardeo",
    );
    expect(readCasesFiltersFromUrl(params)).toEqual({
      ...DEFAULT_CASES_FILTERS,
      search: "timeout",
      severities: ["S0", "S2"],
      states: ["open", "work_in_progress", "closed"],
      caseTypes: ["case", "engagement"],
      assignees: ["alice@example.com", "@me"],
      // `work_in_progress` is one of three selected states here, not the
      // sole one -- workStates can't apply server-side in that shape, so it
      // parses back out as empty. See the exact-match tests below.
      workStates: [],
      projects: ["apim"],
      productNames: ["API Manager", "Asgardeo"],
    });
  });

  it("parses `tags` — a live param again, not the stale no-op it used to be", () => {
    const params = new URLSearchParams("tags=micro-gw,ws-policy");
    expect(readCasesFiltersFromUrl(params)).toEqual({
      ...DEFAULT_CASES_FILTERS,
      tags: ["micro-gw", "ws-policy"],
    });
  });

  it("drops values outside the allowed enums", () => {
    const params = new URLSearchParams(
      "severities=S0,S9,wat&states=open,nonsense&types=case,bogus_type",
    );
    const f = readCasesFiltersFromUrl(params);
    expect(f.severities).toEqual(["S0"]);
    expect(f.states).toEqual(["open"]);
    expect(f.caseTypes).toEqual(["case"]);
  });

  it("drops work-state values outside the allowed enum", () => {
    const params = new URLSearchParams(
      "states=work_in_progress&workStates=ongoing,bogus,2",
    );
    expect(readCasesFiltersFromUrl(params).workStates).toEqual(["ongoing"]);
  });

  it("drops work states when `work_in_progress` is not in the state filter", () => {
    const params = new URLSearchParams("states=open&workStates=ongoing,paused");
    expect(readCasesFiltersFromUrl(params).workStates).toEqual([]);
  });

  it("drops work states when `work_in_progress` is selected alongside another state", () => {
    const params = new URLSearchParams(
      "states=work_in_progress,open&workStates=ongoing,paused",
    );
    expect(readCasesFiltersFromUrl(params).workStates).toEqual([]);
  });

  it("strips empties and over-long free-form entries", () => {
    const long = "x".repeat(121);
    const params = new URLSearchParams();
    params.set("assignees", `alice, ,${long}`);
    expect(readCasesFiltersFromUrl(params).assignees).toEqual(["alice"]);
  });
});

describe("writeCasesFiltersToUrl", () => {
  it("omits default-valued fields to keep the URL clean", () => {
    expect(writeCasesFiltersToUrl(DEFAULT_CASES_FILTERS).toString()).toBe("");
  });

  it("round-trips a non-default filter set", () => {
    const filters: CasesFilters = {
      ...DEFAULT_CASES_FILTERS,
      search: "disk full",
      severities: ["S1"],
      states: ["work_in_progress"],
      caseTypes: ["service_request"],
      assignees: ["carol@example.com"],
      workStates: ["paused"],
      projects: ["streaming"],
      productNames: ["Identity Server", "Asgardeo"],
    };
    const round = readCasesFiltersFromUrl(writeCasesFiltersToUrl(filters));
    expect(round).toEqual(filters);
  });
});

/**
 * Regression coverage for the exact bug class `widgetPreviewUrl.ts` shipped
 * (see `6a9059789`): an op silently decoding back as a different op, or a
 * value-less op being dropped for having no `values` to serialize. This
 * codec avoids the `field~op` mechanism that bug required fixing (see
 * `writeCasesFiltersToUrl`'s doc comment) by giving every op its own named
 * field — these tests exist to prove that actually holds, not just to
 * restate the design.
 */
describe("op-awareness (regression: the widgetPreviewUrl field~op bug)", () => {
  it("`tags` (op:in) and `excludeTags` (op:notIn) never conflate on a round trip", () => {
    const filters: CasesFilters = {
      ...DEFAULT_CASES_FILTERS,
      excludeTags: ["s_dip"],
    };
    const round = readCasesFiltersFromUrl(writeCasesFiltersToUrl(filters));
    // The bug this guards against: `tag notIn [s_dip]` decoding back as
    // `tag in [s_dip]` — an EXCLUSION becoming a FILTER. Assert both halves:
    // the exclusion survived, and it did NOT leak into `tags` (inclusion).
    expect(round.excludeTags).toEqual(["s_dip"]);
    expect(round.tags).toEqual([]);
  });

  it("`tags` and `excludeTags` survive together, independently, when both are set", () => {
    const filters: CasesFilters = {
      ...DEFAULT_CASES_FILTERS,
      tags: ["patch"],
      excludeTags: ["s_dip"],
    };
    const round = readCasesFiltersFromUrl(writeCasesFiltersToUrl(filters));
    expect(round.tags).toEqual(["patch"]);
    expect(round.excludeTags).toEqual(["s_dip"]);
  });

  it("`states` (op:in) and `excludeStates` (op:notIn) never conflate on a round trip", () => {
    const filters: CasesFilters = { ...DEFAULT_CASES_FILTERS, excludeStates: ["closed"] };
    const round = readCasesFiltersFromUrl(writeCasesFiltersToUrl(filters));
    expect(round.excludeStates).toEqual(["closed"]);
    expect(round.states).toEqual([]);
  });

  it("`states` and `excludeStates` survive together, independently, when both are set", () => {
    const filters: CasesFilters = {
      ...DEFAULT_CASES_FILTERS,
      states: ["open", "work_in_progress"],
      excludeStates: ["closed"],
    };
    const round = readCasesFiltersFromUrl(writeCasesFiltersToUrl(filters));
    expect(round.states).toEqual(["open", "work_in_progress"]);
    expect(round.excludeStates).toEqual(["closed"]);
  });

  // Regression: reported live against a dashboard widget's
  // `projectOnboardingStatus notIn ["In-Progress"]` -- the click-through
  // showed an "Onboarding: In-progress" INCLUDE chip/filter, the exact
  // opposite of the widget's own filter.
  it("`onboardingStatuses` (op:in) and `excludeOnboardingStatuses` (op:notIn) never conflate on a round trip (the reported bug)", () => {
    const filters: CasesFilters = {
      ...DEFAULT_CASES_FILTERS,
      excludeOnboardingStatuses: ["In-Progress"],
    };
    const round = readCasesFiltersFromUrl(writeCasesFiltersToUrl(filters));
    expect(round.excludeOnboardingStatuses).toEqual(["In-Progress"]);
    expect(round.onboardingStatuses).toEqual([]);
  });

  it("`onboardingStatuses` and `excludeOnboardingStatuses` survive together, independently, when both are set", () => {
    const filters: CasesFilters = {
      ...DEFAULT_CASES_FILTERS,
      onboardingStatuses: ["completed"],
      excludeOnboardingStatuses: ["In-Progress"],
    };
    const round = readCasesFiltersFromUrl(writeCasesFiltersToUrl(filters));
    expect(round.onboardingStatuses).toEqual(["completed"]);
    expect(round.excludeOnboardingStatuses).toEqual(["In-Progress"]);
  });

  it("a value-less op (`hasEscalation` / escalation isNotEmpty) survives rather than being dropped", () => {
    const filters: CasesFilters = { ...DEFAULT_CASES_FILTERS, hasEscalation: true };
    const href = writeCasesFiltersToUrl(filters);
    // Assert the param is actually present, not just that the round trip
    // happens to produce the right value some other way.
    expect(href.get("escalation")).toBe("yes");
    expect(readCasesFiltersFromUrl(href).hasEscalation).toBe(true);
  });

  it("the other value-less state (`hasEscalation: false` / isEmpty) also survives", () => {
    const filters: CasesFilters = { ...DEFAULT_CASES_FILTERS, hasEscalation: false };
    const href = writeCasesFiltersToUrl(filters);
    expect(href.get("escalation")).toBe("no");
    expect(readCasesFiltersFromUrl(href).hasEscalation).toBe(false);
  });

  it("a gte+lte range on one field round-trips with both bounds intact", () => {
    const filters: CasesFilters = {
      ...DEFAULT_CASES_FILTERS,
      slaElapsedPctGte: 50,
      slaElapsedPctLte: 100,
      createdOnGte: "2026-01-01",
      createdOnLte: "2026-03-31",
    };
    const round = readCasesFiltersFromUrl(writeCasesFiltersToUrl(filters));
    expect(round.slaElapsedPctGte).toBe(50);
    expect(round.slaElapsedPctLte).toBe(100);
    expect(round.createdOnGte).toBe("2026-01-01");
    expect(round.createdOnLte).toBe("2026-03-31");
  });

  it("a one-sided range only sets the bound that was given", () => {
    const filters: CasesFilters = { ...DEFAULT_CASES_FILTERS, slaElapsedPctGte: 90 };
    const round = readCasesFiltersFromUrl(writeCasesFiltersToUrl(filters));
    expect(round.slaElapsedPctGte).toBe(90);
    expect(round.slaElapsedPctLte).toBeNull();
  });
});

describe("casesHref", () => {
  it("returns the bare path when overrides reduce to defaults", () => {
    expect(casesHref({})).toBe("/cases");
  });

  it("builds a query string from a partial override", () => {
    expect(casesHref({ severities: ["S1"] })).toBe("/cases?severities=S1");
  });
});
