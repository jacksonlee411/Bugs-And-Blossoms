import { memo, useCallback, useMemo, useState } from 'react';
import useLocalize from '~/hooks/useLocalize';
import { useMessagesOperations } from '~/Providers/MessagesViewContext';
import Container from './Container';
import {
  AssistantFormalMessage as AssistantFormalMessageType,
  buildAssistantFormalFailurePayload,
  buildAssistantFormalPayload,
  latestAssistantFormalTurn,
  resolveAssistantFormalText,
  upsertAssistantFormalMessage,
} from '~/assistant-formal/runtime';
import {
  confirmAssistantFormalTurn,
  commitAssistantFormalTurn,
  type AssistantFormalAPIError,
} from '~/assistant-formal/api';

function AssistantFormalMessage({ message }: { message: AssistantFormalMessageType }) {
  const localize = useLocalize();
  const { getMessages, setMessages } = useMessagesOperations();
  const [busy, setBusy] = useState(false);
  const payload = message.assistantFormalPayload;

  const patchMessage = useCallback(
    (patch: Partial<AssistantFormalMessageType>) => {
      setMessages(
        upsertAssistantFormalMessage(
          getMessages() ?? [],
          {
            messageId: message.messageId,
            bindingKey: payload?.bindingKey,
          },
          patch,
        ),
      );
    },
    [getMessages, message.messageId, payload?.bindingKey, setMessages],
  );

  const runMutation = useCallback(
    async (mode: 'confirm' | 'commit', candidateId?: string) => {
      if (!payload || !payload.backendConversationId || !payload.turnId) {
        return;
      }
      setBusy(true);
      patchMessage({ assistantFormalPending: true });
      let currentPayload = payload;
      try {
        const conversation =
          mode === 'confirm'
            ? await confirmAssistantFormalTurn(
                payload.backendConversationId,
                payload.turnId,
                candidateId ?? '',
              )
            : await commitAssistantFormalTurn(payload.backendConversationId, payload.turnId);
        const turn = latestAssistantFormalTurn(conversation);
        if (!turn) {
          throw new Error('assistant turn missing');
        }
        const nextPayload = buildAssistantFormalPayload(conversation, turn, turn.reply_nlg, {
          messageId: payload.messageId || message.messageId,
          frontendUserMessageId: payload.frontendUserMessageId,
        });
        currentPayload = nextPayload;
        patchMessage({
          text: resolveAssistantFormalText(nextPayload),
          assistantFormalPayload: nextPayload,
          assistantFormalPending: false,
          error: false,
        });
      } catch (error) {
        const failurePayload = buildAssistantFormalFailurePayload(
          currentPayload,
          error as AssistantFormalAPIError,
        );
        patchMessage({
          text: resolveAssistantFormalText(failurePayload),
          assistantFormalPayload: failurePayload,
          assistantFormalPending: false,
          error: false,
        });
      } finally {
        setBusy(false);
      }
    },
    [patchMessage, payload],
  );

  const selectedCandidate = useMemo(
    () => payload?.candidates.find((candidate) => candidate.candidate_id === payload.selectedCandidateId),
    [payload],
  );

  if (!payload) {
    return null;
  }

  const canSelectCandidate =
    !busy &&
    !message.assistantFormalPending &&
    !payload.errorCode &&
    payload.phase === 'await_candidate_pick' &&
    payload.candidates.length > 0;
  const canCommit =
    !busy &&
    !message.assistantFormalPending &&
    !payload.errorCode &&
    !payload.commitResult &&
    payload.phase === 'await_commit_confirm';
  const toneClasses =
    payload.reply?.kind === 'error' || payload.errorCode
      ? 'border-red-500/20 bg-red-500/5 text-gray-700 dark:text-gray-100'
      : payload.reply?.kind === 'success'
        ? 'border-emerald-500/20 bg-emerald-500/5 text-gray-700 dark:text-gray-100'
        : 'border-border-light bg-surface-primary-alt text-gray-700 dark:text-gray-100';

  return (
    <Container message={message}>
      <div
        className="flex flex-col gap-3"
        data-assistant-conversation-id={payload.backendConversationId || undefined}
        data-assistant-turn-id={payload.turnId || undefined}
        data-assistant-request-id={payload.requestId || undefined}
        data-assistant-message-id={payload.messageId || message.messageId}
        data-assistant-binding-key={payload.bindingKey || undefined}
      >
        <div className={`rounded-xl border px-3 py-3 text-sm ${toneClasses}`}>
          <div className="whitespace-pre-wrap">{resolveAssistantFormalText(payload)}</div>
          {(busy || message.assistantFormalPending) && (
            <div className="mt-2 text-xs opacity-70">{localize('com_ui_loading')}</div>
          )}
          {payload.errorCode && (
            <div className="mt-2 text-xs opacity-70">
              {localize('com_ui_error')}: {payload.errorCode}
            </div>
          )}
        </div>

        {payload.pendingDraftSummary && payload.pendingDraftSummary !== resolveAssistantFormalText(payload) && (
          <div className="rounded-xl border border-border-light bg-surface-primary px-3 py-3 text-sm text-text-primary">
            {payload.pendingDraftSummary}
          </div>
        )}

        {payload.phase === 'await_missing_fields' && payload.missingFields.length > 0 && (
          <div className="rounded-xl border border-amber-500/20 bg-amber-500/5 px-3 py-3 text-sm text-text-primary">
            <div className="font-medium">Missing fields</div>
            <ul className="mt-2 list-disc space-y-1 pl-5">
              {payload.missingFields.map((field) => (
                <li key={field}>{field}</li>
              ))}
            </ul>
          </div>
        )}

        {payload.phase === 'await_candidate_pick' && payload.candidates.length > 0 && (
          <div className="flex flex-col gap-2">
            {payload.candidates.map((candidate) => {
              const isSelected = candidate.candidate_id === payload.selectedCandidateId;
              return (
                <div
                  key={candidate.candidate_id}
                  className={`rounded-xl border px-3 py-3 text-sm ${
                    isSelected
                      ? 'border-[#09a7a3]/40 bg-[#09a7a3]/10'
                      : 'border-border-light bg-surface-primary'
                  }`}
                >
                  <div className="font-medium text-text-primary">{candidate.name || candidate.candidate_code}</div>
                  <div className="mt-1 text-xs text-text-secondary">
                    {candidate.path || candidate.candidate_code}
                  </div>
                  <div className="mt-1 text-xs text-text-secondary">
                    {candidate.candidate_code} · score {candidate.match_score}
                  </div>
                  <div className="mt-3 flex gap-2">
                    {canSelectCandidate && (
                      <button
                        type="button"
                        className="rounded-md bg-[#09a7a3] px-3 py-1.5 text-xs font-medium text-white disabled:opacity-50"
                        onClick={() => void runMutation('confirm', candidate.candidate_id)}
                        disabled={busy || message.assistantFormalPending}
                      >
                        {localize('com_ui_select')} + {localize('com_ui_confirm')}
                      </button>
                    )}
                    {isSelected && (
                      <span className="rounded-md border border-[#09a7a3]/40 px-2 py-1 text-xs text-text-primary">
                        {localize('com_ui_confirm')}
                      </span>
                    )}
                  </div>
                </div>
              );
            })}
          </div>
        )}

        {selectedCandidate && payload.phase !== 'await_candidate_pick' && (
          <div className="rounded-xl border border-[#09a7a3]/30 bg-[#09a7a3]/10 px-3 py-3 text-sm text-text-primary">
            Selected: {selectedCandidate.name || selectedCandidate.candidate_code}
          </div>
        )}

        {payload.commitResult && (
          <div className="rounded-xl border border-emerald-500/20 bg-emerald-500/5 px-3 py-3 text-sm text-text-primary">
            <div>org_code: {payload.commitResult.org_code}</div>
            <div>parent_org_code: {payload.commitResult.parent_org_code}</div>
            <div>effective_date: {payload.commitResult.effective_date}</div>
            <div>event_type: {payload.commitResult.event_type}</div>
          </div>
        )}

        {canCommit && (
          <div>
            <button
              type="button"
              className="rounded-md bg-[#09a7a3] px-3 py-2 text-sm font-medium text-white disabled:opacity-50"
              onClick={() => void runMutation('commit')}
              disabled={busy || message.assistantFormalPending}
            >
              {localize('com_ui_submit')}
            </button>
          </div>
        )}
      </div>
    </Container>
  );
}

export default memo(AssistantFormalMessage);
