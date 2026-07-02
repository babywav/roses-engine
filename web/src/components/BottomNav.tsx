import { motion } from "framer-motion";
import { MessageCircle, Calculator, Eye, Link2, ShieldCheck } from "lucide-react";

export type AppTab = "chat" | "calculos" | "vigilancia" | "portal" | "auditoria";

const TABS: { id: AppTab; label: string; Icon: typeof MessageCircle }[] = [
  { id: "chat", label: "Chat", Icon: MessageCircle },
  { id: "calculos", label: "Cálculos", Icon: Calculator },
  { id: "vigilancia", label: "Vigília", Icon: Eye },
  { id: "portal", label: "Portal", Icon: Link2 },
  { id: "auditoria", label: "Auditoria", Icon: ShieldCheck },
];

interface Props {
  active: AppTab;
  onChange: (tab: AppTab) => void;
}

export default function BottomNav({ active, onChange }: Props) {
  return (
    <div className="flex items-center justify-around px-2 py-2 border-t border-white/[0.06] bg-ink-800/80 backdrop-blur-xl shrink-0">
      {TABS.map(({ id, label, Icon }) => {
        const isActive = active === id;
        return (
          <button
            key={id}
            onClick={() => onChange(id)}
            className="flex flex-col items-center gap-1 px-2 py-1.5 relative min-w-0"
          >
            <div className={`grid place-items-center w-8 h-8 rounded-xl transition-colors ${isActive ? "bg-accent/20" : "bg-transparent"}`}>
              <Icon
                size={18}
                className={`transition-colors ${isActive ? "text-accent-soft" : "text-muted"}`}
              />
            </div>
            <span className={`text-[10px] font-medium transition-colors leading-none ${isActive ? "text-accent-soft" : "text-muted"}`}>
              {label}
            </span>
            {isActive && (
              <motion.div
                layoutId="tab-indicator"
                className="absolute -top-0.5 left-1/2 -translate-x-1/2 w-5 h-0.5 rounded-full bg-accent-soft"
                transition={{ type: "spring", stiffness: 400, damping: 35 }}
              />
            )}
          </button>
        );
      })}
    </div>
  );
}
