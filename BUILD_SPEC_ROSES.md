# Roses — Especificação de Build (handoff para agente de código)

> **Para quem lê:** este documento é uma ordem de serviço para um agente de IA (Claude) construir a próxima versão do Roses. Leia inteiro antes de escrever qualquer linha. Ele descreve o estado real do código, os erros a corrigir, e um roadmap em fases com critérios de aceitação. **Não reconstrua o que já existe** — o inventário na seção 3 diz o que já está pronto.

---

## 0. Como o agente deve trabalhar (contrato inegociável)

1. **Não quebre o que funciona.** DataJud, scraping de portal, motor de dias úteis (`prazos.go`), oportunidades e chat SSE já operam. Evolua por cima; não reescreva do zero.
2. **Trabalhe por fase, na ordem deste doc.** Cada fase tem critério de aceitação. Só avance quando a anterior passar.
3. **Toda fase entrega:** código + migração de banco (se houver) + testes + um trecho no `CHANGELOG` explicando o que mudou e como rodar.
4. **Nada de dado jurídico inventado.** Prazo, artigo de lei, súmula, número de processo e data só entram no produto se vierem de fonte oficial (DataJud/DJEN) ou de documento enviado pelo usuário. Se a IA não tem a fonte, ela diz que não tem. Isso é responsabilidade civil do advogado — trate como requisito de segurança, não de UX.
5. **Confirme contratos externos antes de codar contra eles.** Endpoints do DJEN/DataJud podem mudar; valide o JSON real com uma chamada antes de escrever o parser.
6. **LGPD desde o início.** Todo dado pessoal/sensível/judicial tem dono (usuário), base legal, e trilha de acesso. Nunca logue conteúdo de processo em texto plano.
7. **Pergunte só o que for bloqueante.** Decisões de arquitetura já estão tomadas na seção 2.

---

## 1. Visão: por que este trabalho existe

Roses é um motor de consulta de processos dos tribunais brasileiros com IA assistente (Ross AI). O objetivo desta fase é **deixar de ser "mais um consultor de processos" e virar a ferramenta em que o advogado brasileiro confia para não perder prazo e para trabalhar o caso**. Três apostas definem a diferenciação:

- **Confiabilidade de prazo** via canal oficial (DJEN), não estimativa por heurística.
- **IA que não alucina**, porque cita fonte real de lei/jurisprudência (grounding/RAG).
- **Alerta onde o advogado vive**: WhatsApp, e-mail e agenda — não só num dashboard que ele não abre.

Mercado-alvo: advogado solo e pequenos/médios escritórios. Concorrentes (Astrea, ADVBox, Projuris, Digesto, Jusbrasil Pro, Legal One) são fortes em gestão e fracos em (a) prazo oficial automático confiável e (b) IA aterrada. É aí que se ganha.

---

## 2. Decisões de arquitetura (já tomadas — não reabrir)

| Tema | Decisão |
|---|---|
| Backend | **Manter Go** (`server/`). Evoluir, não reescrever. |
| Front-end | Manter **Vite + React** (`web/`). |
| Persistência | **Migrar de arquivo JSON (`data/processos.json`) para PostgreSQL.** Recomendado Supabase (Postgres gerenciado + Auth + Storage) para não construir auth do zero. Driver Go: `pgx`. |
| Auth / multi-tenant | Todo dado passa a ter `user_id` (advogado) e opcional `escritorio_id`. Substituir a API key única (`ROSES_API_KEY`) por auth por usuário (JWT do Supabase validado no middleware Go). |
| IA | Manter fallback de modelos gratuitos como camada barata, **mas** introduzir um provider pago configurável para trabalho crítico (parecer, peça, análise 13 campos). Ver seção 6. |
| Scraping (`engine/`) | **Rebaixado a fallback de último recurso.** Núcleo de ingestão = DataJud + DJEN. Não investir mais em evasão de WAF como caminho principal. |

---

