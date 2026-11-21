#!/usr/bin/env python3
"""One-time local Trakt PKCE login, saving JSON for GitHub's TRAKT_TOKEN secret.

Usage (Mac Terminal, from repository root):
    python3 scripts/trakt_pkce_login.py YOUR_CLIENT_ID

Keep trakt-token.json private. Never commit it or share it in chat.
"""

import base64
import hashlib
from http.server import BaseHTTPRequestHandler, HTTPServer
import json
from pathlib import Path
import secrets
import sys
import time
from urllib.error import HTTPError, URLError
from urllib.parse import parse_qs, urlencode, urlsplit
from urllib.request import Request, urlopen
import webbrowser

REDIRECT = "http://localhost:5173/callback"
TOKEN_URL = "https://api.trakt.tv/oauth/token"
AUTHORIZE_URL = "https://auth.trakt.tv/oauth/authorize"


def main():
    if len(sys.argv) != 2 or not sys.argv[1].strip():
        sys.exit("Uso: python3 scripts/trakt_pkce_login.py CLIENT_ID")
    client_id = sys.argv[1].strip()
    verifier = secrets.token_urlsafe(64)
    challenge = base64.urlsafe_b64encode(hashlib.sha256(verifier.encode("ascii")).digest()).rstrip(b"=").decode("ascii")
    state = secrets.token_urlsafe(32)
    authorize_url = AUTHORIZE_URL + "?" + urlencode({
        "response_type": "code", "client_id": client_id,
        "redirect_uri": REDIRECT, "state": state,
        "code_challenge": challenge, "code_challenge_method": "S256",
    })
    callback = {}

    class CallbackHandler(BaseHTTPRequestHandler):
        def do_GET(self):
            parsed = urlsplit(self.path)
            if parsed.path != "/callback":
                self.send_error(404)
                return
            args = parse_qs(parsed.query)
            if args.get("state", [""])[0] != state:
                self.send_error(400, "Stato OAuth non valido")
                return
            callback["code"] = args.get("code", [""])[0]
            callback["error"] = args.get("error", [""])[0]
            data = b"Autorizzazione ricevuta. Puoi tornare al Terminale."
            self.send_response(200)
            self.send_header("Content-Type", "text/plain; charset=utf-8")
            self.send_header("Content-Length", str(len(data)))
            self.end_headers()
            self.wfile.write(data)

        def log_message(self, *_):
            # Never log callback URLs: they contain an authorization code.
            return

    try:
        server = HTTPServer(("127.0.0.1", 5173), CallbackHandler)
    except OSError as exc:
        sys.exit(f"Non riesco ad avviare il server sulla porta 5173: {exc}")
    server.timeout = 1
    print("Apro Trakt nel browser per autorizzare l'app...")
    if not webbrowser.open(authorize_url):
        print("Apri manualmente questo URL nel browser:")
        print(authorize_url)
    deadline = time.monotonic() + 180
    try:
        while not callback and time.monotonic() < deadline:
            server.handle_request()
    finally:
        server.server_close()
    if not callback:
        sys.exit("Tempo scaduto: ripeti l'autorizzazione.")
    if callback.get("error"):
        sys.exit("Trakt non ha autorizzato l'applicazione: " + callback["error"])
    if not callback.get("code"):
        sys.exit("Nessun codice ricevuto da Trakt.")

    payload = json.dumps({
        "code": callback["code"], "client_id": client_id,
        "redirect_uri": REDIRECT, "code_verifier": verifier,
        "grant_type": "authorization_code",
    }).encode("utf-8")
    request = Request(TOKEN_URL, data=payload, headers={"Content-Type": "application/json"})
    try:
        with urlopen(request, timeout=20) as response:
            result = json.load(response)
    except HTTPError as exc:
        # Do not print raw tokens or auth callback data.
        sys.exit(f"Trakt ha rifiutato lo scambio del codice (HTTP {exc.code}). Controlla il Redirect URI dell'app.")
    except URLError as exc:
        sys.exit(f"Connessione a Trakt non riuscita: {exc.reason}")
    if not result.get("access_token") or not result.get("refresh_token") or not result.get("expires_in"):
        sys.exit("La risposta Trakt non contiene tutti i token richiesti.")
    created_at = int(result.get("created_at") or time.time())
    record = {
        "access_token": result["access_token"],
        "refresh_token": result["refresh_token"],
        "expires_at": time.strftime("%Y-%m-%dT%H:%M:%SZ", time.gmtime(created_at + int(result["expires_in"]))),
    }
    destination = Path("trakt-token.json")
    fd = destination.open("x") if not destination.exists() else destination.open("w")
    try:
        import os
        os.fchmod(fd.fileno(), 0o600)
        json.dump(record, fd, separators=(",", ":"))
        fd.write("\n")
    finally:
        fd.close()
    print("Autorizzazione completata. Creato trakt-token.json (privato).")
    print("Copia il suo contenuto nel Secret GitHub TRAKT_TOKEN, senza pubblicarlo.")


if __name__ == "__main__":
    main()
