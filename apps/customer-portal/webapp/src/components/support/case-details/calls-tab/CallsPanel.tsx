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

import { Box, Button, Stack } from "@wso2/oxygen-ui";
import { PhoneCall } from "@wso2/oxygen-ui-icons-react";
import { useState, useCallback, useEffect, useMemo, type JSX } from "react";
import type { CallRequest } from "@models/responses";
import { useGetCallRequests } from "@api/useGetCallRequests";
import { usePatchCallRequest } from "@api/usePatchCallRequest";
import useGetUserDetails from "@api/useGetUserDetails";
import useGetProjectFilters from "@api/useGetProjectFilters";
import CallsListSkeleton from "@case-details-calls/CallsListSkeleton";
import CallRequestList from "@case-details-calls/CallRequestList";
import CallsEmptyState from "@case-details-calls/CallsEmptyState";
import CallsErrorState from "@case-details-calls/CallsErrorState";
import RequestCallModal from "@case-details-calls/RequestCallModal";
import DeleteCallRequestModal from "@case-details-calls/DeleteCallRequestModal";
import RejectCallRequestModal from "@case-details-calls/RejectCallRequestModal";
import ApproveCallRequestModal from "@case-details-calls/ApproveCallRequestModal";
import MissingTimezoneDialog from "@case-details-calls/MissingTimezoneDialog";
import UserProfileModal from "@components/common/header/UserProfileModal";
import ErrorBanner from "@components/common/error-banner/ErrorBanner";
import SuccessBanner from "@components/common/success-banner/SuccessBanner";
import { CALL_REQUEST_STATE_CANCELLED } from "@constants/supportConstants";
import { ERROR_BANNER_TIMEOUT_MS } from "@constants/errorBannerConstants";

export interface CallsPanelProps {
  projectId: string;
  caseId: string;
  isCaseClosed?: boolean;
}

/**
 * CallsPanel displays call requests for a specific case.
 *
 * @param {CallsPanelProps} props - The project and case identifiers.
 * @returns {JSX.Element} The rendered calls panel.
 */
