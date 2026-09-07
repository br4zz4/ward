---
title: "Geração de código OTP a partir de secrets"
status: proposed
created: 2026-09-07
updated: 2026-09-07
owner: "@gporpino"
certainty: high
---

# Geração de código OTP a partir de secrets

> **TLDR:** `ward get`, `secrets` e `exec` geram código TOTP de 6 dígitos automaticamente
> quando o valor é uma URI `otpauth://` ou um secret base32 (16/32/64 chars). `--raw`
> restaura o valor original. `tree` e `export` mantêm sempre o valor original.

## Contexto

Hoje o `ward get` imprime o valor bruto do secret — se for uma URI `otpauth://`, o
usuário vê a URL completa e ainda precisa de um app authenticator separado para gerar
o código. O ward já tem acesso ao secret descriptografado; ele mesmo pode gerar o
código TOTP e eliminar o passo extra.

Cenários-alvo:

- **Interativo:** `ward get vault:myapp.otp` → código de 6 dígitos para copiar e colar
  num formulário de 2FA.
- **Scripts/pipes:** `OTP=$(ward get vault:myapp.otp)` para uso em automação.
- **Execução:** `ward exec -- ./publish.sh` injeta o código como env var para comandos
  de curta duração (ex: `gem publish`, `npm publish`).

## Objetivos

- Detectar automaticamente valores que são secrets TOTP (URIs `otpauth://` e strings base32).
- Gerar o código de 6 dígitos no momento da consulta (RFC 6238, TOTP).
- `get`, `secrets` e `exec` exibem/injetam o código por padrão. `--raw` exibe o valor original.
- `tree` e `export` mantêm o valor original — são comandos de inspeção/backup.
- Flag `-v`/`--verbose` exibe metadados (issuer, account, algorithm, remaining time).
- MCP server sempre passa `--raw` — agentes precisam do dado armazenado, não de código efêmero.

## Fora de escopo

- HOTP (contador) — apenas TOTP (tempo).
- Suporte a Steam Guard TOTP (encoder steam).
- Geração de QR codes ou provisioning de novos secrets OTP.
- Alteração no comando `raw` (já exibe o YAML descriptografado completo).

## Mudanças

### Novo pacote: `internal/otp/`

| Arquivo | Descrição |
|---------|-----------|
| `internal/otp/otp.go` | `IsOTP(val string) bool`, `GenerateCode(val string, t time.Time) (string, error)`, `VerboseInfo(val string, t time.Time) string` |
| `internal/otp/otp_test.go` | Testes unitários para detecção, geração e verbose info |

**Detecção (`IsOTP`):**

- Prefixo `otpauth://` → true.
- String composta apenas por `[A-Z2-7]` com comprimento 16, 32 ou 64 → true (base32 nos
  tamanhos típicos de secret TOTP).
- Qualquer outro valor → false.

**Geração (`GenerateCode`):**

- Se `otpauth://`, parseia via `pquerna/otp` (respeita algorithm, digits, period da URI).
- Se base32 puro, usa defaults: SHA1, 6 dígitos, 30s.
- Se inválido, retorna erro (malformed URI, base32 inválido).

**Verbose (`VerboseInfo`):**

- Retorna string multilinha com: Issuer, Account (se disponíveis na URI), Algorithm,
  Digits, Period, Remaining (segundos restantes no ciclo atual).
- Para secret puro, omite Issuer e Account; mostra Algorithm, Digits, Period com
  anotação `(default)` e Remaining.

**Dependência:** `github.com/pquerna/otp` (adicionar ao `go.mod`).

### Flags globais: `--raw` e `--verbose` / `-v`

Registradas no root command (`cmd/ward/main.go`):

```go
rootCmd.PersistentFlags().Bool("raw", false, "show original value, skip OTP generation")
rootCmd.PersistentFlags().BoolP("verbose", "v", false, "show OTP metadata (issuer, algorithm, time remaining)")
```

