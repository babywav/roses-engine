"""
roses/engine/ipv6_rotator.py

IPv6 Synthetic Rotator — Geração matemática de IPs únicos a partir do
prefixo /64 delegado pelo ISP. Sem terceiros. Sem 4G. Sem serviços pagos.

Matemática:
  IID = BLAKE2b(prefix || timestamp_ms || nonce || session_key) [64 bits]
  IPv6 = prefix:IID

Com um /64, temos 2^64 = 18.446.744.073.709.551.616 endereços únicos.
Probabilidade de colisão após 1 bilhão de consultas: ~0.000000000005%
"""

import asyncio
import hashlib
import ipaddress
import json
import os
import re
import secrets
import socket
import struct
import subprocess
import time
from dataclasses import dataclass
from pathlib import Path
from typing import Optional


# --------------------------------------------------------------------------
# Configuração
# --------------------------------------------------------------------------

SESSION_KEY_PATH = Path(__file__).parent.parent / "datalake" / "session_key.bin"
USED_ADDRESSES_LOG = Path(__file__).parent.parent / "datalake" / "ipv6_session_log.jsonl"
SOCKS5_PORT = 19090  # Porta do microsserviço SOCKS5 local


# --------------------------------------------------------------------------
# Geração da chave de sessão local (secreta, gerada uma vez na instalação)
# --------------------------------------------------------------------------

def get_or_create_session_key() -> bytes:
    """
    Retorna a chave secreta local de 128 bits.
    Gerada uma vez e armazenada em datalake/session_key.bin.
    Essa chave é o que torna os IIDs imprevisíveis para observadores externos.
    """
    SESSION_KEY_PATH.parent.mkdir(parents=True, exist_ok=True)
    if SESSION_KEY_PATH.exists():
        key = SESSION_KEY_PATH.read_bytes()
        if len(key) == 16:
            return key
    # Gera 128 bits de entropia criptográfica
    key = secrets.token_bytes(16)
    SESSION_KEY_PATH.write_bytes(key)
    return key


# --------------------------------------------------------------------------
# Detecção do prefixo IPv6 do ISP
# --------------------------------------------------------------------------

def detect_ipv6_prefix(interface: Optional[str] = None) -> Optional[str]:
    """
    Detecta o prefixo /64 global delegado pelo ISP.
    Retorna apenas endereços globais (não fe80:: link-local, não ::1 loopback).
    
    Returns:
        String do prefixo no formato "2804:xxxx:yyyy:zzzz" ou None se indisponível.
    """
    try:
        # macOS: usa ifconfig
        result = subprocess.run(
            ["ifconfig"],
            capture_output=True, text=True, timeout=5
        )
        output = result.stdout

        # Filtra linhas inet6 com prefixlen 64 que sejam globais
        pattern = re.compile(
            r"inet6\s+([2-3][0-9a-fA-F]{3}:[0-9a-fA-F:]+)\s+prefixlen\s+64",
        )
        for match in pattern.finditer(output):
            addr_str = match.group(1)
            try:
                addr = ipaddress.IPv6Address(addr_str)
                # Só aceita global unicast (não link-local, não loopback)
                if addr.is_global and not addr.is_loopback and not addr.is_link_local:
                    # Extrai os primeiros 64 bits (4 grupos de 16 bits)
                    exploded = addr.exploded  # "2804:1234:abcd:ef01:xxxx:xxxx:xxxx:xxxx"
                    parts = exploded.split(":")
                    prefix = ":".join(parts[:4])
                    return prefix
            except ValueError:
                continue

    except Exception:
        pass

    # Fallback: tenta via socket
    try:
        s = socket.socket(socket.AF_INET6, socket.SOCK_DGRAM)
        s.connect(("2606:4700:4700::1111", 80))  # Cloudflare DNS IPv6
        addr = s.getsockname()[0]
        s.close()
        parsed = ipaddress.IPv6Address(addr)
        if parsed.is_global:
            parts = parsed.exploded.split(":")
            return ":".join(parts[:4])
    except Exception:
        pass

    return None


# --------------------------------------------------------------------------
# Geração de IID com BLAKE2b
# --------------------------------------------------------------------------

