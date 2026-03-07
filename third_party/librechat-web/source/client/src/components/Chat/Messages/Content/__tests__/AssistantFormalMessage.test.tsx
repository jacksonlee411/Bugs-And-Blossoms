import React from 'react';
import { render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import type { TMessage } from 'librechat-data-provider';
import AssistantFormalMessage from '~/components/Chat/Messages/Content/AssistantFormalMessage';

const mockGetMessages = jest.fn();
const mockSetMessages = jest.fn();
const mockConfirmAssistantFormalTurn = jest.fn();
const mockCommitAssistantFormalTurn = jest.fn();

jest.mock('~/components/Chat/Messages/Content/Container', () => ({
  __esModule: true,
  default: ({ children }: { children: React.ReactNode }) => <div>{children}</div>,
}));

jest.mock('~/hooks/useLocalize', () => () => (key: string) => {
  const dict: Record<string, string> = {
    com_ui_select: 'Select',
    com_ui_confirm: 'Confirm',
    com_ui_submit: 'Submit',
    com_ui_loading: 'Loading...',
    com_ui_error: 'Error',
  };
  return dict[key] ?? key;
});

jest.mock('~/Providers/MessagesViewContext', () => ({
  useMessagesOperations: () => ({
    getMessages: mockGetMessages,
    setMessages: mockSetMessages,
  }),
}));

jest.mock('~/assistant-formal/api', () => ({
  confirmAssistantFormalTurn: (...args: unknown[]) => mockConfirmAssistantFormalTurn(...args),
  commitAssistantFormalTurn: (...args: unknown[]) => mockCommitAssistantFormalTurn(...args),
}));

describe('AssistantFormalMessage', () => {
  beforeEach(() => {
    jest.clearAllMocks();
  });

  it('renders candidates and confirms selected candidate in-place', async () => {
    const message = {
      messageId: 'msg-1',
      text: '检测到多个候选父组织，请选择一个后继续。',
      sender: 'Assistant',
      isCreatedByUser: false,
      parentMessageId: 'user-1',
      conversationId: null,
      error: false,
      assistantFormalPayload: {
        kind: 'assistant_formal',
        backendConversationId: 'conv-1',
        turnId: 'turn-1',
        requestId: 'req-1',
        traceId: 'trace-1',
        messageId: 'msg-1',
        bindingKey: 'conv-1::turn-1::req-1',
        state: 'validated',
        phase: 'await_candidate_pick',
        missingFields: [],
        candidates: [
          {
            candidate_id: 'cand-1',
            candidate_code: 'FLOWER-A',
            name: '鲜花组织',
            path: '/鲜花组织',
            as_of: '2026-01-01',
            is_active: true,
            match_score: 0.99,
          },
          {
            candidate_id: 'cand-2',
            candidate_code: 'FLOWER-B',
            name: '花束组织',
            path: '/花束组织',
            as_of: '2026-01-01',
            is_active: true,
            match_score: 0.88,
          },
        ],
      },
    } as TMessage & { assistantFormalPayload: any };

    mockGetMessages.mockReturnValue([message]);
    mockConfirmAssistantFormalTurn.mockResolvedValue({
      conversation_id: 'conv-1',
      turns: [
        {
          turn_id: 'turn-1',
          state: 'validated',
          phase: 'await_commit_confirm',
          request_id: 'req-1',
          trace_id: 'trace-1',
          selected_candidate_id: 'cand-1',
          pending_draft_summary: '已选择鲜花组织，等待提交确认。',
          candidates: message.assistantFormalPayload.candidates,
          missing_fields: [],
        },
      ],
    });

    render(<AssistantFormalMessage message={message as any} />);

    expect(screen.getByText('鲜花组织')).toBeInTheDocument();
    await userEvent.click(screen.getAllByRole('button')[0]);

    await waitFor(() => {
      expect(mockConfirmAssistantFormalTurn).toHaveBeenCalledWith('conv-1', 'turn-1', 'cand-1');
    });
    expect(mockSetMessages).toHaveBeenCalled();
    expect(mockCommitAssistantFormalTurn).not.toHaveBeenCalled();
  });


  it('renders failure state inside the official bubble without action buttons', () => {
    const message = {
      messageId: 'msg-failed',
      text: '提交失败',
      sender: 'Assistant',
      isCreatedByUser: false,
      parentMessageId: 'user-1',
      conversationId: null,
      error: false,
      assistantFormalPayload: {
        kind: 'assistant_formal',
        backendConversationId: 'conv-1',
        turnId: 'turn-2',
        requestId: 'req-2',
        traceId: 'trace-2',
        messageId: 'msg-failed',
        bindingKey: 'conv-1::turn-2::req-2',
        state: 'failed',
        phase: 'failed',
        errorCode: 'assistant_commit_failed',
        missingFields: [],
        candidates: [],
        reply: {
          text: '提交失败',
          kind: 'error',
          stage: 'commit_failed',
        },
      },
    } as TMessage & { assistantFormalPayload: any };

    mockGetMessages.mockReturnValue([message]);
    const { container } = render(<AssistantFormalMessage message={message as any} />);

    expect(screen.getByText('提交失败')).toBeInTheDocument();
    expect(screen.getByText('Error: assistant_commit_failed')).toBeInTheDocument();
    expect(screen.queryByRole('button', { name: 'Submit' })).not.toBeInTheDocument();
    expect(container.querySelector('[data-assistant-binding-key="conv-1::turn-2::req-2"]')).not.toBeNull();
  });

  it('exposes binding data attributes on the official assistant bubble', () => {
    const message = {
      messageId: 'msg-1',
      text: '处理中...',
      sender: 'Assistant',
      isCreatedByUser: false,
      parentMessageId: 'user-1',
      conversationId: null,
      error: false,
      assistantFormalPayload: {
        kind: 'assistant_formal',
        backendConversationId: 'conv-1',
        turnId: 'turn-1',
        requestId: 'req-1',
        traceId: 'trace-1',
        messageId: 'msg-1',
        bindingKey: 'conv-1::turn-1::req-1',
        state: 'validated',
        missingFields: [],
        candidates: [],
      },
    } as TMessage & { assistantFormalPayload: any };

    mockGetMessages.mockReturnValue([message]);
    const { container } = render(<AssistantFormalMessage message={message as any} />);
    const bubble = container.querySelector('[data-assistant-binding-key="conv-1::turn-1::req-1"]');

    expect(bubble).not.toBeNull();
    expect(bubble).toHaveAttribute('data-assistant-conversation-id', 'conv-1');
    expect(bubble).toHaveAttribute('data-assistant-turn-id', 'turn-1');
    expect(bubble).toHaveAttribute('data-assistant-request-id', 'req-1');
    expect(bubble).toHaveAttribute('data-assistant-message-id', 'msg-1');
  });
});
