import { motion } from "framer-motion";
import { MessagesSquare } from "lucide-react";
import RossLogo from "../components/RossLogo";

interface Props {
  onStart: () => void;
}

const container = {
  hidden: {},
  show: { transition: { staggerChildren: 0.12, delayChildren: 0.1 } },
};
const item = {
  hidden: { opacity: 0, y: 18 },
  show: { opacity: 1, y: 0, transition: { type: "spring", stiffness: 220, damping: 22 } },
};

export default function Onboarding({ onStart }: Props) {
  return (
    <motion.div
      variants={container}
      initial="hidden"
      animate="show"
      className="flex flex-col h-full px-7 pt-8 pb-10"
    >
      <div className="flex items-center justify-between">
        <RossLogo iconSize={24} fontSize={15} />
        <button onClick={onStart} className="text-sm text-muted hover:text-white transition-colors">
          Pular
        </button>
      </div>

      {/* Visual central: bolhas flutuantes com glow */}
      <div className="flex-1 grid place-items-center">
        <div className="relative">
          <motion.div
            className="absolute -inset-10 rounded-full bg-accent/20 blur-3xl"
            animate={{ opacity: [0.4, 0.8, 0.4] }}
            transition={{ duration: 4, repeat: Infinity }}
          />
          <motion.div
            animate={{ y: [0, -12, 0] }}
            transition={{ duration: 6, repeat: Infinity, ease: "easeInOut" }}
            className="relative grid place-items-center w-44 h-44 rounded-[2.5rem] glass"
          >
            <MessagesSquare size={72} strokeWidth={1.4} className="text-white/85" />
            <motion.div
              initial={{ scale: 0 }}
              animate={{ scale: 1 }}
              transition={{ delay: 0.5, type: "spring", stiffness: 300, damping: 18 }}
              className="absolute -top-3 -right-3 grid place-items-center w-9 h-9 rounded-full bg-accent text-white text-sm font-semibold shadow-glow"
            >
              2
            </motion.div>
          </motion.div>
        </div>
      </div>

      <motion.h1 variants={item} className="text-[2rem] leading-[1.1] font-semibold tracking-tight">
        Seu Assistente
        <br />
        Jurídico
      </motion.h1>
      <motion.p variants={item} className="mt-3 text-[15px] leading-relaxed text-muted">
        Consulte processos por número, nome ou OAB em todos os tribunais do
        Brasil. Acompanhe movimentações e fale com seu copiloto jurídico.
      </motion.p>

      <motion.div variants={item} className="mt-7 flex items-center justify-center gap-2">
        <span className="w-6 h-1.5 rounded-full bg-accent" />
        <span className="w-1.5 h-1.5 rounded-full bg-white/20" />
        <span className="w-1.5 h-1.5 rounded-full bg-white/20" />
      </motion.div>

      <motion.button
        variants={item}
        onClick={onStart}
        whileHover={{ scale: 1.02 }}
        whileTap={{ scale: 0.97 }}
        className="mt-6 w-full py-4 rounded-full bg-accent text-white font-semibold shadow-glow"
      >
        Iniciar consulta
      </motion.button>
    </motion.div>
  );
}
