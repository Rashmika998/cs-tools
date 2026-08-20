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

import { fireEvent, render, screen } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { beforeEach, describe, expect, it, vi } from "vitest";
import "@testing-library/jest-dom/vitest";
import type { ReactNode } from "react";
import CasesFilterBar, {
  type CasesFilters,
} from "@features/csm-cases/components/CasesFilterBar";
import { DEFAULT_CASES_FILTERS } from "@features/csm-cases/utils/casesFiltersUrl";

const postMock = vi.fn();

vi.mock("@api/backend/client", () => ({
  useBackendApi: () => ({ post: postMock, get: vi.fn() }),
}));

vi.mock("@config/apiConfig", () => ({
  apiConfig: { backendUrl: "https://example.test" },
}));

function renderBar(
  filters: CasesFilters,
  onChange = vi.fn(),
  extraProps: Partial<Parameters<typeof CasesFilterBar>[0]> = {},
): { onChange: ReturnType<typeof vi.fn> } {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  });
  render(
    <QueryClientProvider client={queryClient}>
      <CasesFilterBar
        filters={filters}
        onChange={onChange}
        onReset={() => {}}
        isFiltersOpen
        onFiltersToggle={() => {}}
        availableAssigneeUsers={[]}
        availableProjects={[]}
        {...extraProps}
      />
    </QueryClientProvider> as ReactNode,
  );
  return { onChange };
}

describe("CasesFilterBar — active-filter chips for URL-only fields", () => {
  beforeEach(() => {
    postMock.mockReset();
    postMock.mockResolvedValue({ teams: [] });
  });

  it("renders no chips when nothing is active", () => {
    renderBar({ ...DEFAULT_CASES_FILTERS });
    expect(screen.queryByText(/SLA/)).not.toBeInTheDocument();
    expect(screen.queryByText(/Escalat/)).not.toBeInTheDocument();
    expect(screen.queryByText(/CS team:/)).not.toBeInTheDocument();
    expect(screen.queryByText(/^Tag:/)).not.toBeInTheDocument();
  });

  it("renders one chip per URL-only filter and each is independently removable", () => {
    const { onChange } = renderBar({
      ...DEFAULT_CASES_FILTERS,
      slaElapsedPctGte: 80,
      hasEscalation: true,
      onboardingStatuses: ["in_progress"],
      createdOnGte: "2026-07-27",
    });

    expect(screen.getByText("SLA ≥ 80%")).toBeInTheDocument();
    expect(screen.getByText("Escalated")).toBeInTheDocument();
    expect(screen.getByText("Onboarding: In progress")).toBeInTheDocument();
    expect(screen.getByText(/Created after/)).toBeInTheDocument();

    // Removing the SLA chip clears only slaElapsedPctGte, nothing else.
    const slaChip = screen.getByText("SLA ≥ 80%");
    const deleteIcon = slaChip.parentElement?.querySelector(
      '[data-testid="CancelIcon"], svg',
    );
    fireEvent.click(deleteIcon ?? slaChip);

    expect(onChange).toHaveBeenCalledWith(
      expect.objectContaining({
        slaElapsedPctGte: null,
        hasEscalation: true,
        onboardingStatuses: ["in_progress"],
        createdOnGte: "2026-07-27",
      }),
    );
  });

  it("renders a date-only bound as the same local calendar date, not shifted by UTC parsing", () => {
    renderBar({
      ...DEFAULT_CASES_FILTERS,
      createdOnGte: "2026-07-27",
    });

    // A bare YYYY-MM-DD bound must render as that same calendar date
    // regardless of the runner's local timezone offset from UTC — pinning
    // this to a fixed local Date (not `new Date("2026-07-27")`, which is
    // parsed as UTC midnight and can roll back a day) is what makes this
    // assertion timezone-safe.
    const expected = new Date(2026, 6, 27).toLocaleDateString();
    expect(
      screen.getByText(`Created after ${expected}`),
    ).toBeInTheDocument();
  });

  it("clearing one onboarding-status chip removes only that value, keeping siblings", () => {
    const { onChange } = renderBar({
      ...DEFAULT_CASES_FILTERS,
      onboardingStatuses: ["in_progress", "OnHold"],
    });

    expect(screen.getByText("Onboarding: In progress")).toBeInTheDocument();
    expect(screen.getByText("Onboarding: On hold")).toBeInTheDocument();

    const chip = screen.getByText("Onboarding: In progress");
    const deleteIcon = chip.parentElement?.querySelector("svg");
    fireEvent.click(deleteIcon ?? chip);

    expect(onChange).toHaveBeenCalledWith(
      expect.objectContaining({ onboardingStatuses: ["OnHold"] }),
    );
  });

  it("both SLA bounds render and clear independently", () => {
    const { onChange } = renderBar({
      ...DEFAULT_CASES_FILTERS,
      slaElapsedPctGte: 80,
      slaElapsedPctLte: 100,
    });

    expect(screen.getByText("SLA ≥ 80%")).toBeInTheDocument();
    expect(screen.getByText("SLA ≤ 100%")).toBeInTheDocument();

    const chip = screen.getByText("SLA ≤ 100%");
    const deleteIcon = chip.parentElement?.querySelector("svg");
    fireEvent.click(deleteIcon ?? chip);

    expect(onChange).toHaveBeenCalledWith(
      expect.objectContaining({ slaElapsedPctGte: 80, slaElapsedPctLte: null }),
    );
  });
});


