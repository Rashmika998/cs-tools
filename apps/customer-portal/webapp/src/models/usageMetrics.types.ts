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

/** Preset time range for usage charts (placeholder until API exists). */
export type UsageTimeRangePreset = "3m" | "6m" | "12m" | "custom";

/** Environment bucket for product-level usage. */
export type UsageEnvironmentKind = "production" | "test" | "development";

/** Single point for Recharts / LineChart x-axis rows. */
export interface UsageTrendRow {
  name: string;
  value?: number;
  current?: number;
  average?: number;
}

/** Summary counts on the overview tab. */
export interface UsageOverviewSummary {
  environments: number;
  products: number;
  instances: number;
}

/** Product row inside an expanded environment on the overview tab. */
export interface UsageOverviewProductCard {
  id: string;
  name: string;
  version: string;
  instances: number;
  cores: number;
  transactionsLabel: string;
}

/** Collapsible environment row on the overview tab. */
export interface UsageEnvironmentBreakdownRow {
  id: string;
  kind: UsageEnvironmentKind;
  title: string;
  subtitle: string;
  totalCores: number;
  transactionsLabel: string;
  products: UsageOverviewProductCard[];
}

/** Aggregated metric card on the overview tab. */
export interface UsageAggregatedMetricDefinition {
  id: string;
  title: string;
  caption: string;
  headlineValue: string;
  deltaLabel: string;
  stroke: string;
  data: UsageTrendRow[];
  secondaryStroke?: string;
  secondaryName?: string;
}

/** Mini line chart shown when an instance row is expanded. */
export interface UsageInstanceChartBlock {
  title: string;
  caption: string;
  headlineValue: string;
  deltaLabel: string;
  deltaPositive: boolean;
  stroke: string;
  data: UsageTrendRow[];
}

/** Per-instance row when a product is expanded (header + optional drill-down charts). */
export interface UsageProductInstanceRow {
  id: string;
  hostName: string;
  javaVersion: string;
  u2Level: string;
  transactionsLabel: string;
  coreCount: number;
  charts: {
    transactions: UsageInstanceChartBlock;
    cores: UsageInstanceChartBlock;
  };
}

/** Core metric pill labels on the environment product row. */
export interface UsageCoreMetricStat {
  label: string;
  value: string;
}

/** Expandable product within Production / Test / Development tabs. */
export interface UsageEnvironmentProduct {
  id: string;
  name: string;
  version: string;
  runningInstances: number;
  transactionsLabel: string;
  thirdMetricLabel: string;
  thirdMetricValue: string;
  coreMetrics: UsageCoreMetricStat[];
  transactionTrend: UsageTrendRow[];
  coreUsageTrend: UsageTrendRow[];
  instances: UsageProductInstanceRow[];
}
