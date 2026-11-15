---
title: ward set lê valor de arquivo (@path) e stdin (-)
status: done
created: 2026-11-15
updated: 2026-11-15
owner: "@o_porpino"
certainty: high
---

# ward set: valor de arquivo (`@path`) e stdin (`-`)

> **TLDR**: `ward set` passa a ler o valor de um arquivo (`@<path>`) ou do stdin (`-`), com `@@` e `--literal` para gravar valores literais. Erros de leitura falham alto em vez de gravar `@/tmp/x` em silêncio.

# what

    Como usuário do ward, quero passar o valor de um segredo por arquivo (`@path`) ou stdin (`-`),
    para gravar valores longos/JSON/PEM sem corrompê-los e sem deixar o segredo no histórico do shell.

Tabela de **value source** (termo em `docs/CONTEXT.md`) do argumento `<value>` de `ward set`:

| Argumento | Sem `--literal` | Com `--literal` |
|---|---|---|
| `@<path>` | conteúdo do arquivo | `@<path>` literal |
| `@@<x>` | `@<x>` (remove só o primeiro `@`) | `@@<x>` literal |
| `-` | conteúdo do stdin | `-` literal |
| qualquer outro | literal (como hoje) | literal |

Regras para conteúdo lido de arquivo ou stdin:

- remove **exatamente uma** quebra de linha final (`\n` ou `\r\n`) e preserva as quebras internas;
- valor vazio, depois de remover a quebra, gera erro;
- bytes que não são UTF-8 válido geram erro: `value is not valid UTF-8; base64-encode binary data first`;
- mais de 1 MiB (1048576 bytes, medido antes de remover a quebra) gera erro;
- caminho relativo em `@<path>` é resolvido contra o diretório de invocação (cwd antes do `--dir`), não contra o diretório do `--dir`;
- `-` com stdin sendo um terminal gera erro `"-" reads from stdin, but stdin is a terminal — pipe a value (e.g. pbpaste | ward set <scope> -)`, sem prompt interativo;
- todo erro sai com exit ≠ 0 e **nada é gravado**.

Quando o valor vem de arquivo ou stdin, a linha de sucesso indica a origem, nunca o valor: `✓ set <scope> (from <path absoluto>)` / `✓ set <scope> (from stdin)`. Valor literal mantém `✓ set <scope>`.

A tool MCP `ward_set` sempre grava literal: invoca `set --literal -- <path> <value>`, e a descrição do parâmetro `value` diz que ele é gravado como está, sem interpretar `@` ou `-`.

## out of scope

- Migrar ou detectar valores `@...` já gravados por versões anteriores.
- Suporte a conteúdo binário (o usuário faz base64 antes).
- Prompt interativo sem eco para `-` em terminal.
- Mesma sintaxe em outros comandos (`new`, `import`, `override`, …).
- Valores que começam com `-` (ex.: `-abc`), que continuam sendo lidos como flag pelo cobra: o usuário usa `--` como hoje. A única exceção é o `--` que a tool MCP passa a usar.
- Expansão de `~` dentro de `@~/...` (é papel do shell; sem tratamento especial).

