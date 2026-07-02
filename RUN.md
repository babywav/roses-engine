# Roses — Stack completo (Go + React)

Arquitetura:

```
Frontend (React+Vite+TS)  ──HTTP /api──▶  Backend (Go)  ──┬─▶ DataJud (HTTP oficial, em Go)
  web/                                     server/         └─▶ Portal Python (scrapers/) p/ OAB/nome
```

- **Número (CNJ)** → o Go consulta a API do DataJud nativamente (sem Cloudflare).
- **OAB / nome** → o Go chama o scraper Python (`scrapers/pje_portal_scraper.py`).

---

## Front ativo: dashboard Astrea (`public/index.html`)

O frontend em uso é o **dashboard estático** em `public/` (HTML/CSS/JS puro — **não precisa de Node/npm**). O Go serve ele direto. O projeto React em `web/` fica como alternativa.

```bash
# 1 terminal só:
cd "/Users/huncho/Documents/astra/roses/server"
export ROSES_PYTHON="/Users/huncho/Documents/astra/roses/.venv/bin/python"   # p/ OAB/nome (portal)
go run .
```

Abra **http://localhost:8080** → clique em **Astrea** (barra inferior) e mande um número, OAB ou nome.

- Consulta por **número (CNJ)** funciona só com o Go (DataJud, sem venv).
- Consulta por **OAB/nome** precisa do `ROSES_PYTHON` apontando pro venv do portal.

> Alternativa React (`web/`): `cd web && npm install && npm run dev` (porta 5173, proxy /api → 8080).

---

## Produção (1 binário servindo tudo)

```bash
# 1. build do front
cd roses/web && npm install && npm run build      # gera web/dist

# 2. build do backend
cd ../server && go build -o roses-server .

# 3. rodar (serve a API + o front de web/dist)
./roses-server
# abra http://localhost:8080
```

---

## Variáveis de ambiente

| Variável             | Onde   | Padrão                         | Função                                            |
|----------------------|--------|--------------------------------|---------------------------------------------------|
| `ROSES_PORT`         | Go     | `8080`                         | Porta do backend                                  |
| `ROSES_WEB_DIR`      | Go     | `../web/dist`                  | Pasta do build do front                           |
| `ROSES_SCRAPER`      | Go     | `../scrapers/pje_portal_scraper.py` | Caminho do scraper do portal                 |
| `ROSES_PYTHON`       | Go     | `python3`                      | Interpretador do portal (aponte pro venv)         |
| `ROSES_API_KEY`      | Go     | (vazio)                        | Se definido, exige header `X-API-Key`             |
| `ROSES_CORS_ORIGINS` | Go     | `*`                            | Origens liberadas no CORS                         |
| `DATAJUD_API_KEY`    | Go     | chave pública vigente          | Sobrescreve a chave do DataJud                    |
| `VITE_ROSES_API_KEY` | Front  | (vazio)                        | Manda `X-API-Key` nas chamadas (se a API exigir)  |
| `DATABASE_URL`       | Go     | (vazio)                        | URI de conexão com o PostgreSQL do Supabase        |
| `SUPABASE_JWT_SECRET`| Go     | (vazio)                        | Segredo JWT do Supabase para validar tokens HS256  |
| `SUPABASE_SERVICE_ROLE_KEY` | Go | (vazio)                      | Chave de acesso completo (service role) do Supabase|

---

## Migração de Dados Legados (JSON -> Postgres)

Se você possuir dados legados no arquivo `data/processos.json` e deseja importá-los para o banco relacional, execute o script de migração:

```bash
cd server/cmd/migrate_legacy
go run .
```

O script criará o usuário e perfil legado no banco de dados e migrará todos os processos, renomeando o arquivo `processos.json` original para `processos.json.bak` ao final para evitar duplicações.


---

## Telas

1. **Onboarding** — apresentação + "Iniciar consulta".
2. **Chat** — digite número, OAB ou nome; o copiloto detecta o tipo, consulta e responde com cards de processo (número, classe, partes, última movimentação, link).
3. **Configurações da Consulta** (ícone de sliders no topo) — fonte (Auto/DataJud/Portal), formato, movimentações, UF padrão.

Fontes: **Geist** (UI) + **Geist Mono** (números/CNJ). Animações com **Framer Motion**.
