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

import {
  ChartColumn,
  Code2,
  Factory,
  FlaskConical,
  Layers,
  Package,
  Server,
} from "@wso2/oxygen-ui-icons-react";
import type { TabOption } from "@components/common/tab-bar/TabBar";
import type { SupportStatConfig } from "@constants/supportConstants";
import type { UsageTimeRangePreset } from "@models/usageMetrics.types";

export {
  USAGE_AGGREGATED_METRICS,
  USAGE_ENVIRONMENT_BREAKDOWN,
  USAGE_ENVIRONMENT_PRODUCTS,
  USAGE_OVERVIEW_SUMMARY,
} from "@models/mockData";

export const USAGE_TIME_RANGE_LABELS: Record<UsageTimeRangePreset, string> = {
  "3m": "Last 3 Months",
  "6m": "Last 6 Months",
  "12m": "Last 12 Months",
  custom: "Custom range",
};

export const USAGE_METRICS_INNER_TABS: TabOption[] = [
  { id: "um-overview", label: "Overview", icon: ChartColumn },
  { id: "um-production", label: "Production", icon: Factory },
  { id: "um-test", label: "Test", icon: FlaskConical },
  { id: "um-development", label: "Development", icon: Code2 },
];

/** Keys for SupportStatGrid on the usage overview tab. */
export type UsageOverviewStatKey = "environments" | "products" | "instances";

export const USAGE_OVERVIEW_STAT_CONFIGS: SupportStatConfig<UsageOverviewStatKey>[] =
  [
    {
      key: "environments",
      label: "Environments",
      icon: Layers,
      iconColor: "info",
    },
    {
      key: "products",
      label: "Products",
      icon: Package,
      iconColor: "primary",
    },
    {
      key: "instances",
      label: "Instances",
      icon: Server,
      iconColor: "success",
    },
  ];

/** Symmetric chart inset inside usage metric cards (matches dashboard card padding rhythm). */
export const USAGE_LINE_CHART_MARGIN = {
  top: 8,
  right: 8,
  bottom: 24,
  left: 8,
} as const;
