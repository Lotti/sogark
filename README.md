# sogark

CLI cross-platform per l'autenticazione CyberArk via SAML/MFA e la gestione di sessioni SSH tramite PSMP proxy.

Sostituisce gli script PowerShell Windows-only con un singolo binario compilato che funziona su **macOS**, **Linux** e **Windows**.

---

## Indice

- [Installazione](#installazione)
- [Quick start](#quick-start)
- [Comandi](#comandi)
  - [sogark config](#sogark-config) — Configurazione
  - [sogark login](#sogark-login) — Autenticazione SAML/MFA
  - [sogark keys](#sogark-keys) — Gestione chiavi SSH
  - [sogark ssh](#sogark-ssh) — Connessione SSH
  - [sogark scp](#sogark-scp) — Trasferimento file via SCP
  - [sogark hosts](#sogark-hosts) — Registro macchine
  - [sogark multi](#sogark-multi) — Sessioni multi-pane parallele
  - [sogark moba](#sogark-moba) — Sessioni MobaXterm
- [Come funziona](#come-funziona)
- [Parametri di configurazione](#parametri-di-configurazione)
- [Struttura file](#struttura-file)
- [Build](#build)
- [Test](#test)

---

## Installazione

### Prerequisiti

- **Chrome** o **Chromium** installato (necessario per l'autenticazione SAML/MFA)
- **tmux** per il comando `multi` (opzionale, solo macOS/Linux)

### Da sorgente

```bash
git clone <repository-url>
cd cyberark-cli
make build
```

Il binario viene creato in `bin/sogark`. Per installarlo in `/usr/local/bin`:

```bash
make install
```

### Binari precompilati

```bash
make build-all
```

Produce binari per 4 piattaforme nella directory `bin/`:

| File | Sistema |
|------|---------|
| `sogark-darwin-arm64` | macOS Apple Silicon |
| `sogark-darwin-amd64` | macOS Intel |
| `sogark-linux-amd64` | Linux x86_64 |
| `sogark-windows-amd64.exe` | Windows x86_64 |

---

## Quick start

```bash
# 1. Prima configurazione (una volta sola)
sogark config init

# 2. Connessione a una macchina
sogark ssh 10.1.2.3

# Cosa succede:
# → sogark verifica se c'è una chiave SSH valida su disco
# → se non c'è (o è scaduta), apre il browser per autenticazione SAML/MFA
# → scarica le chiavi SSH temporanee (4h) da CyberArk
# → si connette alla macchina via proxy PSMP

# 3. Trasferimento file via SCP
sogark scp file.txt 10.1.2.3:/tmp/

# 4. Upload a tutti gli host con tag (sintassi #tag)
sogark scp file.txt oper1@#webservers:/tmp/

# 5. Multi-pane (tmux/WezTerm)
sogark multi #production

# 6. MobaXterm multi-tab
sogark moba #production
```

Tutto il flusso — autenticazione, download chiavi, connessione — è gestito in automatico con un singolo comando.

---

## Comandi

### sogark config

Gestione della configurazione di sogark. I dati vengono salvati in `~/.sogark/config.yaml`.

#### `sogark config init`

Wizard interattivo per la prima configurazione. I default Sogei sono precompilati: basta inserire il proprio username aziendale e premere Invio per tutto il resto.

```bash
$ sogark config init
Configurazione sogark
─────────────────────
Username aziendale: mario.rossi
PVWA Base URL [https://cyberark.sogei.it/PasswordVault]:
IDP URL [https://aag4837.my.idaptive.app/...]:
Proxy host [psmp.sogei.it]:
Directory chiavi [/Users/mario/.sogark/keys]:
Utente target di default [root]:
Formati chiave [OpenSSH,PEM,PPK]:

✓ Configurazione salvata in /Users/mario/.sogark/config.yaml
```

Se eseguito di nuovo, parte dalla configurazione esistente (per modificare solo ciò che serve).

#### `sogark config set <chiave> <valore>`

Modifica un singolo parametro senza rieseguire il wizard.

```bash
sogark config set username mario.rossi
sogark config set default_target_user admin
sogark config set key_ttl_hours 8
sogark config set key_formats "OpenSSH,PEM"
sogark config set key_dir /opt/mykeys
sogark config set proxy_host psmp2.sogei.it
```

#### `sogark config show`

Mostra la configurazione corrente.

```bash
$ sogark config show
username:            mario.rossi
pvwa_base_url:       https://cyberark.sogei.it/PasswordVault
idp_url:             https://aag4837.my.idaptive.app/login?yfirtnecp...
proxy_host:          psmp.sogei.it
key_dir:             /Users/mario/.sogark/keys
key_formats:         OpenSSH, PEM, PPK
default_target_user: root
ssh_key_name:        id_sogark
key_ttl_hours:       4
```

---

### sogark login

Autenticazione SAML/MFA e download delle chiavi SSH.

```bash
sogark login
```

**Cosa fa:**

1. Apre una finestra Chrome con la pagina di login IDP
2. L'utente esegue l'autenticazione (username, password, MFA) — timeout 5 minuti
3. sogark cattura la SAMLResponse dal DOM del browser
4. Invia la SAMLResponse al CyberArk PVWA per ottenere un token di sessione
5. Usa il token per scaricare le chiavi SSH temporanee
6. Salva le chiavi su disco con timestamp di validità

```bash
$ sogark login
⏳ In attesa autenticazione nel browser...
✓ Chiavi salvate:
    /Users/mario/.sogark/keys/id_sogark        (OpenSSH)
    /Users/mario/.sogark/keys/id_sogark.pem    (PEM)
    /Users/mario/.sogark/keys/id_sogark.ppk    (PPK)
  Scadenza: tra 4h
```

**Flag:**

| Flag | Descrizione |
|------|-------------|
| `-u, --user <username>` | Override dello username aziendale (per questa sessione) |
| `-f, --format <formati>` | Formati chiave da scaricare (es. `openssh,pem`) |

**Esempi:**

```bash
sogark login --user altro.utente
sogark login --format openssh          # solo formato OpenSSH
sogark login --format "pem,ppk"        # solo PEM e PPK
```

---

### sogark keys

Gestione delle chiavi SSH scaricate da CyberArk.

#### `sogark keys`

Verifica la validità delle chiavi. Se scadute o assenti, esegue il login e le riscarica.

```bash
$ sogark keys
✓ Chiave valida (scade tra 2h 34m)
```

```bash
$ sogark keys
⚠ Chiave scaduta o assente, avvio autenticazione...
✓ Chiavi salvate:
    /Users/mario/.sogark/keys/id_sogark        (OpenSSH)
```

**Flag:**

| Flag | Descrizione |
|------|-------------|
| `-d, --dir <directory>` | Directory output (override rispetto alla config) |
| `-f, --format <formati>` | Formati da scaricare (es. `openssh,pem,ppk`) |
| `--force-login` | Forza ri-autenticazione anche con chiave valida |

**Esempi:**

```bash
sogark keys --dir ~/.ssh --format openssh     # esporta chiave OpenSSH in ~/.ssh/
sogark keys --dir /tmp/deploy --format pem    # esporta PEM in /tmp/deploy/
sogark keys --force-login                     # rigenera chiavi anche se valide
```

#### `sogark keys clean`

Cancella le chiavi SSH scaricate e il file di timestamp.

```bash
$ sogark keys clean
Cancellare le chiavi in /Users/mario/.sogark/keys? [y/N] y
✓ Rimossi: id_sogark, id_sogark.pem, id_sogark.ppk, .key_timestamp
```

**Flag:**

| Flag | Descrizione |
|------|-------------|
| `-d, --dir <directory>` | Directory da pulire |
| `-y, --yes` | Salta la conferma |

```bash
sogark keys clean --yes                       # cancella senza chiedere
sogark keys clean --dir /tmp/deploy           # pulisci directory specifica
```

---

### sogark ssh

Connessione SSH completa via PSMP proxy con autenticazione automatica.

```bash
sogark ssh [sogark-flags] [user@]host [ssh-args...]
```

**Flusso:**

1. Se `host` è un nome registrato in `hosts.yaml`, ne risolve indirizzo e utente
2. Verifica che le chiavi SSH siano valide (non scadute)
3. Se scadute o assenti → autenticazione SAML/MFA automatica
4. Costruisce ed esegue il comando SSH nel formato PSMP:
   ```
   ssh utente@target_user@host@proxy -i chiave -o IdentitiesOnly=yes
   ```

Tutti i flag SSH standard sono supportati direttamente. I flag sogark devono precedere l'host.

**Flag sogark:**

| Flag | Descrizione |
|------|-------------|
| `-u, --user <user>` | Override utente target sulla macchina remota |
| `--key-format <format>` | Formato chiave: `openssh` (default) o `pem` |
| `--force-login` | Forza ri-autenticazione |
| `--dry-run` | Mostra il comando SSH senza eseguirlo |

**Esempi:**

```bash
# Connessione base (utente target = default dalla config, es. root)
sogark ssh 10.1.2.3

# Specifica utente target
sogark ssh admin@10.1.2.3

# Usa un host registrato (risolve indirizzo e utente da hosts.yaml)
sogark ssh myserver

# Port forwarding
sogark ssh 10.1.2.3 -L 8080:localhost:80

# Tunnel SOCKS
sogark ssh 10.1.2.3 -D 1080

# Verbose + disabilita host key checking
sogark ssh 10.1.2.3 -v -o StrictHostKeyChecking=no

# Dry run: mostra il comando senza eseguirlo
sogark ssh --dry-run 10.1.2.3
# → ssh mario.rossi@root@10.1.2.3@psmp.sogei.it -i ~/.sogark/keys/id_sogark -o IdentitiesOnly=yes

# Usa chiave PEM invece di OpenSSH
sogark ssh --key-format pem 10.1.2.3

# Override utente con flag -u
sogark ssh -u admin 10.1.2.3
```

---

### sogark scp

Trasferimento file via SCP attraverso PSMP proxy con autenticazione automatica.

```bash
sogark scp [sogark-flags] [scp-flags] source... target
```

Wrapper trasparente per `scp`: sogark si occupa di iniettare la chiave SSH (`-i`) e tradurre i path remoti nel formato PSMP. Tutti i flag nativi di scp sono supportati direttamente.

I path remoti (`host:path` o `user@host:path`) vengono riscritti automaticamente:
```
host:/path  →  corp@target@host@psmp:/path
```

**Sintassi `#tag` inline** — seleziona host per tag direttamente nel path remoto:
```
#tag1#tag2:/path       →  tutti gli host con tag1 AND tag2
user@#tag:/path        →  con override utente target
```

**Flag sogark** (devono precedere i flag scp):

| Flag | Descrizione |
|------|-------------|
| `-u, --user <user>` | Override utente target sulla macchina remota |
| `--key-format <format>` | Formato chiave: `openssh` (default) o `pem` |
| `--force-login` | Forza ri-autenticazione |
| `--dry-run` | Mostra il comando scp senza eseguirlo |
| `--tag <tag>` | Invia a tutti gli host con questo tag (AND) |
| `--any-tag <t1,t2>` | Invia a tutti gli host con almeno un tag (OR) |

**Esempi:**

```bash
# Upload file
sogark scp file.txt 10.1.2.3:/tmp/

# Upload con #tag (a tutti gli host del tag)
sogark scp file.txt #webservers:/tmp/
sogark scp file.txt oper1@#web#prod:/tmp/
sogark scp -r ./deploy oper1@#web#prod:/opt/app/

# Download con #tag (sottocartelle per host)
sogark scp #webservers:/etc/hosts ./configs/
# → crea ./configs/web1/hosts, ./configs/web2/hosts, ...

# Upload con flag --tag
sogark scp --tag webservers file.txt :/tmp/

# Download file singolo
sogark scp 10.1.2.3:/etc/hosts ./

# Con flag scp nativi
sogark scp -C -v -P 2222 file.txt 10.1.2.3:/tmp/

# Dry run
sogark scp --dry-run file.txt #production:/tmp/
```

**Nota:** SCP è ufficialmente supportato attraverso CyberArk PSMP. Usa lo stesso formato username e la stessa chiave SSH della connessione SSH.

---

### sogark hosts

Registro locale delle macchine con sistema di tag per organizzarle e selezionarle in batch.

I dati vengono salvati in `~/.sogark/hosts.yaml`. Ogni host aggiunto viene anche registrato automaticamente in `~/.ssh/config`, così è utilizzabile con qualsiasi client SSH (VSCode Remote-SSH, MobaXterm, ecc.).

#### `sogark hosts add <nome> <indirizzo>`

Registra un host. Se il nome esiste già, viene sovrascritto.

```bash
sogark hosts add web1 10.1.2.1 --tags webservers,production
sogark hosts add web2 10.1.2.2 --tags webservers,staging
sogark hosts add db1 10.1.3.1 --user admin --tags databases,production
sogark hosts add db2 10.1.3.2 --user admin --tags databases,staging
sogark hosts add cache1 10.1.4.1 --tags redis,production
```

**Flag:**

| Flag | Descrizione |
|------|-------------|
| `-u, --user <user>` | Utente target (default: dalla config, es. `root`) |
| `--tags <tag1,tag2>` | Tag separati da virgola |
| `--putty` | Crea anche sessione PuTTY nel registro Windows |

#### `sogark hosts list`

Lista gli host registrati con possibilità di filtrare per tag.

```bash
$ sogark hosts list
  cache1          root@10.1.4.1 [production, redis]
  db1             admin@10.1.3.1 [databases, production]
  db2             admin@10.1.3.2 [databases, staging]
  web1            root@10.1.2.1 [production, webservers]
  web2            root@10.1.2.2 [staging, webservers]

5 host
```

**Filtro AND** — tutti i tag devono corrispondere:

```bash
$ sogark hosts list --tag production
  cache1          root@10.1.4.1 [production, redis]
  db1             admin@10.1.3.1 [databases, production]
  web1            root@10.1.2.1 [production, webservers]

3 host
```

```bash
$ sogark hosts list --tag databases,production
  db1             admin@10.1.3.1 [databases, production]

1 host
```

**Filtro OR** — almeno un tag deve corrispondere:

```bash
$ sogark hosts list --any-tag redis,databases
  cache1          root@10.1.4.1 [production, redis]
  db1             admin@10.1.3.1 [databases, production]
  db2             admin@10.1.3.2 [databases, staging]

3 host
```

| Flag | Descrizione |
|------|-------------|
| `--tag <tag1,tag2>` | Filtro AND: l'host deve avere **tutti** i tag |
| `--any-tag <tag1,tag2>` | Filtro OR: l'host deve avere **almeno uno** dei tag |

#### `sogark hosts tag <nome>`

Aggiunge o rimuove tag da un host esistente.

```bash
sogark hosts tag web1 --add critical,rome
sogark hosts tag web1 --remove staging
```

| Flag | Descrizione |
|------|-------------|
| `--add <tag1,tag2>` | Tag da aggiungere |
| `--remove <tag1,tag2>` | Tag da rimuovere |

#### `sogark hosts remove <nome>`

Rimuove un host dal registro e dalla configurazione SSH.

```bash
sogark hosts remove web2
```

Rimuove anche l'entry corrispondente da `~/.ssh/config` e la sessione PuTTY (su Windows).

---

### sogark multi

Apre una sessione multi-pane con un pannello SSH per ogni host selezionato.

```bash
sogark multi [host...] [--tag tag] [--any-tag tag] [--backend auto|wezterm|wt|tmux] [--no-sync]
```

**Backend supportati:**
- **WezTerm** (`wezterm`) — auto-detect se eseguito dentro WezTerm. **Supporta input sincronizzato** via broadcast (digiti comandi nel pane di controllo, vengono inviati a tutti i pane SSH)
- **Windows Terminal** (`wt`) — fallback su Windows, split pane senza sync
- **tmux** — default su macOS/Linux con `synchronize-panes`

**Auto-detect:** dentro WezTerm → `wezterm`, altrimenti `wt` (Windows) o `tmux` (Unix).

**Esempi:**

```bash
# Con sintassi #tag (auto-detect backend)
sogark multi #production
sogark multi oper1@#web#prod

# Con flag --tag
sogark multi --tag production

# Sessione su host specifici
sogark multi web1 web2 db1

# Forza backend specifico
sogark multi --backend wezterm #production
sogark multi --backend tmux #production

# Senza sincronizzazione input
sogark multi --tag production --no-sync
```

**Flag:**

| Flag | Descrizione |
|------|-------------|
| `--tag <tag>` | Seleziona host per tag (AND) |
| `--any-tag <tag>` | Seleziona host per tag (OR) |
| `--backend <b>` | Backend: `auto` (default), `wezterm`, `wt`, `tmux` |
| `--no-sync` | Disabilita input sincronizzato |

**WezTerm broadcast mode:**
Quando usi il backend `wezterm`, il pane originale diventa un controller:
```
[+] WezTerm: 3 pane SSH aperti
    [pane 1] web1 (root@10.0.0.1)
    [pane 2] web2 (root@10.0.0.2)
    [pane 3] db1 (root@10.0.0.3)
[+] Broadcast attivo. Digita comandi (Ctrl+D per uscire):

[sogark] > uptime
[sogark] > df -h /
[sogark] > ^D
[+] Broadcast terminato. I pane SSH restano attivi.
```

**Tip WezTerm su Windows corporate:** se hai l'errore OpenGL, crea `~/.wezterm.lua`:
```lua
local wezterm = require 'wezterm'
return { front_end = "Software" }
```

---

### sogark moba

Apre MobaXterm con un tab SSH per ogni host selezionato. Stessa sintassi di `multi`.

```bash
sogark moba [host...] [--tag tag] [--any-tag tag] [--moba-path path]
```

**Esempi:**

```bash
# Con sintassi #tag
sogark moba #production
sogark moba oper1@#web#prod

# Con flag --tag
sogark moba --tag webservers

# Host specifici
sogark moba web1 web2 db1

# Percorso MobaXterm personalizzato
sogark moba --moba-path "C:\Tools\MobaXterm.exe" #production
```

Dopo l'apertura, attiva **MultiExec** per inviare comandi a tutti i tab: click destro su un tab → Multi-execution.

**Flag:**

| Flag | Descrizione |
|------|-------------|
| `--tag <tag>` | Seleziona host per tag (AND) |
| `--any-tag <tag>` | Seleziona host per tag (OR) |
| `--moba-path <path>` | Percorso esplicito di MobaXterm.exe |

---

## Come funziona

### Flusso di autenticazione

```
┌──────────┐    ┌────────────┐    ┌──────────┐    ┌──────────┐
│  sogark   │───▶│  Chrome     │───▶│  IDP      │───▶│ CyberArk │
│  CLI      │    │  (go-rod)  │    │  (SAML)   │    │  PVWA    │
└──────────┘    └────────────┘    └──────────┘    └──────────┘
     │                                                   │
     │  1. Apre browser con URL IDP                      │
     │  2. Utente esegue login + MFA                     │
     │  3. Cattura SAMLResponse dal DOM                  │
     │  4. POST SAMLResponse → Token sessione      ◀────┘
     │  5. POST Token → Chiavi SSH temporanee      ◀────┘
     │  6. Salva chiavi su disco
     │  7. Connessione SSH via PSMP proxy
     ▼
┌──────────┐
│  Server  │  ← ssh utente@target@host@proxy -i chiave
└──────────┘
```

### Formato connessione PSMP

CyberArk PSMP usa un formato speciale per lo username SSH:

```
ssh <utente_aziendale>@<utente_target>@<host_destinazione>@<proxy_psmp> -i <chiave>
```

Esempio concreto:

```
ssh mario.rossi@root@10.1.2.3@psmp.sogei.it -i ~/.sogark/keys/id_sogark
```

### Validità chiavi

Le chiavi SSH scaricate da CyberArk hanno una validità limitata (default: 4 ore). sogark tiene traccia della scadenza tramite un file `.key_timestamp`:

- **Chiave valida**: sogark la riusa senza ri-autenticazione
- **Chiave scaduta**: sogark apre automaticamente il browser per il login
- **`--force-login`**: forza la ri-autenticazione anche con chiave valida

### Integrazione SSH config

Quando si aggiunge un host con `sogark hosts add`, viene creata automaticamente una entry in `~/.ssh/config` delimitata da marcatori:

```
# --- sogark:web1 ---
Host web1
    HostName psmp.sogei.it
    User mario.rossi@root@10.1.2.1
    IdentityFile /Users/mario/.sogark/keys/id_sogark
# --- /sogark:web1 ---
```

Questo permette di usare gli host registrati con qualsiasi client SSH:

```bash
ssh web1                           # SSH nativo
code --remote ssh-remote+web1 .    # VSCode Remote-SSH
```

---

## Parametri di configurazione

| Chiave | Descrizione | Default |
|--------|-------------|---------|
| `username` | Username aziendale per l'autenticazione | *(vuoto)* |
| `pvwa_base_url` | URL base del CyberArk PVWA | `https://cyberark.sogei.it/PasswordVault` |
| `idp_url` | URL della pagina di login IDP (SAML) | URL Idaptive Sogei |
| `proxy_host` | Hostname del proxy PSMP | `psmp.sogei.it` |
| `key_dir` | Directory dove salvare le chiavi SSH | `~/.sogark/keys` |
| `key_formats` | Formati chiave da scaricare | `OpenSSH,PEM,PPK` |
| `default_target_user` | Utente target di default sulle macchine remote | `root` |
| `ssh_key_name` | Nome base dei file chiave | `id_sogark` |
| `key_ttl_hours` | Durata validità chiavi in ore | `4` |

---

## Struttura file

```
~/.sogark/
├── config.yaml          # configurazione utente
├── hosts.yaml           # registro macchine con tag
└── keys/
    ├── id_sogark        # chiave OpenSSH
    ├── id_sogark.pem    # chiave PEM
    ├── id_sogark.ppk    # chiave PPK (per PuTTY/Windows)
    └── .key_timestamp   # timestamp per controllo validità
```

I file chiave vengono creati con permessi `0600` (leggibili solo dall'utente). La directory `~/.sogark` viene creata con permessi `0700`.

---

## Build

```bash
make build           # build per la piattaforma corrente → bin/sogark
make build-all       # cross-compile per macOS/Linux/Windows
make install         # build + copia in /usr/local/bin
make clean           # pulisci bin/
```

La versione viene iniettata automaticamente dal commit git corrente tramite ldflags.

## Test

```bash
make test            # esegue tutti i test
go test ./... -v     # output verboso
```

I test coprono tutti i moduli interni: configurazione, parsing chiavi, storage, validatore TTL, registro host, generazione SSH config, costruzione comandi SSH, e API CyberArk (con mock HTTP server).
