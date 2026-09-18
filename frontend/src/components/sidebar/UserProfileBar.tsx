import React, { useState } from "react";
import { LogOut, User as UserIcon } from "lucide-react";
import { useChatStore } from "@/store/useChatStore";
import { apiFetch } from "@/lib/api";

export default function UserProfileBar() {
  const { user, resetState } = useChatStore();
  const [isLoggingOut, setIsLoggingOut] = useState(false);

  const handleLogout = async () => {
    try {
      setIsLoggingOut(true);
      await apiFetch("/api/v1/auth/logout", { method: "POST" });
    } catch {
      // Proceed to reset even on error
    } finally {
      resetState();
      window.location.href = "/";
    }
  };

  const displayName = user
    ? `${user.first_name || ""} ${user.last_name || ""}`.trim() || "Пользователь"
    : "Пользователь";

  const initial = (user?.first_name?.[0] || "U").toUpperCase();

  return (
    <div className="flex items-center justify-between gap-3 p-3 border-t border-white/[0.08] bg-[#0f1118]/80">
      <div className="flex items-center gap-2.5 min-w-0 flex-1">
        {user?.avatar_url ? (
          <img
            src={user.avatar_url}
            alt={displayName}
            className="w-8 h-8 rounded-full object-cover border border-white/10 shrink-0"
          />
        ) : (
          <div className="w-8 h-8 rounded-full bg-indigo-600/30 border border-indigo-500/40 flex items-center justify-center text-indigo-300 font-semibold text-xs shrink-0">
            {initial}
          </div>
        )}

        <div className="flex flex-col min-w-0 flex-1">
          <span className="text-xs font-semibold text-slate-200 truncate">
            {displayName}
          </span>
          <span className="text-[10px] text-slate-500 truncate">
            ID: {user?.id ?? "—"}
          </span>
        </div>
      </div>

      <button
        onClick={handleLogout}
        disabled={isLoggingOut}
        className="p-1.5 rounded-lg text-slate-400 hover:text-red-400 hover:bg-red-500/10 transition-colors shrink-0"
        title="Выйти из аккаунта"
        aria-label="Выйти из аккаунта"
      >
        <LogOut className="w-4 h-4" />
      </button>
    </div>
  );
}
