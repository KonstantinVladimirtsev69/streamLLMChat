import React from "react";
import { Wallet, AlertCircle } from "lucide-react";
import { useChatStore } from "@/store/useChatStore";
import { formatRub } from "@/lib/format";

export default function BalanceChip() {
  const { user, setReferralModalOpen } = useChatStore();

  const balance = user?.balance_rub ?? 0;
  const isZero = balance <= 0;

  return (
    <button
      type="button"
      onClick={() => setReferralModalOpen(true)}
      className={`flex items-center gap-1.5 px-3 py-1.5 rounded-xl border transition-all text-xs font-semibold select-none shadow-sm active:scale-95 ${
        isZero
          ? "bg-amber-950/40 border-amber-500/30 text-amber-300 hover:bg-amber-900/50 hover:border-amber-500/50"
          : "bg-[#141724] border-white/10 text-emerald-400 hover:bg-[#1c2032] hover:border-emerald-500/40"
      }`}
      title="Нажмите для перехода в реферальную программу"
      aria-label="Баланс пользователя"
    >
      {isZero ? (
        <AlertCircle className="w-3.5 h-3.5 text-amber-400 shrink-0" />
      ) : (
        <Wallet className="w-3.5 h-3.5 text-emerald-400 shrink-0" />
      )}
      <span>{formatRub(balance)}</span>
    </button>
  );
}
