# Funzionalità testate manualmente

Elenco delle funzionalità verificate in sessioni di test manuali e confermate come funzionanti.

## Preparazione v0.3.1 - 2026-11-04

Integrati gli aggiornamenti remoti del comando `password` con proxy Windows
e correzioni delle chiavi MFA. SSH, SCP, tmux e MobaXterm usano ora
`accept-new`; i test verificano i comandi generati, gli override espliciti
e i percorsi MobaXterm del file `known_hosts` condiviso.
La verifica offline con OpenSSH Windows conferma il parsing di `accept-new`
e dei percorsi con spazi. Non sono state aperte connessioni PSMP ne
modificati i file `known_hosts` operativi per questi controlli.

## Candidata locale MFA v9 - 2026-11-03

Il confronto con lo script v9 non ha richiesto cambiamenti ai protocolli
Identity, SAML o OIDC. I test automatici Windows verificano directory
alternative, selezione dei formati, rinnovo forzato e ACL native delle
chiavi nuove e sostituite; test completi e vet passano.
Sono state compilate candidate Windows amd64 e Linux amd64.

La candidata Windows usa WinHTTP per le impostazioni proxy automatiche di
sistema/utente e conserva la policy di autenticazione integrata `MEDIUM`.
Test Windows con server locali verificano richieste, body, header, redirect
e precedenza dei proxy d'ambiente. Non e stato eseguito un collaudo con il
proxy aziendale autenticato.

Il riesame dello ZIP ricevuto il 2026-11-03 conferma che lo script v9 e
le guide sono identici, byte per byte, ai materiali precedentemente
analizzati. Non introduce differenze al protocollo o ai formati.

Queste verifiche usano fixture e server locali: non sono stati eseguiti
login o download reali CyberArk. OIDC resta sperimentale.
Il binario e la configurazione dell'installazione operativa non vengono
sostituiti per il collaudo della candidata.

## Milestone v0.3.0 - 2026-10-02

La release **v0.3.0 è confermata funzionante su Windows e Linux**.
Il collaudo manuale dell'utente su Linux, successivo alla pubblicazione,
conferma il flusso nativo completo **QR/push → SAML PVWA → download chiavi**,
non soltanto il rendering QR o i test automatici.

OIDC resta sperimentale; login nativo e console macOS non sono ancora
confermati dal vivo.

---

## ✅ Autenticazione e chiavi

| Funzionalità | Piattaforma | Note |
|---|---|---|
| Login browser SAML/MFA della release precedente | macOS, Windows | Verifica storica, non applicabile al nuovo flusso API |
| Script API nativo di riferimento QR/push + SAML + download | Windows | Confermato dall'utente; non equivale a una verifica del nuovo binario Go |
| Identity QR/push del nuovo binario Go | Linux | Scansione e approvazione push confermate; flusso completo verificato in v0.3.0 |
| Rendering QR compatto del nuovo binario Go | Linux | Visualizzazione e scansione confermate dall'utente |
| Nuovo login Go QR/push + SAML + download chiavi | Windows, Linux | Flusso completo confermato dall'utente; collaudo Linux di v0.3.0 il 2026-10-02 |
| Download chiavi OpenSSH, PEM, PPK della release precedente | macOS, Windows | |
| Validazione TTL chiavi (4h) | macOS, Windows | |
| Auto-login da `sogark ssh` se chiave scaduta | macOS, Windows | |

## ✅ Connessione SSH

| Funzionalità | Piattaforma | Note |
|---|---|---|
| `sogark ssh <ip>` | macOS | Connessione base via PSMP |
| `sogark ssh <nome-host>` | macOS | Risoluzione da hosts.yaml |
| `sogark ssh user@host` | macOS | Override utente target |
| Flag SSH nativi passati a ssh | macOS | `-L`, `-v`, ecc. |
| `sogark ssh` con le chiavi del nuovo login API | Windows | Funzionamento confermato dall'utente |

## ✅ Trasferimento SCP

| Funzionalità | Piattaforma | Note |
|---|---|---|
| `sogark scp` upload singolo | macOS | |
| `sogark scp` con `#tag` inline | macOS | Batch upload su più host |
| `sogark scp` download con `#tag` | macOS | Crea sottocartelle per host |
| `sogark scp` con le chiavi del nuovo login API | Windows | Funzionamento confermato dall'utente |

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
| `sogark moba` con le chiavi del nuovo login API | Windows | Funzionamento confermato dall'utente |

## ✅ Gestione host

| Funzionalità | Piattaforma | Note |
|---|---|---|
| `sogark hosts add` con tag | macOS | |
| `sogark hosts list` con filtri | macOS | AND e OR |
| `sogark hosts import-moba` | macOS | Parser sessioni SSH MobaXterm |

## ✅ Configurazione

| Funzionalità | Piattaforma | Note |
|---|---|---|
| `sogark config init` della release precedente | macOS, Windows | Verifica storica del wizard |
| Blocco config legacy e nuovo editor del wizard | Windows | Confermato dall'utente con la candidata nativa |
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
| Altri target Windows | Build OK; verifica reale effettuata solo con la candidata amd64 |
| `sogark keys clean` | Implementato, non testato |
| Nuovo login Go QR/push + OIDC + cache chiavi | Coperto da test di contratto; utenza reale abilitata necessaria |
| Nuovo wizard di migrazione su macOS | Cross-build amd64/arm64 riuscite; verifica della console macOS ancora necessaria |

## Verifiche automatiche della nuova configurazione

| Funzionalità | Piattaforma | Note |
|---|---|---|
| Blocco configurazioni legacy e profili incompleti | Linux, Windows | Errore prima dei comandi; help, versione, wizard e helper update Windows restano accessibili |
| Migrazione guidata | Linux, Windows | Impostazioni mantenute, backup esatto e annullamento senza modifiche |
| Editor del wizard in console reale | Linux, Windows | Backspace, spazi, ripristino console; su Linux anche wizard completo in PTY da 45 colonne |
| Editor portabile | Linux, Windows | Delete, cursore, Unicode, CRLF, paste, Ctrl+C/Ctrl+D |

---

*Ultimo aggiornamento: 2026-11-04*
