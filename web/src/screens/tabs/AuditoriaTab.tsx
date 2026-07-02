import { useState, useEffect } from "react";
import { motion } from "framer-motion";
import { ShieldCheck, Search, AlertTriangle, CheckCircle, AlertCircle, Clock } from "lucide-react";
import { auditoriaProcesso, auditoriaDivergencias } from "../../api";
import type { AuditoriaEntry } from "../../types";

type Mode = "divergencias" | "processo";

function fmtDate(d?: string) {
  if (!d) return "—";
  if (/^\d{4}-\d{2}-\d{2}$/.test(d)) {
    const [y, m, day] = d.split("-");
    return `${day}/${m}/${y}`;
  }
  try { return new Date(d).toLocaleDateString("pt-BR"); } catch { return d; }
}

function EntryCard({ entry }: { entry: AuditoriaEntry }) {
  return (
    <motion.div
      initial={{ opacity: 0, y: 8 }}
      animate={{ opacity: 1, y: 0 }}
      className={`glass rounded-2xl p-4 ${entry.divergencia ? "border-l-2 border-amber-400" : ""}`}
    >
      <div className="flex items-start gap-3">
        <div className={`mt-0.5 w-7 h-7 rounded-full flex items-center justify-center shrink-0 ${entry.divergencia ? "bg-amber-500/20 text-amber-400" : "bg-emerald-500/15 text-emerald-400"}`}>
          {entry.divergencia ? <AlertTriangle size={14} /> : <CheckCircle size={14} />}
        </div>
        <div className="flex-1 min-w-0">
          <div className="flex items-center gap-2 flex-wrap">
            <span className="text-xs font-mono text-muted truncate">{entry.numero_processo}</span>
            <span className={`text-[10px] px-2 py-0.5 rounded-full font-medium shrink-0 ${
              entry.fonte === "djen" ? "bg-accent/15 text-accent-soft" :
              entry.fonte === "datajud" ? "bg-purple-500/15 text-purple-400" :
              "bg-white/10 text-muted"
            }`}>
              {entry.fonte?.toUpperCase()}
            </span>
          </div>

          {entry.regra && (
            <p className="text-sm font-medium mt-1.5">{entry.regra}</p>
          )}

          <div className="grid grid-cols-2 gap-x-4 gap-y-1 mt-2.5">
            {entry.data_disponibilizacao && (
              <div>
                <p className="text-[10px] text-muted uppercase">Disponibilização</p>
                <p className="text-xs">{fmtDate(entry.data_disponibilizacao)}</p>
              </div>
            )}
            {entry.data_publicacao && (
              <div>
                <p className="text-[10px] text-muted uppercase">Publicação</p>
                <p className="text-xs">{fmtDate(entry.data_publicacao)}</p>
              </div>
            )}
            {entry.dias_uteis !== undefined && (
              <div>
                <p className="text-[10px] text-muted uppercase">Dias úteis</p>
                <p className="text-xs">{entry.dias_uteis} dias</p>
              </div>
            )}
            {entry.vencimento && (
              <div>
                <p className="text-[10px] text-muted uppercase">Vencimento</p>
                <p className={`text-xs font-medium ${entry.divergencia ? "text-amber-400" : "text-emerald-400"}`}>
                  {fmtDate(entry.vencimento)}
                </p>
              </div>
            )}
          </div>

          {entry.base_legal && (
            <p className="text-[10px] text-muted mt-2 italic">{entry.base_legal}</p>
          )}

          {entry.divergencia && entry.detalhe_divergencia && (
            <div className="mt-2.5 bg-amber-500/10 rounded-xl px-3 py-2">
              <p className="text-[11px] text-amber-300 leading-relaxed">{entry.detalhe_divergencia}</p>
            </div>
          )}

          {entry.feriados_considerados?.length > 0 && (
            <div className="mt-2">
              <p className="text-[10px] text-muted uppercase mb-1">Feriados considerados</p>
              <div className="flex flex-wrap gap-1">
                {entry.feriados_considerados.map((f, i) => (
                  <span key={i} className="text-[10px] bg-white/[0.04] rounded px-1.5 py-0.5 text-muted">{f}</span>
                ))}
              </div>
            </div>
          )}
        </div>
      </div>
    </motion.div>
  );
}

