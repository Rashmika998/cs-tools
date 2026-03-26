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

import { render, screen, fireEvent, waitFor } from "@testing-library/react";
import { describe, expect, it, vi, beforeEach } from "vitest";
import { MemoryRouter, Routes, Route } from "react-router";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";

vi.mock(
  "@components/support/novera-ai-assistant/novera-chat-page/ChatInput",
  () => ({
    default: ({
      inputValue,
      setInputValue,
      onSend,
      onCreateCase,
      isCreateCaseLoading,
    }: any) => (
      <div>
        <button onClick={onCreateCase} disabled={isCreateCaseLoading}>
          Create Case
        </button>
        {isCreateCaseLoading ? <div data-testid="circular-progress" /> : null}
        <input
          placeholder="Type your message..."
          value={inputValue}
          onChange={(e) => setInputValue(e.target.value)}
          onKeyDown={(e) => {
            if (e.key === "Enter") onSend();
          }}
        />
        <button onClick={onSend}>Send</button>
      </div>
    ),
  }),
);

// Mock @wso2/oxygen-ui
vi.mock("@wso2/oxygen-ui", () => ({
  Box: ({ children }: { children?: React.ReactNode }) => <div>{children}</div>,
  Button: ({ children, onClick, startIcon }: any) => (
    <button onClick={onClick}>
      {startIcon}
      {children}
    </button>
  ),
  CircularProgress: () => <div data-testid="circular-progress" />,
  Paper: ({ children }: { children?: React.ReactNode }) => <div>{children}</div>,
  Typography: ({ children }: { children?: React.ReactNode }) => <div>{children}</div>,
  alpha: (c: string) => c,
  Tooltip: ({ children }: { children?: React.ReactNode }) => <>{children}</>,
  TextField: ({ value, onChange, placeholder, onKeyPress }: any) => (
    <input
      value={value}
      onChange={onChange}
      placeholder={placeholder}
      onKeyPress={onKeyPress}
    />
  ),
  IconButton: ({ children, onClick, disabled }: any) => (
    <button onClick={onClick} disabled={disabled}>
      {children}
    </button>
  ),
  Divider: () => <hr />,
  Stack: ({ children }: { children?: React.ReactNode }) => <div>{children}</div>,
  colors: {
    orange: {
      700: "#C2410C",
    },
    yellow: {
      800: "#854D0E",
    },
  },
}));

vi.mock("@components/common/rich-text-editor/Editor", () => ({
  default: ({
    value,
    onChange,
    placeholder,
    onSubmitKeyDown,
  }: any) => (
    <input
      data-testid="chat-editor-input"
      value={value}
      placeholder={placeholder}
      onChange={(e) => onChange?.(typeof e === "object" && e?.target ? e.target.value : e)}
      onKeyDown={(e: React.KeyboardEvent) => {
        if (e.key === "Enter" && !e.shiftKey) {
          e.preventDefault();
          onSubmitKeyDown?.();
        }
      }}
    />
  ),
}));

vi.mock("@utils/richTextEditor", () => ({
  htmlToPlainText: (html: string) =>
    (html || "").replace(/<[^>]*>/g, "").trim() || html || "",
}));

// Mock icons
vi.mock("@wso2/oxygen-ui-icons-react", () => ({
  Bot: () => <svg data-testid="bot-icon" />,
  User: () => <svg data-testid="user-icon" />,
  ThumbsUp: () => <svg data-testid="thumbs-up-icon" />,
  ThumbsDown: () => <svg data-testid="thumbs-down-icon" />,
  ArrowLeft: () => <svg data-testid="back-icon" />,
  Send: () => <svg data-testid="send-icon" />,
  CircleAlert: () => <svg data-testid="alert-icon" />,
  Sparkles: () => <svg data-testid="sparkles-icon" />,
  FileText: () => <svg data-testid="file-text-icon" />,
  Copy: () => <svg data-testid="copy-icon" />,
  PanelTopClose: () => <svg data-testid="panel-top-close-icon" />,
}));

const { useAllDeploymentProductsMock } = vi.hoisted(() => ({
  useAllDeploymentProductsMock: vi.fn(),
}));

vi.mock("@hooks/useAllDeploymentProducts", () => ({
  useAllDeploymentProducts: useAllDeploymentProductsMock,
}));

vi.mock("@api/usePostCaseClassifications", () => ({
  usePostCaseClassifications: () => ({
    mutateAsync: vi.fn().mockResolvedValue({}),
    isPending: false,
  }),
}));

const mockConversationResponse = {
  message: "Mock AI response from API",
  conversationId: "conv-123",
  actions: "createCase",
};

vi.mock("@api/usePostConversations", () => ({
  usePostConversations: () => ({
    mutateAsync: vi.fn().mockResolvedValue(mockConversationResponse),
  }),
}));

vi.mock("@api/usePostConversationMessages", () => ({
  usePostConversationMessages: () => ({
    mutateAsync: vi.fn().mockResolvedValue(mockConversationResponse),
  }),
}));

vi.mock("@api/useGetDeployments", () => ({
  useGetDeployments: () => ({
    data: { deployments: [] },
  }),
}));

vi.mock("@api/useGetProjectDeployments", () => ({
  useGetProjectDeployments: () => ({
    data: [
      { id: "dep-1", type: { label: "Staging" } },
      { id: "dep-2", type: { label: "QA" } },
    ],
  }),
}));

