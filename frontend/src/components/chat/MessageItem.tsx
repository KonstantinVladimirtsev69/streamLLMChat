import React from "react";
import ReactMarkdown from "react-markdown";
import remarkGfm from "remark-gfm";
import { Bot, User as UserIcon, Coins } from "lucide-react";
import type { Message } from "@/types/chat";
import { formatDateTime, formatKopecks } from "@/lib/format";
import CodeBlock from "./CodeBlock";

interface MessageItemProps {
  message: Message;
  isStreamingActive?: boolean;
}

export default function MessageItem({
  message,
  isStreamingActive = false,
}: MessageItemProps) {
  const isUser = message.role === "user";

  return (
    <div
      className={`flex gap-3.5 py-4 px-3 sm:px-4 rounded-2xl transition-colors ${
        isUser
          ? "bg-transparent justify-end"
          : "bg-[#141724]/40 border border-white/[0.04]"
      }`}
    >
      {/* Assistant Avatar */}
      {!isUser && (
        <div className="w-8 h-8 rounded-xl bg-gradient-to-tr from-indigo-600 to-purple-600 flex items-center justify-center text-white shrink-0 shadow-sm mt-0.5">
          <Bot className="w-4 h-4" />
        </div>
      )}

      {/* Message Content Body */}
      <div
        className={`flex flex-col min-w-0 max-w-[88%] sm:max-w-[82%] ${
          isUser ? "items-end" : "items-start flex-1"
        }`}
      >
        {/* Header (Role & Time) */}
        <div className="flex items-center gap-2 mb-1.5 text-[11px] text-slate-500">
          <span className="font-semibold text-slate-300">
            {isUser ? "Вы" : message.model || "Ассистент"}
          </span>
          <span>•</span>
          <span>{formatDateTime(message.created_at)}</span>

          {/* Cost Badge for Assistant */}
          {!isUser && Boolean(message.cost_kopecks) && (
            <span className="inline-flex items-center gap-1 px-1.5 py-0.5 rounded-full bg-emerald-500/10 text-emerald-400 text-[10px] font-medium border border-emerald-500/20">
              <Coins className="w-3 h-3" />
              {formatKopecks(message.cost_kopecks)}
            </span>
          )}
        </div>

        {/* Text Container */}
        <div
          className={`text-sm leading-relaxed ${
            isUser
              ? "bg-indigo-600 text-white px-4 py-2.5 rounded-2xl rounded-tr-sm shadow-md"
              : "text-slate-200 w-full"
          }`}
        >
          {isUser ? (
            <p className="whitespace-pre-wrap break-words m-0">{message.content}</p>
          ) : (
            <div className="prose prose-invert prose-sm max-w-none break-words">
              <ReactMarkdown
                remarkPlugins={[remarkGfm]}
                components={{
                  code(props) {
                    const { children, className, ...rest } = props;
                    const match = /language-(\w+)/.exec(className || "");
                    const isInline = !match && typeof children === "string" && !children.includes("\n");

                    return !isInline && match ? (
                      <CodeBlock
                        language={match[1]}
                        value={String(children).replace(/\n$/, "")}
                      />
                    ) : (
                      <code
                        {...rest}
                        className="px-1.5 py-0.5 rounded bg-white/10 text-indigo-300 font-mono text-[12px]"
                      >
                        {children}
                      </code>
                    );
                  },
                }}
              >
                {message.content}
              </ReactMarkdown>

              {/* Streaming Pulsing Cursor */}
              {isStreamingActive && (
                <span className="inline-block w-1.5 h-4 ml-1 bg-indigo-400 animate-pulse align-middle rounded-xs" />
              )}
            </div>
          )}
        </div>
      </div>

      {/* User Avatar */}
      {isUser && (
        <div className="w-8 h-8 rounded-xl bg-slate-800 border border-white/10 flex items-center justify-center text-slate-300 shrink-0 mt-0.5">
          <UserIcon className="w-4 h-4" />
        </div>
      )}
    </div>
  );
}
