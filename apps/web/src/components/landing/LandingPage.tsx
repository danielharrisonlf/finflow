import {
  ArrowDownLeft,
  ArrowLeftRight,
  ArrowRight,
  ArrowUpRight,
  AudioLines,
  Bell,
  CalendarClock,
  Check,
  CheckCheck,
  ChevronRight,
  CreditCard,
  Download,
  Gauge,
  Github,
  HardDrive,
  Heart,
  LayoutDashboard,
  LockKeyhole,
  Menu,
  Plus,
  Scissors,
  Send,
  ShieldCheck,
  Sparkles,
  Target,
  TrendingUp,
  Umbrella,
  Wallet,
  Wand2,
  X,
} from "lucide-react";
import { useEffect, useRef, useState } from "react";
import { Brand } from "./Brand";
import { DemoDashboard } from "./DemoDashboard";
import { DownloadSection } from "./DownloadSection";
import { repoUrl } from "./download";
import { GoalCalculator } from "./GoalCalculator";
import { money } from "./money";
import { ProjectionPreview } from "./ProjectionPreview";
import { SpreadsheetSection } from "./SpreadsheetSection";
import { ThemeToggle } from "./ThemeToggle";
import "./landing.css";

const links = [
  ["Recursos", "#recursos"],
  ["Planilhas", "#planilhas"],
  ["Como funciona", "#como-funciona"],
  ["Dúvidas", "#duvidas"],
];
const year = new Date().getFullYear();

function DownloadButton({ children = "Baixar grátis" }: { children?: React.ReactNode }) {
  return (
    <a className="ff-button" href="#baixar">
      {children}
      <Download size={17} aria-hidden="true" />
    </a>
  );
}

const downloadAnchor = "#baixar";

// Inside the self-hosted app the account link opens the dashboard; the public page has no
// accounts to sign in to, so the same slot leads to the download section.
function AccountAccess({ href, onNavigate }: { href?: string; onNavigate?: () => void }) {
  if (href) {
    return (
      <a className="ff-account-link" href={href} onClick={onNavigate}>
        Acessar minha conta
        <ArrowUpRight size={15} />
      </a>
    );
  }
  return (
    <a className="ff-account-link" href={downloadAnchor} onClick={onNavigate}>
      Baixar grátis
      <Download size={15} aria-hidden="true" />
    </a>
  );
}

function Navbar({ accountHref }: { accountHref?: string }) {
  const [open, setOpen] = useState(false);
  const menuButton = useRef<HTMLButtonElement>(null);
  const mobileNav = useRef<HTMLElement>(null);

  useEffect(() => {
    if (!open) return;
    const previousOverflow = document.body.style.overflow;
    document.body.style.overflow = "hidden";
    // Safari does not consistently focus buttons after pointer activation.
    // Keep the disclosure trigger as the start of the menu's keyboard sequence.
    menuButton.current?.focus();

    const handleKeyDown = (event: KeyboardEvent) => {
      if (event.key === "Escape") {
        setOpen(false);
        menuButton.current?.focus();
        return;
      }
      if (event.key === "Tab" && mobileNav.current && menuButton.current) {
        const focusables = [
          menuButton.current,
          ...Array.from(
            mobileNav.current.querySelectorAll<HTMLElement>(
              'a[href], button:not([disabled]), [tabindex]:not([tabindex="-1"])',
            ),
          ).filter((el) => el.getClientRects().length > 0),
        ];
        if (focusables.length === 0) return;
        const current = focusables.indexOf(document.activeElement as HTMLElement);
        const next = event.shiftKey
          ? current <= 0
            ? focusables.length - 1
            : current - 1
          : (current + 1) % focusables.length;
        // Safari can skip links with its default keyboard preferences.
        // Traverse the complete menu consistently and keep focus inside it.
        event.preventDefault();
        focusables[next]?.focus();
      }
    };

    const handlePointerDown = (event: PointerEvent) => {
      const target = event.target as Node | null;
      if (target && !mobileNav.current?.contains(target) && !menuButton.current?.contains(target)) {
        setOpen(false);
      }
    };

    const handleResize = () => {
      if (window.innerWidth > 1024) setOpen(false);
    };

    window.addEventListener("keydown", handleKeyDown);
    window.addEventListener("pointerdown", handlePointerDown);
    window.addEventListener("resize", handleResize);
    return () => {
      document.body.style.overflow = previousOverflow;
      window.removeEventListener("keydown", handleKeyDown);
      window.removeEventListener("pointerdown", handlePointerDown);
      window.removeEventListener("resize", handleResize);
    };
  }, [open]);

  return (
    <header className="ff-header">
      <nav className="ff-navbar ff-container" aria-label="Navegação principal">
        <Brand />
        <div className="ff-nav-links">
          {links.map(([label, href]) => (
            <a key={href} href={href}>
              {label}
            </a>
          ))}
        </div>
        <div className="ff-nav-actions">
          <ThemeToggle />
          <AccountAccess href={accountHref} />
          <button
            ref={menuButton}
            type="button"
            className="ff-icon-button ff-menu-button"
            aria-label={open ? "Fechar menu" : "Abrir menu"}
            aria-expanded={open}
            aria-controls="ff-mobile-nav"
            onClick={() => setOpen(!open)}
          >
            {open ? <X size={21} /> : <Menu size={21} />}
          </button>
        </div>
      </nav>
      {open && (
        <div className="ff-mobile-backdrop">
          <nav
            ref={mobileNav}
            id="ff-mobile-nav"
            className="ff-mobile-nav"
            aria-label="Navegação móvel"
          >
            <div className="ff-mobile-nav-links">
              {links.map(([label, href]) => (
                <a href={href} key={href} onClick={() => setOpen(false)}>
                  {label}
                  <ChevronRight size={18} />
                </a>
              ))}
            </div>
            <div className="ff-mobile-nav-footer">
              <div className="ff-mobile-theme-row">
                <span>Aparência</span>
                <ThemeToggle />
              </div>
              <AccountAccess href={accountHref} onNavigate={() => setOpen(false)} />
            </div>
          </nav>
        </div>
      )}
    </header>
  );
}

