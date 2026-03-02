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
import { type JSX } from "react";
import {
  Button,
  Stack,
  Typography,
  Paper,
} from "@wso2/oxygen-ui";
import {
  ArrowLeft,
} from "@wso2/oxygen-ui-icons-react";

/**
 * ChangeRequestDetailsPage component to display detailed information about a change request.
 *
 * @returns {JSX.Element} The rendered Change Request Details page.
 */
export default function ChangeRequestDetailsPage(): JSX.Element {
  const navigate = useNavigate();
  const { projectId } = useParams<{
    projectId: string;
    changeRequestId: string;
  }>();

  return (
    <Stack spacing={3}>
      {/* Back Button */}
      <Button
        startIcon={<ArrowLeft size={16} />}
        onClick={() => navigate(`/${projectId}/support/change-requests`)}
        sx={{ alignSelf: "flex-start" }}
        variant="text"
      >
        Back to Change Requests
      </Button>

      {/* Not Available Message */}
      <Paper variant="outlined" sx={{ p: 6, textAlign: "center" }}>
        <Typography variant="h5" color="text.primary" sx={{ mb: 2 }}>
          Change Request Details
        </Typography>
        <Typography variant="body1" color="text.secondary">
          Detailed change request information is not yet available.
        </Typography>
      </Paper>
    </Stack>
  );
}
