package main

// Modelos de saida do motor — espelham o models.py do lado Python,
// para que front e back falem o mesmo schema.

type Party struct {
	Tipo      string `json:"tipo"`
	Nome      string `json:"nome"`
	Documento string `json:"documento,omitempty"`
	OAB       string `json:"oab,omitempty"`
}

type Movement struct {
	Data      string `json:"data"`
	Descricao string `json:"descricao"`
	Orgao     string `json:"orgao,omitempty"`
}

type Process struct {
	Numero          string     `json:"numero"`
	Classe          string     `json:"classe"`
	Assunto         string     `json:"assunto"`
	Tribunal        string     `json:"tribunal"`
	OrgaoJulgador   string     `json:"orgao_julgador,omitempty"`
	DataDistribuicao string    `json:"data_distribuicao,omitempty"`
	Partes          []Party    `json:"partes"`
	Movimentacoes   []Movement `json:"movimentacoes"`
	URLProcesso     string     `json:"url_processo,omitempty"`
}

type Result struct {
	Status        string    `json:"status"`
	Message       string    `json:"message"`
	Tribunal      string    `json:"tribunal"`
	TribunalName  string    `json:"tribunal_name"`
	Query         any       `json:"query"`
	Total         int       `json:"total"`
	Processos     []Process `json:"processos"`
	Fonte         string    `json:"fonte,omitempty"`
	ElapsedSeconds float64  `json:"elapsed_seconds,omitempty"`
}

func errorResult(msg string, query any) Result {
	return Result{Status: "ERROR", Message: msg, Tribunal: "N/A", TribunalName: "N/A", Query: query}
}