export default function CallsPanel({
  projectId,
  caseId,
  isCaseClosed = false,
}: CallsPanelProps): JSX.Element {
  const [isModalOpen, setIsModalOpen] = useState(false);
  const [editCall, setEditCall] = useState<CallRequest | null>(null);
  const [deleteCall, setDeleteCall] = useState<CallRequest | null>(null);
  const [approveCall, setApproveCall] = useState<CallRequest | null>(null);
  const [rejectCall, setRejectCall] = useState<CallRequest | null>(null);
  const [isMissingTzDialogOpen, setIsMissingTzDialogOpen] = useState(false);
  const [isProfileModalOpen, setIsProfileModalOpen] = useState(false);
  const [hasShownTzPrompt, setHasShownTzPrompt] = useState(false);
  const [successMessage, setSuccessMessage] = useState<string | null>(null);
  const [errorMessage, setErrorMessage] = useState<string | null>(null);

  const { data: projectFilters } = useGetProjectFilters(projectId);
  const { data: userDetails } = useGetUserDetails();
  const userTimeZone = userDetails?.timeZone || undefined;

  // Derive filtered state keys from project filters (all non-Canceled states for search body)
  const callRequestStateKeys = useMemo<number[] | undefined>(() => {
    if (!projectFilters?.callRequestStates) return undefined;
    return projectFilters.callRequestStates
      .map((s) => Number(s.id))
      .filter((n) => !Number.isNaN(n));
  }, [projectFilters]);

  // Derive specific state keys from filter labels for Approve / Reject / Cancel
  const approveStateKey = useMemo<number | undefined>(() => {
    const found = projectFilters?.callRequestStates?.find((s) =>
      s.label.toLowerCase().includes("pending on wso2"),
    );
    return found ? Number(found.id) : undefined;
  }, [projectFilters]);

  const rejectStateKey = useMemo<number | undefined>(() => {
    const found = projectFilters?.callRequestStates?.find((s) =>
      s.label.toLowerCase().includes("customer rejected") ||
      s.label.toLowerCase().includes("rejected"),
    );
    return found ? Number(found.id) : undefined;
  }, [projectFilters]);

  // Canceled state key from the raw filter (before filtering out Canceled)
  const cancelStateKey = useMemo<number>(() => {
    // useGetProjectFilters already removes Canceled from callRequestStates,
    // so fall back to the constant which matches the backend value.
    return CALL_REQUEST_STATE_CANCELLED;
  }, []);

  const {
    data,
    isPending,
    isError,
    refetch,
    fetchNextPage,
    hasNextPage,
    isFetchingNextPage,
  } = useGetCallRequests(projectId, caseId, callRequestStateKeys);
  const patchCallRequest = usePatchCallRequest(projectId, caseId);

  const callRequests = data?.pages?.flatMap((p) => p.callRequests ?? []) ?? [];

  const handleOpenModal = () => {
    setEditCall(null);
    setSuccessMessage(null);
    setErrorMessage(null);
    setIsModalOpen(true);
  };
  const handleCloseModal = () => {
    setIsModalOpen(false);
    setEditCall(null);
  };
  const handleEditClick = (call: CallRequest) => {
    setEditCall(call);
    setIsModalOpen(true);
  };
  const handleDeleteClick = useCallback((call: CallRequest) => {
    setDeleteCall(call);
  }, []);
  const handleCloseDeleteModal = useCallback(() => {
    setDeleteCall(null);
    setErrorMessage(null);
  }, []);

  const handleApproveClick = useCallback((call: CallRequest) => {
    setApproveCall(call);
  }, []);
  const handleCloseApproveModal = useCallback(() => {
    setApproveCall(null);
  }, []);

  const handleRejectClick = useCallback((call: CallRequest) => {
    setRejectCall(call);
  }, []);
  const handleCloseRejectModal = useCallback(() => {
    setRejectCall(null);
    setErrorMessage(null);
  }, []);
  const handleConfirmReject = useCallback(
    (reason: string) => {
      if (!rejectCall) return;
      patchCallRequest.mutate(
        {
          callRequestId: rejectCall.id,
          reason: reason.trim(),
          stateKey: rejectStateKey ?? CALL_REQUEST_STATE_CANCELLED,
        },
        {
          onSuccess: () => {
            handleCloseRejectModal();
            setSuccessMessage("Call request rejected.");
            refetch();
          },
          onError: (error) => {
            handleCloseRejectModal();
            setErrorMessage(error.message || "Failed to reject call request.");
          },
        },
      );
    },
    [rejectCall, rejectStateKey, patchCallRequest, handleCloseRejectModal, refetch],
  );

  const handleConfirmDelete = useCallback(
    (reason: string) => {
      if (!deleteCall) return;
      patchCallRequest.mutate(
        {
          callRequestId: deleteCall.id,
          reason: reason.trim(),
          stateKey: cancelStateKey,
        },
      {
        onSuccess: () => {
          handleCloseDeleteModal();
          setSuccessMessage("Call request cancelled successfully.");
          refetch();
        },
        onError: (error) => {
          handleCloseDeleteModal();
          setErrorMessage(error.message || "Failed to cancel call request.");
        },
      },
    );
    },
    [deleteCall, cancelStateKey, patchCallRequest, handleCloseDeleteModal, refetch],
  );
  const handleSuccess = useCallback(() => {
    setSuccessMessage("Call request submitted successfully.");
    refetch();
  }, [refetch]);
  const handleError = useCallback((message: string) => {
    setErrorMessage(message);
  }, []);

  useEffect(() => {
    if (!successMessage) return;
    const t = setTimeout(() => setSuccessMessage(null), ERROR_BANNER_TIMEOUT_MS);
    return () => clearTimeout(t);
  }, [successMessage]);

  useEffect(() => {
    if (!errorMessage) return;
    const t = setTimeout(() => setErrorMessage(null), ERROR_BANNER_TIMEOUT_MS);
    return () => clearTimeout(t);
  }, [errorMessage]);

  // Prompt once per mount when the user has no timezone set and data is loaded
  useEffect(() => {
    if (hasShownTzPrompt) return;
    if (userDetails && !userDetails.timeZone) {
      setIsMissingTzDialogOpen(true);
      setHasShownTzPrompt(true);
    }
  }, [userDetails, hasShownTzPrompt]);

  const requestCallButton = (
    <Button
      variant="contained"
      color="primary"
      startIcon={<PhoneCall size={16} />}
      onClick={handleOpenModal}
      disabled={isCaseClosed}
    >
      Request Call
    </Button>
  );

  return (
    <Stack spacing={3}>
      {successMessage && (
        <SuccessBanner
          message={successMessage}
          onClose={() => setSuccessMessage(null)}
        />
      )}
      {errorMessage && (
        <ErrorBanner
          message={errorMessage}
          onClose={() => setErrorMessage(null)}
        />
      )}

      {!(callRequests.length === 0 && !isPending && !isError) && (
        <Box sx={{ alignSelf: "flex-start" }}>{requestCallButton}</Box>
      )}

      {isPending ? (
        <CallsListSkeleton />
      ) : isError ? (
        <CallsErrorState />
      ) : callRequests.length === 0 ? (
        <CallsEmptyState action={requestCallButton} />
      ) : (
        <>
          <CallRequestList
            requests={callRequests}
            userTimeZone={userTimeZone}
            onEditClick={handleEditClick}
            onDeleteClick={handleDeleteClick}
            onApproveClick={handleApproveClick}
            onRejectClick={handleRejectClick}
          />
          {hasNextPage && (
            <Button
              variant="outlined"
              onClick={() => fetchNextPage()}
              disabled={isFetchingNextPage}
              sx={{ alignSelf: "flex-start" }}
            >
              {isFetchingNextPage ? "Loading..." : "Load more"}
            </Button>
          )}
        </>
      )}

      <DeleteCallRequestModal
        open={!!deleteCall}
        call={deleteCall}
        onClose={handleCloseDeleteModal}
        onConfirm={handleConfirmDelete}
        isDeleting={patchCallRequest.isPending}
        userTimeZone={userTimeZone}
      />

      <RequestCallModal
        open={isModalOpen}
        projectId={projectId}
        caseId={caseId}
        onClose={handleCloseModal}
        onSuccess={handleSuccess}
        onError={handleError}
        editCall={editCall ?? undefined}
        userTimeZone={userTimeZone}
      />

      <ApproveCallRequestModal
        open={!!approveCall}
        call={approveCall}
        projectId={projectId}
        caseId={caseId}
        onClose={handleCloseApproveModal}
        onSuccess={() => {
          handleCloseApproveModal();
          setSuccessMessage("Call request approved successfully.");
          refetch();
        }}
        onError={(message) => setErrorMessage(message)}
        userTimeZone={userTimeZone}
        approveStateKey={approveStateKey}
      />

      <RejectCallRequestModal
        open={!!rejectCall}
        call={rejectCall}
        onClose={handleCloseRejectModal}
        onConfirm={handleConfirmReject}
        isRejecting={patchCallRequest.isPending}
        userTimeZone={userTimeZone}
      />

      <MissingTimezoneDialog
        open={isMissingTzDialogOpen}
        onClose={() => setIsMissingTzDialogOpen(false)}
        onSetTimeZone={() => {
          setIsMissingTzDialogOpen(false);
          setIsProfileModalOpen(true);
        }}
      />

      <UserProfileModal
        open={isProfileModalOpen}
        onClose={() => setIsProfileModalOpen(false)}
      />
    </Stack>
  );
}
