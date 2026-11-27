import qrcode from "qrcode-generator";
import { cn } from "../../lib/cn";

/**
 * QR code em SVG a partir da matriz de módulos. Fundo branco com zona de silêncio fixa
 * (leitores de câmera precisam de contraste escuro sobre claro, nos dois temas).
 */
export function QrCode({
  value,
  size = 168,
  label,
  className,
}: {
  value: string;
  size?: number;
  label: string;
  className?: string;
}) {
  const qr = qrcode(0, "M");
  qr.addData(value);
  qr.make();
  const count = qr.getModuleCount();
  const quiet = 4;
  const total = count + quiet * 2;

  let path = "";
  for (let row = 0; row < count; row++) {
    for (let col = 0; col < count; col++) {
      if (qr.isDark(row, col)) path += `M${col + quiet} ${row + quiet}h1v1h-1z`;
    }
  }

  return (
    <svg
      viewBox={`0 0 ${total} ${total}`}
      width={size}
      height={size}
      role="img"
      aria-label={label}
      shapeRendering="crispEdges"
      className={cn("shrink-0 rounded-lg", className)}
      data-testid="qr-code"
    >
      <rect width={total} height={total} fill="#ffffff" />
      <path d={path} fill="#09090b" />
    </svg>
  );
}
