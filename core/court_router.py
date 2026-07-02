"""
roses/core/court_router.py

Roteador de tribunais — identifica o tribunal alvo com base no número CNJ
ou na sigla UF informada. Ponto central de decisão do motor Roses.
"""

import re
from dataclasses import dataclass
from typing import Optional


@dataclass
class CourtInfo:
    code: str               # Código CNJ do tribunal (ex: "19")
    sigla: str              # Sigla do tribunal (ex: "TJRJ")
    name: str               # Nome completo
    url: str                # URL do portal de consulta pública
    turnstile_sitekey: Optional[str]  # Sitekey Turnstile (se houver)
    recaptcha_sitekey: Optional[str]  # Sitekey reCAPTCHA (se houver)
    engine_hint: str        # Motor recomendado: "gecko" | "chromium"
    status: str             # "operational" | "beta" | "roadmap"


# --------------------------------------------------------------------------
# Registro de tribunais suportados
# --------------------------------------------------------------------------

COURT_REGISTRY: dict[str, CourtInfo] = {
    "15": CourtInfo(
        code="15", sigla="TJPB",
        name="Tribunal de Justiça da Paraíba",
        url="https://consultapublica.tjpb.jus.br/pje/ConsultaPublica/listView.seam",
        turnstile_sitekey="0x4AAAAAAAAjq6WYeRDKmebM",
        recaptcha_sitekey=None,
        engine_hint="gecko",
        status="operational",
    ),
    "19": CourtInfo(
        code="19", sigla="TJRJ",
        name="Tribunal de Justiça do Rio de Janeiro",
        url="https://tjrj.pje.jus.br/pje/ConsultaPublica/listView.seam",
        turnstile_sitekey=None,
        recaptcha_sitekey=None,  # reCAPTCHA v2 detectado dinamicamente
        engine_hint="gecko",
        status="operational",
    ),
    "17": CourtInfo(
        code="17", sigla="TJPE",
        name="Tribunal de Justiça de Pernambuco",
        url="https://pje.tjpe.jus.br/pje/ConsultaPublica/listView.seam",
        turnstile_sitekey=None,
        recaptcha_sitekey=None,
        engine_hint="gecko",
        status="operational",
    ),
    "13": CourtInfo(
        code="13", sigla="TJMG",
        name="Tribunal de Justiça de Minas Gerais",
        url="https://pje.tjmg.jus.br/pje/ConsultaPublica/listView.seam",
        turnstile_sitekey=None,
        recaptcha_sitekey=None,
        engine_hint="gecko",
        status="beta",
    ),
    "05": CourtInfo(
        code="05", sigla="TJBA",
        name="Tribunal de Justiça da Bahia",
        url="https://pje.tjba.jus.br/pje/ConsultaPublica/listView.seam",
        turnstile_sitekey=None,
        recaptcha_sitekey=None,
        engine_hint="gecko",
        status="beta",
    ),
    "08": CourtInfo(
        code="08", sigla="TJSP",
        name="Tribunal de Justiça de São Paulo",
        url="https://esaj.tjsp.jus.br/cpopg/open.do",
        turnstile_sitekey=None,
        recaptcha_sitekey=None,
        engine_hint="chromium",
        status="roadmap",
    ),
}

# Mapeamento de UF para código de tribunal
UF_TO_CODE: dict[str, str] = {
    "PB": "15",
    "RJ": "19",
    "PE": "17",
    "MG": "13",
    "BA": "05",
    "SP": "08",
}


# --------------------------------------------------------------------------
# Funções de roteamento
# --------------------------------------------------------------------------

def parse_cnj_digits(cnj: str) -> Optional[str]:
    """
    Extrai o código do tribunal dos dígitos 14-16 do número CNJ limpo.
    
    Formato CNJ: NNNNNNN-DD.AAAA.J.TT.OOOO (20 dígitos sem pontuação)
    Posição 14-16 = código TT do tribunal.
    
    Args:
        cnj: Número CNJ com ou sem pontuação.
    
    Returns:
        String de 2 dígitos do tribunal, ou None se inválido.
    """
    digits = re.sub(r"\D", "", cnj)
    if len(digits) == 20:
        return digits[14:16]
    return None


def route_query(query: dict) -> CourtInfo:
    """
    Resolve o tribunal alvo a partir dos parâmetros de busca.
    
    Prioridade de resolução:
    1. Número CNJ (extrai código do tribunal pelos dígitos 14-16)
    2. UF informada (--uf ou uf_oab)
    3. Fallback: TJPB (tribunal padrão do sistema)
    
    Args:
        query: Dicionário com os critérios de busca.
               Chaves suportadas: cnj, uf, uf_oab
    
    Returns:
        CourtInfo do tribunal identificado.
    """
    # 1. Tenta pelo número CNJ
    cnj = query.get("cnj")
    if cnj:
        code = parse_cnj_digits(cnj)
        if code and code in COURT_REGISTRY:
            court = COURT_REGISTRY[code]
            return court

    # 2. Tenta pelo UF
    uf = (query.get("uf") or query.get("uf_oab") or "").strip().upper()
    if uf and uf in UF_TO_CODE:
        code = UF_TO_CODE[uf]
        if code in COURT_REGISTRY:
            return COURT_REGISTRY[code]

    # 3. Fallback padrão
    return COURT_REGISTRY["15"]  # TJPB


def get_court_by_sigla(sigla: str) -> Optional[CourtInfo]:
    """Busca um tribunal pela sigla (ex: 'TJRJ')."""
    sigla = sigla.strip().upper()
    for court in COURT_REGISTRY.values():
        if court.sigla == sigla:
            return court
    return None


def list_operational_courts() -> list[CourtInfo]:
    """Retorna apenas os tribunais com status 'operational'."""
    return [c for c in COURT_REGISTRY.values() if c.status == "operational"]


def list_all_courts() -> list[CourtInfo]:
    """Retorna todos os tribunais registrados."""
    return list(COURT_REGISTRY.values())


# --------------------------------------------------------------------------
# CLI (uso direto)
# --------------------------------------------------------------------------

if __name__ == "__main__":
    import sys

    print("🌹 Roses — Court Router")
    print("=" * 50)

    # Testa com um CNJ de RJ
    test_cases = [
        {"cnj": "0001234-56.2023.8.19.0001"},  # TJRJ
        {"cnj": "0001234-56.2023.8.15.0001"},  # TJPB
        {"uf": "PE"},                           # TJPE via UF
        {"uf_oab": "MG"},                       # TJMG via UF OAB
        {},                                     # Fallback
    ]

    for query in test_cases:
        court = route_query(query)
        label = next(iter(query.items()), ("N/A", "N/A"))
        print(f"  Input: {label} → {court.sigla} ({court.name})")
        print(f"         URL: {court.url}")
        print(f"         Status: {court.status} | Engine: {court.engine_hint}")
        print()

    print("Tribunais operacionais:")
    for c in list_operational_courts():
        print(f"  [{c.code}] {c.sigla} — {c.name}")
