import {
  Camera,
  Check,
  CircleCheck,
  Code2,
  ImageIcon,
  KeyRound,
  LockKeyhole,
  ShieldCheck,
  Sparkles,
  X,
} from "lucide-react";
import { useState } from "react";

type Verdict = "YES" | "CAREFUL" | "NO";

interface Proposal {
  title: string;
  detail: string;
}

interface Sample {
  id: string;
  question: string;
  tool: string;
  verdict?: Verdict;
  answer: string;
  facts: string[];
  proposal?: Proposal;
}

const verdictLabel: Record<Verdict, string> = {
  YES: "Cabe no mês",
  CAREFUL: "Com cuidado",
  NO: "Melhor esperar",
};

// Answers are written locally to show the tone of the assistant. Nothing here calls an API.
const questions: Sample[] = [
  {
    id: "jantar",
    question: "Posso jantar fora esse mês?",
    tool: "Consultou seu limite e o ritmo do mês",
    verdict: "YES",
    answer:
      "Pode. Ainda restam R$ 1.240 do seu limite de novembro e você está gastando 8% menos que no mesmo período de outubro. Um jantar de até R$ 180 mantém o mês tranquilo.",
    facts: ["Limite usado depois do jantar: 78%", "Hoje ainda cabem R$ 86"],
  },
  {
    id: "viagem",
    question: "Posso fazer uma viagem?",
    tool: "Consultou sua projeção de saldo",
    verdict: "CAREFUL",
    answer:
      "Uma viagem de R$ 3.000 em dezembro cabe, mas o menor saldo cairia para R$ 420 em 08/01, abaixo da sua faixa de atenção. Em fevereiro a meta Próxima viagem já terá o valor e sobra folga.",
    facts: ["Menor saldo: R$ 420 em 08/01", "Com folga a partir de 10/02"],
  },
  {
    id: "celular",
    question: "Posso comprar um celular em 10x?",
    tool: "Simulou as parcelas na fatura do cartão",
    verdict: "NO",
    answer:
      "Agora não é o melhor momento. Com 10 parcelas de R$ 240, o saldo ficaria em −R$ 180 em 05/03, quando a fatura vence junto com as contas anuais. Sem saldo negativo, cabe a partir de 15/04.",
    facts: ["10 × R$ 240,00 no cartão", "Cabe a partir de 15/04"],
    proposal: {
      title: "Adicionar aos desejos",
      detail: "Celular · R$ 2.400,00 em 10x · avisar em 15/04",
    },
  },
];

const receipt: Sample = {
  id: "comprovante",
  question: "comprovante.jpg",
  tool: "Leu a foto do comprovante",
  answer:
    "Encontrei uma compra de R$ 32,00 na padaria, hoje, no débito. Quer registrar como despesa em Alimentação?",
  facts: ["Confiança da leitura: alta"],
  proposal: {
    title: "Nova despesa",
    detail: "Padaria · R$ 32,00 · Alimentação · hoje",
  },
};

type Decision = "pending" | "confirmed" | "dismissed";

function ProposalCard({
  proposal,
  decision,
  onDecide,
}: {
  proposal: Proposal;
  decision: Decision;
  onDecide: (next: Decision) => void;
}) {
  return (
    <div className="ff-assistant-proposal">
      <span className="ff-assistant-proposal-label">
        <ShieldCheck size={14} aria-hidden="true" />
        Precisa da sua confirmação
      </span>
      <strong>{proposal.title}</strong>
      <span>{proposal.detail}</span>
      {decision === "pending" ? (
        <div className="ff-assistant-proposal-actions">
          <button type="button" className="is-primary" onClick={() => onDecide("confirmed")}>
            <Check size={15} aria-hidden="true" />
            Confirmar
          </button>
          <button type="button" onClick={() => onDecide("dismissed")}>
            <X size={15} aria-hidden="true" />
            Agora não
          </button>
        </div>
      ) : (
        <p className="ff-assistant-proposal-result" role="status">
          {decision === "confirmed"
            ? "Exemplo: nada foi salvo. No app, o registro seria feito só agora, depois do seu toque."
            : "Tudo bem. Nada foi alterado."}
        </p>
      )}
    </div>
  );
}

