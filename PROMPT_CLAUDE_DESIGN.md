# Prompt para Claude Design — Roses AI (Redesign Apple-Engineer)

---

## Contexto do produto

**Roses AI** é um assistente jurídico para advogados brasileiros. Permite consultar processos por número CNJ, OAB ou nome da parte em qualquer tribunal do país. O app roda como PWA mobile-first e tem as seguintes telas:

1. **Onboarding** — boas-vindas, CTA para iniciar
2. **Register** — 6 etapas: Nome → Email/Senha → PIN → Foto → OAB → Sincronização
3. **App principal** com 5 abas na bottom nav:
   - **Chat** — copiloto jurídico (busca conversacional)
   - **Cálculos** — cálculos processuais
   - **Vigília** — monitoramento de CPF/CNPJ
   - **Portal** — acesso a portais de tribunais
   - **Auditoria** — log de consultas

---

## O que deve ser MANTIDO

- **Stack**: React + TypeScript + Tailwind + Framer Motion
- **Fluxo multi-step no Register** com animação de slide horizontal (direção baseada em `dir: 1 | -1`)
- **Estrutura de componentes** separados por tela/step
- **Lógica funcional** (validações, localStorage, `patch()` para estado do form)
- **Variantes de animação** com `staggerChildren` e spring physics
- **5 abas no BottomNav** com os mesmos IDs e ícones

---

## Problemas a corrigir — design atual

### Identidade visual
- Azul genérico (`#3B82F6`) sem personalidade
- Glassmorphism com `rgba(255,255,255,0.03)` quase invisível, sem profundidade real
- Fundo `#0E1117` sem textura ou hierarquia
- Fonte Geist sendo usada de forma plana, sem explorar pesos/tamanhos

### Register
- Ícones flutuantes animados (User, Mail) parecem um template de 2021
- Step "Crie sua conta" tem 3 campos empilhados sem respiração
- O componente `ProgressBar` não tem referência visual do passo atual
- Botão "Voltar" com `glass` parece fraco demais comparado ao CTA
- Nenhuma identidade de marca visível durante o onboarding

### Chat
- Header é apenas texto + ícone sem hierarquia visual
- Balões de mensagem com `rounded-3xl` são genéricos
- Input area fica apertada — o `Paperclip` ocupa espaço sem função clara
- Sugestões de chips somem após a primeira mensagem sem transição suave
- ProcessCard com borda `glass` é difícil de ler no dark bg

### BottomNav
- Indicador de aba ativo está no **topo** (linha em cima do ícone) — incomum e confuso
- Área de toque é pequena (botão com `px-2 py-1.5`)
- Label em 10px é ilegível

### App shell
- O "frame de celular" (`rounded-[2.5rem]`, `max-w-md`) limita a experiência — em telas maiores parece brinquedo
- `h-[860px]` força altura fixa que quebra em viewports menores

---

## Redesign solicitado — Apple Engineer Aesthetic

### Design System

**Paleta**
```
Background:   #000000 (true black, como iOS)
Surface-1:    #1C1C1E  (card/sheet base)
Surface-2:    #2C2C2E  (input fields, secondary cards)
Surface-3:    #3A3A3C  (borders, dividers)
Accent:       #0A84FF  (iOS blue sistema)
Accent-Green: #30D158  (success, online, ativo)
Accent-Red:   #FF453A  (erro, alerta)
Accent-Amber: #FFD60A  (warning)
Text-Primary: #FFFFFF
Text-Secondary: rgba(255,255,255,0.55)
Text-Tertiary:  rgba(255,255,255,0.25)
```

**Tipografia** — SF Pro equivalente via `system-ui` com fallback Inter
```
Display:   700, 34px, tracking -0.4px
Title-1:   700, 28px, tracking -0.3px
Title-2:   600, 22px, tracking -0.2px
Headline:  600, 17px
Body:      400, 17px
Callout:   400, 16px
Subhead:   400, 15px
Footnote:  400, 13px
Caption:   400, 12px, text-secondary
```

