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

import { Box, Button, Typography } from "@wso2/oxygen-ui";
import { Calendar } from "@wso2/oxygen-ui-icons-react";
import type { JSX } from "react";
import { useCallback, useMemo, useState } from "react";
import TabBar from "@components/common/tab-bar/TabBar";
import UsageOverviewPanel from "@components/project-details/usage-metrics/UsageOverviewPanel";
import UsageEnvironmentProductsPanel from "@components/project-details/usage-metrics/UsageEnvironmentProductsPanel";
import {
  USAGE_METRICS_INNER_TABS,
  USAGE_TIME_RANGE_LABELS,
} from "@constants/usageMetricsConstants";
import type { UsageEnvironmentKind, UsageTimeRangePreset } from "@models/usageMetrics.types";

const TIME_RANGE_PRESETS: UsageTimeRangePreset[] = ["3m", "6m", "12m", "custom"];

/**
 * Usage & Metrics area: time range, inner environment tabs, overview and product drill-downs.
 *
 * @returns {JSX.Element} Full usage metrics experience for project details.
 */
export default function UsageAndMetricsTabContent(): JSX.Element {
  const [timeRange, setTimeRange] = useState<UsageTimeRangePreset>("3m");
  const [innerTab, setInnerTab] = useState<string>("um-overview");
  const [expandedEnvironmentIds, setExpandedEnvironmentIds] = useState<Set<string>>(
    () => new Set(),
  );
  const [expandedProductIds, setExpandedProductIds] = useState<Set<string>>(
    () => new Set(),
  );

  const timeLabel = USAGE_TIME_RANGE_LABELS[timeRange];

  const environmentFromTab = useMemo((): UsageEnvironmentKind | null => {
    if (innerTab === "um-production") {
      return "production";
    }
    if (innerTab === "um-test") {
      return "test";
    }
    if (innerTab === "um-development") {
      return "development";
    }
    return null;
  }, [innerTab]);

  const toggleEnvironment = useCallback((id: string) => {
    setExpandedEnvironmentIds((prev) => {
      const next = new Set(prev);
      if (next.has(id)) {
        next.delete(id);
      } else {
        next.add(id);
      }
      return next;
    });
  }, []);

  const toggleProduct = useCallback((productId: string) => {
    setExpandedProductIds((prev) => {
      const next = new Set(prev);
      if (next.has(productId)) {
        next.delete(productId);
      } else {
        next.add(productId);
      }
      return next;
    });
  }, []);

  return (
    <Box sx={{ display: "flex", flexDirection: "column", gap: 2 }}>
      <Box
        sx={{
          display: "flex",
          flexWrap: "wrap",
          alignItems: "center",
          justifyContent: "space-between",
          gap: 2,
          mb: 1,
        }}
      >
        <Box sx={{ display: "flex", flexWrap: "wrap", alignItems: "center", gap: 1.5 }}>
          <Calendar size={18} />
          <Typography variant="body2" sx={{ fontWeight: 600 }}>
            Time Range:
          </Typography>
          <Box sx={{ display: "flex", flexWrap: "wrap", gap: 1 }}>
            {TIME_RANGE_PRESETS.map((preset) => {
              const selected = timeRange === preset;
              return (
                <Button
                  key={preset}
                  size="small"
                  variant={selected ? "contained" : "outlined"}
                  color={selected ? "warning" : "inherit"}
                  onClick={() => setTimeRange(preset)}
                  sx={{ textTransform: "none", minWidth: 48 }}
                >
                  {preset === "3m"
                    ? "3M"
                    : preset === "6m"
                      ? "6M"
                      : preset === "12m"
                        ? "12M"
                        : "Custom"}
                </Button>
              );
            })}
          </Box>
        </Box>
        <Typography variant="body2" color="text.secondary">
          {timeLabel}
        </Typography>
      </Box>

      <TabBar
        tabs={USAGE_METRICS_INNER_TABS}
        activeTab={innerTab}
        onTabChange={setInnerTab}
        sx={{ mb: 1 }}
      />

      {innerTab === "um-overview" && (
        <UsageOverviewPanel
          expandedEnvironmentIds={expandedEnvironmentIds}
          onToggleEnvironment={toggleEnvironment}
        />
      )}

      {environmentFromTab != null && (
        <UsageEnvironmentProductsPanel
          environment={environmentFromTab}
          expandedProductIds={expandedProductIds}
          onToggleProduct={toggleProduct}
        />
      )}
    </Box>
  );
}
