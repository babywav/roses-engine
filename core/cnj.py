"""
roses/core/cnj.py

Validacao e formatacao do numero unico CNJ (Resolucao CNJ 65/2008).
Formato: NNNNNNN-DD.AAAA.J.TT.OOOO

O digito verificador (DD) usa o modulo 97 (ISO 7064 MOD 97-10), o que permite
pegar erros de digitacao ANTES de gastar uma consulta.
"""

from __future__ import annotations

import re

CNJ_FORMAT_RE = re.compile(r"^\d{7}-\d{2}\.\d{4}\.\d\.\d{2}\.\d{4}$")


def only_digits(cnj: str) -> str:
    return re.sub(r"\D", "", cnj or "")


def calc_dv(sequencial: str, ano: str, segmento: str, tribunal: str, origem: str) -> str:
    """Calcula o digito verificador (2 digitos) de um numero CNJ."""
    base = f"{sequencial}{ano}{segmento}{tribunal}{origem}"  # 18 digitos, sem o DD
    resto = (int(base) * 100) % 97
    return f"{98 - resto:02d}"


def is_valid(cnj: str) -> bool:
    """True se o numero tem 20 digitos E o digito verificador confere."""
    d = only_digits(cnj)
    if len(d) != 20:
        return False
    sequencial, dd, ano, seg, tt, origem = d[0:7], d[7:9], d[9:13], d[13], d[14:16], d[16:20]
    return calc_dv(sequencial, ano, seg, tt, origem) == dd


def format_cnj(cnj: str) -> str:
    """Formata 20 digitos como NNNNNNN-DD.AAAA.J.TT.OOOO."""
    d = only_digits(cnj)
    if len(d) != 20:
        return cnj
    return f"{d[0:7]}-{d[7:9]}.{d[9:13]}.{d[13]}.{d[14:16]}.{d[16:20]}"


if __name__ == "__main__":
    import sys
    for arg in sys.argv[1:]:
        print(f"{format_cnj(arg)}  ->  {'VALIDO' if is_valid(arg) else 'INVALIDO'}")