vi.mock("@api/useChatWebSocket", () => ({
  useChatWebSocket: ({ onEvent }: any) => ({
    connect: vi.fn().mockResolvedValue(undefined),
    close: vi.fn(),
    isConnected: true,
    sendUserMessage: vi.fn().mockImplementation(async () => {
      onEvent({
        type: "final",
        payload: { message: "Mock AI response from API" },
      });
    }),
  }),
}));

vi.mock("@api/useGetDeploymentsProducts", () => ({
  fetchDeploymentProducts: vi
    .fn()
    .mockResolvedValue([{ product: { label: "WSO2 API Manager 3.2.0" } }]),
}));

vi.mock("@providers/MockConfigProvider", () => ({
  useMockConfig: () => ({ isMockEnabled: true }),
}));

vi.mock("@asgardeo/react", () => ({
  useAsgardeo: () => ({
    getIdToken: vi.fn().mockResolvedValue("mock-token"),
  }),
}));

vi.mock("@api/useGetProjectDetails", () => ({
  default: () => ({
    data: {
      account: { supportTier: "", activationDate: "", deactivationDate: "", id: "", name: "", region: "" },
    },
    isLoading: false,
    error: null,
  }),
}));

vi.mock("@context/error-banner/ErrorBannerContext", () => ({
  useErrorBanner: () => ({
    showError: vi.fn(),
  }),
}));

vi.mock("@hooks/useLogger", () => ({
  useLogger: () => ({
    debug: vi.fn(),
    info: vi.fn(),
    warn: vi.fn(),
    error: vi.fn(),
  }),
}));

const queryClient = new QueryClient({
  defaultOptions: { queries: { retry: false } },
});

const renderWithRouter = async (initialUserMessage?: string) => {
  const { default: NoveraChatPage } = await import("@pages/NoveraChatPage");
  return render(
    <QueryClientProvider client={queryClient}>
      <MemoryRouter
        initialEntries={[
          initialUserMessage
            ? {
                pathname: "/projects/project-1/support/chat",
                state: { initialUserMessage },
                key: "init",
              }
            : "/projects/project-1/support/chat",
        ]}
      >
        <Routes>
          <Route
            path="/projects/:projectId/support/chat"
            element={<NoveraChatPage />}
          />
          <Route
            path="/projects/:projectId/support"
            element={<div>Support Page</div>}
          />
          <Route
            path="/projects/:projectId/dashboard"
            element={<div>Dashboard Page</div>}
          />
        </Routes>
      </MemoryRouter>
    </QueryClientProvider>,
  );
};

describe("NoveraChatPage", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    useAllDeploymentProductsMock.mockReturnValue({
      productsByDeploymentId: {},
      isLoading: false,
    });
    window.HTMLElement.prototype.scrollIntoView = vi.fn();
  });

  it("should render the initial state correctly", async () => {
    await renderWithRouter();

    expect(
      screen.getByText(/Hi! I'm Novera, your AI support assistant/i),
    ).toBeInTheDocument();
    expect(
      screen.getByPlaceholderText(/Type your message/),
    ).toBeInTheDocument();
  });

  it("should navigate back when clicking 'Back'", async () => {
    await renderWithRouter();

    const backButton = screen.getByText("Back");
    fireEvent.click(backButton);

    expect(screen.getByText("Support Page")).toBeInTheDocument();
  });

  it("should send and display multiple messages", async () => {
    for (let i = 1; i <= 4; i++) {
      const { unmount } = await renderWithRouter(`Message ${i}`);
      await waitFor(
        () =>
          expect(screen.getByText("Mock AI response from API")).toBeInTheDocument(),
        { timeout: 2000 },
      );
      unmount();
    }
  });

  it("should send a message and receive a bot response", async () => {
    await renderWithRouter("Hello Novera!");

    expect(screen.getByText("Hello Novera!")).toBeInTheDocument();

    await waitFor(
      () => {
        expect(screen.getByText("Mock AI response from API")).toBeInTheDocument();
      },
      { timeout: 2000 },
    );
  });

  it("should wait for products to load before classifying case", async () => {
    useAllDeploymentProductsMock
      .mockReturnValueOnce({
        productsByDeploymentId: {},
        isLoading: true,
      })
      .mockReturnValue({
        productsByDeploymentId: { "dep-1": [] },
        isLoading: false,
      });

    const { default: NoveraChatPage } = await import("@pages/NoveraChatPage");

    const { getByTestId, findByText } = render(
      <QueryClientProvider client={new QueryClient()}>
        <MemoryRouter
          initialEntries={[
            {
              pathname: "/projects/project-1/support/chat",
              state: { initialUserMessage: "Message" },
              key: "init",
            },
          ]}
        >
          <Routes>
            <Route
              path="/projects/:projectId/support/chat"
              element={<NoveraChatPage />}
            />
            <Route
              path="/projects/:projectId/support/chat/create-case"
              element={<div>Create Case Page</div>}
            />
          </Routes>
        </MemoryRouter>
      </QueryClientProvider>,
    );

    await waitFor(
      () =>
        expect(
          screen.getByText("Mock AI response from API"),
        ).toBeInTheDocument(),
      { timeout: 2000 },
    );

    const createCaseButton = await findByText("Create Case");
    fireEvent.click(createCaseButton);
    expect(getByTestId("circular-progress")).toBeInTheDocument();
    expect(await findByText("Create Case Page")).toBeInTheDocument();
  });
});
