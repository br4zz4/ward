---
title: AI mode com nível masked, ward check e set --from
status: done
created: 2026-11-24
updated: 2026-11-24
owner: "@o_porpino"
certainty: high
---

# AI mode: nível masked, `ward check` e `ward set --from`

> **TLDR**: o AI mode ganha dois níveis, **strict** (o atual) e **masked** (novo default: `tok_…a9f2 (51 chars)`). Entram também `ward check`, que confere um valor sem revelá-lo, e `ward set --from`, que copia um secret sem revelá-lo. Antes disso, fecha o vazamento de `ward raw`/`ward export`, que hoje ignoram o AI mode.

# what

    Como usuário do ward trabalhando com um agente de IA, quero que o agente consiga identificar,
    conferir e copiar secrets sem nunca ler o valor, para que ele pare de travar nas recusas
    sem que o secret entre no contexto dele.

Termos em `docs/CONTEXT.md`: **AI mode**, **Strict**, **Masked**, **Value source**.

## Gatilhos e nível

Os gatilhos do AI mode não mudam. Ele liga com a flag `--ai-mode`, com `WARD_AI_MODE`, com a auto-detecção de agente na cadeia de processos pai (pulada quando o stdout é terminal) ou pelo MCP. O que muda é que todo AI mode ligado passa a ter um **nível**.

| `WARD_AI_MODE` | Efeito |
|---|---|
| `1` / `true` | liga; nível vem da config |
| `strict` | liga; força strict |
| `masked` | liga; força masked |
| `0` / `false` | desliga, inclusive a auto-detecção |
| `""` ou ausente | auto-detecção (como hoje); se ligar, nível vem da config |
| qualquer outro valor | **fail-closed**: liga em strict e avisa no stderr `ward: unknown WARD_AI_MODE value "<v>", using strict` |

- **Flag `--ai-mode`**: continua booleana e ganha de `WARD_AI_MODE=0|false`. O nível segue a regra abaixo.
- **Nível**: `WARD_AI_MODE=strict|masked` → chave de topo `ai_mode: strict|masked` em `.ward/config.yaml` → default **masked**.
- **Valor inválido em `ai_mode`** na config: strict, com aviso `ward: unknown ai_mode "<v>" in <config>, using strict`.
- **Sem config encontrada** (fora de um projeto): masked.
- **MCP**: continua injetando `WARD_AI_MODE=1` nos subprocessos, então o nível vem da config do projeto sem nada extra.

## Formato masked

`maskValue(v)` conta em **runes**, não em bytes. Com `n = len(runes)` e `k = min(4, n/8)` (divisão inteira):

| Valor | Saída |
|---|---|
| `n >= 8` | `<k primeiros>…<k últimos> (<n> chars)`. Ex.: 51 chars → `tok_…a9f2 (51 chars)`; 16 → `ab…yz (16 chars)`; 10 → `a…z (10 chars)` |
| `n < 8` | `… (<n> chars)` |
| contém `\n` (multi-line: PEM, JSON) | `<sensitive>` |

- Revela no máximo 25% do valor.
- Um valor TOTP (seed base32 ou URI `otpauth://`) é mascarado como valor armazenado. O código gerado nunca aparece.

## Comportamento por comando

| Comando | strict | masked |
|---|---|---|
| `tree`, `secrets` (e aliases `view`, `envs`) | `<sensitive>` | valor mascarado |
| `get` (folha ou subtree) | recusa (exit 3) | stdout com o valor mascarado (subtree: árvore com as folhas mascaradas) |
| `ward_get` (MCP) | JSON de instrução (como hoje) | JSON de instrução + campo `"masked"` |
| `get --json`, `secrets --json` | recusa | recusa |
| `file extract` | recusa | recusa |
| `raw` | recusa (exit 3) | recusa (exit 3) |
| `export <file>` (stdout) | recusa (exit 3) | recusa (exit 3) |
| `export <file> <out>` (disco) | permitido | permitido |
| `exec` | igual hoje (guard de dump de env) | igual hoje |
| `check` | permitido | permitido |
| `set --from` | permitido | permitido |

- **Exit 3** passa a ser o código de toda recusa do AI mode (`aiModeRefusal`). É assim que o MCP distingue "recusado" de "não encontrado".
- **Fora do AI mode** nada muda. O shell humano nunca ativa o AI mode sozinho: sem agente na cadeia de pais não há auto-detecção, e com stdout em terminal ela é pulada.