function Hero() {
  return (
    <section className="ff-hero ff-container" id="inicio">
      <div className="ff-hero-copy">
        <span className="ff-eyebrow ff-launch-status">
          <span />
          FinFlow · grátis e de código aberto
        </span>
        <h1>
          Seu dinheiro em ordem. <span className="ff-hero-emphasis">Mais espaço para viver.</span>
        </h1>
        <p>
          Contas, cartões, metas e planilhas em um só lugar, no seu computador. Entenda seu mês e
          planeje o que vem depois.
        </p>
        <div className="ff-hero-actions">
          <DownloadButton />
          <a className="ff-secondary-link" href="#como-funciona">
            Como funciona
            <ArrowRight size={17} aria-hidden="true" />
          </a>
        </div>
        <ul className="ff-hero-proof">
          <li>
            <Check size={15} />
            Windows, macOS e Linux
          </li>
          <li>
            <Check size={15} />
            Seus dados ficam com você
          </li>
        </ul>
      </div>
      <div className="ff-hero-preview">
        <DemoDashboard />
        <p className="ff-preview-note">
          <ShieldCheck size={14} />
          Dados ilustrativos. Nenhuma conta bancária é acessada.
        </p>
      </div>
    </section>
  );
}

type PhoneScreen = "inicio" | "lancamento" | "cartoes" | "extrato" | "metas";

const mobileSteps: { id: PhoneScreen; title: string; text: string }[] = [
  {
    id: "inicio",
    title: "Seu mês num relance",
    text: "Saldo, gastos e limites organizados para você entender o que está acontecendo.",
  },
  {
    id: "lancamento",
    title: "O pequeno gasto também conta",
    text: "Veja um exemplo de registro de despesa. Aquele café não precisa ficar só na memória.",
  },
  {
    id: "cartoes",
    title: "Faturas e metas no bolso",
    text: "Uma visão das parcelas do cartão e do caminho até sua reserva.",
  },
];

