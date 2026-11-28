# Fundamentos da política de versões

Mesmas fontes da skill `pc-remote-release` (pesquisadas em 16/10/2026), aplicadas ao FinFlow.

## Semantic Versioning 2.0.0 — <https://semver.org/>

- **Regra 3:** "Once a versioned package has been released, the contents of that version MUST NOT be modified."
  Versão publicada é imutável.
- **Regra 4:** "Major version zero (0.y.z) is for initial development. Anything MAY change at any time."
- **Regras 6–8:** PATCH para correções compatíveis, MINOR para funcionalidade compatível, MAJOR para mudança
  incompatível. Valem a partir da 1.0.0.
- **Regra 9/11:** pré-release (`-rc.1`) tem precedência menor que a versão normal; por isso `vX.Y.Z-rc.N` fica fora do
  "latest".
- **FAQ 1.0.0:** "If your software is being used in production, it should probably already be 1.0.0."
- **FAQ "lancei errado":** corrigir e lançar uma versão nova; "it is unacceptable to modify versioned releases".

## Convenção pré-1.0 de Cargo e release-please

- Cargo: <https://doc.rust-lang.org/cargo/reference/semver.html>: "Initial development releases starting with '0.y.z'
  can treat changes in 'y' as a major release, and 'z' as a minor release".
- release-please: <https://github.com/googleapis/release-please/blob/main/docs/manifest-releaser.md>:
  `bump-minor-pre-major` ("BREAKING CHANGE only bumps semver minor if version < 1.0.0") e
  `bump-patch-for-minor-pre-major` ("feat commits bump semver patch instead of minor if version < 1.0.0").

## Conventional Commits 1.0.0 — <https://www.conventionalcommits.org/en/v1.0.0/>

`fix` → PATCH, `feat` → MINOR, `BREAKING CHANGE:` ou `!` → MAJOR, sempre passando pelo deslocamento pré-1.0. O
repositório já usa `feat:`, `fix:`, `docs:`, `chore:` e escopos como `feat(api):`.

## Keep a Changelog 1.1.0 — <https://keepachangelog.com/en/1.1.0/>

Tipos de mudança em pt-BR: Novidades, Alterações, Descontinuado, Removido, Correções, Segurança. Uma entrada por
versão, a mais recente primeiro, com data, dizendo que segue SemVer.

## Git — "On Re-tagging" — <https://git-scm.com/docs/git-tag>

Uma tag publicada não se move: lança-se a próxima.

## Aplicado ao histórico do FinFlow

| Release | Conteúdo | Número publicado | Pela convenção pré-1.0 |
|---|---|---|---|
| 0.2.0 | primeira distribuição: binário Go único, planejamento, planilhas | 0.2.0 | 0.2.0 (troca da API NestJS pelo binário: quebra) |
| 0.3.0 | assistente, investimentos, dívidas, espaços; `serve` passa a escutar só em 127.0.0.1 | 0.3.0 | 0.3.0 (mudança de bind: quebra para quem usava a rede) |
| 0.4.0 | app no navegador; correção do duplo clique | 0.4.0 | 0.3.1 |
| 0.5.0 | assistente local (Ollama), `pnpm start`, Docker | 0.5.0 | 0.3.2 |
| 0.6.0 | remove os provedores locais do assistente, `/api/ai/config`, `/api/ai/models`, variáveis `FINFLOW_AI_*` e o profile `ai` do Docker | 0.6.0 | 0.6.0 (remoção de funcionalidade e de API: quebra) |

Até a 0.5.0 os números seguiram o FAQ do SemVer (MINOR a cada release em 0.y.z). Os números publicados **não são
renumerados**: quem tem a 0.5.0 só aceita versões maiores. A política vale a partir da 0.6.0: a próxima release
compatível é a 0.6.1, e só uma nova quebra leva à 0.7.0.
