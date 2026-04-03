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
import { describe, it, expect } from "vitest";
import ServiceHoursAllocationsCard from "@components/project-details/project-overview/service-hours-allocations/ServiceHoursAllocationsCard";

describe("ServiceHoursAllocationsCard", () => {
  it("should render Service Hours Allocations title", () => {
    render(
      <ServiceHoursAllocationsCard project={null} isLoading={false} />,
    );

    expect(
      screen.getByText("Service Hours Allocations"),
    ).toBeInTheDocument();
  });

  it("should render Query Hours and Onboarding Hours sections", () => {
    render(
      <ServiceHoursAllocationsCard
        project={{
          id: "1",
          consumedQueryHours: 45,
          totalQueryHours: 100,
          remainingQueryHours: 55,
          consumedOnboardingHours: 180,
          totalOnboardingHours: 200,
          remainingOnboardingHours: 20,
          onboardingExpiryDate: "2026-04-30",
        } as any}
        isLoading={false}
      />,
    );

    expect(screen.getByText("Query Hours")).toBeInTheDocument();
    expect(screen.getByText("Onboarding Hours")).toBeInTheDocument();
    expect(
      screen.getByText("45 hrs/100 hrs (45%)"),
    ).toBeInTheDocument();
    expect(
      screen.getByText("180 hrs/200 hrs (90%)"),
    ).toBeInTheDocument();
    expect(screen.getByText("55 hrs")).toBeInTheDocument();
    expect(screen.getByText("20 hrs")).toBeInTheDocument();
    expect(screen.getByText("Apr 30, 2026")).toBeInTheDocument();
  });

  it("should display Not Available when values are missing", () => {
    render(
      <ServiceHoursAllocationsCard
        project={null}
        isLoading={false}
      />,
    );

    const notAvailable = screen.getAllByText("Not Available");
    expect(notAvailable.length).toBeGreaterThan(0);
  });
});