## `ward check <scope> <value-source>`

- Confere se o valor armazenado em `<scope>` (uma folha) é **exatamente igual** ao valor vindo do Value source:
  - argumento literal
  - `@<path>`
  - `-` (stdin)
  - `@@x`
  - `--literal`
- Mesmas regras de leitura do `ward set`: remove uma quebra final, rejeita vazio, rejeita não-UTF-8, limite de 1 MiB.
- Compara com o valor **armazenado**, nunca com o código OTP. A comparação é em tempo constante (`crypto/subtle`).
- Saída:
  - stdout `match` com exit `0`
  - stdout `differs` com exit `1`
  - qualquer erro (scope inexistente ou ambíguo, subtree, value source ilegível, fora de projeto): stderr com exit `2`, stdout vazio
- Nunca imprime nenhum dos dois valores, nem diff, tamanho ou posição.
- Scope não qualificado segue a regra do `get`: busca em todos os vaults e exige exatamente uma ocorrência.
- MCP `ward_check(path, value_file, dir)`:
  - executa `check -- <path> @<value_file>`
  - devolve resultado de sucesso com o texto `match` ou `differs`
  - exit 2 vira erro da tool
  - não aceita valor literal
- `value_file` relativo resolve contra o cwd do processo MCP. A descrição do parâmetro recomenda path absoluto.

## `ward set <dst> --from <src-scope>`

- Copia o valor **armazenado** de uma folha `<src-scope>` para `<dst>`, inclusive entre vaults: decripta com a chave do vault de origem e encripta com a do vault de destino.
- Com `--from`, o comando aceita só o scope de destino (ou nenhum posicional, se usar `--vault/--secret`).
- `--from` combinado com um argumento de valor ou com `--literal` dá erro antes de ler qualquer coisa.
- O source resolve como no `get`: qualificado direto; não qualificado exige uma única ocorrência.
- Erros, sempre com nada gravado:
  - source subtree ou inexistente
  - source igual ao destino: `source and destination are the same`
- Saída: `✓ set <dst> (from secret <src qualificado>)`.
- Funciona dentro e fora do AI mode e não imprime o valor.
- MCP `ward_set(path, value?, from?, dir)`: exatamente um entre `value` e `from` é obrigatório (os dois ou nenhum → erro da tool). `from` executa `set --from <from> -- <path>`; `value` mantém `set --literal -- <path> <value>`.

## Orientação para o agente

A recusa do `get` (`aiModeRefusal`), o JSON do `ward_get` e a seção AI mode do `wardDocs` passam a listar as três saídas, sem nunca sugerir `ward get`:

- **usar**: `ward exec -- sh -c '<command using the env var>'`
- **conferir**: `ward check <scope> @<file>` ou `<cmd> | ward check <scope> -`
- **copiar**: `ward set <dst> --from <src>`

## out of scope

- Override de nível por secret via `_meta` ou por vault.
- Comparar dois secrets internos entre si (`check` contra outro scope).
- Copiar subtree ou mover (`--from` + remoção da origem). Para mover, basta um `ward unset` depois.
- Flag `--ai-mode=<nível>`: a flag continua booleana.
- **Proteger contra um agente malicioso.** O agente consegue desligar o AI mode (`WARD_AI_MODE=0`) ou mudar `ai_mode` na config. O AI mode protege contra **exposição acidental**.
- `ward edit`/`ward new` com `EDITOR=cat` (ou similar) despejam plaintext em AI mode. É um achado desta entrevista: o builder abre uma issue (`br4zz4:issue`), e o problema não é tratado aqui.

