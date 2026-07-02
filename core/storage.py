"""
roses/core/storage.py

Persistencia local (SQLite, sem dependencia externa) do motor Roses.

Guarda processos e movimentacoes e, a cada gravacao, devolve as
movimentacoes NOVAS (diff) — e isso que transforma a consulta em
SINCRONIZACAO: na 2a vez so aparece o que mudou.

Banco em roses/data/roses.db. Tudo dentro do pacote roses.
"""

from __future__ import annotations

import json
import sqlite3
import sys
import time
from pathlib import Path

ROSES_ROOT = Path(__file__).resolve().parent.parent
sys.path.insert(0, str(ROSES_ROOT))
from parsers.models import RosesResult, Process, Movement  # noqa: E402

DB_PATH = ROSES_ROOT / "data" / "roses.db"

SCHEMA = """
CREATE TABLE IF NOT EXISTS processos (
    numero            TEXT PRIMARY KEY,
    tribunal          TEXT,
    classe            TEXT,
    assunto           TEXT,
    orgao_julgador    TEXT,
    data_distribuicao TEXT,
    url_processo      TEXT,
    partes_json       TEXT,
    first_seen        REAL,
    last_seen         REAL,
    last_status       TEXT
);
CREATE TABLE IF NOT EXISTS movimentos (
    id        INTEGER PRIMARY KEY AUTOINCREMENT,
    numero    TEXT NOT NULL,
    data      TEXT,
    descricao TEXT,
    orgao     TEXT,
    seen_at   REAL,
    UNIQUE(numero, data, descricao)
);
CREATE TABLE IF NOT EXISTS tracking (
    id        INTEGER PRIMARY KEY AUTOINCREMENT,
    tipo      TEXT NOT NULL,           -- 'cnj' | 'oab'
    valor     TEXT NOT NULL,
    uf        TEXT,
    criado_em REAL,
    ativo     INTEGER DEFAULT 1,
    UNIQUE(tipo, valor, uf)
);
CREATE INDEX IF NOT EXISTS idx_mov_numero ON movimentos(numero);
"""


class Storage:
    def __init__(self, db_path: Path | str = DB_PATH):
        self.db_path = Path(db_path)
        self.db_path.parent.mkdir(parents=True, exist_ok=True)
        self.conn = sqlite3.connect(str(self.db_path))
        self.conn.row_factory = sqlite3.Row
        self.conn.executescript(SCHEMA)
        self.conn.commit()

    def close(self):
        self.conn.close()

    # -- gravacao + diff ---------------------------------------------------

    def save_process(self, proc: Process, status: str = "OK") -> list[Movement]:
        """
        Faz upsert do processo e insere as movimentacoes ainda nao vistas.
        Retorna a lista de movimentacoes NOVAS (diff desta gravacao).
        """
        now = time.time()
        cur = self.conn.cursor()

        exists = cur.execute(
            "SELECT numero FROM processos WHERE numero=?", (proc.numero,)
        ).fetchone()
        partes_json = json.dumps(
            [{"tipo": p.tipo, "nome": p.nome, "documento": p.documento, "oab": p.oab}
             for p in proc.partes], ensure_ascii=False)

        if exists:
            cur.execute(
                """UPDATE processos SET tribunal=?, classe=?, assunto=?, orgao_julgador=?,
                   data_distribuicao=?, url_processo=?, partes_json=?, last_seen=?, last_status=?
                   WHERE numero=?""",
                (proc.tribunal, proc.classe, proc.assunto, proc.orgao_julgador,
                 proc.data_distribuicao, proc.url_processo, partes_json, now, status, proc.numero),
            )
        else:
            cur.execute(
                """INSERT INTO processos (numero, tribunal, classe, assunto, orgao_julgador,
                   data_distribuicao, url_processo, partes_json, first_seen, last_seen, last_status)
                   VALUES (?,?,?,?,?,?,?,?,?,?,?)""",
                (proc.numero, proc.tribunal, proc.classe, proc.assunto, proc.orgao_julgador,
                 proc.data_distribuicao, proc.url_processo, partes_json, now, now, status),
            )

        novas: list[Movement] = []
        for mov in proc.movimentacoes:
            try:
                cur.execute(
                    "INSERT INTO movimentos (numero, data, descricao, orgao, seen_at) VALUES (?,?,?,?,?)",
                    (proc.numero, mov.data, mov.descricao, mov.orgao, now),
                )
                novas.append(mov)
            except sqlite3.IntegrityError:
                pass  # ja existia (UNIQUE) -> nao e novidade
        self.conn.commit()
        return novas

    def save_result(self, result: RosesResult) -> dict:
        """
        Persiste todos os processos de um RosesResult.
        Retorna {numero: [movimentos novos]} apenas dos que tiveram novidade.
        """
        novidades = {}
        for proc in result.processos:
            novas = self.save_process(proc, result.status)
            if novas:
                novidades[proc.numero] = novas
        return novidades

    # -- consultas ---------------------------------------------------------

    def get_movimentos(self, numero: str) -> list[dict]:
        rows = self.conn.execute(
            "SELECT data, descricao, orgao FROM movimentos WHERE numero=? ORDER BY id", (numero,)
        ).fetchall()
        return [dict(r) for r in rows]

    def count_processos(self) -> int:
        return self.conn.execute("SELECT COUNT(*) FROM processos").fetchone()[0]

    # -- tracking ----------------------------------------------------------

    def add_tracking(self, tipo: str, valor: str, uf: str | None = None) -> None:
        try:
            self.conn.execute(
                "INSERT INTO tracking (tipo, valor, uf, criado_em, ativo) VALUES (?,?,?,?,1)",
                (tipo, valor, uf, time.time()),
            )
            self.conn.commit()
        except sqlite3.IntegrityError:
            pass  # ja monitorado

    def list_tracking(self, tipo: str | None = None) -> list[dict]:
        sql = "SELECT * FROM tracking WHERE ativo=1"
        params = ()
        if tipo:
            sql += " AND tipo=?"
            params = (tipo,)
        return [dict(r) for r in self.conn.execute(sql, params).fetchall()]