describe("CasesFilterBar — removed bar controls fall back to chips", () => {
  beforeEach(() => {
    postMock.mockReset();
  });


  /**
   * The tag bar controls (and, briefly, CS team's) were removed as clutter,
   * so a chip is the ONLY way these filters are visible or clearable after a
   * dashboard click-through. If these break, a user lands on a filtered
   * list with no way to see or undo why. CS team has its own "Team" bar
   * control again (see the describe block below) and is deliberately NOT
   * chipped, to avoid showing the same selection twice.
   */
  it("renders chips for tags/excludeTags now that their bar controls are gone", () => {
    const { onChange } = renderBar({
      ...DEFAULT_CASES_FILTERS,
      tags: ["micro-gw"],
      excludeTags: ["s_dip"],
    });

    expect(screen.getByText("Tag: micro-gw")).toBeInTheDocument();
    expect(screen.getByText("Excluding tag: s_dip")).toBeInTheDocument();

    fireEvent.click(screen.getByText("Excluding tag: s_dip").closest(".MuiChip-root")!.querySelector("svg")!);
    expect(onChange).toHaveBeenCalledWith(
      expect.objectContaining({ excludeTags: [], tags: ["micro-gw"] }),
    );
  });

  it("does not render a chip for csTeams — it has its own 'Team' bar control", () => {
    renderBar({ ...DEFAULT_CASES_FILTERS, csTeams: ["g1"] });
    expect(screen.queryByText(/^CS team:/)).not.toBeInTheDocument();
  });

  it("no longer renders the removed Tags / Exclude tags bar controls", () => {
    renderBar({ ...DEFAULT_CASES_FILTERS });
    expect(screen.queryByLabelText(/^Tags$/)).not.toBeInTheDocument();
    expect(screen.queryByLabelText(/^Exclude tags$/)).not.toBeInTheDocument();
  });

  // Regression: reported live against a dashboard widget's `state`/
  // `projectOnboardingStatus notIn` click-through, which showed an INCLUDE
  // chip (the exact opposite of the widget's own filter) because the
  // exclusion had nowhere of its own to render. `excludeStates` has no bar
  // control (same as `excludeTags`), so its chip is the only way to see or
  // clear it. `excludeOnboardingStatuses` is the same for any value OTHER
  // than "In-Progress" -- that one specific value gets its own checkbox
  // (tested separately below).
  it("renders distinct chips for excludeStates/excludeOnboardingStatuses, never conflated with their include counterparts", () => {
    const { onChange } = renderBar({
      ...DEFAULT_CASES_FILTERS,
      states: ["open"],
      excludeStates: ["closed"],
      onboardingStatuses: ["completed"],
      excludeOnboardingStatuses: ["On-Hold"],
    });

    expect(screen.getByText("Excluding state: Closed")).toBeInTheDocument();
    expect(screen.getByText("Onboarding: Completed")).toBeInTheDocument();
    expect(screen.getByText("Excluding onboarding: On-hold")).toBeInTheDocument();

    fireEvent.click(
      screen.getByText("Excluding onboarding: On-hold").closest(".MuiChip-root")!
        .querySelector("svg")!,
    );
    expect(onChange).toHaveBeenCalledWith(
      expect.objectContaining({
        excludeOnboardingStatuses: [],
        onboardingStatuses: ["completed"],
        excludeStates: ["closed"],
      }),
    );
  });
});

