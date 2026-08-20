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

import {
  Box,
  Button,
  Checkbox,
  Chip,
  Dialog,
  DialogActions,
  DialogContent,
  DialogTitle,
  Divider,
  FormControlLabel,
  Grid,
  IconButton,
  InputAdornment,
  ListItemIcon,
  ListItemText,
  ListSubheader,
  Menu,
  MenuItem,
  Paper,
  TextField,
  Typography,
} from "@wso2/oxygen-ui";
import {
  Bookmark,
  BookmarkPlus,
  Check,
  ChevronDown,
  ChevronUp,
  ListFilter,
  Search,
  Trash2,
  X,
} from "@wso2/oxygen-ui-icons-react";
import { useMemo, useState, type JSX } from "react";
import type {
  CaseState,
  Severity,
} from "@features/csm-dashboard/types/abtDashboard";
import { STATE_LABEL } from "@features/csm-dashboard/utils/abtDashboard";
import {
  countActiveFilters,
  readCasesFiltersFromUrl,
  writeCasesFiltersToUrl,
} from "@features/csm-cases/utils/casesFiltersUrl";
import { useTeams } from "@features/csm-dashboard/api/useTeams";
import {
  deleteFilterView,
  saveFilterView,
  SUGGESTED_FILTER_VIEWS,
  useSavedFilterViews,
} from "@features/csm-cases/utils/savedFilterViews";
import type {
  BeCaseType,
  BeCaseWorkState,
  BeEngagementType,
} from "@api/backend/types";
import {
  ALL_CASE_TYPES,
  CASE_TYPE_LABEL,
} from "@features/csm-cases/utils/caseType";
import AsyncProjectMultiSelect from "@features/csm-cases/components/AsyncProjectMultiSelect";
import MultiSelectField from "@components/MultiSelectField";
import AsyncAssigneeMultiSelect from "@features/csm-cases/components/AsyncAssigneeMultiSelect";
import ProductNameMultiSelect from "@features/csm-cases/components/ProductNameMultiSelect";


/**
 * Filter state for the CSM cases list. `severities` / `states` / `caseTypes`
 * are multi-select arrays driven by fixed enums; `projects` is an id-based
 * type-to-search multi-select. `assignees` holds engineer **emails** plus the
 * sentinel `@me`; `useGetCsmCases` resolves these to the engineer UUIDs that
 * `/cases/search` filters on. All are pushed into the `/cases/search` payload
 * server-side.
 */
