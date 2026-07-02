import { useState } from "react";
import { motion, AnimatePresence } from "framer-motion";
import { Calculator, ChevronDown, RotateCcw, AlertCircle } from "lucide-react";
import { realizarCalculo, fmt, fmtDate } from "../../api";
import type { TipoCalculo, IndiceCorrecao, CalculoResultado } from "../../types";

const TIPOS: { value: TipoCalculo; label: string; desc: string }[] = [
  { value: "correcao_monetaria", label: "Correção Monetária", desc: "SELIC, INPC ou TR" },
  { value: "liquidacao", label: "Liquidação de Sentença", desc: "Principal + correção + juros + honorários" },
  { value: "trabalhista", label: "Verbas Trabalhistas", desc: "FGTS, 13º, férias, aviso, multa 477" },
  { value: "juros", label: "Juros sobre Débito", desc: "Simples ou compostos" },
];

const INDICES: { value: IndiceCorrecao; label: string }[] = [
  { value: "selic", label: "SELIC" },
  { value: "inpc", label: "INPC" },
  { value: "tr", label: "TR" },
];

interface FormState {
  tipo: TipoCalculo;
  valor: string;
  dataBase: string;
  dataFim: string;
  indice: IndiceCorrecao;
  jurosMes: string;
  honorarios: string;
  salario: string;
  meses: string;
  avisoPrevio: boolean;
  faltaGrave: boolean;
  composto: boolean;
}

const FORM_EMPTY: FormState = {
  tipo: "correcao_monetaria",
  valor: "",
  dataBase: "",
  dataFim: "",
  indice: "selic",
  jurosMes: "1",
  honorarios: "10",
  salario: "",
  meses: "",
  avisoPrevio: true,
  faltaGrave: false,
  composto: false,
};

function Input({ label, value, onChange, type = "text", placeholder = "" }: {
  label: string; value: string; onChange: (v: string) => void; type?: string; placeholder?: string;
}) {
  return (
    <div className="space-y-1.5">
      <label className="text-xs text-muted font-medium uppercase tracking-wide">{label}</label>
      <input
        type={type}
        value={value}
        onChange={(e) => onChange(e.target.value)}
        placeholder={placeholder}
        className="w-full bg-white/[0.04] border border-white/[0.08] rounded-xl px-3.5 py-2.5 text-sm outline-none focus:border-accent/50 transition-colors placeholder:text-muted/50"
      />
    </div>
  );
}

function Toggle({ label, checked, onChange }: { label: string; checked: boolean; onChange: (v: boolean) => void }) {
  return (
    <button
      type="button"
      onClick={() => onChange(!checked)}
      className="flex items-center justify-between w-full py-2"
    >
      <span className="text-sm text-ink-100">{label}</span>
      <div className={`w-10 h-5.5 rounded-full transition-colors relative ${checked ? "bg-accent" : "bg-white/10"}`}
        style={{ height: "22px" }}>
        <div className={`absolute top-0.5 w-4.5 h-4.5 rounded-full bg-white shadow transition-transform ${checked ? "translate-x-5" : "translate-x-0.5"}`}
          style={{ width: "18px", height: "18px", transform: checked ? "translateX(20px)" : "translateX(2px)" }} />
      </div>
    </button>
  );
}

