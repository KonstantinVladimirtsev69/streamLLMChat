import { create } from "zustand";
import type { User, Chat, Message, LLMModel } from "@/types/chat";

export interface ChatState {
  // Authentication & User Profile
  user: User | null;
  setUser: (user: User | null) => void;
  applyDeduction: (costKopecks: number) => void;

  // Chats
  chats: Chat[];
  setChats: (chats: Chat[]) => void;
  addChat: (chat: Chat) => void;
  removeChat: (chatId: string) => void;
  updateChatTitle: (chatId: string, title: string) => void;

  // Active Chat Session
  activeChatId: string | null;
  setActiveChatId: (id: string | null) => void;

  // Messages in Active Chat
  messages: Message[];
  setMessages: (messages: Message[]) => void;
  addMessage: (message: Message) => void;
  updateStreamingMessage: (id: string, delta: string) => void;
  finalizeMessage: (id: string, updates: Partial<Message>) => void;

  // Models
  models: LLMModel[];
  selectedModel: string;
  setModels: (models: LLMModel[]) => void;
  setSelectedModel: (modelId: string) => void;

  // Streaming & Generation
  isStreaming: boolean;
  streamingMessageId: string | null;
  setIsStreaming: (isStreaming: boolean, messageId?: string | null) => void;

  // UI state (modals & drawer)
  isMobileDrawerOpen: boolean;
  setMobileDrawerOpen: (open: boolean) => void;
  isReferralModalOpen: boolean;
  setReferralModalOpen: (open: boolean) => void;

  // Reset
  resetState: () => void;
}

const initialState = {
  user: null,
  chats: [],
  activeChatId: null,
  messages: [],
  models: [],
  selectedModel: "gpt-4o-mini",
  isStreaming: false,
  streamingMessageId: null,
  isMobileDrawerOpen: false,
  isReferralModalOpen: false,
};

export const useChatStore = create<ChatState>((set) => ({
  ...initialState,

  setUser: (user) => set({ user }),

  applyDeduction: (costKopecks) =>
    set((state) => {
      if (!state.user || costKopecks <= 0) return state;
      const newKopecks = Math.max(0, state.user.balance_kopecks - costKopecks);
      return {
        user: {
          ...state.user,
          balance_kopecks: newKopecks,
          balance_rub: newKopecks / 100,
        },
      };
    }),

  setChats: (chats) => set({ chats }),

  addChat: (chat) =>
    set((state) => ({
      chats: [chat, ...state.chats.filter((c) => c.id !== chat.id)],
      activeChatId: chat.id,
    })),

  removeChat: (chatId) =>
    set((state) => {
      const remaining = state.chats.filter((c) => c.id !== chatId);
      let newActiveId = state.activeChatId;
      if (state.activeChatId === chatId) {
        newActiveId = remaining.length > 0 ? remaining[0].id : null;
      }
      return {
        chats: remaining,
        activeChatId: newActiveId,
        messages: state.activeChatId === chatId ? [] : state.messages,
      };
    }),

  updateChatTitle: (chatId, title) =>
    set((state) => ({
      chats: state.chats.map((c) => (c.id === chatId ? { ...c, title } : c)),
    })),

  setActiveChatId: (id) =>
    set((state) => {
      if (state.activeChatId === id) return state;
      return { activeChatId: id, messages: [] };
    }),

  setMessages: (messages) => set({ messages }),

  addMessage: (message) =>
    set((state) => ({
      messages: [...state.messages, message],
    })),

  updateStreamingMessage: (id, delta) =>
    set((state) => ({
      messages: state.messages.map((m) =>
        m.id === id ? { ...m, content: m.content + delta } : m
      ),
    })),

  finalizeMessage: (id, updates) =>
    set((state) => ({
      messages: state.messages.map((m) =>
        m.id === id ? { ...m, ...updates } : m
      ),
    })),

  setModels: (models) => {
    set((state) => ({
      models,
      selectedModel:
        models.length > 0 && !models.some((m) => m.id === state.selectedModel)
          ? models[0].id
          : state.selectedModel,
    }));
  },

  setSelectedModel: (modelId) => set({ selectedModel: modelId }),

  setIsStreaming: (isStreaming, messageId = null) =>
    set({ isStreaming, streamingMessageId: messageId }),

  setMobileDrawerOpen: (open) => set({ isMobileDrawerOpen: open }),

  setReferralModalOpen: (open) => set({ isReferralModalOpen: open }),

  resetState: () => set(initialState),
}));
