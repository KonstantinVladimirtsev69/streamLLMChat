"use client";

import React, { useEffect } from "react";
import Header from "@/components/header/Header";
import ChatWindow from "@/components/chat/ChatWindow";
import ChatInput from "@/components/chat/ChatInput";
import { useChatStore } from "@/store/useChatStore";
import { useChatStream } from "@/hooks/useChatStream";
import { apiFetch } from "@/lib/api";
import type { Message } from "@/types/chat";

export default function ChatPage() {
  const { activeChatId, setMessages } = useChatStore();
  const { sendMessage, abortStream } = useChatStream();

  // Load message history when activeChatId changes
  useEffect(() => {
    let isMounted = true;
    if (!activeChatId) {
      setMessages([]);
      return;
    }

    const loadMessages = async () => {
      try {
        const data = await apiFetch<{ messages: Message[] } | Message[]>(
          `/api/v1/chats/${activeChatId}/messages`
        );
        const msgList = Array.isArray(data) ? data : data?.messages || [];
        if (isMounted) {
          setMessages(msgList);
        }
      } catch (err) {
        console.error("Failed to load messages:", err);
      }
    };

    loadMessages();
    return () => {
      isMounted = false;
    };
  }, [activeChatId, setMessages]);

  return (
    <div className="flex flex-col h-full w-full overflow-hidden bg-[#090a0f]">
      {/* Top Navigation Header */}
      <Header />

      {/* Center Chat Message Stream */}
      <ChatWindow onSelectPrompt={(prompt) => sendMessage(prompt)} />

      {/* Bottom Floating Input Box */}
      <ChatInput
        onSend={(text) => sendMessage(text)}
        onAbort={() => abortStream()}
      />
    </div>
  );
}
