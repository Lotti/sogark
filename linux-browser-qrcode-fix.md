# Piano: autenticazione CyberArk Linux headless nel terminale

## Esito dell'analisi

L'errore Windows `PASWS035E` non si è ripresentato il giorno successivo e non
ci sono evidenze di una regressione nel client. Verrà considerato un errore
temporaneo lato CyberArk/IdP; il flusso PowerShell Windows non sarà modificato.

La parte Linux headless, invece, è stata verificata end-to-end sulla rete
aziendale:

- PVWA e IdP sono raggiungibili con TLS valido.
- Chromium headless parte con la sandbox Linux attiva e viene controllato via
  Chrome DevTools Protocol da go-rod.
- La pagina Idira reale espone un QR nativo PNG 147×147, corrispondente a una
  griglia QR 49×49 con moduli da 3 px.
- Il QR estratto dal browser e ristampato nel terminale è stato scansionato con
  successo dall'app Idira Identity.
- Il prototipo ha catturato la `SAMLResponse` dal browser headless tramite CDP.
- La stessa assertion è stata inviata a
  `/API/auth/SAML/Logon/` e PVWA ha restituito un token valido.
- Assertion SAML e token non sono stati stampati né salvati.

Il precedente tentativo headless, poi rimosso, trasformava genericamente il DOM
in prompt ma non gestiva il QR e impostava i campi con assegnazioni JavaScript
fragili. La nuova implementazione manterrà un browser reale per JavaScript,
cookie, polling e redirect, presentando nel TTY soltanto l'interfaccia
necessaria.

## Obiettivo

Permettere a `sogark login` e agli auto-login di funzionare su Linux senza
`DISPLAY`/Wayland:

1. avviare Chrome/Chromium/Edge in modalità headless con sandbox;
2. mostrare nel terminale il QR prodotto dall'IdP oppure guidare i form visibili;
3. mantenere l'intera sessione nel browser headless;
4. intercettare la `SAMLResponse`;
5. completare il normale logon PVWA e download chiavi.

## Implementazione proposta

### 1. Separare modalità GUI e headless Linux

- Ripristinare `SOGARK_AUTH_MODE=auto|gui|headless`.
- In `auto`, usare:
  - `gui` quando esiste `DISPLAY` o `WAYLAND_DISPLAY`;
  - `headless` quando la sessione è solo terminale.
- Conservare il flusso GUI esistente senza variazioni funzionali.
- Estrarre creazione browser, intercettazione SAML e attesa in helper condivisi
  Linux, evitando la duplicazione del precedente tentativo.

### 2. Avviare Chromium in modo sicuro e compatibile Debian/RHEL

- Continuare a supportare Edge, Chrome, Chromium, Brave e
  `SOGARK_BROWSER`.
- Non legare il codice ad `apt` o `dnf`: il binario scopre un browser
  Chromium-based già installato.
- Documentare esempi di installazione:
  - Debian/Ubuntu: `apt install chromium` oppure Chrome/Edge;
  - RHEL/Fedora: `dnf install chromium` oppure Chrome/Edge RPM.
- Avviare headless con CDP su loopback e profilo temporaneo privato.
- Rimuovere l'attuale `NoSandbox(true)` dal percorso Linux:
  - sandbox attiva per default;
  - supporto sia user namespaces sia helper setuid fornito dal package del
    browser;
  - errore diagnostico con `sogark doctor` se la sandbox non può partire;
  - eventuale opt-out soltanto esplicito tramite variabile dedicata, con warning
    forte, non come default né fallback silenzioso.
- Chiudere browser e profilo temporaneo su successo, errore, timeout e segnale.

### 3. Rendere il QR Idira nel terminale senza dipendenze grafiche

- Cercare immagini/canvas quadrati visibili nella pagina corrente.
- Per il flusso verificato:
  - leggere il PNG dalla `data:` URI nativa dell'elemento `<img>`;
  - non usare screenshot CSS, perché il ridimensionamento 147→105 introduce
    antialiasing e rende inaffidabile il QR.
- Supportare anche:
  - URL/blob leggibili dal contesto browser;
  - canvas tramite `toDataURL`, se l'IdP cambia renderer.
- Determinare la dimensione del modulo dal GCD delle run pixel bianco/nero e
  validare che la griglia sia una dimensione QR standard (`21 + 4n`).
- Stampare quiet zone e moduli ad alto contrasto:
  - modalità standard con due celle per modulo;
  - modalità compatta con half-block quando la larghezza TTY è insufficiente;
  - errore esplicito se il terminale è troppo stretto, invece di produrre un QR
    deformato.
- Ridisegnare soltanto quando cambia l'hash dell'immagine, perché il QR può
  ruotare.
- Non salvare su disco PNG o payload QR in produzione.

### 4. Gestire form e azioni visibili come fallback

