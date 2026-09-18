import { NextResponse } from "next/server";
import type { NextRequest } from "next/server";

export function middleware(request: NextRequest) {
  const authToken = request.cookies.get("auth_token")?.value;
  const { pathname } = request.nextUrl;

  // Unauthenticated user attempting to access /chat or subpaths
  if (!authToken && pathname.startsWith("/chat")) {
    const loginUrl = new URL("/", request.url);
    return NextResponse.redirect(loginUrl);
  }

  // Authenticated user landing on root welcome page
  if (authToken && pathname === "/") {
    const chatUrl = new URL("/chat", request.url);
    return NextResponse.redirect(chatUrl);
  }

  return NextResponse.next();
}

export const config = {
  matcher: ["/", "/chat/:path*"],
};
