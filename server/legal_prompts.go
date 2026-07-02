package main

// Estruturas e prompts para a IA Aterrada (Fase 2)

type CriticalClause struct {
	Clause    string `json:"clause"`
	RiskLevel string `json:"risk_level"` // baixo | medio | alto
}

type RiskPoint struct {
	Description    string `json:"description"`
	Recommendation string `json:"recommendation"`
}

type Obligation struct {
	Parties     string `json:"parties"`
	Description string `json:"description"`
	Deadline    string `json:"deadline"`
}

// AnaliseResult contem os 13 campos obrigatorios da analise estruturada de caso.
type AnaliseResult struct {
	Summary               string           `json:"summary"`
	KeyObservations       string           `json:"key_observations"`
	CriticalClauses       []CriticalClause `json:"critical_clauses"`
	RiskPoints            []RiskPoint      `json:"risk_points"`
	Obligations           []Obligation     `json:"obligations"`
	Timeline              []string         `json:"timeline"`
	ThesisPartyA          string           `json:"thesis_party_a"`
	ThesisPartyB          string           `json:"thesis_party_b"`
	LegalBasis            []string         `json:"legal_basis"`
	ProceduralRisks       string           `json:"procedural_risks"`
	ProbabilityAssessment string           `json:"probability_assessment"`
	LawyerRecommendations []string         `json:"lawyer_recommendations"`
	ClientObservations    string           `json:"client_observations"`
}

const AnaliseEstruturadaPrompt = `
Você deve realizar uma análise jurídica estruturada do documento fornecido e retornar EXCLUSIVAMENTE um objeto JSON válido, sem comentários adicionais fora do JSON, seguindo rigorosamente a estrutura abaixo:

{
  "summary": "Resumo conciso do caso/documento",
  "key_observations": "Principais observações jurídicas",
  "critical_clauses": [
    {
      "clause": "Identificação e teor da cláusula crítica",
      "risk_level": "baixo / medio / alto"
    }
  ],
  "risk_points": [
    {
      "description": "Descrição do ponto de risco",
      "recommendation": "Recomendação para mitigar o risco"
    }
  ],
  "obligations": [
    {
      "parties": "Partes envolvidas na obrigação",
      "description": "Teor da obrigação",
      "deadline": "Prazo para cumprimento"
    }
  ],
  "timeline": [
    "Fato/Evento 1 (com data, se houver)",
    "Fato/Evento 2 (com data, se houver)"
  ],
  "thesis_party_a": "Tese jurídica ou interesse da parte A (Autor/Notificante)",
  "thesis_party_b": "Tese jurídica ou interesse da parte B (Réu/Notificado)",
  "legal_basis": [
    "Citação real de Artigos de Lei (ex.: CC art. 186) ou Súmulas baseadas no contexto fornecido"
  ],
  "procedural_risks": "Riscos processuais (prescrição, decadência, custas, competência)",
  "probability_assessment": "Avaliação de probabilidade de êxito (provável, possível, remota) com justificativa técnica",
  "lawyer_recommendations": [
    "Recomendação prática 1 para o advogado",
    "Recomendação prática 2 para o advogado"
  ],
  "client_observations": "Observações ou explicações em linguagem acessível para apresentar ao cliente"
}

REGRAS CRÍTICAS DE VALIDAÇÃO:
1. Retorne APENAS o JSON. Não use blocos de código com "json" ou outros delimitadores.
2. Todos os 13 campos acima são obrigatórios. Se um campo não tiver informações no documento, preencha com "Não localizado no documento" ou uma lista vazia [], mas NUNCA omita o campo.
3. Em "legal_basis", liste APENAS fundamentos legais reais que existam na base legal fornecida no contexto ou que sejam inquestionavelmente reais. Não invente artigos.
4. Para planilhas (XLSX/CSV), procure por divergências matemáticas (somas incorretas, alíquotas erradas) e cite-as claramente no campo "key_observations" e "risk_points".
`
