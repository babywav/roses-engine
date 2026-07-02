import { motion } from "framer-motion";
import { Check } from "lucide-react";

interface Props {
  current: number;
  total: number;
}

export default function ProgressBar({ current, total }: Props) {
  return (
    <div className="flex items-center justify-center gap-3">
      {Array.from({ length: total }, (_, i) => {
        const isCompleted = i < current;
        const isActive = i === current;
        return (
          <div key={i} className="flex items-center gap-3">
            {/* Dot */}
            <motion.div
              initial={false}
              animate={{
                scale: isActive ? 1 : 0.85,
                backgroundColor: isCompleted
                  ? "#3B82F6"
                  : isActive
                    ? "#3B82F6"
                    : "rgba(255,255,255,0.1)",
              }}
              transition={{ type: "spring", stiffness: 400, damping: 28 }}
              className="relative grid place-items-center w-8 h-8 rounded-full"
            >
              {isCompleted ? (
                <motion.div
                  initial={{ scale: 0 }}
                  animate={{ scale: 1 }}
                  transition={{ type: "spring", stiffness: 500, damping: 24 }}
                >
                  <Check size={14} strokeWidth={3} className="text-white" />
                </motion.div>
              ) : (
                <span
                  className={[
                    "text-xs font-semibold",
                    isActive ? "text-white" : "text-white/40",
                  ].join(" ")}
                >
                  {i + 1}
                </span>
              )}
              {isActive && (
                <motion.div
                  layoutId="progress-glow"
                  className="absolute inset-0 rounded-full bg-accent/30 blur-md"
                  transition={{ type: "spring", stiffness: 300, damping: 25 }}
                />
              )}
            </motion.div>
            {/* Connector line */}
            {i < total - 1 && (
              <motion.div
                initial={false}
                animate={{
                  backgroundColor: isCompleted
                    ? "rgba(59,130,246,0.5)"
                    : "rgba(255,255,255,0.08)",
                }}
                className="w-8 h-0.5 rounded-full"
              />
            )}
          </div>
        );
      })}
    </div>
  );
}