## 3. Estado atual do código (inventário — NÃO reconstruir)

Backend Go (`server/`), já funcionando:

- `main.go` — HTTP server, rotas, middleware CORS+API key, SSE. Rotas atuais: `/api/health`, `/api/consulta`, `/api/chat`, `/api/chat/stream`, `/api/oportunidades`, `/api/casos`, `/api/dashboard`, `/api/notificacoes`, `/api/transcribe`, `/api/brightdata`.
- `datajud.go` + `cnj.go` — integração **oficial DataJud** (busca por número CNJ, parser Elasticsearch, normalização de datas). Funciona.
- `portal.go` — ponte para o scraper Python (busca por OAB/nome, passa por Cloudflare).
- `prazos.go` — **motor de prazos por tipo de ato** (regras CPC, contagem em dias úteis com feriados fixos e recesso forense). Bem feito. **Problema: o gatilho é a movimentação raspada, não a intimação oficial.** É isto que a Fase 1 corrige.
- `oportunidades.go` — ranqueia processos por urgência/tempo parado/cliente recorrente.
- `casos.go` — lista "Meus casos" a partir do store.
- `dashboard.go` — dashboard + `/api/notificacoes` (Radar Jurídico: prazos, movimentações recentes, processos parados).
- `store.go` — **persistência em `data/processos.json`.** É o que a Fase 0 migra para Postgres.
- `ai.go` — Ross AI: OpenRouter (`:free`) + fallback Gemini, chat e streaming SSE. `rossSystemPrompt` embutido.
- `transcribe.go` — transcrição de áudio.
- `brightdata.go` — proxy de fetch.
- `models.go` — structs `Result`, `Process`, `Party`, `Movement`, `StoredProcess`.

Python (`datajud/`, `scrapers/`, `engine/`, `parsers/`): cliente DataJud, scraper PJe, datalake de hardware anti-detect, solver Turnstile, parsers.

Front (`web/src/`): React com fluxo de registro (OAB, PIN, credenciais, foto, sync), Onboarding, Chat, componentes (`ProcessCard`, `ProgressBar`, `SettingsPanel`).

**Buracos reais (confirmados no código):** sem banco relacional; sem auth multiusuário; sem captura de intimação oficial (DJEN); sem extração de PDF/DOCX/XLSX; sem análise estruturada de 13 campos; sem RAG/grounding jurídico; sem fila resiliente para 429; alertas não saem da tela.

---

## 4. Erros a corrigir (resumo priorizado)

1. **Prazo por heurística sobre movimentação raspada** → risco de perda de prazo (responsabilidade civil). Corrigir com DJEN (Fase 1).
2. **Persistência em JSON** → sem concorrência, sem multiusuário. Corrigir com Postgres (Fase 0).
3. **API key única / sem multi-tenant** → inviável para SaaS. Corrigir com auth por usuário (Fase 0).
4. **IA só em modelos free, sem grounding** → alucinação de lei/jurisprudência + queda por 429. Corrigir com provider pago + RAG + fila (Fase 2).
5. **Núcleo dependente de evasão de WAF** → frágil e juridicamente cinza. Mitigar rebaixando scraping a fallback (transversal).
6. **Alertas presos ao dashboard** → advogado não vê. Corrigir com entrega WhatsApp/e-mail/Calendar (Fase 3).
7. **LGPD ausente** → obrigatório e vende. Tratar como transversal desde a Fase 0.

---

## 5. FASE 0 — Fundação: Postgres + Auth multi-tenant

**Objetivo:** substituir `data/processos.json` por Postgres e introduzir usuário/escritório em todo dado. É pré-requisito de tudo (DJEN precisa saber de qual OAB/usuário puxar).

### 5.0 Configuração Supabase (reaproveitar o projeto já existente do Juris AI Hub)

O Roses vai usar o **mesmo projeto Supabase** já provisionado no `juris-ai-hub-main`. Valores confirmados no `.env` daquele repo:

