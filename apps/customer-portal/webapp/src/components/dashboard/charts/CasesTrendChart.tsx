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

import { Card, Typography, Box, Skeleton, colors } from "@wso2/oxygen-ui";
import {
  PieChart,
  Pie,
  Cell,
  ResponsiveContainer,
} from "@wso2/oxygen-ui-charts-react";
import type { JSX } from "react";
import ErrorIndicator from "@components/common/error-indicator/ErrorIndicator";
import {
  OUTSTANDING_ENGAGEMENTS_CATEGORY_CHART_DATA,
} from "@constants/dashboardConstants";
import { ChartLegend } from "@components/dashboard/charts/ChartLegend";

interface CasesTrendChartProps {
  isLoading?: boolean;
  isError?: boolean;
}

/**
 * Displays the cases trend chart.
 *
 * `@param` props - Component props
 */
export const CasesTrendChart = ({
  isLoading,
  isError,
}: CasesTrendChartProps): JSX.Element => {
  const chartSource = OUTSTANDING_ENGAGEMENTS_CATEGORY_CHART_DATA;

  const chartData = isError
    ? chartSource.map((item) => ({
        name: item.name,
        value: 1,
        color: colors.grey?.[300] ?? "#D1D5DB",
      }))
    : isLoading
      ? []
      : [
          { name: "Onboarding", value: 12, color: chartSource[0].color },
          { name: "Migration", value: 8, color: chartSource[1].color },
          { name: "Services", value: 15, color: chartSource[2].color },
          { name: "Improvements", value: 10, color: chartSource[3].color },
        ];

  const total = !isError && !isLoading
    ? chartData.reduce((sum, item) => sum + (item.value ?? 0), 0)
    : 0;

  return (
    <Card sx={{ p: 2, height: "100%" }}>
      {/* Title */}
      <Typography variant="h6" component="h3" sx={{ mb: 2 }}>
        Outstanding Engagements
      </Typography>
      {isLoading ? (
        <Box
          sx={{
            height: 240,
          }}
        >
          <Skeleton variant="rectangular" width="100%" height="100%" />
        </Box>
      ) : (
        <Box
          sx={{
            height: 240,
            position: "relative",
          }}
        >
          {isError && (
            <Box
              sx={{
                position: "absolute",
                top: 0,
                left: 0,
                right: 0,
                bottom: 0,
                display: "flex",
                alignItems: "center",
                justifyContent: "center",
                zIndex: 1,
                pointerEvents: "none",
              }}
            >
              <ErrorIndicator entityName="outstanding engagements" />
            </Box>
          )}
          <Box
            sx={{
              height: "100%",
              opacity: isError ? 0.3 : 1,
              filter: isError ? "grayscale(1)" : "none",
              "& *:focus": { outline: "none" },
            }}
          >
            <ResponsiveContainer width="100%" height="100%">
              <PieChart legend={{ show: false }} tooltip={{ show: !isError }}>
                <Pie
                  data={chartData}
                  cx="50%"
                  cy="50%"
                  innerRadius={60}
                  outerRadius={80}
                  paddingAngle={0}
                  minAngle={15}
                  dataKey="value"
                  nameKey="name"
                  startAngle={90}
                  endAngle={-270}
                  label={false}
                  labelLine={false}
                >
                  {chartData.map((entry, index) => (
                    <Cell
                      // eslint-disable-next-line react/no-array-index-key
                      key={`cell-${index}`}
                      fill={entry.color}
                      stroke={colors.common.white}
                    />
                  ))}
                </Pie>
              </PieChart>
            </ResponsiveContainer>
          </Box>
          {/* Center content: total value */}
          {!isError && (
            <Box
              sx={{
                position: "absolute",
                top: "50%",
                left: "50%",
                transform: "translate(-50%, -50%)",
                textAlign: "center",
                pointerEvents: "none",
              }}
            >
              <Typography variant="h4">
                {chartData.length > 0 ? total : "N/A"}
              </Typography>
              <Typography variant="caption">Total</Typography>
            </Box>
          )}
        </Box>
      )}
      {/* Legend */}
      {!isLoading && (
        <ChartLegend
          data={chartSource.map((item) => ({
            name: item.name,
            value: 0,
            color: item.color,
          }))}
          isError={isError}
        />
      )}
    </Card>
  );
};
