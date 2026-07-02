import { useState, useEffect } from "react";
import { motion, AnimatePresence } from "framer-motion";
import { Link2, Plus, Copy, Trash2, Check, AlertCircle, X, ExternalLink } from "lucide-react";
import { listarPortalLinks, criarPortalLink, revogarPortalLink } from "../../api";
import type { PortalLink } from "../../types";

function fmtDate(d?: string) {
  if (!d) return null;
  try { return new Date(d).toLocaleDateString("pt-BR", { day: "2-digit", month: "short", year: "numeric" }); }
  catch { return d; }
}

function CopyBtn({ text }: { text: string }) {
  const [copied, setCopied] = useState(false);

  async function copy() {
    try {
      await navigator.clipboard.writeText(text);
      setCopied(true);
      setTimeout(() => setCopied(false), 2000);
    } catch { /* ignore */ }
  }

  return (
    <button
      onClick={copy}
      className={`grid place-items-center w-8 h-8 rounded-full transition-colors ${copied ? "bg-emerald-500/20 text-emerald-400" : "bg-white/[0.05] hover:bg-white/[0.1] text-muted hover:text-white"}`}
    >
      {copied ? <Check size={14} /> : <Copy size={14} />}
    </button>
  );
}

export default function PortalTab() {
  const [links, setLinks] = useState<PortalLink[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [showForm, setShowForm] = useState(false);
  const [form, setForm] = useState({ numero: "", nome: "", dias: "" });
  const [saving, setSaving] = useState(false);
  const [formError, setFormError] = useState<string | null>(null);
  const [deleting, setDeleting] = useState<string | null>(null);

  async function load() {
    setLoading(true);
    setError(null);
    try {
      const data = await listarPortalLinks();
      setLinks(data);
    } catch (e) {
      setError((e as Error).message);
    } finally {
      setLoading(false);
    }
  }

  useEffect(() => { load(); }, []);

  async function criar() {
    if (!form.numero.trim()) {
      setFormError("Informe o número do processo");
      return;
    }
    setSaving(true);
    setFormError(null);
    try {
      const res = await criarPortalLink({
        numero_processo: form.numero.trim(),
        nome_cliente: form.nome.trim() || undefined,
        dias_validade: form.dias ? parseInt(form.dias) : undefined,
      });
      // Recarrega lista para pegar o item completo
      await load();
      setForm({ numero: "", nome: "", dias: "" });
      setShowForm(false);
    } catch (e) {
      setFormError((e as Error).message);
    } finally {
      setSaving(false);
    }
  }

  async function revogar(id: string) {
    setDeleting(id);
    try {
      await revogarPortalLink(id);
      setLinks((prev) => prev.map((l) => l.id === id ? { ...l, ativo: false } : l));
    } catch (e) {
      setError((e as Error).message);
    } finally {
      setDeleting(null);
    }
  }

  function buildUrl(link: PortalLink) {
    return `${window.location.origin}/api/portal/${link.token}`;
  }

  return (
    <div className="flex flex-col h-full overflow-hidden">
      {/* Header */}
      <div className="flex items-center gap-3 px-5 py-4 border-b border-white/5 shrink-0">
        <div className="grid place-items-center w-9 h-9 rounded-full bg-accent/15 text-accent-soft">
          <Link2 size={18} />
        </div>
        <div className="leading-tight">
          <div className="font-semibold">Portal do Cliente</div>
          <div className="text-xs text-muted">Links de acesso sem login</div>
        </div>
        <button
          onClick={() => { setShowForm(true); setFormError(null); }}
          className="ml-auto grid place-items-center w-9 h-9 rounded-full bg-accent/20 hover:bg-accent/30 text-accent-soft transition-colors"
        >
          <Plus size={18} />
        </button>
      </div>

      {/* Formulário */}
      <AnimatePresence>
        {showForm && (
          <motion.div
            initial={{ height: 0, opacity: 0 }}
            animate={{ height: "auto", opacity: 1 }}
            exit={{ height: 0, opacity: 0 }}
            className="overflow-hidden px-4 shrink-0"
          >
            <div className="glass rounded-2xl p-4 mt-3 space-y-3">
              <div className="flex items-center justify-between">
                <p className="text-sm font-medium">Gerar link</p>
                <button onClick={() => setShowForm(false)} className="text-muted hover:text-white transition-colors">
                  <X size={16} />
                </button>
              </div>
              <input
                value={form.numero}
                onChange={(e) => setForm((f) => ({ ...f, numero: e.target.value }))}
                placeholder="Número CNJ do processo"
                className="w-full bg-white/[0.04] border border-white/[0.08] rounded-xl px-3.5 py-2.5 text-sm outline-none focus:border-accent/50 transition-colors placeholder:text-muted/50 font-mono"
              />
              <input
                value={form.nome}
                onChange={(e) => setForm((f) => ({ ...f, nome: e.target.value }))}
                placeholder="Nome do cliente (opcional)"
                className="w-full bg-white/[0.04] border border-white/[0.08] rounded-xl px-3.5 py-2.5 text-sm outline-none focus:border-accent/50 transition-colors placeholder:text-muted/50"
              />
              <div className="flex gap-2">
                {["7", "30", "90"].map((d) => (
                  <button
                    key={d}
                    onClick={() => setForm((f) => ({ ...f, dias: f.dias === d ? "" : d }))}
                    className={`flex-1 py-2 rounded-lg text-xs font-medium transition-colors ${form.dias === d ? "bg-accent text-white" : "bg-white/[0.04] text-muted hover:text-white"}`}
                  >
                    {d} dias
                  </button>
                ))}
                <button
                  onClick={() => setForm((f) => ({ ...f, dias: "" }))}
                  className={`flex-1 py-2 rounded-lg text-xs font-medium transition-colors ${!form.dias ? "bg-accent text-white" : "bg-white/[0.04] text-muted hover:text-white"}`}
                >
                  Sem expirar
                </button>
              </div>
              {formError && (
                <div className="flex items-center gap-2 text-red-400 text-xs">
                  <AlertCircle size={14} /> {formError}
                </div>
              )}
              <motion.button
                whileTap={{ scale: 0.97 }}
                onClick={criar}
                disabled={saving}
                className="w-full py-3 rounded-xl bg-accent text-white text-sm font-semibold disabled:opacity-50"
              >
                {saving ? "Gerando…" : "Gerar link"}
              </motion.button>
            </div>
          </motion.div>
        )}
      </AnimatePresence>

      {/* Lista */}
      <div className="flex-1 overflow-y-auto px-4 py-3">
        {loading ? (
          <div className="flex items-center justify-center h-32 text-muted text-sm">Carregando…</div>
        ) : error ? (
          <div className="flex flex-col items-center justify-center h-32 gap-2 text-center">
            <AlertCircle size={24} className="text-red-400" />
            <p className="text-sm text-muted">{error}</p>
            <button onClick={load} className="text-xs text-accent-soft underline">Tentar novamente</button>
          </div>
        ) : links.length === 0 ? (
          <div className="flex flex-col items-center justify-center h-48 gap-3 text-center">
            <Link2 size={36} className="text-muted/30" />
            <p className="text-sm text-muted">Nenhum link gerado ainda.</p>
            <p className="text-xs text-muted/60">Gere um link para que o cliente acompanhe o processo sem precisar de conta.</p>
            <button onClick={() => setShowForm(true)} className="text-sm text-accent-soft underline mt-1">
              Gerar primeiro link
            </button>
          </div>
        ) : (
          <div className="space-y-2.5">
            {links.map((link) => (
              <motion.div
                key={link.id}
                initial={{ opacity: 0, y: 8 }}
                animate={{ opacity: 1, y: 0 }}
                className={`glass rounded-2xl p-4 ${!link.ativo ? "opacity-50" : ""}`}
              >
                <div className="flex items-start justify-between gap-2">
                  <div className="min-w-0">
                    {link.nome_cliente && (
                      <p className="text-sm font-medium truncate">{link.nome_cliente}</p>
                    )}
                    <p className="text-xs font-mono text-muted mt-0.5 truncate">{link.numero_processo}</p>
                  </div>
                  <span className={`text-[10px] px-2 py-0.5 rounded-full font-medium shrink-0 mt-0.5 ${link.ativo ? "bg-emerald-500/15 text-emerald-400" : "bg-red-500/15 text-red-400"}`}>
                    {link.ativo ? "ativo" : "revogado"}
                  </span>
                </div>

                <div className="flex items-center gap-1.5 mt-3 bg-white/[0.04] rounded-xl px-3 py-2">
                  <span className="text-[11px] text-muted font-mono truncate flex-1 min-w-0">/portal/{link.token}</span>
                  <CopyBtn text={buildUrl(link)} />
                  <a
                    href={buildUrl(link)}
                    target="_blank"
                    rel="noopener noreferrer"
                    className="grid place-items-center w-8 h-8 rounded-full bg-white/[0.05] hover:bg-white/[0.1] text-muted hover:text-white transition-colors"
                  >
                    <ExternalLink size={13} />
                  </a>
                </div>

                <div className="flex items-center justify-between mt-3">
                  <div className="flex items-center gap-3">
                    <span className="text-[10px] text-muted">{link.acessos} acesso{link.acessos !== 1 ? "s" : ""}</span>
                    {link.ultimo_acesso && (
                      <span className="text-[10px] text-muted">Último: {fmtDate(link.ultimo_acesso)}</span>
                    )}
                    {link.expira_em && (
                      <span className="text-[10px] text-muted">Expira: {fmtDate(link.expira_em)}</span>
                    )}
                  </div>
                  {link.ativo && (
                    <button
                      onClick={() => revogar(link.id)}
                      disabled={deleting === link.id}
                      className="text-muted hover:text-red-400 transition-colors disabled:opacity-40"
                    >
                      <Trash2 size={14} />
                    </button>
                  )}
                </div>
              </motion.div>
            ))}
          </div>
        )}
      </div>
    </div>
  );
}
