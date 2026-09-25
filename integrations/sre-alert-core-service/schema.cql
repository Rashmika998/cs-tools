-- Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
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

CREATE KEYSPACE IF NOT EXISTS alertintegration WITH replication = {
  'class': 'SimpleStrategy', 'replication_factor': 1
};

CREATE TABLE IF NOT EXISTS alertintegration.alerts (
  id         text PRIMARY KEY,
  vendor     text,
  alert      text,
  created_at timestamp
);

CREATE TABLE IF NOT EXISTS alertintegration.alert_seq (
  name text PRIMARY KEY,
  seq  bigint
) WITH cosmosdb_cell_level_timestamp = true
  AND cosmosdb_cell_level_timestamp_tombstones = true
  AND cosmosdb_cell_level_timetolive = true;

CREATE TABLE IF NOT EXISTS alertintegration.incidents_proccessed (
  fingerprint     text PRIMARY KEY,
  incident_number text, -- placeholder ("PENDING-<fp>") until CSM assigns the real number
  status          text,
  severity        int,
  impact          int,
  urgency         int,
  service         text,
  metric_name     text,
  description     text,
  category        text,
  environment     text,
  source          text,
  alert_ids       list<text>,
  alert_count     int,
  work_notes      list<text>,
  first_seen      timestamp,
  last_seen       timestamp,
  notified        boolean,
  csm_confirmed   boolean
) WITH cosmosdb_cell_level_timestamp = true
  AND cosmosdb_cell_level_timestamp_tombstones = true
  AND cosmosdb_cell_level_timetolive = true;

CREATE TABLE IF NOT EXISTS alertintegration.alert_cursor (
  name text PRIMARY KEY,
  seq  bigint
) WITH cosmosdb_cell_level_timestamp = true
  AND cosmosdb_cell_level_timestamp_tombstones = true
  AND cosmosdb_cell_level_timetolive = true;

CREATE TABLE IF NOT EXISTS alertintegration.processor_lease (
  name       text PRIMARY KEY,
  owner      text,
  expires_at timestamp
) WITH cosmosdb_cell_level_timestamp = true
  AND cosmosdb_cell_level_timestamp_tombstones = true
  AND cosmosdb_cell_level_timetolive = true;