**Raios de borda**
```
xs:  6px   (chips, badges)
sm:  10px  (inputs)
md:  14px  (cards pequenos)
lg:  20px  (cards grandes, modais)
xl:  28px  (sheets)
pill: 9999px (botões CTA)
```

**Materiais** — substituir glassmorphism genérico por:
```css
.material-thin {
  background: rgba(28, 28, 30, 0.85);
  backdrop-filter: saturate(180%) blur(20px);
  -webkit-backdrop-filter: saturate(180%) blur(20px);
}
.material-regular {
  background: rgba(44, 44, 46, 0.92);
  backdrop-filter: saturate(200%) blur(24px);
}
```

**Sombras**
```
sm:  0 1px 3px rgba(0,0,0,0.5)
md:  0 4px 16px rgba(0,0,0,0.6)
lg:  0 8px 32px rgba(0,0,0,0.7)
glow-blue: 0 0 0 1px rgba(10,132,255,0.4), 0 4px 20px rgba(10,132,255,0.3)
```

---

### App Shell

Remover o "fake phone frame" em desktop. O app deve:
- Ser **full viewport** em mobile (`min-h-screen`, `max-w-390px` somente em tablet/desktop)
- Usar `safe-area-inset` padding no topo e base (para notch/home indicator)
- Background `#000000` puro

```tsx
// Estrutura
<div className="min-h-screen bg-black text-white" style={{ maxWidth: '390px', margin: '0 auto' }}>
```

---

### Onboarding

**Visual principal**: Em vez do ícone flutuante num quadrado, criar uma **composição de cards 3D empilhados** que sugerem processos jurídicos sendo organizados. Cards sutis em `Surface-1` com profundidade via `transform: perspective(800px) rotateX(8deg) rotateY(-4deg)` e `box-shadow: lg`. Nenhuma animação de "float" chamativa — apenas `opacity` e `scale` suaves.

**Headline**: `"Cada processo,\nao alcance\nda sua voz."` — 34px, bold, tracking tight

**Subtexto**: Body 17px, `text-secondary`, máx 2 linhas

**CTA**: Botão pill `bg-accent`, full-width, 56px height, sem `shadow-glow` exagerado

**Dots indicator**: 3 pontos, dot ativo é `bg-accent` com `w-6`, inativos são `w-2 bg-white/25`

---

### Register — Reformulação completa

**ProgressBar**: Substituir por **step counter + segmented bar**
```
"Passo 2 de 6"  →  [■■□□□□] em Surface-3/accent
```
Cada segmento é um retângulo `h-1`, preenchido com `accent` animado via `scaleX`.

**Step 0 — Nome**
- Sem ícone flutuante
- Topo: logo/wordmark "Roses" pequeno em `text-secondary`  
- Título: `"Bem-vindo."` Display 34px bold
- Subtítulo Body: `"Como você prefere ser chamado?"` em text-secondary
- Input único: campo grande, 56px height, `Surface-2`, sem label flutuante — placeholder some ao focar
- Sem ícone de `<User>` dentro do input — apenas campo limpo com cursor iOS

**Step 1 — Email/Senha**
- Título: `"Crie sua\nconta."` Title-1, 2 linhas
- Campos em `Surface-2`, altura 52px, `border-radius: 10px`
- Campos separados por `gap-3` — sem linha divisória interna
- **Password strength**: em vez de barras coloridas, mostrar checkmarks:
  ```
  ✓ 8+ caracteres
  ✓ Letra maiúscula
  ✓ Número
  ```
  Cada item anima de `text-tertiary` para `text-green` quando satisfeito

**Step 2 — PIN**
- 4 dígitos grandes estilo iOS — cada dígito em quadrado `Surface-2` 64×64px
- Fundo do dígito preenchido anima com `scale: 1 → 1.08 → 1` ao pressionar
- Teclado numérico custom (não teclado do browser)

