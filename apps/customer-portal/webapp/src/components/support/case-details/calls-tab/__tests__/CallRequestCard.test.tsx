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

import { render, screen, fireEvent } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import type { CallRequest } from "@models/responses";
import CallRequestCard from "@case-details-calls/CallRequestCard";

const mockCall: CallRequest = {
  id: "call-1",
  case: { id: "case-1", label: "CS0438719" },
  reason: "Test notes for the call",
  preferredTimes: ["2024-10-29 14:00:00"],
  durationMin: 60,
  scheduleTime: "2024-11-05 14:00:00",
  createdOn: "2024-10-29 10:00:00",
  updatedOn: "2024-10-29 10:00:00",
  state: { id: "1", label: "Pending on WSO2" },
};

describe("CallRequestCard", () => {
  it("should render call request details correctly", () => {
    render(<CallRequestCard call={mockCall} />);

    expect(screen.getByText(/Call Request/i)).toBeInTheDocument();
    expect(screen.getByText(/Pending on WSO2/i)).toBeInTheDocument();
    expect(screen.getByText(/Test notes for the call/i)).toBeInTheDocument();
    expect(screen.getByText(/60 minutes/i)).toBeInTheDocument();
  });

  it("should format createdOn date correctly", () => {
    render(<CallRequestCard call={mockCall} />);
    expect(screen.getByText(/Requested on Oct 29/i)).toBeInTheDocument();
  });

  it("should show Reschedule and Cancel buttons for non-pending-on-customer state", () => {
    render(<CallRequestCard call={mockCall} />);
    expect(screen.getByRole("button", { name: /Reschedule/i })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: /Cancel/i })).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: /Approve/i })).not.toBeInTheDocument();
    expect(screen.queryByRole("button", { name: /Reject/i })).not.toBeInTheDocument();
  });

  it("should hide Reschedule and Cancel for Notes Pending state", () => {
    const notesPendingCall: CallRequest = {
      ...mockCall,
      state: { id: "7", label: "Notes Pending" },
    };
    render(
      <CallRequestCard
        call={notesPendingCall}
        onEditClick={vi.fn()}
        onDeleteClick={vi.fn()}
      />,
    );
    expect(screen.queryByRole("button", { name: /Reschedule/i })).not.toBeInTheDocument();
    expect(screen.queryByRole("button", { name: /Cancel/i })).not.toBeInTheDocument();
  });

  it("should hide Reschedule and Cancel for Customer Rejected state", () => {
    const customerRejectedCall: CallRequest = {
      ...mockCall,
      state: { id: "4", label: "Customer Rejected" },
    };
    render(
      <CallRequestCard
        call={customerRejectedCall}
        onEditClick={vi.fn()}
        onDeleteClick={vi.fn()}
      />,
    );
    expect(screen.queryByRole("button", { name: /Reschedule/i })).not.toBeInTheDocument();
    expect(screen.queryByRole("button", { name: /Cancel/i })).not.toBeInTheDocument();
  });

  it("should show Approve and Reject buttons for 'Pending on Customer' state", () => {
    const pendingOnCustomerCall: CallRequest = {
      ...mockCall,
      state: { id: "3", label: "Pending on Customer" },
    };
    const onApproveClick = vi.fn();
    const onRejectClick = vi.fn();
    render(
      <CallRequestCard
        call={pendingOnCustomerCall}
        onApproveClick={onApproveClick}
        onRejectClick={onRejectClick}
      />,
    );
    expect(screen.getByRole("button", { name: /Approve/i })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: /Reject/i })).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: /Reschedule/i })).not.toBeInTheDocument();
    expect(screen.queryByRole("button", { name: /Cancel/i })).not.toBeInTheDocument();
  });

  it("should call onApproveClick when Approve is clicked for 'Pending on Customer'", () => {
    const pendingOnCustomerCall: CallRequest = {
      ...mockCall,
      state: { id: "3", label: "Pending on Customer" },
    };
    const onApproveClick = vi.fn();
    render(<CallRequestCard call={pendingOnCustomerCall} onApproveClick={onApproveClick} />);
    fireEvent.click(screen.getByRole("button", { name: /Approve/i }));
    expect(onApproveClick).toHaveBeenCalledWith(pendingOnCustomerCall);
  });

  it("should call onRejectClick when Reject is clicked for 'Pending on Customer'", () => {
    const pendingOnCustomerCall: CallRequest = {
      ...mockCall,
      state: { id: "3", label: "Pending on Customer" },
    };
    const onRejectClick = vi.fn();
    render(<CallRequestCard call={pendingOnCustomerCall} onRejectClick={onRejectClick} />);
    fireEvent.click(screen.getByRole("button", { name: /Reject/i }));
    expect(onRejectClick).toHaveBeenCalledWith(pendingOnCustomerCall);
  });

  it("should display '--' for missing or nullish values", () => {
    const incompleteCall: CallRequest = {
      id: "call-incomplete",
      case: { id: "c1", label: "CS1" },
      reason: "",
      preferredTimes: [],
      durationMin: undefined,
      scheduleTime: "",
      createdOn: "",
      updatedOn: "",
      state: { id: "1", label: "" },
    };

    render(<CallRequestCard call={incompleteCall} />);

    expect(screen.getByText(/Requested on --/i)).toBeInTheDocument();
    expect(screen.getAllByText("--").length).toBeGreaterThan(0);
  });
});
