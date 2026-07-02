import { motion } from "framer-motion";
import { User } from "lucide-react";
import FormInput from "../../components/FormInput";
import type { RegisterData } from "../../types";

interface Props {
  data: RegisterData;
  onChange: (patch: Partial<RegisterData>) => void;
  onNext: () => void;
  error?: string;
}

const container = {
  hidden: {},
  show: { transition: { staggerChildren: 0.1, delayChildren: 0.05 } },
};
const item = {
  hidden: { opacity: 0, y: 16 },
  show: { opacity: 1, y: 0, transition: { type: "spring", stiffness: 260, damping: 24 } },
};

export default function StepPersonal({ data, onChange, onNext, error }: Props) {
  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault();
    onNext();
  };

  return (
    <motion.form
      variants={container}
      initial="hidden"
      animate="show"
      onSubmit={handleSubmit}
      className="flex flex-col h-full px-7 pt-6 pb-8"
    >
      {/* Visual */}
      <motion.div variants={item} className="flex-1 flex flex-col items-center justify-center">
        <div className="relative mb-8">
          <motion.div
            className="absolute -inset-8 rounded-full bg-accent/20 blur-2xl"
            animate={{ opacity: [0.3, 0.6, 0.3] }}
            transition={{ duration: 4, repeat: Infinity }}
          />
          <motion.div
            animate={{ y: [0, -8, 0] }}
            transition={{ duration: 5, repeat: Infinity, ease: "easeInOut" }}
            className="relative grid place-items-center w-24 h-24 rounded-[1.8rem] glass"
          >
            <User size={44} strokeWidth={1.4} className="text-white/80" />
          </motion.div>
        </div>

        <motion.h1 variants={item} className="text-2xl font-semibold tracking-tight text-center">
          Vamos começar
        </motion.h1>
        <motion.p variants={item} className="mt-2 text-sm text-muted text-center max-w-[280px]">
          Primeiro, nos diga seu nome completo para personalizar sua experiência.
        </motion.p>
      </motion.div>

      {/* Form */}
      <motion.div variants={item} className="space-y-5">
        <FormInput
          label="Nome completo"
          value={data.nome}
          onChange={(v) => onChange({ nome: v })}
          placeholder="Ex: Maria Silva dos Santos"
          icon={<User size={16} />}
          error={error}
          autoFocus
        />

        <motion.button
          type="submit"
          whileHover={{ scale: 1.02 }}
          whileTap={{ scale: 0.97 }}
          className="w-full py-4 rounded-full bg-accent text-white font-semibold shadow-glow"
        >
          Continuar
        </motion.button>
      </motion.div>
    </motion.form>
  );
}
