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
  - [sogark connect](#sogark-connect) — Connessione SSH
  - [sogark hosts](#sogark-hosts) — Registro macchine
  - [sogark multi](#sogark-multi) — Sessioni tmux parallele
  - [sogark exec](#sogark-exec) — Esecuzione parallela
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
sogark connect 10.1.2.3

# Cosa succede:
# → sogark verifica se c'è una chiave SSH valida su disco
# → se non c'è (o è scaduta), apre il browser per autenticazione SAML/MFA
# → scarica le chiavi SSH temporanee (4h) da CyberArk
# → si connette alla macchina via proxy PSMP
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

### sogark connect

Connessione SSH completa via PSMP proxy con autenticazione automatica.

```bash
sogark connect [user@]host [-- ssh-args...]
```

**Flusso:**

1. Se `host` è un nome registrato in `hosts.yaml`, ne risolve indirizzo e utente
2. Verifica che le chiavi SSH siano valide (non scadute)
3. Se scadute o assenti → autenticazione SAML/MFA automatica
4. Costruisce ed esegue il comando SSH nel formato PSMP:
   ```
   ssh utente@target_user@host@proxy -i chiave
   ```

**Flag:**

| Flag | Descrizione |
|------|-------------|
| `-u, --user <user>` | Override utente target sulla macchina remota |
| `--key-format <format>` | Formato chiave: `openssh` (default) o `pem` |
| `--force-login` | Forza ri-autenticazione |
| `--dry-run` | Mostra il comando SSH senza eseguirlo |

**Esempi:**

```bash
# Connessione base (utente target = default dalla config, es. root)
sogark connect 10.1.2.3

# Specifica utente target
sogark connect admin@10.1.2.3

# Usa un host registrato (risolve indirizzo e utente da hosts.yaml)
sogark connect myserver

# Port forwarding (argomenti dopo -- passati direttamente a ssh)
sogark connect 10.1.2.3 -- -L 8080:localhost:80

# Tunnel SOCKS
sogark connect 10.1.2.3 -- -D 1080

# Dry run: mostra il comando senza eseguirlo
sogark connect 10.1.2.3 --dry-run
# → ssh mario.rossi@root@10.1.2.3@psmp.sogei.it -i /Users/mario/.sogark/keys/id_sogark

# Usa chiave PEM invece di OpenSSH
sogark connect 10.1.2.3 --key-format pem

# Override utente con flag -u
sogark connect 10.1.2.3 -u admin
```

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

Apre una sessione tmux con un pannello SSH per ogni host selezionato, con input sincronizzato tra tutti i pannelli. Utile per eseguire comandi interattivi contemporaneamente su più macchine.

```bash
sogark multi [host...] [--tag tag] [--any-tag tag] [--no-sync]
```

**Richiede:** `tmux` installato (`brew install tmux` su macOS, `apt install tmux` su Linux).

**Esempi:**

```bash
# Sessione su tutti gli host con tag "production" (input sincronizzato)
sogark multi --tag production

# Sessione su host specifici
sogark multi web1 web2 db1

# Sessione con filtro OR
sogark multi --any-tag webservers,databases

# Senza sincronizzazione input (ogni pannello è indipendente)
sogark multi --tag production --no-sync
```

**Flag:**

| Flag | Descrizione |
|------|-------------|
| `--tag <tag>` | Seleziona host per tag (AND) |
| `--any-tag <tag>` | Seleziona host per tag (OR) |
| `--no-sync` | Disabilita `synchronize-panes` (input indipendente) |

**Come funziona:**

- Crea una sessione tmux chiamata `sogark`
- Apre un pannello per ogni host con la connessione SSH via PSMP
- Abilita `synchronize-panes`: tutto ciò che si digita viene inviato a tutti i pannelli
- Per uscire: `exit` in tutti i pannelli, oppure `Ctrl+B` poi `:kill-session`

---

### sogark exec

Esecuzione parallela di un comando su più host. L'output viene raccolto e visualizzato con prefisso `[hostname]`.

```bash
sogark exec [host...] <comando>
sogark exec --tag <tag> <comando>
```

**Esempi:**

```bash
# Uptime su tutti i webserver
sogark exec --tag webservers "uptime"

# Hostname su host specifici
sogark exec web1 web2 db1 "cat /etc/hostname"

# Stato servizio con filtro OR
sogark exec --any-tag web,db "systemctl status nginx"

# Comando più complesso
sogark exec --tag production "df -h / | tail -1"
```

**Output:**

```
Host selezionati: web1, web2, db1
[web1]  10:32:04 up 45 days,  3:21,  0 users,  load average: 0.12, 0.08, 0.05
[web2]  10:32:04 up 12 days,  1:05,  0 users,  load average: 0.45, 0.32, 0.28
[db1]   10:32:05 up 90 days,  7:14,  0 users,  load average: 0.03, 0.02, 0.01
✓ 3/3 host completati
```

Se alcuni host falliscono:

```
[web1] output...
[web2] errore: exit status 255
⚠ 1/2 host completati, 1 falliti
```

**Flag:**

| Flag | Descrizione |
|------|-------------|
| `--tag <tag>` | Seleziona host per tag (AND) |
| `--any-tag <tag>` | Seleziona host per tag (OR) |

**Nota:** quando si usano `--tag` o `--any-tag`, l'intero primo argomento è il comando. Senza flag tag, l'ultimo argomento è il comando e i precedenti sono nomi host.

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