```env
# Público (pode ficar no .env do builder / front) — projeto existente
SUPABASE_PROJECT_ID=utzoxnitkvhpeivdqgdu
SUPABASE_URL=https://utzoxnitkvhpeivdqgdu.supabase.co
SUPABASE_PUBLISHABLE_KEY=eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJpc3MiOiJzdXBhYmFzZSIsInJlZiI6InV0em94bml0a3ZocGVpdmRxZ2R1Iiwicm9sZSI6ImFub24iLCJpYXQiOjE3ODE3NDEwOTMsImV4cCI6MjA5NzMxNzA5M30.cDZ8-AHTfNeH5Dk-d2Eccc62Q_zf5q6xmbjnSLq-T9Y

# SEGREDOS — NÃO versionar este .md com estes valores. Copiar do arquivo
# juris-ai-hub-main/.env para o .env do backend Go (que está no .gitignore):
SUPABASE_SERVICE_ROLE_KEY=<copiar de juris-ai-hub-main/.env>   # bypassa RLS — acesso total
SUPABASE_JWT_SECRET=<copiar de juris-ai-hub-main/.env>          # valida os JWT no middleware Go

# Connection string direta do Postgres (para o pgx) NÃO está no .env do Juris Hub.
# Pegar no painel Supabase → Project Settings → Database → Connection string (URI):
DATABASE_URL=postgresql://postgres:<DB_PASSWORD>@db.utzoxnitkvhpeivdqgdu.supabase.co:5432/postgres
# (ou a string do pooler na porta 6543 para serverless)
```

**Instruções ao builder:**
- `SUPABASE_JWT_SECRET` é o segredo usado para **validar os JWT** dos usuários no middleware Go (HS256). É assim que o Go confia no login feito pelo Supabase Auth.
- A **service role key** só deve ser usada no backend Go, nunca exposta ao front. Bypassa RLS — use com parcimônia (migração/jobs), e prefira o fluxo com JWT do usuário nas rotas normais.
- **Reaproveitar o schema existente quando fizer sentido:** o Juris Hub já tem migrations em `juris-ai-hub-main/supabase/migrations/` e um `setup-completo.sql`. Antes de criar `usuarios`, verificar se já existe tabela equivalente (ex.: `profiles`/`auth.users`) para não duplicar identidade — o Supabase Auth já provê `auth.users`. As tabelas do Roses (`processos`, e depois `intimacoes`, `minutas`, etc.) devem referenciar `auth.users(id)` como `user_id`.
- **Não commitar** `SUPABASE_SERVICE_ROLE_KEY`, `SUPABASE_JWT_SECRET` nem `DATABASE_URL` neste `.md` nem em nenhum arquivo versionado.

**Escopo:**
- Provisionar Supabase (ou Postgres). Auth do Supabase para login por advogado; validar JWT no middleware Go (substitui a checagem de `ROSES_API_KEY` para rotas de usuário; manter API key só para rotas de serviço internas).
- Camada de acesso a dados em Go com `pgx`. Criar `server/db.go`.
- Migrar `store.go`: `saveProcessos`/`loadProcessos` passam a ler/gravar no banco, sempre com `user_id`. Manter a mesma assinatura pública para não quebrar `oportunidades.go`, `casos.go`, `dashboard.go`, `prazos.go`.
- Script de migração one-shot: importa o `processos.json` existente para o banco sob um usuário "legado".

**Schema mínimo (SQL):** identidade vem do `auth.users` do Supabase; o Roses só adiciona um `profile` com dados de OAB.
```sql
-- Perfil do advogado (estende auth.users; NÃO recriar identidade).
create table perfis (
  id uuid primary key references auth.users(id) on delete cascade,
  nome text,
  oab_numero text,
  oab_uf text,
  escritorio_id uuid,
  created_at timestamptz default now()
);

create table processos (
  id uuid primary key default gen_random_uuid(),
  user_id uuid not null references auth.users(id) on delete cascade,
  numero text not null,
  tribunal text,
  classe text,
  assunto text,
  orgao_julgador text,
  fonte text,                       -- datajud | portal | djen
  data_distribuicao date,
  partes jsonb default '[]',
  movimentacoes jsonb default '[]',
  last_seen timestamptz default now(),
  unique (user_id, numero)
);

create index on processos (user_id);
-- RLS ligado: cada usuário só enxerga o próprio dado.
```

