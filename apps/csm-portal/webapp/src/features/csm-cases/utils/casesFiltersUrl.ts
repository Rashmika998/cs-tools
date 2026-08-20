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

import type {
  CaseState,
  Severity,
} from "@features/csm-dashboard/types/abtDashboard";
import type {
  BeCaseType,
  BeCaseWorkState,
  BeEngagementType,
} from "@api/backend/types";
import type { CasesFilters } from "@features/csm-cases/components/CasesFilterBar";
import { ALL_CASE_TYPES } from "@features/csm-cases/utils/caseType";

export const DEFAULT_CASES_FILTERS: CasesFilters = {
  search: "",
  severities: [],
  states: [],
  excludeStates: [],
  caseTypes: [],
  assignees: [],
  workStates: [],
  projects: [],
  engagementTypes: [],
  productNames: [],
  csTeams: [],
  sreTeams: [],
  tags: [],
  excludeTags: [],
  onboardingStatuses: [],
  excludeOnboardingStatuses: [],
  slaElapsedPctGte: null,
  slaElapsedPctLte: null,
  hasEscalation: null,
  escalationLevels: [],
  projectTypes: [],
  createdOnGte: null,
  createdOnLte: null,
  updatedOnGte: null,
  updatedOnLte: null,
  closedOnGte: null,
  closedOnLte: null,
  // Note: `tags`/`excludeTags` are real, wired-through fields as of this
  // codec (round-trip URL + `/cases/search` payload) — only the filter-bar
  // *control* for them is still pending; they surface only as removable
  // chips (see `buildActiveFilterChips` in `CasesFilterBar.tsx`).
  // `TagsMultiSelect` and `useSearchTags` remain used by the case-detail
  // "Add tag" picker regardless.
};

const VALID_SEVERITIES: Severity[] = ["S0", "S1", "S2", "S3", "S4"];
const VALID_STATES: CaseState[] = [
  "open",
  "work_in_progress",
  "solution_proposed",
  "awaiting_info",
  "waiting_on_wso2",
  "closed",
];
const VALID_CASE_TYPES: BeCaseType[] = ALL_CASE_TYPES;
const VALID_WORK_STATES: BeCaseWorkState[] = ["ongoing", "paused"];
const VALID_ENGAGEMENT_TYPES: BeEngagementType[] = [
  "migration",
  "consultancy",
  "new_feature_improvement",
  "follow_up",
  "onboarding",
];

function parseCsv<T extends string>(raw: string | null, allowed: T[]): T[] {
  if (!raw) return [];
  return raw
    .split(",")
    .map((s) => s.trim())
    .filter((s): s is T => (allowed as string[]).includes(s));
}

/**
 * Parse a CSV of free-form strings (used for assignee / project values that
 * aren't part of a fixed enum). Empties stripped, length-capped per entry to
 * avoid pathological URL growth.
 */
function parseFreeFormCsv(raw: string | null, maxEntryLen = 120): string[] {
  if (!raw) return [];
  return raw
    .split(",")
    .map((s) => s.trim())
    .filter((s) => s.length > 0 && s.length <= maxEntryLen);
}

/**
 * Parse a single free-form scalar (a date bound, a percent). `null` for an
 * absent, empty, or over-long param — same "silently drop, never throw"
 * policy as every other reader in this file.
 */
function parseFreeFormScalar(raw: string | null, maxLen = 40): string | null {
  if (!raw) return null;
  const trimmed = raw.trim();
  return trimmed.length > 0 && trimmed.length <= maxLen ? trimmed : null;
}

/**
 * Parse a single non-negative integer param (the SLA business-elapsed
 * percent bounds — mirrors the backend's own `parseCaseFilterPercent`, which
 * has no upper bound: a long-overdue SLA's percent keeps climbing well past
 * 100). `null` for anything absent, negative, or non-numeric.
 */
function parseNonNegativeInt(raw: string | null): number | null {
  if (!raw) return null;
  const n = Number(raw);
  return Number.isInteger(n) && n >= 0 ? n : null;
}

/**
 * Parse the `escalation` param (`"yes"` | `"no"`) into the tri-state
 * `CasesFilters.hasEscalation`. Any other value (including absence) is
 * "unfiltered" (`null`), never silently coerced to one of the two active
 * states.
 */
function parseEscalationParam(raw: string | null): boolean | null {
  if (raw === "yes") return true;
  if (raw === "no") return false;
  return null;
}

