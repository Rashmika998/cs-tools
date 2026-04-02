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
  Accordion,
  AccordionDetails,
  AccordionSummary,
  Box,
  Card,
  Grid,
  Typography,
  alpha,
  colors,
} from "@wso2/oxygen-ui";
import {
  ChevronDown,
  Factory,
  FlaskConical,
  Code2,
  Package,
} from "@wso2/oxygen-ui-icons-react";
import type { JSX } from "react";
import { useMemo } from "react";
import type {
  UsageEnvironmentBreakdownRow,
  UsageEnvironmentKind,
} from "@models/usageMetrics.types";
import {
  USAGE_AGGREGATED_METRICS,
  USAGE_ENVIRONMENT_BREAKDOWN,
  USAGE_OVERVIEW_STAT_CONFIGS,
  USAGE_OVERVIEW_SUMMARY,
  type UsageOverviewStatKey,
} from "@constants/usageMetricsConstants";
import SupportStatGrid from "@components/common/stat-grid/SupportStatGrid";
import UsageMetricTrendCard from "@components/project-details/usage-metrics/UsageMetricTrendCard";

function accentForEnvironment(kind: UsageEnvironmentKind): {
  title: string;
  main: string;
  border: string;
  headerBg: string;
  headerHoverBg: string;
  iconWellBg: string;
  iconColor: string;
  statTileBg: string;
} {
  const main =
    kind === "production"
      ? (colors.orange?.[600] ?? "#EA580C")
      : kind === "test"
        ? (colors.blue?.[600] ?? "#2563EB")
        : (colors.green?.[600] ?? "#16A34A");
  const title =
    kind === "production"
      ? (colors.orange?.[800] ?? "#9A3412")
      : kind === "test"
        ? (colors.blue?.[800] ?? "#1E40AF")
        : (colors.green?.[800] ?? "#166534");

  return {
    main,
    title,
    border: alpha(main, 0.2),
    headerBg: alpha(main, 0.08),
    headerHoverBg: alpha(main, 0.12),
    iconWellBg: alpha(main, 0.15),
    iconColor: main,
    statTileBg: alpha(main, 0.08),
  };
}

function EnvironmentIcon({ kind }: { kind: UsageEnvironmentKind }): JSX.Element {
  const color = accentForEnvironment(kind).iconColor;
  if (kind === "production") {
    return <Factory size={20} color={color} />;
  }
  if (kind === "test") {
    return <FlaskConical size={20} color={color} />;
  }
  return <Code2 size={20} color={color} />;
}

interface EnvironmentBreakdownAccordionProps {
  row: UsageEnvironmentBreakdownRow;
  expanded: boolean;
  onToggle: () => void;
}

function EnvironmentBreakdownAccordion({
  row,
  expanded,
  onToggle,
}: EnvironmentBreakdownAccordionProps): JSX.Element {
  const a = accentForEnvironment(row.kind);

  return (
    <Accordion
      expanded={expanded}
      onChange={() => onToggle()}
      disableGutters
      elevation={0}
      sx={{
        "&:before": { display: "none" },
        boxShadow: 1,
        overflow: "hidden",
        border: "1px solid",
        borderColor: a.border,
        borderRadius: 0,
        bgcolor: "background.paper",
        "&.Mui-expanded": { margin: 0 },
      }}
    >
      <AccordionSummary
        expandIcon={
          <Box sx={{ display: "flex", alignItems: "center" }}>
            <ChevronDown
              size={20}
              className="usage-env-accordion-chevron"
              color={a.iconColor}
              style={{ transition: "transform 0.2s ease" }}
            />
          </Box>
        }
        sx={{
          px: 2,
          py: 1.5,
          minHeight: 56,
          bgcolor: a.headerBg,
          borderRadius: 0,
          "&:hover": {
            bgcolor: a.headerHoverBg,
          },
          "& .MuiAccordionSummary-expandIconWrapper.Mui-expanded": {
            "& .usage-env-accordion-chevron": {
              transform: "rotate(180deg)",
            },
          },
        }}
      >
        <Box
          sx={{
            display: "flex",
            alignItems: "center",
            justifyContent: "space-between",
            width: "100%",
            pr: 1,
          }}
        >
          <Box sx={{ display: "flex", alignItems: "center", gap: 2 }}>
            <Box
              sx={{
                p: 1,
                bgcolor: a.iconWellBg,
                display: "flex",
                alignItems: "center",
                justifyContent: "center",
              }}
            >
              <EnvironmentIcon kind={row.kind} />
            </Box>
            <Box sx={{ textAlign: "left" }}>
              <Typography variant="subtitle1" sx={{ fontWeight: 600, color: a.title }}>
                {row.title}
              </Typography>
              <Typography variant="body2" color="text.secondary">
                {row.subtitle}
              </Typography>
            </Box>
          </Box>
          <Box
            sx={{
              display: { xs: "none", sm: "flex" },
              alignItems: "center",
              gap: 3,
            }}
          >
            <Box sx={{ textAlign: "right" }}>
              <Typography variant="caption" color="text.secondary">
                Total Cores
              </Typography>
              <Typography variant="body2" sx={{ fontWeight: 600 }}>
                {row.totalCores}
              </Typography>
            </Box>
            <Box sx={{ textAlign: "right" }}>
              <Typography variant="caption" color="text.secondary">
                Transactions
              </Typography>
              <Typography variant="body2" sx={{ fontWeight: 600 }}>
                {row.transactionsLabel}
              </Typography>
            </Box>
          </Box>
        </Box>
      </AccordionSummary>
      <AccordionDetails
        sx={{
          px: 2,
          py: 2,
          bgcolor: "background.paper",
          borderTop: 1,
          borderColor: "divider",
          borderRadius: 0,
        }}
      >
        <Grid container spacing={2}>
          {row.products.map((product) => (
            <Grid key={product.id} size={{ xs: 12, md: 6, lg: 4 }}>
              <Card
                sx={{
                  p: 2,
                  height: "100%",
                  border: "1px solid",
                  borderColor: alpha(a.main, 0.15),
                }}
              >
                <Box sx={{ display: "flex", alignItems: "center", gap: 1.5, mb: 1.5 }}>
                  <Box
                    sx={{
                      p: 1,
                      bgcolor: a.iconWellBg,
                      display: "flex",
                      alignItems: "center",
                    }}
                  >
                    <Package size={20} color={a.iconColor} />
                  </Box>
                  <Box sx={{ minWidth: 0, flex: 1 }}>
                    <Typography variant="subtitle2" noWrap sx={{ fontWeight: 600 }}>
                      {product.name}
                    </Typography>
                    <Typography variant="caption" color="text.secondary">
                      {product.version}
                    </Typography>
                  </Box>
                </Box>
                <Grid container spacing={1} sx={{ mb: 1 }}>
                  <Grid size={6}>
                    <Box sx={{ bgcolor: a.statTileBg, p: 1.5 }}>
                      <Typography variant="caption" color="text.secondary">
                        Instances
                      </Typography>
                      <Typography variant="h6" sx={{ fontWeight: 600 }}>
                        {product.instances}
                      </Typography>
                    </Box>
                  </Grid>
                  <Grid size={6}>
                    <Box sx={{ bgcolor: a.statTileBg, p: 1.5 }}>
                      <Typography variant="caption" color="text.secondary">
                        Cores
                      </Typography>
                      <Typography variant="h6" sx={{ fontWeight: 600 }}>
                        {product.cores}
                      </Typography>
                    </Box>
                  </Grid>
                </Grid>
                <Box sx={{ bgcolor: a.statTileBg, p: 1.5 }}>
                  <Typography variant="caption" color="text.secondary">
                    Total Transactions
                  </Typography>
                  <Typography variant="h6" sx={{ fontWeight: 600 }}>
                    {product.transactionsLabel}
                  </Typography>
                </Box>
              </Card>
            </Grid>
          ))}
        </Grid>
      </AccordionDetails>
    </Accordion>
  );
}