**Critérios de aceitação:**
- App roda sem `processos.json`; todo o fluxo atual (consulta → salva → oportunidades/casos/dashboard) funciona idêntico, agora no banco.
- Dois usuários distintos não veem dados um do outro (testar RLS).
- Migração do JSON legado roda sem perda.

---

## 6. FASE 1 — Prazos oficiais via DJEN/Comunica  ⭐ prioridade máxima

**Objetivo:** o gatilho de prazo deixa de ser a movimentação raspada e passa a ser a **publicação oficial no Diário de Justiça Eletrônico Nacional (DJEN)**. Desde 16/05/2025 o CNJ determina que os prazos correm exclusivamente das publicações no DJEN / Domicílio Judicial Eletrônico. Existe **consulta pública por OAB**, sem captcha.

**Contrato da API (confirmar no Swagger antes de codar):**
- Base pública: `https://comunicaapi.pje.jus.br/api/v1/comunicacao`
- Swagger: `https://app.swaggerhub.com/apis-docs/cnj/pcp/1.0.0`
- Consulta (GET) por parâmetros como `numeroOab`, `ufOab`, `siglaTribunal`, `dataDisponibilizacaoInicio`, `dataDisponibilizacaoFim`, `nomeAdvogado`, `numeroProcesso`. **O agente deve fazer uma chamada real, inspecionar o JSON de resposta, e construir o parser em cima do formato observado — não presumir campos.**
- Consulta de leitura é aberta; **envio** de comunicação exige credencial no Corporativo do CNJ (não é necessário para o Roses, que só lê).

**Escopo:**
- Novo arquivo `server/djen.go`: cliente HTTP do Comunica, com retry e rate-limit gentil.
- Novo endpoint `POST /api/djen/sync` — recebe `{oab, uf, desde}` do usuário logado, busca publicações no DJEN, e persiste como `intimacoes`.
- Nova tabela `intimacoes`:
```sql
create table intimacoes (
  id uuid primary key default gen_random_uuid(),
  user_id uuid not null references auth.users(id),
  numero_processo text,
  tribunal text,
  tipo_comunicacao text,            -- intimação | citação | etc.
  texto text,                       -- inteiro teor
  data_disponibilizacao date,       -- data no DJEN
  data_publicacao date,             -- dia útil seguinte (marco legal)
  prazo_dias int,                   -- casado com prazos.go
  prazo_rotulo text,
  prazo_base text,                  -- fundamento (ex.: CPC art. 335)
  vencimento date,
  status text,                      -- vencido | hoje | urgente | emdia
  lido boolean default false,
  created_at timestamptz default now(),
  unique (user_id, numero_processo, data_disponibilizacao, tipo_comunicacao)
);
```
- **Regra do marco legal:** publicação = dia útil seguinte à disponibilização no DJEN; prazo começa no dia útil seguinte à publicação. Reaproveitar `adicionaDiasUteis`, `ehDiaUtil`, `feriadosFixos`, `ehRecessoForense` do `prazos.go` — **não duplicar**; extrair para funções compartilhadas se preciso.
- `prazos.go`: passar a calcular a partir de `intimacoes` (fonte oficial) quando existir; manter o cálculo por movimentação como *fallback* rotulado como "estimado (não oficial)". A UI deve distinguir claramente **prazo oficial** de **estimativa**.
- Agendar sync diário do DJEN por usuário (job/cron no backend).

