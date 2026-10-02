# sogark — Guida utente

## Indice

- [Installazione](#installazione)
- [Prima configurazione](#prima-configurazione)
- [Riferimento configurazione](#riferimento-configurazione)
- [Comandi](#comandi)
- [Client esterni supportati](#client-esterni-supportati)
- [Esempi d'uso](#esempi-duso)
- [File di configurazione](#file-di-configurazione)

---

## Installazione

### Requisiti di sistema

| Piattaforma | Requisiti |
|-------------|-----------|
| macOS | Terminale interattivo e accesso a Identity/PVWA |
| Linux | Terminale interattivo; nessuna sessione grafica o browser richiesti |
| Windows | **Windows 10 o 11**; PowerShell built-in usato solo da funzionalità non autenticative |

### macOS / Linux

```bash
curl -fsSL https://github.com/Lotti/sogark/releases/latest/download/install.sh | bash
```

### Windows (PowerShell)

```powershell
irm https://github.com/Lotti/sogark/releases/latest/download/install.ps1 | iex
```

Lo script scarica il binario in `~/.sogark/bin/`, verifica il checksum SHA-256, prova a rimuovere i metadati di quarantena/MOTW dove possibile, aggiunge la directory al PATH e configura automaticamente `update_repo` per gli aggiornamenti futuri.

Per limitare i warning di Windows/macOS, usa preferibilmente questi script o `sogark update` invece di aprire direttamente il file scaricato dal browser.

### Versione specifica

```bash
VERSION=v1.2.0 curl -fsSL .../install.sh | bash
```

### Da sorgente

```bash
make build          # macOS/Linux → bin/sogark
make build-all      # cross-compile per darwin/linux/windows
```

### Aggiornamento

```bash
sogark update               # aggiorna all'ultima versione
sogark update --check       # controlla senza aggiornare
sogark update --version v1.2.0  # installa versione specifica
```

Ogni release include `checksums.txt` e `checksums.txt.bundle`, che contiene
la firma e il certificato Sigstore keyless. Installazione e aggiornamento
verificano il checksum SHA-256; la provenienza si puo verificare
separatamente tramite il bundle.

---

## Prima configurazione

```bash
sogark config init
```

Il wizard chiede tutti i parametri necessari. Nessun endpoint aziendale
è pre-compilato: URL, tenant e hostname vanno inseriti alla prima configurazione.
Il nome iniziale proposto è `primary` e il protocollo proposto è `saml`;
sono modificabili e non determinano alcun endpoint o tenant.

Dopo l'aggiornamento, una configurazione priva della nuova struttura viene
bloccata prima dei comandi applicativi. Eseguire `sogark config init`, oppure
`sogark --config <file> config init` per un file alternativo. Il wizard
mantiene username, PSMP, directory e nomi chiavi e altre impostazioni
esistenti; prima del salvataggio crea una copia `config.yaml.legacy-*.bak`.
Ctrl+C o fine dell'input annullano l'operazione senza modificare il file.

Il wizard supporta Backspace, Delete e spostamento del cursore. Gli spazi
iniziali e finali vengono rimossi, anche dai valori proposti; gli spazi
interni ai percorsi vengono mantenuti. Invio accetta il valore proposto.

Per modificare un singolo parametro:

```bash
sogark config set username mario.rossi
sogark config show
```

### Esempio configurazione generica SAML

Gli URL sono esempi, non endpoint preconfigurati. Devono essere comunicati
dall'amministratore CyberArk insieme al tenant e al protocollo corretto.

```yaml
username: user@example.com
auth_profile: primary
auth_profiles:
  primary:
    auth_type: saml
    tenant_id: EXAMPLE
    pvwa_base_url: https://vault.example.com/PasswordVault
    start_authentication_url: https://identity.example.com/Security/StartAuthentication
    advance_authentication_url: https://identity.example.com/Security/AdvanceAuthentication
    saml_bootstrap_url: https://vault.example.com/PasswordVault/api/auth/saml/logon
    saml_logon_url: https://vault.example.com/PasswordVault/API/auth/SAML/Logon/
    ssh_keys_cache_url: https://vault.example.com/PasswordVault/API/Users/Secret/SSHKeys/Cache
proxy_host: psmp.example.com
ssh_key_name: id_sogark
key_dir: ~/.sogark/keys
key_formats:
  - OpenSSH
  - PEM
  - PPK
key_ttl_hours: 4
auth_timeout_minutes: 2
default_ssh_user: root
default_scp_user: oper1
```

---

## Riferimento configurazione

| Chiave | Tipo | Default | Descrizione |
|--------|------|---------|-------------|
| `username` | stringa | — | Username aziendale per l'autenticazione |
| `auth_profile` | stringa | — | Nome del profilo attivo |
| `auth_profiles` | mappa | vuota | Tenant, protocollo e URL API completi per ciascun profilo |
| `proxy_host` | hostname | — | Hostname del PSMP proxy (es. `psmp.example.com`) |
| `ssh_key_name` | stringa | — | Nome base del file chiave SSH (es. `id_sogark`) |
| `key_dir` | path | `~/.sogark/keys` | Directory dove vengono salvate le chiavi SSH temporanee |
| `key_formats` | lista | `OpenSSH,PEM,PPK` | Formati chiave da scaricare |
| `key_ttl_hours` | intero | `4` | Durata in ore delle chiavi SSH temporanee |
| `auth_timeout_minutes` | intero | `2` | Timeout in minuti per QR/push e logon PVWA |
| `default_ssh_user` | stringa | — | Utente target SSH di default (es. `root`) |
| `default_scp_user` | stringa | — | Utente target SCP. Se vuoto, usa `default_ssh_user` |
| `moba_path` | path | auto-detect | Percorso eseguibile MobaXterm |
| `moba_max_sessions` | intero | `20` | Numero massimo di tab MobaXterm aperti da `sogark moba` |
| `tabby_path` | path | auto-detect | Percorso eseguibile Tabby |
| `winscp_path` | path | auto-detect | Percorso eseguibile WinSCP |
| `default_multi_backend` | stringa | `auto` | Backend default per `sogark multi` |
| `update_repo` | stringa | `Lotti/sogark` | Repository GitHub per self-update |

### Note sul `key_dir`

Il default `~/.sogark/keys` viene risolto automaticamente:
- macOS/Linux: `$HOME/.sogark/keys`
- Windows: `%USERPROFILE%\.sogark\keys`

---

## Comandi

### `sogark config`

```
sogark config init                          # wizard interattivo
sogark config show                          # mostra configurazione
sogark config set <key> <value>             # modifica parametro
sogark config wezterm                       # genera ~/.wezterm.lua per VM
```

### `sogark doctor`

```bash
sogark doctor                               # valida config e prerequisiti locali
```

### `sogark login`

```bash
sogark login                                # login SAML/MFA + scarica chiavi
sogark login --user altro.utente
sogark login --format pem
sogark --config ~/.sogark/private.yaml login
```

Il QR viene mostrato nel terminale; dopo la scansione, approva la notifica push.
Il rendering compatto dimezza larghezza e altezza rispetto ai blocchi interi,
senza modificare i moduli del QR o il bordo bianco. Se il terminale e troppo
piccolo, l'errore indica colonne e righe minime: ingrandiscilo o riduci il font.
Su Windows usa una console PowerShell o Windows Terminal: sogark legge le
dimensioni dall'output e abilita temporaneamente i colori del QR, senza WinForms.
Il meccanismo `OTP` di questo flusso viene avviato come push, non come codice
da digitare. Se QR manca, sogark non passa automaticamente a password, SMS o
un altro profilo.

Il flag globale `--config` seleziona un file alternativo senza cambiare
`config.yaml`. Per modificare un campo del profilo:

```bash
sogark --config ~/.sogark/private.yaml config set auth_profile primary
sogark --config ~/.sogark/private.yaml config set auth_profiles.primary.tenant_id EXAMPLE
```

Consulta la [guida di migrazione](native-api-migration.md) per OIDC, endpoint
richiesti, limiti del proxy e sostituzione delle configurazioni legacy.

### `sogark keys`

```bash
sogark keys                                 # verifica/scarica chiavi
sogark keys --dir ~/.ssh --format openssh   # output in directory specifica
sogark keys --force-login                   # forza login
sogark keys clean                           # elimina chiavi
sogark keys clean --yes                     # senza conferma
```

### `sogark ssh`

```bash
sogark ssh 10.1.2.3                         # connessione base
sogark ssh admin@10.1.2.3                   # utente target specifico
sogark ssh myserver                         # risolve da hosts.yaml
sogark ssh --dry-run 10.1.2.3               # preview
sogark ssh 10.1.2.3 -L 8080:localhost:80    # flag SSH nativi
```

### `sogark scp`

```bash
sogark scp file.txt 10.1.2.3:/tmp/          # upload singolo
sogark scp 10.1.2.3:/etc/hosts ./           # download
sogark scp file.txt #webservers:/tmp/       # batch con #tag
sogark scp file.txt oper1@#web#prod:/tmp/   # con utente
sogark scp --tag web file.txt :/tmp/        # batch con flag
sogark scp --dry-run file.txt 10.1.2.3:/tmp/
```

L'utente target SCP segue: flag `-u` → `default_scp_user` → `default_ssh_user`.

### `sogark hosts`

```bash
sogark hosts add web1 10.1.2.1 --tags web,prod
sogark hosts add db1 10.1.3.1 --user admin --tags db,prod
sogark hosts list
sogark hosts list --tag prod                       # AND
sogark hosts list --any-tag web,db                 # OR
sogark hosts remove web1
sogark hosts tag web1 --add critical --remove old

# Ricerca con wildcard
sogark hosts search "web*"
sogark hosts search --name "*db*" --ip "10.50.*"
sogark hosts search --tag prod --add-tag reviewed

# Import MobaXterm
sogark hosts import-moba sessions.mxtsessions
sogark hosts import-moba --dry-run sessions.mxtsessions
```

### `sogark multi`

```bash
sogark multi --tag production               # auto-detect backend
sogark multi #production                    # shorthand #tag
sogark multi oper1@#web#prod                # con utente
sogark multi web1 web2 db1                  # host espliciti
sogark multi --backend wezterm --tag prod   # forza backend
sogark multi --backend tabby --tag prod
sogark multi --no-sync --tag prod           # senza sync
```

Backend: `wezterm` (broadcast), `tabby`, `wt` (Windows Terminal), `tmux`.

### `sogark moba`

```bash
sogark moba --tag production
sogark moba web1 web2
sogark moba --moba-path "C:\Tools\MobaXterm.exe" --tag prod
```

### `sogark winscp`

```bash
sogark winscp 10.1.2.3
sogark winscp --tag production
sogark winscp --winscp-path "C:\WinSCP\WinSCP.exe" --tag prod
```

---

## Client esterni supportati

| Client | Comando | Piattaforma | Input sync |
|--------|---------|-------------|------------|
| WezTerm | `sogark multi --backend wezterm` | Tutte | ✅ broadcast |
| Tabby | `sogark multi --backend tabby` | Tutte | ❌ |
| Windows Terminal | `sogark multi --backend wt` | Windows | ❌ |
| tmux | `sogark multi --backend tmux` | macOS/Linux | ✅ synchronize-panes |
| MobaXterm | `sogark moba` | Windows | ✅ via MultiExec |
| WinSCP | `sogark winscp` | Windows | — (GUI SCP/SFTP) |

### WezTerm su VM con GPU limitata

```bash
sogark config wezterm
```

Genera `~/.wezterm.lua` con `prefer_egl = true` e keybinding clipboard. Se il file esiste già, stampa le righe da aggiungere.

Per la clipboard su Windows, aggiungere al file:

```lua
keys = {
  { key = 'c', mods = 'CTRL|SHIFT', action = wezterm.action.CopyTo('Clipboard') },
  { key = 'v', mods = 'CTRL|SHIFT', action = wezterm.action.PasteFrom('Clipboard') },
},
```

### MobaXterm — import sessioni

```bash
sogark hosts import-moba exported.mxtsessions
sogark hosts import-moba --tag extra --dry-run exported.mxtsessions
```

Le cartelle MobaXterm vengono convertite in tag sogark. Cartelle annidate (`A\B`) producono due tag separati: `a`, `b`.

---

## Esempi d'uso

### Workflow giornaliero

```bash
sogark login                               # rinnova chiavi (4h)
sogark ssh myserver                        # connessione rapida
sogark scp -r ./dist/ --tag web :/var/www/ # deploy su tutti i webserver
sogark multi --tag production              # multi-pane di produzione
```

### Gestione host

```bash
# Importa macchine da MobaXterm
sogark hosts import-moba sessions.mxtsessions

# Cerca e tagga in batch
sogark hosts search --ip "10.50.1.*" --add-tag legacy
sogark hosts search --name "*web*" --tag prod --remove-tag old
```

---

## File di configurazione

```
~/.sogark/
├── config.yaml      # Configurazione principale
├── hosts.yaml       # Registro macchine
└── keys/
    ├── id_sogark        # Chiave OpenSSH
    ├── id_sogark.pem    # Chiave PEM
    ├── id_sogark.ppk    # Chiave PuTTY/MobaXterm
    └── .key_timestamp   # Timestamp validità
```

Permessi: directory `0700`, chiavi `0600`.
