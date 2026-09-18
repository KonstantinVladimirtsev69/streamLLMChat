import React, { useState, useEffect } from "react";
import { Gift, X, Copy, Check, Users, Coins, Sparkles } from "lucide-react";
import { useChatStore } from "@/store/useChatStore";
import { formatRub } from "@/lib/format";

export default function ReferralModal() {
  const { isReferralModalOpen, setReferralModalOpen, user } = useChatStore();
  const [copied, setCopied] = useState(false);
  const [origin, setOrigin] = useState("");

  useEffect(() => {
    if (typeof window !== "undefined") {
      setOrigin(window.location.origin);
    }
  }, []);

  // Close on Escape key
  useEffect(() => {
    const handleKeyDown = (e: KeyboardEvent) => {
      if (e.key === "Escape" && isReferralModalOpen) {
        setReferralModalOpen(false);
      }
    };
    window.addEventListener("keydown", handleKeyDown);
    return () => window.removeEventListener("keydown", handleKeyDown);
  }, [isReferralModalOpen, setReferralModalOpen]);

  if (!isReferralModalOpen) return null;

  const referralCode = user?.referral_code || "";
  const referralLink = referralCode ? `${origin}/?ref=${referralCode}` : origin;

  const handleCopy = async () => {
    try {
      await navigator.clipboard.writeText(referralLink);
      setCopied(true);
      setTimeout(() => setCopied(false), 2000);
    } catch (err) {
      console.error("Failed to copy referral link:", err);
    }
  };

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center p-4">
      {/* Backdrop */}
      <div
        onClick={() => setReferralModalOpen(false)}
        className="fixed inset-0 bg-black/75 backdrop-blur-sm transition-opacity animate-in fade-in duration-200"
      />

      {/* Modal Card */}
      <div className="relative z-10 w-full max-w-md p-6 rounded-2xl bg-[#0f1118] border border-white/10 shadow-2xl space-y-6 animate-in zoom-in-95 fade-in duration-200 text-left">
        {/* Header */}
        <div className="flex items-center justify-between">
          <div className="flex items-center gap-2.5">
            <div className="w-9 h-9 rounded-xl bg-purple-600/20 border border-purple-500/30 flex items-center justify-center text-purple-400">
              <Gift className="w-5 h-5" />
            </div>
            <div>
              <h3 className="text-base font-bold text-white">
                Реферальная программа
              </h3>
              <p className="text-xs text-slate-400">
                Зарабатывайте бонусы на общение с ИИ
              </p>
            </div>
          </div>

          <button
            onClick={() => setReferralModalOpen(false)}
            className="p-1.5 rounded-lg text-slate-400 hover:text-white hover:bg-white/10 transition-colors"
            aria-label="Закрыть окно"
          >
            <X className="w-4 h-4" />
          </button>
        </div>

        {/* Promo Badge */}
        <div className="p-3.5 rounded-xl bg-gradient-to-r from-indigo-950/60 to-purple-950/60 border border-indigo-500/30 text-xs text-indigo-200 flex items-start gap-2.5">
          <Sparkles className="w-4 h-4 text-indigo-400 shrink-0 mt-0.5" />
          <span className="leading-relaxed">
            Приглашайте друзей и получайте{" "}
            <strong className="text-white">+2.00 ₽</strong> на баланс за каждого
            зарегистрированного пользователя!
          </span>
        </div>

        {/* Referral Link Input Box */}
        <div className="space-y-1.5">
          <label className="text-xs font-semibold text-slate-300">
            Ваша персональная ссылка
          </label>
          <div className="flex items-center gap-2 p-1.5 rounded-xl bg-[#141724] border border-white/10">
            <input
              type="text"
              readOnly
              value={referralLink}
              className="flex-1 bg-transparent px-2.5 text-xs text-slate-200 outline-none select-all font-mono truncate"
            />
            <button
              onClick={handleCopy}
              className="flex items-center gap-1.5 px-3 py-1.5 rounded-lg bg-indigo-600 hover:bg-indigo-500 text-white font-medium text-xs transition-colors shrink-0 shadow-sm active:scale-95"
            >
              {copied ? (
                <>
                  <Check className="w-3.5 h-3.5 text-emerald-300" />
                  <span>Скопировано!</span>
                </>
              ) : (
                <>
                  <Copy className="w-3.5 h-3.5" />
                  <span>Копировать</span>
                </>
              )}
            </button>
          </div>
        </div>

        {/* Statistics Grid */}
        <div className="grid grid-cols-2 gap-3 pt-1">
          <div className="p-3 rounded-xl bg-[#141724] border border-white/5 space-y-1">
            <div className="flex items-center gap-1.5 text-slate-400 text-xs">
              <Users className="w-3.5 h-3.5 text-purple-400" />
              <span>Приглашено друзей</span>
            </div>
            <p className="text-lg font-bold text-white">
              {user?.invited_count ?? 0}
            </p>
          </div>

          <div className="p-3 rounded-xl bg-[#141724] border border-white/5 space-y-1">
            <div className="flex items-center gap-1.5 text-slate-400 text-xs">
              <Coins className="w-3.5 h-3.5 text-emerald-400" />
              <span>Заработано всего</span>
            </div>
            <p className="text-lg font-bold text-emerald-400">
              {formatRub(user?.referral_earnings_rub ?? 0)}
            </p>
          </div>
        </div>
      </div>
    </div>
  );
}