function PhoneHome({ saved }: { saved: boolean }) {
  return (
    <div className="ff-phone-view">
      <div className="ff-phone-greeting">
        <span className="ff-phone-avatar">D</span>
        <div>
          <small>Bom dia,</small>
          <strong>Daniel</strong>
        </div>
        <span className="ff-phone-bell">
          <Bell size={16} />
        </span>
      </div>

      <div className="ff-phone-balance">
        <small>Saldo livre em novembro</small>
        <strong>{saved ? "R$ 6.388,00" : "R$ 6.420,00"}</strong>
        <span>
          <TrendingUp size={13} />
          +12,8% em relação a outubro
        </span>
      </div>

      <div className="ff-phone-quick">
        <span>
          <i>
            <ArrowDownLeft size={17} />
          </i>
          Receita
        </span>
        <span>
          <i>
            <ArrowUpRight size={17} />
          </i>
          Despesa
        </span>
        <span>
          <i>
            <ArrowLeftRight size={17} />
          </i>
          Transferir
        </span>
        <span>
          <i>
            <Target size={17} />
          </i>
          Metas
        </span>
      </div>

      <div className="ff-phone-card">
        <div className="ff-phone-card-head">
          <strong>Orçamentos</strong>
          <small>2 de 3 no ritmo</small>
        </div>
        <div className="ff-phone-budget">
          <div>
            <span>Alimentação</span>
            <span>{saved ? "67%" : "64%"}</span>
          </div>
          <div className="ff-progress">
            <span className="ff-income-bg" style={{ width: saved ? "67%" : "64%" }} />
          </div>
        </div>
        <div className="ff-phone-budget">
          <div>
            <span>Lazer</span>
            <span>82%</span>
          </div>
          <div className="ff-progress">
            <span className="ff-warning-bg" style={{ width: "82%" }} />
          </div>
        </div>
      </div>

      <div className="ff-phone-list">
        <strong className="ff-phone-list-title">Hoje</strong>
        {saved && (
          <div className="ff-phone-row is-new">
            <span className="ff-phone-row-icon">
              <Wallet size={15} />
            </span>
            <div>
              <strong>Padaria</strong>
              <small>Agora</small>
            </div>
            <strong>− R$ 32,00</strong>
          </div>
        )}
        <div className="ff-phone-row">
          <span className="ff-phone-row-icon">
            <ArrowLeftRight size={15} />
          </span>
          <div>
            <strong>Uber</strong>
            <small>08:45</small>
          </div>
          <strong>− R$ 28,90</strong>
        </div>
        {!saved && (
          <div className="ff-phone-row">
            <span className="ff-phone-row-icon income">
              <ArrowDownLeft size={15} />
            </span>
            <div>
              <strong>Reembolso Pix</strong>
              <small>07:12</small>
            </div>
            <strong className="ff-income">+ R$ 180,00</strong>
          </div>
        )}
      </div>
    </div>
  );
}

function PhoneQuickAdd({ saved, onToggle }: { saved: boolean; onToggle: () => void }) {
  return (
    <div className="ff-phone-view ff-phone-sheet-view">
      <div className="ff-phone-sheet-head">
        <strong>Nova despesa</strong>
        <span>Hoje, 09:41</span>
      </div>
      <div className="ff-phone-amount">
        <small>Valor</small>
        <strong>R$ 32,00</strong>
        <span>Padaria</span>
      </div>
      <div className="ff-phone-field">
        <small>Categoria</small>
        <div className="ff-phone-chips">
          <span className="active">Alimentação</span>
          <span>Transporte</span>
          <span>Lazer</span>
        </div>
      </div>
      <div className="ff-phone-field">
        <small>Pagar com</small>
        <div className="ff-phone-account">
          <Wallet size={16} />
          <span>Conta do dia a dia</span>
          <ChevronRight size={16} />
        </div>
      </div>
      <button type="button" className="ff-phone-confirm" onClick={onToggle}>
        {saved ? <CheckCheck size={17} /> : <Check size={17} />}
        {saved ? "Registrada. Toque para desfazer" : "Registrar despesa"}
      </button>
    </div>
  );
}

function PhoneCards() {
  return (
    <div className="ff-phone-view">
      <div className="ff-phone-sheet-head">
        <strong>Cartões</strong>
        <span>Novembro</span>
      </div>
      <div className="ff-phone-credit">
        <div>
          <span>Cartão do dia a dia</span>
          <CreditCard size={18} />
        </div>
        <small>Fatura aberta</small>
        <strong>R$ 2.384,50</strong>
        <div className="ff-phone-credit-bar">
          <span style={{ width: "24%" }} />
        </div>
        <div>
          <small>Limite livre R$ 7.615,50</small>
          <small>Fecha dia 08</small>
        </div>
      </div>
      <div className="ff-phone-card">
        <div className="ff-phone-card-head">
          <strong>Próximas parcelas</strong>
          <small>3 compras</small>
        </div>
        <div className="ff-phone-installment">
          <span>Dezembro</span>
          <strong>R$ 350,00</strong>
        </div>
        <div className="ff-phone-installment">
          <span>Janeiro</span>
          <strong>R$ 350,00</strong>
        </div>
      </div>
      <div className="ff-phone-card">
        <div className="ff-phone-card-head">
          <strong>Reserva de emergência</strong>
          <small className="ff-income">84%</small>
        </div>
        <div className="ff-progress">
          <span className="ff-income-bg" style={{ width: "84%" }} />
        </div>
        <small className="ff-phone-goal-note">R$ 25.200 de R$ 30.000</small>
      </div>
    </div>
  );
}

