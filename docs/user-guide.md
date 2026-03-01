# sogark — Guida utente

## Indice

- [Installazione](#installazione)
- [Prima configurazione](#prima-configurazione)
- [Riferimento configurazione](#riferimento-configurazione)
- [Comandi](#comandi)
- [Esempi d'uso avanzati](#esempi-duso-avanzati)

---

## Installazione

Scarica il binario per il tuo sistema operativo dalla pagina release, oppure compila:

```bash
make build          # macOS/Linux → bin/sogark
make build-all      # cross-compile per darwin/linux/windows
```

Aggiungi `bin/` al `PATH` oppure copia il binario in `/usr/local/bin` (macOS/Linux) o `C:\Windows\System32` (Windows).

---

## Prima configurazione

```bash
sogark config init
```

Il wizard interattivo chiede tutti i parametri necessari. I valori tra parentesi sono quelli attuali (se già configurati).

Puoi modificare un singolo parametro in qualsiasi momento:

```bash
sogark config set username mario.rossi
sogark config show
```

---

## Riferimento configurazione

| Chiave | Tipo | Default | Descrizione |
|--------|------|---------|-------------|
| `username` | stringa | — | Username aziendale usato per autenticarsi al PVWA |
| `pvwa_base_url` | URL | — | URL base del PVWA (es. `https://cyberark.example.com/PasswordVault`) |
| `idp_url` | URL | — | URL dell'Identity Provider SAML per il login MFA |
| `proxy_host` | hostname | — | Hostname del PSMP proxy (es. `psmp.example.com`) |
| `ssh_key_name` | stringa | — | Nome base del file chiave SSH (es. `id_sogark`) |
| `key_dir` | path | `~/.sogark/keys` | Directory dove vengono salvate le chiavi SSH temporanee |
| `key_formats` | lista | `OpenSSH,PEM,PPK` | Formati chiave da scaricare: `OpenSSH`, `PEM`, `PPK` |
| `key_ttl_hours` | intero | `4` | Durata in ore delle chiavi SSH temporanee |
| `saml_timeout_minutes` | intero | `5` | Timeout in minuti per completare l'autenticazione SAML |
| `default_target_user` | stringa | — | Utente target SSH di default (es. `root`) |
| `default_scp_user` | stringa | — | Utente target SCP di default. Se vuoto, usa `default_target_user` |
| `moba_path` | path | auto-detect | Percorso eseguibile MobaXterm (es. `C:\Tools\MobaXterm.exe`) |
| `moba_max_sessions` | intero | `20` | Numero massimo di tab MobaXterm aperti da `sogark moba` |
| `tabby_path` | path | auto-detect | Percorso eseguibile Tabby (es. `C:\Users\user\AppData\Local\Programs\Tabby\tabby.exe`) |
| `winscp_path` | path | auto-detect | Percorso eseguibile WinSCP (es. `C:\Program Files (x86)\WinSCP\WinSCP.exe`) |

### Note sul `key_dir`

Il default `~/.sogark/keys` viene risolto automaticamente:
- macOS/Linux: `$HOME/.sogark/keys`
- Windows: `%USERPROFILE%\.sogark\keys`

---

## Esempio configurazione Sogei

> ⚠️ I valori seguenti sono specifici dell'ambiente Sogei. Non inserirli se sei in un ambiente diverso.

```yaml
username: mario.rossi
pvwa_base_url: https://cyberark.sogei.it/PasswordVault
idp_url: https://aag4837.my.idaptive.app/login?yfirtnecapplogin=true&appKey=0f8346cb-fc6f-4ed4-9ebc-e2fcf5ae90c8&customerId=AAG4837&stateId=hFdfLAHPLkyZj2ml2B5cjMBjVjnT6AZd42pjywyZBoU1&yfirtnecrun=true
proxy_host: psmp.sogei.it
ssh_key_name: id_sogark
key_dir: ~/.sogark/keys
key_formats:
  - OpenSSH
  - PEM
  - PPK
key_ttl_hours: 4
saml_timeout_minutes: 5
default_target_user: root
```

---

## Comandi

### `sogark config`

```
sogark config init               # Wizard interattivo prima configurazione
sogark config show               # Mostra configurazione corrente
sogark config set <key> <value>  # Modifica un parametro
sogark config wezterm            # Genera ~/.wezterm.lua per VM con GPU limitata
```

### `sogark login`

Esegue solo l'autenticazione SAML/MFA e salva le chiavi SSH su disco.

```bash
sogark login
sogark login --format pem        # scarica solo formato PEM
```

### `sogark keys`

```
sogark keys show    # mostra chiavi presenti e scadenza
sogark keys clear   # elimina chiavi da disco
```

### `sogark ssh`

Connessione SSH a un host tramite PSMP. Autentica automaticamente se la chiave è scaduta.

```bash
sogark ssh 10.1.2.3
sogark ssh admin@10.1.2.3
sogark ssh myserver              # risolve il nome dal registro hosts
sogark ssh --tag production      # connessione al primo host con tag production
sogark ssh --dry-run 10.1.2.3   # mostra il comando senza eseguirlo
```

### `sogark scp`

Trasferimento file SCP tramite PSMP.

```bash
sogark scp file.txt 10.1.2.3:/tmp/
sogark scp 10.1.2.3:/etc/hosts ./
sogark scp --tag webservers file.txt :/tmp/     # upload su tutti gli host del tag
sogark scp #web:/etc/nginx.conf ./configs/      # download con #tag inline
sogark scp --dry-run file.txt 10.1.2.3:/tmp/
```

L'utente target per SCP segue questa priorità:
1. Flag `-u / --user`
2. Config `default_scp_user`
3. Config `default_target_user`

### `sogark hosts`

Gestione registro macchine locale (`~/.sogark/hosts.yaml`).

```bash
sogark hosts add myserver 10.1.2.3 --tag prod,web
sogark hosts list
sogark hosts list --tag prod
sogark hosts search --name "web*"
sogark hosts search --ip "10.50.*" --add-tag legacy
sogark hosts remove myserver
sogark hosts tag myserver --add prod --remove old
sogark hosts import-moba sessions.mxtsessions   # importa da MobaXterm
sogark hosts import-moba --dry-run sessions.mxtsessions
```

### `sogark multi`

Sessioni SSH parallele con input sincronizzato.

```bash
sogark multi --tag production
sogark multi web1 web2 db1
sogark multi --backend wezterm --tag prod
sogark multi --backend tabby --tag prod
sogark multi --no-sync --tag prod    # senza sincronizzazione input
```

Backend disponibili (auto-detect):
- `wezterm` — WezTerm con broadcast input
- `tabby` — Tabby terminal
- `wt` — Windows Terminal
- `tmux` — tmux

### `sogark moba`

Apre sessioni SSH in MobaXterm (Windows).

```bash
sogark moba --tag production
sogark moba web1 web2
sogark moba --moba-path "C:\Tools\MobaXterm.exe" --tag prod
```

### `sogark winscp`

Apre sessioni SCP/SFTP in WinSCP (Windows).

```bash
sogark winscp 10.1.2.3
sogark winscp --tag production
sogark winscp --winscp-path "C:\WinSCP\WinSCP.exe" 10.1.2.3
```

---

## Esempi d'uso avanzati

### Workflow tipico giornaliero

```bash
# Al mattino: rinnova le chiavi (durano 4 ore)
sogark login

# Connessione SSH rapida
sogark ssh myserver

# Upload deploy su tutti i webserver
sogark scp -r ./dist/ --tag web :/var/www/app/

# Apri sessione multi-pane su tutti i server di produzione
sogark multi --tag production
```

### Tag annidati (stile MobaXterm)

```bash
# Importa e usa le cartelle MobaXterm come tag sogark
sogark hosts import-moba --dry-run ~/Desktop/sessioni.mxtsessions
sogark hosts import-moba ~/Desktop/sessioni.mxtsessions

# Cerca host con più criteri
sogark hosts search --name "*web*" --tag prod
sogark hosts search --ip "10.50.1.*"
```

### WezTerm su VM con GPU limitata

```bash
# Genera configurazione ottimizzata per VM
sogark config wezterm

# Se il file esiste già, il comando stampa le righe da aggiungere manualmente
```

---

## File di configurazione

Tutti i file sogark sono in `~/.sogark/`:

```
~/.sogark/
├── config.yaml      # Configurazione principale
├── hosts.yaml       # Registro macchine
└── keys/
    ├── id_sogark        # Chiave OpenSSH
    ├── id_sogark.pem    # Chiave PEM
    └── id_sogark.ppk    # Chiave PuTTY/MobaXterm
```
