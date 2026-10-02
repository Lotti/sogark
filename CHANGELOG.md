# Changelog

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

Consultare [docs/native-api-migration.md](docs/native-api-migration.md).
I test di contratto dei due protocolli non sostituiscono la verifica reale.

### Stato della verifica reale

- Windows: confermati QR/push, SAML, download chiavi, SSH, SCP, MobaXterm
  e controllo config/input del wizard.
- Linux: confermati QR/push e rendering; il download completo nella rete
  di destinazione resta da verificare.
- OIDC: sperimentale, implementato e coperto da test automatici ma non
  ancora verificato con un'utenza reale abilitata.
- macOS: build e test portabili disponibili; console e login nativo
  restano da confermare dal vivo.