## acceptance criteria

    Cenário: masked é o default sem config
      Dado um projeto sem `ai_mode` na config e um secret "tok_AbCdEfGhIjKlMnOpQrStUvWxYz0123456789abcdefXa9f2" (51 chars)
      Quando rodo `WARD_AI_MODE=1 ward secrets`
      Então a linha do secret mostra "tok_…a9f2 (51 chars)" e não contém o valor completo

    Cenário: config strict restaura o comportamento antigo
      Dado `ai_mode: strict` na config
      Quando rodo `WARD_AI_MODE=1 ward tree`
      Então os valores aparecem como "<sensitive>"
      E `WARD_AI_MODE=1 ward get app:k` sai com exit 3 sem o valor

    Cenário: env força o nível sobre a config
      Dado `ai_mode: strict` na config
      Quando rodo `WARD_AI_MODE=masked ward get app:k`
      Então o stdout é o valor mascarado
      E com `ai_mode: masked` na config, `WARD_AI_MODE=strict ward get app:k` sai com exit 3

    Cenário: true/false aceitos
      Quando rodo `WARD_AI_MODE=true ward secrets`
      Então os valores aparecem mascarados
      E `WARD_AI_MODE=false ward secrets` mostra os valores em claro

    Cenário alternativo: valor desconhecido na env é fail-closed
      Quando rodo `WARD_AI_MODE=masekd ward secrets`
      Então os valores aparecem como "<sensitive>"
      E o stderr contém `unknown WARD_AI_MODE value "masekd", using strict`

    Cenário alternativo: valor inválido em ai_mode é fail-closed
      Dado `ai_mode: loose` na config
      Quando rodo `WARD_AI_MODE=1 ward secrets`
      Então os valores aparecem como "<sensitive>" e o stderr contém `unknown ai_mode "loose"`

    Cenário: regra de revelação por tamanho
      Dado valores com 51, 16, 10, 8, 7 e 0 runes
      Então maskValue devolve 4+4, 2+2, 1+1, 1+1, só "… (7 chars)" e "… (0 chars)"
      E um valor com acentos/emoji é cortado por rune, nunca no meio de um caractere

    Cenário: multi-line fica sensitive no masked
      Dado um secret PEM com quebras de linha
      Quando rodo `WARD_AI_MODE=masked ward tree`
      Então esse valor aparece como "<sensitive>"

    Cenário: seed TOTP é mascarada, nunca o código
      Dado um secret TOTP com seed base32 de 32 chars
      Quando rodo `WARD_AI_MODE=masked ward secrets`
      Então aparece a seed mascarada (4+4, 32 chars) e não um código de 6 dígitos

    Cenário: --json e file extract continuam recusando no masked
      Quando rodo `WARD_AI_MODE=masked ward secrets --json`, `ward get --json app:k` e `ward file extract sa.json /tmp`
      Então todos saem com exit ≠ 0 sem emitir valor

    Cenário: MCP ward_get devolve o mascarado no masked
      Dado um projeto com `ai_mode: masked`
      Quando a tool ward_get recebe path "app:k"
      Então o JSON contém "masked": "tok_…a9f2 (51 chars)" e as instruções de exec/check/set --from
      E com `ai_mode: strict` o JSON não contém o campo "masked"
      E com path inexistente a tool devolve erro

    Cenário: raw recusa em AI mode
      Quando rodo `WARD_AI_MODE=1 ward raw` ou `WARD_AI_MODE=1 ward raw app.ward`
      Então o exit é 3, o stdout fica vazio e nenhum valor aparece
      E a tool MCP ward_raw devolve a recusa

    Cenário: export para stdout recusa, para arquivo é permitido
      Quando rodo `WARD_AI_MODE=1 ward export app.ward`
      Então o exit é 3 e o stdout fica vazio
      E `WARD_AI_MODE=1 ward export app.ward /tmp/out.yaml` grava o arquivo e sai com 0

    Cenário: check match
      Dado um arquivo /tmp/v com o mesmo valor de app:k seguido de "\n"
      Quando rodo `ward check app:k @/tmp/v`
      Então o stdout é "match" e o exit é 0

    Cenário: check differs via pipe
      Quando rodo `printf 'outro' | ward check app:k -`
      Então o stdout é "differs", o exit é 1 e nenhum valor aparece em stdout/stderr

    Cenário alternativo: check com erro
      Quando rodo `ward check app:nao.existe @/tmp/v`, `ward check app:grupo @/tmp/v` (subtree) ou `ward check app:k @/tmp/nao-existe`
      Então o exit é 2, o stdout fica vazio e o stderr explica o motivo

    Cenário: check funciona em AI mode strict
      Quando rodo `WARD_AI_MODE=strict ward check app:k @/tmp/v`
      Então o resultado é "match" com exit 0

    Cenário: MCP ward_check
      Quando a tool ward_check recebe path "app:k" e value_file "/tmp/v" com o mesmo valor
      Então o resultado é "match"
      E com valor diferente o resultado é "differs" (sem erro da tool)
      E com path inexistente a tool devolve erro

    Cenário: set --from copia entre vaults sem revelar
      Dado app:k no vault "app" e o vault "other" com outra chave
      Quando rodo `ward set other:k --from app:k`
      Então `ward get other:k` (fora do AI mode) retorna o mesmo valor de app:k
      E o stderr contém "(from secret app:k)" e não contém o valor

    Cenário alternativo: set --from inválido
      Quando rodo `ward set other:k valor --from app:k`, `ward set --literal other:k --from app:k`, `ward set other:k --from app:grupo` (subtree), `ward set other:k --from app:nao.existe` ou `ward set app:k --from app:k`
      Então o exit é ≠ 0 e nenhum arquivo .ward é criado ou alterado

    Cenário: MCP ward_set com from
      Quando a tool ward_set recebe path "other:k" e from "app:k"
      Então other:k recebe o valor de app:k
      E com value e from juntos, ou sem nenhum dos dois, a tool devolve erro sem gravar

    Cenário: recusa orienta as três saídas
      Quando rodo `WARD_AI_MODE=strict ward get app:k`
      Então o stderr cita `ward exec`, `ward check` e `ward set <dst> --from` e não cita `ward get`

    Cenário: shell do usuário nunca ativa sozinho
      Dado WARD_AI_MODE ausente e nenhum agente na cadeia de processos pai
      Quando rodo `ward secrets | cat`
      Então os valores aparecem em claro
      E com um agente na cadeia mas stdout em terminal os valores também aparecem em claro

    Cenário: plataforma
      Dado macOS ou Linux
      Então todos os cenários acima passam nos dois sistemas (detecção via `ps -ww -o ppid= -o command=`, paths via `filepath`)

