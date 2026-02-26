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

import { useParams, useNavigate } from "react-router";
import {
  useState,
  useMemo,
  useEffect,
  type JSX,
  type ChangeEvent,
} from "react";
import { useLoader } from "@context/linear-loader/LoaderContext";
import {
  Box,
  Button,
  Stack,
  Typography,
  Pagination,
} from "@wso2/oxygen-ui";
import { ArrowLeft, Plus } from "@wso2/oxygen-ui-icons-react";
import useGetProjectCases from "@api/useGetProjectCases";
import { CaseType } from "@constants/supportConstants";
import ServiceRequestsList from "@components/support/service-requests/ServiceRequestsList";
import ServiceRequestsSearchBar from "@components/support/service-requests/ServiceRequestsSearchBar";

const SERVICE_REQUEST_TYPE_LABEL = "Service Request";

export type ServiceRequestStatusFilter =
  | "all"
  | "pending"
  | "in_progress"
  | "completed";

/**
 * ServiceRequestsPage displays all service requests with filters and search.
 *
 * @returns {JSX.Element} The rendered Service Requests page.
 */
export default function ServiceRequestsPage(): JSX.Element {
  const navigate = useNavigate();
  const { projectId } = useParams<{ projectId: string }>();

  const [searchTerm, setSearchTerm] = useState("");
  const [statusFilter, setStatusFilter] =
    useState<ServiceRequestStatusFilter>("all");
  const [page, setPage] = useState(1);
  const pageSize = 10;

  const caseSearchRequest = useMemo(
    () => ({
      filters: {
        caseTypes: [CaseType.SERVICE_REQUEST],
        searchQuery: searchTerm.trim() || undefined,
      },
      sortBy: {
        field: "createdOn" as const,
        order: "desc" as const,
      },
    }),
    [searchTerm],
  );

  const {
    data,
    isLoading: isCasesQueryLoading,
    hasNextPage,
    fetchNextPage,
  } = useGetProjectCases(projectId || "", caseSearchRequest);

  const { showLoader, hideLoader } = useLoader();

  const hasCasesResponse = data !== undefined;
  const isCasesAreaLoading =
    isCasesQueryLoading || (!!projectId && !hasCasesResponse);

  useEffect(() => {
    if (isCasesAreaLoading) {
      showLoader();
      return () => hideLoader();
    }
    hideLoader();
  }, [isCasesAreaLoading, showLoader, hideLoader]);

  useEffect(() => {
    if (!data || !hasNextPage) {
      return;
    }
    void fetchNextPage();
  }, [data, hasNextPage, fetchNextPage]);

  const allServiceRequests = useMemo(
    () => data?.pages.flatMap((p) => p.cases) ?? [],
    [data],
  );

  const filteredServiceRequests = useMemo(() => {
    let filtered = [...allServiceRequests];

    if (searchTerm.trim()) {
      const searchLower = searchTerm.toLowerCase();
      filtered = filtered.filter(
        (sr) =>
          sr.number?.toLowerCase().includes(searchLower) ||
          sr.title?.toLowerCase().includes(searchLower) ||
          sr.description?.toLowerCase().includes(searchLower),
      );
    }

    if (statusFilter !== "all") {
      filtered = filtered.filter((sr) => {
        const statusLabel = sr.status?.label?.toLowerCase() ?? "";
        switch (statusFilter) {
          case "pending":
            return (
              statusLabel.includes("open") ||
              statusLabel.includes("awaiting") ||
              statusLabel.includes("waiting")
            );
          case "in_progress":
            return statusLabel.includes("progress");
          case "completed":
            return (
              statusLabel.includes("closed") ||
              statusLabel.includes("resolved")
            );
          default:
            return true;
        }
      });
    }

    filtered.sort((a, b) => {
      const dateA = new Date(a.createdOn).getTime() || 0;
      const dateB = new Date(b.createdOn).getTime() || 0;
      return dateB - dateA;
    });
      
    return filtered;
  }, [allServiceRequests, searchTerm, statusFilter]);

  const totalItems = filteredServiceRequests.length;

  const paginatedServiceRequests = useMemo(() => {
    const startIndex = (page - 1) * pageSize;
    return filteredServiceRequests.slice(startIndex, startIndex + pageSize);
  }, [filteredServiceRequests, page]);

  const totalPages = Math.ceil(totalItems / pageSize);

  const handlePageChange = (_event: ChangeEvent<unknown>, value: number) => {
    setPage(value);
  };

  const handleStatusFilterChange = (value: ServiceRequestStatusFilter) => {
    setStatusFilter(value);
    setPage(1);
  };

  const handleSearchChange = (value: string) => {
    setSearchTerm(value);
    setPage(1);
  };

  const handleNewServiceRequest = () => {
    navigate(`/${projectId}/support/chat/create-case`, {
      state: { caseType: SERVICE_REQUEST_TYPE_LABEL },
    });
  };

  return (
    <Stack spacing={3}>
      <Box
        sx={{
          display: "flex",
          justifyContent: "space-between",
          alignItems: "flex-start",
        }}
      >
        <Box>
          <Button
            startIcon={<ArrowLeft size={16} />}
            onClick={() => navigate("..")}
            sx={{ mb: 2 }}
            variant="text"
          >
            Back to Support Center
          </Button>
          <Typography variant="h4" color="text.primary" sx={{ mb: 1 }}>
            Service Requests
          </Typography>
          <Typography variant="body2" color="text.secondary">
            Manage deployments, and operations, infrastructure change, and
            service configurations
          </Typography>
        </Box>
        <Button
          variant="contained"
          color="primary"
          startIcon={<Plus size={16} />}
          onClick={handleNewServiceRequest}
          sx={{ mt: 4 }}
        >
          New Service Request
        </Button>
      </Box>

      <ServiceRequestsSearchBar
        searchTerm={searchTerm}
        onSearchChange={handleSearchChange}
        statusFilter={statusFilter}
        onStatusFilterChange={handleStatusFilterChange}
      />

      <Box
        sx={{
          display: "flex",
          justifyContent: "space-between",
          alignItems: "center",
        }}
      >
        <Typography variant="body2" color="text.secondary">
          Showing {paginatedServiceRequests.length} of {totalItems} service
          requests
        </Typography>
        
      </Box>

      <ServiceRequestsList
        serviceRequests={paginatedServiceRequests}
        isLoading={isCasesAreaLoading}
        onServiceRequestClick={(sr) =>
          navigate(`/${projectId}/support/cases/${sr.id}`)
        }
      />

      {totalPages > 1 && (
        <Box sx={{ display: "flex", justifyContent: "flex-end", mt: 4 }}>
          <Pagination
            count={totalPages}
            page={page}
            onChange={handlePageChange}
            color="primary"
            variant="outlined"
            shape="rounded"
          />
        </Box>
      )}
    </Stack>
  );
}