export default function CalculosTab() {
  const [form, setForm] = useState<FormState>(FORM_EMPTY);
  const [loading, setLoading] = useState(false);
  const [result, setResult] = useState<CalculoResultado | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [tipoOpen, setTipoOpen] = useState(false);

  const patch = (p: Partial<FormState>) => setForm((f) => ({ ...f, ...p }));

  async function calcular() {
    setError(null);
    setResult(null);
    setLoading(true);
    try {
      const res = await realizarCalculo({
        tipo: form.tipo,
        valor_principal: parseFloat(form.valor.replace(",", ".")) || 0,
        data_base: form.dataBase,
        data_fim: form.dataFim,
        indice: form.indice,
        juros_mes: parseFloat(form.jurosMes.replace(",", ".")) || 0,
        honorarios: parseFloat(form.honorarios.replace(",", ".")) || 0,
        salario_mensal: parseFloat(form.salario.replace(",", ".")) || 0,
        meses_trabalhados: parseInt(form.meses) || 0,
        aviso_previo: form.avisoPrevio,
        falta_grave: form.faltaGrave,
        composto: form.composto,
      });
      setResult(res);
    } catch (e) {
      setError((e as Error).message);
    } finally {
      setLoading(false);
    }
  }

  const tipoAtual = TIPOS.find((t) => t.value === form.tipo)!;

  return (
    <div className="flex flex-col h-full overflow-hidden">
      {/* Header */}
      <div className="flex items-center gap-3 px-5 py-4 border-b border-white/5 shrink-0">
        <div className="grid place-items-center w-9 h-9 rounded-full bg-accent/15 text-accent-soft">
          <Calculator size={18} />
        </div>
        <div className="leading-tight">
          <div className="font-semibold">Cálculos Judiciais</div>
          <div className="text-xs text-muted">Correção · Liquidação · Trabalhista · Juros</div>
        </div>
        {result && (
          <button
            onClick={() => { setResult(null); setError(null); }}
            className="ml-auto grid place-items-center w-9 h-9 rounded-full bg-white/[0.05] hover:bg-white/[0.1] text-muted hover:text-white transition-colors"
          >
            <RotateCcw size={16} />
          </button>
        )}
      </div>

      <div className="flex-1 overflow-y-auto px-4 py-4">
        <AnimatePresence mode="wait">
          {!result ? (
            <motion.div key="form" initial={{ opacity: 0 }} animate={{ opacity: 1 }} exit={{ opacity: 0 }} className="space-y-4">
              {/* Tipo de cálculo */}
              <div className="space-y-1.5">
                <label className="text-xs text-muted font-medium uppercase tracking-wide">Tipo de Cálculo</label>
                <button
                  type="button"
                  onClick={() => setTipoOpen((o) => !o)}
                  className="w-full bg-white/[0.04] border border-white/[0.08] rounded-xl px-3.5 py-2.5 text-sm text-left flex items-center justify-between"
                >
                  <div>
                    <div className="text-sm">{tipoAtual.label}</div>
                    <div className="text-xs text-muted mt-0.5">{tipoAtual.desc}</div>
                  </div>
                  <ChevronDown size={16} className={`text-muted transition-transform ${tipoOpen ? "rotate-180" : ""}`} />
                </button>
                <AnimatePresence>
                  {tipoOpen && (
                    <motion.div
                      initial={{ opacity: 0, y: -8 }}
                      animate={{ opacity: 1, y: 0 }}
                      exit={{ opacity: 0, y: -8 }}
                      className="glass rounded-xl overflow-hidden"
                    >
                      {TIPOS.map((t) => (
                        <button
                          key={t.value}
                          type="button"
                          onClick={() => { patch({ tipo: t.value }); setTipoOpen(false); }}
                          className={`w-full text-left px-4 py-3 hover:bg-white/[0.04] transition-colors border-b border-white/[0.04] last:border-0 ${form.tipo === t.value ? "text-accent-soft" : ""}`}
                        >
                          <div className="text-sm">{t.label}</div>
                          <div className="text-xs text-muted">{t.desc}</div>
                        </button>
                      ))}
                    </motion.div>
                  )}
                </AnimatePresence>
              </div>

              {/* Campos comuns */}
              {form.tipo !== "trabalhista" && (
                <>
                  <Input label="Valor Principal (R$)" value={form.valor} onChange={(v) => patch({ valor: v })} placeholder="10000,00" />
                  <div className="grid grid-cols-2 gap-3">
                    <Input label="Data Base" value={form.dataBase} onChange={(v) => patch({ dataBase: v })} placeholder="01/01/2022" />
                    <Input label="Data Fim" value={form.dataFim} onChange={(v) => patch({ dataFim: v })} placeholder="31/12/2024" />
                  </div>
                </>
              )}

              {/* Correção / liquidação */}
              {(form.tipo === "correcao_monetaria" || form.tipo === "liquidacao") && (
                <div className="space-y-1.5">
                  <label className="text-xs text-muted font-medium uppercase tracking-wide">Índice de Correção</label>
                  <div className="flex gap-2">
                    {INDICES.map((idx) => (
                      <button
                        key={idx.value}
                        type="button"
                        onClick={() => patch({ indice: idx.value })}
                        className={`flex-1 py-2 rounded-lg text-sm font-medium transition-colors ${form.indice === idx.value ? "bg-accent text-white" : "bg-white/[0.04] text-muted hover:text-white"}`}
                      >
                        {idx.label}
                      </button>
                    ))}
                  </div>
                </div>
              )}

              {/* Liquidação */}
              {form.tipo === "liquidacao" && (
                <div className="grid grid-cols-2 gap-3">
                  <Input label="Juros ao mês (%)" value={form.jurosMes} onChange={(v) => patch({ jurosMes: v })} placeholder="1" />
                  <Input label="Honorários (%)" value={form.honorarios} onChange={(v) => patch({ honorarios: v })} placeholder="10" />
                </div>
              )}

              {/* Juros simples/compostos */}
              {form.tipo === "juros" && (
                <>
                  <Input label="Juros ao mês (%)" value={form.jurosMes} onChange={(v) => patch({ jurosMes: v })} placeholder="1" />
                  <Toggle label="Juros compostos" checked={form.composto} onChange={(v) => patch({ composto: v })} />
                </>
              )}

              {/* Trabalhista */}
              {form.tipo === "trabalhista" && (
                <>
                  <Input label="Salário Mensal (R$)" value={form.salario} onChange={(v) => patch({ salario: v })} placeholder="3000,00" />
                  <Input label="Meses Trabalhados" value={form.meses} onChange={(v) => patch({ meses: v })} placeholder="24" />
                  <div className="glass rounded-xl px-4 py-2 space-y-1">
                    <Toggle label="Aviso prévio" checked={form.avisoPrevio} onChange={(v) => patch({ avisoPrevio: v })} />
                    <Toggle label="Falta grave (sem multa FGTS)" checked={form.faltaGrave} onChange={(v) => patch({ faltaGrave: v })} />
                  </div>
                </>
              )}

              {error && (
                <div className="flex items-center gap-2 px-3.5 py-3 rounded-xl bg-red-500/10 border border-red-500/20 text-red-400 text-sm">
                  <AlertCircle size={16} className="shrink-0" />
                  {error}
                </div>
              )}

              <motion.button
                whileTap={{ scale: 0.97 }}
                onClick={calcular}
                disabled={loading}
                className="w-full py-3.5 rounded-2xl bg-accent text-white font-semibold text-sm shadow-glow disabled:opacity-50 transition-opacity"
              >
                {loading ? "Calculando…" : "Calcular"}
              </motion.button>
            </motion.div>
          ) : (
            <motion.div key="result" initial={{ opacity: 0, y: 16 }} animate={{ opacity: 1, y: 0 }} className="space-y-4">
              {/* Total destaque */}
              <div className="glass rounded-2xl p-5 text-center">
                <p className="text-xs text-muted uppercase tracking-wider mb-1">Total Geral</p>
                <p className="text-3xl font-bold text-accent-soft">{fmt(result.total_geral)}</p>
                <p className="text-xs text-muted mt-1">{fmtDate(result.data_calculo)}</p>
              </div>

              {/* Parcelas */}
              {result.parcelas.length > 0 && (
                <div className="glass rounded-2xl overflow-hidden">
                  <div className="px-4 py-2.5 border-b border-white/[0.06]">
                    <p className="text-xs text-muted uppercase tracking-wider">Composição</p>
                  </div>
                  {result.parcelas.map((p, i) => (
                    <div key={i} className="flex justify-between items-center px-4 py-3 border-b border-white/[0.04] last:border-0">
                      <span className="text-sm text-ink-100">{p.descricao}</span>
                      <span className="text-sm font-medium text-white">{fmt(p.valor)}</span>
                    </div>
                  ))}
                </div>
              )}

              {/* Fator / índice */}
              {result.fator_correcao && (
                <div className="flex gap-3">
                  <div className="flex-1 glass rounded-xl p-3 text-center">
                    <p className="text-xs text-muted">Fator</p>
                    <p className="text-lg font-semibold">{result.fator_correcao.toFixed(4)}</p>
                  </div>
                  {result.valor_corrigido && (
                    <div className="flex-1 glass rounded-xl p-3 text-center">
                      <p className="text-xs text-muted">Valor Corrigido</p>
                      <p className="text-lg font-semibold">{fmt(result.valor_corrigido)}</p>
                    </div>
                  )}
                </div>
              )}

              {/* Observações */}
              {result.observacoes?.length > 0 && (
                <div className="glass rounded-2xl p-4 space-y-1.5">
                  <p className="text-xs text-muted uppercase tracking-wider mb-2">Observações</p>
                  {result.observacoes.map((o, i) => (
                    <p key={i} className="text-sm text-muted leading-relaxed">• {o}</p>
                  ))}
                </div>
              )}
            </motion.div>
          )}
        </AnimatePresence>
      </div>
    </div>
  );
}
