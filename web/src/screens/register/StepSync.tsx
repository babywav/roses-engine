import { motion } from "framer-motion";
import { RefreshCw, ArrowRight } from "lucide-react";
import type { RegisterData } from "../../types";

interface Props {
  data: RegisterData;
  onSync: () => void;
  onSkip: () => void;
}

const container = {
  hidden: {},
  show: { transition: { staggerChildren: 0.12, delayChildren: 0.1 } },
};
const item = {
  hidden: { opacity: 0, y: 18 },
  show: { opacity: 1, y: 0, transition: { type: "spring", stiffness: 240, damping: 22 } },
};

export default function StepSync({ data, onSync, onSkip }: Props) {
  return (
    <motion.div
      variants={container}
      initial="hidden"
      animate="show"
      className="flex flex-col h-full px-7 pt-6 pb-8"
    >
      {/* Visual */}
      <div className="flex-1 flex flex-col items-center justify-center">
        <motion.div variants={item} className="relative mb-8">
          <motion.div
            className="absolute -inset-10 rounded-full bg-accent/20 blur-3xl"
            animate={{ opacity: [0.3, 0.7, 0.3] }}
            transition={{ duration: 3, repeat: Infinity }}
          />
          <motion.div
            animate={{ rotate: [0, 360] }}
            transition={{ duration: 8, repeat: Infinity, ease: "linear" }}
            className="relative grid place-items-center w-28 h-28 rounded-[2rem] glass"
          >
            <RefreshCw size={48} strokeWidth={1.4} className="text-accent-soft" />
          </motion.div>
        </motion.div>

        <motion.h1 variants={item} className="text-2xl font-semibold tracking-tight text-center">
          Sincronizar processos?
        </motion.h1>
        <motion.p variants={item} className="mt-3 text-sm text-muted text-center max-w-[300px] leading-relaxed">
          Encontramos processos vinculados à OAB{" "}
          <span className="text-white font-medium">{data.oab}</span>
          {" / "}
          <span className="text-white font-medium">{data.oabEstado}</span>.
          Deseja sincronizá-los agora?
        </motion.p>
      </div>

      {/* Option cards */}
      <motion.div variants={item} className="space-y-3">
        <motion.button
          whileHover={{ scale: 1.02 }}
          whileTap={{ scale: 0.97 }}
          onClick={onSync}
          className="w-full p-5 rounded-2xl glass border border-accent/20 hover:border-accent/40 transition-colors text-left group"
        >
          <div className="flex items-center gap-4">
            <div className="grid place-items-center w-11 h-11 rounded-xl bg-accent/15">
              <RefreshCw size={20} className="text-accent-soft" />
            </div>
            <div className="flex-1">
              <div className="font-semibold text-white">Sincronizar agora</div>
              <div className="text-xs text-muted mt-0.5">
                Importar todos os processos vinculados
              </div>
            </div>
            <ArrowRight size={18} className="text-muted group-hover:text-accent-soft transition-colors" />
          </div>
        </motion.button>

        <motion.button
          whileHover={{ scale: 1.02 }}
          whileTap={{ scale: 0.97 }}
          onClick={onSkip}
          className="w-full p-5 rounded-2xl glass hover:bg-white/[0.04] transition-colors text-left group"
        >
          <div className="flex items-center gap-4">
            <div className="grid place-items-center w-11 h-11 rounded-xl bg-white/[0.06]">
              <ArrowRight size={20} className="text-muted" />
            </div>
            <div className="flex-1">
              <div className="font-semibold text-white/80">Agora não</div>
              <div className="text-xs text-muted mt-0.5">
                Você pode sincronizar depois nas configurações
              </div>
            </div>
            <ArrowRight size={18} className="text-muted group-hover:text-white/50 transition-colors" />
          </div>
        </motion.button>
      </motion.div>
    </motion.div>
  );
}