**Critérios de aceitação:**
- Dado uma OAB real, `/api/djen/sync` traz publicações reais e gera prazos com `vencimento` correto pela regra de dias úteis.
- Prazo com origem DJEN aparece marcado como **oficial**; prazo por heurística aparece como **estimativa**. Nunca confundir os dois.
- Idempotência: rodar o sync duas vezes não duplica intimação (constraint `unique`).

---

## 7. FASE 2 — IA aterrada + extração de documentos + análise 13 campos

**Objetivo:** transformar o Ross AI de "chat que às vezes inventa" em assistente que (a) lê o documento real do advogado, (b) cita fonte jurídica verificável, (c) entrega análise estruturada padronizada, e (d) não cai quando o modelo free dá 429.

### 7.1 Extração real de documentos
- Extração no backend Go ou num worker: **PDF** (texto por página, ~até 500 págs), **DOCX**, **XLSX** → texto/estrutura. Se mais fácil no front, replicar a abordagem do Juris Hub (`pdfjs`, `mammoth`, `xlsx`); decisão do agente, mas o texto extraído tem de chegar ao backend para análise e ficar atrelado ao caso.
- Guardar documento em Storage (Supabase Storage), atrelado a `processo`/`caso`, com `user_id`.

### 7.2 Análise estruturada de 13 campos
A IA gera **JSON validado** com estes campos obrigatórios: `summary`, `key_observations`, `critical_clauses[]` (com nível de risco), `risk_points[]` (com recomendação), `obligations[]` (partes + prazos), `timeline[]`, `thesis_party_a`, `thesis_party_b`, `legal_basis[]` (artigos CTN/CLT/CC/CPC + súmulas STJ/STF), `procedural_risks` (prescrição/decadência/custas), `probability_assessment` (com justificativa), `lawyer_recommendations[]`, `client_observations`.
- Persistir em tabela `analises` (jsonb do resultado + `documento_id` + `user_id`).
- Validar o JSON no servidor (schema). Se a IA devolver fora do formato, reprocessar/reparar, não gravar lixo.

### 7.3 Grounding / anti-alucinação (o item mais importante da fase)
- **RAG sobre base jurídica real:** indexar CF, CPC, CLT, CTN, CC e súmulas STJ/STF (fontes públicas). Antes de afirmar "art. X" ou "súmula Y", a IA recupera o trecho e cita. `legal_basis` só aceita referência que exista na base indexada.
- Guardrail: se não há fonte, o campo diz "sem base localizada" em vez de inventar.

### 7.4 Modos e especialidades
- Especialidades: Tributário, Trabalhista, Cível, Penal, Empresarial, Consumidor, Família.
- Modos: chat de apoio, análise estruturada, parecer formal, peça processual, impugnação fiscal.
- Extrair os prompts para `server/legal_prompts.go` (ou arquivo de config), fora do código, versionáveis.

### 7.5 Auditoria de planilha
- Prompt/rotina específica para planilha fiscal/trabalhista/extrato: conferir somas de coluna, apurar decadência (CTN art. 173), sinalizar divergência de centavos. Entrada vinda do extrator XLSX.

### 7.6 Resiliência (mata o 429)
- Fila de jobs de análise no banco com status: `pending | running | waiting_retry | done | error`.
- Pipeline **map-reduce** para documento grande: se texto < ~75k chars, uma chamada; senão, resumir partes (map) e consolidar (reduce), salvando progresso parcial.
- Em 429/queda: marcar `waiting_retry`, worker/cron reprocessa a cada ~2 min; front acompanha progresso (polling ou realtime).
- Provider pago configurável para análise crítica (env `AI_CRITICAL_PROVIDER`/`AI_CRITICAL_MODEL`); free continua para chat casual.

**Critérios de aceitação:**
- Upload de um PDF de contestação de ~100 págs gera a análise de 13 campos, com `legal_basis` citando só artigos que existem na base indexada.
- Documento grande não estoura contexto (map-reduce) e sobrevive a um 429 sem recomeçar do zero.
- Nenhum campo jurídico é inventado: teste com um caso sem base → campo retorna "sem base localizada".

