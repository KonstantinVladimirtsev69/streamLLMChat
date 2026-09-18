import React from "react";
import { Menu } from "lucide-react";
import { useChatStore } from "@/store/useChatStore";
import ModelSelector from "./ModelSelector";
import BalanceChip from "./BalanceChip";

interface HeaderProps {
  rightSlot?: React.ReactNode;
}

export default function Header({ rightSlot }: HeaderProps) {
  const { setMobileDrawerOpen, chats, activeChatId } = useChatStore();

  const activeChat = chats.find((c) => c.id === activeChatId);
  const chatTitle = activeChat?.title || "Новый диалог";

  return (
    <header className="flex items-center justify-between h-14 px-4 border-b border-white/[0.08] bg-[#090a0f]/80 backdrop-blur-md shrink-0 z-10">
      {/* Left Area: Mobile Menu Toggle & Title */}
      <div className="flex items-center gap-3 min-w-0">
        <button
          onClick={() => setMobileDrawerOpen(true)}
          className="p-1.5 rounded-lg text-slate-400 hover:text-white hover:bg-white/10 md:hidden transition-colors"
          aria-label="Открыть меню"
        >
          <Menu className="w-5 h-5" />
        </button>

        <h1 className="text-sm font-semibold text-slate-200 truncate max-w-[140px] sm:max-w-[220px] md:max-w-[300px]">
          {chatTitle}
        </h1>
      </div>

      {/* Right Area: Model Selector & Balance */}
      <div className="flex items-center gap-2.5 shrink-0">
        <ModelSelector />
        <BalanceChip />
        {rightSlot}
      </div>
    </header>
  );
}