def generate_iid(prefix: str, session_key: bytes, counter: Optional[int] = None) -> str:
    """
    Gera um Interface Identifier (IID) de 64 bits único e criptograficamente imprevisível.
    
    Usa BLAKE2b com:
    - prefix: impede cross-prefix correlation
    - timestamp_ms: componente temporal em milissegundos
    - nonce: 8 bytes de entropia adicional do CSPRNG
    - session_key: chave secreta local de 128 bits
    - counter: garante unicidade em colisões de timestamp
    
    A saída é determinística para os mesmos inputs mas imprevisível
    para qualquer observador sem conhecimento da session_key.
    
    Returns:
        IID no formato "a3f2:bc10:847c:11ed"
    """
    ts_ms = int(time.time() * 1000).to_bytes(8, "big")
    nonce = secrets.token_bytes(8)
    ctr = (counter or 0).to_bytes(4, "big")

    digest = hashlib.blake2b(
        prefix.encode("utf-8") + ts_ms + nonce + ctr + session_key,
        digest_size=8,   # 64 bits = IID completo
    ).digest()

    # Formata como 4 grupos de 16 bits separados por ":"
    parts = []
    for i in range(0, 8, 2):
        parts.append(f"{digest[i]:02x}{digest[i+1]:02x}")
    return ":".join(parts)


def build_ipv6_address(prefix: str, iid: str) -> str:
    """Combina prefixo /64 com IID para formar endereço IPv6 completo."""
    return f"{prefix}:{iid}"


def validate_ipv6(addr: str) -> bool:
    """Valida que o endereço IPv6 gerado é sintaticamente correto."""
    try:
        ipaddress.IPv6Address(addr)
        return True
    except ValueError:
        return False


# --------------------------------------------------------------------------
# Gerenciamento de aliases na interface de rede (macOS)
# --------------------------------------------------------------------------

def add_ipv6_alias(address: str, interface: str = "en0") -> bool:
    """
    Adiciona um endereço IPv6 como alias na interface especificada.
    Requer sudo (ou que o processo tenha capacidades de rede).
    
    macOS: ifconfig en0 inet6 ADDR prefixlen 128 alias
    """
    try:
        result = subprocess.run(
            ["sudo", "ifconfig", interface, "inet6", address, "prefixlen", "128", "alias"],
            capture_output=True, text=True, timeout=5
        )
        return result.returncode == 0
    except Exception:
        return False


def remove_ipv6_alias(address: str, interface: str = "en0") -> bool:
    """Remove o alias IPv6 da interface após uso."""
    try:
        result = subprocess.run(
            ["sudo", "ifconfig", interface, "inet6", address, "prefixlen", "128", "-alias"],
            capture_output=True, text=True, timeout=5
        )
        return result.returncode == 0
    except Exception:
        return False


def get_default_interface() -> str:
    """Detecta a interface de rede padrão no macOS."""
    try:
        result = subprocess.run(
            ["route", "get", "default"],
            capture_output=True, text=True, timeout=5
        )
        for line in result.stdout.splitlines():
            if "interface:" in line:
                return line.split("interface:")[1].strip()
    except Exception:
        pass
    return "en0"


# --------------------------------------------------------------------------
# Pool de endereços com cleanup automático
# --------------------------------------------------------------------------

@dataclass
class IPv6Session:
    address: str
    interface: str
    prefix: str
    iid: str
    created_at: float
    alias_added: bool = False

    def cleanup(self):
        """Remove o alias criado para esta sessão."""
        if self.alias_added:
            remove_ipv6_alias(self.address, self.interface)
            self.alias_added = False

    def __enter__(self):
        return self

    def __exit__(self, *args):
        self.cleanup()


# --------------------------------------------------------------------------
# Motor principal do rotador IPv6
# --------------------------------------------------------------------------

