"""
roses.datajud — Cliente da API Publica do DataJud (CNJ).

Caminho PRIMARIO do motor Roses para consulta por numero de processo (CNJ).
Fonte oficial e gratuita do Conselho Nacional de Justica (Resolucao 331/2020),
sem Cloudflare, sem captcha, sem proxy de terceiros.

Use o portal (scraper) apenas para o que o DataJud nao cobre:
busca por nome de parte / CPF / OAB e atualizacao em tempo real.
"""

from .client import (
    DataJudClient,
    DataJudError,
    resolve_alias,
    ALIAS_BY_CODE,
    ALIAS_BY_SIGLA,
)

__all__ = [
    "DataJudClient",
    "DataJudError",
    "resolve_alias",
    "ALIAS_BY_CODE",
    "ALIAS_BY_SIGLA",
]
