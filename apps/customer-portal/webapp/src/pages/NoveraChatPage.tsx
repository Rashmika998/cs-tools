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

import { Box, Paper, Divider } from "@wso2/oxygen-ui";
import {
  useState,
  useRef,
  useEffect,
  useCallback,
  useMemo,
  type JSX,
} from "react";
import { useNavigate, useParams, useLocation } from "react-router";
import { useGetProjectDeployments } from "@api/useGetProjectDeployments";
import { usePostCaseClassifications } from "@api/usePostCaseClassifications";
import { usePostConversations } from "@api/usePostConversations";
import { usePostConversationMessages } from "@api/usePostConversationMessages";
import type { ChatNavState } from "@models/chatNavState";
import type { SlotState } from "@models/responses";
import { useAllDeploymentProducts } from "@hooks/useAllDeploymentProducts";
import {
  DEFAULT_CONVERSATION_REGION,
  DEFAULT_CONVERSATION_TIER,
} from "@constants/conversationConstants";
import {
  formatChatHistoryForClassification,
  buildEnvProducts,
} from "@utils/caseCreation";
import { htmlToPlainText } from "@utils/richTextEditor";
import ChatHeader from "@components/support/novera-ai-assistant/novera-chat-page/ChatHeader";
import ChatInput from "@components/support/novera-ai-assistant/novera-chat-page/ChatInput";
import ChatMessageList from "@components/support/novera-ai-assistant/novera-chat-page/ChatMessageList";

export interface Message {
  id: string;
  text: string;
  sender: "user" | "bot";
  timestamp: Date;
  showCreateCaseAction?: boolean;
  isLoading?: boolean;
  isError?: boolean;
  slotState?: SlotState;
}

/**
 * NoveraChatPage component to provide AI-powered support assistance.
 *
 * @returns {JSX.Element} The rendered NoveraChatPage.
 */
