import React, { useEffect } from "react";
import { X } from "lucide-react";
import { useChatStore } from "@/store/useChatStore";
import ChatSidebar from "./ChatSidebar";

export default function MobileDrawer() {
  const { isMobileDrawerOpen, setMobileDrawerOpen } = useChatStore();

  // Close drawer on Esc key
  useEffect(() => {
    const handleKeyDown = (e: KeyboardEvent) => {
      if (e.key === "Escape" && isMobileDrawerOpen) {
        setMobileDrawerOpen(false);
      }
    };
    window.addEventListener("keydown", handleKeyDown);
    return () => window.removeEventListener("keydown", handleKeyDown);
  }, [isMobileDrawerOpen, setMobileDrawerOpen]);

  if (!isMobileDrawerOpen) return null;

  return (
    <div className="fixed inset-0 z-50 md:hidden flex">
      {/* Backdrop */}
      <div
        onClick={() => setMobileDrawerOpen(false)}
        className="fixed inset-0 bg-black/70 backdrop-blur-xs transition-opacity animate-in fade-in duration-200"
      />

      {/* Drawer Container */}
      <div className="relative z-10 flex flex-col h-full w-[280px] bg-[#0f1118] shadow-2xl animate-in slide-in-from-left duration-300">
        <button
          onClick={() => setMobileDrawerOpen(false)}
          className="absolute top-3 right-3 p-1.5 rounded-lg text-slate-400 hover:text-white hover:bg-white/10 transition-colors z-20"
          aria-label="Закрыть меню"
        >
          <X className="w-4 h-4" />
        </button>

        <div className="h-full">
          <ChatSidebar onItemClick={() => setMobileDrawerOpen(false)} />
        </div>
      </div>
    </div>
  );
}