# why

Hoje o AI mode é tudo ou nada. Sem ver nem um pedaço do valor, o agente não consegue confirmar se pegou o secret certo (`live_` ou `test_`, staging ou prod, token novo ou antigo). Ele também não tem como conferir ou copiar um valor sem lê-lo. O resultado: os agentes travam em recusas seguidas, se perdem, e o usuário acaba tendo que abrir o valor manualmente. O nível masked dá ao agente o suficiente para identificar o valor (no máximo 25%, prefixo + sufixo + tamanho), sem dar o suficiente para reproduzi-lo. `check` e `set --from` cobrem conferir e copiar sem leitura nenhuma.

O masked vira o default de propósito: é o comportamento útil, e a máscara foi feita para ser segura como default. Quem quiser o comportamento antigo põe `ai_mode: strict` na config. O release que muda o default destaca isso.

Na entrevista apareceu um furo no strict: `ward raw` e `ward export` para stdout ignoram o AI mode, e a tool MCP `ward_raw` devolve o YAML decriptado inteiro para o agente. Não faz sentido introduzir um nível "mais aberto" sobre um nível "fechado" que vaza, por isso o primeiro PR fecha o furo.

# how

Cinco PRs empilhados. Cada um vai para produção sozinho e gera release próprio, conforme o `AGENTS.md` do repo.

**Sem feature flag:**
- PR 1 só fecha um vazamento.
- PR 2 muda o default de propósito (decisão do usuário), e `ai_mode: strict` é a saída.
- PRs 3, 4 e 5 são aditivos.

**Sem telas** (CLI + MCP). `staging: none`, porque o repo não declara staging.

**Validação geral de cada PR:** `make test && make test.integration && make test.e2e`.

**Testes:** toda task segue TDD. O **tester** escreve os testes que falham a partir dos cenários Gherkin, e o builder/intern implementa. Arquivos de teste só o tester edita.

**Release após o merge de cada PR:**
- `git tag -a vX.Y.Z -m "..." origin/main && git push origin vX.Y.Z`
- instalação via **mise** (`mise use -g ward@X.Y.Z`). A regra global proíbe asdf, e o `AGENTS.md` do repo ainda cita asdf: o builder atualiza essa linha no PR 1.

## PR 1 — fechar raw/export em AI mode (branch: `fix/ai-mode-raw-export`, base: `main`)

**Entrega:** `raw` e `export` para stdout recusam em AI mode, e toda recusa sai com exit 3.
**Pronto para prod:** fora do AI mode nada muda. Em AI mode, só fecha a saída de plaintext.
**Release:** patch `v0.7.2` (a tag `v0.7.1` já existia).
**Valida:** `make test && make test.e2e`.

