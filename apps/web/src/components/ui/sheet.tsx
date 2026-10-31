import * as Dialog from "@radix-ui/react-dialog";
import { X } from "lucide-react";
import type { ReactNode } from "react";

/**
 * Bottom sheet no mobile, modal centralizado no desktop.
 * Sobe com gesto nativo de scroll e respeita safe-area.
 */
export function Sheet({
  open,
  onOpenChange,
  title,
  children,
}: {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  title: string;
  children: ReactNode;
}) {
  return (
    <Dialog.Root open={open} onOpenChange={onOpenChange}>
      <Dialog.Portal>
        <Dialog.Overlay className="sheet-overlay fixed inset-0 z-40 bg-black/60 backdrop-blur-[2px]" />
        <Dialog.Content
          className="sheet-content fixed inset-x-0 bottom-0 z-50 max-h-[92dvh] overflow-y-auto rounded-t-3xl border-t border-zinc-800 bg-zinc-900 pb-[max(1.25rem,env(safe-area-inset-bottom))] outline-none md:inset-x-auto md:top-1/2 md:bottom-auto md:left-1/2 md:w-full md:max-w-md md:-translate-x-1/2 md:-translate-y-1/2 md:rounded-3xl md:border md:pb-5"
          aria-describedby={undefined}
        >
          <div className="mx-auto mt-2.5 h-1 w-10 rounded-full bg-zinc-700 md:hidden" />
          <div className="flex items-center justify-between px-5 pt-3 pb-1 md:pt-5">
            <Dialog.Title className="text-base font-semibold text-zinc-100">{title}</Dialog.Title>
            <Dialog.Close
              className="flex h-9 w-9 cursor-pointer items-center justify-center rounded-full text-zinc-400 hover:bg-zinc-800 hover:text-zinc-100"
              aria-label="Fechar"
            >
              <X size={18} />
            </Dialog.Close>
          </div>
          <div className="px-5 pt-2">{children}</div>
        </Dialog.Content>
      </Dialog.Portal>
    </Dialog.Root>
  );
}