describe("CasesFilterBar — 'Team' control (replaces the removed 'Work state' one)", () => {
  beforeEach(() => {
    postMock.mockReset();
    postMock.mockResolvedValue({
      teams: [
        { id: "abt-1", name: "ABT One", family: "cre-abt", creGroupId: "g-1" },
        { id: "abt-2", name: "ABT Two", family: "cre-abt", creGroupId: "g-2" },
        // No creGroupId configured -- must not appear as a selectable option.
        { id: "abt-3", name: "ABT Three", family: "cre-abt" },
      ],
    });
  });

  it("renders team display names as options, backed by creGroupId (what the filter actually matches on)", async () => {
    renderBar({ ...DEFAULT_CASES_FILTERS });

    fireEvent.mouseDown(screen.getByRole("combobox", { name: "Team" }));
    expect(await screen.findByRole("option", { name: "ABT One" })).toBeInTheDocument();
    expect(screen.getByRole("option", { name: "ABT Two" })).toBeInTheDocument();
    expect(screen.queryByRole("option", { name: "ABT Three" })).not.toBeInTheDocument();
  });

  it("selecting a team sets csTeams to its creGroupId, not its registry id", async () => {
    const { onChange } = renderBar({ ...DEFAULT_CASES_FILTERS });

    fireEvent.mouseDown(screen.getByRole("combobox", { name: "Team" }));
    fireEvent.click(await screen.findByRole("option", { name: "ABT One" }));

    expect(onChange).toHaveBeenCalledWith(expect.objectContaining({ csTeams: ["g-1"] }));
  });
});

describe("CasesFilterBar — 'Hide onboarding in progress' checkbox", () => {
  beforeEach(() => {
    postMock.mockReset();
  });

  it("is unchecked by default and checking it sets excludeOnboardingStatuses to exactly [\"In-Progress\"]", () => {
    const { onChange } = renderBar({ ...DEFAULT_CASES_FILTERS });
    const checkbox = screen.getByRole("checkbox", { name: /hide onboarding in progress/i });
    expect(checkbox).not.toBeChecked();

    fireEvent.click(checkbox);

    expect(onChange).toHaveBeenCalledWith(
      expect.objectContaining({ excludeOnboardingStatuses: ["In-Progress"] }),
    );
  });

  it("is checked when excludeOnboardingStatuses already contains \"In-Progress\", and unchecking it clears just that value", () => {
    const { onChange } = renderBar({
      ...DEFAULT_CASES_FILTERS,
      excludeOnboardingStatuses: ["In-Progress"],
    });
    const checkbox = screen.getByRole("checkbox", { name: /hide onboarding in progress/i });
    expect(checkbox).toBeChecked();
    // The checkbox is the only representation of this value now -- no
    // redundant chip alongside it.
    expect(screen.queryByText(/Excluding onboarding/)).not.toBeInTheDocument();

    fireEvent.click(checkbox);

    expect(onChange).toHaveBeenCalledWith(
      expect.objectContaining({ excludeOnboardingStatuses: [] }),
    );
  });

  it("checking it preserves any other excludeOnboardingStatuses value already set (e.g. from a dashboard click-through)", () => {
    const { onChange } = renderBar({
      ...DEFAULT_CASES_FILTERS,
      excludeOnboardingStatuses: ["On-Hold"],
    });
    const checkbox = screen.getByRole("checkbox", { name: /hide onboarding in progress/i });

    fireEvent.click(checkbox);

    expect(onChange).toHaveBeenCalledWith(
      expect.objectContaining({ excludeOnboardingStatuses: ["On-Hold", "In-Progress"] }),
    );
  });
});