- Presentare un piccolo interattore semantico, non un browser testuale generico:
  - testo visibile essenziale;
  - input, checkbox, select e pulsanti realmente visibili;
  - password/OTP con `term.ReadPassword`;
  - scelta numerata quando esistono più azioni.
- Usare gli elementi Rod (`Input`, `Click`, selezione nativa) invece di
  assegnare direttamente `el.value`, così i framework frontend ricevono eventi
  reali.
- Passare lo username configurato all'autenticazione tramite una struttura
  opzioni condivisa, mantenendo l'override `--user`.
- Nella schermata Idira iniziale offrire:
  - QR come percorso predefinito;
  - username come alternativa esplicita.
- Attendere cambi DOM, navigazione o richieste di rete senza riproporre
  continuamente gli stessi prompt.

### 5. Catturare la SAMLResponse in modo resiliente

- Abilitare `Network` CDP con body POST fino a 10 MiB.
- Mantenere più livelli di cattura:
  - body delle richieste in uscita;
  - `Network.getRequestPostData`;
  - intercettazione `submit()`/`requestSubmit()` e submit event;
  - valore DOM corrente;
  - query string per redirect binding.
- Fermare il submit soltanto dopo aver acquisito una assertion non vuota.
- Non stampare, persistere o includere in errori assertion, cookie o token.
- In verbose mostrare solo lunghezza e hash abbreviato, come nel prototipo.
- Rispettare context, Ctrl+C e `saml_timeout_minutes`.

### 6. Integrare doctor, errori e configurazione

- Aggiornare `SAMLPrerequisite` affinché in modalità headless verifichi:
  - browser trovato;
  - TTY interattivo;
  - avvio sandboxed;
  - dimensioni terminale minime.
- Distinguere messaggi per:
  - browser assente;
  - sandbox non disponibile;
  - terminale non interattivo/troppo stretto;
  - QR non riconoscibile;
  - form IdP non supportato;
  - timeout MFA;
  - SAML catturata ma rifiutata da PVWA.
- Correggere il messaggio di installazione browser, oggi specifico per `apt`,
  includendo Debian e RHEL.

### 7. Test automatici e validazione reale

- Unit test Linux per:
  - risoluzione `auto|gui|headless`;
  - browser discovery e override;
  - nessun `--no-sandbox` implicito;
  - estrazione PNG da data URI;
  - rilevamento griglia 49×49 da PNG 147×147;
  - quiet zone e rendering standard/compatto;
  - terminale troppo stretto;
  - redazione di assertion/token;
  - snapshot di form visibili e azioni Rod.
- Test per la cattura SAML da POST data, DOM e redirect binding.
- Server/browser fixture locale per simulare:
  - login form dinamico;
  - QR che ruota;
  - submit SAML automatico.
- Eseguire:
  - `go test ./internal/auth ./cmd/sogark`;
  - `go test ./...`;
  - build Linux amd64/arm64 e cross-build Windows/macOS per evitare regressioni.
- Test manuale finale su:
  - Debian/WSL headless, già validato dal prototipo;
  - RHEL 9 headless con package browser aziendale;
  - Linux GUI per confermare il comportamento esistente;
  - Windows per confermare che PowerShell/WinForms sia invariato.

### 8. Documentazione

- Aggiornare README, guida utente e funzionalità testate.
- Documentare:
  - comportamento automatico GUI/headless;
  - `SOGARK_AUTH_MODE`;
  - `SOGARK_BROWSER`;
  - browser supportati e installazione Debian/RHEL;
  - requisiti sandbox;
  - dimensione minima consigliata del terminale;
  - sicurezza: QR, SAML e token restano effimeri e non vengono salvati.
- Registrare Linux headless QR + MFA + logon PVWA tra le funzionalità testate
  manualmente solo dopo l'integrazione nel binario reale.

## File/componenti previsti

- `internal/auth/saml_linux.go`
- nuovi helper Linux in `internal/auth/` per browser, TTY, QR e form
- `internal/auth/saml_common.go`
- `internal/auth/*_test.go`
- `cmd/sogark/ssh.go` o nuova struttura opzioni auth condivisa
- `cmd/sogark/doctor.go`
- `internal/messages/messages.go`
- `internal/messages/messages_linux.go`
- `README.md`
- `docs/user-guide.md`
- `docs/tested-features.md`

## Criteri di completamento

- Su Linux senza GUI, `sogark login` mostra un QR scansionabile nel terminale.
- Dopo MFA cattura la SAML e completa logon PVWA e download chiavi.
- Funziona con browser Chromium-based su Debian e RHEL senza path hard-coded.
- La sandbox resta attiva per default.
- Nessun QR, assertion, cookie o token viene persistito o stampato.
- Il fallback username/form usa eventi browser reali.
- GUI Linux, macOS e PowerShell Windows non regrediscono.