## acceptance criteria

    Cenário: valor lido de arquivo
      Dado um arquivo /tmp/v contendo "projects/1/locations/global/x\n"
      Quando rodo `ward set app:ci.WIF @/tmp/v`
      Então o exit é 0
      E `ward get app:ci.WIF` retorna "projects/1/locations/global/x" sem quebra de linha final
      E o stderr contém "(from /tmp/v)"

    Cenário: arquivo multilinha preserva quebras internas
      Dado um arquivo com um JSON de 3 linhas terminado em "\n"
      Quando rodo `ward set app:ci.SA_JSON @<arquivo>`
      Então o valor gravado é o JSON com as quebras internas e sem a quebra final

    Cenário: CRLF final é removido
      Dado um arquivo contendo "abc\r\n"
      Quando rodo `ward set app:k @<arquivo>`
      Então o valor gravado é "abc"

    Cenário alternativo: arquivo inexistente (o bug original)
      Dado que /tmp/nao-existe não existe
      Quando rodo `ward set app:k @/tmp/nao-existe`
      Então o exit é diferente de 0
      E o stderr contém "/tmp/nao-existe" e "--literal"
      E nenhum arquivo .ward é criado ou alterado

    Cenário alternativo: arquivo vazio
      Dado um arquivo contendo apenas "\n"
      Quando rodo `ward set app:k @<arquivo>`
      Então o exit é diferente de 0 e nada é gravado

    Cenário alternativo: conteúdo não UTF-8
      Dado um arquivo com bytes 0xff 0xfe
      Quando rodo `ward set app:k @<arquivo>`
      Então o exit é diferente de 0, o stderr menciona "UTF-8" e nada é gravado

    Cenário alternativo: arquivo acima de 1 MiB
      Dado um arquivo com 1048577 bytes
      Quando rodo `ward set app:k @<arquivo>`
      Então o exit é diferente de 0, o stderr menciona o limite e nada é gravado

    Cenário: caminho relativo com --dir
      Dado que estou em /work, com o arquivo /work/v, e o projeto está em /proj
      Quando rodo `ward --dir /proj set app:k @v`
      Então o valor gravado é o conteúdo de /work/v

    Cenário: escape @@
      Quando rodo `ward set app:k @@org/x`
      Então o valor gravado é "@org/x"

    Cenário: --literal
      Quando rodo `ward set --literal app:k @/tmp/v`
      Então o valor gravado é "@/tmp/v" e nenhum arquivo é lido
      E `ward set --literal app:k -` grava "-"
      E `ward set --literal app:k @@x` grava "@@x"

    Cenário: --literal com flags de escopo
      Quando rodo `ward set --vault app --secret k --literal @x`
      Então o valor gravado é "@x"

    Cenário: valor lido do stdin
      Quando rodo `printf 's3cr3t\n' | ward set app:k -`
      Então o exit é 0, o valor gravado é "s3cr3t" e o stderr contém "(from stdin)"

    Cenário alternativo: stdin é terminal
      Dado que o stdin é um TTY
      Quando rodo `ward set app:k -`
      Então o exit é diferente de 0, o stderr sugere usar pipe e nada é gravado

    Cenário: valor literal comum continua igual
      Quando rodo `ward set app:k postgres://x`
      Então o valor gravado é "postgres://x" e o stderr não contém "(from"

    Cenário: MCP grava literal
      Quando a tool `ward_set` recebe value "@/etc/hosts"
      Então o valor gravado é "@/etc/hosts" e /etc/hosts não é lido

    Cenário: plataforma
      Dado macOS ou Linux
      Então todos os cenários acima passam nos dois sistemas (paths via `filepath`, sem dependência de shell)

# why

Na 0.5.1, `ward set vault:scope.KEY @/tmp/secret.val` sai com 0 e grava a string `@/tmp/secret.val`. O valor corrompido só apareceu em runtime: foi passado como `workload_identity_provider` ao `google-github-actions/auth` e resultou em 403 no GCP, sem nenhum aviso no momento da escrita. `@file` é convenção conhecida (curl, gh), então o usuário espera esse comportamento. Hoje o valor só pode ir como argumento, o que deixa o segredo no histórico do shell e no `ps`, e dificulta passar valores longos (JSON de service account, PEM). Ler de arquivo ou stdin, com falha alta em qualquer erro de leitura, resolve o bug e esses dois problemas.

Versão: **minor `0.7.0`** (feature nova). É uma quebra de comportamento para quem gravava `@...` de propósito: o comando passa a falhar com erro explícito, nunca a corromper em silêncio. A quebra fica registrada no README e na mensagem da tag.

# how

Um único PR. A mudança é coesa (uma camada: CLI `set` mais o adaptador MCP) e tem 4 arquivos de código.

