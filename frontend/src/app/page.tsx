import React from "react";

export default function Home() {
  return (
    <div className="min-h-screen bg-zinc-950 text-zinc-100 flex flex-col justify-between selection:bg-indigo-500 selection:text-white">
      <header className="border-b border-zinc-800/80 px-6 py-4 flex items-center justify-between backdrop-blur-md sticky top-0 z-10 bg-zinc-950/70">
        <div className="flex items-center gap-3">
          <div className="h-8 w-8 rounded-lg bg-gradient-to-tr from-indigo-600 to-violet-500 flex items-center justify-center font-bold text-white shadow-lg shadow-indigo-500/20">
            AI
          </div>
          <div>
            <h1 className="font-semibold text-sm tracking-wide">LLM Chat Platform</h1>
            <p className="text-xs text-zinc-400">routerai.ru</p>
          </div>
        </div>
        <div className="flex items-center gap-3">
          <span className="inline-flex items-center gap-1.5 px-2.5 py-1 rounded-full text-xs font-medium bg-emerald-950/80 text-emerald-400 border border-emerald-800/60">
            <span className="h-1.5 w-1.5 rounded-full bg-emerald-400 animate-pulse"></span>
            Система инициализирована
          </span>
        </div>
      </header>

      <main className="max-w-4xl mx-auto px-6 py-16 flex flex-col items-center text-center">
        <div className="inline-flex items-center gap-2 px-3 py-1.5 rounded-full bg-zinc-900 border border-zinc-800 text-xs text-zinc-300 mb-6">
          <span className="text-indigo-400 font-semibold">Phase 1</span>
          <span>•</span>
          <span>Project Scaffold & Infrastructure</span>
        </div>

        <h2 className="text-4xl sm:text-5xl font-extrabold tracking-tight bg-gradient-to-b from-white via-zinc-200 to-zinc-500 bg-clip-text text-transparent max-w-2xl leading-tight">
          Чат с LLM моделями с оплатой за токены
        </h2>
        
        <p className="mt-4 text-zinc-400 text-base sm:text-lg max-w-xl">
          Авторизация через VK OAuth, приветственный баланс 5 руб, реферальная программа (+2 руб) и потоковый вывод ответов нейросетей через routerai.ru.
        </p>

        <div className="grid grid-cols-1 sm:grid-cols-2 gap-4 mt-12 w-full text-left">
          <div className="p-5 rounded-xl bg-zinc-900/60 border border-zinc-800/80 hover:border-zinc-700 transition-colors">
            <div className="flex items-center justify-between mb-2">
              <span className="text-sm font-semibold text-zinc-200">Backend API</span>
              <span className="text-xs text-indigo-400 font-mono">Go 1.27 + chi</span>
            </div>
            <p className="text-xs text-zinc-400">
              Высокопроизводительный сервер с поддержкой SSE стриминга, биллингом в транзакциях и интеграцией с routerai.
            </p>
          </div>

          <div className="p-5 rounded-xl bg-zinc-900/60 border border-zinc-800/80 hover:border-zinc-700 transition-colors">
            <div className="flex items-center justify-between mb-2">
              <span className="text-sm font-semibold text-zinc-200">Frontend UI</span>
              <span className="text-xs text-violet-400 font-mono">Next.js 16 + Tailwind</span>
            </div>
            <p className="text-xs text-zinc-400">
              Реактивный интерфейс чата с Markdown рендерингом, подсветкой кода, историей диалогов и реферальной системой.
            </p>
          </div>

          <div className="p-5 rounded-xl bg-zinc-900/60 border border-zinc-800/80 hover:border-zinc-700 transition-colors">
            <div className="flex items-center justify-between mb-2">
              <span className="text-sm font-semibold text-zinc-200">PostgreSQL 18</span>
              <span className="text-xs text-blue-400 font-mono">ACID Биллинг</span>
            </div>
            <p className="text-xs text-zinc-400">
              Хранение профилей пользователей, финансового баланса, транзакций списаний и реферальных связей.
            </p>
          </div>

          <div className="p-5 rounded-xl bg-zinc-900/60 border border-zinc-800/80 hover:border-zinc-700 transition-colors">
            <div className="flex items-center justify-between mb-2">
              <span className="text-sm font-semibold text-zinc-200">MongoDB 8</span>
              <span className="text-xs text-emerald-400 font-mono">История сообщений</span>
            </div>
            <p className="text-xs text-zinc-400">
              Гибкое хранение сессий переписки, контекста диалогов и метаданных генераций LLM.
            </p>
          </div>
        </div>
      </main>

      <footer className="border-t border-zinc-900 py-6 text-center text-xs text-zinc-500">
        LLM Chat Platform • Go 1.27+ • Next.js 16+ • Dokploy Ready
      </footer>
    </div>
  );
}