### Comandos modificados

| Comando | Arquivo | Mudança |
|---------|---------|---------|
| `ward get` | `internal/cmd/get.go` | Na leaf (`node.Value`) e em cada leaf de subtree: se `!raw && otp.IsOTP(value)`, imprime `otp.GenerateCode(value)`. Se erro, imprime em stderr + fallback para valor original. Com `-v`, imprime `otp.VerboseInfo`. |
| `ward secrets` | `internal/cmd/secrets.go` | No output de cada valor: se `!raw && otp.IsOTP(value)`, mostra código. `-v` é ignorado neste comando (tabela não comporta metadados multilinha). |
| `ward exec` | `internal/cmd/exec.go` | Antes de injetar cada env var: se `!raw && otp.IsOTP(value)`, gera código e injeta código. Com `-v`, imprime metadados no stderr antes de rodar o comando. |
| `ward tree` | `internal/cmd/tree.go` | **Nenhuma mudança** — sempre valor original. |
| `ward export` | `internal/cmd/export.go` | **Nenhuma mudança** — sempre valor original. |

### MCP server

| Arquivo | Mudança |
|---------|---------|
| `internal/mcp/server.go` | `ward_get` tool: adiciona `--raw` aos args para que o MCP sempre receba o valor original. |

## Como verificar

1. **Detecção `otpauth://`:**
   ```bash
   ward set vault:test.otp "otpauth://totp/Test:user?secret=JBSWY3DPEHPK3PXP&issuer=Test"
   ward get vault:test.otp          # → código de 6 dígitos
   ward get vault:test.otp --raw    # → otpauth://totp/Test:user?secret=...
   ward get vault:test.otp -v       # → metadados + código
   ```

2. **Detecção base32:**
   ```bash
   ward set vault:test.otp2 "JBSWY3DPEHPK3PXP"
   ward get vault:test.otp2         # → código de 6 dígitos
   ```

3. **Não-OTP (token normal) não é afetado:**
   ```bash
   ward set vault:test.token "abc123-my-token"
   ward get vault:test.token        # → abc123-my-token (sem mudança)
   ```

4. **`ward secrets`:**
   ```bash
   ward secrets                     # → OTPs aparecem como código
   ward secrets --raw               # → OTPs aparecem como URI/secret original
   ```

5. **`ward exec`:**
   ```bash
   ward exec -- env | grep OTP      # → código de 6 dígitos na env var
   ward exec -v -- env | grep OTP   # → metadados no stderr
   ```

6. **`ward tree` e `ward export` mantêm original:**
   ```bash
   ward tree                        # → mostra URI/secret, nunca código
   ward export                      # → exporta URI/secret original
   ```

7. **Erro com valor malformado:**
   ```bash
   ward set vault:test.bad "otpauth://totp/Test?secret=INVALID!!!"
   ward get vault:test.bad          # → stderr: erro + stdout: valor original
   ```

8. **MCP retorna valor original:**
   ```bash
   ward --mcp                       # → ward_get tool sempre retorna valor original
   ```

9. **Subtree no get:**
   ```bash
   ward set vault:test.nested.otp1 "..."
   ward set vault:test.nested.otp2 "..."
   ward get vault:test.nested        # → árvore com códigos OTP em cada leaf
   ward get vault:test.nested --raw  # → árvore com URIs originais
   ```

10. **Testes unitários:**
    ```bash
    go test ./internal/otp/...
    ```

## Documentação

- `docs/configuration.md` — nova seção "OTP Code Generation" documentando:
  - Formatos suportados (`otpauth://` e secret base32)
  - Comportamento padrão (código) vs `--raw` (original)
  - Flag `-v`/`--verbose`
  - Limitação: código injetado via `exec` é estático, expira em 30s
  - Parâmetros padrão para secret puro (SHA1, 6 dígitos, 30s)