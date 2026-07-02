import type {
  Result,
  SearchType,
  CalculoRequest,
  CalculoResultado,
  Vigilancia,
  VigilanciaAlerta,
  PortalLink,
  AuditoriaEntry,
} from "./types";

const API_KEY = import.meta.env.VITE_ROSES_API_KEY as string | undefined;

function authHeaders(): Record<string, string> {
  const h: Record<string, string> = { "Content-Type": "application/json" };
  if (API_KEY) h["X-API-Key"] = API_KEY;
  const token = localStorage.getItem("roses_token");
  if (token) h["Authorization"] = `Bearer ${token}`;
  return h;
}

async function apiCall<T>(path: string, init: RequestInit = {}): Promise<T> {
  const resp = await fetch(path, {
    ...init,
    headers: { ...authHeaders(), ...(init.headers as Record<string, string> | undefined) },
  });
  if (!resp.ok) {
    const body = await resp.json().catch(() => ({}));
    throw new Error((body as { error?: string }).error ?? `Erro ${resp.status}`);
  }
  return resp.json() as Promise<T>;
}

// ─── Consulta de processos ────────────────────────────────────────────────────

export async function consultar(
  tipo: SearchType,
  valor: string,
  uf?: string,
): Promise<Result> {
  return apiCall<Result>("/api/consulta", {
    method: "POST",
    body: JSON.stringify({ tipo, valor, uf: uf ?? "" }),
  });
}

export function detectIntent(text: string): { tipo: SearchType; valor: string; uf: string } {
  const t = text.trim();
  const digits = t.replace(/\D/g, "");
  if (digits.length === 20) return { tipo: "cnj", valor: t, uf: "" };

  const oabMatch = t.match(/oab\s*:?\s*(\d{2,6})(?:\s*[\/-]?\s*([A-Za-z]{2}))?/i);
  if (oabMatch) {
    return { tipo: "oab", valor: oabMatch[1], uf: (oabMatch[2] || "").toUpperCase() };
  }

  const ufMatch = t.match(/\b([A-Za-z]{2})\b\s*$/);
  return {
    tipo: "nome",
    valor: t.replace(/\b[A-Za-z]{2}\b\s*$/, "").trim() || t,
    uf: ufMatch ? ufMatch[1].toUpperCase() : "",
  };
}

// ─── Cálculos Judiciais ───────────────────────────────────────────────────────

export async function realizarCalculo(req: CalculoRequest): Promise<CalculoResultado> {
  // Mapeia camelCase do front para snake_case do backend
  const body = {
    tipo: req.tipo,
    valor_principal: req.valor_principal,
    data_base: req.data_base,
    data_fim: req.data_fim,
    indice: req.indice,
    juros_mes: req.juros_mes,
    honorarios: req.honorarios,
    salario_mensal: req.salario_mensal,
    meses_trabalhados: req.meses_trabalhados,
    aviso_previo: req.aviso_previo,
    falta_grave: req.falta_grave,
    composto: req.composto,
  };
  return apiCall<CalculoResultado>("/api/calculos", { method: "POST", body: JSON.stringify(body) });
}

export async function historicoCalculos(): Promise<CalculoResultado[]> {
  return apiCall<CalculoResultado[]>("/api/calculos/historico");
}

// ─── Vigília CPF/CNPJ ────────────────────────────────────────────────────────

export async function listarVigilancias(): Promise<Vigilancia[]> {
  return apiCall<Vigilancia[]>("/api/vigilancias");
}

export async function criarVigilancia(data: {
  documento: string;
  nome: string;
  tipo?: string;
  tribunais?: string[];
}): Promise<Vigilancia> {
  return apiCall<Vigilancia>("/api/vigilancias", { method: "POST", body: JSON.stringify(data) });
}

export async function excluirVigilancia(id: string): Promise<void> {
  await apiCall<void>(`/api/vigilancias/${id}`, { method: "DELETE" });
}

export async function listarAlertas(): Promise<VigilanciaAlerta[]> {
  return apiCall<VigilanciaAlerta[]>("/api/vigilancias/alertas");
}

// ─── Portal do Cliente ────────────────────────────────────────────────────────

export async function listarPortalLinks(): Promise<PortalLink[]> {
  return apiCall<PortalLink[]>("/api/portal/links");
}

export async function criarPortalLink(data: {
  numero_processo: string;
  nome_cliente?: string;
  dias_validade?: number;
}): Promise<{ id: string; token: string; url: string }> {
  return apiCall("/api/portal/links", { method: "POST", body: JSON.stringify(data) });
}

export async function revogarPortalLink(id: string): Promise<void> {
  await apiCall<void>(`/api/portal/links/${id}`, { method: "DELETE" });
}

// ─── Auditoria de Prazos ─────────────────────────────────────────────────────

export async function auditoriaProcesso(numero: string): Promise<AuditoriaEntry[]> {
  const encoded = encodeURIComponent(numero);
  return apiCall<AuditoriaEntry[]>(`/api/auditoria/${encoded}`);
}

export async function auditoriaDivergencias(): Promise<AuditoriaEntry[]> {
  return apiCall<AuditoriaEntry[]>("/api/auditoria/divergencias");
}

// ─── Helpers ─────────────────────────────────────────────────────────────────

export function fmt(v: number) {
  return v.toLocaleString("pt-BR", { style: "currency", currency: "BRL" });
}

export function fmtDate(d?: string) {
  if (!d) return "—";
  // Handles "DD/MM/YYYY" or ISO
  if (/^\d{2}\/\d{2}\/\d{4}$/.test(d)) return d;
  try {
    return new Date(d).toLocaleDateString("pt-BR");
  } catch {
    return d;
  }
}