function PhoneTransactions({ saved }: { saved: boolean }) {
  return (
    <div className="ff-phone-view">
      <div className="ff-phone-sheet-head">
        <strong>Extrato</strong>
        <span>Novembro · exemplo</span>
      </div>
      <p className="ff-fineprint">Um lugar para lembrar de cada gasto.</p>
      {[
        ...(saved ? [{ name: "Padaria", category: "Alimentação", amount: "− R$ 32,00" }] : []),
        { name: "Uber", category: "Transporte", amount: "− R$ 28,90" },
        { name: "Reembolso Pix", category: "Entrada", amount: "+ R$ 180,00" },
        { name: "Mercado", category: "Alimentação", amount: "− R$ 286,90" },
      ].map((item) => (
        <div className="ff-phone-row" key={item.name}>
          <span className="ff-phone-row-icon">
            <ArrowLeftRight size={15} aria-hidden="true" />
          </span>
          <div>
            <strong>{item.name}</strong>
            <small>{item.category}</small>
          </div>
          <strong>{item.amount}</strong>
        </div>
      ))}
    </div>
  );
}

function PhoneGoals() {
  return (
    <div className="ff-phone-view">
      <div className="ff-phone-sheet-head">
        <strong>Minhas metas</strong>
        <Target size={17} aria-hidden="true" />
      </div>
      <div className="ff-phone-card">
        <div className="ff-phone-card-head">
          <strong>Reserva de emergência</strong>
          <small>84%</small>
        </div>
        <div className="ff-progress">
          <span style={{ width: "84%" }} />
        </div>
        <small className="ff-phone-goal-note">R$ 25.200 de R$ 30.000</small>
      </div>
      <div className="ff-phone-card">
        <div className="ff-phone-card-head">
          <strong>Próxima viagem</strong>
          <small>40%</small>
        </div>
        <div className="ff-progress">
          <span style={{ width: "40%" }} />
        </div>
        <small className="ff-phone-goal-note">R$ 2.000 de R$ 5.000</small>
      </div>
      <p className="ff-fineprint">Objetivos de exemplo. Cada passo conta.</p>
    </div>
  );
}

function MobileExperience() {
  const [screen, setScreen] = useState<PhoneScreen>("inicio");
  const [saved, setSaved] = useState(false);
  const showScreen = (next: PhoneScreen) => {
    setScreen(next);
    if (matchMedia("(max-width: 600px)").matches) {
      document.querySelector("#mobile .ff-phone")?.scrollIntoView({
        block: "center",
        behavior: matchMedia("(prefers-reduced-motion: reduce)").matches ? "instant" : "smooth",
      });
    }
  };

  return (
    <section className="ff-section ff-container" id="mobile">
      <div className="ff-mobile-grid">
        <div className="ff-mobile-stage">
          <div className="ff-phone">
            <div className="ff-phone-screen">
              <span className="ff-phone-island" aria-hidden="true" />
              <div className="ff-phone-status" aria-hidden="true">
                <span>9:41</span>
                <span className="ff-phone-signal">
                  <i />
                  <i />
                  <i />
                  <b />
                </span>
              </div>
              <div className="ff-phone-body">
                {screen === "inicio" && <PhoneHome saved={saved} />}
                {screen === "lancamento" && (
                  <PhoneQuickAdd saved={saved} onToggle={() => setSaved(!saved)} />
                )}
                {screen === "cartoes" && <PhoneCards />}
                {screen === "extrato" && <PhoneTransactions saved={saved} />}
                {screen === "metas" && <PhoneGoals />}
              </div>
              <fieldset className="ff-phone-tabbar" aria-label="Navegação do aplicativo">
                <button
                  type="button"
                  className={screen === "inicio" ? "active" : ""}
                  aria-pressed={screen === "inicio"}
                  onClick={() => setScreen("inicio")}
                >
                  <LayoutDashboard size={18} />
                  Início
                </button>
                <button
                  type="button"
                  className={screen === "extrato" ? "active" : ""}
                  aria-pressed={screen === "extrato"}
                  onClick={() => setScreen("extrato")}
                >
                  <ArrowLeftRight size={18} />
                  Extrato
                </button>
                <button
                  type="button"
                  className="ff-phone-fab"
                  aria-label="Registrar despesa rápida"
                  onClick={() => setScreen("lancamento")}
                >
                  <Plus size={22} />
                </button>
                <button
                  type="button"
                  className={screen === "cartoes" ? "active" : ""}
                  aria-pressed={screen === "cartoes"}
                  onClick={() => setScreen("cartoes")}
                >
                  <CreditCard size={18} />
                  Cartões
                </button>
                <button
                  type="button"
                  className={screen === "metas" ? "active" : ""}
                  aria-pressed={screen === "metas"}
                  onClick={() => setScreen("metas")}
                >
                  <Target size={18} />
                  Metas
                </button>
              </fieldset>
              <span className="ff-phone-home" aria-hidden="true" />
            </div>
          </div>
          <div className={`ff-phone-toast ${saved ? "is-visible" : ""}`} aria-live="polite">
            {saved && (
              <>
                <span>
                  <Check size={14} />
                </span>
                Despesa de R$ 32,00 registrada
              </>
            )}
          </div>
        </div>

        <div className="ff-mobile-copy">
          <span className="ff-eyebrow">Pensado para a vida que acontece</span>
          <h2>
            O café de hoje.
            <br />
            Os planos de amanhã.
            <br />
            <span>Tudo no seu bolso.</span>
          </h2>
          <p>
            Cuidar do dinheiro não deveria tomar o seu dia. Uma experiência simples no celular, com
            o que importa ao alcance da mão.
          </p>
          <fieldset className="ff-mobile-steps" aria-label="Telas do aplicativo">
            {mobileSteps.map((step) => (
              <button
                key={step.id}
                type="button"
                className="ff-mobile-step"
                aria-pressed={screen === step.id}
                onClick={() => showScreen(step.id)}
              >
                <strong>{step.title}</strong>
                <span>{step.text}</span>
              </button>
            ))}
          </fieldset>
          <p className="ff-fineprint">
            Prévia ilustrativa. A interface se adapta à tela do celular.
          </p>
          <div className="ff-mobile-telegram">
            <div>
              <Send size={18} />
              <span>Um exemplo por mensagem</span>
            </div>
            <TelegramPreview />
          </div>
        </div>
      </div>
    </section>
  );
}

