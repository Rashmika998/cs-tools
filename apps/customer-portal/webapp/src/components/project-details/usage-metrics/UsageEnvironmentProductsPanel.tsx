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
  Activity,
  ChevronDown,
  Code2,
  Cpu,
  Package,
  Server,
} from "@wso2/oxygen-ui-icons-react";
import { LineChart } from "@wso2/oxygen-ui-charts-react";
import type { JSX } from "react";
import { useCallback, useState } from "react";
import type {
  UsageEnvironmentKind,
  UsageEnvironmentProduct,
  UsageInstanceChartBlock,
  UsageProductInstanceRow,
} from "@models/usageMetrics.types";
import {
  USAGE_ENVIRONMENT_PRODUCTS,
  USAGE_LINE_CHART_MARGIN,
} from "@constants/usageMetricsConstants";
import { UsageChartSurface } from "@components/project-details/usage-metrics/UsageChartSurface";

function panelAccent(kind: UsageEnvironmentKind): {
  main: string;
  stroke: string;
  borderDefault: string;
  borderHover: string;
  headerBg: string;
  headerHoverBg: string;
  iconWellBg: string;
  iconColor: string;
} {
  const main =
    kind === "production"
      ? (colors.orange?.[600] ?? "#EA580C")
      : kind === "test"
        ? (colors.blue?.[600] ?? "#2563EB")
        : (colors.green?.[600] ?? "#16A34A");

  return {
    main,
    stroke: main,
    borderDefault: alpha(main, 0.15),
    borderHover: alpha(main, 0.35),
    headerBg: alpha(main, 0.08),
    headerHoverBg: alpha(main, 0.12),
    iconWellBg: alpha(main, 0.15),
    iconColor: main,
  };
}

function instanceAccordionKey(productId: string, instanceId: string): string {
  return `${productId}::${instanceId}`;
}

interface UsageEnvironmentProductsPanelProps {
  environment: UsageEnvironmentKind;
  expandedProductIds: Set<string>;
  onToggleProduct: (productId: string) => void;
}

function InstanceMiniTrendCard({ block }: { block: UsageInstanceChartBlock }): JSX.Element {
  const deltaColor = block.deltaPositive
    ? (colors.green?.[600] ?? "#16A34A")
    : (colors.red?.[600] ?? "#DC2626");

  return (
    <Card
      sx={{
        p: 2,
        border: 1,
        borderColor: "divider",
        borderRadius: 0,
        display: "flex",
        flexDirection: "column",
        gap: 0,
      }}
    >
      <Box
        sx={{
          display: "flex",
          alignItems: "flex-start",
          justifyContent: "space-between",
          mb: 1.5,
          gap: 1,
        }}
      >
        <Box sx={{ minWidth: 0 }}>
          <Typography variant="subtitle2" sx={{ fontWeight: 600 }}>
            {block.title}
          </Typography>
          <Typography variant="caption" color="text.secondary">
            {block.caption}
          </Typography>
        </Box>
        <Box sx={{ textAlign: "right", flexShrink: 0 }}>
          <Typography variant="h6" sx={{ fontWeight: 600 }}>
            {block.headlineValue}
          </Typography>
          <Typography variant="caption" sx={{ fontWeight: 600, color: deltaColor }}>
            {block.deltaLabel}
          </Typography>
        </Box>
      </Box>
      <UsageChartSurface minHeight={150}>
        <LineChart
          data={block.data}
          xAxisDataKey="name"
          height={150}
          width="100%"
          margin={USAGE_LINE_CHART_MARGIN}
          accessibilityLayer={false}
          legend={{ show: false }}
          grid={{ show: true, strokeDasharray: "3 3" }}
          lines={[
            {
              dataKey: "value",
              name: block.title,
              stroke: block.stroke,
              strokeWidth: 2,
              dot: false,
            },
          ]}
        />
      </UsageChartSurface>
    </Card>
  );
}

