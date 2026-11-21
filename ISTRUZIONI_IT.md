# IMDb → Trakt, senza Client Secret — istruzioni per Mac

Questa è una **modifica sperimentale** del progetto originale. Il flusso PKCE è basato sulla documentazione Trakt (novembre 2026), ma **non è stato testato con credenziali reali**. Non attivare `add-only` prima di avere un `dry-run` riuscito e verificato.

## 1. Sostituisci i file del tuo Fork

Puoi usare **GitHub Desktop** (https://desktop.github.com): accedi, seleziona `File → Clone repository`, clona `HeathenKemistry/imdb-trakt-sync`. Dal Finder copia tutti i file e le cartelle del progetto modificato nella cartella del clone, sostituendo quelli esistenti. Per vedere `.github` premi `Cmd+Shift+.` nel Finder. In GitHub Desktop controlla le differenze e usa `Commit to main` → `Push origin`. **Non copiare `trakt-token.json` nel repository.**

## 2. Autorizza Trakt sul Mac, una sola volta

Nella configurazione della tua app Trakt (Development), assicurati che il Redirect URI sia esattamente:

`http://localhost:5173/callback`

Apri **Terminale**, vai nella cartella locale del progetto modificato (es. trascina la cartella nel Terminale dopo `cd `), e controlla `python3 --version`. Se Python 3 manca, installalo da https://www.python.org/downloads/macos/.

Esegui:

`python3 scripts/trakt_pkce_login.py IL_TUO_CLIENT_ID`

Usa il **Client ID** della tua app personale, non quello negli esempi. Si apre il browser, autorizza la tua applicazione su Trakt. Lo script salva localmente il file `trakt-token.json`: è **segreto**. NON inviarlo a nessuno né caricarlo nel repository.

Per copiarlo negli appunti su Mac:

`pbcopy < trakt-token.json`

## 3. Inserisci Secrets e Variables nel Fork

Apri il tuo repository GitHub → `Settings → Secrets and variables → Actions`.

**Secrets** (`New repository secret`):
- `TRAKT_CLIENTID`: Client ID della tua app Trakt.
- `TRAKT_TOKEN`: incolla il contenuto JSON copiato con `pbcopy`.
- `IMDB_EMAIL` e `IMDB_PASSWORD`: credenziali IMDb (se usi IMDB_AUTH=credentials). In caso di MFA/CAPTCHA potresti dover usare `IMDB_AUTH=cookies` e il secret `IMDB_COOKIEATMAIN` invece.
- `GH_PAT`: token GitHub *fine-grained*, con accesso solo al tuo fork e permesso repository `Secrets: Read and write`, per aggiornare il token Trakt ruotato. Crealo da https://github.com/settings/personal-access-tokens/new. Conserva il PAT come secret e non nel codice.

**Variables** (`New repository variable`):
- `IMDB_AUTH` = `credentials`
- `SYNC_MODE` = `dry-run`
- `SYNC_HISTORY` = `true`
- `SYNC_RATINGS` = `true`
- `SYNC_WATCHLIST` = `true`
- `SYNC_LISTS` = `false`
- `AUTO_SYNC` = `false` (impedisce le esecuzioni programmate fino ai test)

Nessun `TRAKT_CLIENTSECRET`. Il file `config.yaml` già contiene impostazioni sicure per la modalità di prova.

## 4. Esegui una prova

Vai in `Actions → sync → Run workflow`. Controlla l'esito e i log, ma non condividere segreti. La modalità `dry-run` non deve modificare i dati su Trakt. Se qualcosa non funziona, NON usare `add-only`: occorre correggere il codice o le credenziali.

Se la prova funziona e i risultati sono sensati, modifica `SYNC_MODE` in `add-only`. **Non usare `full`**: può cancellare elementi presenti soltanto su Trakt. Poi imposta `AUTO_SYNC=true` per eseguire ogni 12 ore.

## Limitazioni

- Non si può confermare la compatibilità reale OAuth, IMDb e Trakt senza eseguire un'autorizzazione sul tuo account.
- Il progetto originale è stato archiviato nell'agosto 2026 e il web scraping IMDb può smettere di funzionare.
- Trakt usa refresh token monouso; GitHub Actions deve aggiornarli dopo ogni rinnovo e il secret `GH_PAT` è indispensabile.
- Se il workflow fallisce dopo un rinnovo ma prima dell'aggiornamento del secret, può essere necessaria una nuova autorizzazione dal Mac.

Documentazione: https://developer.trakt.tv/docs/pkce e https://developer.trakt.tv/docs/authentication-oauth

**Nota sulla cronologia:** il progetto originale considera «visto» un titolo che hai valutato su IMDb. Non può ricostruire le vere date di visione: se alcuni film sono marcati come visti su IMDb senza voto, potrebbero non essere trasferiti.
