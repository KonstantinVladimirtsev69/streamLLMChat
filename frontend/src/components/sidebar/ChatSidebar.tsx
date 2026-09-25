import React, { useEffect, useState } from "react";
import { Plus, Sparkles, Loader2 } from "lucide-react";
import { useChatStore } from "@/store/useChatStore";
import { apiFetch } from "@/lib/api";
import type { Chat } from "@/types/chat";
import ChatItem from "./ChatItem";
import UserProfileBar from "./UserProfileBar";

interface ChatSidebarProps {
  onItemClick?: () => void;
}

export default function ChatSidebar({ onItemClick }: ChatSidebarProps) {
  const {
    chats,
    setChats,
    addChat,
    removeChat,
    activeChatId,
    setActiveChatId,
    selectedModel,
    setMobileDrawerOpen,
  } = useChatStore();

  const [isLoading, setIsLoading] = useState(false);
  const [isCreating, setIsCreating] = useState(false);

  useEffect(() => {
    let isMounted = true;
    const loadChats = async () => {
      try {
        setIsLoading(true);
        const data = await apiFetch<{ chats: Chat[] } | Chat[]>("/api/v1/chats");
        const chatList = Array.isArray(data) ? data : data?.chats || [];
        if (isMounted) {
          setChats(chatList);
          if (!activeChatId && chatList.length > 0) {
            setActiveChatId(chatList[0].id);
          }
        }
      } catch (err) {
        console.error("Failed to load chats:", err);
      } finally {
        if (isMounted) setIsLoading(false);
      }
    };

    loadChats();
    return () => {
      isMounted = false;
    };
  }, [setChats, activeChatId, setActiveChatId]);

  const handleCreateChat = async () => {
    try {
      setIsCreating(true);
      const res = await apiFetch<{ chat: Chat } | Chat>("/api/v1/chats", {
        method: "POST",
        body: JSON.stringify({ model: selectedModel || "gpt-4o-mini" }),
      });
      const newChat = (res as { chat: Chat }).chat || (res as Chat);
      if (newChat && newChat.id) {
        addChat(newChat);
        setActiveChatId(newChat.id);
      }
      setMobileDrawerOpen(false);
      onItemClick?.();
    } catch (err) {
      console.error("Failed to create chat:", err);
    } finally {
      setIsCreating(false);
    }
  };

  const handleDeleteChat = async (id: string) => {
    try {
      await apiFetch(`/api/v1/chats/${id}`, { method: "DELETE" });
      removeChat(id);
    } catch (err) {
      console.error("Failed to delete chat:", err);
    }
  };

  const handleSelectChat = (id: string) => {
    setActiveChatId(id);
    setMobileDrawerOpen(false);
    onItemClick?.();
  };

  // Sort chats by updated_at descending
  const sortedChats = [...chats].sort(
    (a, b) => new Date(b.updated_at).getTime() - new Date(a.updated_at).getTime()
  );

  return (
    <aside className="flex flex-col h-full w-[260px] bg-[#0f1118] border-r border-white/[0.08] select-none">
      {/* Brand Header */}
      <div className="flex items-center gap-2.5 px-4 h-14 border-b border-white/[0.08]">
        <div className="w-7 h-7 rounded-lg bg-gradient-to-tr from-indigo-600 to-purple-600 flex items-center justify-center text-white shadow-sm">
          <Sparkles className="w-4 h-4" />
        </div>
        <span className="font-bold text-sm tracking-tight text-white">
          LLM Chat
        </span>
      </div>

      {/* New Chat Button */}
      <div className="p-3">
        <button
          onClick={handleCreateChat}
          disabled={isCreating}
          className="flex items-center justify-center gap-2 w-full py-2.5 px-3 rounded-xl bg-indigo-600 hover:bg-indigo-500 text-white font-medium text-xs transition-all shadow-md shadow-indigo-600/20 active:scale-[0.98] disabled:opacity-50"
        >
          {isCreating ? (
            <Loader2 className="w-3.5 h-3.5 animate-spin" />
          ) : (
            <Plus className="w-3.5 h-3.5" />
          )}
          <span>Новый диалог</span>
        </button>
      </div>

      {/* Chat List */}
      <div className="flex-1 overflow-y-auto px-2 space-y-1">
        {isLoading && chats.length === 0 ? (
          <div className="flex items-center justify-center py-8 text-xs text-slate-500">
            <Loader2 className="w-4 h-4 animate-spin mr-2" />
            Загрузка диалогов...
          </div>
        ) : sortedChats.length === 0 ? (
          <div className="px-3 py-8 text-center text-xs text-slate-500">
            Нет активных диалогов. Создайте первый диалог!
          </div>
        ) : (
          sortedChats.map((chat) => (
            <ChatItem
              key={chat.id}
              chat={chat}
              isActive={chat.id === activeChatId}
              onSelect={() => handleSelectChat(chat.id)}
              onDelete={() => handleDeleteChat(chat.id)}
            />
          ))
        )}
      </div>

      {/* User Profile Bar */}
      <UserProfileBar />
    </aside>
  );
}
