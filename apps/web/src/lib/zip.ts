// Zip mínimo, só "store" (sem compressão), para o backup de todos os espaços do FinFlow no
// navegador num arquivo só. Bancos SQLite já são o conteúdo; compressão não vale uma dependência.
// Formato: APPNOTE.TXT (PKWARE) — cabeçalho local + dados por arquivo, diretório central e o
// registro final. Nomes em UTF-8 (bit 11). Sem ZIP64: cada arquivo e o total ficam abaixo de 4 GB.

export interface ZipEntry {
  name: string;
  data: Uint8Array<ArrayBuffer>;
}

let crcTable: Uint32Array | null = null;

function crc32(data: Uint8Array): number {
  if (!crcTable) {
    crcTable = new Uint32Array(256);
    for (let n = 0; n < 256; n++) {
      let c = n;
      for (let k = 0; k < 8; k++) c = c & 1 ? 0xedb88320 ^ (c >>> 1) : c >>> 1;
      crcTable[n] = c >>> 0;
    }
  }
  let crc = 0xffffffff;
  for (const byte of data) crc = (crcTable[(crc ^ byte) & 0xff] ?? 0) ^ (crc >>> 8);
  return (crc ^ 0xffffffff) >>> 0;
}

/** Data e hora no formato do MS-DOS (resolução de 2 s), como o zip espera. */
function dosDateTime(date: Date): { time: number; date: number } {
  return {
    time: (date.getHours() << 11) | (date.getMinutes() << 5) | Math.floor(date.getSeconds() / 2),
    date: ((date.getFullYear() - 1980) << 9) | ((date.getMonth() + 1) << 5) | date.getDate(),
  };
}

export function createZip(entries: ZipEntry[], now = new Date()): Blob {
  const encoder = new TextEncoder();
  const { time, date } = dosDateTime(now);
  const parts: BlobPart[] = [];
  const central: Uint8Array<ArrayBuffer>[] = [];
  let offset = 0;

  for (const entry of entries) {
    const name = encoder.encode(entry.name);
    const crc = crc32(entry.data);
    const size = entry.data.length;

    const local = new DataView(new ArrayBuffer(30));
    local.setUint32(0, 0x04034b50, true); // assinatura do cabeçalho local
    local.setUint16(4, 20, true); // versão necessária (2.0)
    local.setUint16(6, 0x0800, true); // nomes em UTF-8
    local.setUint16(8, 0, true); // método: store
    local.setUint16(10, time, true);
    local.setUint16(12, date, true);
    local.setUint32(14, crc, true);
    local.setUint32(18, size, true); // tamanho comprimido
    local.setUint32(22, size, true); // tamanho original
    local.setUint16(26, name.length, true);
    local.setUint16(28, 0, true); // extra
    parts.push(local.buffer, name, entry.data);

    const header = new DataView(new ArrayBuffer(46));
    header.setUint32(0, 0x02014b50, true); // assinatura do diretório central
    header.setUint16(4, 20, true); // feito por
    header.setUint16(6, 20, true); // versão necessária
    header.setUint16(8, 0x0800, true);
    header.setUint16(10, 0, true);
    header.setUint16(12, time, true);
    header.setUint16(14, date, true);
    header.setUint32(16, crc, true);
    header.setUint32(20, size, true);
    header.setUint32(24, size, true);
    header.setUint16(28, name.length, true);
    // 30: extra, 32: comentário, 34: disco, 36: atributos internos, 38: externos = 0
    header.setUint32(42, offset, true); // posição do cabeçalho local
    const record = new Uint8Array(46 + name.length);
    record.set(new Uint8Array(header.buffer), 0);
    record.set(name, 46);
    central.push(record);

    offset += 30 + name.length + size;
  }

  const centralSize = central.reduce((total, record) => total + record.length, 0);
  const end = new DataView(new ArrayBuffer(22));
  end.setUint32(0, 0x06054b50, true); // fim do diretório central
  end.setUint16(8, entries.length, true); // registros neste disco
  end.setUint16(10, entries.length, true); // registros no total
  end.setUint32(12, centralSize, true);
  end.setUint32(16, offset, true); // início do diretório central
  parts.push(...central, end.buffer);
  return new Blob(parts, { type: "application/zip" });
}

/**
 * Lê um zip "store" (o que `createZip` gera). Arquivos comprimidos são recusados com erro claro:
 * para restaurar, use o zip de backup do próprio FinFlow ou um `.db` solto.
 */
export function readZip(bytes: Uint8Array): ZipEntry[] {
  const view = new DataView(bytes.buffer, bytes.byteOffset, bytes.byteLength);
  let end = -1;
  for (let i = bytes.length - 22; i >= Math.max(0, bytes.length - 22 - 0xffff); i--) {
    if (view.getUint32(i, true) === 0x06054b50) {
      end = i;
      break;
    }
  }
  if (end < 0) throw new Error("arquivo .zip inválido");
  const count = view.getUint16(end + 10, true);
  let pos = view.getUint32(end + 16, true);
  const decoder = new TextDecoder();
  const entries: ZipEntry[] = [];
  for (let i = 0; i < count; i++) {
    if (view.getUint32(pos, true) !== 0x02014b50) throw new Error("arquivo .zip inválido");
    const method = view.getUint16(pos + 10, true);
    const size = view.getUint32(pos + 20, true);
    const nameLength = view.getUint16(pos + 28, true);
    const extraLength = view.getUint16(pos + 30, true);
    const commentLength = view.getUint16(pos + 32, true);
    const localOffset = view.getUint32(pos + 42, true);
    const name = decoder.decode(bytes.subarray(pos + 46, pos + 46 + nameLength));
    if (method !== 0)
      throw new Error(`"${name}" está comprimido; use o backup gerado pelo FinFlow`);
    const localName = view.getUint16(localOffset + 26, true);
    const localExtra = view.getUint16(localOffset + 28, true);
    const start = localOffset + 30 + localName + localExtra;
    entries.push({ name, data: bytes.slice(start, start + size) });
    pos += 46 + nameLength + extraLength + commentLength;
  }
  return entries;
}