**Step 3 — Foto**
- Círculo 120px central, borda dashed `Surface-3` que vira `accent` ao hover
- Dois botões: `"Tirar foto"` e `"Escolher da galeria"` — ambos como botões secundários pill
- `"Pular por agora"` como link underline em `text-secondary`

**Step 4 — OAB**
- Campo OAB + select de estado lado a lado: `[OAB número] [Estado ▾]`
- Estado como dropdown estilizado (não `<select>` nativo)
- Explicação em Footnote: `"Opcional. Permite buscar processos por sua OAB automaticamente."`

**Step 5 — Sincronização**
- Card central em `Surface-1` com ícone `iCloud`-like (circles sobrepostos)
- Toggle iOS-style para ativar sync
- Botão primário: `"Começar agora"`, secundário ghost: `"Configurar depois"`

**Navegação entre steps**:
- Botão voltar: apenas ícone `<ChevronLeft>` no canto superior esquerdo (não botão pill)
- Botão avançar: pill azul full-width, `56px` height
- Transições: `x: ±100%` com `opacity 0→1`, spring `stiffness:380, damping:32`

---

### Chat — Reformulação

**Header**
```
[Avatar circular 36px com gradiente linear]  Roses  "Assistente Jurídico"
                                                                    [⚙ circular button]
```
- Avatar: não ícone de balança — criar gradiente `from-blue-500 to-purple-600` no círculo
- Separador: `border-b` em `rgba(255,255,255,0.08)`

