import React, { useState } from "react";
import { MessageSquare, Trash2, Check, X } from "lucide-react";
import type { Chat } from "@/types/chat";
import { formatDateTime } from "@/lib/format";

interface ChatItemProps {
  chat: Chat;
  isActive: boolean;
  onSelect: () => void;
  onDelete: () => void;
}

export default function ChatItem({
  chat,
  isActive,
  onSelect,
  onDelete,
}: ChatItemProps) {
  const [confirmDelete, setConfirmDelete] = useState(false);

  const handleDeleteClick = (e: React.MouseEvent) => {
    e.stopPropagation();
    setConfirmDelete(true);
  };

  const handleConfirmDelete = (e: React.MouseEvent) => {
    e.stopPropagation();
    onDelete();
  };

  const handleCancelDelete = (e: React.MouseEvent) => {
    e.stopPropagation();
    setConfirmDelete(false);
  };

  return (
    <div
      onClick={onSelect}
      className={`group relative flex items-center justify-between w-full px-3 py-2.5 rounded-xl cursor-pointer transition-all text-left select-none ${
        isActive
          ? "bg-[#1c2032] text-white font-medium shadow-sm border border-indigo-500/30"
          : "text-slate-400 hover:bg-[#141724] hover:text-slate-200 border border-transparent"
      }`}
    >
      <div className="flex items-center gap-2.5 min-w-0 flex-1 mr-2">
        <MessageSquare
          className={`w-4 h-4 shrink-0 ${
            isActive ? "text-indigo-400" : "text-slate-500 group-hover:text-slate-400"
          }`}
        />
        <div className="flex flex-col min-w-0 flex-1">
          <span className="text-sm truncate">
            {chat.title || "Новый диалог"}
          </span>
          <span className="text-[10px] text-slate-500 truncate">
            {formatDateTime(chat.updated_at)}
          </span>
        </div>
      </div>

      {/* Delete / Confirmation Actions */}
      <div className="flex items-center shrink-0">
        {confirmDelete ? (
          <div className="flex items-center gap-1 bg-red-950/80 px-1.5 py-0.5 rounded-lg border border-red-500/40">
            <button
              onClick={handleConfirmDelete}
              className="p-1 text-red-300 hover:text-white rounded hover:bg-red-800/50 transition-colors"
              title="Подтвердить удаление"
              aria-label="Подтвердить удаление"
            >
              <Check className="w-3.5 h-3.5" />
            </button>
            <button
              onClick={handleCancelDelete}
              className="p-1 text-slate-400 hover:text-slate-200 rounded hover:bg-white/10 transition-colors"
              title="Отмена"
              aria-label="Отмена"
            >
              <X className="w-3.5 h-3.5" />
            </button>
          </div>
        ) : (
          <button
            onClick={handleDeleteClick}
            className={`p-1.5 rounded-lg text-slate-500 hover:text-red-400 hover:bg-red-500/10 transition-colors opacity-0 group-hover:opacity-100 focus:opacity-100 ${
              isActive ? "opacity-60" : ""
            }`}
            title="Удалить диалог"
            aria-label="Удалить диалог"
          >
            <Trash2 className="w-3.5 h-3.5" />
          </button>
        )}
      </div>
    </div>
  );
}
