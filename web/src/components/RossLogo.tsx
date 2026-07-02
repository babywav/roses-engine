interface Props {
  /** Tamanho do ícone SVG em px (default 32) */
  iconSize?: number;
  /** Tamanho do texto em px (default 22) */
  fontSize?: number;
  /** Cor de tudo — padrão branco */
  color?: string;
  /** Mostra só o ícone, sem o texto */
  iconOnly?: boolean;
  className?: string;
}

export default function RossLogo({
  iconSize = 32,
  fontSize = 22,
  color = "#ffffff",
  iconOnly = false,
  className = "",
}: Props) {
  return (
    <div
      className={className}
      style={{ display: "flex", alignItems: "center", gap: iconSize * 0.4 }}
    >
      {/* Ícone hexagonal */}
      <svg
        width={iconSize}
        height={iconSize}
        viewBox="0 0 100 100"
        xmlns="http://www.w3.org/2000/svg"
        style={{ flexShrink: 0 }}
      >
        {/* Linhas de conexão */}
        <g
          stroke={color}
          strokeWidth="2.2"
          fill="none"
          strokeLinecap="round"
          strokeLinejoin="round"
          opacity={0.9}
        >
          {/* Hexágono Externo */}
          <polygon points="50,5 89,27 89,72 50,95 11,72 11,27" />
          {/* Hexágono Interno */}
          <polygon points="50,20 75,35 75,65 50,80 25,65 25,35" />
          {/* Eixos */}
          <line x1="50" y1="5" x2="50" y2="95" />
          <line x1="11" y1="27" x2="89" y2="72" />
          <line x1="11" y1="72" x2="89" y2="27" />
          {/* Conexões zig-zag */}
          <line x1="50" y1="5" x2="25" y2="35" />
          <line x1="50" y1="5" x2="75" y2="35" />
          <line x1="89" y1="27" x2="50" y2="20" />
          <line x1="89" y1="27" x2="75" y2="65" />
          <line x1="89" y1="72" x2="75" y2="35" />
          <line x1="89" y1="72" x2="50" y2="80" />
          <line x1="50" y1="95" x2="75" y2="65" />
          <line x1="50" y1="95" x2="25" y2="65" />
          <line x1="11" y1="72" x2="50" y2="80" />
          <line x1="11" y1="72" x2="25" y2="35" />
          <line x1="11" y1="27" x2="25" y2="65" />
          <line x1="11" y1="27" x2="50" y2="20" />
        </g>
        {/* Nós */}
        <g fill={color} opacity={0.95}>
          {/* Externos */}
          <circle cx="50" cy="5" r="3.5" />
          <circle cx="89" cy="27" r="3.5" />
          <circle cx="89" cy="72" r="3.5" />
          <circle cx="50" cy="95" r="3.5" />
          <circle cx="11" cy="72" r="3.5" />
          <circle cx="11" cy="27" r="3.5" />
          {/* Internos */}
          <circle cx="50" cy="20" r="3" />
          <circle cx="75" cy="35" r="3" />
          <circle cx="75" cy="65" r="3" />
          <circle cx="50" cy="80" r="3" />
          <circle cx="25" cy="65" r="3" />
          <circle cx="25" cy="35" r="3" />
          {/* Central */}
          <circle cx="50" cy="50" r="3.5" />
        </g>
      </svg>

      {/* Texto ROSS AI */}
      {!iconOnly && (
        <div style={{ display: "flex", alignItems: "baseline", gap: 0 }}>
          <span
            style={{
              fontFamily: "'Montserrat', -apple-system, system-ui, sans-serif",
              fontSize,
              fontWeight: 800,
              color,
              letterSpacing: "-0.04em",
              lineHeight: 1,
            }}
          >
            ROSS
          </span>
          <span
            style={{
              fontFamily: "'Montserrat', -apple-system, system-ui, sans-serif",
              fontSize,
              fontWeight: 400,
              color,
              letterSpacing: "-0.04em",
              lineHeight: 1,
              marginLeft: "0.25em",
              opacity: 0.75,
            }}
          >
            AI
          </span>
        </div>
      )}
    </div>
  );
}
