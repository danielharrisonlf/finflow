import type { Space, SpaceKind } from "@finflow/contracts";
import { useEffect } from "react";
import { useSpaces } from "./queries";
import {
  DEFAULT_SPACE_SLUG,
  getSpaceKindHint,
  rememberSpaceKind,
  resetToDefaultSpace,
  useActiveSpaceSlug,
} from "./space";

export const SPACE_KIND_LABEL: Record<SpaceKind, string> = {
  PERSONAL: "Pessoal",
  HOME: "Casa",
  BUSINESS: "Empresa",
};

/** Espaço ativo com o tipo resolvido (cache local enquanto a lista não chega). */
export function useActiveSpace(): {
  slug: string;
  space: Space | undefined;
  kind: SpaceKind;
  spaces: Space[];
} {
  const slug = useActiveSpaceSlug();
  const spaces = useSpaces();
  const space = spaces.data?.find((s) => s.slug === slug);
  const kind = space?.kind ?? (slug === DEFAULT_SPACE_SLUG ? "PERSONAL" : getSpaceKindHint());
  return { slug, space, kind, spaces: spaces.data ?? [] };
}

/**
 * Chamado uma vez no AppShell: guarda o tipo do espaço e volta para o `pessoal` quando o
 * slug salvo não existe mais na lista da API.
 */
export function useSpaceGuard(): void {
  const slug = useActiveSpaceSlug();
  const spaces = useSpaces();
  useEffect(() => {
    if (!spaces.data) return;
    const space = spaces.data.find((s) => s.slug === slug);
    if (space) rememberSpaceKind(space.kind);
    else if (slug !== DEFAULT_SPACE_SLUG) resetToDefaultSpace();
  }, [spaces.data, slug]);
}