function CardPreview() {
  const [future, setFuture] = useState(false);
  return (
    <div className="ff-card-preview">
      <div className="ff-physical-card" aria-hidden="true">
        <div>
          <span>Seu cartão</span>
          <CreditCard size={20} />
        </div>
        <span className="ff-card-chip" />
        <div>
          <span>•••• 4829</span>
          <span>12/29</span>
        </div>
      </div>
      <div className="ff-invoice">
        <div className="ff-invoice-head">
          <span>Próxima fatura</span>
          <span className="ff-badge">{future ? "Dezembro" : "Novembro"}</span>
        </div>
        <strong>{money(future ? 35000 : 128450)}</strong>
        <div className="ff-invoice-row">
          <span>Compras do mês</span>
          <span>{money(future ? 0 : 93450)}</span>
        </div>
        <div className="ff-invoice-row">
          <span>Parcelas previstas</span>
          <span>{money(35000)}</span>
        </div>
        <button type="button" onClick={() => setFuture(!future)}>
          {future ? "Voltar à fatura atual" : "Ver o próximo mês"}
          <ArrowRight size={15} />
        </button>
      </div>
    </div>
  );
}

function BudgetPreview() {
  return (
    <div className="ff-budget-preview">
      {[
        {
          title: "Alimentação",
          amount: "R$ 540 de R$ 900",
          percent: 60,
          color: "income",
          status: "Dentro do planejado",
        },
        {
          title: "Lazer",
          amount: "R$ 410 de R$ 500",
          percent: 82,
          color: "warning",
          status: "Pede atenção",
        },
        {
          title: "Transporte",
          amount: "R$ 580 de R$ 600",
          percent: 97,
          color: "danger",
          status: "Perto do limite",
        },
      ].map((b) => (
        <div className="ff-budget-item" key={b.title}>
          <div>
            <strong>{b.title}</strong>
            <span>{b.amount}</span>
          </div>
          <div className="ff-progress">
            <span className={`ff-${b.color}-bg`} style={{ width: `${b.percent}%` }} />
          </div>
          <span className={`ff-budget-status ff-${b.color}`}>
            {b.percent}% usado, {b.status.toLowerCase()}
          </span>
        </div>
      ))}
    </div>
  );
}

function TelegramPreview() {
  const [sent, setSent] = useState(false);
  return (
    <div className="ff-telegram">
      <div className="ff-chat-heading">
        <span className="ff-chat-avatar">
          <Send size={17} />
        </span>
        <div>
          <strong>FinFlow no Telegram</strong>
          <small>prévia com dados fictícios</small>
        </div>
      </div>
      <div className="ff-chat-message ff-chat-outgoing">
        45,90 almoço #alimentacao
        <small>
          12:42
          <CheckCheck size={13} />
        </small>
      </div>
      <div className="ff-chat-message ff-chat-incoming">
        <span>
          <Check size={14} />
          Pronto, almoço registrado.
        </span>
        <strong>R$ 45,90</strong>
        <small>Alimentação, dados ilustrativos</small>
      </div>
      {sent && (
        <div className="ff-chat-message ff-chat-incoming" role="status">
          <span>Seu saldo disponível</span>
          <strong>R$ 24.586,50</strong>
          <small>Consulta simulada</small>
        </div>
      )}
      <button type="button" className="ff-chat-input" onClick={() => setSent(!sent)}>
        {sent ? "Reiniciar exemplo" : "Ver exemplo de consulta"}
        <Send size={16} />
      </button>
    </div>
  );
}

