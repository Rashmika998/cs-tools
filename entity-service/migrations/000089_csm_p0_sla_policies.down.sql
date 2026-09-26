-- Copyright (c) 2026 WSO2 LLC. (https://www.wso2.com).
--
-- WSO2 LLC. licenses this file to you under the Apache License,
-- Version 2.0 (the "License"); you may not use this file except
-- in compliance with the License.
-- You may obtain a copy of the License at
--
-- http://www.apache.org/licenses/LICENSE-2.0
--
-- Unless required by applicable law or agreed to in writing,
-- software distributed under the License is distributed on an
-- "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
-- KIND, either express or implied.  See the License for the
-- specific language governing permissions and limitations
-- under the License.

-- Delete CSM-owned "sla" clocks before their "sla_policy" rows go away below
-- (this migration runs before 000088's down in a reverse rollback, while
-- source is still a real column) -- otherwise a rolled-back environment is
-- left with orphaned CSM clocks that, once 000088's down drops the source
-- column, become indistinguishable from real ServiceNow-synced rows and
-- would be served as such by GET /sla-status.
DELETE FROM sla WHERE source = 'CSM';

DELETE FROM sla_policy
WHERE created_by = 'migration-000089'
  AND name IN (
    'P0 - Response (Managed Services)',
    'P0 - Workaround (Managed Services)',
    'P0 - Resolution (Managed Services)'
  );
