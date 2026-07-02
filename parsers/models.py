"""
roses/parsers/models.py

Modelos de dados normalizados para processos jurídicos.
Output padrão do motor Roses, independente do tribunal consultado.
"""

from dataclasses import dataclass, field, asdict
from typing import Optional
import json


@dataclass
class Party:
    """Parte do processo (autor, réu, advogado, etc.)."""
    tipo: str          # "Autor", "Réu", "Advogado", "Representante", etc.
    nome: str
    documento: Optional[str] = None   # CPF/CNPJ se disponível
    oab: Optional[str] = None         # Número OAB se for advogado


@dataclass
class Movement:
    """Movimentação/andamento do processo."""
    data: str          # Data no formato DD/MM/AAAA
    descricao: str     # Descrição do andamento
    orgao: Optional[str] = None  # Órgão julgador


@dataclass
class Process:
    """Processo jurídico completo."""
    numero: str                           # Número CNJ completo
    classe: str                           # Classe processual
    assunto: str                          # Assunto principal
    tribunal: str                         # Sigla do tribunal
    orgao_julgador: Optional[str] = None  # Vara/câmara
    data_distribuicao: Optional[str] = None
    partes: list[Party] = field(default_factory=list)
    movimentacoes: list[Movement] = field(default_factory=list)
    url_processo: Optional[str] = None    # Link direto para o processo

    def to_dict(self) -> dict:
        return asdict(self)


@dataclass
class RosesResult:
    """Resultado completo de uma consulta Roses."""
    status: str          # "OK" | "NOT_FOUND" | "INVALID" | "CHALLENGE" | "ERROR" | "UNKNOWN"
    message: str
    tribunal: str
    tribunal_name: str
    query: dict
    total: int = 0
    processos: list[Process] = field(default_factory=list)
    raw_html: Optional[str] = None  # HTML bruto (debug)
    elapsed_seconds: Optional[float] = None

    def to_dict(self) -> dict:
        d = asdict(self)
        d.pop("raw_html", None)  # Não exporta HTML bruto por padrão
        return d

    def to_json(self, indent: int = 2) -> str:
        return json.dumps(self.to_dict(), indent=indent, ensure_ascii=False)

    @classmethod
    def error(cls, message: str, query: dict = None) -> "RosesResult":
        return cls(
            status="ERROR",
            message=message,
            tribunal="N/A",
            tribunal_name="N/A",
            query=query or {},
        )

    @classmethod
    def not_found(cls, tribunal: str, tribunal_name: str, query: dict = None) -> "RosesResult":
        return cls(
            status="NOT_FOUND",
            message="Nenhum processo encontrado para os critérios informados.",
            tribunal=tribunal,
            tribunal_name=tribunal_name,
            query=query or {},
            total=0,
        )
