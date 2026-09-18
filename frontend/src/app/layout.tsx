import type { Metadata } from "next";
import { Inter, JetBrains_Mono } from "next/font/google";
import "./globals.css";

const inter = Inter({
  subsets: ["latin", "cyrillic"],
  variable: "--font-inter",
  display: "swap",
});

const jetbrainsMono = JetBrains_Mono({
  subsets: ["latin", "cyrillic"],
  variable: "--font-jetbrains-mono",
  display: "swap",
});

export const metadata: Metadata = {
  title: "LLM Chat Platform | routerai.ru",
  description: "Быстрый веб-чат с современными языковыми моделями через routerai.ru",
};

export default function RootLayout({
  children,
}: Readonly<{
  children: React.ReactNode;
}>) {
  return (
    <html
      lang="ru"
      className={`${inter.variable} ${jetbrainsMono.variable} dark`}
    >
      <body className="min-h-screen bg-[var(--color-bg)] text-[var(--color-text)] font-sans antialiased">
        {children}
      </body>
    </html>
  );
}
