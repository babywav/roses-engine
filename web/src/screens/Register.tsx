import { useState, useCallback } from "react";
import { AnimatePresence, motion } from "framer-motion";
import ProgressBar from "../components/ProgressBar";
import RossLogo from "../components/RossLogo";
import StepPersonal from "./register/StepPersonal";
import StepCredentials from "./register/StepCredentials";
import StepPin from "./register/StepPin";
import StepPhoto from "./register/StepPhoto";
import StepOAB from "./register/StepOAB";
import StepSync from "./register/StepSync";
import type { RegisterData } from "../types";

interface Props {
  onComplete: () => void;
}

const EMPTY: RegisterData = {
  nome: "",
  email: "",
  senha: "",
  senhaConfirm: "",
  pin: "",
  foto: "",
  oab: "",
  oabEstado: "",
  sincronizar: false,
};

// nome → email/senha → PIN → foto → OAB → sync
const TOTAL_STEPS = 6;

const slideVariants = {
  enter: (dir: number) => ({ x: dir > 0 ? 280 : -280, opacity: 0 }),
  center: { x: 0, opacity: 1 },
  exit: (dir: number) => ({ x: dir > 0 ? -280 : 280, opacity: 0 }),
};

export default function Register({ onComplete }: Props) {
  const [step, setStep] = useState(0);
  const [dir, setDir] = useState(1);
  const [data, setData] = useState<RegisterData>(EMPTY);
  const [errors, setErrors] = useState<Record<string, string>>({});

  const patch = useCallback(
    (p: Partial<RegisterData>) => setData((d) => ({ ...d, ...p })),
    []
  );

  function goNext() {
    setDir(1);
    setStep((s) => s + 1);
    setErrors({});
  }

  function goBack() {
    setDir(-1);
    setStep((s) => Math.max(0, s - 1));
    setErrors({});
  }

  // --- Validation ---

  function validatePersonal(): boolean {
    if (!data.nome.trim()) {
      setErrors({ nome: "Informe seu nome completo" });
      return false;
    }
    if (data.nome.trim().length < 3) {
      setErrors({ nome: "Nome deve ter pelo menos 3 caracteres" });
      return false;
    }
    return true;
  }

  function validateCredentials(): boolean {
    const errs: Record<string, string> = {};
    if (!data.email.trim()) {
      errs.email = "Informe seu email";
    } else if (!/^[^\s@]+@[^\s@]+\.[^\s@]+$/.test(data.email)) {
      errs.email = "Email inválido";
    }
    if (!data.senha) {
      errs.senha = "Informe uma senha";
    } else if (data.senha.length < 8) {
      errs.senha = "Mínimo de 8 caracteres";
    }
    if (data.senha && data.senhaConfirm !== data.senha) {
      errs.senhaConfirm = "As senhas não coincidem";
    }
    if (Object.keys(errs).length > 0) {
      setErrors(errs);
      return false;
    }
    return true;
  }

  function validatePin(): boolean {
    if (data.pin.replace(/\D/g, "").length < 4) {
      setErrors({ pin: "Digite os 4 dígitos do PIN" });
      return false;
    }
    return true;
  }

  function validateOAB(): boolean {
    if (data.oab.trim() && !data.oabEstado) {
      setErrors({ oabEstado: "Selecione o estado de emissão" });
      return false;
    }
    return true;
  }

  // --- Handlers ---

  const handlePersonalNext = () => validatePersonal() && goNext();
  const handleCredentialsNext = () => validateCredentials() && goNext();
  const handlePinNext = () => validatePin() && goNext();
  const handleOABNext = () => validateOAB() && goNext();
  const handleOABSkip = () => saveAndFinish(false);
  const handleSync = () => saveAndFinish(true);
  const handleSkipSync = () => saveAndFinish(false);

  function saveAndFinish(sync: boolean) {
    const finalData = { ...data, sincronizar: sync };
    localStorage.setItem("roses_user", JSON.stringify(finalData));
    onComplete();
  }

  // Se não informar OAB, o step de sincronização é pulado.
  const visibleSteps = data.oab.trim() ? TOTAL_STEPS : TOTAL_STEPS - 1;
  const progressStep = Math.min(step, visibleSteps);

  return (
    <div className="flex flex-col h-full">
      <div className="px-7 pt-6 pb-2">
        <div className="flex items-center justify-between mb-4">
          <RossLogo iconSize={22} fontSize={14} color="rgba(255,255,255,0.55)" />
        </div>
        <ProgressBar current={progressStep} total={visibleSteps} />
      </div>

      <div className="flex-1 relative overflow-hidden">
        <AnimatePresence mode="wait" custom={dir}>
          <motion.div
            key={step}
            custom={dir}
            variants={slideVariants}
            initial="enter"
            animate="center"
            exit="exit"
            transition={{ type: "spring", stiffness: 300, damping: 30 }}
            className="absolute inset-0"
          >
            {step === 0 && (
              <StepPersonal data={data} onChange={patch} onNext={handlePersonalNext} error={errors.nome} />
            )}
            {step === 1 && (
              <StepCredentials
                data={data}
                onChange={patch}
                onNext={handleCredentialsNext}
                onBack={goBack}
                errors={{ email: errors.email, senha: errors.senha, senhaConfirm: errors.senhaConfirm }}
              />
            )}
            {step === 2 && (
              <StepPin data={data} onChange={patch} onNext={handlePinNext} onBack={goBack} error={errors.pin} />
            )}
            {step === 3 && (
              <StepPhoto data={data} onChange={patch} onNext={goNext} onBack={goBack} />
            )}
            {step === 4 && (
              <StepOAB
                data={data}
                onChange={patch}
                onNext={handleOABNext}
                onSkip={handleOABSkip}
                onBack={goBack}
                errors={{ oab: errors.oab, oabEstado: errors.oabEstado }}
              />
            )}
            {step === 5 && (
              <StepSync data={data} onSync={handleSync} onSkip={handleSkipSync} />
            )}
          </motion.div>
        </AnimatePresence>
      </div>
    </div>
  );
}