### Task 1.1: exit 3 nas recusas
**Arquivos:**
- Modify `internal/cmd/ai_mode.go`: `aiModeRefusal` passa a fazer `os.Exit(3)`; constante `exitAIRefusal = 3`
- Test `internal/cmd/ai_mode_test.go`

**Passos:** teste do código de saída via subprocess helper (falha) → implementação → passa → commit `fix: exit 3 on AI mode refusal`.
**Valida:** `go test ./internal/cmd -run TestAIModeRefusal`

### Task 1.2: raw e export recusam
**Arquivos:**
- Modify `internal/cmd/raw.go` e `internal/cmd/export.go`. `raw` checa `IsAIMode()` antes de decriptar. `export` só checa quando `len(args) == 1` (stdout). Mensagem: uma variante de `aiModeRefusal` que diz que o conteúdo decriptado nunca vai para o contexto e sugere `ward export <file> <out>` para gravar em disco.
- Test `test/e2e/raw/raw_test.go`, `test/e2e/export/export_test.go`

**Passos:** e2e dos cenários "raw recusa" e "export para stdout recusa" (falham) → implementação → passam → commit `fix: refuse raw and export to stdout in AI mode`.
**Valida:** `go test -tags e2e ./test/e2e/raw/... ./test/e2e/export/...`

### Task 1.3: MCP e docs
**Arquivos:**
- Modify `internal/mcp/server.go`: descrições de `ward_raw` e `ward_export` dizem "refused in AI mode (always, via MCP)"; `wardDocs` lista `raw` e `export` entre os que recusam.
- Modify `docs/configuration.md` (seção AI mode) e `AGENTS.md` (asdf → mise na seção Release).
- Test `internal/mcp/server_test.go`: `ward_raw` devolve erro sem conteúdo YAML.

**Passos:** teste MCP (falha) → implementação → passa → commit `fix: document raw/export refusal in AI mode`.
**Valida:** `go test ./internal/mcp`

## PR 2 — nível strict/masked (branch: `feat/ai-mode-masked-2`, base: `fix/ai-mode-raw-export`)

**Entrega:** níveis, config `ai_mode`, env estendida, `maskValue`, aplicação em `tree`/`secrets`/`get`/`ward_get`.
**Pronto para prod:** nível default masked (decidido). `ai_mode: strict` restaura o comportamento anterior.
**Release:** minor `v0.8.0`, com a mensagem da tag destacando "AI mode now defaults to masked; set `ai_mode: strict` to keep the previous behaviour".
**Valida:** `make test && make test.integration && make test.e2e`.

### Task 2.1: chave `ai_mode` na config
**Arquivos:**
- Modify `internal/config/config.go`: campo `AIMode string \`yaml:"ai_mode,omitempty"\`` em `Config`
- Test `internal/config/config_test.go`

**Passos:** teste de load com `ai_mode: masked`, `strict`, ausente e inválido (o valor bruto é preservado; a validação fica no cmd) (falha) → implementação → passa → commit `feat: add ai_mode config key`.
**Valida:** `go test ./internal/config`

### Task 2.2: `maskValue`
**Arquivos:**
- Create `internal/cmd/mask.go`: `maskValue(v string) string` com a tabela do `# what` (runes, `k = min(4, n/8)`, `n < 8` → `… (n chars)`, `\n` → `<sensitive>`)
- Test `internal/cmd/mask_test.go`

**Passos:** table-driven com 51, 16, 10, 8, 7, 0 runes, acentos/emoji e multi-line (falha) → implementação → passa → commit `feat: add masked value formatter`.
**Valida:** `go test ./internal/cmd -run TestMaskValue`

### Task 2.3: resolução de nível
**Arquivos:**
- Modify `internal/cmd/ai_mode.go`:
  - tipo `aiLevel` (`levelStrict`, `levelMasked`)
  - `IsAIMode()` passa a aceitar `1|true|strict|masked` como on e `0|false` como off; valor desconhecido = on + aviso único no stderr
  - nova `AILevel()`: env `strict|masked` → `cfg.AIMode` lido via `resolvedConfigFile()` + `config.Load`, com cache por processo; sem config → masked; valor inválido → strict + aviso único
  - `aiValue(v)`: strict → `<sensitive>`; masked → `maskValue(v)`
  - `aiModeReason()` inclui o nível
  - `printAIModeBanner()` cita `ai_mode: strict|masked`