function InstanceAccordionRow({
  instance,
  expanded,
  onToggle,
}: {
  instance: UsageProductInstanceRow;
  expanded: boolean;
  onToggle: () => void;
}): JSX.Element {
  const borderMuted = alpha(colors.grey?.[500] ?? "#6B7280", 0.2);
  const wellBg = alpha(colors.grey?.[500] ?? "#6B7280", 0.04);

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
        borderColor: borderMuted,
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
              className="usage-instance-accordion-chevron"
              color={colors.grey?.[400] ?? "#9CA3AF"}
              style={{ transition: "transform 0.2s ease" }}
            />
          </Box>
        }
        sx={{
          px: 2,
          py: 2,
          minHeight: 56,
          textAlign: "left",
          borderRadius: 0,
          "&:hover": { bgcolor: wellBg },
          "& .MuiAccordionSummary-expandIconWrapper.Mui-expanded": {
            "& .usage-instance-accordion-chevron": {
              transform: "rotate(180deg)",
            },
          },
        }}
      >
        <Box
          sx={{
            display: "flex",
            flexDirection: { xs: "column", lg: "row" },
            alignItems: { xs: "stretch", lg: "center" },
            justifyContent: "space-between",
            gap: 2,
            width: "100%",
            pr: 1,
          }}
        >
          <Box
            sx={{
              display: "flex",
              alignItems: "center",
              gap: 1.5,
              minWidth: { lg: 200 },
            }}
          >
            <Server size={18} color={colors.grey?.[500]} />
            <Typography variant="body2" sx={{ fontWeight: 600 }}>
              {instance.hostName}
            </Typography>
          </Box>
          <Box
            sx={{
              display: "flex",
              flexWrap: "wrap",
              gap: { xs: 2, lg: 4 },
              alignItems: "center",
              justifyContent: { xs: "flex-start", lg: "flex-end" },
              flex: 1,
            }}
          >
            <MetricPill
              icon={<Code2 size={16} color={colors.grey?.[400]} />}
              label="Java Version"
              value={instance.javaVersion}
            />
            <MetricPill
              icon={<Package size={16} color={colors.grey?.[400]} />}
              label="U2 Level"
              value={instance.u2Level}
            />
            <MetricPill
              icon={<Activity size={16} color={colors.grey?.[400]} />}
              label="Total Transactions"
              value={instance.transactionsLabel}
            />
            <MetricPill
              icon={<Cpu size={16} color={colors.grey?.[400]} />}
              label="Core Count"
              value={String(instance.coreCount)}
            />
          </Box>
        </Box>
      </AccordionSummary>
      <AccordionDetails
        sx={{
          px: 2,
          pb: 2,
          pt: 0,
          borderTop: 1,
          borderColor: "divider",
          bgcolor: alpha(colors.grey?.[500] ?? "#6B7280", 0.06),
          borderRadius: 0,
        }}
      >
        <Grid container spacing={2} sx={{ mt: 1 }}>
          <Grid size={{ xs: 12, lg: 6 }}>
            <InstanceMiniTrendCard block={instance.charts.transactions} />
          </Grid>
          <Grid size={{ xs: 12, lg: 6 }}>
            <InstanceMiniTrendCard block={instance.charts.cores} />
          </Grid>
        </Grid>
      </AccordionDetails>
    </Accordion>
  );
}

