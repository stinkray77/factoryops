import type { Metadata } from "next";
import "./globals.css";

export const metadata: Metadata = {
  title: "FactoryOps | Manufacturing Operations",
  description:
    "Track production work, inventory constraints, and purchase-order extraction from one operations workspace.",
  icons: {
    icon: "/favicon.svg",
    shortcut: "/favicon.svg",
  },
};

export default function RootLayout({
  children,
}: Readonly<{
  children: React.ReactNode;
}>) {
  return (
    <html lang="en">
      <body>{children}</body>
    </html>
  );
}
