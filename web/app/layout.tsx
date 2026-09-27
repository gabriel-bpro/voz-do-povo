import type { Metadata } from "next";
import { Roboto } from "next/font/google";
import "./globals.css";

const roboto = Roboto({
  subsets: ["latin"],
  weight: ["400", "500", "700"],
  variable: "--font-roboto",
});

export const metadata: Metadata = {
  title: "Voz do Povo",
  description:
    "Dados públicos sobre os políticos que você elegeu, explicados em linguagem simples.",
};

export default function RootLayout({ children }: LayoutProps<"/">) {
  return (
    <html lang="pt-BR" className={roboto.variable}>
      <body>{children}</body>
    </html>
  );
}
