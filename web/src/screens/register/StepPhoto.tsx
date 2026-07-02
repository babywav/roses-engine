import { useRef } from "react";
import { motion } from "framer-motion";
import { Camera, User } from "lucide-react";
import type { RegisterData } from "../../types";

interface Props {
  data: RegisterData;
  onChange: (patch: Partial<RegisterData>) => void;
  onNext: () => void;
  onBack: () => void;
}

const container = {
  hidden: {},
  show: { transition: { staggerChildren: 0.1, delayChildren: 0.05 } },
};
const item = {
  hidden: { opacity: 0, y: 16 },
  show: { opacity: 1, y: 0, transition: { type: "spring", stiffness: 260, damping: 24 } },
};

export default function StepPhoto({ data, onChange, onNext, onBack }: Props) {
  const fileRef = useRef<HTMLInputElement | null>(null);

  function pickFile(e: React.ChangeEvent<HTMLInputElement>) {
    const file = e.target.files?.[0];
    if (!file) return;
    const reader = new FileReader();
    reader.onload = () => onChange({ foto: String(reader.result || "") });
    reader.readAsDataURL(file);
  }

  return (
    <motion.div
      variants={container}
      initial="hidden"
      animate="show"
      className="flex flex-col h-full px-7 pt-6 pb-8"
    >
      <motion.div variants={item} className="mb-2">
        <h1 className="text-2xl font-semibold tracking-tight">Foto de perfil</h1>
        <p className="mt-1.5 text-sm text-muted">Opcional — dá um rosto à sua conta.</p>
      </motion.div>

      <div className="flex-1 flex flex-col items-center justify-center">
        <motion.button
          type="button"
          variants={item}
          whileHover={{ scale: 1.03 }}
          whileTap={{ scale: 0.97 }}
          onClick={() => fileRef.current?.click()}
          className="relative group"
        >
          <div className="absolute -inset-6 rounded-full bg-accent/20 blur-2xl opacity-60" />
          <div className="relative w-36 h-36 rounded-full glass grid place-items-center overflow-hidden border border-white/10">
            {data.foto ? (
              <img src={data.foto} alt="Prévia" className="w-full h-full object-cover" />
            ) : (
              <User size={56} strokeWidth={1.3} className="text-white/40" />
            )}
          </div>
          <div className="absolute bottom-1 right-1 grid place-items-center w-11 h-11 rounded-full bg-accent text-white shadow-glow border-4 border-ink-800">
            <Camera size={18} />
          </div>
        </motion.button>

        <motion.p variants={item} className="mt-6 text-sm text-muted">
          {data.foto ? "Toque para trocar a foto" : "Toque para adicionar uma foto"}
        </motion.p>

        <input ref={fileRef} type="file" accept="image/*" onChange={pickFile} className="hidden" />
      </div>

      <motion.div variants={item} className="flex gap-3">
        <button
          type="button"
          onClick={onBack}
          className="flex-1 py-4 rounded-full glass font-medium text-muted hover:text-white transition-colors"
        >
          Voltar
        </button>
        <motion.button
          type="button"
          onClick={onNext}
          whileHover={{ scale: 1.02 }}
          whileTap={{ scale: 0.97 }}
          className="flex-[2] py-4 rounded-full bg-accent text-white font-semibold shadow-glow"
        >
          {data.foto ? "Continuar" : "Pular"}
        </motion.button>
      </motion.div>
    </motion.div>
  );
}