function ProductAccordionRow({
  product,
  environment,
  expanded,
  onToggle,
  expandedInstanceKeys,
  onToggleInstance,
}: {
  product: UsageEnvironmentProduct;
  environment: UsageEnvironmentKind;
  expanded: boolean;
  onToggle: () => void;
  expandedInstanceKeys: Set<string>;
  onToggleInstance: (key: string) => void;
}): JSX.Element {
  const a = panelAccent(environment);

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
        borderColor: a.borderDefault,
        borderRadius: 0,
        transition: "border-color 0.2s ease",
        bgcolor: "background.paper",
        "&:hover": { borderColor: a.borderHover },
        "&.Mui-expanded": { margin: 0 },
      }}
    >
      <AccordionSummary
        expandIcon={
          <Box sx={{ display: "flex", alignItems: "center" }}>
            <ChevronDown
              size={20}
              className="usage-product-accordion-chevron"
              color={a.iconColor}
              style={{ transition: "transform 0.2s ease" }}
            />
          </Box>
        }
        sx={{
          px: 2,
          py: 2,
          minHeight: 56,
          bgcolor: a.headerBg,
          borderRadius: 0,
          "&:hover": {
            bgcolor: a.headerHoverBg,
          },
          "& .MuiAccordionSummary-expandIconWrapper.Mui-expanded": {
            "& .usage-product-accordion-chevron": {
              transform: "rotate(180deg)",
            },
          },
        }}
      >
        <Box
          sx={{
            display: "flex",
            flexDirection: { xs: "column", lg: "row" },
            alignItems: { xs: "stretch", lg: "flex-start" },
            gap: 2,
            width: "100%",
            pr: 1,
          }}
        >
          <Box
            sx={{
              display: "flex",
              alignItems: "center",
              gap: 2,
              minWidth: { lg: 250 },
            }}
          >
            <Box
              sx={{
                p: 1.5,
                bgcolor: a.iconWellBg,
                display: "flex",
                flexShrink: 0,
              }}
            >
              <Package size={24} color={a.iconColor} />
            </Box>
            <Box sx={{ textAlign: "left" }}>
              <Typography variant="subtitle1" sx={{ fontWeight: 600 }}>
                {product.name}
              </Typography>
              <Typography variant="body2" color="text.secondary">
                {product.version}
              </Typography>
            </Box>
          </Box>

          <Box
            sx={{
              flex: 1,
              display: "grid",
              gridTemplateColumns: { xs: "1fr", sm: "repeat(3, 1fr)" },
              gap: 2,
              alignItems: "start",
            }}
          >
            <Box sx={{ textAlign: "center" }}>
              <Typography variant="caption" color="text.secondary">
                Running Instances
              </Typography>
              <Typography variant="h6" sx={{ fontWeight: 600 }}>
                {product.runningInstances}
              </Typography>
            </Box>
            <Box sx={{ textAlign: "center" }}>
              <Typography variant="caption" color="text.secondary">
                Total Transactions
              </Typography>
              <Typography variant="h6" sx={{ fontWeight: 600 }}>
                {product.transactionsLabel}
              </Typography>
            </Box>
            <Box sx={{ textAlign: "center" }}>
              <Typography variant="caption" color="text.secondary">
                {product.thirdMetricLabel}
              </Typography>
              <Typography variant="h6" sx={{ fontWeight: 600 }}>
                {product.thirdMetricValue}
              </Typography>
            </Box>
          </Box>

          <Box
            sx={{
              borderLeft: { lg: "1px solid" },
              borderColor: { lg: "divider" },
              pl: { lg: 2 },
              minWidth: { lg: 200 },
            }}
          >
            <Typography
              variant="caption"
              sx={{ fontWeight: 600, color: "text.secondary", mb: 1, display: "block" }}
            >
              Core Metrics
            </Typography>
            <Box sx={{ display: "flex", flexWrap: "wrap", gap: 1.5 }}>
              {product.coreMetrics.map((m) => (
                <Box key={m.label} sx={{ textAlign: "center", minWidth: 56 }}>
                  <Typography variant="caption" color="text.secondary">
                    {m.label}
                  </Typography>
                  <Typography variant="body2" sx={{ fontWeight: 600 }}>
                    {m.value}
                  </Typography>
                </Box>
              ))}
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
          <Grid size={{ xs: 12, lg: 6 }}>
            <Card sx={{ p: 2, borderRadius: 0 }}>
              <Typography variant="subtitle2" sx={{ fontWeight: 600, mb: 2 }}>
                Transaction Trends
              </Typography>
              <UsageChartSurface minHeight={200}>
                <LineChart
                  data={product.transactionTrend}
                  xAxisDataKey="name"
                  height={200}
                  width="100%"
                  margin={USAGE_LINE_CHART_MARGIN}
                  accessibilityLayer={false}
                  legend={{ show: false }}
                  grid={{ show: true, strokeDasharray: "3 3" }}
                  lines={[
                    {
                      dataKey: "value",
                      name: "Transactions",
                      stroke: a.stroke,
                      strokeWidth: 2.5,
                      dot: false,
                    },
                  ]}
                />
              </UsageChartSurface>
            </Card>
          </Grid>
          <Grid size={{ xs: 12, lg: 6 }}>
            <Card sx={{ p: 2, borderRadius: 0 }}>
              <Typography variant="subtitle2" sx={{ fontWeight: 600, mb: 2 }}>
                Core Usage Over Time
              </Typography>
              <UsageChartSurface minHeight={200}>
                <LineChart
                  data={product.coreUsageTrend}
                  xAxisDataKey="name"
                  height={200}
                  width="100%"
                  margin={USAGE_LINE_CHART_MARGIN}
                  accessibilityLayer={false}
                  legend={{ show: true, align: "center", verticalAlign: "bottom" }}
                  grid={{ show: true, strokeDasharray: "3 3" }}
                  colors={[a.stroke, colors.grey?.[400] ?? "#9CA3AF"]}
                  lines={[
                    {
                      dataKey: "current",
                      name: "Current",
                      stroke: a.stroke,
                      strokeWidth: 2.5,
                      dot: false,
                    },
                    {
                      dataKey: "average",
                      name: "Average",
                      stroke: colors.grey?.[500] ?? "#6B7280",
                      strokeWidth: 2,
                      strokeDasharray: "5 5",
                      dot: false,
                    },
                  ]}
                />
              </UsageChartSurface>
            </Card>
          </Grid>
        </Grid>

        <Box sx={{ mt: 2 }}>
          <Typography variant="subtitle2" sx={{ fontWeight: 600, mb: 1.5 }}>
            Instances
          </Typography>
          <Box sx={{ display: "flex", flexDirection: "column", gap: 1 }}>
            {product.instances.map((inst) => (
              <InstanceAccordionRow
                key={inst.id}
                instance={inst}
                expanded={expandedInstanceKeys.has(
                  instanceAccordionKey(product.id, inst.id),
                )}
                onToggle={() =>
                  onToggleInstance(instanceAccordionKey(product.id, inst.id))
                }
              />
            ))}
          </Box>
        </Box>
      </AccordionDetails>
    </Accordion>
  );
}

