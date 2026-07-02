import { motion } from "framer-motion";
import { Mail, Lock } from "lucide-react";
import FormInput from "../../components/FormInput";
import type { RegisterData } from "../../types";

interface Props {
  data: RegisterData;
  onChange: (patch: Partial<RegisterData>) => void;
  onNext: () => void;
  onBack: () => void;
  errors?: { email?: string; senha?: string; senhaConfirm?: string };
}

const container = {
  hidden: {},
  show: { transition: { staggerChildren: 0.08, delayChildren: 0.05 } },
};
const item = {
  hidden: { opacity: 0, y: 16 },
  show: { opacity: 1, y: 0, transition: { type: "spring", stiffness: 260, damping: 24 } },
};

export default function StepCredentials({ data, onChange, onNext, onBack, errors }: Props) {
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
      {/* Header */}
      <motion.div variants={item} className="mb-2">
        <h1 className="text-2xl font-semibold tracking-tight">Crie sua conta</h1>
        <p className="mt-1.5 text-sm text-muted">
          Seus dados ficam seguros e protegidos.
        </p>
      </motion.div>

      {/* Form */}
      <div className="flex-1 flex flex-col justify-center">
        <motion.div variants={item} className="space-y-4">
          <FormInput
            label="Email"
            type="email"
            value={data.email}
            onChange={(v) => onChange({ email: v })}
            placeholder="seu@email.com"
            icon={<Mail size={16} />}
            error={errors?.email}
            autoFocus
          />
          <FormInput
            label="Senha"
            type="password"
            value={data.senha}
            onChange={(v) => onChange({ senha: v })}
            placeholder="Mínimo 8 caracteres"
            icon={<Lock size={16} />}
            error={errors?.senha}
          />
          <FormInput
            label="Confirmar senha"
            type="password"
            value={data.senhaConfirm}
            onChange={(v) => onChange({ senhaConfirm: v })}
            placeholder="Repita a senha"
            icon={<Lock size={16} />}
            error={errors?.senhaConfirm}
          />

          {/* Password strength hint */}
          {data.senha.length > 0 && (
            <motion.div
              initial={{ opacity: 0, height: 0 }}
              animate={{ opacity: 1, height: "auto" }}
              className="flex items-center gap-2"
            >
              {[1, 2, 3, 4].map((level) => (
                <div
                  key={level}
                  className={[
                    "h-1 flex-1 rounded-full transition-colors duration-300",
                    data.senha.length >= level * 3
                      ? level <= 2
                        ? "bg-amber-500"
                        : "bg-emerald-500"
                      : "bg-white/10",
                  ].join(" ")}
                />
              ))}
              <span className="text-[11px] text-muted ml-1">
                {data.senha.length < 6
                  ? "Fraca"
                  : data.senha.length < 10
                    ? "Média"
                    : "Forte"}
              </span>
            </motion.div>
          )}
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
          Continuar
        </motion.button>
      </motion.div>
    </motion.form>
  );
}
