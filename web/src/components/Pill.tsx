import { motion } from "framer-motion";

interface PillProps {
  label: string;
  active?: boolean;
  onClick?: () => void;
}

export default function Pill({ label, active, onClick }: PillProps) {
  return (
    <motion.button
      onClick={onClick}
      whileTap={{ scale: 0.94 }}
      whileHover={{ y: -1 }}
      className={[
        "px-3.5 py-1.5 rounded-full text-sm font-medium transition-colors duration-200 select-none",
        active
          ? "bg-accent text-white shadow-glow"
          : "bg-white/[0.05] text-muted hover:text-white hover:bg-white/[0.08]",
      ].join(" ")}
    >
      {label}
    </motion.button>
  );
}