describe("CasesFilterBar — work state has no bar control, only a chip", () => {
  beforeEach(() => {
    postMock.mockReset();
  });

  it("no longer renders a Work state bar control", () => {
    renderBar({ ...DEFAULT_CASES_FILTERS, states: ["work_in_progress"] });
    expect(screen.queryByRole("combobox", { name: "Work state" })).not.toBeInTheDocument();
  });

  it("renders a removable chip when workStates is set (e.g. from a saved view or dashboard click-through)", () => {
    const { onChange } = renderBar({
      ...DEFAULT_CASES_FILTERS,
      states: ["work_in_progress"],
      workStates: ["ongoing"],
    });

    expect(screen.getByText("Work state: Ongoing")).toBeInTheDocument();

    fireEvent.click(
      screen.getByText("Work state: Ongoing").closest(".MuiChip-root")!.querySelector("svg")!,
    );
    expect(onChange).toHaveBeenCalledWith(expect.objectContaining({ workStates: [] }));
  });

  // The underlying invariant (workStates only means something when
  // work_in_progress is the *sole* selected state) still matters even
  // without a bar control to disable -- a stale workStates value from a
  // saved view/URL must not survive the State control widening past
  // work_in_progress alone.
  it("clears workStates when a second state is added alongside work_in_progress", () => {
    const { onChange } = renderBar({
      ...DEFAULT_CASES_FILTERS,
      states: ["work_in_progress"],
      workStates: ["ongoing"],
    });

    fireEvent.mouseDown(screen.getByRole("combobox", { name: "State" }));
    fireEvent.click(screen.getByRole("option", { name: "Open" }));

    expect(onChange).toHaveBeenCalledWith(
      expect.objectContaining({
        states: ["work_in_progress", "open"],
        workStates: [],
      }),
    );
  });
});

describe("CasesFilterBar — case-type control label", () => {
  beforeEach(() => {
    postMock.mockReset();
  });

  it("defaults the case-type control's label to \"Case type\"", () => {
    renderBar({ ...DEFAULT_CASES_FILTERS });
    expect(screen.getByLabelText("Case type")).toBeInTheDocument();
  });

  it("renders a caller-supplied typeFilterLabel instead (e.g. a project's mixed work-items view)", () => {
    renderBar({ ...DEFAULT_CASES_FILTERS }, vi.fn(), {
      typeFilterLabel: "Work item type",
    });
    expect(screen.getByLabelText("Work item type")).toBeInTheDocument();
    expect(screen.queryByLabelText("Case type")).not.toBeInTheDocument();
  });

  it("hides the control entirely when hideTypeFilter is set, regardless of typeFilterLabel", () => {
    renderBar({ ...DEFAULT_CASES_FILTERS }, vi.fn(), {
      hideTypeFilter: true,
      typeFilterLabel: "Work item type",
    });
    expect(screen.queryByLabelText("Work item type")).not.toBeInTheDocument();
    expect(screen.queryByLabelText("Case type")).not.toBeInTheDocument();
  });
});