- Test `internal/cmd/ai_mode_test.go`: os testes existentes continuam verdes, mais os casos novos da tabela de env e da config

**Passos:** testes (falham) → implementação → passam → commit `feat: resolve AI mode level from env and config`.
**Valida:** `go test ./internal/cmd -run 'TestIsAIMode|TestAILevel'`

### Task 2.4: aplicar o nível nos comandos
**Arquivos:**
- Modify `internal/cmd/secrets.go`: o transform da tabela usa `aiValue(v)` em AI mode, sem `otpValue`. Assim a seed é mascarada e o código nunca aparece.
- Modify `internal/cmd/get.go`: recusa só em strict. Em masked, folha imprime `aiValue(raw)` e subtree usa `printTreeWith` com `aiValue`. `--json` continua recusado em AI mode, em qualquer nível.
- Modify `internal/cmd/helpers.go`: confirmar que `tree` usa `aiValue` (já usa nas linhas 606 e 641) e que o corte da linha 492 continua pulado em AI mode.
- Test `internal/cmd/*_test.go` e `test/e2e/ai-mode/ai_mode_test.go`. Os e2e existentes que esperam `<sensitive>` com `WARD_AI_MODE=1` passam a usar `WARD_AI_MODE=strict` (ou fixture com `ai_mode: strict`), e ganham casos novos para masked.

**Passos:** e2e dos cenários de nível, TOTP, multi-line, `--json` e file extract (falham) → implementação → passam → commit `feat: apply masked level to tree, secrets and get`.
**Valida:** `go test -tags e2e ./test/e2e/ai-mode/... ./test/e2e/secrets/... ./test/e2e/tree/... ./test/e2e/get/... ./test/e2e/json/...`

### Task 2.5: MCP `ward_get` com masked
**Arquivos:**
- Modify `internal/mcp/server.go`:
  - `ward_get` com path não vazio executa `get <path>` (com `--dir`).
  - Exit 0: `aiGetInstruction(path, masked)` inclui `"masked": <stdout>`.
  - Exit 3: JSON sem `masked`, como hoje.
  - Outro erro: `fail(err)`.
  - `run` ganha uma variante que devolve o exit code (`runExit`).
  - Path vazio: instrução atual, apontando para `ward_tree`.
  - Descrições de `ward_get`/`ward_tree`/`ward_envs` citam o nível.
- Test `internal/mcp/server_test.go`

**Passos:** testes do cenário "MCP ward_get devolve o mascarado" (falham) → implementação → passam → commit `feat: return masked value from ward_get`.
**Valida:** `go test ./internal/mcp`

### Task 2.6: docs
**Arquivos:**
- Modify `docs/configuration.md`: seção AI mode com a tabela de env, `ai_mode`, formato masked e a tabela por comando
- Modify `README.md`: linha 264 e menções ao AI mode
- Modify `internal/mcp/server.go`: `wardDocs`

**Valida:** leitura do diff + `go test ./internal/mcp`

## PR 3 — `ward check` (branch: `feat/ai-mode-masked-3`, base: `feat/ai-mode-masked-2`)

**Entrega:** comando `check` + tool MCP `ward_check`.
**Pronto para prod:** comando novo, aditivo.
**Release:** minor `v0.9.0`.
**Valida:** `make test && make test.e2e`.

### Task 3.1: lookup de folha que devolve erro
**Arquivos:**
- Modify `internal/cmd/get.go`: extrair `lookupLeaf(eng, result, sc) (dotPath string, value string, err error)` a partir de `resolveGetTarget`, sem `os.Exit` (erros de not found, ambíguo e subtree viram `error`). `resolveGetTarget` passa a usar essa função e mantém o comportamento atual.
- Test `internal/cmd/get_test.go` (criar se não existir)

**Passos:** teste dos erros (falha) → refactor → passa → commit `refactor: extract leaf lookup returning errors`.
**Valida:** `go test ./internal/cmd -run TestLookupLeaf`

