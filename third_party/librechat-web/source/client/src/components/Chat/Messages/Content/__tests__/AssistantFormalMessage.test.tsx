import React from 'react';
import { render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import type { TMessage } from 'librechat-data-provider';
import AssistantFormalMessage from '~/components/Chat/Messages/Content/AssistantFormalMessage';

const mockGetMessages = jest.fn();
const mockSetMessages = jest.fn();
const mockConfirmAssistantFormalTurn = jest.fn();
const mockCommitAssistantFormalTurn = jest.fn();
const mockRenderAssistantFormalReply = jest.fn();

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
  renderAssistantFormalReply: (...args: unknown[]) => mockRenderAssistantFormalReply(...args),
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
          state: 'confirmed',
          phase: 'await_candidate_confirm',
          selected_candidate_id: 'cand-1',
          candidates: message.assistantFormalPayload.candidates,
          missing_fields: [],
        },
      ],
    });
    mockRenderAssistantFormalReply.mockResolvedValue({
      text: '候选已确认，可以继续提交。',
      kind: 'info',
      stage: 'candidate_confirm',
    });

    render(<AssistantFormalMessage message={message as any} />);

    expect(screen.getByText('鲜花组织')).toBeInTheDocument();
    await userEvent.click(screen.getAllByRole('button')[0]);

    await waitFor(() => {
      expect(mockConfirmAssistantFormalTurn).toHaveBeenCalledWith('conv-1', 'turn-1', 'cand-1');
    });
    expect(mockRenderAssistantFormalReply).toHaveBeenCalledWith('conv-1', 'turn-1', 'zh');
    expect(mockSetMessages).toHaveBeenCalled();
    expect(mockCommitAssistantFormalTurn).not.toHaveBeenCalled();
  });
});
