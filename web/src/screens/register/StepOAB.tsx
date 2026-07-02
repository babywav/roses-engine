import { motion } from "framer-motion";
import { Scale, Info } from "lucide-react";
import FormInput from "../../components/FormInput";
import StateSelect from "../../components/StateSelect";
import type { RegisterData } from "../../types";

interface Props {
  data: RegisterData;
  onChange: (patch: Partial<RegisterData>) => void;
  onNext: () => void;
  onSkip: () => void;
  onBack: () => void;
  errors?: { oab?: string; oabEstado?: string };
}

const container = {
  hidden: {},
  show: { transition: { staggerChildren: 0.08, delayChildren: 0.05 } },
};
const item = {
  hidden: { opacity: 0, y: 16 },
  show: { opacity: 1, y: 0, transition: { type: "spring", stiffness: 260, damping: 24 } },
};

export default function StepOAB({ data, onChange, onNext, onSkip, onBack, errors }: Props) {
  const hasOab = data.oab.trim().length > 0;

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault();
    if (hasOab) {
      onNext();
    } else {
      onSkip();
    }
  };

  return (
    <motion.form
      variants={container}
      initial="hidden"
      animate="show"
      onSubmit={handleSubmit}
      className="flex flex-col h-full px-7 pt-6 pb-8"
    >
      {/* Header */}
      <motion.div variants={item} className="mb-2">
        <h1 className="text-2xl font-semibold tracking-tight">Dados da OAB</h1>
        <p className="mt-1.5 text-sm text-muted">
          Opcional — você pode pular e adicionar depois.
        </p>
      </motion.div>

      {/* Form */}
      <div className="flex-1 flex flex-col justify-center">
        <motion.div variants={item} className="space-y-4">
          <FormInput
            label="Número OAB"
            value={data.oab}
            onChange={(v) => onChange({ oab: v.replace(/\D/g, "") })}
            placeholder="Ex: 36684"
            icon={<Scale size={16} />}
            error={errors?.oab}
            autoFocus
          />

          {hasOab && (
            <motion.div
              initial={{ opacity: 0, height: 0 }}
              animate={{ opacity: 1, height: "auto" }}
              transition={{ type: "spring", stiffness: 300, damping: 25 }}
            >
              <StateSelect
                value={data.oabEstado}
                onChange={(v) => onChange({ oabEstado: v })}
                error={errors?.oabEstado}
              />
            </motion.div>
          )}

          {/* Info box */}
          <motion.div
            variants={item}
            className="flex gap-3 p-4 rounded-2xl bg-accent/[0.08] border border-accent/20"
          >
            <Info size={18} className="text-accent-soft shrink-0 mt-0.5" />
            <p className="text-[13px] leading-relaxed text-white/70">
              Ao informar sua OAB, poderemos{" "}
              <span className="text-accent-soft font-medium">
                sincronizar automaticamente
              </span>{" "}
              todos os processos vinculados ao seu registro profissional.
            </p>
          </motion.div>
        </motion.div>
      </div>

      {/* Buttons */}
      <motion.div variants={item} className="flex gap-3 mt-4">
        <button
          type="button"
          onClick={onBack}
          className="flex-1 py-4 rounded-full glass font-medium text-muted hover:text-white transition-colors"
        >
          Voltar
        </button>
        <motion.button
          type="submit"
          whileHover={{ scale: 1.02 }}
          whileTap={{ scale: 0.97 }}
          className="flex-[2] py-4 rounded-full bg-accent text-white font-semibold shadow-glow"
        >
          {hasOab ? "Continuar" : "Pular"}
        </motion.button>
      </motion.div>
    </motion.form>
  );
}