export default function NoveraChatPage(): JSX.Element {
  const navigate = useNavigate();
  const { projectId } = useParams<{ projectId: string }>();
  const location = useLocation();
  const navState = location.state as ChatNavState | null;
  const initialUserMessage = navState?.initialUserMessage;
  const conversationResponse = navState?.conversationResponse;

  const handleBack = () => {
    if (projectId) {
      navigate(`/${projectId}/support`);
    } else {
      navigate(-1);
    }
  };

  const { data: projectDeployments } = useGetProjectDeployments(
    projectId || "",
  );
  const { productsByDeploymentId, isLoading: isAllProductsLoading } =
    useAllDeploymentProducts(projectDeployments);
  const envProducts = useMemo(
    () => buildEnvProducts(productsByDeploymentId, projectDeployments),
    [productsByDeploymentId, projectDeployments],
  );
  const { mutateAsync: classifyCase } = usePostCaseClassifications();
  const { mutateAsync: postConversation } = usePostConversations();
  const { mutateAsync: postConversationMessages } =
    usePostConversationMessages();
  const [conversationId, setConversationId] = useState<string | null>(
    () => conversationResponse?.conversationId ?? null,
  );
  const [isCreateCaseLoading, setIsCreateCaseLoading] = useState(false);
  const [isWaitingForClassification, setIsWaitingForClassification] =
    useState(false);
  const [messages, setMessages] = useState<Message[]>(() => {
    const botWelcome: Message = {
      id: "1",
      text: "Hi! I'm Novera, your AI support assistant. I'm here to help you resolve your issue quickly. Can you describe the problem you're experiencing?",
      sender: "bot",
      timestamp: new Date(),
    };
    // Coming from describe-issue with API response: show only user message + bot response (no welcome).
    if (conversationResponse?.message) {
      const userMsg = initialUserMessage?.trim();
      const msgs: Message[] = [
        {
          id: "3",
          text: conversationResponse.message,
          sender: "bot",
          timestamp: new Date(),
          showCreateCaseAction: conversationResponse.actions != null,
          slotState: conversationResponse.slotState,
        },
      ];
      if (userMsg) {
        msgs.unshift({
          id: "2",
          text: userMsg,
          sender: "user",
          timestamp: new Date(),
        });
      }
      return msgs;
    }
    if (initialUserMessage?.trim()) {
      return [
        botWelcome,
        {
          id: "2",
          text: initialUserMessage.trim(),
          sender: "user",
          timestamp: new Date(),
        },
      ];
    }
    return [botWelcome];
  });

  const performClassification = useCallback(async () => {
    if (!projectId) {
      navigate("/");
      setIsCreateCaseLoading(false);
      setIsWaitingForClassification(false);
      return;
    }

    try {
      const chatHistory = formatChatHistoryForClassification(messages);
      const hasEnvProducts = Object.keys(envProducts).length > 0;

      if (chatHistory && hasEnvProducts) {
        try {
          const classificationResponse = await classifyCase({
            chatHistory,
            envProducts,
            region: DEFAULT_CONVERSATION_REGION,
            tier: DEFAULT_CONVERSATION_TIER,
          });
          navigate(`/${projectId}/support/chat/create-case`, {
            state: { messages, classificationResponse },
          });
        } catch {
          navigate(`/${projectId}/support/chat/create-case`, {
            state: { messages },
          });
        }
      } else {
        navigate(`/${projectId}/support/chat/create-case`, {
          state: { messages },
        });
      }
    } finally {
      setIsCreateCaseLoading(false);
      setIsWaitingForClassification(false);
    }
  }, [projectId, navigate, messages, envProducts, classifyCase]);

  const handleCreateCase = useCallback(() => {
    setIsCreateCaseLoading(true);

    if (isAllProductsLoading) {
      setIsWaitingForClassification(true);
    } else {
      performClassification();
    }
  }, [isAllProductsLoading, performClassification]);

  useEffect(() => {
    if (isWaitingForClassification && !isAllProductsLoading) {
      setIsWaitingForClassification(false);
      performClassification();
    }
  }, [isWaitingForClassification, isAllProductsLoading, performClassification]);
  const [inputValue, setInputValue] = useState("");
  const inputValueRef = useRef("");
  const [resetTrigger, setResetTrigger] = useState(0);
  const [isSending, setIsSending] = useState(false);
  const messagesEndRef = useRef<HTMLDivElement>(null);

  const setInputValueAndRef = useCallback((v: string) => {
    inputValueRef.current = v;
    setInputValue(v);
  }, []);

  useEffect(() => {
    messagesEndRef.current?.scrollIntoView({ behavior: "smooth" });
  }, [messages]);

  const handleSlotSelection = useCallback(
    (messageId: string, slot: string, value: string) => {
      setMessages((prev) =>
        prev.map((m) => {
          if (m.id === messageId && m.slotState) {
            const updatedFilledSlots = {
              ...(m.slotState.filledSlots || {}),
              [slot]: value,
            };
            return {
              ...m,
              slotState: {
                ...m.slotState,
                filledSlots: updatedFilledSlots,
              },
            };
          }
          return m;
        }),
      );
    },
    [],
  );

  const filledSlotsForDisplay = useMemo(() => {
    // Get the latest bot message with slotState
    const latestBotWithSlots = [...messages]
      .reverse()
      .find(
        (m) =>
          m.sender === "bot" &&
          m.slotState?.slotOptions &&
          m.slotState.slotOptions.length > 0,
      );

    if (
      !latestBotWithSlots?.slotState?.filledSlots ||
      !latestBotWithSlots.slotState.slotOptions
    ) {
      return [];
    }

    // Map filled slots to label-value pairs
    return latestBotWithSlots.slotState.slotOptions
      .map((option) => {
        const value = latestBotWithSlots.slotState?.filledSlots?.[option.slot];
        if (value) {
          return { label: option.label, value };
        }
        return null;
      })
      .filter(
        (item): item is { label: string; value: string } => item !== null,
      );
  }, [messages]);

  const sendToApi = useCallback(
    async (userText: string) => {
      if (!projectId) return null;
      const hasEnvProducts = Object.keys(envProducts).length > 0;
      const payload = {
        projectId,
        message: userText,
        envProducts: hasEnvProducts ? envProducts : {},
        region: DEFAULT_CONVERSATION_REGION,
        tier: DEFAULT_CONVERSATION_TIER,
      };
      if (conversationId) {
        return postConversationMessages({
          ...payload,
          conversationId,
        });
      }
      return postConversation(payload);
    },
    [
      projectId,
      conversationId,
      envProducts,
      postConversation,
      postConversationMessages,
    ],
  );

  const handleSendMessage = useCallback(async (): Promise<boolean> => {
    const text = htmlToPlainText(inputValueRef.current).trim();
    if (!text || isSending || !projectId) return false;

    const userMessage: Message = {
      id: `user-${Date.now()}`,
      text,
      sender: "user",
      timestamp: new Date(),
    };
    const botMessageId = `bot-${Date.now()}`;
    const loadingBot: Message = {
      id: botMessageId,
      text: "",
      sender: "bot",
      timestamp: new Date(),
      isLoading: true,
    };

    setMessages((prev) => [...prev, userMessage, loadingBot]);
    setInputValueAndRef("");
    setResetTrigger((prev) => prev + 1);
    setIsSending(true);

    try {
      const response = await sendToApi(text);
      if (response?.conversationId) {
        setConversationId(response.conversationId);
      }
      setMessages((prev) =>
        prev.map((m) =>
          m.id === botMessageId
            ? {
                ...m,
                text: response?.message ?? "",
                isLoading: false,
                isError: false,
                showCreateCaseAction: response?.actions != null,
                slotState: response?.slotState,
              }
            : m,
        ),
      );
      return true;
    } catch {
      setMessages((prev) =>
        prev.map((m) =>
          m.id === botMessageId
            ? {
                ...m,
                text: "",
                isLoading: false,
                isError: true,
              }
            : m,
        ),
      );
      return false;
    } finally {
      setIsSending(false);
    }
  }, [isSending, projectId, sendToApi, setInputValueAndRef]);

  const handleChooseSlots = useCallback(
    (messageId: string) => {
      const message = messages.find((m) => m.id === messageId);
      if (!message?.slotState?.filledSlots || !message.slotState.slotOptions) {
        return;
      }

      // Format the message as "Environment is {value} and Product is {value}"
      const parts = message.slotState.slotOptions
        .map((option) => {
          const value = message.slotState?.filledSlots?.[option.slot];
          if (value) {
            return `${option.label} is ${value}`;
          }
          return null;
        })
        .filter((part): part is string => part !== null);

      const formattedMessage = parts.join(" and ");

      // Set the formatted message as plain text and trigger send
      inputValueRef.current = formattedMessage;
      setInputValue(formattedMessage);

      void handleSendMessage().then((didSend) => {
        if (!didSend) {
          return;
        }
        setMessages((prev) =>
          prev.map((currentMessage) =>
            currentMessage.id === messageId && currentMessage.slotState
              ? {
                  ...currentMessage,
                  slotState: {
                    ...currentMessage.slotState,
                    filledSlots: {},
                  },
                }
              : currentMessage,
          ),
        );
      });
    },
    [messages, handleSendMessage],
  );

  return (
    <Box
      sx={{
        height: (theme) => `calc(100vh - ${theme.spacing(21)})`,
        display: "flex",
      }}
    >
      <Box
        sx={{
          flex: 1,
          display: "flex",
          flexDirection: "column",
          width: "100%",
          overflow: "hidden",
        }}
      >
        <ChatHeader onBack={handleBack} />

        {/* Chat window */}
        <Paper
          sx={{
            flex: 1,
            display: "flex",
            flexDirection: "column",
            overflow: "hidden",
          }}
        >
          <ChatMessageList
            messages={messages}
            messagesEndRef={messagesEndRef}
            onCreateCase={handleCreateCase}
            isCreateCaseLoading={isCreateCaseLoading}
            onSlotSelection={handleSlotSelection}
            onChooseSlots={handleChooseSlots}
          />

          <Divider />

          <ChatInput
            onSend={handleSendMessage}
            inputValue={inputValue}
            setInputValue={setInputValueAndRef}
            isSending={isSending}
            resetTrigger={resetTrigger}
            filledSlots={filledSlotsForDisplay}
          />
        </Paper>
      </Box>
    </Box>
  );
}
