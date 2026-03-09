import type { TMessage } from 'librechat-data-provider';
import {
  assistantFormalBindingKey,
  buildAssistantFormalFailurePayload,
  buildAssistantFormalPayload,
  isAssistantFormalMessage,
  isFormalAssistantPath,
  patchAssistantFormalMessage,
  resolveAssistantFormalText,
  upsertAssistantFormalMessage,
} from '~/assistant-formal/runtime';

describe('assistant formal runtime', () => {
  it('detects formal assistant path', () => {
    expect(isFormalAssistantPath('/app/assistant/librechat/c/new')).toBe(true);
    expect(isFormalAssistantPath('/c/new')).toBe(false);
  });

  it('builds payload and resolves visible text from reply', () => {
    const payload = buildAssistantFormalPayload(
      {
        conversation_id: 'conv-1',
        turns: [],
      },
      {
        turn_id: 'turn-1',
        state: 'validated',
        phase: 'await_candidate_confirm',
        pending_draft_summary: 'draft summary',
        missing_fields: [],
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
        ],
        selected_candidate_id: 'cand-1',
      },
      {
        text: '候选已确认，可以继续提交。',
        kind: 'info',
        stage: 'candidate_confirm',
      },
    );

    expect(payload.backendConversationId).toBe('conv-1');
    expect(payload.turnId).toBe('turn-1');
    expect(payload.selectedCandidateId).toBe('cand-1');
    expect(resolveAssistantFormalText(payload)).toBe('候选已确认，可以继续提交。');
  });



  it('adds stable binding key and request mapping', () => {
    const payload = buildAssistantFormalPayload(
      { conversation_id: 'conv-1', turns: [] },
      {
        turn_id: 'turn-1',
        state: 'validated',
        request_id: 'req-1',
        trace_id: 'trace-1',
      },
      undefined,
      { messageId: 'msg-1', frontendUserMessageId: 'user-1' },
    );

    expect(payload.messageId).toBe('msg-1');
    expect(payload.requestId).toBe('req-1');
    expect(payload.traceId).toBe('trace-1');
    expect(payload.bindingKey).toBe(
      assistantFormalBindingKey({
        backendConversationId: 'conv-1',
        turnId: 'turn-1',
        requestId: 'req-1',
      }),
    );
  });

  it('upserts by binding key and removes duplicate assistant bubbles', () => {
    const bindingKey = assistantFormalBindingKey({
      backendConversationId: 'conv-1',
      turnId: 'turn-1',
      requestId: 'req-1',
    });
    const messages: TMessage[] = [
      {
        messageId: 'msg-1',
        text: '旧内容',
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
          bindingKey,
          state: 'validated',
          missingFields: [],
          candidates: [],
        },
      } as TMessage,
      {
        messageId: 'msg-dup',
        text: '重复气泡',
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
          messageId: 'msg-dup',
          bindingKey,
          state: 'validated',
          missingFields: [],
          candidates: [],
        },
      } as TMessage,
    ];

    const next = upsertAssistantFormalMessage(messages, { bindingKey }, { text: '新内容' });

    expect(next).toHaveLength(1);
    expect(next[0].messageId).toBe('msg-1');
    expect(next[0].text).toBe('新内容');
  });

  it('patches assistant message with runtime payload', () => {
    const messages: TMessage[] = [
      {
        messageId: 'msg-1',
        text: '',
        sender: 'Assistant',
        isCreatedByUser: false,
        parentMessageId: 'parent-1',
        conversationId: null,
        error: false,
      },
    ];
    const failurePayload = buildAssistantFormalFailurePayload(
      {
        backendConversationId: 'conv-1',
        turnId: 'turn-1',
      },
      { code: 'assistant_commit_failed', message: '提交失败' },
    );
    const next = patchAssistantFormalMessage(messages, 'msg-1', {
      text: resolveAssistantFormalText(failurePayload),
      assistantFormalPayload: failurePayload,
    });

    expect(isAssistantFormalMessage(next[0])).toBe(true);
    expect(resolveAssistantFormalText((next[0] as any).assistantFormalPayload)).toBe('提交失败');
  });
});