export function AssistantSection() {
  const [selected, setSelected] = useState<string>(questions[0]?.id ?? "");
  const [decision, setDecision] = useState<Decision>("pending");
  const sample =
    selected === receipt.id
      ? receipt
      : (questions.find((q) => q.id === selected) ?? (questions[0] as Sample));
  const choose = (id: string) => {
    setSelected(id);
    setDecision("pending");
  };

  return (
    <section className="ff-section ff-container" id="assistente">
      <div className="ff-ai-panel ff-assistant">
        <div className="ff-ai-copy">
          <span className="ff-eyebrow">
            <Sparkles size={16} aria-hidden="true" />
            Assistente
          </span>
          <h2>Pergunte ao FinFlow.</h2>
          <p>
            Antes de uma compra, pergunte em português. O assistente consulta seu saldo projetado,
            seus limites e compromissos e responde com números, não com palpites.
          </p>
          <ul className="ff-ai-benefits">
            <li>
              <KeyRound size={16} aria-hidden="true" />
              Opcional: coloque a sua chave da API do Claude. Sem chave, o recurso fica desligado e
              todo o resto funciona.
            </li>
            <li>
              <ShieldCheck size={16} aria-hidden="true" />
              Registrar, agendar ou guardar dinheiro só acontece depois que você confirma.
            </li>
            <li>
              <Camera size={16} aria-hidden="true" />
              Envie a foto do comprovante e o lançamento chega preenchido para você revisar.
            </li>
          </ul>
          <p className="ff-fineprint">
            Exemplo ilustrativo: as respostas desta página são escritas de antemão e rodam só no seu
            navegador. No app, apenas as perguntas que você fizer ao assistente vão para a API do
            Claude, com a sua chave.
          </p>
        </div>

        <div className="ff-ai-chat ff-assistant-chat">
          <div className="ff-chat-heading">
            <span className="ff-chat-avatar">
              <Sparkles size={17} aria-hidden="true" />
            </span>
            <div>
              <strong>Pergunte ao FinFlow</strong>
              <small>dados fictícios</small>
            </div>
            <span className="ff-assistant-badge">Exemplo</span>
          </div>

          <fieldset className="ff-prompt-options" aria-label="Perguntas de exemplo">
            {questions.map((q) => (
              <button
                key={q.id}
                type="button"
                aria-pressed={selected === q.id}
                onClick={() => choose(q.id)}
              >
                {q.question}
              </button>
            ))}
          </fieldset>

          <div className="ff-assistant-thread" aria-live="polite">
            {sample.id === receipt.id ? (
              <div className="ff-chat-message ff-chat-outgoing ff-assistant-attachment">
                <ImageIcon size={16} aria-hidden="true" />
                <span>
                  {sample.question}
                  <small>foto do comprovante</small>
                </span>
              </div>
            ) : (
              <div className="ff-chat-message ff-chat-outgoing">{sample.question}</div>
            )}
            <p className="ff-assistant-tool">
              <CircleCheck size={13} aria-hidden="true" />
              {sample.tool}
            </p>
            <div className="ff-chat-message ff-chat-incoming ff-assistant-answer">
              {sample.verdict && (
                <span className={`ff-assistant-verdict is-${sample.verdict.toLowerCase()}`}>
                  {verdictLabel[sample.verdict]}
                </span>
              )}
              <p>{sample.answer}</p>
              <ul>
                {sample.facts.map((fact) => (
                  <li key={fact}>{fact}</li>
                ))}
              </ul>
            </div>
            {sample.proposal && (
              <ProposalCard proposal={sample.proposal} decision={decision} onDecide={setDecision} />
            )}
          </div>

          <button
            type="button"
            className="ff-assistant-receipt"
            aria-pressed={selected === receipt.id}
            onClick={() => choose(receipt.id)}
          >
            <Camera size={17} aria-hidden="true" />
            Enviar foto do comprovante
          </button>
          <div className="ff-ai-chat-footer">
            <LockKeyhole size={13} aria-hidden="true" />
            Respostas simuladas no navegador. Nada é enviado.
          </div>
        </div>
      </div>

      <ul className="ff-assistant-more">
        <li>
          <span className="ff-feature-icon">
            <Sparkles size={18} aria-hidden="true" />
          </span>
          <div>
            <h3>Use com o Claude ou outra IA</h3>
            <p>
              O servidor MCP incluído entrega seus dados a assistentes compatíveis, pelo terminal ou
              por HTTP em /api/mcp, protegido por token.
            </p>
          </div>
        </li>
        <li>
          <span className="ff-feature-icon">
            <Code2 size={18} aria-hidden="true" />
          </span>
          <div>
            <h3>Sua API pessoal</h3>
            <p>
              Tudo o que a interface faz está numa API local, para planilhas, scripts e automações
              suas.
            </p>
          </div>
        </li>
      </ul>
    </section>
  );
}
