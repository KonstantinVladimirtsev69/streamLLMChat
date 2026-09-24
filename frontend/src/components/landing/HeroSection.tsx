import React from "react";
import { Coins, Users, Bot, ArrowRight, ShieldCheck, Zap } from "lucide-react";

export default function HeroSection() {
  const isDev = process.env.NODE_ENV === "development";
  const apiBase = process.env.NEXT_PUBLIC_API_URL || "";

  return (
    <div className="relative flex flex-col items-center justify-center min-h-screen px-4 py-16 overflow-hidden bg-[#090a0f] text-[#f8fafc]">
      {/* Dynamic Background Glows */}
      <div className="absolute top-1/4 left-1/2 -translate-x-1/2 -translate-y-1/2 w-[600px] h-[350px] bg-gradient-to-tr from-indigo-600/20 via-purple-600/15 to-transparent blur-[120px] pointer-events-none rounded-full" />
      <div className="absolute bottom-10 right-10 w-[300px] h-[300px] bg-blue-600/10 blur-[100px] pointer-events-none rounded-full" />

      {/* Main Container */}
      <div className="relative z-10 flex flex-col items-center max-w-3xl mx-auto text-center space-y-8">
        
        {/* Welcome Bonus Badge */}
        <div className="inline-flex items-center gap-2 px-4 py-1.5 rounded-full border border-indigo-500/30 bg-indigo-500/10 text-indigo-300 text-xs font-medium tracking-wide shadow-sm animate-pulse">
          <Coins className="w-4 h-4 text-indigo-400" />
          <span>Приветственный бонус: 5.00 ₽ на баланс при регистрации</span>
        </div>

        {/* Title */}
        <div className="space-y-4">
          <h1 className="text-4xl sm:text-6xl font-extrabold tracking-tight leading-tight">
            Умный диалог с ИИ <br />
            <span className="bg-clip-text text-transparent bg-gradient-to-r from-indigo-400 via-purple-300 to-pink-400">
              через routerai.ru
            </span>
          </h1>
          <p className="max-w-xl mx-auto text-base sm:text-lg text-slate-400 font-normal leading-relaxed">
            Единый доступ к передовым моделям искусственного интеллекта.
            Потоковый вывод SSE, поддержка подсветки кода и честная тарификация за токены.
          </p>
        </div>

        {/* Action Buttons */}
        <div className="flex flex-col sm:flex-row items-center justify-center gap-4 w-full sm:w-auto pt-2">
          <a
            href={`${apiBase}/api/v1/auth/vk/login`}
            className="flex items-center justify-center gap-3 w-full sm:w-auto px-7 py-3.5 rounded-xl bg-gradient-to-r from-blue-600 to-indigo-600 hover:from-blue-500 hover:to-indigo-500 text-white font-medium text-sm transition-all shadow-lg shadow-blue-500/25 active:scale-[0.98]"
          >
            <svg className="w-5 h-5 fill-current" viewBox="0 0 24 24">
              <path d="M12.78 17.5c-4.94 0-7.75-3.38-7.87-9h2.46c.08 4.12 1.9 5.87 3.34 6.23V8.5h2.32v3.55c1.42-.15 2.89-1.77 3.39-3.55h2.32c-.39 2.21-2.02 3.83-3.17 4.5 1.15.54 3.01 1.93 3.65 4.5h-2.55c-.62-1.93-2.16-3.43-3.41-3.55v3.55h-.48z" />
            </svg>
            <span>Войти через VK ID</span>
            <ArrowRight className="w-4 h-4 text-blue-200" />
          </a>

          {isDev && (
            <a
              href={`${apiBase}/api/v1/auth/mock`}
              className="flex items-center justify-center gap-2 w-full sm:w-auto px-6 py-3.5 rounded-xl border border-white/10 bg-white/5 hover:bg-white/10 text-slate-300 font-medium text-sm transition-all active:scale-[0.98]"
            >
              <Zap className="w-4 h-4 text-amber-400" />
              <span>Быстрый вход (Dev Mock)</span>
            </a>
          )}
        </div>

        {/* Feature Grid */}
        <div className="grid grid-cols-1 sm:grid-cols-3 gap-4 w-full pt-8 text-left">
          <div className="p-4 rounded-xl border border-white/5 bg-[#141724]/70 backdrop-blur-md space-y-2">
            <div className="w-8 h-8 rounded-lg bg-indigo-500/10 flex items-center justify-center text-indigo-400">
              <Bot className="w-4 h-4" />
            </div>
            <h3 className="text-sm font-semibold text-white">Выбор моделей</h3>
            <p className="text-xs text-slate-400 leading-normal">
              Мгновенное переключение между ведущими LLM без смены диалога.
            </p>
          </div>

          <div className="p-4 rounded-xl border border-white/5 bg-[#141724]/70 backdrop-blur-md space-y-2">
            <div className="w-8 h-8 rounded-lg bg-emerald-500/10 flex items-center justify-center text-emerald-400">
              <ShieldCheck className="w-4 h-4" />
            </div>
            <h3 className="text-sm font-semibold text-white">Прозрачный баланс</h3>
            <p className="text-xs text-slate-400 leading-normal">
              Списание строго по завершении ответа с точностью до копейки.
            </p>
          </div>

          <div className="p-4 rounded-xl border border-white/5 bg-[#141724]/70 backdrop-blur-md space-y-2">
            <div className="w-8 h-8 rounded-lg bg-purple-500/10 flex items-center justify-center text-purple-400">
              <Users className="w-4 h-4" />
            </div>
            <h3 className="text-sm font-semibold text-white">Реферальная сеть</h3>
            <p className="text-xs text-slate-400 leading-normal">
              Получайте +2.00 ₽ на баланс за каждого приглашенного друга.
            </p>
          </div>
        </div>

      </div>
    </div>
  );
}
