# Changelog

## Unreleased

### Changed

- Su Windows il client HTTP nativo usa la configurazione proxy automatica
  WinHTTP e l'autenticazione integrata limitata dalla policy Windows
  `MEDIUM`; HTTP_PROXY/HTTPS_PROXY espliciti restano prioritari.

### Fixed

- `keys --dir` applicato sia alla verifica della cache sia al download,
  senza salvare gli override nella configurazione.
- Verifica dei formati richiesti prima di riutilizzare le chiavi;
  cache PEM/PPK utilizzabili anche senza OpenSSH, con rifiuto di file vuoti.
- SSH/SCP controllano il file selezionato; WinSCP controlla la chiave PPK.
- Chiavi Windows salvate con proprietario e DACL privata dell'utente
  corrente, senza ereditarieta. Ogni file viene preparato con permessi
  privati prima di sostituire quello esistente; su Unix resta il modo 0600.

### Documentation

- Documentata la compatibilita con lo script MFA v9 gia implementata
  e la distinzione tra registrazione dell'app e login.
- Chiarito l'isolamento di binari, home e configurazioni per le prove
  parallele e il possibile impatto server-side del rinnovo chiavi.
- OIDC resta sperimentale fino a un collaudo reale autorizzato.

## v0.3.0 - 2026-10-02

### Breaking change

Le configurazioni precedenti sono incompatibili con il nuovo login.
All'avvio viene richiesto `sogark config init`, oppure
`sogark --config <file> config init` per un file alternativo.
Il wizard conserva le impostazioni non autenticative e crea un backup
prima di salvare la migrazione. I file gia configurati con
`auth_profile`/`auth_profiles` non richiedono una nuova configurazione.

### Changed

- Sostituito il login browser con API Identity QR/push, senza WebView,
  WinForms, Chromium o esecuzione PowerShell per autenticarsi.
- Il logon SAML usa bootstrap PVWA, recupero dell'asserzione tramite HTTP
  e secondo logon prima della cache chiavi.
- Il ramo OIDC usa la sessione Identity condivisa, Authorize, form code/state,
  callback Token e sessione cookie/header PVWA.
- Aggiunti profili nominati con tutti gli URL API configurabili e selezione
  esplicita tramite auth_profile. Nessun fallback SMS o cambio automatico.
- Aggiunto il flag globale --config per file privati alternativi.
- Aggiunto il blocco iniziale delle configurazioni legacy con istruzioni
  per `config init`; il wizard conserva le impostazioni non autenticative
  e salva una copia del file originale prima di completare la migrazione.
- Corretto l'editor del wizard su Windows/Linux/macOS: dimensioni reali del
  terminale, cancellazione, spazi ai bordi, paste e annullamento senza salvataggio.
- QR terminale compatto: larghezza e altezza dimezzate, senza perdita di
  moduli; controllate anche le righe disponibili per evitare QR tagliati.
- Corretto il QR in PowerShell: dimensioni lette dall'handle di output
  Windows e colori ANSI abilitati temporaneamente, senza interfaccia grafica.
- Richiesti tutti i formati chiave prima del salvataggio e aggiornamento TTL.
- Rimossi riferimenti aziendali da esempi, fixture e link della documentazione.

### Security

- Rimossi i preview di chiavi private dai log di debug.
- Limitate le risposte HTTP e le dimensioni delle immagini QR.
- Validati HTTPS e origini dei redirect; redatti URL con stato e body di errore.
- Riconosciuti e bloccati anche i form HTML del bootstrap SAML diretti
  fuori dalle origini configurate, con diagnostica del prerequisito di rete.
- Preservata la configurazione legacy; la migrazione deve essere esplicita.

### Migration

Consultare la [guida di migrazione](https://github.com/Lotti/sogark/blob/v0.3.0/docs/native-api-migration.md).
I test di contratto dei due protocolli non sostituiscono la verifica reale.

### Stato della verifica reale

- Windows: confermati QR/push, SAML, download chiavi, SSH, SCP, MobaXterm
  e controllo config/input del wizard.
- Linux: confermato dall'utente il flusso completo QR/push, SAML PVWA e
  download chiavi con v0.3.0 il 2026-10-02, dopo la pubblicazione.
- OIDC: sperimentale, implementato e coperto da test automatici ma non
  ancora verificato con un'utenza reale abilitata.
- macOS: build e test portabili disponibili; console e login nativo
  restano da confermare dal vivo.
