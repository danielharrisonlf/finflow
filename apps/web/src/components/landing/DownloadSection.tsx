import * as Tabs from "@radix-ui/react-tabs";
import {
  ArrowUpRight,
  Boxes,
  Check,
  Code2,
  Copy,
  Download,
  FolderOpen,
  Github,
  Globe,
  HardDrive,
  Laptop,
  Sparkles,
} from "lucide-react";
import { useEffect, useRef, useState } from "react";
import {
  type Detection,
  detectPlatform,
  installCommands,
  type Platform,
  platformById,
  platforms,
  repoUrl,
  sourceCommands,
} from "./download";

function InstallCommand({ label, command }: { label: string; command: string }) {
  const [status, setStatus] = useState("");
  const code = useRef<HTMLElement>(null);
  const copy = async () => {
    try {
      await navigator.clipboard.writeText(command);
      setStatus("Copiado");
    } catch {
      // Clipboard can be blocked; select the text so the visitor can copy it manually.
      const selection = window.getSelection();
      if (code.current && selection) {
        const range = document.createRange();
        range.selectNodeContents(code.current);
        selection.removeAllRanges();
        selection.addRange(range);
      }
      setStatus("Selecionado. Use Ctrl+C");
    }
    window.setTimeout(() => setStatus(""), 2400);
  };
  return (
    <div className="ff-install-command">
      <div className="ff-install-command-head">
        <span>{label}</span>
        <button type="button" onClick={copy} aria-label={`Copiar comando para ${label}`}>
          {status === "Copiado" ? <Check size={15} /> : <Copy size={15} />}
          <span aria-live="polite">{status || "Copiar"}</span>
        </button>
      </div>
      <code ref={code}>
        {/* Break long URLs after slashes instead of mid-word. */}
        {command.split("/").map((part, index) => (
          // biome-ignore lint/suspicious/noArrayIndexKey: static command fragments never reorder.
          <span key={index}>
            {index > 0 && "/"}
            {index > 0 && <wbr />}
            {part}
          </span>
        ))}
      </code>
    </div>
  );
}