---

## 8. FASE 3 — Entrega de alertas onde o advogado vive

**Objetivo:** os alertas de prazo/movimentação (que `notificacoes` já calcula) saem da tela e chegam por WhatsApp, e-mail e agenda.

**Escopo:**
- **WhatsApp:** integração via API oficial do WhatsApp Cloud (Meta) ou provedor (Twilio/Z-API). Enviar resumo diário de prazos e alertas urgentes. Opt-in por usuário, com template aprovado.
- **E-mail:** digest diário (Resend/SMTP) com prazos de hoje/urgentes/vencidos.
- **Google Calendar:** criar/atualizar evento para cada prazo oficial (OAuth por usuário), com lembrete antecedente.
- Preferências por usuário (canais, horário do digest, antecedência). Nova tabela `preferencias_notificacao`.
- Reutilizar `computePrazos()` e `handleNotificacoes` como fonte; a Fase 3 é só **entrega**, não recalcula regra.

**Critérios de aceitação:**
- Um prazo urgente dispara mensagem no canal escolhido pelo usuário, uma única vez (sem spam/duplicидade).
- Evento de prazo aparece no Google Calendar do usuário com lembrete.
- Tudo é opt-in e respeita LGPD (consentimento registrado).

---

## 9. DIFERENCIAL DO PRODUTO — o que faz o Roses ser trocado por, não mais um

> Esta é a alma do produto. Tudo nas Fases 0–3 existe **para viabilizar isto**. O diferencial não é uma feature isolada: é a única coisa que exige ter DJEN + motor de prazo + IA + documentos **juntos** — que é exatamente a stack do Roses e que um concorrente teria de reconstruir inteira para copiar. Astrea/ADVBox têm alerta de prazo; Jusbrasil/Digesto têm IA; **ninguém solda a intimação oficial à ação e à minuta.**

### 9.1 FLAGSHIP — "Da intimação à minuta" (loop fechado)

**Dor:** hoje o advogado faz 5 passos manuais — recebe intimação, lê o inteiro teor, descobre o que o juízo pede, calcula o prazo, redige a peça. O passo do cálculo é onde ele perde prazo.

**O Roses transforma isso em um clique**, encadeando componentes que já existem:

```
DJEN (djen.go, Fase 1)            → captura a intimação oficial + inteiro teor
        │
        ▼
Classificador de ato (IA)         → o que o juízo determinou?
        │                           (contestar | manifestar | replicar | pagar |
        │                            recorrer | especificar provas | emendar ...)
        ▼
Motor de prazo (prazos.go)        → casa o ato com a regra CPC já mapeada →
        │                           vencimento oficial em dias úteis
        ▼
Redator (IA + RAG + docs do caso) → 1ª versão da peça exigida, aterrada em
                                     lei/jurisprudência real (nunca inventada)
```

**Experiência:** o advogado abre o app e vê um card acionável:
> *"Intimação no proc. 0001234-56.2024.8.19.0001 — o juízo determinou **réplica** (CPC art. 350). Vence em **12 dias úteis (23/07/2026)**. **Minuta pronta para revisão.**"*
> Botões: `Ver inteiro teor` · `Abrir minuta` · `Ajustar prazo` · `Marcar como protocolado`

