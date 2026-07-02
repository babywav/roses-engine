import { useEffect, useRef, useState } from "react";
import { motion, AnimatePresence } from "framer-motion";
import { Plus, X, ArrowUp, Mic, Paperclip, MoreHorizontal, ChevronDown, Image as ImageIcon, Video } from "lucide-react";
import { consultar, detectIntent } from "../api";
import type { Process, QuerySettings } from "../types";
import ProcessCard from "../components/ProcessCard";

interface Msg {
  id: number;
  role: "user" | "assistant";
  text?: string;
  processos?: Process[];
  loading?: boolean;
  files?: UploadedFile[];
}

interface UploadedFile {
  name: string;
  size: number;
  type: string;
  url: string;
}

interface Props {
  settings: QuerySettings;
  onOpenSettings: () => void;
}

const SUGGESTIONS = ["0000166-95.1997.8.15.0211", "OAB 14233 PB", "Maria Silva PB"];

export default function Chat({ settings, onOpenSettings }: Props) {
  const [isMenuOpen, setIsMenuOpen] = useState(false);
  const [msgs, setMsgs] = useState<Msg[]>([]);
  const [input, setInput] = useState("");
  const [busy, setBusy] = useState(false);
  const [pendingFiles, setPendingFiles] = useState<UploadedFile[]>([]);
  const [speakingId, setSpeakingId] = useState<number | null>(null);
  const [ttsMode, setTtsMode] = useState(false);
  const msgsRef = useRef<Msg[]>([]);

  const textareaRef = useRef<HTMLTextAreaElement>(null);
  const mainRef = useRef<HTMLDivElement>(null);
  const menuRef = useRef<HTMLDivElement>(null);
  const fileInputRef = useRef<HTMLInputElement>(null);

  const hasStarted = msgs.length > 0;

  // Keep ref in sync for TTS callbacks
  useEffect(() => { msgsRef.current = msgs; }, [msgs]);

  // Auto-scroll
  useEffect(() => {
    if (mainRef.current) mainRef.current.scrollTop = mainRef.current.scrollHeight;
  }, [msgs]);

  // Focus on mount
  useEffect(() => {
    textareaRef.current?.focus();
  }, []);

  // Close menu on outside click
  useEffect(() => {
    function onClickOutside(e: MouseEvent) {
      if (menuRef.current && !menuRef.current.contains(e.target as Node)) {
        setIsMenuOpen(false);
      }
    }
    document.addEventListener("mousedown", onClickOutside);
    return () => document.removeEventListener("mousedown", onClickOutside);
  }, []);

  // Auto-resize textarea
  const handleInput = (e: React.ChangeEvent<HTMLTextAreaElement>) => {
    setInput(e.target.value);
    const ta = e.target;
    ta.style.height = "auto";
    ta.style.height = `${Math.min(ta.scrollHeight, 128)}px`;
  };

  // Handle file selection
  const handleFileSelect = (e: React.ChangeEvent<HTMLInputElement>) => {
    const files = Array.from(e.target.files || []);
    const uploaded: UploadedFile[] = files.map((f) => ({
      name: f.name,
      size: f.size,
      type: f.type,
      url: URL.createObjectURL(f),
    }));
    setPendingFiles((prev) => [...prev, ...uploaded]);
    setIsMenuOpen(false);
    // reset input so same file can be selected again
    e.target.value = "";
  };

  async function send(raw?: string) {
    const text = (raw ?? input).trim();
    if ((!text && pendingFiles.length === 0) || busy) return;
    setInput("");
    if (textareaRef.current) {
      textareaRef.current.style.height = "auto";
    }
    const files = [...pendingFiles];
    setPendingFiles([]);
    setIsMenuOpen(false);
    setBusy(true);

    const userId = Date.now();
    const loadingId = userId + 1;
    setMsgs((m) => [
      ...m,
      { id: userId, role: "user", text: text || undefined, files: files.length > 0 ? files : undefined },
      { id: loadingId, role: "assistant", loading: true },
    ]);

    try {
      const intent = detectIntent(text);
      const uf = intent.uf || settings.ufPadrao || "";
      const res = await consultar(intent.tipo, intent.valor, uf);
      const resumo =
        res.status === "OK"
          ? `Encontrei ${res.total} processo(s) em ${res.tribunal} via ${res.fonte ?? "motor"}.`
          : res.status === "NOT_FOUND"
            ? "Não encontrei processos para esse critério."
            : `Não consegui concluir: ${res.message}`;
      setMsgs((m) =>
        m.map((x) =>
          x.id === loadingId
            ? { ...x, loading: false, text: resumo, processos: res.processos }
            : x,
        ),
      );
    } catch {
      setMsgs((m) =>
        m.map((x) =>
          x.id === loadingId
            ? { ...x, loading: false, text: "Erro ao consultar. Verifique se o backend está rodando." }
            : x,
        ),
      );
    } finally {
      setBusy(false);
    }
  }

  // ── TTS ─────────────────────────────────────────────────
  function speakMsg(msg: Msg) {
    if (!msg.text || !window.speechSynthesis) return;
    window.speechSynthesis.cancel();
    const utter = new SpeechSynthesisUtterance(msg.text);
    utter.lang = "pt-BR";
    utter.rate = 1.05;
    setSpeakingId(msg.id);
    setTtsMode(true);
    utter.onend = () => {
      setSpeakingId(null);
      // auto-play next assistant message
      const all = msgsRef.current;
      const idx = all.findIndex((x) => x.id === msg.id);
      const next = all.slice(idx + 1).find((x) => x.role === "assistant" && x.text && !x.loading);
      if (next) speakMsg(next);
      else setTtsMode(false);
    };
    window.speechSynthesis.speak(utter);
  }

  function stopTTS() {
    window.speechSynthesis.cancel();
    setSpeakingId(null);
    setTtsMode(false);
  }

  // Auto-play new assistant messages when ttsMode is on
  useEffect(() => {
    if (!ttsMode || speakingId !== null) return;
    const filtered = msgs.filter((m) => m.role === "assistant" && m.text && !m.loading);
    const last = filtered.length > 0 ? filtered[filtered.length - 1] : undefined;
    if (last) speakMsg(last);
  // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [msgs, ttsMode]);

  const canSend = (input.trim().length > 0 || pendingFiles.length > 0) && !busy;

  return (
    <div
      style={{
        background: "#0b1121",
        position: "relative",
        overflow: "hidden",
        display: "flex",
        flexDirection: "column",
        height: "100%",
        fontFamily: "-apple-system, BlinkMacSystemFont, 'Inter', system-ui, sans-serif",
        WebkitFontSmoothing: "antialiased",
      }}
    >
      {/* Background glow */}
      <div
        style={{
          position: "absolute",
          inset: 0,
          pointerEvents: "none",
          zIndex: 0,
          background:
            "radial-gradient(circle at 50% 40%, rgba(25,45,90,0.4) 0%, rgba(15,25,55,0.3) 40%, rgba(10,13,26,0) 70%)",
        }}
      />

      {/* Hidden file input */}
      <input
        ref={fileInputRef}
        type="file"
        multiple
        style={{ display: "none" }}
        onChange={handleFileSelect}
      />

      {/* Scroll area */}
      <div
        ref={mainRef}
        style={{
          flex: 1,
          overflowY: "auto",
          display: "flex",
          flexDirection: "column",
          alignItems: "center",
          width: "100%",
          padding: hasStarted ? "24px 16px 160px" : "0 16px 220px",
          position: "relative",
          zIndex: 10,
        }}
      >
        {/* Welcome */}
        <AnimatePresence>
          {!hasStarted && (
            <motion.div
              key="welcome"
              initial={{ opacity: 0 }}
              animate={{ opacity: 1 }}
              exit={{ opacity: 0 }}
              transition={{ duration: 0.4 }}
              style={{
                flex: 1,
                display: "flex",
                flexDirection: "column",
                alignItems: "center",
                justifyContent: "center",
                width: "100%",
                minHeight: "55vh",
              }}
            >
              <h1
                style={{
                  fontSize: "clamp(24px, 5vw, 42px)",
                  fontWeight: 300,
                  color: "#fff",
                  textAlign: "center",
                  letterSpacing: "0.01em",
                  lineHeight: 1.2,
                }}
              >
                {input.trim().length > 0 ? "Alguma sugestão fora da caixa?" : "Sua vez!"}
              </h1>
            </motion.div>
          )}
        </AnimatePresence>

        {/* Messages */}
        {hasStarted && (
          <div style={{ width: "100%", maxWidth: 768, display: "flex", flexDirection: "column", gap: 24, paddingTop: 8 }}>
            <AnimatePresence initial={false}>
              {msgs.map((m) => (
                <motion.div
                  key={m.id}
                  initial={{ opacity: 0, y: 14 }}
                  animate={{ opacity: 1, y: 0 }}
                  transition={{ duration: 0.35, ease: [0.22, 0.8, 0.36, 1] }}
                  style={{
                    display: "flex",
                    width: "100%",
                    justifyContent: m.role === "user" ? "flex-end" : "flex-start",
                  }}
                >
                  <div style={{ maxWidth: m.role === "user" ? "85%" : "100%", width: m.role === "assistant" ? "100%" : undefined }}>
                    {/* File attachments */}
                    {m.files && m.files.length > 0 && (
                      <div style={{ display: "flex", flexWrap: "wrap", gap: 8, marginBottom: 8, justifyContent: "flex-end" }}>
                        {m.files.map((f, i) => (
                          <div
                            key={i}
                            style={{
                              background: "rgba(255,255,255,0.08)",
                              border: "1px solid rgba(255,255,255,0.1)",
                              borderRadius: 12,
                              padding: "8px 12px",
                              display: "flex",
                              alignItems: "center",
                              gap: 8,
                              fontSize: 13,
                              color: "rgba(255,255,255,0.7)",
                            }}
                          >
                            <Paperclip size={14} />
                            <span style={{ maxWidth: 160, overflow: "hidden", textOverflow: "ellipsis", whiteSpace: "nowrap" }}>{f.name}</span>
                          </div>
                        ))}
                      </div>
                    )}
                    {m.text && (
                      <div>
                        <div
                          style={{
                            background: m.role === "user" ? "#2a2f3a" : "transparent",
                            borderRadius: m.role === "user" ? "18px 18px 4px 18px" : 0,
                            padding: m.role === "user" ? "12px 18px" : "4px 0",
                            fontSize: 15,
                            lineHeight: 1.65,
                            color: m.role === "user" ? "rgba(255,255,255,0.9)" : "rgba(255,255,255,0.78)",
                          }}
                        >
                          {m.text}
                        </div>
                        {/* TTS button — only on assistant messages */}
                        {m.role === "assistant" && (
                          <div style={{ marginTop: 8, display: "flex" }}>
                            {speakingId === m.id ? (
                              <button
                                onClick={stopTTS}
                                title="Parar leitura"
                                style={ttsButtonStyle}
                              >
                                {/* Stop square */}
                                <svg width="14" height="14" viewBox="0 0 14 14" fill="none">
                                  <rect x="2" y="2" width="10" height="10" rx="2" fill="currentColor"/>
                                </svg>
                              </button>
                            ) : (
                              <button
                                onClick={() => speakMsg(m)}
                                title="Ouvir mensagem"
                                style={ttsButtonStyle}
                              >
                                {/* Play triangle */}
                                <svg width="14" height="14" viewBox="0 0 14 14" fill="none">
                                  <path d="M3 2.5 L11.5 7 L3 11.5 Z" fill="currentColor"/>
                                </svg>
                              </button>
                            )}
                          </div>
                        )}
                      </div>
                    )}
                    {m.loading && <ThinkingAnimation />}
                    {m.processos && m.processos.length > 0 && (
                      <div style={{ marginTop: 12, display: "flex", flexDirection: "column", gap: 10 }}>
                        {m.processos.map((p, i) => (
                          <ProcessCard key={p.numero + i} process={p} index={i} />
                        ))}
                      </div>
                    )}
                  </div>
                </motion.div>
              ))}
            </AnimatePresence>
          </div>
        )}
      </div>

      {/* Input bar — absolute positioned, animates from center to bottom */}
      <div
        style={{
          position: "absolute",
          left: 0,
          right: 0,
          zIndex: 20,
          bottom: hasStarted ? 0 : "35%",
          display: "flex",
          justifyContent: "center",
          padding: "0 16px 16px",
          transition: "bottom 0.7s cubic-bezier(.22,.8,.36,1)",
        }}
      >
        <div ref={menuRef} style={{ position: "relative", width: "100%", maxWidth: 768 }}>

          {/* Dropdown menu */}
          <div
            style={{
              position: "absolute",
              bottom: "calc(100% + 12px)",
              left: 0,
              background: "#1f232b",
              borderRadius: 16,
              padding: "8px 0",
              minWidth: 240,
              border: "1px solid rgba(255,255,255,0.06)",
              boxShadow: "0 10px 25px rgba(0,0,0,0.55)",
              transition: "opacity 0.18s, transform 0.18s",
              transformOrigin: "bottom left",
              opacity: isMenuOpen ? 1 : 0,
              transform: isMenuOpen ? "scale(1)" : "scale(0.95)",
              pointerEvents: isMenuOpen ? "auto" : "none",
            }}
          >
            <button
              onClick={() => fileInputRef.current?.click()}
              style={menuItemStyle}
              onMouseEnter={hoverOn}
              onMouseLeave={hoverOff}
            >
              <Paperclip size={18} style={{ color: "rgba(255,255,255,0.5)", marginRight: 12 }} />
              Enviar arquivos
            </button>
            <button style={{ ...menuItemStyle, justifyContent: "space-between" }} onMouseEnter={hoverOn} onMouseLeave={hoverOff}>
              <span style={{ display: "flex", alignItems: "center" }}>
                <MoreHorizontal size={18} style={{ color: "rgba(255,255,255,0.5)", marginRight: 12 }} />
                Mais uploads
              </span>
              <ChevronDown size={14} style={{ opacity: 0.5, transform: "rotate(-90deg)" }} />
            </button>
            <div style={{ height: 1, background: "rgba(255,255,255,0.08)", margin: "4px 16px" }} />
            <button style={menuItemStyle} onMouseEnter={hoverOn} onMouseLeave={hoverOff}>
              <ImageIcon size={18} style={{ color: "rgba(255,255,255,0.5)", marginRight: 12 }} />
              Criar imagem
            </button>
            <button style={{ ...menuItemStyle, color: "rgba(255,255,255,0.28)", cursor: "not-allowed" }}>
              <Video size={18} style={{ color: "rgba(255,255,255,0.2)", marginRight: 12 }} />
              Criar vídeo
            </button>
          </div>

          {/* Pending file chips */}
          <AnimatePresence>
            {pendingFiles.length > 0 && (
              <motion.div
                initial={{ opacity: 0, y: 8 }}
                animate={{ opacity: 1, y: 0 }}
                exit={{ opacity: 0, y: 8 }}
                style={{ display: "flex", flexWrap: "wrap", gap: 8, marginBottom: 10 }}
              >
                {pendingFiles.map((f, i) => (
                  <div
                    key={i}
                    style={{
                      background: "rgba(255,255,255,0.07)",
                      border: "1px solid rgba(255,255,255,0.1)",
                      borderRadius: 20,
                      padding: "6px 12px",
                      display: "flex",
                      alignItems: "center",
                      gap: 8,
                      fontSize: 13,
                      color: "rgba(255,255,255,0.7)",
                    }}
                  >
                    <Paperclip size={13} />
                    <span style={{ maxWidth: 160, overflow: "hidden", textOverflow: "ellipsis", whiteSpace: "nowrap" }}>{f.name}</span>
                    <button
                      onClick={() => setPendingFiles((prev) => prev.filter((_, idx) => idx !== i))}
                      style={{ background: "none", border: "none", cursor: "pointer", color: "rgba(255,255,255,0.4)", padding: 0, display: "flex" }}
                    >
                      <X size={13} />
                    </button>
                  </div>
                ))}
              </motion.div>
            )}
          </AnimatePresence>

          {/* Suggestions */}
          <AnimatePresence>
            {!hasStarted && pendingFiles.length === 0 && (
              <motion.div
                initial={{ opacity: 0, y: 8 }}
                animate={{ opacity: 1, y: 0 }}
                exit={{ opacity: 0, y: 8 }}
                transition={{ duration: 0.3 }}
                style={{ display: "flex", flexWrap: "wrap", gap: 8, marginBottom: 12, justifyContent: "center" }}
              >
                {SUGGESTIONS.map((s) => (
                  <button
                    key={s}
                    onClick={() => send(s)}
                    style={suggestionStyle}
                    onMouseEnter={(e) => { e.currentTarget.style.color = "rgba(255,255,255,0.9)"; e.currentTarget.style.background = "rgba(255,255,255,0.1)"; }}
                    onMouseLeave={(e) => { e.currentTarget.style.color = "rgba(255,255,255,0.5)"; e.currentTarget.style.background = "rgba(255,255,255,0.06)"; }}
                  >
                    {s}
                  </button>
                ))}
              </motion.div>
            )}
          </AnimatePresence>

          {/* Input pill */}
          <div
            style={{
              background: "rgba(30,35,45,0.85)",
              backdropFilter: "blur(24px)",
              WebkitBackdropFilter: "blur(24px)",
              border: "1px solid rgba(255,255,255,0.1)",
              borderRadius: 9999,
              display: "flex",
              alignItems: "flex-end",
              padding: "10px 12px",
              boxShadow: "0 4px 24px rgba(0,0,0,0.4)",
              transition: "border-color 0.25s, background 0.25s",
            }}
          >
            {/* + / X */}
            <button
              onClick={() => setIsMenuOpen(!isMenuOpen)}
              style={{
                width: 40,
                height: 40,
                borderRadius: "50%",
                background: isMenuOpen ? "rgba(255,255,255,0.08)" : "transparent",
                display: "flex",
                alignItems: "center",
                justifyContent: "center",
                color: isMenuOpen ? "#fff" : "rgba(255,255,255,0.55)",
                flexShrink: 0,
                marginBottom: 2,
                transition: "background 0.2s, color 0.2s",
                border: "none",
                cursor: "pointer",
              }}
            >
              {isMenuOpen ? <X size={20} /> : <Plus size={20} />}
            </button>

            {/* Textarea */}
            <textarea
              ref={textareaRef}
              value={input}
              onChange={handleInput}
              onKeyDown={(e) => {
                if (e.key === "Enter" && !e.shiftKey) {
                  e.preventDefault();
                  send();
                }
              }}
              placeholder="Peça ao Ross"
              rows={1}
              style={{
                flex: 1,
                background: "transparent",
                border: "none",
                color: "rgba(255,255,255,0.9)",
                fontSize: 15,
                padding: "10px 8px",
                resize: "none",
                maxHeight: 128,
                overflowY: "auto",
                lineHeight: 1.5,
                outline: "none",
                fontFamily: "inherit",
                margin: "0 4px",
                minHeight: 40,
              }}
            />

            {/* Send / Mic */}
            <div style={{ display: "flex", alignItems: "center", gap: 4, marginBottom: 2, flexShrink: 0 }}>
              {canSend ? (
                <motion.button
                  key="send"
                  initial={{ scale: 0, opacity: 0 }}
                  animate={{ scale: 1, opacity: 1 }}
                  exit={{ scale: 0, opacity: 0 }}
                  transition={{ type: "spring", stiffness: 400, damping: 30 }}
                  onClick={() => send()}
                  disabled={busy}
                  style={{
                    width: 36,
                    height: 36,
                    borderRadius: "50%",
                    background: "#1a56db",
                    display: "flex",
                    alignItems: "center",
                    justifyContent: "center",
                    color: "#fff",
                    boxShadow: "0 2px 8px rgba(26,86,219,0.5)",
                    border: "none",
                    cursor: busy ? "not-allowed" : "pointer",
                    opacity: busy ? 0.5 : 1,
                  }}
                >
                  <ArrowUp size={18} strokeWidth={2.5} />
                </motion.button>
              ) : (
                <button
                  style={{
                    width: 40,
                    height: 40,
                    borderRadius: "50%",
                    display: "flex",
                    alignItems: "center",
                    justifyContent: "center",
                    color: "rgba(255,255,255,0.45)",
                    background: "none",
                    border: "none",
                    cursor: "pointer",
                    transition: "color 0.2s",
                  }}
                  onMouseEnter={(e) => { e.currentTarget.style.color = "rgba(255,255,255,0.8)"; }}
                  onMouseLeave={(e) => { e.currentTarget.style.color = "rgba(255,255,255,0.45)"; }}
                >
                  <Mic size={18} />
                </button>
              )}
            </div>
          </div>

          <div style={{ textAlign: "center", marginTop: 10, fontSize: 11, color: "rgba(255,255,255,0.22)", letterSpacing: "0.01em" }}>
            A IA pode cometer erros. Considere verificar informações importantes.
          </div>
        </div>
      </div>
    </div>
  );
}

// ── Styles ────────────────────────────────────────────────

const ttsButtonStyle: React.CSSProperties = {
  display: "flex",
  alignItems: "center",
  justifyContent: "center",
  width: 28,
  height: 28,
  borderRadius: "50%",
  background: "rgba(255,255,255,0.05)",
  border: "1px solid rgba(255,255,255,0.08)",
  color: "rgba(255,255,255,0.35)",
  cursor: "pointer",
  transition: "background 0.15s, color 0.15s",
};

const menuItemStyle: React.CSSProperties = {
  display: "flex",
  alignItems: "center",
  padding: "12px 20px",
  fontSize: 14,
  color: "rgba(255,255,255,0.7)",
  width: "100%",
  background: "none",
  border: "none",
  cursor: "pointer",
  fontFamily: "inherit",
  textAlign: "left",
  transition: "background 0.15s",
};

const suggestionStyle: React.CSSProperties = {
  padding: "7px 14px",
  borderRadius: 9999,
  background: "rgba(255,255,255,0.06)",
  border: "1px solid rgba(255,255,255,0.08)",
  color: "rgba(255,255,255,0.5)",
  fontSize: 13,
  cursor: "pointer",
  fontFamily: "inherit",
  transition: "color 0.2s, background 0.2s",
};

function hoverOn(e: React.MouseEvent<HTMLButtonElement>) {
  e.currentTarget.style.background = "rgba(255,255,255,0.05)";
}
function hoverOff(e: React.MouseEvent<HTMLButtonElement>) {
  e.currentTarget.style.background = "none";
}

// ── Thinking animation ────────────────────────────────────

const THINKING_PHRASES = [
  "Pensando...",
  "Analisando o contexto...",
  "Lendo nas entrelinhas...",
  "Formulando a melhor resposta...",
  "Quase lá...",
];

function ThinkingAnimation() {
  const [phraseIndex, setPhraseIndex] = useState(0);
  const [opacity, setOpacity] = useState(1);

  useEffect(() => {
    const interval = setInterval(() => {
      setOpacity(0);
      setTimeout(() => {
        setPhraseIndex((prev) => (prev + 1) % THINKING_PHRASES.length);
        setOpacity(1);
      }, 600);
    }, 3000);
    return () => clearInterval(interval);
  }, []);

  return (
    <div style={{ display: "flex", flexDirection: "column", alignItems: "center", gap: 24, padding: "40px 0", width: "100%" }}>
      <div style={{ position: "relative", display: "flex", alignItems: "center", justifyContent: "center" }}>
        <div style={{ position: "absolute", width: 86, height: 86, borderRadius: "50%", background: "#1a56db", opacity: 0.28, filter: "blur(18px)" }} />
        <div className="animate-star" style={{ position: "relative", display: "flex", alignItems: "center", justifyContent: "center" }}>
          <svg width="48" height="48" viewBox="0 0 100 100" style={{ overflow: "visible" }}>
            <g transform="translate(50, 50)">
              {[0, 45, 90, 135, 180, 225, 270, 315].map((angle, i) => (
                <path
                  key={i}
                  d="M 0 6 C 2.5 6, 4.2 -10, 2.5 -38 C 1.8 -43, -1.8 -43, -2.5 -38 C -4.2 -10, -2.5 6, 0 6 Z"
                  fill="#1a56db"
                  transform={`rotate(${angle}) scale(${i % 2 === 0 ? "1, 1" : "0.88, 0.85"})`}
                />
              ))}
            </g>
          </svg>
        </div>
      </div>
      <div style={{ height: 24, display: "flex", alignItems: "center", justifyContent: "center" }}>
        <span style={{ color: "rgba(255,255,255,0.45)", fontSize: 14, letterSpacing: "0.04em", transition: "opacity 0.5s ease", opacity }}>
          {THINKING_PHRASES[phraseIndex]}
        </span>
      </div>
    </div>
  );
}
