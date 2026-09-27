import type { Metadata } from "next";
import "./globals.css";

export const metadata: Metadata = {
  title: "Voz do Povo",
  description:
    "Dados públicos sobre os políticos que você elegeu, explicados em linguagem simples.",
};

export default function RootLayout({ children }: LayoutProps<"/">) {
  return (
    <html lang="pt-BR">
      <body>{children}</body>
    </html>
  );
}
