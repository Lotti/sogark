# sogark

CLI cross-platform per l'autenticazione CyberArk via SAML/MFA e la gestione di sessioni SSH tramite PSMP proxy.

Sostituisce gli script PowerShell Windows-only con un singolo binario compilato che funziona su **macOS**, **Linux** e **Windows**.

---

## Indice

- [Installazione](#installazione)
- [Quick start](#quick-start)
- [Comandi](#comandi)
  - [sogark config](#sogark-config)
  - [sogark login](#sogark-login)
  - [sogark keys](#sogark-keys)
  - [sogark ssh](#sogark-ssh)
  - [sogark scp](#sogark-scp)
  - [sogark hosts](#sogark-hosts)
  - [sogark multi](#sogark-multi)
  - [sogark moba](#sogark-moba)
  - [sogark winscp](#sogark-winscp)
- [Come funziona](#come-funziona)
- [Parametri di configurazione](#parametri-di-configurazione)
- [Struttura file](#struttura-file)
- [Build](#build)
- [Test](#test)

---

## Installazione

### Prerequisiti

- **Chrome** o **Chromium** (necessario per l'autenticazione SAML/MFA)
- **tmux** per `sogark multi` su macOS/Linux (opzionale)

### Da sorgente

```bash
git clone <repository-url>
cd sogark
make build          # → bin/sogark
make install        # → /usr/local/bin/sogark
```

### Cross-compile

```bash
make build-all
```

| File | Sistema |
|------|---------|
| `sogark-darwin-arm64` | macOS Apple Silicon |
| `sogark-darwin-amd64` | macOS Intel |
| `sogark-linux-amd64` | Linux x86_64 |
| `sogark-windows-amd64.exe` | Windows x86_64 |

---

## Quick start

```bash
# 1. Configurazione iniziale
sogark config init

# 2. Connessione SSH (autentica automaticamente se necessario)
sogark ssh 10.1.2.3

# 3. Trasferimento file
sogark scp file.txt 10.1.2.3:/tmp/

# 4. Multi-sessione parallela
sogark multi --tag production

# 5. MobaXterm (Windows)
sogark moba --tag production

# 6. WinSCP (Windows)
sogark winscp --tag production
```

---

## Comandi

### sogark config

```
sogark config init                      # wizard interattivo
sogark config show                      # mostra configurazione
sogark config set <key> <value>         # modifica parametro
sogark config wezterm                   # genera ~/.wezterm.lua per VM
```

Il wizard non ha valori aziendali pre-compilati — ogni campo va impostato alla prima esecuzione.

`config wezterm` genera un file WezTerm ottimizzato per VM con GPU limitata (`prefer_egl = true`) e keybinding clipboard. Se il file esiste già, stampa le righe da aggiungere manualmente.

---

### sogark login

Esegue l'autenticazione SAML/MFA e scarica le chiavi SSH temporanee.

```bash
sogark login
sogark login --user altro.utente
sogark login --format openssh,pem
```

| Flag | Descrizione |
|------|-------------|
| `-u, --user` | Override username aziendale |
| `-f, --format` | Formati chiave (CSV) |

---

### sogark keys

```bash
sogark keys                             # verifica/scarica chiavi
sogark keys --dir ~/.ssh --format pem   # output in directory specifica
sogark keys --force-login               # forza ri-autenticazione
sogark keys clean                       # cancella chiavi da disco
sogark keys clean --yes                 # senza conferma
```

| Flag | Descrizione |
|------|-------------|
| `-d, --dir` | Directory output |
| `-f, --format` | Formati da scaricare |
| `--force-login` | Forza login anche con chiave valida |
| `-y, --yes` | Salta conferma (solo `clean`) |

---

### sogark ssh

```bash
sogark ssh 10.1.2.3                         # connessione base
sogark ssh admin@10.1.2.3                   # utente target specifico
sogark ssh myserver                         # risolve da hosts.yaml
sogark ssh -u admin 10.1.2.3               # override utente
sogark ssh --key-format pem 10.1.2.3       # usa chiave PEM
sogark ssh --dry-run 10.1.2.3              # mostra comando senza eseguire
sogark ssh 10.1.2.3 -L 8080:localhost:80   # port forwarding (flag SSH nativi)
```

Flag sogark devono precedere l'host. Tutti i flag SSH standard sono passati direttamente a `ssh`.

| Flag | Descrizione |
|------|-------------|
| `-u, --user` | Override utente target |
| `--key-format` | `openssh` (default) o `pem` |
| `--force-login` | Forza ri-autenticazione |
| `--dry-run` | Solo preview |

---

### sogark scp

```bash
# Upload/download singoli
sogark scp file.txt 10.1.2.3:/tmp/
sogark scp 10.1.2.3:/etc/hosts ./

# Batch con #tag inline
sogark scp file.txt #webservers:/tmp/
sogark scp file.txt oper1@#web#prod:/tmp/
sogark scp #webservers:/etc/hosts ./configs/    # crea sottocartelle

# Batch con flag
sogark scp --tag webservers file.txt :/tmp/
sogark scp --any-tag web,db -r ./deploy :/opt/

# Dry run
sogark scp --dry-run file.txt #production:/tmp/
```

L'utente target SCP segue questa priorità: flag `-u` → `default_scp_user` → `default_target_user`.

| Flag | Descrizione |
|------|-------------|
| `-u, --user` | Override utente target |
| `--key-format` | `openssh` (default) o `pem` |
| `--force-login` | Forza ri-autenticazione |
| `--dry-run` | Solo preview |
| `--tag` | Batch AND |
| `--any-tag` | Batch OR |

---

### sogark hosts

```bash
# Aggiunta e gestione
sogark hosts add web1 10.1.2.1 --tags web,prod
sogark hosts add db1 10.1.3.1 --user admin --tags db,prod
sogark hosts remove web1
sogark hosts tag web1 --add critical --remove old

# Lista
sogark hosts list
sogark hosts list --tag prod              # AND
sogark hosts list --any-tag web,db        # OR

# Ricerca con wildcard
sogark hosts search "web*"
sogark hosts search --name "*db*" --ip "10.50.*" --tag prod
sogark hosts search --ip "10.0.*" --add-tag legacy --remove-tag old

# Import da MobaXterm
sogark hosts import-moba sessions.mxtsessions
sogark hosts import-moba --tag extra --dry-run sessions.mxtsessions
```

| Sottocomando | Flag |
|--------------|------|
| `add <name> <addr>` | `-u/--user`, `--tags`, `--putty` |
| `list` | `--tag`, `--any-tag` |
| `remove <name>` | — |
| `tag <name>` | `--add`, `--remove` |
| `search [pattern]` | `--name`, `--ip`, `--tag`, `--add-tag`, `--remove-tag` |
| `import-moba <file>` | `--tag`, `--dry-run` |

Ogni host aggiunto crea automaticamente un'entry in `~/.ssh/config`.

---

### sogark multi

Sessioni SSH parallele con pane sincronizzati.

```bash
sogark multi --tag production
sogark multi #production
sogark multi oper1@#web#prod
sogark multi web1 web2 db1
sogark multi --backend wezterm --tag prod
sogark multi --backend tabby --tag prod
sogark multi --no-sync --tag prod
```

| Backend | Piattaforma | Sync input |
|---------|-------------|------------|
| `wezterm` | Tutte | ✅ broadcast |
| `tabby` | Tutte | ❌ |
| `wt` | Windows | ❌ |
| `tmux` | macOS/Linux | ✅ `synchronize-panes` |

Auto-detect: WezTerm (se `$TERM_PROGRAM=WezTerm`) → Windows Terminal → Tabby → tmux.

| Flag | Descrizione |
|------|-------------|
| `--tag` | Seleziona host AND |
| `--any-tag` | Seleziona host OR |
| `--backend` | `auto`, `wezterm`, `tabby`, `wt`, `tmux` |
| `--no-sync` | Disabilita sincronizzazione |

---

### sogark moba

Sessioni SSH in MobaXterm (Windows). Auto-detect percorso o prompt interattivo.

```bash
sogark moba --tag production
sogark moba web1 web2
sogark moba --moba-path "C:\Tools\MobaXterm.exe" --tag prod
```

Limite sessioni configurabile con `moba_max_sessions` (default 20). Il percorso viene salvato in config dopo il primo prompt interattivo.

| Flag | Descrizione |
|------|-------------|
| `--tag` | Seleziona host AND |
| `--any-tag` | Seleziona host OR |
| `--moba-path` | Percorso MobaXterm.exe |

---

### sogark winscp

Sessioni SCP/SFTP in WinSCP (Windows). Usa automaticamente la chiave `.ppk`.

```bash
sogark winscp 10.1.2.3
sogark winscp --tag production
sogark winscp --winscp-path "C:\WinSCP\WinSCP.exe" --tag prod
```

| Flag | Descrizione |
|------|-------------|
| `--tag` | Seleziona host AND |
| `--any-tag` | Seleziona host OR |
| `--winscp-path` | Percorso WinSCP.exe |

---

## Come funziona

### Flusso di autenticazione

```
sogark CLI → Chrome (go-rod) → IDP SAML → utente fa MFA
                                              ↓
                                        SAMLResponse
                                              ↓
sogark CLI → PVWA /API/auth/SAML/Logon/ → token sessione
                                              ↓
sogark CLI → PVWA /API/Users/Secret/SSHKeys/Cache → chiavi SSH
                                              ↓
                                  salva su disco (4h TTL)
```

### Formato connessione PSMP

```
ssh <utente_aziendale>@<utente_target>@<host>@<proxy_psmp> -i <chiave>
```

---

## Parametri di configurazione

| Chiave | Tipo | Default | Descrizione |
|--------|------|---------|-------------|
| `username` | stringa | — | Username aziendale |
| `pvwa_base_url` | URL | — | URL base CyberArk PVWA |
| `idp_url` | URL | — | URL login IDP SAML |
| `proxy_host` | hostname | — | Proxy PSMP |
| `ssh_key_name` | stringa | — | Nome base file chiave |
| `key_dir` | path | `~/.sogark/keys` | Directory chiavi |
| `key_formats` | lista | `OpenSSH,PEM,PPK` | Formati chiave |
| `key_ttl_hours` | intero | `4` | Durata chiavi (ore) |
| `saml_timeout_minutes` | intero | `5` | Timeout autenticazione SAML |
| `default_target_user` | stringa | — | Utente target SSH di default |
| `default_scp_user` | stringa | — | Utente target SCP (fallback a `default_target_user`) |
| `moba_path` | path | auto-detect | Percorso MobaXterm.exe |
| `moba_max_sessions` | intero | `20` | Limite tab MobaXterm |
| `tabby_path` | path | auto-detect | Percorso Tabby |
| `winscp_path` | path | auto-detect | Percorso WinSCP.exe |

---

## Struttura file

```
~/.sogark/
├── config.yaml          # configurazione utente
├── hosts.yaml           # registro macchine con tag
└── keys/
    ├── id_sogark        # chiave OpenSSH
    ├── id_sogark.pem    # chiave PEM
    ├── id_sogark.ppk    # chiave PPK
    └── .key_timestamp   # timestamp per TTL
```

Permessi: directory `0700`, file chiave `0600`.

---

## Build

```bash
make build          # → bin/sogark
make build-all      # cross-compile 4 piattaforme
make install        # → /usr/local/bin
make clean          # pulisci bin/
```

## Test

```bash
make test           # go test ./...
go test ./... -v    # output verboso
go test ./internal/keys/... -run TestParse   # test singolo
```
