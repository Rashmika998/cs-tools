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
// KIND, either express or implied. See the License for the
// specific language governing permissions and limitations
// under the License.

import {
  type InfiniteData,
  useInfiniteQuery,
  useQuery,
  type UseInfiniteQueryResult,
  type UseQueryResult,
} from "@tanstack/react-query";
import { useAsgardeo } from "@asgardeo/react";
import { useAuthApiClient } from "@api/useAuthApiClient";
import { useLogger } from "@hooks/useLogger";
import { ApiQueryKeys } from "@constants/apiConstants";
import type { DeployedProductSearchRequest } from "@models/requests";
import type {
  DeploymentProductItem,
  DeployedProductsResponsePayload,
  DeployedProductsResponse,
} from "@models/responses";
import { isDeployedProductsResponse } from "@models/responses";

const DEFAULT_PAGE_SIZE = 10;

export type FetchFn = (url: string, init?: RequestInit) => Promise<Response>;

async function postDeploymentProductsSearchPage(params: {
  deploymentId: string;
  request: DeployedProductSearchRequest | undefined;
  offset: number;
  limit: number;
  fetchFn: FetchFn;
  logger?: ReturnType<typeof useLogger>;
}): Promise<DeployedProductsResponsePayload> {
  const { deploymentId, request, offset, limit, fetchFn, logger } = params;

  const baseUrl = window.config?.CUSTOMER_PORTAL_BACKEND_BASE_URL;
  if (!baseUrl) {
    throw new Error("CUSTOMER_PORTAL_BACKEND_BASE_URL is not configured");
  }

  const requestUrl = `${baseUrl}/deployments/${deploymentId}/products/search`;
  const payload: DeployedProductSearchRequest = {
    ...(request ?? {}),
    pagination: {
      offset,
      limit,
      ...(request?.pagination ?? {}),
    },
  };

  const response = await fetchFn(requestUrl, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(payload),
  });

  logger?.debug?.(
    `[usePostDeploymentProductsSearch] Response status: ${response.status}`,
  );

  if (!response.ok) {
    throw new Error(
      `Error searching deployment products: ${response.statusText}`,
    );
  }

  return (await response.json()) as DeployedProductsResponsePayload;
}

function normalizeProductsPayload(
  payload: DeployedProductsResponsePayload,
): DeployedProductsResponse {
  if (Array.isArray(payload)) {
    return {
      deployedProducts: payload,
      totalRecords: payload.length,
      offset: 0,
      limit: payload.length,
    };
  }
  return payload;
}

export async function fetchDeploymentProductsAll(params: {
  deploymentId: string;
  request?: DeployedProductSearchRequest;
  pageSize?: number;
  fetchFn: FetchFn;
  logger?: ReturnType<typeof useLogger>;
}): Promise<DeploymentProductItem[]> {
  const {
    deploymentId,
    request,
    pageSize = DEFAULT_PAGE_SIZE,
    fetchFn,
    logger,
  } = params;

  const results: DeploymentProductItem[] = [];
  let offset = 0;
  let total = Number.POSITIVE_INFINITY;

  while (offset < total) {
    const payload = await postDeploymentProductsSearchPage({
      deploymentId,
      request,
      offset,
      limit: pageSize,
      fetchFn,
      logger,
    });
    const page = normalizeProductsPayload(payload);
    results.push(...(page.deployedProducts ?? []));

    total = page.totalRecords ?? results.length;
    offset = (page.offset ?? offset) + (page.limit ?? pageSize);

    if ((page.deployedProducts ?? []).length === 0) {
      break;
    }
  }

  return results;
}

export interface UsePostDeploymentProductsSearchInfiniteOptions {
  request?: DeployedProductSearchRequest;
  pageSize?: number;
  enabled?: boolean;
}

/**
 * Search products of a deployment with server pagination.
 * Use when the UI should load 10 first, then load more on scroll/pagination.
 *
 * @param {string} deploymentId - Deployment ID.
 * @param {UsePostDeploymentProductsSearchInfiniteOptions} [options] - request/pageSize/enabled.
 * @returns {UseInfiniteQueryResult<InfiniteData<DeployedProductsResponsePayload>, Error>} Infinite query result.
 */
export function usePostDeploymentProductsSearchInfinite(
  deploymentId: string,
  options?: UsePostDeploymentProductsSearchInfiniteOptions,
): UseInfiniteQueryResult<InfiniteData<DeployedProductsResponsePayload>, Error> {
  const logger = useLogger();
  const { isSignedIn, isLoading: isAuthLoading } = useAsgardeo();
  const authFetch = useAuthApiClient();
  const {
    request,
    pageSize = DEFAULT_PAGE_SIZE,
    enabled = true,
  } = options ?? {};

  return useInfiniteQuery<DeployedProductsResponsePayload, Error>({
    queryKey: [
      ApiQueryKeys.DEPLOYMENT_PRODUCTS,
      deploymentId,
      "search",
      request,
      pageSize,
    ],
    initialPageParam: 0,
    queryFn: ({ pageParam }) =>
      postDeploymentProductsSearchPage({
        deploymentId,
        request,
        offset: Number(pageParam) || 0,
        limit: pageSize,
        fetchFn: authFetch,
        logger,
      }),
    getNextPageParam: (lastPayload) => {
      const lastPage = normalizeProductsPayload(lastPayload);
      const total = lastPage.totalRecords ?? 0;
      const offset = lastPage.offset ?? 0;
      const limit = lastPage.limit ?? pageSize;
      const nextOffset = offset + limit;
      return nextOffset < total ? nextOffset : undefined;
    },
    enabled: enabled && !!deploymentId && isSignedIn && !isAuthLoading,
    staleTime: 5 * 60 * 1000,
  });
}

export interface UsePostDeploymentProductsSearchAllOptions {
  request?: DeployedProductSearchRequest;
  pageSize?: number;
  enabled?: boolean;
}

/**
 * Search products of a deployment and automatically fetch all pages until the end.
 * Use for classification/background logic where the full list is required.
 *
 * @param {string} deploymentId - Deployment ID.
 * @param {UsePostDeploymentProductsSearchAllOptions} [options] - request/pageSize/enabled.
 * @returns {UseQueryResult<DeploymentProductItem[], Error>} Full products list.
 */
export function usePostDeploymentProductsSearchAll(
  deploymentId: string,
  options?: UsePostDeploymentProductsSearchAllOptions,
): UseQueryResult<DeploymentProductItem[], Error> {
  const logger = useLogger();
  const { isSignedIn, isLoading: isAuthLoading } = useAsgardeo();
  const authFetch = useAuthApiClient();
  const {
    request,
    pageSize = DEFAULT_PAGE_SIZE,
    enabled = true,
  } = options ?? {};

  return useQuery<DeploymentProductItem[], Error>({
    queryKey: [
      ApiQueryKeys.DEPLOYMENT_PRODUCTS,
      deploymentId,
      "search-all",
      request,
      pageSize,
    ],
    queryFn: () =>
      fetchDeploymentProductsAll({
        deploymentId,
        request,
        pageSize,
        fetchFn: authFetch,
        logger,
      }),
    enabled: enabled && !!deploymentId && isSignedIn && !isAuthLoading,
    staleTime: 5 * 60 * 1000,
  });
}

export function extractDeploymentProducts(
  payload: DeployedProductsResponsePayload | undefined,
): DeploymentProductItem[] {
  if (!payload) return [];
  if (Array.isArray(payload)) return payload;
  if (isDeployedProductsResponse(payload)) return payload.deployedProducts ?? [];
  return [];
}

