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

import { Box, Typography } from "@wso2/oxygen-ui";
import type { JSX } from "react";
import UsageAndMetricsTabContent from "@components/project-details/usage-metrics/UsageAndMetricsTabContent";

/**
 * Standalone Usage & Metrics page (sidebar navigation).
 *
 * @returns {JSX.Element} Usage & Metrics view for the current project.
 */
export default function UsageMetricsPage(): JSX.Element {
  return (
    <Box sx={{ display: "flex", flexDirection: "column", gap: 2 }}>
      <Typography variant="h5" component="h1" sx={{ fontWeight: 600 }}>
        Usage & Metrics
      </Typography>
      <UsageAndMetricsTabContent />
    </Box>
  );
}