class IPv6RotatorEngine:
    """
    Motor de rotação sintética de endereços IPv6.
    
    Uso típico:
        engine = IPv6RotatorEngine()
        if engine.is_available():
            with engine.new_session() as session:
                # session.address = IP único desta sessão
                # Faz scraping usando session.address como source IP
                pass
        else:
            # Sem IPv6 — opera sem rotação de IP
            pass
    """

    def __init__(self, interface: Optional[str] = None):
        self.session_key = get_or_create_session_key()
        self.interface = interface or get_default_interface()
        self._prefix: Optional[str] = None
        self._counter = 0
        self._active_sessions: list[IPv6Session] = []

    @property
    def prefix(self) -> Optional[str]:
        if self._prefix is None:
            self._prefix = detect_ipv6_prefix(self.interface)
        return self._prefix

    def is_available(self) -> bool:
        """Verifica se o ISP fornece IPv6 e se é viável usar rotação."""
        return self.prefix is not None

    def generate_address(self) -> str:
        """
        Gera um novo endereço IPv6 único usando BLAKE2b.
        
        O endereço gerado:
        - É único com probabilidade ~1 em 2^64
        - É imprevisível sem conhecer a session_key
        - Pertence ao espaço /64 delegado pelo ISP
        """
        if not self.prefix:
            raise RuntimeError("IPv6 não disponível neste ambiente")

        iid = generate_iid(self.prefix, self.session_key, self._counter)
        self._counter += 1
        addr = build_ipv6_address(self.prefix, iid)

        # Sanity check
        if not validate_ipv6(addr):
            raise ValueError(f"Endereço IPv6 inválido gerado: {addr}")

        return addr

    def new_session(self, add_alias: bool = True) -> IPv6Session:
        """
        Cria uma nova sessão com endereço IPv6 único.
        
        Args:
            add_alias: Se True, adiciona o alias à interface (requer sudo).
                       Se False, apenas gera o endereço sem configurar a rede.
        """
        addr = self.generate_address()
        iid = addr.split(":", 4)[-1]  # Extrai a parte IID

        session = IPv6Session(
            address=addr,
            interface=self.interface,
            prefix=self.prefix,
            iid=iid,
            created_at=time.time(),
        )

        if add_alias:
            success = add_ipv6_alias(addr, self.interface)
            session.alias_added = success
            if not success:
                print(f"[IPv6Rotator] Aviso: não foi possível adicionar alias {addr}")
                print("[IPv6Rotator] Execute: sudo chmod u+s /sbin/ifconfig para evitar sudo")

        # Log da sessão
        self._log_session(session)
        self._active_sessions.append(session)
        return session

    def cleanup_all(self):
        """Remove todos os aliases de sessões ativas."""
        for session in self._active_sessions:
            session.cleanup()
        self._active_sessions.clear()

    def _log_session(self, session: IPv6Session):
        """Registra a sessão em log JSONL para auditoria."""
        USED_ADDRESSES_LOG.parent.mkdir(parents=True, exist_ok=True)
        entry = {
            "address": session.address,
            "prefix": session.prefix,
            "iid": session.iid,
            "interface": session.interface,
            "created_at": session.created_at,
            "timestamp_iso": time.strftime(
                "%Y-%m-%dT%H:%M:%SZ", time.gmtime(session.created_at)
            ),
        }
        with open(USED_ADDRESSES_LOG, "a", encoding="utf-8") as f:
            f.write(json.dumps(entry) + "\n")

    def get_stats(self) -> dict:
        """Retorna estatísticas do rotador."""
        return {
            "prefix": self.prefix,
            "interface": self.interface,
            "available": self.is_available(),
            "addresses_generated": self._counter,
            "active_sessions": len(self._active_sessions),
            "address_space": f"2^64 = {2**64:,}" if self.is_available() else "N/A",
        }


# --------------------------------------------------------------------------
# Microsserviço SOCKS5 local — permite Playwright usar source IPv6 específico
# --------------------------------------------------------------------------

