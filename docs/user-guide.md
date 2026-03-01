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

Il wizard chiede tutti i parametri necessari. Nessun valore è pre-compilato: tutti gli URL e hostname vanno inseriti manualmente alla prima esecuzione.

Per modificare un singolo parametro:

```bash
sogark config set username mario.rossi
sogark config show
```

### Esempio configurazione Sogei

> I valori seguenti sono specifici dell'ambiente Sogei.

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
default_scp_user: oper1
```

---

## Riferimento configurazione

| Chiave | Tipo | Default | Descrizione |
|--------|------|---------|-------------|
| `username` | stringa | — | Username aziendale per l'autenticazione |
| `pvwa_base_url` | URL | — | URL base del PVWA (es. `https://cyberark.example.com/PasswordVault`) |
| `idp_url` | URL | — | URL dell'Identity Provider SAML per il login MFA |
| `proxy_host` | hostname | — | Hostname del PSMP proxy (es. `psmp.example.com`) |
| `ssh_key_name` | stringa | — | Nome base del file chiave SSH (es. `id_sogark`) |
| `key_dir` | path | `~/.sogark/keys` | Directory dove vengono salvate le chiavi SSH temporanee |
| `key_formats` | lista | `OpenSSH,PEM,PPK` | Formati chiave da scaricare |
| `key_ttl_hours` | intero | `4` | Durata in ore delle chiavi SSH temporanee |
| `saml_timeout_minutes` | intero | `5` | Timeout in minuti per completare l'autenticazione SAML |
| `default_target_user` | stringa | — | Utente target SSH di default (es. `root`) |
| `default_scp_user` | stringa | — | Utente target SCP. Se vuoto, usa `default_target_user` |
| `moba_path` | path | auto-detect | Percorso eseguibile MobaXterm |
| `moba_max_sessions` | intero | `20` | Numero massimo di tab MobaXterm aperti da `sogark moba` |
| `tabby_path` | path | auto-detect | Percorso eseguibile Tabby |
| `winscp_path` | path | auto-detect | Percorso eseguibile WinSCP |

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

### `sogark login`

```bash
sogark login                                # login SAML/MFA + scarica chiavi
sogark login --user altro.utente
sogark login --format pem
```

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

L'utente target SCP segue: flag `-u` → `default_scp_user` → `default_target_user`.

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
