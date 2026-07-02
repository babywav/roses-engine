import { motion } from "framer-motion";
import { ExternalLink, Scale, Clock } from "lucide-react";
import type { Process } from "../types";

interface Props {
  process: Process;
  index: number;
}

export default function ProcessCard({ process, index }: Props) {
  const ultima = process.movimentacoes?.[0];
  return (
    <motion.div
      initial={{ opacity: 0, y: 16, scale: 0.98 }}
      animate={{ opacity: 1, y: 0, scale: 1 }}
      transition={{ delay: index * 0.06, type: "spring", stiffness: 260, damping: 24 }}
      className="glass rounded-3xl p-4 hover:border-accent/40 transition-colors"
    >
      <div className="flex items-start justify-between gap-3">
        <div className="min-w-0">
          <div className="font-mono text-[13px] text-accent-soft tracking-tight truncate">
            {process.numero}
          </div>
          <div className="mt-1 text-sm font-medium leading-snug">{process.classe}</div>
        </div>
        {process.url_processo && (
          <a
            href={process.url_processo}
            target="_blank"
            rel="noreferrer"
            className="shrink-0 grid place-items-center w-8 h-8 rounded-full bg-white/[0.05] hover:bg-accent hover:text-white text-muted transition-colors"
            title="Ver detalhes"
          >
            <ExternalLink size={15} />
          </a>
        )}
      </div>

      {process.assunto && (
        <div className="mt-2 flex items-center gap-1.5 text-xs text-muted">
          <Scale size={13} className="shrink-0" />
          <span className="truncate">{process.assunto}</span>
        </div>
      )}

      {process.partes?.length > 0 && (
        <div className="mt-3 text-xs leading-relaxed text-white/80">
          {process.partes.map((p, i) => (
            <span key={i}>
              {i > 0 && <span className="text-muted"> &nbsp;×&nbsp; </span>}
              {p.nome}
            </span>
          ))}
        </div>
      )}

      {ultima && (
        <div className="mt-3 pt-3 border-t border-white/5 flex items-center gap-1.5 text-xs text-muted">
          <Clock size={13} className="shrink-0 text-accent-soft" />
          <span className="truncate">{ultima.descricao}</span>
          <span className="ml-auto font-mono text-[11px] shrink-0">{ultima.data}</span>
        </div>
      )}
    </motion.div>
  );
}
