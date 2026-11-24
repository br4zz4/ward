# ward

## Release

Toda mudança mergeada na `main` sobe uma versão:

1. Tag anotada `vX.Y.Z` em `origin/main` (`git tag -a vX.Y.Z -m "..." origin/main && git push origin vX.Y.Z`); o workflow `release.yml` (goreleaser) publica.
2. Docs ou fix → patch (`0.6.x`); funcionalidade nova → minor (`0.x.0`); quebra de compatibilidade → avise o usuário antes.
3. Depois do release, instale na máquina do usuário: `mise use -g ward@<versão>`.
