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

import { CHANGE_REQUEST_STATE_ORDER } from "@constants/changeRequestConstants";
import { ChangeRequestStates, type ChangeRequestState } from "@constants/supportConstants";
import type { ChangeRequestDetails } from "@models/responses";
import type {
  ChangeRequestDecisionMode,
  ChangeRequestWorkflowStage,
} from "@/types/changeRequestTypes";

export function getChangeRequestDecisionMode(
  changeRequest?: ChangeRequestDetails | null,
): ChangeRequestDecisionMode {
  const state = changeRequest?.state?.label;
  if (state === ChangeRequestStates.CUSTOMER_APPROVAL) {
    return "customerApproval";
  }
  if (state === ChangeRequestStates.CUSTOMER_REVIEW) {
    return "customerReview";
  }
  return "none";
}

export function buildChangeRequestWorkflowStages(
  changeRequest?: ChangeRequestDetails | null,
): { workflowStages: ChangeRequestWorkflowStage[]; currentStateIndex: number } {
  if (!changeRequest) {
    return { workflowStages: [], currentStateIndex: -1 };
  }

  const currentState =
    (changeRequest.state?.label as ChangeRequestState) ||
    ChangeRequestStates.NEW;
  const { hasCustomerApproved, hasCustomerReviewed } = changeRequest;
  const currentIndex = CHANGE_REQUEST_STATE_ORDER.indexOf(currentState);

  return {
    currentStateIndex: currentIndex,
    workflowStages: [
      {
        name: ChangeRequestStates.NEW,
        description: "Change request created",
        completed: currentIndex > 0,
        current: currentState === ChangeRequestStates.NEW,
        disabled: false,
      },
      {
        name: ChangeRequestStates.ASSESS,
        description: "Technical assessment completed",
        completed: currentIndex > 1,
        current: currentState === ChangeRequestStates.ASSESS,
        disabled: false,
      },
      {
        name: ChangeRequestStates.AUTHORIZE,
        description: "Internal authorization obtained",
        completed: currentIndex > 2,
        current: currentState === ChangeRequestStates.AUTHORIZE,
        disabled: false,
      },
      {
        name: ChangeRequestStates.CUSTOMER_APPROVAL,
        description: "Customer approval received",
        completed: currentIndex > 3 && hasCustomerApproved,
        current: currentState === ChangeRequestStates.CUSTOMER_APPROVAL,
        disabled:
          (currentState === ChangeRequestStates.IMPLEMENT ||
            currentState === ChangeRequestStates.REVIEW) &&
          !hasCustomerApproved,
      },
      {
        name: ChangeRequestStates.SCHEDULED,
        description: "Maintenance window scheduled",
        completed: currentIndex > 4,
        current: currentState === ChangeRequestStates.SCHEDULED,
        disabled: false,
      },
      {
        name: ChangeRequestStates.IMPLEMENT,
        description: "Change implementation",
        completed: currentIndex > 5,
        current: currentState === ChangeRequestStates.IMPLEMENT,
        disabled: false,
      },
      {
        name: ChangeRequestStates.REVIEW,
        description: "Internal review",
        completed: currentIndex > 6,
        current: currentState === ChangeRequestStates.REVIEW,
        disabled: false,
      },
      {
        name: ChangeRequestStates.CUSTOMER_REVIEW,
        description: "Customer validation",
        completed: currentIndex > 7 && hasCustomerReviewed,
        current: currentState === ChangeRequestStates.CUSTOMER_REVIEW,
        disabled:
          (currentState === ChangeRequestStates.ROLLBACK ||
            currentState === ChangeRequestStates.CLOSED ||
            currentState === ChangeRequestStates.CANCELED) &&
          !hasCustomerReviewed,
      },
      {
        name: ChangeRequestStates.ROLLBACK,
        description: "Change rollback if needed",
        completed: false,
        current: currentState === ChangeRequestStates.ROLLBACK,
        disabled:
          currentState === ChangeRequestStates.CLOSED ||
          currentState === ChangeRequestStates.CANCELED,
      },
      {
        name: ChangeRequestStates.CLOSED,
        description: "Change request completed",
        completed: false,
        current: currentState === ChangeRequestStates.CLOSED,
        disabled:
          currentState === ChangeRequestStates.CANCELED ||
          currentState === ChangeRequestStates.ROLLBACK,
      },
      {
        name: ChangeRequestStates.CANCELED,
        description: "Change request canceled",
        completed: false,
        current: currentState === ChangeRequestStates.CANCELED,
        disabled:
          currentState === ChangeRequestStates.CLOSED ||
          currentState === ChangeRequestStates.ROLLBACK,
      },
    ],
  };
}
