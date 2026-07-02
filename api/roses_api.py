"""
roses/api/roses_api.py

Roses API — Interface HTTP via FastAPI.
Expõe endpoints para consulta de processos, status dos tribunais
e gerenciamento de perfis de hardware.
"""

import os
import subprocess
import sys
import time
from pathlib import Path
from typing import Any, Optional

try:
    from fastapi import FastAPI, HTTPException, Header, Depends
    from fastapi.middleware.cors import CORSMiddleware
    from pydantic import BaseModel
except ImportError:
    raise ImportError("FastAPI não instalado. Execute: pip install fastapi uvicorn")

# Importações internas (ajuste o path conforme necessário)
sys.path.insert(0, str(Path(__file__).parent.parent))
from core.court_router import list_all_courts, list_operational_courts, route_query
from engine.hardware_datalake import generate_hardware_fingerprint
from parsers.models import RosesResult
from datajud import DataJudClient, DataJudError

# Cliente DataJud (caminho primario para consulta por numero CNJ).
datajud_client = DataJudClient()

# --------------------------------------------------------------------------
# Configuração da API
# --------------------------------------------------------------------------

app = FastAPI(
    title="🌹 Roses API",
    description=(
        "Motor unificado de consulta a processos jurídicos brasileiros. "
        "Suporta busca por CNJ, nome de parte, OAB e CPF em múltiplos tribunais."
    ),
    version="1.0.0",
    docs_url="/docs",
    redoc_url="/redoc",
)

# CORS configuravel por env (ROSES_CORS_ORIGINS="https://app.seudominio.com,...").
# Default "*" para nao quebrar dev local; em producao, defina a env.
_cors = os.environ.get("ROSES_CORS_ORIGINS")
_allow_origins = [o.strip() for o in _cors.split(",")] if _cors else ["*"]

app.add_middleware(
    CORSMiddleware,
    allow_origins=_allow_origins,
    allow_credentials=True,
    allow_methods=["*"],
    allow_headers=["*"],
)

# Autenticacao opcional por chave de API. Se ROSES_API_KEY estiver definida,
# os endpoints sensiveis exigem o header X-API-Key. Se nao, fica aberto (dev).
ROSES_API_KEY = os.environ.get("ROSES_API_KEY")


async def require_api_key(x_api_key: Optional[str] = Header(default=None)):
    if ROSES_API_KEY and x_api_key != ROSES_API_KEY:
        raise HTTPException(status_code=401, detail="API key invalida ou ausente.")
    return True

# Scraper do portal — AGORA DENTRO do pacote roses (autossuficiente).
SCRAPER_PATH = Path(__file__).resolve().parent.parent / "scrapers" / "pje_portal_scraper.py"

# Python que executa o scraper. Por padrao usa o mesmo interpretador da API.
# Pode ser sobrescrito via env var ROSES_PYTHON (ex: apontar pra um venv).
_roses_python = os.environ.get("ROSES_PYTHON")
VENV_PYTHON = Path(_roses_python) if _roses_python else Path(sys.executable)


# --------------------------------------------------------------------------
# Schemas de request
# --------------------------------------------------------------------------

class SearchRequest(BaseModel):
    cnj: Optional[str] = None
    nome_parte: Optional[str] = None
    nome_advogado: Optional[str] = None
    oab: Optional[str] = None
    cpf: Optional[str] = None
    cnpj: Optional[str] = None
    uf: Optional[str] = None
    tribunal: Optional[str] = None  # Força um tribunal específico (ex: "TJRJ")
    headed: bool = False
    timeout: int = 70
    force_portal: bool = False  # Ignora o DataJud e vai direto pro portal (scraper)

    class Config:
        schema_extra = {
            "example": {
                "cnj": "0001234-56.2023.8.19.0001",
                "uf": "RJ",
            }
        }


class HardwareProfileRequest(BaseModel):
    engine: str = "gecko"
    profile_type: str = "mac"
    seed: Optional[int] = None


# --------------------------------------------------------------------------
# Endpoints
# --------------------------------------------------------------------------

@app.get("/", tags=["Status"])
async def root():
    """Endpoint raiz — retorna status e versão do motor."""
    return {
        "engine": "🌹 Roses",
        "version": "1.0.0",
        "status": "online",
        "tribunais_operacionais": len(list_operational_courts()),
    }


@app.get("/tribunais", tags=["Tribunais"])
async def get_tribunais(apenas_operacionais: bool = False):
    """Lista todos os tribunais registrados no motor Roses."""
    courts = list_operational_courts() if apenas_operacionais else list_all_courts()
    return {
        "total": len(courts),
        "tribunais": [
            {
                "codigo": c.code,
                "sigla": c.sigla,
                "nome": c.name,
                "url": c.url,
                "status": c.status,
                "engine": c.engine_hint,
                "turnstile": bool(c.turnstile_sitekey),
            }
            for c in courts
        ],
    }