export function readCasesFiltersFromUrl(
  params: URLSearchParams,
): CasesFilters {
  const states = parseCsv(params.get("states"), VALID_STATES);
  // Work sub-states only apply to a search scoped to `work_in_progress` alone
  // — the server can't apply a work-state filter once another state is also
  // selected (see caseSearchPayload.ts's own matching guard). Mirror that
  // exact-match invariant here (not just "includes work_in_progress") so a
  // hand-edited / stale / dashboard-link URL with e.g.
  // `states=work_in_progress,open&workStates=ongoing` can't load an active
  // but un-clearable work-state filter behind the disabled control.
  const workStates =
    states.length === 1 && states[0] === "work_in_progress"
      ? parseCsv(params.get("workStates"), VALID_WORK_STATES)
      : [];
  return {
    search: params.get("search") ?? "",
    severities: parseCsv(params.get("severities"), VALID_SEVERITIES),
    states,
    excludeStates: parseCsv(params.get("excludeStates"), VALID_STATES),
    caseTypes: parseCsv(params.get("types"), VALID_CASE_TYPES),
    assignees: parseFreeFormCsv(params.get("assignees")),
    workStates,
    projects: parseFreeFormCsv(params.get("projects")),
    engagementTypes: parseCsv(params.get("engagementTypes"), VALID_ENGAGEMENT_TYPES),
    productNames: parseFreeFormCsv(params.get("products")),
    csTeams: parseFreeFormCsv(params.get("csTeams")),
    sreTeams: parseFreeFormCsv(params.get("sreTeams")),
    tags: parseFreeFormCsv(params.get("tags")),
    excludeTags: parseFreeFormCsv(params.get("excludeTags")),
    onboardingStatuses: parseFreeFormCsv(params.get("onboardingStatuses")),
    excludeOnboardingStatuses: parseFreeFormCsv(params.get("excludeOnboardingStatuses")),
    slaElapsedPctGte: parseNonNegativeInt(params.get("slaPctGte")),
    slaElapsedPctLte: parseNonNegativeInt(params.get("slaPctLte")),
    hasEscalation: parseEscalationParam(params.get("escalation")),
    escalationLevels: parseFreeFormCsv(params.get("escalationLevels")),
    projectTypes: parseFreeFormCsv(params.get("projectTypes")),
    createdOnGte: parseFreeFormScalar(params.get("createdFrom")),
    createdOnLte: parseFreeFormScalar(params.get("createdTo")),
    updatedOnGte: parseFreeFormScalar(params.get("updatedFrom")),
    updatedOnLte: parseFreeFormScalar(params.get("updatedTo")),
    closedOnGte: parseFreeFormScalar(params.get("closedFrom")),
    closedOnLte: parseFreeFormScalar(params.get("closedTo")),
  };
}

/**
 * Build the search-params object representing these filters. Default values
 * are omitted so the URL stays clean.
 *
 * **Op-awareness.** `writeWidgetPreviewHref` (in `csm-dashboard/utils/
 * widgetPreviewUrl.ts`) shipped a real bug from encoding an *opaque*
 * field/op/values array with one query param per `field` and no room for the
 * `op`: `tag notIn [x]` decoded back as `tag in [x]` (a tag EXCLUSION became
 * a tag FILTER) and value-less ops (`isEmpty`/`isNotEmpty`) were dropped for
 * having no values to serialize (see `6a9059789`). That file's fix was to
 * encode the op into the param name (`field~op`).
 *
 * `CasesFilters` doesn't need that trick: it is a fixed, *named*-field
 * struct, not a generic opaque array, so every op that would otherwise
 * collide on one field name already gets its own dedicated field/param
 * instead —
 *   - `tags` (op:in) vs. `excludeTags` (op:notIn), `states` vs.
 *     `excludeStates`, and `onboardingStatuses` vs.
 *     `excludeOnboardingStatuses` — each an in/notIn pair as two arrays, not
 *     one array plus an op flag, so `in` and `notIn` can never be conflated
 *     on the round trip. These three are the only case-search fields whose
 *     backend contract accepts `notIn` at all (see `POST /cases/search`'s
 *     `caseFilterOpSet`/per-field op table) — every other field is `in`-only
 *     and has no exclude counterpart to conflate with in the first place;
 *   - `slaElapsedPctGte`/`slaElapsedPctLte`, and the `createdOnGte/Lte`,
 *     `updatedOnGte/Lte`, `closedOnGte/Lte` date-range pairs — one param per
 *     bound, not a shared field with an op suffix;
 *   - `hasEscalation` — a tri-state (`true`/`false`/`null`) rather than a
 *     value-less-op name a caller could mistype; both of its states
 *     (`isEmpty`/`isNotEmpty`) always round-trip because presence, not a
 *     `values` array, is the entire predicate.
 * The `field~op` failure mode (an op silently decoding back as the default)
 * is therefore structurally not reachable here — there is no default op to
 * fall back to, because every op already has its own field. Reuse `field~op`
 * only if `CasesFilters` ever grows a *generic* filter escape hatch; as long
 * as it stays a named-field struct, one param per field+op pair is both
 * simpler and — since nothing is inferred from an omitted suffix — no less
 * safe.
 */