interface UsageOverviewPanelProps {
  expandedEnvironmentIds: Set<string>;
  onToggleEnvironment: (id: string) => void;
}

/**
 * Usage overview: summary tiles, environment breakdown, aggregated line charts.
 *
 * @param expandedEnvironmentIds - Which environment rows show product grids.
 * @param onToggleEnvironment - Toggle handler for an environment row id.
 * @returns {JSX.Element} Overview tab content.
 */
export default function UsageOverviewPanel({
  expandedEnvironmentIds,
  onToggleEnvironment,
}: UsageOverviewPanelProps): JSX.Element {
  const overviewStats = useMemo(
    (): Record<UsageOverviewStatKey, number> => ({
      environments: USAGE_OVERVIEW_SUMMARY.environments,
      products: USAGE_OVERVIEW_SUMMARY.products,
      instances: USAGE_OVERVIEW_SUMMARY.instances,
    }),
    [],
  );

  return (
    <Box sx={{ display: "flex", flexDirection: "column", gap: 3 }}>
      <SupportStatGrid<UsageOverviewStatKey>
        isLoading={false}
        isError={false}
        configs={USAGE_OVERVIEW_STAT_CONFIGS}
        stats={overviewStats}
        entityName="usage metrics"
        spacing={2}
        itemSize={{ xs: 12, md: 4 }}
      />

      <Box>
        <Typography variant="h6" sx={{ fontWeight: 600, mb: 2 }}>
          Environment Breakdown
        </Typography>
        <Box sx={{ display: "flex", flexDirection: "column", gap: 1.5 }}>
          {USAGE_ENVIRONMENT_BREAKDOWN.map((row) => (
            <EnvironmentBreakdownAccordion
              key={row.id}
              row={row}
              expanded={expandedEnvironmentIds.has(row.id)}
              onToggle={() => onToggleEnvironment(row.id)}
            />
          ))}
        </Box>
      </Box>

      <Box>
        <Typography variant="h6" sx={{ fontWeight: 600, mb: 2 }}>
          Aggregated Metrics
        </Typography>
        <Box sx={{ display: "flex", flexDirection: "column", gap: 2 }}>
          <Grid container spacing={2}>
            {USAGE_AGGREGATED_METRICS.slice(0, 2).map((metric) => (
              <Grid key={metric.id} size={{ xs: 12, lg: 6 }}>
                <UsageMetricTrendCard metric={metric} />
              </Grid>
            ))}
          </Grid>
          <Grid container spacing={2}>
            {USAGE_AGGREGATED_METRICS.slice(2, 4).map((metric) => (
              <Grid key={metric.id} size={{ xs: 12, lg: 6 }}>
                <UsageMetricTrendCard metric={metric} />
              </Grid>
            ))}
          </Grid>
          <Grid container spacing={2}>
            <Grid size={{ xs: 12, lg: 6 }}>
              <UsageMetricTrendCard metric={USAGE_AGGREGATED_METRICS[4]} />
            </Grid>
          </Grid>
        </Box>
      </Box>
    </Box>
  );
}
