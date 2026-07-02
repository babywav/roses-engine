import { motion } from "framer-motion";
import { X, ChevronDown } from "lucide-react";
import type { QuerySettings } from "../types";
import Pill from "./Pill";

interface Props {
  settings: QuerySettings;
  onChange: (s: QuerySettings) => void;
  onClose: () => void;
}

function Section({ title, children }: { title: string; children: React.ReactNode }) {
  return (
    <div className="glass rounded-3xl p-4">
      <div className="flex items-center justify-between">
        <h3 className="font-semibold">{title}</h3>
        <ChevronDown size={18} className="text-muted" />
      </div>
      <div className="mt-4 space-y-5">{children}</div>
    </div>
  );
}

function Label({ children }: { children: React.ReactNode }) {
  return <div className="text-[11px] font-semibold tracking-widest text-muted uppercase">{children}</div>;
}

export default function SettingsPanel({ settings, onChange, onClose }: Props) {
  const set = (patch: Partial<QuerySettings>) => onChange({ ...settings, ...patch });

  return (
    <motion.div
      initial={{ x: "100%" }}
      animate={{ x: 0 }}
      exit={{ x: "100%" }}
      transition={{ type: "spring", stiffness: 320, damping: 34 }}
      className="absolute inset-0 z-20 flex flex-col bg-ink-900/95 backdrop-blur-xl"
    >
      <div className="flex items-center justify-between px-5 py-4">
        <h2 className="text-xl font-semibold tracking-tight">Configurações da Consulta</h2>
        <button
          onClick={onClose}
          className="grid place-items-center w-9 h-9 rounded-full bg-white/[0.06] hover:bg-white/[0.12] text-muted hover:text-white transition-colors"
        >
          <X size={18} />
        </button>
      </div>

      <div className="flex-1 overflow-y-auto px-5 pb-5 space-y-4">
        <Section title="Saída">
          <div>
            <Label>Formato do resultado</Label>
            <div className="mt-2.5 flex flex-wrap gap-2">
              {(["resumo", "lista", "completo"] as const).map((v) => (
                <Pill key={v} label={cap(v)} active={settings.saida === v} onClick={() => set({ saida: v })} />
              ))}
            </div>
          </div>
          <div>
            <Label>Movimentações</Label>
            <div className="mt-2.5">
              <Toggle
                on={settings.incluirMovimentacoes}
                onToggle={() => set({ incluirMovimentacoes: !settings.incluirMovimentacoes })}
                label="Incluir histórico de movimentações"
              />
            </div>
          </div>
        </Section>

        <Section title="Fonte dos dados">
          <div>
            <Label>De onde buscar</Label>
            <div className="mt-2.5 flex flex-wrap gap-2">
              {(["auto", "datajud", "portal"] as const).map((v) => (
                <Pill key={v} label={fonteLabel(v)} active={settings.fonte === v} onClick={() => set({ fonte: v })} />
              ))}
            </div>
            <p className="mt-2 text-xs text-muted leading-relaxed">
              {settings.fonte === "datajud"
                ? "API oficial do CNJ. Rápido, por número, sem captcha."
                : settings.fonte === "portal"
                  ? "Portal do tribunal. Necessário para busca por nome/OAB."
                  : "Escolhe sozinho: número → DataJud; nome/OAB → portal."}
            </p>
          </div>
          <div>
            <Label>UF padrão</Label>
            <input
              value={settings.ufPadrao}
              onChange={(e) => set({ ufPadrao: e.target.value.toUpperCase().slice(0, 2) })}
              placeholder="PB"
              className="mt-2.5 w-full glass rounded-2xl px-4 py-3 outline-none font-mono tracking-widest placeholder:text-muted/60"
            />
          </div>
        </Section>
      </div>

      <div className="p-4 flex gap-3">
        <motion.button
          whileTap={{ scale: 0.97 }}
          onClick={onClose}
          className="flex-1 py-3.5 rounded-full bg-accent text-white font-semibold shadow-glow"
        >
          Salvar
        </motion.button>
        <button className="flex-1 py-3.5 rounded-full glass font-medium text-muted hover:text-white transition-colors">
          Restaurar
        </button>
      </div>
    </motion.div>
  );
}

function Toggle({ on, onToggle, label }: { on: boolean; onToggle: () => void; label: string }) {
  return (
    <button onClick={onToggle} className="flex items-center gap-3 w-full">
      <span
        className={[
          "relative w-11 h-6 rounded-full transition-colors shrink-0",
          on ? "bg-accent" : "bg-white/15",
        ].join(" ")}
      >
        <motion.span
          layout
          transition={{ type: "spring", stiffness: 500, damping: 32 }}
          className="absolute top-0.5 w-5 h-5 rounded-full bg-white"
          style={{ left: on ? 22 : 2 }}
        />
      </span>
      <span className="text-sm text-white/85">{label}</span>
    </button>
  );
}

const cap = (s: string) => s.charAt(0).toUpperCase() + s.slice(1);
const fonteLabel = (v: string) => (v === "auto" ? "Automático" : v === "datajud" ? "DataJud" : "Portal");