**Contrato de dados — o classificador de ato.** A IA recebe o `texto` da intimação e devolve JSON validado:
```json
{
  "tipo_ato": "replica",
  "ato_legivel": "Réplica à contestação",
  "prazo_rotulo": "Réplica",
  "prazo_base": "CPC art. 350",
  "prazo_dias": 15,
  "exige_peca": true,
  "peca_sugerida": "replica",
  "confianca": 0.0,
  "trecho_gatilho": "…intime-se a parte autora para réplica…",
  "observacao_ia": "..."
}
```
- `tipo_ato` deve mapear 1:1 com as `keywords`/`rotulo` já existentes em `prazoRules` (`prazos.go`). **Não criar uma segunda taxonomia** — a IA classifica, mas o prazo em dias/base vem sempre de `prazoRules` (fonte da verdade legal). Se a IA e a regra divergirem no `prazo_dias`, **vale a regra** e registra-se o conflito.
- `confianca < 0.75` → não gera minuta automática; marca "requer confirmação do advogado".
- `trecho_gatilho` é obrigatório: a UI destaca no inteiro teor o trecho que embasou a classificação (transparência = confiança do usuário).

**O redator.** Usa o modo "peça processual" (Fase 2), o RAG jurídico e os documentos do caso. Gera rascunho **sempre rotulado como "minuta — revisar antes de protocolar"**, com as citações legais linkadas à base. Nunca cita acórdão/artigo que não exista no índice.

**Tabela `minutas`:**
```sql
create table minutas (
  id uuid primary key default gen_random_uuid(),
  user_id uuid not null references auth.users(id),
  intimacao_id uuid references intimacoes(id),
  numero_processo text,
  tipo_peca text,                 -- replica | contestacao | manifestacao | ...
  conteudo text,                  -- rascunho gerado
  status text default 'rascunho', -- rascunho | revisada | protocolada
  fontes_citadas jsonb default '[]',
  created_at timestamptz default now()
);
```

**Critérios de aceitação:**
- Uma intimação de réplica capturada no DJEN gera automaticamente: classificação correta, prazo oficial em dias úteis, e uma minuta de réplica com base legal citada da base indexada.
- `confianca` baixa ou ato desconhecido → NÃO gera minuta; pede confirmação. Nunca inventa o tipo de ato.
- Em conflito IA vs. `prazoRules`, prevalece a regra e o conflito fica registrado/auditável.
- Toda minuta sai marcada como rascunho a revisar; nenhuma é apresentada como definitiva.

### 9.2 Blindagem de prazo com prova (dupla-fonte + trilha auditável)

Você tem **duas fontes** do mesmo prazo: a intimação oficial (DJEN) e a movimentação (DataJud/portal). Cruze-as:
- **Divergência de data** entre DJEN e movimentação → alerta vermelho.
- **Movimentação relevante sem intimação DJEN correspondente** → alerta *"possível publicação não capturada, confira manualmente"*.
- **Trilha auditável** por prazo: data de disponibilização, data de publicação (dia útil seguinte), regra aplicada, base legal, dias úteis contados, feriados/recesso considerados — tudo persistido e exportável em PDF.

Isso transforma "alerta de prazo" em **blindagem defensável** — o argumento que faz um escritório sério confiar e pagar. É a diferença entre "app que avisa" e "seguro contra perda de prazo".

**Tabela `prazo_auditoria`** (append-only): `intimacao_id`, `fonte`, `data_disponibilizacao`, `data_publicacao`, `regra`, `base_legal`, `dias_uteis`, `feriados_considerados jsonb`, `divergencia bool`, `detalhe_divergencia`, `created_at`.

**Critérios de aceitação:** dado um prazo, o usuário consegue abrir a "prova do cálculo" e exportá-la; divergência entre fontes gera alerta visível; movimentação órfã (sem intimação) é sinalizada.

### 9.3 Vigília de CPF/CNPJ multi-tribunal (radar de parte)

Reusa o roteador de tribunais + busca por parte + DataJud. O advogado cadastra uma pessoa/empresa (parte adversa, devedor, cliente) e o Roses monitora: **novos processos** em qualquer tribunal, **novas execuções**, indícios de **patrimônio para penhora**. Ouro em execução e due diligence — pouca gente entrega bem no Brasil. Sem construir ingestão nova.

**Tabela `vigilancias`:** `user_id`, `documento` (CPF/CNPJ), `nome`, `tribunais[]`, `ultima_verificacao`, `ativo`. Job diário compara resultado novo com o anterior e notifica (Fase 3) só o que mudou.

