"""
roses/core/engine.py

Facade unico de consulta do motor Roses. Esconde de onde o dado vem:

  - Consulta por NUMERO (CNJ)            -> DataJud (oficial, sem Cloudflare)
  - Consulta por NOME / OAB / CPF / CNPJ -> Portal (scraper, com Cloudflare)

Sempre devolve um RosesResult (mesmo schema), venha de onde vier.
"""

from __future__ import annotations

import json
import logging
import subprocess
import sys
from pathlib import Path

ROSES_ROOT = Path(__file__).resolve().parent.parent
sys.path.insert(0, str(ROSES_ROOT))

from datajud.client import DataJudClient, DataJudError  # noqa: E402
from core.result_normalizer import portal_to_roses_result  # noqa: E402
from parsers.models import RosesResult  # noqa: E402

logger = logging.getLogger("roses.engine")

SCRAPER_PATH = ROSES_ROOT / "scrapers" / "pje_portal_scraper.py"


class RosesEngine:
    """Ponto de entrada de consulta. Roteia entre DataJud e portal."""

    def __init__(self, python_bin: str | None = None, datajud_client: DataJudClient | None = None):
        # Python que roda o scraper do portal (precisa de camoufox/playwright).
        self.python_bin = python_bin or sys.executable
        self.datajud = datajud_client or DataJudClient()

    # -- API publica -------------------------------------------------------

    def consultar(
        self,
        cnj: str | None = None,
        oab: str | None = None,
        uf: str | None = None,
        nome_parte: str | None = None,
        nome_advogado: str | None = None,
        cpf: str | None = None,
        cnpj: str | None = None,
        force_portal: bool = False,
        timeout: int = 120,
        max_pages: int = 200,
    ) -> RosesResult:
        is_name_based = any([oab, nome_parte, nome_advogado, cpf, cnpj])

        # Caminho primario: CNJ via DataJud.
        if cnj and not is_name_based and not force_portal:
            try:
                result = self.datajud.by_cnj(cnj)
                if result.status == "OK":
                    return result
                # NOT_FOUND/ERROR -> tenta o portal como fallback
                logger.info("DataJud sem resultado (%s); tentando portal.", result.status)
            except DataJudError as e:
                logger.warning("DataJud indisponivel (%s); tentando portal.", e)

        # Caminho do portal (nome/oab/doc, ou fallback de CNJ).
        return self._consultar_portal(
            cnj=cnj, oab=oab, uf=uf, nome_parte=nome_parte,
            nome_advogado=nome_advogado, cpf=cpf, cnpj=cnpj,
            timeout=timeout, max_pages=max_pages,
        )

    # -- portal ------------------------------------------------------------

    def _consultar_portal(self, *, cnj, oab, uf, nome_parte, nome_advogado,
                          cpf, cnpj, timeout, max_pages) -> RosesResult:
        query = {k: v for k, v in {
            "cnj": cnj, "oab": oab, "uf": uf, "nome_parte": nome_parte,
            "nome_advogado": nome_advogado, "cpf": cpf, "cnpj": cnpj,
        }.items() if v}

        cmd = [self.python_bin, str(SCRAPER_PATH), "--json-only", "--max-pages", str(max_pages)]
        if cnj:           cmd += ["--cnj", cnj]
        if nome_parte:    cmd += ["--nome-parte", nome_parte]
        if nome_advogado: cmd += ["--nome-advogado", nome_advogado]
        if cpf:           cmd += ["--cpf", cpf]
        if cnpj:          cmd += ["--cnpj", cnpj]
        if oab:           cmd += ["--oab", oab]
        if uf:            cmd += ["--uf", uf]

        try:
            proc = subprocess.run(cmd, capture_output=True, text=True, timeout=timeout + 15)
        except subprocess.TimeoutExpired:
            return RosesResult(status="TIMEOUT", message=f"Portal excedeu {timeout}s",
                               tribunal="N/A", tribunal_name="N/A", query=query)
        except FileNotFoundError:
            return RosesResult.error(
                f"Interpretador do portal nao encontrado: {self.python_bin}. "
                "Ative o venv do roses (veja requirements.txt).", query)

        if proc.returncode != 0 and not proc.stdout.strip():
            return RosesResult.error(
                (proc.stderr or "Erro desconhecido no scraper")[-400:], query)

        try:
            data = json.loads(proc.stdout)
        except json.JSONDecodeError:
            return RosesResult(status="UNKNOWN", message="Scraper retornou saida nao-JSON",
                               tribunal="N/A", tribunal_name="N/A", query=query)

        tribunal = data.get("court", "") or "N/A"
        return portal_to_roses_result(data, tribunal, tribunal, query)


# --------------------------------------------------------------------------
# CLI
# --------------------------------------------------------------------------

if __name__ == "__main__":
    import argparse

    ap = argparse.ArgumentParser(description="Roses — consulta unificada (DataJud + portal).")
    ap.add_argument("--cnj")
    ap.add_argument("--oab")
    ap.add_argument("--uf")
    ap.add_argument("--nome-parte", dest="nome_parte")
    ap.add_argument("--nome-advogado", dest="nome_advogado")
    ap.add_argument("--cpf")
    ap.add_argument("--cnpj")
    ap.add_argument("--force-portal", action="store_true")
    args = ap.parse_args()

    logging.basicConfig(level=logging.INFO, format="%(levelname)s %(name)s: %(message)s")
    engine = RosesEngine()
    res = engine.consultar(
        cnj=args.cnj, oab=args.oab, uf=args.uf,
        nome_parte=args.nome_parte, nome_advogado=args.nome_advogado,
        cpf=args.cpf, cnpj=args.cnpj, force_portal=args.force_portal,
    )
    print(res.to_json())
