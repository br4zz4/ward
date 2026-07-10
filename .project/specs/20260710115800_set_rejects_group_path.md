# ward set: rejeitar path que aponta para grupo

> TLDR: `ward set` deve falhar quando o dot-path aponta para um grupo (nó com filhos), evitando sobrescrever e perder todas as chaves filhas.

**Status:** proposed
**Created:** 2026-07-10
**Owner:** @oporpino

---

## Context

Atualmente `ward set app.main value` sobrescreve silenciosamente um grupo inteiro (`app.main`) por um valor escalar, apagando todos os filhos (`name`, `password`, `token`). O comportamento correto é rejeitar com erro claro antes de qualquer escrita.

## Objectives

- Detectar quando o dot-path resolve para um grupo nos arquivos carregados
- Abortar com mensagem de erro listando os filhos que seriam perdidos
- Não escrever nada no disco quando o path é um grupo

## Changes

- `internal/cmd/set.go` — após `resolveSetTarget`, checar se o path é um grupo via `Tree.Kind`; se for `KindGroup`, chamar `fatal` com mensagem descritiva
- `internal/cmd/set_test.go` — testes unitários cobrindo: grupo detectado, mensagem correta
- `test/e2e/set/set_test.go` — teste e2e: `ward set app.main value` falha com mensagem mencionando as chaves filhas

## How to verify

```bash
# fixture basic tem app.main.{name, password, token}
ward-dev set app.main qualquer-valor
# deve sair com código != 0 e stderr contendo "group" e os filhos
```

## Documentation

No documentation changes needed.