**Critérios de aceitação:** cadastrar um CNPJ e, ao surgir processo novo, o usuário é notificado uma única vez com o processo novo destacado.

### 9.4 Ross agêntico sobre a carteira

O `oportunidades.go` já ranqueia a carteira. Deixe o advogado **conversar com ela** e o Ross **agir**: *"quais prazos vencem esta semana e me faça as minutas"*, *"clientes com processo parado há mais de 6 meses"*, *"me prepare para a audiência de amanhã"*. O Ross ganha ferramentas (tools) que chamam os endpoints internos (`/api/prazos`, `/api/oportunidades`, gerar minuta, criar vigília) — deixa de ser chatbot genérico e vira assistente que opera nos processos reais do usuário.

**Critérios de aceitação:** o Ross responde "prazos desta semana" a partir de dados reais do banco (não texto livre) e consegue disparar a geração de minuta para um prazo específico via tool call.

### 9.5 Apoio (Fase 4 tardia — só depois do flagship)
- **Cálculos judiciais** (revisional, liquidação, trabalhista, atualização monetária) com auditoria de planilha — alta retenção.
- **Portal do cliente:** andamento em linguagem leiga; reduz churn e o "doutor, e o meu processo?".

### Prioridade de construção do diferencial
O **9.1 (flagship)** e **9.2 (blindagem)** são o coração — construir assim que Fase 1 (DJEN) e Fase 2 (IA/RAG) existirem. 9.3 e 9.4 vêm em seguida. 9.5 por último.

---

## 10. Transversais (valem para todas as fases)

- **LGPD:** base legal por dado, retenção configurável, trilha de acesso, criptografia em repouso para conteúdo de processo, e nunca logar inteiro teor em texto plano. Página de política + exportação/exclusão a pedido do titular.
- **Observabilidade:** logs estruturados (sem PII), métricas de sync DJEN, taxa de 429, latência de IA.
- **Testes:** unidade para regras de prazo (dias úteis/feriados/recesso), parser DJEN, validação do JSON de 13 campos; integração para o fluxo consulta→prazo→alerta.
- **Config por env:** DataJud key, DJEN base, provider/modelo de IA, canais de notificação, credenciais de banco. Nada hardcoded.

---

## 11. Ordem de execução recomendada ao agente

1. **Fase 0** (banco + auth) — base de tudo.
2. **Fase 1** (DJEN) — maior alavanca de valor; depende da Fase 0.
3. **Fase 2** (IA aterrada + docs + 13 campos + fila).
4. **Diferencial 9.1 + 9.2** (flagship "intimação → minuta" + blindagem de prazo) — **o motivo do produto existir.** Construir assim que 1 e 2 estiverem de pé; não deixar para o fim.
5. **Fase 3** (entrega de alertas) — depende de 0/1; potencializa o flagship (a minuta pronta chega no WhatsApp).
6. **Diferencial 9.3 + 9.4** (vigília + Ross agêntico).
7. **9.5** (cálculos, portal do cliente).

Ao terminar cada fase: rodar testes, atualizar `CHANGELOG` e `RUN.md`, e listar o que mudou + como validar manualmente.

---

## 12. Perguntas que o agente pode precisar confirmar com o dono

- Provedor de WhatsApp (Meta Cloud API direto vs. Twilio/Z-API) e se já há número/business verificado.
- Qual provider pago de IA para trabalho crítico (e orçamento) — define `AI_CRITICAL_*`.
- ~~Supabase vs. Postgres self-hosted~~ → **DECIDIDO: Supabase** (Postgres gerenciado + Auth + Storage). Não reabrir.
- Fontes exatas dos textos legais para indexar no RAG (LexML, Planalto, sites dos tribunais).

Se nenhuma resposta vier, adotar o default recomendado em cada seção e seguir, deixando `TODO` marcado.
