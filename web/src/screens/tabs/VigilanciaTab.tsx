import { useState, useEffect } from "react";
import { motion, AnimatePresence } from "framer-motion";
import { Eye, Plus, Bell, Trash2, AlertCircle, X, ChevronRight } from "lucide-react";
import { listarVigilancias, criarVigilancia, excluirVigilancia, listarAlertas } from "../../api";
import type { Vigilancia, VigilanciaAlerta } from "../../types";

type SubTab = "monitorados" | "alertas";

function maskDoc(doc: string) {
  if (doc.length === 11) return `${doc.slice(0, 3)}.***.***-${doc.slice(9)}`;
  if (doc.length === 14) return `${doc.slice(0, 2)}.***.***/****-${doc.slice(12)}`;
  return doc;
}

function fmtDate(d?: string) {
  if (!d) return "nunca";
  try { return new Date(d).toLocaleDateString("pt-BR", { day: "2-digit", month: "short", year: "numeric" }); }
  catch { return d; }
}

export default function VigilanciaTab() {
  const [sub, setSub] = useState<SubTab>("monitorados");
  const [vigilancias, setVigilancias] = useState<Vigilancia[]>([]);
  const [alertas, setAlertas] = useState<VigilanciaAlerta[]>([]);
  const [loading, setLoading] = useState(true);
  const [showForm, setShowForm] = useState(false);
  const [form, setForm] = useState({ documento: "", nome: "", tipo: "parte" });
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [deleting, setDeleting] = useState<string | null>(null);

  async function load() {
    setLoading(true);
    setError(null);
    try {
      const [vs, as] = await Promise.all([listarVigilancias(), listarAlertas()]);
      setVigilancias(vs);
      setAlertas(as);
    } catch (e) {
      setError((e as Error).message);
    } finally {
      setLoading(false);
    }
  }

  useEffect(() => { load(); }, []);

  async function salvar() {
    if (!form.documento.trim() || !form.nome.trim()) {
      setError("Preencha o documento e o nome");
      return;
    }
    setSaving(true);
    setError(null);
    try {
      const v = await criarVigilancia({
        documento: form.documento.replace(/\D/g, ""),
        nome: form.nome,
        tipo: form.tipo,
      });
      setVigilancias((prev) => [v, ...prev]);
      setForm({ documento: "", nome: "", tipo: "parte" });
      setShowForm(false);
    } catch (e) {
      setError((e as Error).message);
    } finally {
      setSaving(false);
    }
  }

  async function remover(id: string) {
    setDeleting(id);
    try {
      await excluirVigilancia(id);
      setVigilancias((prev) => prev.filter((v) => v.id !== id));
    } catch (e) {
      setError((e as Error).message);
    } finally {
      setDeleting(null);
    }
  }

  const novosAlertas = alertas.filter((a) => !a.lido).length;

  return (
    <div className="flex flex-col h-full overflow-hidden">
      {/* Header */}
      <div className="flex items-center gap-3 px-5 py-4 border-b border-white/5 shrink-0">
        <div className="grid place-items-center w-9 h-9 rounded-full bg-accent/15 text-accent-soft">
          <Eye size={18} />
        </div>
        <div className="leading-tight">
          <div className="font-semibold">Vigília</div>
          <div className="text-xs text-muted">Monitoramento de CPF/CNPJ</div>
        </div>
        <button
          onClick={() => { setShowForm(true); setError(null); }}
          className="ml-auto grid place-items-center w-9 h-9 rounded-full bg-accent/20 hover:bg-accent/30 text-accent-soft transition-colors"
        >
          <Plus size={18} />
        </button>
      </div>

      {/* Sub-tabs */}
      <div className="flex gap-1 px-4 pt-3 pb-1 shrink-0">
        {(["monitorados", "alertas"] as SubTab[]).map((t) => (
          <button
            key={t}
            onClick={() => setSub(t)}
            className={`relative flex-1 py-2 rounded-xl text-sm font-medium transition-colors ${sub === t ? "bg-accent/15 text-accent-soft" : "text-muted hover:text-white"}`}
          >
            {t === "alertas" && novosAlertas > 0 && (
              <span className="absolute top-1 right-3 w-4 h-4 rounded-full bg-red-500 text-[10px] text-white flex items-center justify-center font-bold">
                {novosAlertas}
              </span>
            )}
            {t === "monitorados" ? "Monitorados" : "Alertas"}
          </button>
        ))}
      </div>

      {/* Formulário de adição */}
      <AnimatePresence>
        {showForm && (
          <motion.div
            initial={{ height: 0, opacity: 0 }}
            animate={{ height: "auto", opacity: 1 }}
            exit={{ height: 0, opacity: 0 }}
            className="overflow-hidden px-4 shrink-0"
          >
            <div className="glass rounded-2xl p-4 mt-2 space-y-3">
              <div className="flex items-center justify-between">
                <p className="text-sm font-medium">Nova vigília</p>
                <button onClick={() => setShowForm(false)} className="text-muted hover:text-white transition-colors">
                  <X size={16} />
                </button>
              </div>
              <input
                value={form.documento}
                onChange={(e) => setForm((f) => ({ ...f, documento: e.target.value }))}
                placeholder="CPF (11 dígitos) ou CNPJ (14 dígitos)"
                className="w-full bg-white/[0.04] border border-white/[0.08] rounded-xl px-3.5 py-2.5 text-sm outline-none focus:border-accent/50 transition-colors placeholder:text-muted/50"
              />
              <input
                value={form.nome}
                onChange={(e) => setForm((f) => ({ ...f, nome: e.target.value }))}
                placeholder="Nome da parte (ex: João Silva)"
                className="w-full bg-white/[0.04] border border-white/[0.08] rounded-xl px-3.5 py-2.5 text-sm outline-none focus:border-accent/50 transition-colors placeholder:text-muted/50"
              />
              <div className="flex gap-2">
                {["parte", "advogado", "empresa"].map((t) => (
                  <button
                    key={t}
                    onClick={() => setForm((f) => ({ ...f, tipo: t }))}
                    className={`flex-1 py-2 rounded-lg text-xs font-medium capitalize transition-colors ${form.tipo === t ? "bg-accent text-white" : "bg-white/[0.04] text-muted hover:text-white"}`}
                  >
                    {t}
                  </button>
                ))}
              </div>
              {error && (
                <div className="flex items-center gap-2 text-red-400 text-xs">
                  <AlertCircle size={14} /> {error}
                </div>
              )}
              <motion.button
                whileTap={{ scale: 0.97 }}
                onClick={salvar}
                disabled={saving}
                className="w-full py-3 rounded-xl bg-accent text-white text-sm font-semibold disabled:opacity-50"
              >
                {saving ? "Adicionando…" : "Adicionar e verificar agora"}
              </motion.button>
            </div>
          </motion.div>
        )}
      </AnimatePresence>

      {/* Conteúdo */}
      <div className="flex-1 overflow-y-auto px-4 py-3">
        {loading ? (
          <div className="flex items-center justify-center h-32 text-muted text-sm">Carregando…</div>
        ) : error && !showForm ? (
          <div className="flex flex-col items-center justify-center h-32 gap-2 text-center">
            <AlertCircle size={24} className="text-red-400" />
            <p className="text-sm text-muted">{error}</p>
            <button onClick={load} className="text-xs text-accent-soft underline">Tentar novamente</button>
          </div>
        ) : sub === "monitorados" ? (
          <div className="space-y-2.5">
            {vigilancias.length === 0 ? (
              <div className="flex flex-col items-center justify-center h-40 gap-3 text-center">
                <Eye size={32} className="text-muted/40" />
                <p className="text-sm text-muted">Nenhum documento monitorado ainda.</p>
                <button onClick={() => setShowForm(true)} className="text-sm text-accent-soft underline">
                  Adicionar primeiro
                </button>
              </div>
            ) : (
              vigilancias.map((v) => (
                <motion.div
                  key={v.id}
                  initial={{ opacity: 0, y: 8 }}
                  animate={{ opacity: 1, y: 0 }}
                  className="glass rounded-2xl p-4"
                >
                  <div className="flex items-start justify-between gap-2">
                    <div className="min-w-0">
                      <p className="text-sm font-medium truncate">{v.nome}</p>
                      <p className="text-xs text-muted font-mono mt-0.5">{maskDoc(v.documento)}</p>
                    </div>
                    <button
                      onClick={() => remover(v.id)}
                      disabled={deleting === v.id}
                      className="text-muted hover:text-red-400 transition-colors shrink-0 disabled:opacity-40"
                    >
                      <Trash2 size={15} />
                    </button>
                  </div>
                  <div className="flex items-center gap-3 mt-3">
                    <span className={`text-[10px] px-2 py-0.5 rounded-full font-medium ${v.ativo ? "bg-emerald-500/15 text-emerald-400" : "bg-red-500/15 text-red-400"}`}>
                      {v.ativo ? "ativo" : "inativo"}
                    </span>
                    <span className="text-[10px] text-muted capitalize">{v.tipo}</span>
                    <span className="text-[10px] text-muted ml-auto">Última check: {fmtDate(v.ultima_verificacao)}</span>
                  </div>
                </motion.div>
              ))
            )}
          </div>
        ) : (
          <div className="space-y-2.5">
            {alertas.length === 0 ? (
              <div className="flex flex-col items-center justify-center h-40 gap-3 text-center">
                <Bell size={32} className="text-muted/40" />
                <p className="text-sm text-muted">Nenhum alerta registrado.</p>
              </div>
            ) : (
              alertas.map((a) => (
                <motion.div
                  key={a.id}
                  initial={{ opacity: 0, y: 8 }}
                  animate={{ opacity: 1, y: 0 }}
                  className={`glass rounded-2xl p-4 ${!a.lido ? "border-l-2 border-accent" : ""}`}
                >
                  <div className="flex items-start gap-3">
                    <div className={`mt-0.5 w-6 h-6 rounded-full flex items-center justify-center shrink-0 ${a.tipo_alerta === "novo_processo" ? "bg-emerald-500/20 text-emerald-400" : "bg-accent/20 text-accent-soft"}`}>
                      <Bell size={12} />
                    </div>
                    <div className="min-w-0 flex-1">
                      <p className="text-sm font-medium">{a.titulo}</p>
                      {a.numero_processo && (
                        <p className="text-xs font-mono text-muted mt-0.5">{a.numero_processo}</p>
                      )}
                      {a.detalhe && (
                        <p className="text-xs text-muted mt-1 leading-relaxed">{a.detalhe}</p>
                      )}
                      <p className="text-[10px] text-muted/60 mt-1.5">{fmtDate(a.created_at)}</p>
                    </div>
                    {!a.lido && (
                      <div className="w-2 h-2 rounded-full bg-accent-soft mt-1 shrink-0" />
                    )}
                  </div>
                </motion.div>
              ))
            )}
          </div>
        )}
      </div>
    </div>
  );
}
