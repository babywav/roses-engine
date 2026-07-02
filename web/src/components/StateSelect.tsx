import { useState, useRef, useEffect } from "react";
import { motion, AnimatePresence } from "framer-motion";
import { ChevronDown, MapPin } from "lucide-react";

const ESTADOS = [
  { sigla: "AC", nome: "Acre" },
  { sigla: "AL", nome: "Alagoas" },
  { sigla: "AP", nome: "Amapá" },
  { sigla: "AM", nome: "Amazonas" },
  { sigla: "BA", nome: "Bahia" },
  { sigla: "CE", nome: "Ceará" },
  { sigla: "DF", nome: "Distrito Federal" },
  { sigla: "ES", nome: "Espírito Santo" },
  { sigla: "GO", nome: "Goiás" },
  { sigla: "MA", nome: "Maranhão" },
  { sigla: "MT", nome: "Mato Grosso" },
  { sigla: "MS", nome: "Mato Grosso do Sul" },
  { sigla: "MG", nome: "Minas Gerais" },
  { sigla: "PA", nome: "Pará" },
  { sigla: "PB", nome: "Paraíba" },
  { sigla: "PR", nome: "Paraná" },
  { sigla: "PE", nome: "Pernambuco" },
  { sigla: "PI", nome: "Piauí" },
  { sigla: "RJ", nome: "Rio de Janeiro" },
  { sigla: "RN", nome: "Rio Grande do Norte" },
  { sigla: "RS", nome: "Rio Grande do Sul" },
  { sigla: "RO", nome: "Rondônia" },
  { sigla: "RR", nome: "Roraima" },
  { sigla: "SC", nome: "Santa Catarina" },
  { sigla: "SP", nome: "São Paulo" },
  { sigla: "SE", nome: "Sergipe" },
  { sigla: "TO", nome: "Tocantins" },
];

interface Props {
  value: string;
  onChange: (v: string) => void;
  error?: string;
}

export default function StateSelect({ value, onChange, error }: Props) {
  const [open, setOpen] = useState(false);
  const [search, setSearch] = useState("");
  const ref = useRef<HTMLDivElement>(null);

  useEffect(() => {
    function handleClick(e: MouseEvent) {
      if (ref.current && !ref.current.contains(e.target as Node)) {
        setOpen(false);
      }
    }
    document.addEventListener("mousedown", handleClick);
    return () => document.removeEventListener("mousedown", handleClick);
  }, []);

  const filtered = ESTADOS.filter(
    (e) =>
      e.sigla.toLowerCase().includes(search.toLowerCase()) ||
      e.nome.toLowerCase().includes(search.toLowerCase())
  );

  const selected = ESTADOS.find((e) => e.sigla === value);

  return (
    <div className="space-y-2">
      <label className="block text-[11px] font-semibold tracking-widest text-muted uppercase">
        Estado de emissão
      </label>
      <div ref={ref} className="relative">
        <button
          type="button"
          onClick={() => setOpen(!open)}
          className={[
            "w-full glass rounded-2xl px-4 py-3.5 flex items-center gap-3 text-left transition-all duration-200",
            "focus:ring-2 focus:ring-accent/40 focus:border-accent/30",
            error ? "ring-2 ring-red-500/40 border-red-500/30" : "",
          ].join(" ")}
        >
          <MapPin size={16} className="text-muted shrink-0" />
          <span className={selected ? "text-white text-[15px]" : "text-muted/50 text-[15px]"}>
            {selected ? `${selected.sigla} — ${selected.nome}` : "Selecione o estado"}
          </span>
          <ChevronDown
            size={16}
            className={[
              "ml-auto text-muted transition-transform duration-200 shrink-0",
              open ? "rotate-180" : "",
            ].join(" ")}
          />
        </button>

        <AnimatePresence>
          {open && (
            <motion.div
              initial={{ opacity: 0, y: -8, scale: 0.97 }}
              animate={{ opacity: 1, y: 0, scale: 1 }}
              exit={{ opacity: 0, y: -8, scale: 0.97 }}
              transition={{ duration: 0.15 }}
              className="absolute left-0 right-0 top-full mt-2 z-30 glass rounded-2xl overflow-hidden shadow-card"
            >
              <div className="p-2">
                <input
                  autoFocus
                  value={search}
                  onChange={(e) => setSearch(e.target.value)}
                  placeholder="Buscar estado..."
                  className="w-full bg-white/[0.04] rounded-xl px-3 py-2.5 text-sm outline-none placeholder:text-muted/50"
                />
              </div>
              <div className="max-h-48 overflow-y-auto px-1 pb-1">
                {filtered.map((e) => (
                  <button
                    key={e.sigla}
                    type="button"
                    onClick={() => {
                      onChange(e.sigla);
                      setOpen(false);
                      setSearch("");
                    }}
                    className={[
                      "w-full text-left px-3 py-2.5 rounded-xl text-sm transition-colors",
                      e.sigla === value
                        ? "bg-accent/15 text-accent-soft"
                        : "hover:bg-white/[0.06] text-white/80",
                    ].join(" ")}
                  >
                    <span className="font-mono font-semibold mr-2">{e.sigla}</span>
                    <span className="text-muted">{e.nome}</span>
                  </button>
                ))}
                {filtered.length === 0 && (
                  <div className="px-3 py-4 text-center text-sm text-muted">
                    Nenhum estado encontrado
                  </div>
                )}
              </div>
            </motion.div>
          )}
        </AnimatePresence>
      </div>
      {error && <p className="text-xs text-red-400 pl-1">{error}</p>}
    </div>
  );
}