## PR 1 — set lê valor de arquivo e stdin (branch: `feat/set-value-source`, base: `main`)

**Entrega:** value source em `ward set`, `--literal`, MCP literal e docs.
**Pronto para prod:** valores literais comuns mantêm o comportamento atual; os únicos casos que mudam são `@...` e `-`, como especificado.
**Valida:** `make test && make test.integration && make test.e2e`.

Sem telas (CLI).

### Task 1: diretório de invocação preservado antes do `--dir`
**Arquivos:** Modify `internal/cmd/dir_flag.go` — Test `internal/cmd/dir_flag_test.go`
**Entrega:** `applyDirFlag` grava em uma variável de pacote `invocationDir` o `os.Getwd()` anterior ao `os.Chdir` (também quando `dir == ""`) e expõe `resolveInvocationPath(p string) string`: retorna `p` se for absoluto, senão `filepath.Join(invocationDir, p)`, caindo no cwd atual se `invocationDir` estiver vazio.
**Passos:** o teste "com `--dir`, um path relativo resolve contra o cwd original" falha → implementação → teste passa.
**Valida:** `go test ./internal/cmd -run 'TestApplyDirFlag|TestResolveInvocationPath'`

### Task 2: resolução do value source
**Arquivos:** Create `internal/cmd/set_value.go` — Test `internal/cmd/set_value_test.go`
**Entrega:** `resolveSetValue(raw string, literal bool, stdin io.Reader, stdinIsTTY bool) (value, source string, err error)`:
- `literal` → `(raw, "", nil)`;
- prefixo `@@` → `(raw[1:], "", nil)`;
- prefixo `@` → abre `resolveInvocationPath(raw[1:])` e lê com `io.LimitReader(f, maxSetValueBytes+1)`; `source` = path absoluto;
- `raw == "-"` → erro se `stdinIsTTY`, senão lê o stdin com o mesmo limite; `source` = `"stdin"`;
- outro → `(raw, "", nil)`;
- conteúdo lido passa por `normalizeReadValue`: acima de `maxSetValueBytes` (1048576) dá erro de limite; remove um `\r\n` ou `\n` final; vazio dá erro; `!utf8.ValidString` dá erro de UTF-8;
- erro de abertura/leitura: `reading value from <path>: <err> (use --literal to store "<raw>" as-is)`.
**Passos:** testes table-driven para cada linha da tabela e regra do `# what` (falham) → implementação → passam.
**Valida:** `go test ./internal/cmd -run TestResolveSetValue`

### Task 3: integrar no comando `set`
**Arquivos:** Modify `internal/cmd/set.go` — Test `internal/cmd/set_test.go`
**Entrega:** flag bool `--literal` ("store the value as-is; do not read @file or - (stdin)"); `Use: "set [scope] <value|@file|->"`; `Long` documenta a tabela de value source. Logo após `splitSetArgs`, e **antes** de carregar o vault e escrever, chama `resolveSetValue(value, literal, os.Stdin, term.IsTerminal(int(os.Stdin.Fd())))`; em erro, `fatal(err)`. `reportSet` recebe `source` e acrescenta ` (from <source>)` quando não vazio.
**Passos:** teste de `reportSet` com e sem source (falha) → implementação → passa.
**Valida:** `go test ./internal/cmd -run 'TestSet|TestReportSet'`

### Task 4: MCP `ward_set` sempre literal
**Arquivos:** Modify `internal/mcp/server.go` — Test `internal/mcp/server_test.go`
**Entrega:** o handler invoca `run(dirArgs(dir, "set", "--literal", "--", path, value)...)`; a descrição do parâmetro `value` passa a ser "value to store as-is (never read from a file or stdin; '@' and '-' are literal)".
**Passos:** teste que confirma os args montados e/ou que `@/etc/hosts` é gravado literal (falha) → implementação → passa.
**Valida:** `go test ./internal/mcp`

