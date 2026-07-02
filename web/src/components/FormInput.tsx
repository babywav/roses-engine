import { useState } from "react";
import { Eye, EyeOff } from "lucide-react";

interface Props {
  label: string;
  value: string;
  onChange: (v: string) => void;
  type?: "text" | "email" | "password";
  placeholder?: string;
  error?: string;
  icon?: React.ReactNode;
  autoFocus?: boolean;
}

export default function FormInput({
  label,
  value,
  onChange,
  type = "text",
  placeholder,
  error,
  icon,
  autoFocus,
}: Props) {
  const [showPassword, setShowPassword] = useState(false);
  const isPassword = type === "password";
  const inputType = isPassword ? (showPassword ? "text" : "password") : type;

  return (
    <div className="space-y-2">
      <label className="block text-[11px] font-semibold tracking-widest text-muted uppercase">
        {label}
      </label>
      <div className="relative">
        {icon && (
          <div className="absolute left-4 top-1/2 -translate-y-1/2 text-muted pointer-events-none">
            {icon}
          </div>
        )}
        <input
          type={inputType}
          value={value}
          onChange={(e) => onChange(e.target.value)}
          placeholder={placeholder}
          autoFocus={autoFocus}
          className={[
            "w-full glass rounded-2xl px-4 py-3.5 outline-none text-[15px] placeholder:text-muted/50 transition-all duration-200",
            "focus:ring-2 focus:ring-accent/40 focus:border-accent/30",
            icon ? "pl-11" : "",
            error
              ? "ring-2 ring-red-500/40 border-red-500/30"
              : "",
          ].join(" ")}
        />
        {isPassword && (
          <button
            type="button"
            onClick={() => setShowPassword(!showPassword)}
            className="absolute right-3 top-1/2 -translate-y-1/2 text-muted hover:text-white transition-colors p-1"
          >
            {showPassword ? <EyeOff size={16} /> : <Eye size={16} />}
          </button>
        )}
      </div>
      {error && (
        <p className="text-xs text-red-400 pl-1">{error}</p>
      )}
    </div>
  );
}
