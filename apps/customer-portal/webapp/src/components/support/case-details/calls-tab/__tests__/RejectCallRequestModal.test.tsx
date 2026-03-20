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
import RejectCallRequestModal from "@case-details-calls/RejectCallRequestModal";

const mockCall: CallRequest = {
  id: "call-1",
  case: { id: "case-1", label: "CS0438719" },
  reason: "Test notes",
  preferredTimes: [],
  durationMin: 30,
  scheduleTime: "",
  createdOn: "2024-10-29 10:00:00",
  updatedOn: "2024-10-29 10:00:00",
  state: { id: "3", label: "Pending on Customer" },
};

const defaultProps = {
  open: true,
  call: mockCall,
  onClose: vi.fn(),
  onConfirm: vi.fn(),
};

describe("RejectCallRequestModal", () => {
  it("should render with title and reason input", () => {
    render(<RejectCallRequestModal {...defaultProps} />);
    expect(screen.getByText("Reject Call Request")).toBeInTheDocument();
    expect(
      screen.getByPlaceholderText(/Enter reason for rejection/i),
    ).toBeInTheDocument();
  });

  it("should not render when open is false", () => {
    render(<RejectCallRequestModal {...defaultProps} open={false} />);
    expect(screen.queryByText("Reject Call Request")).not.toBeInTheDocument();
  });

  it("should have Reject button disabled when reason is empty", () => {
    render(<RejectCallRequestModal {...defaultProps} />);
    const rejectButtons = screen.getAllByRole("button", { name: /^Reject$/i });
    expect(rejectButtons.at(-1)).toBeDisabled();
  });

  it("should enable Reject button when reason is entered", () => {
    render(<RejectCallRequestModal {...defaultProps} />);
    fireEvent.change(screen.getByPlaceholderText(/Enter reason for rejection/i), {
      target: { value: "Not available" },
    });
    const rejectButtons = screen.getAllByRole("button", { name: /^Reject$/i });
    expect(rejectButtons.at(-1)).not.toBeDisabled();
  });

  it("should call onConfirm with trimmed reason when Reject is clicked", () => {
    const onConfirm = vi.fn();
    render(<RejectCallRequestModal {...defaultProps} onConfirm={onConfirm} />);
    fireEvent.change(screen.getByPlaceholderText(/Enter reason for rejection/i), {
      target: { value: "  Not available  " },
    });
    fireEvent.click(screen.getAllByRole("button", { name: /^Reject$/i }).at(-1)!);
    expect(onConfirm).toHaveBeenCalledWith("Not available");
  });

  it("should call onClose when Go Back is clicked", () => {
    const onClose = vi.fn();
    render(<RejectCallRequestModal {...defaultProps} onClose={onClose} />);
    fireEvent.click(screen.getByRole("button", { name: /Go Back/i }));
    expect(onClose).toHaveBeenCalled();
  });

  it("should disable all buttons when isRejecting is true", () => {
    render(<RejectCallRequestModal {...defaultProps} isRejecting />);
    expect(screen.getByRole("button", { name: /Rejecting.../i })).toBeDisabled();
    expect(screen.getByRole("button", { name: /Go Back/i })).toBeDisabled();
  });

  it("should show scheduled time in description when scheduleTime is provided", () => {
    render(
      <RejectCallRequestModal
        {...defaultProps}
        call={{ ...mockCall, scheduleTime: "2024-11-05 14:00:00" }}
      />,
    );
    expect(
      screen.getByText(/Are you sure you want to reject the call request scheduled for/i),
    ).toBeInTheDocument();
  });
});
