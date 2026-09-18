import React, { useEffect, useRef } from "react";
import { Sparkles, Code2, Lightbulb, Rocket } from "lucide-react";
import { useChatStore } from "@/store/useChatStore";
import MessageItem from "./MessageItem";

interface ChatWindowProps {
  onSelectPrompt?: (prompt: string) => void;
}

const PROMPT_SUGGESTIONS = [
  {
    icon: Code2,
    title: "Написать код",
    prompt: "Напиши пример асинхронного воркера на Go с graceful shutdown.",
  },
  {
    icon: Lightbulb,
    title: "Объяснить концепцию",
    prompt: "Объясни, как работает механизм Attention в Transformer архитектуре.",
  },
  {
    icon: Rocket,
    title: "Оптимизация",
    prompt: "Как ускорить загрузку страниц в Next.js 16 и настроить кэширование?",
  },
];

export default function ChatWindow({ onSelectPrompt }: ChatWindowProps) {
  const { messages, isStreaming, streamingMessageId } = useChatStore();
  const bottomRef = useRef<HTMLDivElement | null>(null);

  useEffect(() => {
    bottomRef.current?.scrollIntoView({ behavior: "smooth" });
  }, [messages, isStreaming]);

  return (
    <div className="flex-1 overflow-y-auto px-4 py-6">
      <div className="max-w-[800px] mx-auto space-y-4">
        {messages.length === 0 ? (
          /* Empty State */
          <div className="flex flex-col items-center justify-center min-h-[50vh] text-center space-y-6 pt-10">
            <div className="w-12 h-12 rounded-2xl bg-indigo-600/20 border border-indigo-500/30 flex items-center justify-center text-indigo-400 shadow-inner">
              <Sparkles className="w-6 h-6" />
            </div>

            <div className="space-y-2">
              <h2 className="text-xl font-bold text-white">
                Чем я могу помочь вам сегодня?
              </h2>
              <p className="text-xs text-slate-400 max-w-sm mx-auto">
                Выберите один из готовых промптов или задайте любой интересующий вопрос.
              </p>
            </div>

            <div className="grid grid-cols-1 sm:grid-cols-3 gap-3 w-full pt-4">
              {PROMPT_SUGGESTIONS.map((item, idx) => {
                const Icon = item.icon;
                return (
                  <button
                    key={idx}
                    onClick={() => onSelectPrompt?.(item.prompt)}
                    className="flex flex-col items-start p-3.5 rounded-xl bg-[#141724]/70 hover:bg-[#1c2032] border border-white/5 hover:border-indigo-500/30 transition-all text-left group"
                  >
                    <div className="p-2 rounded-lg bg-indigo-500/10 text-indigo-400 mb-2 group-hover:bg-indigo-500/20 transition-colors">
                      <Icon className="w-4 h-4" />
                    </div>
                    <span className="text-xs font-semibold text-slate-200 mb-1">
                      {item.title}
                    </span>
                    <span className="text-[11px] text-slate-400 line-clamp-2">
                      {item.prompt}
                    </span>
                  </button>
                );
              })}
            </div>
          </div>
        ) : (
          /* Messages Stream */
          messages.map((message) => (
            <MessageItem
              key={message.id}
              message={message}
              isStreamingActive={isStreaming && streamingMessageId === message.id}
            />
          ))
        )}

        <div ref={bottomRef} className="h-4" />
      </div>
    </div>
  );
}
