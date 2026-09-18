import { useRef, useCallback, useEffect } from "react";
import { useChatStore } from "@/store/useChatStore";
import { apiFetch } from "@/lib/api";
import type { Chat, StreamEvent, User } from "@/types/chat";

export function useChatStream() {
  const {
    activeChatId,
    setActiveChatId,
    addChat,
    selectedModel,
    addMessage,
    updateStreamingMessage,
    finalizeMessage,
    setIsStreaming,
    applyDeduction,
    setUser,
  } = useChatStore();

  const abortControllerRef = useRef<AbortController | null>(null);

  // Abort active stream
  const abortStream = useCallback(() => {
    if (abortControllerRef.current) {
      abortControllerRef.current.abort();
      abortControllerRef.current = null;
      setIsStreaming(false, null);
    }
  }, [setIsStreaming]);

  // Clean up on unmount
  useEffect(() => {
    return () => {
      if (abortControllerRef.current) {
        abortControllerRef.current.abort();
      }
    };
  }, []);

  const sendMessage = useCallback(
    async (content: string) => {
      const trimmed = content.trim();
      if (!trimmed) return;

      let chatId = activeChatId;

      // If no active chat, create one first
      if (!chatId) {
        try {
          const newChat = await apiFetch<Chat>("/api/v1/chats", {
            method: "POST",
            body: JSON.stringify({ model: selectedModel || "gpt-4o-mini" }),
          });
          addChat(newChat);
          setActiveChatId(newChat.id);
          chatId = newChat.id;
        } catch (err) {
          console.error("Failed to create chat before streaming:", err);
          return;
        }
      }

      // Add user message to UI
      const userMsgId = `user_${Date.now()}`;
      addMessage({
        id: userMsgId,
        chat_id: chatId,
        role: "user",
        content: trimmed,
        created_at: new Date().toISOString(),
      });

      // Prepare assistant placeholder message
      const assistantMsgId = `assistant_${Date.now()}`;
      addMessage({
        id: assistantMsgId,
        chat_id: chatId,
        role: "assistant",
        content: "",
        model: selectedModel,
        created_at: new Date().toISOString(),
      });

      const controller = new AbortController();
      abortControllerRef.current = controller;
      setIsStreaming(true, assistantMsgId);

      try {
        const response = await fetch("/api/v1/chat/stream", {
          method: "POST",
          credentials: "include",
          headers: {
            "Content-Type": "application/json",
          },
          body: JSON.stringify({
            chat_id: chatId,
            model: selectedModel,
            content: trimmed,
          }),
          signal: controller.signal,
        });

        if (response.status === 402) {
          finalizeMessage(assistantMsgId, {
            content: "⚠️ Недостаточно средств на балансе. Пожалуйста, пригласите друзей для получения бонуса.",
          });
          setIsStreaming(false, null);
          return;
        }

        if (response.status === 401) {
          // eslint-disable-next-line @next/next/no-location-assign-relative-destination
          window.location.href = "/";
          return;
        }

        if (!response.ok || !response.body) {
          throw new Error(`Ошибка сервера: HTTP ${response.status}`);
        }

        const reader = response.body.getReader();
        const decoder = new TextDecoder("utf-8");
        let buffer = "";

        while (true) {
          const { done, value } = await reader.read();
          if (done) break;

          buffer += decoder.decode(value, { stream: true });
          const lines = buffer.split("\n");
          // Keep the last incomplete fragment in buffer
          buffer = lines.pop() || "";

          for (const line of lines) {
            const trimmedLine = line.trim();
            if (!trimmedLine || !trimmedLine.startsWith("data:")) continue;

            const jsonStr = trimmedLine.replace(/^data:\s*/, "");
            try {
              const event: StreamEvent = JSON.parse(jsonStr);

              if (event.type === "delta" && event.delta) {
                updateStreamingMessage(assistantMsgId, event.delta);
              } else if (event.type === "done") {
                if (event.usage) {
                  finalizeMessage(assistantMsgId, {
                    prompt_tokens: event.usage.prompt_tokens,
                    completion_tokens: event.usage.completion_tokens,
                    cost_kopecks: event.usage.cost_kopecks,
                  });
                  applyDeduction(event.usage.cost_kopecks);
                  // Refresh user balance from server asynchronously
                  apiFetch<User>("/api/v1/auth/me")
                    .then((fresh) => fresh && setUser(fresh))
                    .catch(() => {});
                }
              } else if (event.type === "error") {
                finalizeMessage(assistantMsgId, {
                  content: `⚠️ Ошибка: ${event.error || "Неизвестная ошибка модели"}`,
                });
              }
            } catch (parseErr) {
              console.warn("Failed to parse SSE event chunk:", jsonStr, parseErr);
            }
          }
        }
      } catch (err: unknown) {
        if ((err as Error)?.name === "AbortError") {
          // Normal user abort
        } else {
          finalizeMessage(assistantMsgId, {
            content: `⚠️ Ошибка соединения: ${(err as Error)?.message || "Сбой стриминга"}`,
          });
        }
      } finally {
        setIsStreaming(false, null);
        abortControllerRef.current = null;
      }
    },
    [
      activeChatId,
      selectedModel,
      addChat,
      setActiveChatId,
      addMessage,
      updateStreamingMessage,
      finalizeMessage,
      setIsStreaming,
      applyDeduction,
      setUser,
    ]
  );

  return {
    sendMessage,
    abortStream,
  };
}
