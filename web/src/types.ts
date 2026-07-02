// Espelha o schema do backend Go (models.go) e do motor Python (models.py).

export interface Party {
  tipo: string;
  nome: string;
  documento?: string;
  oab?: string;
}

export interface Movement {
  data: string;
  descricao: string;
  orgao?: string;
}

export interface Process {
  numero: string;
  classe: string;
  assunto: string;
  tribunal: string;
  orgao_julgador?: string;
  data_distribuicao?: string;
  partes: Party[];
  movimentacoes: Movement[];
  url_processo?: string;
}

export interface Result {
  status: string;
  message: string;
  tribunal: string;
  tribunal_name: string;
  query: unknown;
  total: number;
  processos: Process[];
  fonte?: string;
  elapsed_seconds?: number;
}

export type SearchType = "cnj" | "oab" | "nome" | "advogado";

export interface QuerySettings {
  fonte: "auto" | "datajud" | "portal";
  saida: "resumo" | "lista" | "completo";
  incluirMovimentacoes: boolean;
  ufPadrao: string;
}

export interface RegisterData {
  nome: string;
  email: string;
  senha: string;
  senhaConfirm: string;
  pin: string;
  foto: string; // dataURL (base64) da foto de perfil
  oab: string;
  oabEstado: string;
  sincronizar: boolean;
}

// ─── Cálculos Judiciais ───────────────────────────────────────────────────────

export type TipoCalculo = "correcao_monetaria" | "liquidacao" | "trabalhista" | "juros";
export type IndiceCorrecao = "selic" | "inpc" | "tr";

export interface CalculoRequest {
  tipo: TipoCalculo;
  valor_principal?: number;
  data_base?: string;
  data_fim?: string;
  indice?: IndiceCorrecao;
  juros_mes?: number;
  honorarios?: number;
  salario_mensal?: number;
  meses_trabalhados?: number;
  aviso_previo?: boolean;
  falta_grave?: boolean;
  composto?: boolean;
}

export interface Parcela {
  descricao: string;
  valor: number;
}

export interface CalculoResultado {
  tipo: string;
  valor_principal: number;
  valor_corrigido?: number;
  fator_correcao?: number;
  juros_montante?: number;
  honorarios?: number;
  total_geral: number;
  parcelas: Parcela[];
  observacoes: string[];
  data_calculo: string;
}

// ─── Vigília CPF/CNPJ ────────────────────────────────────────────────────────

export interface Vigilancia {
  id: string;
  documento: string;
  nome: string;
  tipo: string;
  tribunais: string[];
  ultima_verificacao?: string;
  ativo: boolean;
  created_at: string;
}

export interface VigilanciaAlerta {
  id: string;
  vigilancia_id: string;
  tipo_alerta: string;
  numero_processo?: string;
  titulo: string;
  detalhe?: string;
  lido: boolean;
  created_at: string;
}

// ─── Portal do Cliente ────────────────────────────────────────────────────────

export interface PortalLink {
  id: string;
  numero_processo: string;
  nome_cliente?: string;
  token: string;
  url: string;
  ativo: boolean;
  acessos: number;
  ultimo_acesso?: string;
  expira_em?: string;
  created_at: string;
}

// ─── Auditoria de Prazos ─────────────────────────────────────────────────────

export interface AuditoriaEntry {
  id: string;
  numero_processo: string;
  fonte: string;
  data_disponibilizacao?: string;
  data_publicacao?: string;
  regra?: string;
  base_legal?: string;
  dias_uteis?: number;
  vencimento?: string;
  feriados_considerados: string[];
  divergencia: boolean;
  detalhe_divergencia?: string;
  created_at: string;
}