export interface CasesFilters {
  search: string;
  severities: Severity[];
  states: CaseState[];
  /** States the case must NOT be in (`state` op:notIn). Not the inverse of
   * `states` — a distinct field so `in` and `notIn` can never be conflated
   * on the round trip, same reasoning as `tags`/`excludeTags`. No dedicated
   * bar control (like `excludeTags`) — only ever set via a dashboard
   * click-through, surfaced/removable as a chip. */
  excludeStates: CaseState[];
  /** Case-type filter (BE `typeKeys`). Empty = all types. */
  caseTypes: BeCaseType[];
  /** Engineer emails (+ the `@me` sentinel) to filter by assigned engineer. */
  assignees: string[];
  /** Work sub-state filter; only meaningful when `states` includes `work_in_progress`. */
  workStates: BeCaseWorkState[];
  projects: string[];
  /** Engagement sub-type filter; only meaningful when `caseTypes` is locked to `engagement`. */
  engagementTypes: BeEngagementType[];
  /** Product family names (e.g. "API Manager"); matches all versions of each. */
  productNames: string[];
  /** CS team group ids (`creTeam` op:in) the case's project is scoped to. */
  csTeams: string[];
  /** SRE team group ids (`sreTeam` op:in) the case's project is scoped to.
   * Independent of `csTeams` -- a case's account may carry both a CRE and
   * an SRE team assignment. */
  sreTeams: string[];
  /** Tags the case must carry (`tag` op:in). Independent of `excludeTags` —
   * both may be set at once (the backend ANDs them). */
  tags: string[];
  /** Tags the case must NOT carry (`tag` op:notIn). Not the inverse of
   * `tags` — a distinct field so `in` and `notIn` can never be conflated on
   * the round trip (see `casesFiltersUrl.ts`'s codec doc comment). */
  excludeTags: string[];
  /** Project onboarding status values (`projectOnboardingStatus` op:in). */
  onboardingStatuses: string[];
  /** Project onboarding status values the case's project must NOT have
   * (`projectOnboardingStatus` op:notIn). Not the inverse of
   * `onboardingStatuses` — a distinct field so `in` and `notIn` can never be
   * conflated on the round trip, same reasoning as `tags`/`excludeTags`. */
  excludeOnboardingStatuses: string[];
  /** Inclusive lower bound on the case's active task's SLA business-elapsed
   * percent (`taskSLABusinessElapsedPercent` op:gte). `null` = unset. */
  slaElapsedPctGte: number | null;
  /** Inclusive upper bound, same field, op:lte. `null` = unset. */
  slaElapsedPctLte: number | null;
  /** Escalation presence (`escalation` field): `true` = has an active
   * escalation (op:isNotEmpty), `false` = has none (op:isEmpty), `null` =
   * unfiltered. Deliberately not string-typed on the ops themselves — the
   * value-less op IS the whole predicate here, so a tri-state is the
   * accurate shape rather than an op name a caller could typo. */
  hasEscalation: boolean | null;
  /** Escalation level values (`escalationLevel` op:in). */
  escalationLevels: string[];
  /** Project-type ids (`projectType` op:in). */
  projectTypes: string[];
  /** `createdOn` range bounds (op:gte / op:lte respectively); RFC3339 or
   * `YYYY-MM-DD`. `null` = unbounded on that side. */
  createdOnGte: string | null;
  createdOnLte: string | null;
  /** `updatedOn` range bounds — same shape as `createdOnGte`/`createdOnLte`. */
  updatedOnGte: string | null;
  updatedOnLte: string | null;
  /** `closedOn` range bounds — same shape as `createdOnGte`/`createdOnLte`. */
  closedOnGte: string | null;
  closedOnLte: string | null;
}

/**
 * Lightweight user-directory entry surfaced in the assignee picker. The filter
 * stores the `email` as the value; the `name` is shown as the option label.
 */
export interface AssigneeUser {
  name: string;
  email: string;
}

interface CasesFilterBarProps {
  filters: CasesFilters;
  onChange: (next: CasesFilters) => void;
  onReset: () => void;
  isFiltersOpen: boolean;
  onFiltersToggle: () => void;
  /** Full user directory shown in the assignee picker. */
  availableAssigneeUsers: AssigneeUser[];
  /** Projects for the (id-based) project filter — value is the id, label the name. */
  availableProjects: { id: string; name: string }[];
  /**
   * Show the severity control. Severity (S1-S4) is a support-case concept, so
   * this is only meaningful when the list is scoped to support cases; other
   * record types (service requests, engagements, etc.) hide it.
   */
  showSeverityFilter?: boolean;
  /** Hide the case-type control when the surrounding view locks the type. */
  hideTypeFilter?: boolean;
  /**
   * Label for the case-type control. Defaults to "Case type"; a view that
   * mixes every record type under a broader umbrella term (e.g. a project's
   * Work items tab, which spans cases/service requests/security reports/
   * engagements/announcements) can override it to "Work item type" so the
   * label matches what the surrounding page calls these records, without
   * changing the control's behavior or its `caseTypes` value shape.
   */
  typeFilterLabel?: string;
  /** Hide the project control when the surrounding view is project-scoped. */
  hideProjectFilter?: boolean;
  /** Show the engagement-type multi-select (only relevant when type is locked to engagement). */
  showEngagementTypeFilter?: boolean;
}

const ALL_ENGAGEMENT_TYPES: BeEngagementType[] = [
  "migration",
  "consultancy",
  "new_feature_improvement",
  "follow_up",
  "onboarding",
];

const ENGAGEMENT_TYPE_LABEL: Record<BeEngagementType, string> = {
  migration: "Migration",
  consultancy: "Consultancy",
  new_feature_improvement: "New feature / improvement",
  follow_up: "Follow-up",
  onboarding: "Onboarding",
};

// Work state has no bar control of its own (see `buildActiveFilterChips`'s
// doc comment) -- this only labels its chip now.
const WORK_STATE_LABEL: Record<BeCaseWorkState, string> = {
  ongoing: "Ongoing",
  paused: "Paused",
};