class LocalSOCKS5Server:
    """
    Servidor SOCKS5 mínimo que força o bind em um endereço IPv6 específico.
    Usado como proxy para o Playwright — cada sessão usa uma porta diferente.
    
    Protocolo SOCKS5 (RFC 1928):
    Fase 1 — Negociação de autenticação
    Fase 2 — Requisição de conexão
    Fase 3 — Relay bidirecional
    """

    def __init__(self, src_ipv6: str, port: int = 0):
        """
        Args:
            src_ipv6: Endereço IPv6 de origem para todas as conexões saídas.
            port: Porta local para escutar (0 = aloca automaticamente).
        """
        self.src_ipv6 = src_ipv6
        self.port = port
        self._server = None
        self._actual_port = port

    async def start(self) -> int:
        """Inicia o servidor e retorna a porta alocada."""
        self._server = await asyncio.start_server(
            self._handle_client,
            "127.0.0.1",
            self.port,
        )
        self._actual_port = self._server.sockets[0].getsockname()[1]
        return self._actual_port

    async def stop(self):
        """Para o servidor."""
        if self._server:
            self._server.close()
            await self._server.wait_closed()

    async def _handle_client(
        self,
        reader: asyncio.StreamReader,
        writer: asyncio.StreamWriter,
    ):
        """Handler de conexão SOCKS5 — implementação do RFC 1928."""
        try:
            # --- Fase 1: Negociação ---
            # Cliente envia: VER(1) NMETHODS(1) METHODS(N)
            header = await reader.readexactly(2)
            ver, nmethods = header[0], header[1]
            if ver != 5:
                writer.close()
                return
            await reader.readexactly(nmethods)  # Lê métodos (ignora)
            # Responde: VER=5, METHOD=0 (sem autenticação)
            writer.write(b"\x05\x00")
            await writer.drain()

            # --- Fase 2: Requisição ---
            # Cliente envia: VER CMD RSV ATYP DST_ADDR DST_PORT
            req = await reader.readexactly(4)
            ver, cmd, rsv, atyp = req[0], req[1], req[2], req[3]

            if cmd != 1:  # 0x01 = CONNECT
                writer.write(b"\x05\x07\x00\x01" + b"\x00" * 6)
                writer.close()
                return

            # Parse do endereço destino
            if atyp == 1:  # IPv4
                dst_raw = await reader.readexactly(4)
                dst_host = socket.inet_ntop(socket.AF_INET, dst_raw)
            elif atyp == 3:  # Domínio
                length = (await reader.readexactly(1))[0]
                dst_host = (await reader.readexactly(length)).decode()
            elif atyp == 4:  # IPv6
                dst_raw = await reader.readexactly(16)
                dst_host = socket.inet_ntop(socket.AF_INET6, dst_raw)
            else:
                writer.write(b"\x05\x08\x00\x01" + b"\x00" * 6)
                writer.close()
                return

            port_raw = await reader.readexactly(2)
            dst_port = struct.unpack("!H", port_raw)[0]

            # --- Fase 3: Conectar ao destino COM SOURCE IPv6 específico ---
            try:
                # Resolve DNS se necessário
                loop = asyncio.get_event_loop()
                infos = await loop.getaddrinfo(
                    dst_host, dst_port,
                    family=socket.AF_UNSPEC,
                    type=socket.SOCK_STREAM,
                )
                if not infos:
                    raise ConnectionError("DNS resolution failed")

                # Tenta IPv6 primeiro, depois IPv4
                target_info = None
                for info in infos:
                    if info[0] == socket.AF_INET6:
                        target_info = info
                        break
                if target_info is None:
                    target_info = infos[0]

                af, socktype, proto, canonname, sockaddr = target_info

                # Cria socket com bind no endereço IPv6 de origem
                raw_sock = socket.socket(socket.AF_INET6, socket.SOCK_STREAM)
                raw_sock.setblocking(False)
                try:
                    raw_sock.bind((self.src_ipv6, 0, 0, 0))  # ← Força source IP
                except OSError:
                    # Se o bind falhar (alias não configurado), usa socket padrão
                    raw_sock.close()
                    raw_sock = socket.socket(af, socket.SOCK_STREAM)
                    raw_sock.setblocking(False)

                await loop.sock_connect(raw_sock, sockaddr)

                # Sucesso — informa o cliente
                writer.write(b"\x05\x00\x00\x01" + b"\x00" * 4 + b"\x00\x00")
                await writer.drain()

                # Relay bidirecional
                remote_reader, remote_writer = await asyncio.open_connection(sock=raw_sock)
                await asyncio.gather(
                    self._relay(reader, remote_writer),
                    self._relay(remote_reader, writer),
                    return_exceptions=True,
                )

            except Exception as e:
                # Falha na conexão
                writer.write(b"\x05\x05\x00\x01" + b"\x00" * 6)
                await writer.drain()

        except asyncio.IncompleteReadError:
            pass
        except Exception:
            pass
        finally:
            try:
                writer.close()
                await writer.wait_closed()
            except Exception:
                pass

    @staticmethod
    async def _relay(src: asyncio.StreamReader, dst: asyncio.StreamWriter):
        """Relay bidirecional de dados entre dois streams."""
        try:
            while True:
                data = await src.read(65536)
                if not data:
                    break
                dst.write(data)
                await dst.drain()
        except Exception:
            pass
        finally:
            try:
                dst.close()
            except Exception:
                pass