export default function AuditoriaTab() {
  const [mode, setMode] = useState<Mode>("divergencias");
  const [entries, setEntries] = useState<AuditoriaEntry[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [search, setSearch] = useState("");
  const [searching, setSearching] = useState(false);

  async function loadDivergencias() {
    setLoading(true);
    setError(null);
    try {
      const data = await auditoriaDivergencias();
      setEntries(data ?? []);
    } catch (e) {
      setError((e as Error).message);
    } finally {
      setLoading(false);
    }
  }

  async function buscarProcesso() {
    if (!search.trim()) return;
    setSearching(true);
    setError(null);
    setEntries([]);
    try {
      const data = await auditoriaProcesso(search.trim());
      setEntries(data ?? []);
    } catch (e) {
      setError((e as Error).message);
    } finally {
      setSearching(false);
    }
  }

  useEffect(() => {
    if (mode === "divergencias") loadDivergencias();
    else { setEntries([]); setLoading(false); }
  }, [mode]);

  const totalDiverg = entries.filter((e) => e.divergencia).length;

  return (
    <div className="flex flex-col h-full overflow-hidden">
      {/* Header */}
      <div className="flex items-center gap-3 px-5 py-4 border-b border-white/5 shrink-0">
        <div className="grid place-items-center w-9 h-9 rounded-full bg-accent/15 text-accent-soft">
          <ShieldCheck size={18} />
        </div>
        <div className="leading-tight">
          <div className="font-semibold">Auditoria de Prazos</div>
          <div className="text-xs text-muted">Prova do cálculo · Divergências DJEN × DataJud</div>
        </div>
      </div>

      {/* Modo */}
      <div className="flex gap-1 px-4 pt-3 pb-1 shrink-0">
        <button
          onClick={() => setMode("divergencias")}
          className={`flex-1 py-2 rounded-xl text-sm font-medium transition-colors ${mode === "divergencias" ? "bg-accent/15 text-accent-soft" : "text-muted hover:text-white"}`}
        >
          Divergências
          {totalDiverg > 0 && mode === "divergencias" && (
            <span className="ml-1.5 text-[10px] bg-amber-500/25 text-amber-400 px-1.5 py-0.5 rounded-full">{totalDiverg}</span>
          )}
        </button>
        <button
          onClick={() => setMode("processo")}
          className={`flex-1 py-2 rounded-xl text-sm font-medium transition-colors ${mode === "processo" ? "bg-accent/15 text-accent-soft" : "text-muted hover:text-white"}`}
        >
          Por processo
        </button>
      </div>

      {/* Busca por processo */}
      {mode === "processo" && (
        <div className="px-4 pt-2 pb-1 shrink-0">
          <div className="flex items-center gap-2 glass rounded-xl pl-3.5 pr-2 py-2">
            <Search size={16} className="text-muted shrink-0" />
            <input
              value={search}
              onChange={(e) => setSearch(e.target.value)}
              onKeyDown={(e) => e.key === "Enter" && buscarProcesso()}
              placeholder="Número CNJ do processo…"
              className="flex-1 bg-transparent outline-none text-sm placeholder:text-muted/60 font-mono"
            />
            <button
              onClick={buscarProcesso}
              disabled={searching || !search.trim()}
              className="px-3 py-1.5 rounded-lg bg-accent text-white text-xs font-medium disabled:opacity-50 transition-opacity"
            >
              {searching ? "…" : "Buscar"}
            </button>
          </div>
        </div>
      )}

      {/* Conteúdo */}
      <div className="flex-1 overflow-y-auto px-4 py-3">
        {loading || searching ? (
          <div className="flex items-center justify-center h-32 gap-2 text-muted text-sm">
            <Clock size={16} className="animate-spin opacity-50" />
            Carregando…
          </div>
        ) : error ? (
          <div className="flex flex-col items-center justify-center h-32 gap-2 text-center">
            <AlertCircle size={24} className="text-red-400" />
            <p className="text-sm text-muted">{error}</p>
            <button onClick={mode === "divergencias" ? loadDivergencias : buscarProcesso} className="text-xs text-accent-soft underline">
              Tentar novamente
            </button>
          </div>
        ) : entries.length === 0 ? (
          <div className="flex flex-col items-center justify-center h-48 gap-3 text-center px-4">
            {mode === "divergencias" ? (
              <>
                <CheckCircle size={36} className="text-emerald-400/40" />
                <p className="text-sm text-muted">Nenhuma divergência detectada.</p>
                <p className="text-xs text-muted/60">Todos os prazos DJEN coincidem com a estimativa DataJud.</p>
              </>
            ) : (
              <>
                <Search size={36} className="text-muted/30" />
                <p className="text-sm text-muted">Busque um processo para ver a trilha de auditoria.</p>
              </>
            )}
          </div>
        ) : (
          <div className="space-y-2.5">
            {mode === "processo" && (
              <p className="text-xs text-muted pb-1">{entries.length} registro{entries.length !== 1 ? "s" : ""} encontrado{entries.length !== 1 ? "s" : ""}</p>
            )}
            {entries.map((e) => <EntryCard key={e.id} entry={e} />)}
          </div>
        )}
      </div>
    </div>
  );
}
