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
import { describe, expect, it, vi } from "vitest";
import type { CallRequest } from "@models/responses";
import DeleteCallRequestModal from "@case-details-calls/DeleteCallRequestModal";

const mockCall: CallRequest = {
  id: "call-1",
  case: { id: "case-1", label: "CS0438719" },
  reason: "Cancel",
  preferredTimes: ["2026-04-02 15:45:00"],
  durationMin: 30,
  scheduleTime: "",
  createdOn: "2026-04-02 10:00:00",
  updatedOn: "2026-04-02 10:00:00",
  state: { id: "3", label: "Scheduled" },
};

describe("DeleteCallRequestModal", () => {
  it("should show converted preferred time in confirmation text", () => {
    render(
      <DeleteCallRequestModal
        open
        call={mockCall}
        userTimeZone="America/New_York"
        onClose={vi.fn()}
        onConfirm={vi.fn()}
      />,
    );
    expect(
      screen.getByText(/scheduled for Apr 2, 11:45 AM/i),
    ).toBeInTheDocument();
  });

  it("should render required reason label with a single required indicator", () => {
    render(
      <DeleteCallRequestModal
        open
        call={mockCall}
        userTimeZone="America/New_York"
        onClose={vi.fn()}
        onConfirm={vi.fn()}
      />,
    );
    expect(screen.getByLabelText(/Reason/)).toBeInTheDocument();
    expect(screen.queryByText(/\*\s*\*/)).not.toBeInTheDocument();
  });
});