const moreFeatures = [
  {
    icon: CalendarClock,
    title: "Compromissos",
    text: "Contas fixas e parcelas dos próximos meses, separadas entre a pagar e a receber.",
  },
  {
    icon: Gauge,
    title: "Ritmo do mês",
    text: "Quanto você gastou até hoje comparado ao mesmo período do mês passado.",
  },
  {
    icon: Scissors,
    title: "Onde dá para cortar",
    text: "Cada gasto é Fixo, Cortável ou Variável. Assim fica claro o que pode sair primeiro.",
  },
  {
    icon: Umbrella,
    title: "Reserva de emergência",
    text: "Uma meta de 6 meses do seu custo de vida, ou 12 se alguém depende de você.",
  },
  {
    icon: Heart,
    title: "Desejos",
    text: "Sua lista de compras por prioridade, com a data em que cada item cabe no saldo.",
  },
  {
    icon: Wand2,
    title: "Automações",
    text: "Regras que categorizam lançamentos e importações sozinhas pela descrição.",
  },
];

function DailyFeatures() {
  return (
    <section className="ff-section ff-container" id="recursos">
      <div className="ff-section-head">
        <span className="ff-eyebrow">Clareza para as decisões do dia a dia</span>
        <h2>
          O mês fica mais leve
          <br />
          quando você vê o todo.
        </h2>
        <p>
          Menos contas de cabeça. Mais clareza sobre o que já foi, o que vem pela frente e o que
          ainda cabe nos seus planos.
        </p>
      </div>
      <div className="ff-features-grid">
        <article className="ff-feature-card ff-projection-feature" id="projecao">
          <div className="ff-feature-copy">
            <span className="ff-feature-icon">
              <TrendingUp size={20} aria-hidden="true" />
            </span>
            <h3>Veja o saldo de cada dia antes de ele chegar.</h3>
            <p>
              Contas fixas, parcelas e faturas entram na projeção do saldo dia a dia, com faixas de
              cor do folgado ao negativo. O FinFlow avisa qual será o menor saldo nos próximos 30
              dias e quanto dá para guardar sem apertar.
            </p>
          </div>
          <ProjectionPreview />
        </article>
        <article className="ff-feature-card ff-cards-feature" id="cartoes">
          <div className="ff-feature-copy">
            <span className="ff-feature-icon">
              <CreditCard size={20} aria-hidden="true" />
            </span>
            <h3>Cartões e faturas sem surpresa.</h3>
            <p>
              Saiba o que já comprou e o que ainda vem nas próximas parcelas. O fim do mês não
              precisa ser uma surpresa.
            </p>
          </div>
          <CardPreview />
        </article>
        <article className="ff-feature-card ff-budgets-feature">
          <div className="ff-feature-copy">
            <span className="ff-feature-icon">
              <Wallet size={20} aria-hidden="true" />
            </span>
            <h3>Seu limite, no seu ritmo.</h3>
            <p>
              Defina um limite mensal e veja se o mês está mais rápido que o anterior. Cores por
              categoria ajudam a decidir, sem fazer conta de cabeça.
            </p>
          </div>
          <BudgetPreview />
        </article>
      </div>
      <ul className="ff-feature-list">
        {moreFeatures.map(({ icon: Icon, title, text }) => (
          <li key={title}>
            <span className="ff-feature-icon">
              <Icon size={19} aria-hidden="true" />
            </span>
            <div>
              <h3>{title}</h3>
              <p>{text}</p>
            </div>
          </li>
        ))}
      </ul>
    </section>
  );
}

function HowItWorks() {
  return (
    <section className="ff-section ff-how ff-container" id="como-funciona">
      <div className="ff-section-head">
        <span className="ff-eyebrow">Um começo possível</span>
        <h2>
          Um pouco de organização.
          <br />
          Uma nova visão do mês.
        </h2>
        <p>Reunir as informações e ajudar você a decidir com mais clareza, no seu tempo.</p>
      </div>
      <ol className="ff-how-steps">
        {[
          {
            title: "Reúna o que entra e sai",
            text: "Lance receitas e despesas ou importe sua planilha e o extrato do banco.",
          },
          {
            title: "Encontre o seu ritmo",
            text: "Separe o fixo do variável, escolha um limite mensal e acompanhe o ritmo do mês.",
          },
          {
            title: "Dê espaço aos seus planos",
            text: "Veja o saldo dos próximos dias, guarde quando sobrar e monte sua reserva.",
          },
        ].map((step, index) => (
          <li key={step.title}>
            <span className="ff-step-index">0{index + 1}</span>
            <h3>{step.title}</h3>
            <p>{step.text}</p>
          </li>
        ))}
      </ol>
    </section>
  );
}