@app.post("/search", tags=["Consulta"])
async def search(req: SearchRequest, _auth: bool = Depends(require_api_key)) -> dict[str, Any]:
    """
    Realiza uma consulta a processos jurídicos.
    
    - Identifica automaticamente o tribunal pelo CNJ ou UF
    - Usa emulação de navegador stealth (Gecko/Chromium)
    - Resolve desafios Turnstile/reCAPTCHA automaticamente
    """
    query = {
        "cnj": req.cnj,
        "uf": req.uf,
        "uf_oab": req.uf,
    }

    # Valida que ao menos um critério foi informado
    active = {k: v for k, v in {
        "cnj": req.cnj,
        "nome_parte": req.nome_parte,
        "nome_advogado": req.nome_advogado,
        "oab": req.oab,
        "cpf": req.cpf,
        "cnpj": req.cnpj,
    }.items() if v}

    if not active:
        raise HTTPException(
            status_code=400,
            detail="Nenhum critério de busca informado. Informe cnj, nome_parte, oab, cpf ou cnpj.",
        )

    # ----------------------------------------------------------------------
    # CAMINHO PRIMARIO — DataJud (oficial, sem Cloudflare)
    #
    # Consulta por NUMERO (CNJ) e resolvida pela API publica do CNJ: rapida,
    # gratuita, do proprio IP. So usamos o portal/scraper quando:
    #   - a busca e por nome/OAB/CPF/CNPJ (DataJud nao expoe isso), OU
    #   - force_portal=True, OU
    #   - o DataJud nao achou o processo (fallback).
    # ----------------------------------------------------------------------
    is_name_based = any([req.nome_parte, req.nome_advogado, req.oab, req.cpf, req.cnpj])
    if req.cnj and not is_name_based and not req.force_portal:
        start = time.time()
        try:
            result = datajud_client.by_cnj(req.cnj)
            data = result.to_dict()
            data["fonte"] = "datajud"
            data["elapsed_seconds"] = round(time.time() - start, 2)
            if result.status == "OK":
                return data
            # NOT_FOUND/INVALID/ERROR -> cai pro portal como fallback
        except DataJudError as e:
            # Erro de comunicacao com o DataJud: registra e tenta o portal.
            pass

    # Monta os argumentos para o scraper
    python_bin = str(VENV_PYTHON) if VENV_PYTHON.exists() else sys.executable
    cmd = [python_bin, str(SCRAPER_PATH), "--json-only"]

    if req.cnj:
        cmd += ["--cnj", req.cnj]
    if req.nome_parte:
        cmd += ["--nome-parte", req.nome_parte]
    if req.nome_advogado:
        cmd += ["--nome-advogado", req.nome_advogado]
    if req.oab:
        cmd += ["--oab", req.oab]
    if req.cpf:
        cmd += ["--cpf", req.cpf]
    if req.cnpj:
        cmd += ["--cnpj", req.cnpj]
    if req.uf:
        cmd += ["--uf", req.uf]
    if req.headed:
        cmd += ["--headed"]
    if req.timeout:
        cmd += ["--overall-timeout", str(req.timeout)]

    start = time.time()
    try:
        result = subprocess.run(
            cmd,
            capture_output=True,
            text=True,
            timeout=req.timeout + 10,
        )
        elapsed = round(time.time() - start, 2)

        if result.returncode != 0:
            return {
                "status": "ERROR",
                "message": result.stderr[-500:] if result.stderr else "Erro desconhecido no scraper",
                "elapsed_seconds": elapsed,
            }

        # Tenta parsear o JSON de saída
        import json
        try:
            data = json.loads(result.stdout)
            data["elapsed_seconds"] = elapsed
            return data
        except json.JSONDecodeError:
            return {
                "status": "UNKNOWN",
                "message": "Scraper retornou saída não-JSON",
                "raw": result.stdout[-1000:],
                "elapsed_seconds": elapsed,
            }

    except subprocess.TimeoutExpired:
        return {
            "status": "TIMEOUT",
            "message": f"Consulta excedeu o tempo limite de {req.timeout}s",
            "elapsed_seconds": round(time.time() - start, 2),
        }
    except Exception as e:
        raise HTTPException(status_code=500, detail=str(e))


@app.post("/hardware/generate", tags=["Hardware Datalake"])
async def generate_profile(req: HardwareProfileRequest):
    """
    Gera um perfil de hardware falso para evasão anti-bot.
    
    Retorna o perfil completo incluindo o script JS de injeção.
    """
    fp = generate_hardware_fingerprint(
        engine=req.engine,
        profile_type=req.profile_type,
        seed=req.seed,
    )
    result = fp.to_dict()
    result["js_init_script"] = fp.to_js_init_script()
    return result


@app.get("/hardware/profiles", tags=["Hardware Datalake"])
async def list_profiles():
    """Lista os perfis de hardware salvos no datalake."""
    datalake_dir = Path(__file__).parent.parent / "datalake"
    profiles = list(datalake_dir.glob("profile_*.json"))
    return {
        "total": len(profiles),
        "profiles": [p.stem for p in profiles],
    }


@app.post("/datajud/cnj", tags=["DataJud"])
async def datajud_cnj(cnj: str, _auth: bool = Depends(require_api_key)):
    """Consulta direta por numero CNJ na API oficial do DataJud (sem portal)."""
    try:
        result = datajud_client.by_cnj(cnj)
        data = result.to_dict()
        data["fonte"] = "datajud"
        return data
    except DataJudError as e:
        raise HTTPException(status_code=502, detail=str(e))


@app.get("/datajud/count", tags=["DataJud"])
async def datajud_count(tribunal: str, _auth: bool = Depends(require_api_key)):
    """Total de processos indexados de um tribunal (ex: TJPB, 15, api_publica_tjpb)."""
    try:
        return {"tribunal": tribunal.upper(), "total_indexado": datajud_client.count(tribunal)}
    except DataJudError as e:
        raise HTTPException(status_code=502, detail=str(e))


@app.get("/health", tags=["Status"])
async def health():
    """Health check do serviço."""
    scraper_ok = SCRAPER_PATH.exists()
    venv_ok = VENV_PYTHON.exists()
    return {
        "status": "healthy" if scraper_ok else "degraded",
        "scraper_found": scraper_ok,
        "venv_found": venv_ok,
        "scraper_path": str(SCRAPER_PATH),
    }


# --------------------------------------------------------------------------
# Entry point
# --------------------------------------------------------------------------

if __name__ == "__main__":
    import uvicorn
    uvicorn.run("roses_api:app", host="0.0.0.0", port=8001, reload=True)