### Task 3.2: comando `check`
**Arquivos:**
- Create `internal/cmd/check.go`: `NewCheckCmd`
  - `Use: "check <scope> <value|@file|->"`, flag `--literal`, `registerScopeFlags`
  - reaproveita `splitSetArgs`/`resolveSetValue`/`resolvedConfigFile`/`lookupLeaf`
  - compara com `subtle.ConstantTimeCompare`
  - todo erro: stderr + `os.Exit(2)`, sem `fatal`
  - não checa AI mode
- Modify `cmd/ward/main.go`: registrar o comando
- Test `internal/cmd/check_test.go`, `test/e2e/check/check_test.go`

**Passos:** e2e dos cenários de check (falham) → implementação → passam → commit `feat: add ward check`.
**Valida:** `go test -tags e2e ./test/e2e/check/...`

### Task 3.3: MCP `ward_check` + docs
**Arquivos:**
- Modify `internal/mcp/server.go`: tool `ward_check(path, value_file, dir)` executando `check -- <path> @<value_file>` via `runExit` (0 → `ok("match")`, 1 → `ok("differs")`, outro → `fail`); `wardDocs`.
- Modify `README.md` (seção `### ward check`), `docs/configuration.md` (tabela por comando)
- Test `internal/mcp/server_test.go`

**Valida:** `go test ./internal/mcp`

## PR 4 — `ward set --from` (branch: `feat/ai-mode-masked-4`, base: `feat/ai-mode-masked-3`)

**Entrega:** `set --from` + parâmetro `from` no `ward_set`.
**Pronto para prod:** flag nova, aditiva; sem `--from`, o `set` não muda.
**Release:** minor `v0.10.0`.
**Valida:** `make test && make test.e2e`.

### Task 4.1: `--from` no `set`
**Arquivos:**
- Modify `internal/cmd/set.go`:
  - flag `--from <scope>`
  - com `--from`, `splitSetArgs` aceita só o destino
  - valor posicional ou `--literal` junto com `--from` dá erro antes de qualquer leitura
  - resolve a origem com `newEngine` + `MergeScoped("")` + `lookupLeaf`, e origem == destino (pelo path qualificado) dá erro
  - `reportSet` recebe source `secret <src qualificado>`
  - `Long` documenta `--from`
- Test `internal/cmd/set_test.go`, `test/e2e/set/set_test.go` (cópia entre vaults com chaves diferentes; casos inválidos com verificação de que nenhum `.ward` mudou)

**Passos:** e2e (falham) → implementação → passam → commit `feat: copy a secret with ward set --from`.
**Valida:** `go test -tags e2e ./test/e2e/set/...`

### Task 4.2: MCP `ward_set` com `from` + docs
**Arquivos:**
- Modify `internal/mcp/server.go`: `value` deixa de ser `Required`, entra `from`; exatamente um dos dois, senão `fail` sem executar; `from` → `set --from <from> -- <path>`; `wardDocs`.
- Modify `README.md` (seção `ward set`)
- Test `internal/mcp/server_test.go`

**Valida:** `go test ./internal/mcp`

## PR 5 — orientação ao agente (branch: `feat/ai-mode-masked-5`, base: `feat/ai-mode-masked-4`)

**Entrega:** recusa, `ward_get` e `wardDocs` listam exec / check / set --from.
**Pronto para prod:** só texto. Depende de `check` e `--from` já existirem (PRs 3 e 4).
**Release:** patch `v0.10.1`.
**Valida:** `make test && make test.e2e`.

### Task 5.1: mensagens
**Arquivos:**
- Modify `internal/cmd/ai_mode.go` (`aiModeRefusal`)
- Modify `internal/mcp/server.go` (`aiGetInstruction`: campo `instructions` com as três saídas; seção AI mode do `wardDocs`)
- Test `internal/cmd/ai_mode_test.go`, `internal/mcp/server_test.go`, `test/e2e/ai-mode/ai_mode_test.go`. A recusa contém `ward exec`, `ward check` e `--from`, e não contém `ward get`.

**Passos:** testes (falham) → implementação → passam → commit `feat: guide agents to exec, check and set --from`.
**Valida:** `go test ./internal/cmd ./internal/mcp && go test -tags e2e ./test/e2e/ai-mode/...`

## SEQUENCE
1. #<pr1> — fechar raw/export em AI mode
2. #<pr2> — nível strict/masked
3. #<pr3> — ward check
4. #<pr4> — ward set --from
5. #<pr5> — orientação ao agente
Merge in this order: #<pr1> #<pr2> #<pr3> #<pr4> #<pr5>

