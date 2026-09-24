export interface User {
  id: number;
  vk_id: number;
  first_name: string;
  last_name: string;
  avatar_url?: string;
  balance_rub: number;
  balance_kopecks: number;
  ref_code?: string;
  referral_code?: string;
  ref_link?: string;
  invited_count?: number;
  referral_earnings_rub?: number;
}

export interface Chat {
  id: string;
  user_id: number;
  title: string;
  model: string;
  created_at: string;
  updated_at: string;
}

export interface Message {
  id: string;
  chat_id: string;
  role: "user" | "assistant" | "system";
  content: string;
  model?: string;
  prompt_tokens?: number;
  completion_tokens?: number;
  cost_kopecks?: number;
  created_at: string;
}

export interface LLMModel {
  id: string;
  name: string;
  description?: string;
  context_length?: number;
  prompt_price?: number;
  completion_price?: number;
  prompt_price_per_1m?: number;
  completion_price_per_1m?: number;
}

export interface StreamUsage {
  prompt_tokens: number;
  completion_tokens: number;
  total_tokens: number;
  cost_kopecks: number;
}

export interface StreamEvent {
  type: "delta" | "done" | "error";
  delta?: string;
  content?: string;
  chat_id?: string;
  model?: string;
  usage?: StreamUsage;
  error?: string;
}