const prompts = [
  {
    question: "Quanto ainda posso gastar esta semana?",
    answer:
      "Neste exemplo, restam R$ 360 no seu orçamento de alimentação do mês. Para uma semana tranquila, você pode planejar até R$ 90 nessa categoria.",
    label: "Consultar orçamento",
    value: "R$ 360 disponíveis",
  },
  {
    question: "Adicione 35 reais de almoço.",
    answer:
      "Almoço de R$ 35,00 registrado em Alimentação. O orçamento do mês foi atualizado. Mais um gasto que não ficou para depois.",
    label: "Registrar uma despesa",
    value: "R$ 35,00 em Alimentação",
  },
  {
    question: "Me ajude a entender meus gastos.",
    answer:
      "Neste exemplo, Alimentação está dentro do planejado. Lazer já usou 82% do orçamento, e Transporte está perto do limite. Vale acompanhar essas duas categorias.",
    label: "Olhar o mês com clareza",
    value: "3 categorias analisadas",
  },
] as const;

function AIAssistant() {
  const [selected, setSelected] = useState(0);
  const sample = prompts[selected] ?? prompts[0];
  return (
    <section className="ff-section ff-container" id="assistente">
      <div className="ff-ai-panel">
        <div className="ff-ai-copy">
          <span className="ff-eyebrow">
            <Sparkles size={16} />
            MAIS CONVERSA. MENOS FORMULÁRIOS.
          </span>
          <h2>Às vezes, você só quer perguntar.</h2>
          <p>
            “Quanto gastei com alimentação?” Uma conversa pode ajudar a entender o mês. Veja
            exemplos de como o FinFlow pode apresentar suas informações a um assistente compatível.
          </p>
          <ul className="ff-ai-benefits">
            <li>
              <Check size={16} />
              Perguntas do dia a dia, em linguagem simples.
            </li>
            <li>
              <Check size={16} />
              Você decide o que quer consultar ou registrar.
            </li>
          </ul>
          <p className="ff-fineprint">
            Os exemplos são ilustrativos. O FinFlow inclui um servidor MCP e uma linha de comando; o
            recurso depende de um assistente compatível configurado por você.
          </p>
        </div>
        <div className="ff-ai-chat">
          <div className="ff-chat-heading">
            <span className="ff-chat-avatar">
              <Sparkles size={17} />
            </span>
            <div>
              <strong>Seu assistente financeiro</strong>
              <small>simulação local</small>
            </div>
          </div>
          <fieldset className="ff-prompt-options" aria-label="Exemplos de conversa">
            {prompts.map((p, i) => (
              <button
                key={p.label}
                type="button"
                aria-pressed={selected === i}
                onClick={() => setSelected(i)}
              >
                {p.label}
              </button>
            ))}
          </fieldset>
          <div className="ff-chat-message ff-chat-outgoing">{sample.question}</div>
          <div className="ff-ai-answer" aria-live="polite">
            <span className="ff-ai-answer-icon">
              <Sparkles size={15} />
            </span>
            <div>
              <p>{sample.answer}</p>
              <span className="ff-ai-answer-result">
                <Check size={13} />
                {sample.value}
              </span>
            </div>
          </div>
          <div className="ff-ai-chat-footer">
            <LockKeyhole size={13} />
            Você está explorando dados ilustrativos.
          </div>
        </div>
      </div>
    </section>
  );
}

function Privacy() {
  return (
    <section className="ff-section ff-privacy ff-container" id="seguranca">
      <div className="ff-privacy-heading">
        <span className="ff-eyebrow">Confiança começa com clareza</span>
        <h2>
          Seu dinheiro.
          <br />
          Seus dados. Suas escolhas.
        </h2>
        <p>
          O FinFlow roda no seu computador. Sem conta, sem cadastro e sem enviar seus dados
          financeiros para servidor nenhum.
        </p>
        <a className="ff-text-link" href="#baixar">
          Baixar o FinFlow
          <Download size={16} aria-hidden="true" />
        </a>
      </div>
      <div className="ff-privacy-features">
        {[
          {
            icon: HardDrive,
            title: "Um arquivo no seu computador",
            text: "Tudo fica num banco SQLite local. Para fazer backup, basta copiar o arquivo. Nada vai para a nuvem.",
          },
          {
            icon: LockKeyhole,
            title: "Nenhuma senha bancária",
            text: "O FinFlow não se conecta ao seu banco nem faz pagamentos. Você importa o extrato quando quiser.",
          },
          {
            icon: Github,
            title: "Código aberto e gratuito",
            text: "Qualquer pessoa pode ler o código, compilar a própria versão e conferir o que o programa faz.",
          },
        ].map(({ icon: Icon, title, text }) => (
          <article key={title}>
            <span className="ff-feature-icon">
              <Icon size={19} />
            </span>
            <div>
              <h3>{title}</h3>
              <p>{text}</p>
            </div>
          </article>
        ))}
      </div>
    </section>
  );
}

