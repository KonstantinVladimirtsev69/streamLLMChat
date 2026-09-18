"use client";

import React, { useEffect } from "react";
import ChatSidebar from "@/components/sidebar/ChatSidebar";
import MobileDrawer from "@/components/sidebar/MobileDrawer";
import { useChatStore } from "@/store/useChatStore";
import { apiFetch } from "@/lib/api";
import type { User } from "@/types/chat";

export default function ChatLayout({
  children,
}: {
  children: React.ReactNode;
}) {
  const { setUser } = useChatStore();

  useEffect(() => {
    let isMounted = true;
    const fetchUser = async () => {
      try {
        const userData = await apiFetch<User>("/api/v1/auth/me");
        if (isMounted && userData) {
          setUser(userData);
        }
      } catch (err) {
        console.error("Failed to load user profile:", err);
      }
    };

    fetchUser();
    return () => {
      isMounted = false;
    };
  }, [setUser]);

  return (
    <div className="flex h-screen w-screen overflow-hidden bg-[#090a0f] text-[#f8fafc]">
      {/* Desktop Sidebar (hidden on mobile) */}
      <div className="hidden md:flex h-full shrink-0">
        <ChatSidebar />
      </div>

      {/* Mobile Drawer (visible when isMobileDrawerOpen === true) */}
      <MobileDrawer />

      {/* Main Chat Workspace Area */}
      <main className="flex-1 flex flex-col h-full min-w-0 overflow-hidden relative">
        {children}
      </main>
    </div>
  );
}