const ALL_SEVERITIES: Severity[] = ["S0", "S1", "S2", "S3", "S4"];

// The one `projectOnboardingStatus` value every dashboard widget's `notIn`
// actually excludes today (a project still being onboarded). A single
// checkbox for this specific value -- same simple boolean-toggle pattern as
// `IncidentsFilterBar`'s "SLA violated" checkbox -- rather than a full
// multi-select picker for a field with no fixed, known choice list.
const ONBOARDING_IN_PROGRESS = "In-Progress";
const PRIMARY_STATES: CaseState[] = [
  "open",
  "work_in_progress",
  "awaiting_info",
  "solution_proposed",
  "waiting_on_wso2",
  "closed",
];

/**
 * Turns a raw backend token (`snake_case`, `PascalCase`, or a mix — the
 * onboarding-status/escalation-level vocabularies aren't normalized to one
 * casing) into a readable label: `"in_progress"` / `"OnHold"` both become
 * `"In progress"` / `"On hold"`. Falls back to the raw token unchanged when
 * it's empty.
 */
function humanizeToken(raw: string): string {
  if (!raw) return raw;
  const spaced = raw.replace(/_/g, " ").replace(/([a-z0-9])([A-Z])/g, "$1 $2");
  const lower = spaced.toLowerCase();
  return lower.charAt(0).toUpperCase() + lower.slice(1);
}

/** Formats a `createdOn`/`updatedOn`/`closedOn` bound for a chip label — a
 * locale date when parseable, the raw string otherwise (never throws on a
 * malformed value; this is a display fallback, not validation). */
function formatDateBound(raw: string): string {
  // A bare `YYYY-MM-DD` is parsed as UTC midnight by `Date`, which renders as
  // the previous day for any locale behind UTC — pin it to local midnight.
  const d = /^\d{4}-\d{2}-\d{2}$/.test(raw)
    ? new Date(`${raw}T00:00:00`)
    : new Date(raw);
  return Number.isNaN(d.getTime()) ? raw : d.toLocaleDateString();
}

/** One removable "active filter" chip. */
interface ActiveFilterChip {
  key: string;
  label: string;
  onRemove: (filters: CasesFilters) => CasesFilters;
}

/**
 * Fields extended onto `CasesFilters` for lossless dashboard click-through
 * (onboarding status, SLA %, escalation, project type, the three date
 * ranges) get no dedicated bar control of their own — ten new fields would
 * overwhelm the bar, and most of them only ever get set by a widget
 * click-through, not hand-picked in the bar. They must still be visible and
 * individually removable, though, or a user landing on a dashboard-filtered
 * cases list has no way to see (or undo) *why* it's filtered — hence one
 * chip per active value here, shown regardless of whether the filter grid
 * itself is expanded. `sreTeams`/`tags`/`excludeTags`/`workStates` are
 * included here too: their bar controls were removed as clutter (they are
 * advanced, rarely hand-picked, and a better home for advanced filters is
 * still to be designed), so a chip is now the ONLY way a user can see or
 * clear them after arriving from a dashboard click-through. `csTeams` has
 * its own "Team" bar control (see the filter grid below) and is
 * deliberately NOT chipped here too — every other bar-controlled field
 * (`states`, `severities`, ...) shows its selection inside its own control,
 * not as a second, redundant chip.
 */