const questions = [
  {
    q: "O FinFlow é gratuito?",
    a: "Sim. O FinFlow é gratuito e de código aberto. Não há cadastro, mensalidade ou pedido de cartão: você baixa o programa e usa no seu computador.",
  },
  {
    q: "Onde ficam os meus dados?",
    a: "Num arquivo SQLite no seu computador, na pasta de configuração do usuário (AppData no Windows, Application Support no macOS, ~/.config no Linux). Nada é enviado para a nuvem. Para fazer backup, copie esse arquivo.",
  },
  {
    q: "Preciso instalar alguma coisa?",
    a: "Não. O FinFlow é um único arquivo para Windows, macOS ou Linux. Abra o programa e use pelo navegador em localhost:3333. Não precisa de Docker nem de banco de dados à parte.",
  },
  {
    q: "Consigo trazer a planilha que já uso?",
    a: "Sim. O FinFlow importa a planilha diária (Entrada, Saída, Diário e Saldo), arquivos do Excel e CSV, e o extrato OFX do banco. Antes de importar, você confere uma prévia com possíveis duplicados. E pode exportar tudo para uma pasta .xlsx com fórmulas.",
  },
  {
    q: "O FinFlow se conecta automaticamente ao meu banco?",
    a: "Não. O FinFlow não pede sua senha bancária nem acessa contas reais. Você registra os lançamentos ou importa o extrato exportado pelo internet banking.",
  },
  {
    q: "Preciso entender de finanças para usar?",
    a: "Não. A proposta é ajudar quem quer organizar o dinheiro no dia a dia, mesmo sem conhecer termos financeiros. Receitas, gastos, limites e metas aparecem com valores e explicações simples.",
  },
  {
    q: "O que acontece com os valores que eu testar aqui?",
    a: "Os controles da prévia e da calculadora funcionam só no seu navegador. Os valores voltam ao início quando você recarrega a página. A preferência de tema é salva no navegador, quando ele permite.",
  },
];

function FAQ() {
  return (
    <section className="ff-section ff-faq ff-container" id="duvidas">
      <h2>
        Clareza desde
        <br />o primeiro passo.
      </h2>
      <div>
        {questions.map((item) => (
          <details key={item.q}>
            <summary>
              {item.q}
              <Plus size={18} />
            </summary>
            <p>{item.a}</p>
          </details>
        ))}
      </div>
    </section>
  );
}

function Footer() {
  return (
    <>
      <section className="ff-final ff-container">
        <div className="ff-final-inner">
          <span className="ff-final-icon">
            <AudioLines size={26} strokeWidth={2.2} />
          </span>
          <h2>
            Mais tranquilidade.
            <br />
            <span>Um passo de cada vez.</span>
          </h2>
          <p>Baixe, abra no seu computador e comece hoje. Grátis e sem cadastro.</p>
          <DownloadButton />
          <a className="ff-text-link" href="#inicio">
            Voltar ao início
            <ArrowUpRight size={17} aria-hidden="true" />
          </a>
        </div>
      </section>
      <footer className="ff-footer ff-container">
        <div>
          <Brand />
          <p>O dinheiro organizado. Mais espaço para viver.</p>
        </div>
        <div className="ff-footer-links">
          <a href="#recursos">Recursos</a>
          <a href="#planilhas">Planilhas</a>
          <a href="#mobile">No celular</a>
          <a href="#como-funciona">Como funciona</a>
          <a href="#seguranca">Privacidade</a>
          <a href="#baixar">Baixar</a>
          <a href="#duvidas">Dúvidas</a>
          <a href={repoUrl} rel="noopener">
            Código-fonte
          </a>
        </div>
        <div className="ff-footer-bottom">
          <span>© {year} FinFlow. Gratuito, de código aberto e feito para a vida real.</span>
          <a href="#inicio">
            Voltar ao início
            <ArrowUpRight size={14} />
          </a>
        </div>
      </footer>
    </>
  );
}

export function LandingPage({ accountHref }: { accountHref?: string }) {
  return (
    <div className="finflow-landing">
      <a className="ff-skip-link" href="#ff-content">
        Pular para o conteúdo
      </a>
      <Navbar accountHref={accountHref} />
      <main id="ff-content">
        <Hero />
        <DailyFeatures />
        <SpreadsheetSection />
        <HowItWorks />
        <MobileExperience />
        <GoalCalculator />
        <AIAssistant />
        <Privacy />
        <DownloadSection />
        <FAQ />
      </main>
      <Footer />
    </div>
  );
}
