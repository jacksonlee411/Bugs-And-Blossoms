import { useEffect, useState } from 'react';
import { v4 } from 'uuid';
import { SSE } from 'sse.js';
import { useSetRecoilState } from 'recoil';
import {
  request,
  Constants,
  /* @ts-ignore */
  createPayload,
  LocalStorageKeys,
  removeNullishValues,
} from 'librechat-data-provider';
import type { TMessage, TPayload, TSubmission, EventSubmission } from 'librechat-data-provider';
import type { EventHandlerParams } from './useEventHandlers';
import type { TResData } from '~/common';
import { useGenTitleMutation, useGetStartupConfig, useGetUserBalance } from '~/data-provider';
import { useAuthContext } from '~/hooks/AuthContext';
import {
  createAssistantFormalConversation,
  createAssistantFormalTurn,
  renderAssistantFormalReply,
  type AssistantFormalAPIError,
} from '~/assistant-formal/api';
import {
  buildAssistantFormalFailurePayload,
  buildAssistantFormalPayload,
  buildAssistantFormalPendingPayload,
  clearStoredAssistantFormalConversationId,
  detectAssistantFormalLocale,
  getStoredAssistantFormalConversationId,
  isFormalAssistantPath,
  latestAssistantFormalTurn,
  upsertAssistantFormalMessage,
  resolveAssistantFormalText,
  setStoredAssistantFormalConversationId,
  shouldResetAssistantFormalConversation,
} from '~/assistant-formal/runtime';
import useEventHandlers from './useEventHandlers';
import store from '~/store';

const clearDraft = (conversationId?: string | null) => {
  if (conversationId) {
    localStorage.removeItem(`${LocalStorageKeys.TEXT_DRAFT}${conversationId}`);
    localStorage.removeItem(`${LocalStorageKeys.FILES_DRAFT}${conversationId}`);
  } else {
    localStorage.removeItem(`${LocalStorageKeys.TEXT_DRAFT}${Constants.NEW_CONVO}`);
    localStorage.removeItem(`${LocalStorageKeys.FILES_DRAFT}${Constants.NEW_CONVO}`);
  }
};

type ChatHelpers = Pick<
  EventHandlerParams,
  | 'setMessages'
  | 'getMessages'
  | 'setConversation'
  | 'setIsSubmitting'
  | 'newConversation'
  | 'resetLatestMessage'
>;

