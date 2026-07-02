import { useState } from "react";
import { AnimatePresence, motion } from "framer-motion";
import Onboarding from "./screens/Onboarding";
import Register from "./screens/Register";
import Chat from "./screens/Chat";
import CalculosTab from "./screens/tabs/CalculosTab";
import VigilanciaTab from "./screens/tabs/VigilanciaTab";
import PortalTab from "./screens/tabs/PortalTab";
import AuditoriaTab from "./screens/tabs/AuditoriaTab";
import SettingsPanel from "./components/SettingsPanel";
import BottomNav, { type AppTab } from "./components/BottomNav";
import type { QuerySettings } from "./types";

type Screen = "onboarding" | "register" | "app";

const DEFAULT_SETTINGS: QuerySettings = {
  fonte: "auto",
  saida: "lista",
  incluirMovimentacoes: true,
  ufPadrao: "PB",
};

export default function App() {
  const hasUser = !!localStorage.getItem("roses_user");
  const [screen, setScreen] = useState<Screen>(hasUser ? "app" : "onboarding");
  const [tab, setTab] = useState<AppTab>("chat");
  const [settingsOpen, setSettingsOpen] = useState(false);
  const [settings, setSettings] = useState<QuerySettings>(DEFAULT_SETTINGS);

  return (
    <>
      <div className="relative z-10" style={{ background: "#0b1121", height: "100vh", overflow: "hidden" }}>
        <AnimatePresence mode="wait">
          {screen === "onboarding" ? (
              <motion.div
                key="onboarding"
                initial={{ opacity: 0 }}
                animate={{ opacity: 1 }}
                exit={{ opacity: 0, scale: 0.98 }}
                transition={{ duration: 0.4 }}
                className="absolute inset-0"
              >
                <Onboarding onStart={() => setScreen("register")} />
              </motion.div>
            ) : screen === "register" ? (
              <motion.div
                key="register"
                initial={{ opacity: 0, x: 40 }}
                animate={{ opacity: 1, x: 0 }}
                exit={{ opacity: 0, x: -40 }}
                transition={{ duration: 0.4 }}
                className="absolute inset-0"
              >
                <Register onComplete={() => setScreen("app")} />
              </motion.div>
            ) : (
              <motion.div
                key="app"
                initial={{ opacity: 0, y: 20 }}
                animate={{ opacity: 1, y: 0 }}
                exit={{ opacity: 0 }}
                transition={{ duration: 0.4 }}
                className="absolute inset-0 flex flex-col"
              >
                {/* Conteúdo da aba */}
                <div className="flex-1 min-h-0 overflow-hidden">
                  <AnimatePresence mode="wait">
                    {tab === "chat" && (
                      <motion.div
                        key="chat"
                        initial={{ opacity: 0 }}
                        animate={{ opacity: 1 }}
                        exit={{ opacity: 0 }}
                        transition={{ duration: 0.2 }}
                        className="absolute inset-0"
                        style={{ bottom: "65px" }}
                      >
                        <Chat settings={settings} onOpenSettings={() => setSettingsOpen(true)} />
                      </motion.div>
                    )}
                    {tab === "calculos" && (
                      <motion.div
                        key="calculos"
                        initial={{ opacity: 0 }}
                        animate={{ opacity: 1 }}
                        exit={{ opacity: 0 }}
                        transition={{ duration: 0.2 }}
                        className="absolute inset-0"
                        style={{ bottom: "65px" }}
                      >
                        <CalculosTab />
                      </motion.div>
                    )}
                    {tab === "vigilancia" && (
                      <motion.div
                        key="vigilancia"
                        initial={{ opacity: 0 }}
                        animate={{ opacity: 1 }}
                        exit={{ opacity: 0 }}
                        transition={{ duration: 0.2 }}
                        className="absolute inset-0"
                        style={{ bottom: "65px" }}
                      >
                        <VigilanciaTab />
                      </motion.div>
                    )}
                    {tab === "portal" && (
                      <motion.div
                        key="portal"
                        initial={{ opacity: 0 }}
                        animate={{ opacity: 1 }}
                        exit={{ opacity: 0 }}
                        transition={{ duration: 0.2 }}
                        className="absolute inset-0"
                        style={{ bottom: "65px" }}
                      >
                        <PortalTab />
                      </motion.div>
                    )}
                    {tab === "auditoria" && (
                      <motion.div
                        key="auditoria"
                        initial={{ opacity: 0 }}
                        animate={{ opacity: 1 }}
                        exit={{ opacity: 0 }}
                        transition={{ duration: 0.2 }}
                        className="absolute inset-0"
                        style={{ bottom: "65px" }}
                      >
                        <AuditoriaTab />
                      </motion.div>
                    )}
                  </AnimatePresence>
                </div>

                {/* Bottom Nav */}
                <div className="absolute bottom-0 left-0 right-0">
                  <BottomNav active={tab} onChange={setTab} />
                </div>

                {/* Settings overlay */}
                <AnimatePresence>
                  {settingsOpen && (
                    <SettingsPanel
                      settings={settings}
                      onChange={setSettings}
                      onClose={() => setSettingsOpen(false)}
                    />
                  )}
                </AnimatePresence>
              </motion.div>
            )}
          </AnimatePresence>
        </div>
    </>
  );
}