export function DownloadSection({ appHref }: { appHref: string }) {
  const [detection, setDetection] = useState<Detection>({
    platform: "windows-amd64",
    mobile: false,
  });
  useEffect(() => {
    let active = true;
    detectPlatform().then((result) => {
      if (active) setDetection(result);
    });
    return () => {
      active = false;
    };
  }, []);

  const primary = platformById(detection.platform);
  const others = platforms.filter((p) => p.id !== primary.id);
  const openStep: Record<Platform["os"], string> = {
    windows: "Extraia o .zip e dê dois cliques em finflow.exe.",
    darwin: "Extraia o arquivo e rode ./finflow no Terminal.",
    linux: "Extraia o arquivo e rode ./finflow no terminal.",
  };
  const steps = [
    {
      icon: Download,
      title: "Baixe",
      text: `Um único arquivo para ${primary.label}, sem instalador.`,
    },
    { icon: FolderOpen, title: "Abra", text: openStep[primary.os] },
    { icon: Globe, title: "Use", text: "O navegador abre em localhost:3333 com o FinFlow pronto." },
  ];

  return (
    <section className="ff-section ff-download ff-container" id="baixar">
      <div className="ff-section-head">
        <span className="ff-eyebrow">Código aberto · licença MIT</span>
        <h2>Rode no seu computador.</h2>
        <p>
          Baixe o programa pronto, rode a partir do código ou suba com Docker. Em todos, o banco é
          um arquivo SQLite seu, sem cadastro, sem mensalidade e sem servidor de terceiros.
        </p>
        <a className="ff-text-link" href={appHref}>
          Experimentar no navegador, sem instalar
          <ArrowUpRight size={16} aria-hidden="true" />
        </a>
      </div>
      <Tabs.Root defaultValue="executavel" className="ff-download-tabs">
        <Tabs.List className="ff-download-tablist" aria-label="Como rodar o FinFlow">
          <Tabs.Trigger value="executavel">
            <Download size={16} aria-hidden="true" />
            Executável
          </Tabs.Trigger>
          <Tabs.Trigger value="codigo">
            <Code2 size={16} aria-hidden="true" />
            Código-fonte
          </Tabs.Trigger>
          <Tabs.Trigger value="docker">
            <Boxes size={16} aria-hidden="true" />
            Docker
          </Tabs.Trigger>
        </Tabs.List>
        <Tabs.Content value="executavel" className="ff-download-panel">
          <div className="ff-download-grid">
            <div className="ff-download-card">
              <span className="ff-feature-icon">
                <Laptop size={21} aria-hidden="true" />
              </span>
              {detection.mobile && (
                <p className="ff-download-mobile">
                  No celular, use o FinFlow no navegador, pelo link acima. O programa é para o
                  computador: abra esta página nele para baixar, ou escolha o sistema abaixo.
                </p>
              )}
              <a
                className="ff-button ff-download-primary"
                href={primary.href}
                data-platform={primary.id}
              >
                <Download size={18} aria-hidden="true" />
                Baixar para {primary.label}
              </a>
              <small className="ff-download-detail">{primary.detail} · versão mais recente</small>
              <div className="ff-download-others">
                <span>Outros sistemas</span>
                <ul>
                  {others.map((p) => (
                    <li key={p.id}>
                      <a href={p.href} data-platform={p.id}>
                        {p.label}
                        <small>{p.detail}</small>
                      </a>
                    </li>
                  ))}
                </ul>
              </div>
              <p className="ff-download-privacy">
                <HardDrive size={16} aria-hidden="true" />
                Seus dados ficam num arquivo SQLite no seu computador. Nada vai para a nuvem.
              </p>
            </div>
            <div className="ff-download-side">
              <ol className="ff-download-steps">
                {steps.map(({ icon: Icon, title, text }, index) => (
                  <li key={title}>
                    <span className="ff-step-index">0{index + 1}</span>
                    <div>
                      <h3>
                        <Icon size={17} aria-hidden="true" />
                        {title}
                      </h3>
                      <p>{text}</p>
                    </div>
                  </li>
                ))}
              </ol>
              <div className="ff-download-terminal">
                <h3>Prefere o terminal?</h3>
                <InstallCommand label="macOS e Linux" command={installCommands.unix} />
                <InstallCommand label="Windows (PowerShell)" command={installCommands.windows} />
              </div>
            </div>
          </div>
        </Tabs.Content>
        <Tabs.Content value="codigo" className="ff-download-panel">
          <div className="ff-download-grid">
            <div className="ff-download-terminal">
              <h3>Clone e rode com um comando</h3>
              <p>
                O <code>pnpm start</code> confere os requisitos, compila o que mudou e abre o
                FinFlow no navegador, com o banco num arquivo seu.
              </p>
              <InstallCommand label="Terminal" command={sourceCommands.clone} />
              <InstallCommand label="Com dados de exemplo" command={sourceCommands.seed} />
              <a className="ff-text-link" href={repoUrl} rel="noopener">
                <Github size={16} aria-hidden="true" />
                Código no GitHub
              </a>
            </div>
            <ul className="ff-download-requirements" aria-label="Requisitos">
              <li>
                <strong>Go 1.26 ou mais novo</strong>
                <span>go.dev/dl</span>
              </li>
              <li>
                <strong>Node 22.12 ou mais novo</strong>
                <span>nodejs.org</span>
              </li>
              <li>
                <strong>pnpm pelo Corepack</strong>
                <span>corepack enable</span>
              </li>
            </ul>
          </div>
        </Tabs.Content>
        <Tabs.Content value="docker" className="ff-download-panel">
          <div className="ff-download-grid">
            <div className="ff-download-terminal">
              <h3>Sem Go nem Node na máquina</h3>
              <p>
                Sobe o FinFlow em localhost:3333, com o banco num volume do Docker e acesso só por
                este computador.
              </p>
              <InstallCommand label="Docker Compose" command={sourceCommands.docker} />
            </div>
            <div className="ff-download-terminal">
              <h3>
                <Sparkles size={17} aria-hidden="true" />
                Com o assistente
              </h3>
              <p>
                Opcional: passe a sua chave da API do Claude ao subir. Sem chave, o recurso fica
                desligado e todo o resto funciona.
              </p>
              <InstallCommand
                label="Docker com o assistente"
                command={sourceCommands.dockerAssistant}
              />
            </div>
          </div>
        </Tabs.Content>
      </Tabs.Root>
    </section>
  );
}