function buildActiveFilterChips(
  filters: CasesFilters,
  /** groupId -> team display name, so a team chip never shows a raw UUID.
   * Falls back to the id when the lookup has not resolved (or the team is
   * unknown) rather than hiding the chip — an unlabelled filter the user can
   * still see and remove beats an invisible one. Only feeds the `sreTeams`
   * chip now (`csTeams` has its own bar control), but still covers both
   * `creGroupId` and `sreGroupId` keys since the caller passes one merged
   * map either way. */
  teamLabels: Record<string, string> = {},
): ActiveFilterChip[] {
  const chips: ActiveFilterChip[] = [];

  filters.sreTeams.forEach((groupId) => {
    chips.push({
      key: `sreTeam-${groupId}`,
      label: `SRE team: ${teamLabels[groupId] ?? groupId}`,
      onRemove: (f) => ({ ...f, sreTeams: f.sreTeams.filter((t) => t !== groupId) }),
    });
  });

  filters.tags.forEach((tag) => {
    chips.push({
      key: `tag-${tag}`,
      label: `Tag: ${tag}`,
      onRemove: (f) => ({ ...f, tags: f.tags.filter((t) => t !== tag) }),
    });
  });

  filters.excludeTags.forEach((tag) => {
    chips.push({
      key: `excludeTag-${tag}`,
      label: `Excluding tag: ${tag}`,
      onRemove: (f) => ({ ...f, excludeTags: f.excludeTags.filter((t) => t !== tag) }),
    });
  });

  filters.excludeStates.forEach((state) => {
    chips.push({
      key: `excludeState-${state}`,
      label: `Excluding state: ${STATE_LABEL[state] ?? state}`,
      onRemove: (f) => ({
        ...f,
        excludeStates: f.excludeStates.filter((s) => s !== state),
      }),
    });
  });

  filters.workStates.forEach((workState) => {
    chips.push({
      key: `workState-${workState}`,
      label: `Work state: ${WORK_STATE_LABEL[workState] ?? workState}`,
      onRemove: (f) => ({
        ...f,
        workStates: f.workStates.filter((w) => w !== workState),
      }),
    });
  });

  filters.onboardingStatuses.forEach((status) => {
    chips.push({
      key: `onboarding-${status}`,
      label: `Onboarding: ${humanizeToken(status)}`,
      onRemove: (f) => ({
        ...f,
        onboardingStatuses: f.onboardingStatuses.filter((s) => s !== status),
      }),
    });
  });

  // ONBOARDING_IN_PROGRESS has its own checkbox in the bar now (see the
  // filter grid below) -- skip it here so it isn't shown twice. Any other
  // excluded value (only reachable via a dashboard click-through, since the
  // field has no fixed choice list) still gets a chip, same as before.
  filters.excludeOnboardingStatuses
    .filter((status) => status !== ONBOARDING_IN_PROGRESS)
    .forEach((status) => {
      chips.push({
        key: `excludeOnboarding-${status}`,
        label: `Excluding onboarding: ${humanizeToken(status)}`,
        onRemove: (f) => ({
          ...f,
          excludeOnboardingStatuses: f.excludeOnboardingStatuses.filter((s) => s !== status),
        }),
      });
    });

  if (filters.slaElapsedPctGte !== null) {
    chips.push({
      key: "sla-gte",
      label: `SLA ≥ ${filters.slaElapsedPctGte}%`,
      onRemove: (f) => ({ ...f, slaElapsedPctGte: null }),
    });
  }
  if (filters.slaElapsedPctLte !== null) {
    chips.push({
      key: "sla-lte",
      label: `SLA ≤ ${filters.slaElapsedPctLte}%`,
      onRemove: (f) => ({ ...f, slaElapsedPctLte: null }),
    });
  }

  if (filters.hasEscalation !== null) {
    chips.push({
      key: "escalation",
      label: filters.hasEscalation ? "Escalated" : "No escalation",
      onRemove: (f) => ({ ...f, hasEscalation: null }),
    });
  }

  filters.escalationLevels.forEach((level) => {
    chips.push({
      key: `escalation-level-${level}`,
      label: `Escalation level: ${level}`,
      onRemove: (f) => ({
        ...f,
        escalationLevels: f.escalationLevels.filter((l) => l !== level),
      }),
    });
  });

  filters.projectTypes.forEach((projectType) => {
    chips.push({
      key: `project-type-${projectType}`,
      // No project-type name lookup exists in the frontend yet (the backend
      // filter is keyed by an opaque id, not a slug) — shows the raw id
      // rather than guessing at a label.
      label: `Project type: ${projectType}`,
      onRemove: (f) => ({
        ...f,
        projectTypes: f.projectTypes.filter((t) => t !== projectType),
      }),
    });
  });

  const dateRanges: [string, keyof CasesFilters, keyof CasesFilters][] = [
    ["Created", "createdOnGte", "createdOnLte"],
    ["Updated", "updatedOnGte", "updatedOnLte"],
    ["Closed", "closedOnGte", "closedOnLte"],
  ];
  for (const [labelPrefix, gteKey, lteKey] of dateRanges) {
    const gte = filters[gteKey] as string | null;
    if (gte !== null) {
      chips.push({
        key: `${gteKey}`,
        label: `${labelPrefix} after ${formatDateBound(gte)}`,
        onRemove: (f) => ({ ...f, [gteKey]: null }),
      });
    }
    const lte = filters[lteKey] as string | null;
    if (lte !== null) {
      chips.push({
        key: `${lteKey}`,
        label: `${labelPrefix} before ${formatDateBound(lte)}`,
        onRemove: (f) => ({ ...f, [lteKey]: null }),
      });
    }
  }

  return chips;
}

