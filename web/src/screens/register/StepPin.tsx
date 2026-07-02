import { useRef } from "react";
import { motion } from "framer-motion";
import { ShieldCheck } from "lucide-react";
import type { RegisterData } from "../../types";

interface Props {
  data: RegisterData;
  onChange: (patch: Partial<RegisterData>) => void;
  onNext: () => void;
  onBack: () => void;
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

const LEN = 4;

export default function StepPin({ data, onChange, onNext, onBack, error }: Props) {
  const refs = useRef<Array<HTMLInputElement | null>>([]);
  const digits = data.pin.padEnd(LEN, " ").slice(0, LEN).split("");

  function setDigit(i: number, v: string) {
    const clean = v.replace(/\D/g, "").slice(-1);
    const arr = data.pin.padEnd(LEN, " ").slice(0, LEN).split("");
    arr[i] = clean || " ";
    onChange({ pin: arr.join("").replace(/\s/g, "") });
    if (clean && i < LEN - 1) refs.current[i + 1]?.focus();
  }

  function onKey(i: number, e: React.KeyboardEvent<HTMLInputElement>) {
    if (e.key === "Backspace" && !digits[i].trim() && i > 0) refs.current[i - 1]?.focus();
  }

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
      <motion.div variants={item} className="flex-1 flex flex-col items-center justify-center">
        <div className="relative mb-8">
          <motion.div
            className="absolute -inset-8 rounded-full bg-accent/20 blur-2xl"
            animate={{ opacity: [0.3, 0.6, 0.3] }}
            transition={{ duration: 4, repeat: Infinity }}
          />
          <div className="relative grid place-items-center w-24 h-24 rounded-[1.8rem] glass">
            <ShieldCheck size={44} strokeWidth={1.4} className="text-accent-soft" />
          </div>
        </div>

        <motion.h1 variants={item} className="text-2xl font-semibold tracking-tight text-center">
          Crie um PIN
        </motion.h1>
        <motion.p variants={item} className="mt-2 text-sm text-muted text-center max-w-[280px]">
          Um código de {LEN} dígitos para acesso rápido e seguro ao app.
        </motion.p>

        <motion.div variants={item} className="mt-8 flex gap-3">
          {digits.map((d, i) => (
            <input
              key={i}
              ref={(el) => { refs.current[i] = el; }}
              inputMode="numeric"
              maxLength={1}
              value={d.trim()}
              onChange={(e) => setDigit(i, e.target.value)}
              onKeyDown={(e) => onKey(i, e)}
              autoFocus={i === 0}
              className={[
                "w-14 h-16 text-center text-2xl font-mono font-semibold glass rounded-2xl outline-none transition-all",
                "focus:ring-2 focus:ring-accent/50 focus:border-accent/40",
                error ? "ring-2 ring-red-500/40" : "",
              ].join(" ")}
            />
          ))}
        </motion.div>
        {error && <p className="mt-3 text-xs text-red-400">{error}</p>}
      </motion.div>

      <motion.div variants={item} className="flex gap-3">
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
