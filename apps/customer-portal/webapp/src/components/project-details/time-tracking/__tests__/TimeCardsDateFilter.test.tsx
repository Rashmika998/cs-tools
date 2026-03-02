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

import { render, screen } from "@testing-library/react";
import { describe, it, expect, vi } from "vitest";
import userEvent from "@testing-library/user-event";
import TimeCardsDateFilter from "@time-tracking/TimeCardsDateFilter";

describe("TimeCardsDateFilter", () => {
  const mockTimeCardStates = [
    { id: "Pending", label: "Pending" },
    { id: "Submitted", label: "Submitted" },
    { id: "Approved", label: "Approved" },
  ];

  it("should render filter by date range label and date inputs", () => {
    render(
      <TimeCardsDateFilter
        startDate="2025-01-01"
        endDate="2025-12-31"
        onStartDateChange={() => {}}
        onEndDateChange={() => {}}
        state=""
        onStateChange={() => {}}
        timeCardStates={mockTimeCardStates}
      />,
    );

    expect(
      screen.getByText("Filter by Date Range:"),
    ).toBeInTheDocument();
    expect(
      screen.getByText("Filter by State:"),
    ).toBeInTheDocument();
  });

  it("should show state dropdown with all options and allow selection", async () => {
    const user = userEvent.setup();
    const onStateChange = vi.fn();
    
    render(
      <TimeCardsDateFilter
        startDate="2025-01-01"
        endDate="2025-12-31"
        onStartDateChange={() => {}}
        onEndDateChange={() => {}}
        state=""
        onStateChange={onStateChange}
        timeCardStates={mockTimeCardStates}
      />,
    );

    expect(screen.getByText("Filter by State:")).toBeInTheDocument();
    
    // Find the Select component
    const stateSelect = screen.getByRole("combobox", { name: /filter by state/i });
    expect(stateSelect).toBeInTheDocument();
    
    // Click to open the dropdown
    await user.click(stateSelect);
    
    // Check that all state options are present
    expect(screen.getByRole("option", { name: "All States" })).toBeInTheDocument();
    expect(screen.getByRole("option", { name: "Pending" })).toBeInTheDocument();
    expect(screen.getByRole("option", { name: "Submitted" })).toBeInTheDocument();
    expect(screen.getByRole("option", { name: "Approved" })).toBeInTheDocument();
    
    // Select a state
    await user.click(screen.getByRole("option", { name: "Pending" }));
    expect(onStateChange).toHaveBeenCalledWith("Pending");
  });
});
