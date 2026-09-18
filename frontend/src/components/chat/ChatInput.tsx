import React, { useState, useRef } from "react";
import { Send, Square } from "lucide-react";
import { useChatStore } from "@/store/useChatStore";
import ZeroBalanceAlert from "./ZeroBalanceAlert";

interface ChatInputProps {
  onSend: (message: string) => void;
  onAbort: () => void;
  disabled?: boolean;
  placeholder?: string;
}

export default function ChatInput({
  onSend,
  onAbort,
  disabled = false,
  placeholder = "Спросите о чём угодно... (Enter для отправки, Shift+Enter для переноса)",
}: ChatInputProps) {
  const [input, setInput] = useState("");
  const textareaRef = useRef<HTMLTextAreaElement | null>(null);
  const { isStreaming, selectedModel, user, setReferralModalOpen } = useChatStore();

  const isZeroBalance = user !== null && user.balance_rub <= 0;
  const isInputDisabled = disabled || isZeroBalance;

  const handleKeyDown = (e: React.KeyboardEvent<HTMLTextAreaElement>) => {
    if (e.key === "Enter" && !e.shiftKey) {
      e.preventDefault();
      if (!isInputDisabled && !isStreaming && input.trim()) {
        onSend(input);
        setInput("");
        if (textareaRef.current) {
          textareaRef.current.style.height = "auto";
        }
      }
    }
  };

  const handleInput = (e: React.ChangeEvent<HTMLTextAreaElement>) => {
    setInput(e.target.value);
    const target = e.target;
    target.style.height = "auto";
    target.style.height = `${Math.min(target.scrollHeight, 180)}px`;
  };

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault();
    if (isStreaming) {
      onAbort();
    } else if (!isInputDisabled && input.trim()) {
      onSend(input);
      setInput("");
      if (textareaRef.current) {
        textareaRef.current.style.height = "auto";
      }
    }
  };

  const effectivePlaceholder = isZeroBalance
    ? "Баланс исчерпан (0.00 ₽). Пригласите друзей, чтобы продолжить диалог."
    : placeholder;

  return (
    <div className="w-full px-4 pb-4 pt-2 bg-gradient-to-t from-[#090a0f] via-[#090a0f]/90 to-transparent">
      <div className="max-w-[800px] mx-auto flex flex-col gap-2">
        {/* Zero Balance Alert Banner */}
        {isZeroBalance && (
          <ZeroBalanceAlert onOpenReferral={() => setReferralModalOpen(true)} />
        )}

        {/* Input Form Box */}
        <form
          onSubmit={handleSubmit}
          className="relative flex items-end gap-2 p-2 rounded-2xl bg-[#141724] border border-white/10 shadow-lg focus-within:border-indigo-500/50 transition-all"
        >
          <textarea
            ref={textareaRef}
            rows={1}
            value={input}
            onChange={handleInput}
            onKeyDown={handleKeyDown}
            disabled={isInputDisabled}
            placeholder={effectivePlaceholder}
            className="flex-1 max-h-[180px] py-2 px-3 bg-transparent text-sm text-slate-200 placeholder-slate-500 resize-none outline-none leading-relaxed disabled:opacity-50 disabled:cursor-not-allowed"
          />

          {/* Action Button: Send or Abort */}
          {isStreaming ? (
            <button
              type="button"
              onClick={onAbort}
              className="flex items-center justify-center gap-1.5 px-3 py-2 rounded-xl bg-red-600/90 hover:bg-red-500 text-white font-medium text-xs transition-colors shrink-0 shadow-md shadow-red-600/20 active:scale-95"
              title="Остановить генерацию"
              aria-label="Остановить генерацию"
            >
              <Square className="w-3.5 h-3.5 fill-current" />
              <span className="hidden sm:inline">Остановить</span>
            </button>
          ) : (
            <button
              type="submit"
              disabled={isInputDisabled || !input.trim()}
              className="p-2.5 rounded-xl bg-indigo-600 hover:bg-indigo-500 text-white disabled:opacity-30 disabled:hover:bg-indigo-600 transition-all shrink-0 shadow-md shadow-indigo-600/20 active:scale-95"
              title="Отправить сообщение"
              aria-label="Отправить сообщение"
            >
              <Send className="w-4 h-4" />
            </button>
          )}
        </form>

        {/* Status Caption */}
        <div className="flex items-center justify-between px-2 text-[11px] text-slate-500">
          <span>Модель: <span className="text-slate-400 font-medium">{selectedModel}</span></span>
          <span>Shift + Enter для переноса строки</span>
        </div>
      </div>
    </div>
  );
}
