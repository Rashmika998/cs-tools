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

import { ListingTable, Divider, Box } from "@wso2/oxygen-ui";
import { type JSX, useState, useMemo, useEffect } from "react";
import { useNavigate } from "react-router";
import { useAsgardeo } from "@asgardeo/react";
import useGetProjectCases from "@api/useGetProjectCases";
import { useGetProjectCasesPage } from "@api/useGetProjectCasesPage";
import useGetProjectFilters from "@api/useGetProjectFilters";
import { useGetDeployments } from "@api/useGetDeployments";
import type { FilterField } from "@components/common/filter-panel/FilterPopover";
import CasesTableHeader from "@components/dashboard/cases-table/CasesTableHeader";
import CasesFilters from "@components/dashboard/cases-table/CasesFilters";
import CasesList from "@components/dashboard/cases-table/CasesList";
import { mapSeverityToDisplay, isS0Case } from "@utils/support";
import { isS0SeverityLabel } from "@constants/dashboardConstants";
import type { CaseListItem, CaseSearchResponse } from "@models/responses";

const OUTSTANDING_STATUS_IDS = [1, 10, 18, 1003, 1006] as const;

interface CasesTableProps {
  projectId: string;
  excludeS0?: boolean;
}

const CasesTable = ({
  projectId,
  excludeS0 = false,
}: CasesTableProps): JSX.Element => {
  const navigate = useNavigate();
  const { isLoading: isAuthLoading } = useAsgardeo();
  const [filters, setFilters] = useState<Record<string, any>>({});
  const [isFilterOpen, setIsFilterOpen] = useState(false);
  const [page, setPage] = useState(0);
  const [rowsPerPage, setRowsPerPage] = useState(5);
  const [showAll, setShowAll] = useState(false);
  const [isLoadingAll, setIsLoadingAll] = useState(false);

  const {
    data: filtersMetadata,
  } = useGetProjectFilters(projectId);

  // Fetch deployments for the deployment filter
  const { data: deploymentsData } = useGetDeployments(projectId);

  const severityOptions = (filtersMetadata?.severities ?? [])
    .filter((s) => !excludeS0 || !isS0SeverityLabel(s.label))
    .map((s) => ({
      label: mapSeverityToDisplay(s.label),
      value: s.id,
    }));

  const dynamicFilterFields: FilterField[] = [
    {
      id: "statusId",
      label: "Status",
      type: "select",
      options: (
        filtersMetadata?.caseStates ??
        filtersMetadata?.statuses ??
        []
      ).map((s) => ({ label: s.label, value: s.id })),
    },
    {
      id: "severityId",
      label: "Severity",
      type: "select",
      options: severityOptions,
    },
    {
      id: "issueTypes",
      label: "Category",
      type: "select",
      options:
        filtersMetadata?.issueTypes?.map((t) => ({
          label: t.label,
          value: t.id,
        })) || [],
    },
    {
      id: "deploymentId",
      label: "Deployment",
      type: "select",
      options:
        deploymentsData?.deployments?.map((d) => ({
          label: d.type?.label || d.name,
          value: d.id,
        })) || [],
    },
  ];

  const caseSearchRequest = useMemo(
    () => ({
      filters: {
        statusId: filters.statusId ? Number(filters.statusId) : undefined,
        statusIds: filters.statusId ? undefined : [...OUTSTANDING_STATUS_IDS],
        severityId: filters.severityId ? Number(filters.severityId) : undefined,
        issueId: filters.issueTypes ? Number(filters.issueTypes) : undefined,
        deploymentId: filters.deploymentId || undefined,
      },
      sortBy: {
        field: "createdOn",
        order: "desc" as const,
      },
    }),
    [filters],
  );

  const offset = page * rowsPerPage;
  const limit = rowsPerPage;

  const pageQuery = useGetProjectCasesPage(
    projectId,
    caseSearchRequest,
    offset,
    limit,
    { enabled: !showAll },
  );

  const infiniteQuery = useGetProjectCases(projectId, caseSearchRequest, {
    enabled: showAll,
  });

  useEffect(() => {
    if (showAll && infiniteQuery.hasNextPage && !infiniteQuery.isFetchingNextPage && isLoadingAll) {
      void infiniteQuery.fetchNextPage();
    } else if (showAll && !infiniteQuery.hasNextPage && isLoadingAll) {
      setIsLoadingAll(false);
    }
  }, [
    showAll,
    infiniteQuery.hasNextPage,
    infiniteQuery.isFetchingNextPage,
    infiniteQuery.fetchNextPage,
    isLoadingAll,
    infiniteQuery.data,
  ]);

  const paginatedData = useMemo((): CaseSearchResponse => {
    const filterS0 = (items: CaseListItem[]): CaseListItem[] =>
      excludeS0 ? items.filter((c) => !isS0Case(c)) : items;

    if (showAll) {
      if (!infiniteQuery.data) {
        return { cases: [], totalRecords: 0, offset: 0, limit: 0 };
      }
      const rawCases = infiniteQuery.data.pages.flatMap((p) => p.cases ?? []);
      const cases = filterS0(rawCases);
      const totalRecords = excludeS0
        ? cases.length
        : (infiniteQuery.data.pages[0]?.totalRecords ?? cases.length);
      return {
        cases,
        totalRecords,
        offset: 0,
        limit: cases.length,
      };
    }
    const pageData = pageQuery.data;
    const rawCases = (pageData?.cases ?? []) as CaseListItem[];
    const cases = filterS0(rawCases);
    const totalRecords = excludeS0
      ? cases.length
      : (pageData?.totalRecords ?? 0);
    return { cases, totalRecords, offset, limit };
  }, [showAll, infiniteQuery.data, pageQuery.data, offset, limit, excludeS0]);

  const isFetchingCases = showAll
    ? isLoadingAll || infiniteQuery.isFetching || infiniteQuery.isFetchingNextPage
    : pageQuery.isFetching;
  const isError = showAll ? infiniteQuery.isError : pageQuery.isError;

  const handleClearFilters = () => {
    setFilters({});
    setPage(0);
    setIsLoadingAll(false);
  };

  const handleUpdateFilter = (field: string, value: any) => {
    setFilters((prev) => ({
      ...prev,
      [field]: value,
    }));
    setPage(0);
    setShowAll(false);
    setIsLoadingAll(false);
  };

  const handleChangePage = (_event: unknown, newPage: number) => {
    setPage(newPage);
  };

  const handleChangeRowsPerPage = (
    event: React.ChangeEvent<HTMLInputElement>,
  ) => {
    setRowsPerPage(parseInt(event.target.value, 10));
    setPage(0);
  };

  const getActiveFiltersCount = () => {
    return Object.values(filters).filter((v) => v !== "" && v != null).length;
  };

  return (
    <ListingTable.Container sx={{ width: "100%", mb: 4, p: 3 }}>
      {/* Header */}
      <CasesTableHeader
        activeFiltersCount={getActiveFiltersCount()}
        isFiltersOpen={isFilterOpen}
        onFilterToggle={() => {
          if (getActiveFiltersCount() > 0) {
            handleClearFilters();
          } else {
            setIsFilterOpen(!isFilterOpen);
          }
        }}
      />

      {/* Filter dropdowns section */}
      {isFilterOpen && (
        <>
          <Divider sx={{ my: 2 }} />
          <Box sx={{ mb: 3 }}>
            <CasesFilters
              filters={filters}
              filterFields={dynamicFilterFields as any}
              onFilterChange={handleUpdateFilter}
            />
          </Box>
        </>
      )}

      {/* Cases list */}
      <CasesList
        isLoading={isFetchingCases || isAuthLoading}
        isError={isError}
        data={paginatedData}
        page={page}
        rowsPerPage={rowsPerPage}
        onPageChange={handleChangePage}
        onRowsPerPageChange={handleChangeRowsPerPage}
        onCaseClick={(c) => navigate(`/projects/${projectId}/support/cases/${c.id}`)}
        showPagination={!showAll}
      />
    </ListingTable.Container>
  );
};

export default CasesTable;
