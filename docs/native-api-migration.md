# Migrazione alle API native CyberArk

Il login non usa piu browser, WebView, WinForms o script PowerShell.
Usa Identity QR/push e una sessione PVWA SAML oppure OIDC, scelta tramite
un profilo esplicito. Non modifica automaticamente la configurazione legacy.

## Controllo dopo l'aggiornamento

All'avvio la CLI rifiuta file senza `auth_profile` e `auth_profiles` nella
nuova struttura. I comandi applicativi richiedono anche un profilo attivo
completo. Il messaggio indica `sogark config init`; help, versione e wizard
restano accessibili. `config set/show` possono gestire un file gia nella
nuova struttura mentre si completa la configurazione.

Per migrare esplicitamente il file predefinito:

```bash
sogark config init
```

Il wizard conserva i parametri non autenticativi, richiede tenant e URL
del profilo e crea un backup `config.yaml.legacy-*.bak` prima di salvare.
Non deduce gli endpoint dai vecchi URL. Annullamento, input incompleto o
validazione fallita non modificano il file originale.

## Preparare un file separato

Conservare il vecchio `config.yaml` e creare un file privato nuovo:

```bash
sogark --config ~/.sogark/private.yaml config init
sogark --config ~/.sogark/private.yaml config show
sogark --config ~/.sogark/private.yaml doctor
```

Usare permessi 0600 e non committare configurazioni operative.
Per le prove scegliere un `key_dir` diverso dalla directory chiavi attiva.
Il flag `--config` governa caricamento e salvataggio; senza flag rimane
il percorso predefinito `~/.sogark/config.yaml`.

## Profili e URL

`auth_profile` seleziona una voce di `auth_profiles`. Il nome e liberamente
scelto dall'utente e non determina tenant o protocollo.

Ogni profilo richiede:

| Campo | Valore |
|---|---|
| `auth_type` | `saml` oppure `oidc` |
| `tenant_id` | Identificativo comunicato dall'amministratore |
| `pvwa_base_url` | URL base HTTPS del PVWA |
| `start_authentication_url` | URL completo di StartAuthentication |
| `advance_authentication_url` | URL completo di AdvanceAuthentication |
| `ssh_keys_cache_url` | URL completo della cache chiavi |

Un profilo SAML richiede inoltre `saml_bootstrap_url` e `saml_logon_url`.
Possono identificare lo stesso endpoint, mantenendo la grafia e lo slash
finale richiesti dall'installazione.

Un profilo OIDC richiede invece `oidc_authorize_url` e `oidc_token_url`.
Esempio da affiancare al profilo SAML della guida utenti:

```yaml
auth_profiles:
  modern:
    auth_type: oidc
    tenant_id: EXAMPLE_OIDC
    pvwa_base_url: https://vault.example.com/PasswordVault
    start_authentication_url: https://identity2.example.com/Security/StartAuthentication
    advance_authentication_url: https://identity2.example.com/Security/AdvanceAuthentication
    oidc_authorize_url: https://vault.example.com/PasswordVault/api/auth/OIDC/identity/Authorize
    oidc_token_url: https://vault.example.com/PasswordVault/api/Auth/OIDC/identity/Token
    ssh_keys_cache_url: https://vault.example.com/PasswordVault/API/Users/Secret/SSHKeys/Cache
```

Tutti gli URL operativi sono configurati, non incorporati nel Go.
Gli endpoint Identity devono avere la stessa origine; gli endpoint PVWA
devono corrispondere all'origine del PVWA configurato. Sono richiesti HTTPS
e URL senza credenziali, query o frammenti; HTTP e ammesso solo su localhost
per i test.

Per cambiare profilo:

```bash
sogark --config ~/.sogark/private.yaml config set auth_profile modern
```

Non avviene alcun cambio automatico dopo un errore. Se l'utenza non e abilitata
su un tenant, selezionare esplicitamente un profilo su cui e abilitata.

## Flusso del login

1. StartAuthentication invia TenantId, User e Version con header nativo.
2. Sogark attiva il meccanismo QR con StartOOB e visualizza il PNG nel terminale.
3. Dopo la scansione, il polling rileva NewPackage/StartNextChallenge.
4. Il meccanismo OTP viene avviato con StartOOB per l'approvazione push.
5. LoginSuccess conclude Identity; non e richiesto Result.Token.
6. Il profilo SAML esegue bootstrap, GET dell'asserzione e secondo logon;
   quello OIDC esegue Authorize, GET del form e callback Token.
7. La sessione PVWA scarica le chiavi dalla cache con i propri header e cookie.
8. Tutti i formati richiesti devono essere presenti e riconoscibili prima
   del salvataggio e dell'aggiornamento del timestamp.

Il flusso richiede QR/push: se QR manca, l'errore chiede di verificare profilo,
tenant e abilitazione dell'utenza. Non viene offerto SMS come fallback.
`auth_timeout_minutes` controlla il timeout Identity/PVWA; il default e 2.

`idp_url`, `saml_timeout_minutes` e la precedente configurazione
Identity-only non sono sufficienti: gli URL e il tenant del profilo vanno
configurati esplicitamente, senza deduzioni dall'hostname.
Restano invariati username, formati, PSMP, directory, nomi file e TTL.

## Rete e protezione dati

Go usa HTTP_PROXY, HTTPS_PROXY e NO_PROXY. Non eredita automaticamente il
proxy di sistema Windows con DefaultNetworkCredentials. VPN e gateway
di accesso possono quindi richiedere una preparazione della rete.

Redirect fuori dalle origini configurate vengono rifiutati con un errore
azionabile. Non disabilitare TLS o aggiungere domini di gateway al sorgente
per aggirarlo. Il login del gateway e distinto dalla MFA Identity.
Anche un HTTP 200 puo contenere un form HTML di un gateway invece dell'URL
di bootstrap SAML: sogark lo segnala senza inoltrare il form. Un successo
QR/push non dimostra che l'accesso di rete al PVWA sia gia autorizzato.

Token, cookie, asserzioni, code/state e chiavi non vengono inclusi nei log,
neppure con SOGARK_DEBUG. Le query dei redirect e i body di errore non
vengono riportati. Il QR viene mostrato solo come immagine terminale.

La prova reale deve confermare QR, push e file chiave utilizzabili.
Una suite di mock o una risposta HTTP 200 non basta a dichiarare verificata
un'installazione CyberArk.

Il nuovo flusso Go QR/push, SAML e download chiavi e stato confermato dal vivo
su Windows e Linux. Il 2026-10-02 l'utente ha confermato il funzionamento
completo della release v0.3.0 anche su Linux, incluso il download chiavi.
La milestone e registrata in [Funzionalita testate](tested-features.md).
OIDC e implementato e coperto da test di contratto, ma resta sperimentale
finche non viene verificato con un'utenza abilitata.
