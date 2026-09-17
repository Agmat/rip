import type { Metadata } from "next";
import { Geist, Geist_Mono } from "next/font/google";
import "./globals.css";

const geistSans = Geist({
  variable: "--font-geist-sans",
  subsets: ["latin"],
});

const geistMono = Geist_Mono({
  variable: "--font-geist-mono",
  subsets: ["latin"],
});

export const metadata: Metadata = {
  title: "rip",
  description: "Open Magic: The Gathering booster packs.",
};

export default function RootLayout({ children }: LayoutProps<"/">) {
  return (
    <html
      lang="en"
      className={`${geistSans.variable} ${geistMono.variable} h-full antialiased`}
    >
      <body className="min-h-full flex flex-col bg-neutral-950 text-neutral-100">
        <main className="flex flex-1 flex-col items-center justify-center gap-8 p-6">
          {children}
        </main>
        <footer className="p-4 text-center text-xs text-neutral-500">
          rip is unofficial Fan Content permitted under the Fan Content Policy. Not
          approved/endorsed by Wizards. Portions of the materials used are property of
          Wizards of the Coast. ©Wizards of the Coast LLC.
        </footer>
      </body>
    </html>
  );
}
