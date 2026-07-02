"""
pje_results_parser.py

Parser dos resultados da Consulta Publica do PJe (tabela RichFaces).
Extrai os dados estruturados de TODOS os processos de uma pagina de resultados
(consulta por CNJ, nome, documento ou OAB).

Projetado para ser RESILIENTE a mudancas de layout:
  - nao depende de ids fixos do JSF (que mudam: 'fPP:j_id225' etc.);
  - localiza a linha pela presenca de um numero CNJ;
  - cada campo tem extracao tolerante, com fallback, e nunca quebra a linha
    inteira se um pedaco faltar.

Funcao pura (so recebe HTML) => testavel offline contra HTML real salvo.
Sem dependencias externas.
"""

from __future__ import annotations

import html as _html
import re
from typing import Optional

# Numero CNJ no formato NNNNNNN-DD.AAAA.J.TT.OOOO
CNJ_RE = re.compile(r"\d{7}-\d{2}\.\d{4}\.\d\.\d{2}\.\d{4}")

# Linhas e celulas da tabela RichFaces de resultados.
ROW_RE = re.compile(r'<tr[^>]*class="[^"]*rich-table-row[^"]*"[^>]*>(.*?)</tr>', re.S | re.I)
CELL_RE = re.compile(r'<td[^>]*class="[^"]*rich-table-cell[^"]*"[^>]*>(.*?)</td>', re.S | re.I)

# Token 'ca=' do link de detalhe (DetalheProcessoConsultaPublica).
CA_RE = re.compile(r"ca=([0-9a-fA-F]+)")

# Data/hora da movimentacao: "(22/02/2026 18:33:05)" ou "(22/02/2026)".
DATE_RE = re.compile(r"\((\d{2}/\d{2}/\d{4}(?:\s+\d{2}:\d{2}:\d{2})?)\)")

# Total reportado pelo portal ("31 resultados encontrados").
TOTAL_RE = re.compile(r"(\d+)\s+resultados?\s+encontrados?", re.I)

# Base para montar a URL de detalhe a partir do token 'ca'.
DETALHE_BASE = (
    "https://consultapublica.tjpb.jus.br/pje/ConsultaPublica/"
    "DetalheProcessoConsultaPublica/listView.seam?ca={ca}"
)


def _clean_text(fragment: str) -> str:
    """Remove tags HTML, desescapa entidades e normaliza espacos."""
    if not fragment:
        return ""
    text = re.sub(r"<[^>]+>", " ", fragment)
    text = _html.unescape(text)
    return re.sub(r"\s+", " ", text).strip()


def _split_partes(parts_txt: str) -> tuple[str, str]:
    """Separa 'POLO ATIVO X POLO PASSIVO'. Tolerante a ' X ' e ' x '."""
    if not parts_txt:
        return "", ""
    m = re.split(r"\s+[Xx]\s+", parts_txt, maxsplit=1)
    if len(m) == 2:
        return m[0].strip(), m[1].strip()
    return parts_txt.strip(), ""


def parse_row(row_html: str, detalhe_base: str = DETALHE_BASE) -> Optional[dict]:
    """Extrai um processo de uma linha da tabela. Retorna None se nao for valida."""
    cells = CELL_RE.findall(row_html)
    if not cells:
        # fallback: alguns layouts usam <td> sem a classe rich-table-cell
        cells = re.findall(r"<td[^>]*>(.*?)</td>", row_html, re.S | re.I)
    if not cells:
        return None

    # A celula "principal" e a que contem um numero CNJ.
    info_cell = next((c for c in cells if CNJ_RE.search(c)), None)
    if info_cell is None:
        return None

    numero = CNJ_RE.search(info_cell).group(0)

    # Token e URL de detalhe (procura na linha inteira).
    ca_match = CA_RE.search(row_html)
    detalhe_ca = ca_match.group(1) if ca_match else None
    detalhe_url = detalhe_base.format(ca=detalhe_ca) if detalhe_ca else None

    # Classe: texto antes do primeiro <a> da celula principal.
    classe = _clean_text(re.split(r"<a\b", info_cell, maxsplit=1)[0])

    # Assunto: dentro do <b> ("ABBR numero - ASSUNTO") => parte apos " - ".
    assunto = ""
    b_match = re.search(r"<b[^>]*>(.*?)</b>", info_cell, re.S | re.I)
    if b_match:
        b_txt = _clean_text(b_match.group(1))
        if " - " in b_txt:
            assunto = b_txt.split(" - ", 1)[1].strip()

    # Partes: texto apos o ultimo </a> da celula principal.
    after_links = re.sub(r"(?s).*</a>", "", info_cell)
    partes_raw = _clean_text(after_links)
    polo_ativo, polo_passivo = _split_partes(partes_raw)

    # Ultima movimentacao: a celula que contem uma data.
    mov_cell = next((c for c in cells if DATE_RE.search(c)), "")
    mov_full = _clean_text(mov_cell)
    data_match = DATE_RE.search(mov_cell)
    data_mov = data_match.group(1) if data_match else None
    # Descricao sem o "(data)" no final.
    ultima_mov = re.sub(r"\s*\([^)]*\)\s*$", "", mov_full).strip() if mov_full else ""

    return {
        "numero": numero,
        "classe": classe,
        "assunto": assunto,
        "polo_ativo": polo_ativo,
        "polo_passivo": polo_passivo,
        "partes_raw": partes_raw,
        "ultima_movimentacao": ultima_mov,
        "data_ultima_movimentacao": data_mov,
        "detalhe_ca": detalhe_ca,
        "detalhe_url": detalhe_url,
    }


def parse_results_html(html_str: str, detalhe_base: str = DETALHE_BASE) -> dict:
    """
    Extrai todos os processos de uma pagina de resultados do PJe.

    Retorna:
      {
        "status": "OK" | "NOT_FOUND" | "UNKNOWN",
        "total_reported": int | None,   # "N resultados encontrados"
        "total_extracted": int,
        "processos": [ {...}, ... ],
      }
    """
    if not html_str:
        return {"status": "UNKNOWN", "total_reported": None, "total_extracted": 0, "processos": []}

    total_reported = None
    tm = TOTAL_RE.search(html_str)
    if tm:
        total_reported = int(tm.group(1))

    processos = []
    seen = set()
    for row in ROW_RE.findall(html_str):
        proc = parse_row(row, detalhe_base)
        if proc and proc["numero"] not in seen:
            seen.add(proc["numero"])
            processos.append(proc)

    lowered = html_str.lower()
    if processos:
        status = "OK"
    elif "nenhum resultado encontrado" in lowered or (total_reported == 0):
        status = "NOT_FOUND"
    else:
        status = "UNKNOWN"

    return {
        "status": status,
        "total_reported": total_reported,
        "total_extracted": len(processos),
        "processos": processos,
    }


# --------------------------------------------------------------------------
# Teste/CLI: roda o parser contra um HTML salvo.
#   python3 pje_results_parser.py caminho/para/resultado.html
# --------------------------------------------------------------------------

if __name__ == "__main__":
    import json
    import sys

    if len(sys.argv) < 2:
        print("uso: python3 pje_results_parser.py <arquivo.html>")
        sys.exit(1)

    with open(sys.argv[1], encoding="utf-8", errors="replace") as fh:
        data = parse_results_html(fh.read())

    print(f"status={data['status']}  reportados={data['total_reported']}  extraidos={data['total_extracted']}")
    print(json.dumps(data["processos"][:3], ensure_ascii=False, indent=2))
