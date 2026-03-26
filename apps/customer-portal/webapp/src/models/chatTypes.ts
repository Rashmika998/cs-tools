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

import type { ConversationResponse, SlotState } from "@models/responses";

export type ChatSender = "user" | "bot";

export interface Recommendation {
  title: string;
  articleId: string;
  score: number;
}

export interface Message {
  id: string;
  text: string;
  sender: ChatSender;
  timestamp: Date;
  showCreateCaseAction?: boolean;
  isLoading?: boolean;
  isError?: boolean;
  slotState?: SlotState;
  recommendations?: Recommendation[];
  thinkingSteps?: string[];
  thinkingLabel?: string | null;
  isStreaming?: boolean;
}

export interface ChatNavState {
  initialUserMessage?: string;
  conversationResponse?: ConversationResponse;
  initialEnvProducts?: Record<string, string[]>;
  accountId?: string;
}

export interface ChatWebSocketEvent {
  type: string;
  [key: string]: unknown;
}

export interface ChatWebSocketPayload {
  type: "user_message";
  accountId: string;
  conversationId: string;
  message: string;
  envProducts: Record<string, string[]>;
}

export interface UseChatWebSocketOptions {
  onEvent: (event: ChatWebSocketEvent) => void;
  onClose?: () => void;
  onError?: (message: string) => void;
}
