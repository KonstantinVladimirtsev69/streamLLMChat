import React, { useState, useEffect, useRef } from "react";
import { Sparkles, ChevronDown, Check, Search, Cpu } from "lucide-react";
import { useChatStore } from "@/store/useChatStore";
import { apiFetch } from "@/lib/api";
import type { LLMModel } from "@/types/chat";
import { formatTokens } from "@/lib/format";

export default function ModelSelector() {
  const {
    models,
    setModels,
    selectedModel,
    setSelectedModel,
  } = useChatStore();

  const [isOpen, setIsOpen] = useState(false);
  const [search, setSearch] = useState("");
  const dropdownRef = useRef<HTMLDivElement | null>(null);

  // Fetch models if not loaded yet
  useEffect(() => {
    let isMounted = true;
    if (models.length === 0) {
      apiFetch<{ models: LLMModel[] } | LLMModel[]>("/api/v1/models")
        .then((data) => {
          const modelList = Array.isArray(data) ? data : data?.models || [];
          if (isMounted && modelList.length > 0) {
            setModels(modelList);
          }
        })
        .catch((err) => console.error("Failed to load models:", err));
    }
    return () => {
      isMounted = false;
    };
  }, [models.length, setModels]);

  // Click outside to close
  useEffect(() => {
    const handleClickOutside = (e: MouseEvent) => {
      if (
        dropdownRef.current &&
        !dropdownRef.current.contains(e.target as Node)
      ) {
        setIsOpen(false);
      }
    };
    document.addEventListener("mousedown", handleClickOutside);
    return () => document.removeEventListener("mousedown", handleClickOutside);
  }, []);

  const handleSelect = async (modelId: string) => {
    setSelectedModel(modelId);
    setIsOpen(false);
  };

  const filteredModels = models.filter((m) =>
    (m.name || m.id).toLowerCase().includes(search.toLowerCase())
  );

  const activeModelObj = models.find((m) => m.id === selectedModel);

  return (
    <div className="relative" ref={dropdownRef}>
      {/* Model Selector Trigger Button */}
      <button
        type="button"
        onClick={() => setIsOpen(!isOpen)}
        className="flex items-center gap-2 px-3 py-1.5 rounded-xl bg-[#141724] hover:bg-[#1c2032] border border-white/10 hover:border-indigo-500/40 transition-all text-xs text-slate-200 select-none shadow-sm"
      >
        <Sparkles className="w-3.5 h-3.5 text-indigo-400" />
        <span className="font-semibold truncate max-w-[140px] sm:max-w-[180px]">
          {activeModelObj?.name || selectedModel || "Выберите модель"}
        </span>
        <ChevronDown
          className={`w-3.5 h-3.5 text-slate-400 transition-transform duration-200 ${
            isOpen ? "rotate-180" : ""
          }`}
        />
      </button>

      {/* Dropdown Menu */}
      {isOpen && (
        <div className="absolute top-full left-0 mt-2 w-72 sm:w-80 rounded-2xl bg-[#0f1118] border border-white/10 shadow-2xl z-50 overflow-hidden animate-in fade-in zoom-in-95 duration-150">
          {/* Search Box */}
          <div className="p-2 border-b border-white/[0.08]">
            <div className="flex items-center gap-2 px-2.5 py-1.5 rounded-lg bg-[#141724] border border-white/5 text-xs text-slate-200">
              <Search className="w-3.5 h-3.5 text-slate-400 shrink-0" />
              <input
                type="text"
                value={search}
                onChange={(e) => setSearch(e.target.value)}
                placeholder="Поиск модели..."
                className="w-full bg-transparent outline-none text-xs placeholder-slate-500"
                autoFocus
              />
            </div>
          </div>

          {/* Model List */}
          <div className="max-h-64 overflow-y-auto p-1.5 space-y-1">
            {filteredModels.length === 0 ? (
              <div className="p-4 text-center text-xs text-slate-500">
                Модели не найдены
              </div>
            ) : (
              filteredModels.map((model) => {
                const isSelected = model.id === selectedModel;
                return (
                  <div
                    key={model.id}
                    onClick={() => handleSelect(model.id)}
                    className={`flex items-center justify-between p-2 rounded-xl cursor-pointer transition-all ${
                      isSelected
                        ? "bg-indigo-600/20 text-white border border-indigo-500/30"
                        : "hover:bg-[#141724] text-slate-300"
                    }`}
                  >
                    <div className="flex items-center gap-2.5 min-w-0 flex-1">
                      <Cpu className="w-4 h-4 text-indigo-400 shrink-0" />
                      <div className="flex flex-col min-w-0">
                        <span className="text-xs font-medium truncate">
                          {model.name || model.id}
                        </span>
                        {model.context_length && (
                          <span className="text-[10px] text-slate-500">
                            Контекст: {formatTokens(model.context_length)} токенов
                          </span>
                        )}
                      </div>
                    </div>

                    {isSelected && (
                      <Check className="w-4 h-4 text-indigo-400 shrink-0 ml-2" />
                    )}
                  </div>
                );
              })
            )}
          </div>
        </div>
      )}
    </div>
  );
}