export default function useSSE(
  submission: TSubmission | null,
  chatHelpers: ChatHelpers,
  isAddedRequest = false,
  runIndex = 0,
) {
  const genTitle = useGenTitleMutation();
  const setActiveRunId = useSetRecoilState(store.activeRunFamily(runIndex));

  const { token, isAuthenticated } = useAuthContext();
  const [completed, setCompleted] = useState(new Set());
  const setAbortScroll = useSetRecoilState(store.abortScrollFamily(runIndex));
  const setShowStopButton = useSetRecoilState(store.showStopButtonByIndex(runIndex));

  const {
    setMessages,
    getMessages,
    setConversation,
    setIsSubmitting,
    newConversation,
    resetLatestMessage,
  } = chatHelpers;

  const {
    clearStepMaps,
    stepHandler,
    syncHandler,
    finalHandler,
    errorHandler,
    messageHandler,
    contentHandler,
    createdHandler,
    attachmentHandler,
    abortConversation,
  } = useEventHandlers({
    genTitle,
    setMessages,
    getMessages,
    setCompleted,
    isAddedRequest,
    setConversation,
    setIsSubmitting,
    newConversation,
    setShowStopButton,
    resetLatestMessage,
  });

  const { data: startupConfig } = useGetStartupConfig();
  const balanceQuery = useGetUserBalance({
    enabled: !!isAuthenticated && startupConfig?.balance?.enabled,
  });

  useEffect(() => {
    if (submission == null || Object.keys(submission).length === 0) {
      return;
    }

    if (isFormalAssistantPath()) {
      const assistantMessageId = submission.initialResponse?.messageId;
      const frontendUserMessageId = submission.userMessage.messageId;
      const locale = detectAssistantFormalLocale();
      let cancelled = false;
      let currentBindingKey = '';
      let currentPayload = assistantMessageId
        ? buildAssistantFormalPendingPayload({
            messageId: assistantMessageId,
            frontendUserMessageId,
          })
        : undefined;

      const patchFormalMessage = (patch: Partial<TMessage>) => {
        if (!assistantMessageId) {
          return;
        }
        setMessages(
          upsertAssistantFormalMessage(
            getMessages() ?? [],
            {
              messageId: assistantMessageId,
              bindingKey: currentBindingKey || undefined,
            },
            patch,
          ),
        );
      };

      const runFormalSubmission = async () => {
        setIsSubmitting(true);
        setShowStopButton(true);
        setAbortScroll(false);

        if (shouldResetAssistantFormalConversation(submission)) {
          clearStoredAssistantFormalConversationId();
        }

        patchFormalMessage({
          text: locale === 'en' ? 'Processing...' : '处理中...',
          assistantFormalPayload: currentPayload,
          assistantFormalPending: true,
          error: false,
        } as Partial<TMessage>);

        try {
          let backendConversationId = getStoredAssistantFormalConversationId();
          if (!backendConversationId) {
            const conversation = await createAssistantFormalConversation(token);
            backendConversationId = conversation.conversation_id;
            setStoredAssistantFormalConversationId(backendConversationId);
          }

          let conversation;
          try {
            conversation = await createAssistantFormalTurn(
              backendConversationId,
              submission.userMessage.text,
              token,
            );
          } catch (error) {
            if (backendConversationId) {
              clearStoredAssistantFormalConversationId();
              const retryConversation = await createAssistantFormalConversation(token);
              backendConversationId = retryConversation.conversation_id;
              setStoredAssistantFormalConversationId(backendConversationId);
              conversation = await createAssistantFormalTurn(
                backendConversationId,
                submission.userMessage.text,
                token,
              );
            } else {
              throw error;
            }
          }

          const turn = latestAssistantFormalTurn(conversation);
          if (!turn) {
            throw new Error('assistant turn missing');
          }
          currentPayload = buildAssistantFormalPayload(conversation, turn, turn.reply_nlg, {
            messageId: assistantMessageId,
            frontendUserMessageId,
          });
          currentBindingKey = currentPayload.bindingKey;
          const reply = turn.reply_nlg ??
            (await renderAssistantFormalReply(conversation.conversation_id, turn.turn_id, locale, token));
          const payload = buildAssistantFormalPayload(conversation, turn, reply, {
            messageId: assistantMessageId,
            frontendUserMessageId,
          });
          currentPayload = payload;
          currentBindingKey = payload.bindingKey;
          if (cancelled) {
            return;
          }
          clearDraft(submission.conversation?.conversationId);
          patchFormalMessage({
            text: resolveAssistantFormalText(payload),
            assistantFormalPayload: payload,
            assistantFormalPending: false,
            error: false,
          } as Partial<TMessage>);
        } catch (error) {
          if (cancelled) {
            return;
          }
          const failurePayload = buildAssistantFormalFailurePayload(
            currentPayload ?? {},
            error as AssistantFormalAPIError,
          );
          currentBindingKey = failurePayload.bindingKey;
          patchFormalMessage({
            text: resolveAssistantFormalText(failurePayload),
            assistantFormalPayload: failurePayload,
            assistantFormalPending: false,
            error: false,
          } as Partial<TMessage>);
        } finally {
          if (!cancelled) {
            setIsSubmitting(false);
            setShowStopButton(false);
          }
        }
      };

      void runFormalSubmission();

      return () => {
        cancelled = true;
        setShowStopButton(false);
      };
    }

    let { userMessage } = submission;

    const payloadData = createPayload(submission);
    let { payload } = payloadData;
    payload = removeNullishValues(payload) as TPayload;

    let textIndex = null;
    clearStepMaps();

    const sse = new SSE(payloadData.server, {
      payload: JSON.stringify(payload),
      headers: { 'Content-Type': 'application/json', Authorization: `Bearer ${token}` },
    });

    sse.addEventListener('attachment', (e: MessageEvent) => {
      try {
        const data = JSON.parse(e.data);
        attachmentHandler({ data, submission: submission as EventSubmission });
      } catch (error) {
        console.error(error);
      }
    });

    sse.addEventListener('message', (e: MessageEvent) => {
      const data = JSON.parse(e.data);

      if (data.final != null) {
        clearDraft(submission.conversation?.conversationId);
        const { plugins } = data;
        finalHandler(data, { ...submission, plugins } as EventSubmission);
        (startupConfig?.balance?.enabled ?? false) && balanceQuery.refetch();
        console.log('final', data);
        return;
      } else if (data.created != null) {
        const runId = v4();
        setActiveRunId(runId);
        userMessage = {
          ...userMessage,
          ...data.message,
          overrideParentMessageId: userMessage.overrideParentMessageId,
        };

        createdHandler(data, { ...submission, userMessage } as EventSubmission);
      } else if (data.event != null) {
        stepHandler(data, { ...submission, userMessage } as EventSubmission);
      } else if (data.sync != null) {
        const runId = v4();
        setActiveRunId(runId);
        syncHandler(data, { ...submission, userMessage } as EventSubmission);
      } else if (data.type != null) {
        const { text, index } = data;
        if (text != null && index !== textIndex) {
          textIndex = index;
        }

        contentHandler({ data, submission: submission as EventSubmission });
      } else {
        const text = data.text ?? data.response;
        const { plugin, plugins } = data;

        const initialResponse = {
          ...(submission.initialResponse as TMessage),
          parentMessageId: data.parentMessageId,
          messageId: data.messageId,
        };

        if (data.message != null) {
          messageHandler(text, { ...submission, plugin, plugins, userMessage, initialResponse });
        }
      }
    });

    sse.addEventListener('open', () => {
      setAbortScroll(false);
      console.log('connection is opened');
    });

    sse.addEventListener('cancel', async () => {
      const streamKey = (submission as TSubmission | null)?.['initialResponse']?.messageId;
      if (completed.has(streamKey)) {
        setIsSubmitting(false);
        setCompleted((prev) => {
          prev.delete(streamKey);
          return new Set(prev);
        });
        return;
      }

      setCompleted((prev) => new Set(prev.add(streamKey)));
      const latestMessages = getMessages();
      const conversationId = latestMessages?.[latestMessages.length - 1]?.conversationId;
      return await abortConversation(
        conversationId ??
          userMessage.conversationId ??
          submission.conversation?.conversationId ??
          '',
        submission as EventSubmission,
        latestMessages,
      );
    });

    sse.addEventListener('error', async (e: MessageEvent) => {
      /* @ts-ignore */
      if (e.responseCode === 401) {
        try {
          const refreshResponse = await request.refreshToken();
          const token = refreshResponse?.token ?? '';
          if (!token) {
            throw new Error('Token refresh failed.');
          }
          sse.headers = {
            'Content-Type': 'application/json',
            Authorization: `Bearer ${token}`,
          };

          request.dispatchTokenUpdatedEvent(token);
          sse.stream();
          return;
        } catch (error) {
          console.log(error);
        }
      }

      console.log('error in server stream.');
      (startupConfig?.balance?.enabled ?? false) && balanceQuery.refetch();

      let data: TResData | undefined = undefined;
      try {
        data = JSON.parse(e.data) as TResData;
      } catch (error) {
        console.error(error);
        console.log(e);
        setIsSubmitting(false);
      }

      errorHandler({ data, submission: { ...submission, userMessage } as EventSubmission });
    });

    setIsSubmitting(true);
    sse.stream();

    return () => {
      const isCancelled = sse.readyState <= 1;
      sse.close();
      if (isCancelled) {
        const e = new Event('cancel');
        /* @ts-ignore */
        sse.dispatchEvent(e);
      }
    };
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [submission]);
}
