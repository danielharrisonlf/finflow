import type * as Client from "../src/runtime/client";

// No build do servidor (binário), `src/runtime/client` é trocado por este módulo (vite-plugin.ts):
// o app usa a API do FinFlow local e não carrega o worker nem o SQLite do navegador, que de outro
// modo entrariam no binário como arquivos sem uso. Os tipos seguem o cliente real, então uma
// mudança de assinatura lá quebra o typecheck aqui.

export type { RuntimeInfo } from "../src/runtime/client";

const unavailable = () =>
  Promise.reject(new Error("o runtime do navegador não existe no FinFlow instalado"));

export const isBrowserRuntime: typeof Client.isBrowserRuntime = false;
export const startRuntime: typeof Client.startRuntime = unavailable;
export const runtimeFetch: typeof Client.runtimeFetch = unavailable;
export const exportDatabase: typeof Client.exportDatabase = unavailable;
export const importDatabase: typeof Client.importDatabase = unavailable;
export const resetAllData: typeof Client.resetAllData = unavailable;
export const requestPersistentStorage: typeof Client.requestPersistentStorage = unavailable;
export const onRuntimeBlocked: typeof Client.onRuntimeBlocked = () => () => {};
export const onRuntimeLost: typeof Client.onRuntimeLost = () => () => {};
export const takeOver: typeof Client.takeOver = () => {};