# definition of done

- [x] Todo cenário de "acceptance criteria" foi validado (automatizado ou verificado com comando exato)
- [x] Testes automáticos passando — suíte verde, não só a parte nova (`make test && make test.integration && make test.e2e`)
- [x] Toda task do "how" executada
- [x] Documentação atualizada (README, `docs/configuration.md`, `wardDocs`, `AGENTS.md`; `docs/CONTEXT.md` feito junto com esta spec)
- [x] Staging: `staging: none` (sem staging declarado no `AGENTS.md` do repo)
- [x] Cada PR da sequência validado isoladamente (build e testes verdes sem os PRs seguintes)
- [x] Último PR liga a funcionalidade e remove código temporário (não há código temporário nem flag)
- [x] Issue aberta para `ward edit`/`ward new` com `EDITOR=cat` em AI mode (`br4zz4:issue`): #41; achados do review: #42 (vault add/rotate-key imprimem a chave) e #43 (`ward --nope check` sai 1)
- [x] Sem config, `WARD_AI_MODE=1 ward secrets` mostra `tok_…a9f2 (51 chars)` (e2e)
- [x] `ai_mode: strict` → `<sensitive>` em tree e `get` com exit 3 (e2e)
- [x] `WARD_AI_MODE=strict|masked` sobrepõe a config nos dois sentidos (e2e)
- [x] `WARD_AI_MODE=true` liga e `false` desliga (unit + e2e)
- [x] Valor desconhecido na env → strict + aviso no stderr (unit + e2e)
- [x] `ai_mode` inválido na config → strict + aviso (unit + e2e)
- [x] `maskValue` com 51/16/10/8/7/0 runes, unicode e multi-line conforme a tabela (unit)
- [x] Multi-line em masked → `<sensitive>` (e2e)
- [x] TOTP em masked mostra a seed mascarada, nunca o código (e2e)
- [x] `--json` (secrets/get) e `file extract` recusam em masked (e2e)
- [x] `ward_get` MCP: `masked` presente em masked, ausente em strict, erro em path inexistente (`internal/mcp`)
- [x] `ward raw` (com e sem arquivo) em AI mode: exit 3, stdout vazio; `ward_raw` MCP recusa (e2e + `internal/mcp`)
- [x] `ward export <file>` em AI mode: exit 3; `ward export <file> <out>` grava e sai com 0 (e2e)
- [x] `ward check` match (0), differs (1) via `@file` e `-`, erros (2) com stdout vazio (e2e)
- [x] `ward check` funciona com `WARD_AI_MODE=strict` (e2e)
- [x] `ward_check` MCP: match, differs sem erro, erro em path inexistente (`internal/mcp`)
- [x] `ward set other:k --from app:k` copia entre vaults com chaves diferentes, stderr sem o valor (e2e)
- [x] `set --from` inválido (valor junto, `--literal`, subtree, inexistente, origem == destino): exit ≠ 0 e nenhum `.ward` alterado (e2e)
- [x] `ward_set` MCP com `from` copia; `value`+`from` ou nenhum → erro sem gravar (`internal/mcp`)
- [x] Recusa do `get` cita exec, check e `--from`, e não cita `ward get` (unit + e2e)
- [x] Shell humano: sem agente na cadeia, `ward secrets | cat` em claro; agente na cadeia + TTY em claro (testes existentes de `ai_mode_test.go` + e2e de `agent_detection_test.go` verdes)
- [x] Suíte verde em macOS e Linux (CI)
- [x] Releases `v0.7.2` (a `v0.7.1` já existia), `v0.8.0` (tag destaca o novo default), `v0.9.0`, `v0.10.0`, `v0.10.1` publicados e instalados via `mise use -g ward@<versão>`
- [x] Banner `FINISHED` impresso como última saída (`br4zz4:finish`, Step 7), só com todos os itens acima marcados

## documentation

- `docs/CONTEXT.md`: termos **AI mode**, **Strict** e **Masked** (feito junto com esta spec).
- `docs/configuration.md`: seção AI mode (env, `ai_mode`, formato masked, tabela por comando, `check`, `set --from`).
- `README.md`: menções ao AI mode, seções `ward check` e `ward set --from`.
- `internal/mcp/server.go` (`wardDocs`): AI mode com níveis e as três saídas.
- `AGENTS.md`: release instala via mise.