export default function CasesFilterBar({
  filters,
  onChange,
  onReset,
  isFiltersOpen,
  onFiltersToggle,
  availableAssigneeUsers,
  availableProjects,
  showSeverityFilter = true,
  hideTypeFilter = false,
  typeFilterLabel = "Case type",
  hideProjectFilter = false,
  showEngagementTypeFilter = false,
}: CasesFilterBarProps): JSX.Element {
  const activeCount = countActiveFilters(filters);
  const hasActive = activeCount > 0;

  // Team is a fixed, small enough list to fetch in full (same endpoint/hook
  // the team-based dashboards use -- see AbtDashboardHeader) rather than a
  // type-to-search async picker, and doubles as the source for the "CS
  // team" bar control below and the SRE-team chip label (SRE team has no
  // bar control of its own -- see `buildActiveFilterChips`).
  const { data: teams } = useTeams(true);
  const teamLabels = useMemo(() => {
    const labels: Record<string, string> = {};
    for (const t of teams ?? []) {
      if (t.creGroupId) labels[t.creGroupId] = t.name;
      if (t.sreGroupId) labels[t.sreGroupId] = t.name;
    }
    return labels;
  }, [teams]);
  // `creGroupId` (not the registry `id`) is what a `creTeam`/`csTeams` filter
  // entry actually matches on; only teams with one configured are
  // selectable here (an id-less team has nothing such a filter could hold).
  const teamOptions = useMemo(
    () =>
      (teams ?? [])
        .filter((t): t is typeof t & { creGroupId: string } => Boolean(t.creGroupId))
        .map((t) => ({ value: t.creGroupId, label: t.name })),
    [teams],
  );

  const activeFilterChips = useMemo(
    () => buildActiveFilterChips(filters, teamLabels),
    [filters, teamLabels],
  );

  // ── Saved views ──────────────────────────────────────────────────────────
  // A saved view is just a name pointing at a serialized filter query string;
  // applying one feeds the parsed filters back through onChange (which the page
  // writes to the URL), so the URL stays the source of truth.
  const savedViews = useSavedFilterViews();
  const currentQs = writeCasesFiltersToUrl(filters).toString();
  // Canonicalize a query string (normalize comma encoding, param order, and
  // drop unknown params) so the "active view" check matches regardless of how a
  // view's qs was authored — suggested presets use literal commas, while
  // writeCasesFiltersToUrl emits %2C.
  const canonicalQs = (qs: string): string =>
    writeCasesFiltersToUrl(
      readCasesFiltersFromUrl(new URLSearchParams(qs)),
    ).toString();
  const currentCanonical = canonicalQs(currentQs);
  const isActiveView = (qs: string): boolean => canonicalQs(qs) === currentCanonical;
  const [savedAnchor, setSavedAnchor] = useState<HTMLElement | null>(null);
  const [saveDialogOpen, setSaveDialogOpen] = useState(false);
  const [newViewName, setNewViewName] = useState("");

  const applyView = (qs: string): void => {
    setSavedAnchor(null);
    onChange(readCasesFiltersFromUrl(new URLSearchParams(qs)));
  };

  const handleSaveView = (): void => {
    if (!newViewName.trim()) return;
    saveFilterView(newViewName, currentQs);
    setNewViewName("");
    setSaveDialogOpen(false);
    setSavedAnchor(null);
  };

  const severityOptions = useMemo(
    () => ALL_SEVERITIES.map((s) => ({ value: s, label: s })),
    [],
  );
  const stateOptions = useMemo(
    () => PRIMARY_STATES.map((s) => ({ value: s, label: STATE_LABEL[s] })),
    [],
  );
  const caseTypeOptions = useMemo(
    () => ALL_CASE_TYPES.map((t) => ({ value: t, label: CASE_TYPE_LABEL[t] })),
    [],
  );
  const engagementTypeOptions = useMemo(
    () => ALL_ENGAGEMENT_TYPES.map((t) => ({ value: t, label: ENGAGEMENT_TYPE_LABEL[t] })),
    [],
  );

  // Project filter loads the first page of projects on open and pages through
  // the rest on scroll (and narrows as you type) rather than loading the whole
  // catalogue at once. `availableProjects` (projects on the loaded cases) only
  // seeds chip labels for already-selected ids before any page loads.
  const projectNameSeed = useMemo(
    () => new Map(availableProjects.map((p) => [p.id, p.name])),
    [availableProjects],
  );

  // The assignee filter searches the user directory from the backend as you
  // type (see AsyncAssigneeMultiSelect), so anyone is findable — not just the
  // first page of users. `availableAssigneeUsers` (the directory prefetch /
  // owners on loaded cases) only seeds chip labels for already-selected emails
  // before any search has run.
  const assigneeNameSeed = useMemo(() => {
    const m = new Map<string, string>();
    availableAssigneeUsers.forEach((u) => {
      if (u.email) m.set(u.email, u.name);
    });
    return m;
  }, [availableAssigneeUsers]);

  return (
    <Paper sx={{ p: 2.5, display: "flex", flexDirection: "column", gap: 1.5 }}>
      {/* Search + saved views + filters toggle. */}
      <Box sx={{ display: "flex", alignItems: "center", gap: 1.5, flexWrap: "wrap" }}>
        <Box sx={{ position: "relative", flex: 1, minWidth: 240 }}>
          <TextField
            fullWidth
            size="small"
            placeholder="Search by case #, subject, customer, project, assignee…"
            value={filters.search}
            onChange={(e) => onChange({ ...filters, search: e.target.value })}
            slotProps={{
              input: {
                startAdornment: (
                  <InputAdornment position="start">
                    <Search size={16} />
                  </InputAdornment>
                ),
                endAdornment: filters.search ? (
                  <InputAdornment position="end">
                    <IconButton
                      size="small"
                      edge="end"
                      onClick={() => onChange({ ...filters, search: "" })}
                      aria-label="Clear search"
                    >
                      <X size={16} />
                    </IconButton>
                  </InputAdornment>
                ) : undefined,
              },
            }}
          />
        </Box>

        <Button
          variant="outlined"
          size="small"
          color="inherit"
          onClick={(e) => setSavedAnchor(e.currentTarget)}
          startIcon={<Bookmark size={16} />}
          endIcon={<ChevronDown size={16} />}
          aria-haspopup="true"
          aria-expanded={Boolean(savedAnchor)}
        >
          Saved views
        </Button>
        <Menu
          anchorEl={savedAnchor}
          open={Boolean(savedAnchor)}
          onClose={() => setSavedAnchor(null)}
          anchorOrigin={{ vertical: "bottom", horizontal: "left" }}
        >
          <MenuItem
            onClick={() => {
              setSavedAnchor(null);
              setSaveDialogOpen(true);
            }}
          >
            <ListItemIcon>
              <BookmarkPlus size={16} />
            </ListItemIcon>
            <ListItemText primary="Save current view…" />
          </MenuItem>
          <Divider />
          <ListSubheader sx={{ lineHeight: "32px" }}>Suggested</ListSubheader>
          {SUGGESTED_FILTER_VIEWS.map((v) => (
            <MenuItem
              key={`suggested-${v.name}`}
              selected={isActiveView(v.qs)}
              onClick={() => applyView(v.qs)}
            >
              <ListItemIcon>
                {isActiveView(v.qs) ? <Check size={16} /> : null}
              </ListItemIcon>
              <ListItemText primary={v.name} />
            </MenuItem>
          ))}
          <Divider />
          <ListSubheader sx={{ lineHeight: "32px" }}>Saved</ListSubheader>
          {savedViews.length === 0 ? (
            <MenuItem disabled>
              <ListItemText
                primary="No saved views yet"
                slotProps={{ primary: { variant: "body2" } }}
              />
            </MenuItem>
          ) : (
            savedViews.map((v) => (
              <MenuItem
                key={`saved-${v.name}`}
                selected={isActiveView(v.qs)}
                onClick={() => applyView(v.qs)}
              >
                <ListItemIcon>
                  {isActiveView(v.qs) ? <Check size={16} /> : null}
                </ListItemIcon>
                <ListItemText primary={v.name} />
                <IconButton
                  size="small"
                  edge="end"
                  aria-label={`Delete saved view ${v.name}`}
                  onClick={(e) => {
                    e.stopPropagation();
                    deleteFilterView(v.name);
                  }}
                  sx={{ ml: 1 }}
                >
                  <Trash2 size={15} />
                </IconButton>
              </MenuItem>
            ))
          )}
        </Menu>

        <Button
          variant="outlined"
          size="small"
          onClick={hasActive ? onReset : onFiltersToggle}
          startIcon={hasActive ? <X size={16} /> : <ListFilter size={16} />}
          endIcon={
            !hasActive &&
            (isFiltersOpen ? <ChevronUp size={16} /> : <ChevronDown size={16} />)
          }
        >
          {hasActive ? `Clear filters (${activeCount})` : "Filters"}
        </Button>
      </Box>

      {/* Fields with no bar control of their own (see `buildActiveFilterChips`'s
          doc comment) — shown regardless of `isFiltersOpen` so a
          dashboard-filtered arrival is self-explanatory even with the filter
          grid collapsed, and each is individually removable right here. */}
      {activeFilterChips.length > 0 && (
        <Box sx={{ display: "flex", flexWrap: "wrap", gap: 1 }}>
          {activeFilterChips.map((chip) => (
            <Chip
              key={chip.key}
              size="small"
              label={chip.label}
              onDelete={() => onChange(chip.onRemove(filters))}
            />
          ))}
        </Box>
      )}

      <Dialog
        open={saveDialogOpen}
        onClose={() => setSaveDialogOpen(false)}
        maxWidth="xs"
        fullWidth
      >
        <DialogTitle>Save current view</DialogTitle>
        <DialogContent>
          <TextField
            autoFocus
            fullWidth
            size="small"
            margin="dense"
            label="View name"
            placeholder="e.g. My open S1/S2"
            value={newViewName}
            onChange={(e) => setNewViewName(e.target.value)}
            onKeyDown={(e) => {
              if (e.key === "Enter") {
                e.preventDefault();
                handleSaveView();
              }
            }}
            helperText={
              activeCount === 0
                ? "Tip: no filters are active — this view will show all cases."
                : `Captures the ${activeCount} active filter${activeCount === 1 ? "" : "s"}.`
            }
          />
        </DialogContent>
        <DialogActions>
          <Button color="inherit" onClick={() => setSaveDialogOpen(false)}>
            Cancel
          </Button>
          <Button
            variant="contained"
            onClick={handleSaveView}
            disabled={!newViewName.trim()}
          >
            Save
          </Button>
        </DialogActions>
      </Dialog>

      {/* Collapsible filter grid. Severity / state / case type are fixed
          multi-selects; assignee / project are type-to-search Autocompletes. */}
      {isFiltersOpen && (
        <>
          <Divider />
          <Grid container spacing={2} sx={{ mt: 0 }}>
            {showSeverityFilter && (
              <Grid size={{ xs: 12, sm: 6, md: 4, lg: 2 }}>
                <MultiSelectField
                  id="cases-filter-severity"
                  label="Severity"
                  values={filters.severities}
                  options={severityOptions}
                  onChange={(next) => onChange({ ...filters, severities: next })}
                />
              </Grid>
            )}
            <Grid size={{ xs: 12, sm: 6, md: 4, lg: 2 }}>
              <MultiSelectField
                id="cases-filter-state"
                label="State"
                values={filters.states}
                options={stateOptions}
                // Work sub-state only applies when `work_in_progress` is the
                // *sole* selected state — with other states also selected the
                // work-state filter can't be applied server-side, so drop any
                // selected work states as soon as the selection stops being
                // exactly that one state.
                onChange={(next) =>
                  onChange({
                    ...filters,
                    states: next,
                    workStates:
                      next.length === 1 && next[0] === "work_in_progress"
                        ? filters.workStates
                        : [],
                  })
                }
              />
            </Grid>
            <Grid size={{ xs: 12, sm: 6, md: 4, lg: 2 }}>
              {/* CS team the case's project is scoped to (`creTeam`). Options
                  are `creGroupId`s (what the filter actually matches on);
                  labels are team display names, never the raw group-id
                  UUID. `workStates` has no bar control of its own now (it's
                  a narrow, rarely hand-picked sub-filter of "state") -- it
                  still round-trips losslessly via the URL/a saved view/a
                  dashboard click-through, surfaced as a removable chip
                  instead (see `buildActiveFilterChips`). */}
              <MultiSelectField
                id="cases-filter-cs-team"
                label="Team"
                values={filters.csTeams}
                options={teamOptions}
                onChange={(next) => onChange({ ...filters, csTeams: next })}
              />
            </Grid>
            {showEngagementTypeFilter && (
              <Grid size={{ xs: 12, sm: 6, md: 4, lg: 2 }}>
                <MultiSelectField
                  id="cases-filter-engagement-type"
                  label="Engagement type"
                  values={filters.engagementTypes}
                  options={engagementTypeOptions}
                  onChange={(next) => onChange({ ...filters, engagementTypes: next })}
                />
              </Grid>
            )}
            {!hideTypeFilter && (
              <Grid size={{ xs: 12, sm: 6, md: 4, lg: 2 }}>
                <MultiSelectField
                  id="cases-filter-type"
                  label={typeFilterLabel}
                  values={filters.caseTypes}
                  options={caseTypeOptions}
                  onChange={(next) => onChange({ ...filters, caseTypes: next })}
                />
              </Grid>
            )}
            <Grid size={{ xs: 12, sm: 6, md: 4, lg: 2 }}>
              {/* Email/`@me`-based picker; `useGetCsmCases` resolves the
                  selection to the UUIDs `/cases/search` expects (`@me` via the
                  app-wide current-user context, named engineers via
                  `/users/search`). Searches the directory as you type. */}
              <AsyncAssigneeMultiSelect
                values={filters.assignees}
                onChange={(next) => onChange({ ...filters, assignees: next })}
                nameSeed={assigneeNameSeed}
              />
            </Grid>
            {!hideProjectFilter && (
              <Grid size={{ xs: 12, sm: 6, md: 4, lg: 2 }}>
                <AsyncProjectMultiSelect
                  values={filters.projects}
                  onChange={(next) => onChange({ ...filters, projects: next })}
                  nameSeed={projectNameSeed}
                />
              </Grid>
            )}
            <Grid size={{ xs: 12, sm: 6, md: 4, lg: 2 }}>
              {/* Product family filter; the selected names map straight to
                  `productNames` (SN matches product.name, all versions). */}
              <ProductNameMultiSelect
                values={filters.productNames}
                onChange={(next) => onChange({ ...filters, productNames: next })}
              />
            </Grid>
            <Grid
              size={{ xs: 12, sm: 6, md: 4, lg: 2 }}
              sx={{ display: "flex", alignItems: "center", height: 40 }}
            >
              <FormControlLabel
                control={
                  <Checkbox
                    id="cases-filter-exclude-onboarding-in-progress"
                    size="small"
                    checked={filters.excludeOnboardingStatuses.includes(ONBOARDING_IN_PROGRESS)}
                    onChange={(e) =>
                      onChange({
                        ...filters,
                        excludeOnboardingStatuses: e.target.checked
                          ? [
                              ...filters.excludeOnboardingStatuses.filter(
                                (s) => s !== ONBOARDING_IN_PROGRESS,
                              ),
                              ONBOARDING_IN_PROGRESS,
                            ]
                          : filters.excludeOnboardingStatuses.filter(
                              (s) => s !== ONBOARDING_IN_PROGRESS,
                            ),
                      })
                    }
                  />
                }
                label="Hide onboarding in progress"
              />
            </Grid>
          </Grid>
          {activeCount > 0 && (
            <Box sx={{ display: "flex", justifyContent: "flex-end" }}>
              <Typography variant="caption" color="text.secondary">
                {activeCount} {activeCount === 1 ? "filter" : "filters"} active
              </Typography>
            </Box>
          )}
        </>
      )}
    </Paper>
  );
}