**Balões de mensagem**
- Usuário: `bg-accent` (#0A84FF), `border-radius: 20px 20px 4px 20px`, max-w 75%
- Assistente: `bg-Surface-2` (#2C2C2E), `border-radius: 20px 20px 20px 4px`, max-w 88%
- Padding: `px-4 py-3`, font-size 15px (Callout), line-height 1.5
- Sem sombra nos balões — flat sobre o fundo escuro

**Typing indicator**: 3 pontos em `Surface-2`, cada dot 8px, animação `y: 0→-5→0` defasada

**ProcessCard** — redesenhar para parecer um "resultado" claro:
- Fundo: `Surface-1` (#1C1C1E), border: `rgba(255,255,255,0.06)`, `border-radius: 14px`
- Header: número em `Accent`, monospace, 13px + badge do tribunal (pill `Surface-2`)
- Classe do processo: Headline 17px semibold
- Assunto: Footnote em text-secondary
- Partes: separadas por `·` em text-secondary
- Última movimentação: no rodapé com `ChevronRight` → link para abrir detalhes
- Sem `ExternalLink` como botão separado — toda a card é clicável

**Chips de sugestão**
- Desaparecem com `AnimatePresence` suave (opacity + y) quando usuário digita pela 1ª vez
- Cada chip: `Surface-2 px-3 py-2 rounded-full text-sm text-secondary`
- Sem ícone `Sparkles` — texto apenas

**Input area**
- Remover `Paperclip` (sem função implementada — não exibir affordance morta)
- Input: `Surface-1` fill, border: `rgba(255,255,255,0.08)`, `border-radius: 24px`, `px-4 py-3`
- Botão send: `bg-accent` circle 40px, aparece apenas quando `input.length > 0` com `scale: 0→1` spring
- Placeholder: `"Número CNJ, OAB ou nome…"`
- Sem borda extra ao redor da área de input

---

### BottomNav — iOS Tab Bar

```
|  Chat  |  Cálculos  |  Vigília  |  Portal  |  Auditoria  |
```
- Fundo: `rgba(28,28,30,0.85)` com `backdrop-filter: blur(20px)`
- Borda superior: `rgba(255,255,255,0.1)` 1px
- Cada tab: ícone 24px + label 10px com `letter-spacing: 0.02em`
- **Ativo**: ícone e label em `#0A84FF` — sem background colorido, sem indicador de linha
- **Inativo**: `rgba(255,255,255,0.4)`
- `layoutId="tab-indicator"` → remover. Cor é o único indicador
- Altura: 83px total (49px de conteúdo + `env(safe-area-inset-bottom)`)
- `padding-bottom: env(safe-area-inset-bottom, 16px)`

---

### VigilanciaTab

**Header**: igual às outras abas — consistência absoluta
**Sub-tabs "Monitorados | Alertas"**: estilo iOS segmented control
```css
.segmented {
  background: Surface-2;
  border-radius: 9px;
  padding: 2px;
}
.segment-active {
  background: Surface-3;
  border-radius: 7px;
  box-shadow: sm;
}
```

**Cards de vigilância**: `Surface-1`, `border-radius: 14px`, padding `16px`
- Nome em Headline, documento mascarado em `font-mono text-secondary`
- Badge "ativo/inativo": pill `12px`, sem background colorido — apenas ponto colorido `w-2 h-2` + texto
- Botão delete: aparece apenas com swipe-left (simulado com estado hover no desktop)

**Formulário de nova vigília**: sheet deslizante de baixo para cima (não expand inline)

---

### Animações globais — Apple Motion Guidelines

```
Micro interactions: spring stiffness:400, damping:35 (rápido, decisivo)
Page transitions:   spring stiffness:300, damping:30, duration max 0.35s
Enter cards:        y:12→0, opacity:0→1, staggerChildren:0.05s
Tap feedback:       scale:0.96 (whileTap), sem delay
Focus ring:         outline:none, box-shadow: 0 0 0 3px rgba(10,132,255,0.5)
```

Regras:
- Nunca animar cores de texto diretamente — usar opacity ou transform
- Todo botão CTA tem `whileTap={{ scale: 0.96 }}` — uniforme em 0.96 (não 0.97 ou 0.98)
- Cards que aparecem em lista: `staggerChildren: 0.04` (não 0.06 — mais rápido)

---

## Entregáveis esperados

Recriar os seguintes arquivos, mantendo a mesma estrutura de pastas:

```
web/src/
├── index.css                   ← novo design system (variáveis CSS)
├── App.tsx                     ← remover phone frame, ajustar shell
├── components/
│   ├── BottomNav.tsx           ← iOS tab bar
│   ├── FormInput.tsx           ← input iOS-style (sem label flutuante)
│   ├── ProcessCard.tsx         ← card reformulado
│   ├── ProgressBar.tsx         ← segmented step counter
│   └── SettingsPanel.tsx       ← sheet com backdrop blur
├── screens/
│   ├── Onboarding.tsx          ← visual 3D cards + nova copy
│   ├── Register.tsx            ← sem alterações de lógica
│   ├── Chat.tsx                ← reformulado
│   └── register/
│       ├── StepPersonal.tsx    ← sem ícone flutuante
│       ├── StepCredentials.tsx ← password checklist
│       ├── StepPin.tsx         ← teclado numérico custom
│       ├── StepPhoto.tsx       ← círculo + dois botões
│       ├── StepOAB.tsx         ← campo inline + dropdown
│       └── StepSync.tsx        ← toggle iOS
```

**Não alterar**: `api.ts`, `types.ts`, `screens/tabs/*.tsx` (exceto VigilanciaTab), `main.tsx`, lógica de validação em `Register.tsx`.

---

## Referências visuais de inspiração

- iOS Settings app (listas agrupadas em `Surface-1`)
- Apple Health app (cards com dados limpos, typography hierarchy)
- Vercel dashboard dark mode (focus nos dados, não na decoração)
- Linear app (ações rápidas, sem ruído visual)

**Anti-referências** (evitar):
- Glassmorphism com `rgba(255,255,255,0.03)` — invisível e sem propósito
- Glow shadows exageradas (`shadow-glow` azul em todos os botões)
- Ícones animados em loop como elemento decorativo principal
- Bordas arredondadas em `2.5rem`+ em elementos que não são "sheets"