### Task 5: e2e
**Arquivos:** Modify `test/e2e/set/set_test.go` (e fixtures em `test/e2e/fixtures/set/` se precisar)
**Entrega:** e2e com o binário real para cada cenário Gherkin, exceto "stdin é terminal" e "MCP", que ficam cobertos nas Tasks 2 e 4. Arquivos de valor em `t.TempDir()`; a ausência de escrita é verificada pela comparação do mtime/conteúdo dos `.ward` antes e depois.
**Valida:** `go test -tags e2e ./test/e2e/set/...`

### Task 6: documentação
**Arquivos:** Modify `README.md` (seção `### ward set <scope> <value>`); `docs/CONTEXT.md` já ganhou o termo **Value source**.
**Entrega:** tabela de value source, exemplos (`ward set app:ci.SA_JSON @sa.json`, `pbpaste | ward set app:k -`, `ward set --literal app:handle @org`), regras (quebra final, 1 MiB, UTF-8, path relativo ao cwd) e nota de compatibilidade da 0.7.0 ("values starting with `@` or equal to `-` are no longer stored literally; use `--literal` or `@@`").
**Valida:** leitura do diff do README.

### Release (techleader, após o merge)
Tag `v0.7.0` em `origin/main`, com mensagem que cita a quebra de comportamento, seguida de `asdf install ward 0.7.0 && asdf global ward 0.7.0` (ver `AGENTS.md`).

## SEQUENCE
1. #<pr1> — set lê valor de arquivo e stdin
Merge in this order: #<pr1>

# definition of done

- [x] Todo cenário de "acceptance criteria" foi validado (automatizado ou verificado com comando exato)
- [x] Testes automáticos passando — suíte verde, não só a parte nova (`make test && make test.integration && make test.e2e`)
- [x] Toda task do "how" executada
- [x] Documentação atualizada (README + `docs/CONTEXT.md`)
- [x] Staging: `staging: none` (sem staging declarado no `AGENTS.md` do repo)
- [x] `@<path>` grava o conteúdo do arquivo, sem a quebra final e com "(from <path>)" no stderr (e2e)
- [x] JSON multilinha preserva as quebras internas (e2e)
- [x] `\r\n` final é removido (unit)
- [x] Arquivo inexistente: exit ≠ 0, mensagem com path e `--literal`, nenhum `.ward` alterado (e2e)
- [x] Arquivo vazio ou só com `\n`: exit ≠ 0 e nada gravado (unit + e2e)
- [x] Conteúdo não UTF-8: exit ≠ 0, mensagem com "UTF-8" (unit)
- [x] Conteúdo > 1 MiB: exit ≠ 0, mensagem de limite (unit)
- [x] `--dir` com `@` relativo resolve contra o cwd de invocação (unit + e2e)
- [x] `@@x` grava `@x` (unit + e2e)
- [x] `--literal` grava `@...`, `@@...` e `-` como estão, inclusive com `--vault/--secret` (unit + e2e)
- [x] `printf 's3cr3t\n' | ward set app:k -` grava `s3cr3t` com "(from stdin)" (e2e)
- [x] `-` com stdin TTY: erro sugerindo pipe e nada gravado (unit via `stdinIsTTY=true`)
- [x] Valor literal comum mantém o comportamento e o stderr não contém "(from" (e2e)
- [x] MCP `ward_set` com `@/etc/hosts` grava literal (teste em `internal/mcp`)
- [x] Nenhum path hardcoded com `/`: uso de `filepath` (revisão); suíte verde em macOS e Linux (CI)
- [x] Release `v0.7.0` publicado e instalado via asdf (0.6.0 já existia; última era 0.6.2)

## documentation

- `README.md` — seção `ward set`: value source, exemplos, regras e nota de compatibilidade da 0.7.0.
- `docs/CONTEXT.md` — termo **Value source** (feito junto com esta spec).
