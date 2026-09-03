"use client";

import {
  createContext,
  useCallback,
  useContext,
  useEffect,
  useMemo,
  useRef,
  useState,
} from "react";

/**
 * Agent nima qilayotganini ko'rsatadigan lenta.
 *
 * ┌─ NEGA BU KERAK ────────────────────────────────────────────────────┐
 * WebMCP'da sahifani agent boshqaradi: ekran o'zi almashadi, savatga
 * taom qo'shiladi. Bu foydalanuvchi uchun tushunarsiz — u nima
 * bo'layotganini bilmasa, ilova buzuq yoki birov boshqarayotgandek
 * ko'rinadi.
 *
 * Lenta ikkita savolga javob beradi: KIM qilyapti va NIMA qilyapti.
 * Uchinchi javob — "buni qanday to'xtataman" — brauzerning o'zida
 * (agentni to'xtatish), shuning uchun bu yerda tugma yo'q: bo'lmagan
 * boshqaruvni ko'rsatish yolg'on bo'lardi.
 * └────────────────────────────────────────────────────────────────────┘
 *
 * ┌─ NEGA MOBIL ILOVADAGIDAN SODDAROQ ─────────────────────────────────┐
 * Flutter'da "barmoq" animatsiyasi bor, chunki u yerda amalni
 * ilovaning O'ZI bajaradi va qaysi tugma bosilayotganini ko'rsatish
 * kerak. Bu yerda amalni agent chaqiradi va natija darhol ekranda
 * ko'rinadi (menyu ochiladi, savat to'ladi) — qo'shimcha animatsiya
 * ortiqcha bo'lardi.
 * └────────────────────────────────────────────────────────────────────┘
 */

type Activity = {
  /** Oxirgi amallar (eng yangisi oxirida). */
  log: string[];
  push: (message: string) => void;
};

const AgentActivityContext = createContext<Activity | null>(null);

/** Lenta shuncha vaqt harakatsizlikdan keyin yashiriladi. */
const HIDE_AFTER_MS = 6000;

export function AgentActivityProvider({
  children,
}: {
  children: React.ReactNode;
}) {
  const [log, setLog] = useState<string[]>([]);
  const [visible, setVisible] = useState(false);
  const timer = useRef<ReturnType<typeof setTimeout> | null>(null);

  const push = useCallback((message: string) => {
    setLog((prev) => [...prev.slice(-4), message]);
    setVisible(true);
  }, []);

  useEffect(() => {
    if (!visible) return;
    if (timer.current) clearTimeout(timer.current);
    timer.current = setTimeout(() => setVisible(false), HIDE_AFTER_MS);
    return () => {
      if (timer.current) clearTimeout(timer.current);
    };
  }, [log, visible]);

  const value = useMemo<Activity>(() => ({ log, push }), [log, push]);

  return (
    <AgentActivityContext.Provider value={value}>
      {children}
      {visible && log.length > 0 && <ActivityBar text={log[log.length - 1]} />}
    </AgentActivityContext.Provider>
  );
}

export function useAgentActivity(): Activity {
  const ctx = useContext(AgentActivityContext);
  // Provider bo'lmasa ham ishlashi kerak: WebMCP tool'lari sahifaning
  // boshqa joylarida ham ishlatilishi mumkin va lenta yo'qligi
  // sababli butun amal yiqilmasligi kerak.
  return ctx ?? { log: [], push: () => {} };
}

function ActivityBar({ text }: { text: string }) {
  return (
    <div
      role="status"
      aria-live="polite"
      className="pointer-events-none fixed inset-x-0 top-0 z-50 flex justify-center px-3 pt-3"
    >
      <div className="pointer-events-auto flex max-w-md items-center gap-2.5 rounded-2xl bg-neutral-900/92 px-3.5 py-2.5 text-white shadow-lg backdrop-blur">
        <span className="relative flex h-2 w-2 shrink-0">
          <span className="absolute inline-flex h-full w-full animate-ping rounded-full bg-brand opacity-75" />
          <span className="relative inline-flex h-2 w-2 rounded-full bg-brand" />
        </span>
        <div className="min-w-0">
          <div className="text-[11px] font-bold leading-tight">
            Shaddiy Ai Agent
          </div>
          <div className="truncate text-[11.5px] leading-tight text-neutral-300">
            {text}
          </div>
        </div>
      </div>
    </div>
  );
}
