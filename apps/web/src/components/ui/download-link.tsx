import type { ReactNode } from "react";
import { downloadFile, downloadUrl } from "../../lib/api";
import { showToast } from "../../lib/toast";
import { isBrowserRuntime } from "../../runtime/client";

/**
 * Link de download de um arquivo da API (exportações, planilhas). Modo servidor: link comum com
 * `download` (o servidor manda o arquivo). Modo navegador: o clique baixa pelo runtime local e
 * salva o arquivo, sem rede.
 */
export function DownloadLink({
  path,
  params,
  filename,
  className,
  children,
}: {
  path: string;
  params?: URLSearchParams;
  /** Nome usado se a resposta não trouxer Content-Disposition. */
  filename: string;
  className?: string;
  children: ReactNode;
}) {
  const href = downloadUrl(path, params);
  if (!isBrowserRuntime) {
    return (
      <a href={href} download className={className}>
        {children}
      </a>
    );
  }
  return (
    <a
      href={href}
      download={filename}
      className={className}
      onClick={(event) => {
        event.preventDefault();
        downloadFile(path, filename, params).catch((error: unknown) => {
          showToast({
            tone: "error",
            message:
              error instanceof Error
                ? `Não foi possível baixar o arquivo: ${error.message}`
                : "Não foi possível baixar o arquivo.",
            durationMs: 6000,
          });
        });
      }}
    >
      {children}
    </a>
  );
}