export function writeCasesFiltersToUrl(f: CasesFilters): URLSearchParams {
  const out = new URLSearchParams();
  if (f.search) out.set("search", f.search);
  if (f.severities.length) out.set("severities", f.severities.join(","));
  if (f.states.length) out.set("states", f.states.join(","));
  if (f.excludeStates.length) out.set("excludeStates", f.excludeStates.join(","));
  if (f.caseTypes.length) out.set("types", f.caseTypes.join(","));
  if (f.assignees.length) out.set("assignees", f.assignees.join(","));
  if (f.workStates.length) out.set("workStates", f.workStates.join(","));
  if (f.projects.length) out.set("projects", f.projects.join(","));
  if (f.engagementTypes.length) out.set("engagementTypes", f.engagementTypes.join(","));
  if (f.productNames.length) out.set("products", f.productNames.join(","));
  if (f.csTeams.length) out.set("csTeams", f.csTeams.join(","));
  if (f.sreTeams.length) out.set("sreTeams", f.sreTeams.join(","));
  if (f.tags.length) out.set("tags", f.tags.join(","));
  if (f.excludeTags.length) out.set("excludeTags", f.excludeTags.join(","));
  if (f.onboardingStatuses.length) {
    out.set("onboardingStatuses", f.onboardingStatuses.join(","));
  }
  if (f.excludeOnboardingStatuses.length) {
    out.set("excludeOnboardingStatuses", f.excludeOnboardingStatuses.join(","));
  }
  if (f.slaElapsedPctGte !== null) out.set("slaPctGte", String(f.slaElapsedPctGte));
  if (f.slaElapsedPctLte !== null) out.set("slaPctLte", String(f.slaElapsedPctLte));
  // Value-less predicate: `hasEscalation` alone (no `values`) is the whole
  // filter, so it must be written whenever it's non-null rather than only
  // when some paired array is non-empty — the exact class of bug
  // `writeWidgetPreviewHref` shipped by skipping value-less entries.
  if (f.hasEscalation !== null) out.set("escalation", f.hasEscalation ? "yes" : "no");
  if (f.escalationLevels.length) out.set("escalationLevels", f.escalationLevels.join(","));
  if (f.projectTypes.length) out.set("projectTypes", f.projectTypes.join(","));
  if (f.createdOnGte !== null) out.set("createdFrom", f.createdOnGte);
  if (f.createdOnLte !== null) out.set("createdTo", f.createdOnLte);
  if (f.updatedOnGte !== null) out.set("updatedFrom", f.updatedOnGte);
  if (f.updatedOnLte !== null) out.set("updatedTo", f.updatedOnLte);
  if (f.closedOnGte !== null) out.set("closedFrom", f.closedOnGte);
  if (f.closedOnLte !== null) out.set("closedTo", f.closedOnLte);
  return out;
}

/**
 * Count the filters that carry a non-default value (search counts as one).
 * Used for the filter-bar badge and by the cases page to tell whether the user
 * has expressed any intent yet — 0 means "show the search/filter prompt and
 * don't load anything".
 */
export function countActiveFilters(f: CasesFilters): number {
  let n = 0;
  if (f.search.trim()) n += 1;
  if (f.severities.length) n += 1;
  if (f.states.length) n += 1;
  if (f.excludeStates.length) n += 1;
  if (f.caseTypes.length) n += 1;
  if (f.assignees.length) n += 1;
  if (f.workStates.length) n += 1;
  if (f.projects.length) n += 1;
  if (f.engagementTypes.length) n += 1;
  if (f.productNames.length) n += 1;
  if (f.csTeams.length) n += 1;
  if (f.sreTeams.length) n += 1;
  if (f.tags.length) n += 1;
  if (f.excludeTags.length) n += 1;
  if (f.onboardingStatuses.length) n += 1;
  if (f.excludeOnboardingStatuses.length) n += 1;
  if (f.slaElapsedPctGte !== null) n += 1;
  if (f.slaElapsedPctLte !== null) n += 1;
  if (f.hasEscalation !== null) n += 1;
  if (f.escalationLevels.length) n += 1;
  if (f.projectTypes.length) n += 1;
  if (f.createdOnGte !== null) n += 1;
  if (f.createdOnLte !== null) n += 1;
  if (f.updatedOnGte !== null) n += 1;
  if (f.updatedOnLte !== null) n += 1;
  if (f.closedOnGte !== null) n += 1;
  if (f.closedOnLte !== null) n += 1;
  return n;
}

/**
 * Convenience: build a `/cases?...` href from a partial filter override.
 * Anything not specified falls back to the defaults.
 */
export function casesHref(overrides: Partial<CasesFilters>): string {
  const full: CasesFilters = { ...DEFAULT_CASES_FILTERS, ...overrides };
  const qs = writeCasesFiltersToUrl(full).toString();
  return qs ? `/cases?${qs}` : "/cases";
}
