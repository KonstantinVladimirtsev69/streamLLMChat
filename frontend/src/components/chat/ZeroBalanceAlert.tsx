import React from "react";
import { AlertTriangle, Gift } from "lucide-react";

interface ZeroBalanceAlertProps {
  onOpenReferral: () => void;
}

export default function ZeroBalanceAlert({
  onOpenReferral,
}: ZeroBalanceAlertProps) {
  return (
    <div className="flex flex-col sm:flex-row items-center justify-between gap-3 p-3 rounded-xl bg-amber-950/50 border border-amber-500/30 text-amber-200 text-xs shadow-md animate-in fade-in slide-in-from-bottom-2 duration-200">
      <div className="flex items-center gap-2.5 min-w-0">
        <AlertTriangle className="w-4 h-4 text-amber-400 shrink-0" />
        <span className="leading-snug">
          Баланс исчерпан (0.00 ₽). Пригласите друзей в реферальную программу, чтобы получить по +2.00 ₽ за каждого.
        </span>
      </div>

      <button
        type="button"
        onClick={onOpenReferral}
        className="flex items-center gap-1.5 px-3 py-1.5 rounded-lg bg-amber-500 hover:bg-amber-400 text-slate-950 font-semibold text-xs transition-colors shrink-0 shadow-sm active:scale-95"
      >
        <Gift className="w-3.5 h-3.5" />
        <span>Получить бонус</span>
      </button>
    </div>
  );
}