function MetricPill({
  icon,
  label,
  value,
}: {
  icon: JSX.Element;
  label: string;
  value: string;
}): JSX.Element {
  return (
    <Box sx={{ display: "flex", alignItems: "center", gap: 0.75 }}>
      {icon}
      <Box>
        <Typography variant="caption" color="text.secondary">
          {label}
        </Typography>
        <Typography variant="body2" sx={{ fontWeight: 600 }}>
          {value}
        </Typography>
      </Box>
    </Box>
  );
}

/**
 * Production / Test / Development product list with expandable charts and instances.
 *
 * @param environment - Which environment dataset to render.
 * @param expandedProductIds - Expanded product keys for this view.
 * @param onToggleProduct - Toggle expand for a product id.
 * @returns {JSX.Element} Environment tab content.
 */
export default function UsageEnvironmentProductsPanel({
  environment,
  expandedProductIds,
  onToggleProduct,
}: UsageEnvironmentProductsPanelProps): JSX.Element {
  const products = USAGE_ENVIRONMENT_PRODUCTS[environment];
  const [expandedInstanceKeys, setExpandedInstanceKeys] = useState<Set<string>>(
    () => new Set(),
  );

  const toggleInstance = useCallback((key: string) => {
    setExpandedInstanceKeys((prev) => {
      const next = new Set(prev);
      if (next.has(key)) {
        next.delete(key);
      } else {
        next.add(key);
      }
      return next;
    });
  }, []);

  return (
    <Box sx={{ display: "flex", flexDirection: "column", gap: 1.5 }}>
      {products.map((product) => (
        <ProductAccordionRow
          key={product.id}
          product={product}
          environment={environment}
          expanded={expandedProductIds.has(product.id)}
          onToggle={() => onToggleProduct(product.id)}
          expandedInstanceKeys={expandedInstanceKeys}
          onToggleInstance={toggleInstance}
        />
      ))}
    </Box>
  );
}