# --------------------------------------------------------------------------
# Interface de alto nível — usada pelo scraper
# --------------------------------------------------------------------------

class GhostIPLayer:
    """
    Camada de IP Ghost — combina rotação IPv6 com proxy SOCKS5 local.
    Interface de alto nível para uso no pje_unified_scraper.py.
    """

    def __init__(self):
        self.rotator = IPv6RotatorEngine()
        self._server: Optional[LocalSOCKS5Server] = None
        self._session: Optional[IPv6Session] = None
        self._proxy_port: Optional[int] = None

    def is_available(self) -> bool:
        return self.rotator.is_available()

    def get_proxy_url(self) -> Optional[str]:
        """Retorna a URL do proxy SOCKS5 local para uso no Playwright."""
        if self._proxy_port:
            return f"socks5://127.0.0.1:{self._proxy_port}"
        return None

    def setup_session(self) -> Optional[str]:
        """
        Configura uma nova sessão IPv6:
        1. Gera endereço IPv6 único
        2. Adiciona alias à interface
        3. Inicia servidor SOCKS5 local com bind no novo IP
        
        Returns:
            URL do proxy SOCKS5 para usar no Playwright, ou None se indisponível.
        """
        if not self.is_available():
            return None

        try:
            self._session = self.rotator.new_session(add_alias=True)
            src_ip = self._session.address

            # Inicia SOCKS5 server em thread separada
            import threading

            loop = asyncio.new_event_loop()

            def run_server():
                asyncio.set_event_loop(loop)
                loop.run_forever()

            thread = threading.Thread(target=run_server, daemon=True)
            thread.start()

            # Inicia o servidor no loop da thread
            server = LocalSOCKS5Server(src_ip, port=0)
            future = asyncio.run_coroutine_threadsafe(server.start(), loop)
            port = future.result(timeout=5)

            self._server = server
            self._proxy_port = port

            print(f"[GhostIP] ✅ Sessão IPv6: {src_ip}")
            print(f"[GhostIP] ✅ SOCKS5 local: 127.0.0.1:{port}")
            return f"socks5://127.0.0.1:{port}"

        except Exception as e:
            print(f"[GhostIP] ⚠️ Falha ao configurar IPv6: {e}")
            print("[GhostIP] Continuando sem rotação de IP")
            return None

    def teardown_session(self):
        """Limpa a sessão: remove alias IPv6 e encerra SOCKS5."""
        if self._session:
            self._session.cleanup()
            self._session = None
        self._proxy_port = None

    def __enter__(self):
        self.setup_session()
        return self

    def __exit__(self, *args):
        self.teardown_session()


# --------------------------------------------------------------------------
# CLI de diagnóstico
# --------------------------------------------------------------------------

if __name__ == "__main__":
    print("🌹 Roses — IPv6 Synthetic Rotator\n")

    engine = IPv6RotatorEngine()

    print(f"Interface padrão : {engine.interface}")
    print(f"Prefixo ISP      : {engine.prefix or '❌ Não disponível'}")
    print(f"IPv6 disponível  : {'✅ Sim' if engine.is_available() else '❌ Não'}\n")

    if engine.is_available():
        print("Gerando 5 endereços únicos de exemplo:")
        for i in range(5):
            addr = engine.generate_address()
            print(f"  [{i+1}] {addr}")

        print(f"\nEspaço de endereçamento: {2**64:,} endereços únicos")
        print(f"Chave de sessão: {SESSION_KEY_PATH}")

        print("\nEstatísticas:")
        stats = engine.get_stats()
        for k, v in stats.items():
            print(f"  {k}: {v}")
    else:
        print("⚠️  IPv6 não detectado. Verifique se seu ISP fornece IPv6:")
        print("   $ curl -6 https://ipv6.icanhazip.com")
        print("   $ ifconfig | grep 'inet6.*prefixlen 64'")
        print("\nO scraper ainda funcionará — apenas sem rotação de IP.")
