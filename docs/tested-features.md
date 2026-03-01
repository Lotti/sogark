# Funzionalità testate manualmente

Elenco delle funzionalità verificate in sessioni di test manuali e confermate come funzionanti.

---

## ✅ Autenticazione e chiavi

| Funzionalità | Piattaforma | Note |
|---|---|---|
| `sogark login` — SAML/MFA via Chrome | macOS, Windows | Flusso completo con MFA |
| Download chiavi OpenSSH, PEM, PPK | macOS, Windows | |
| Validazione TTL chiavi (4h) | macOS, Windows | |
| Auto-login da `sogark ssh` se chiave scaduta | macOS, Windows | |

## ✅ Connessione SSH

| Funzionalità | Piattaforma | Note |
|---|---|---|
| `sogark ssh <ip>` | macOS | Connessione base via PSMP |
| `sogark ssh <nome-host>` | macOS | Risoluzione da hosts.yaml |
| `sogark ssh user@host` | macOS | Override utente target |
| Flag SSH nativi passati a ssh | macOS | `-L`, `-v`, ecc. |

## ✅ Trasferimento SCP

| Funzionalità | Piattaforma | Note |
|---|---|---|
| `sogark scp` upload singolo | macOS | |
| `sogark scp` con `#tag` inline | macOS | Batch upload su più host |
| `sogark scp` download con `#tag` | macOS | Crea sottocartelle per host |

## ✅ Multi-pane

| Funzionalità | Piattaforma | Note |
|---|---|---|
| `sogark multi` — WezTerm broadcast | Windows | Input sincronizzato funzionante |
| WezTerm focus su pane broadcaster | Windows | Il focus va correttamente al pane [sogark] |
| WezTerm auto-exit quando pane chiusi | Windows | Il broadcaster esce quando Ctrl+D sugli SSH |
| WezTerm grid layout | Windows | Pane disposti correttamente |
| Uscita con Ctrl+D dal broadcaster | Windows | |

## ✅ MobaXterm

| Funzionalità | Piattaforma | Note |
|---|---|---|
| `sogark moba --tag` | Windows | Apertura multi-tab |
| MobaXterm auto-detect percorso | Windows | |
| MobaXterm prompt interattivo percorso | Windows | Salva in config |
| MobaXterm salvataggio `moba_path` | Windows | Persistente tra sessioni |
| MobaXterm backslash nel path chiave | Windows | Convertiti in forward slash |
| Delay tra tab MobaXterm | Windows | 2s delay, tutte le tab si aprono |

## ✅ Gestione host

| Funzionalità | Piattaforma | Note |
|---|---|---|
| `sogark hosts add` con tag | macOS | |
| `sogark hosts list` con filtri | macOS | AND e OR |
| `sogark hosts import-moba` | macOS | Parser sessioni SSH MobaXterm |

## ✅ Configurazione

| Funzionalità | Piattaforma | Note |
|---|---|---|
| `sogark config init` | macOS, Windows | Wizard interattivo |
| `sogark config set/show` | macOS, Windows | |
| `sogark config wezterm` | Windows | Genera file con `prefer_egl = true` |

---

## ❓ Non ancora testato manualmente

| Funzionalità | Note |
|---|---|
| `sogark multi --backend tabby` | Implementato, non testato |
| `sogark winscp` | Implementato, non testato |
| `sogark hosts search` | Implementato, coperto da test automatici |
| `sogark scp --any-tag` (OR batch) | Implementato, non testato |
| `default_scp_user` | Implementato, non testato |
| `moba_max_sessions` | Implementato, non testato |
| Cross-compile Windows | Build OK, non testato in runtime |
| Cross-compile Linux | Build OK, non testato in runtime |
| `sogark keys clean` | Implementato, non testato |

---

*Ultimo aggiornamento: marzo 2026*
