# /// script
# requires-python = ">=3.10"
# dependencies = ["msal>=1.28.0"]
# ///
"""WAM (Web Account Manager) authentication helper for mogcli.

Invoked as a subprocess by the mog CLI on Windows.
Reads a JSON request from stdin, performs WAM-brokered authentication
via MSAL Python, and writes a JSON response to stdout.

Protocol:
  Request:  {"action": "login"|"acquire_silent", "client_id": "...", "authority": "...", "scopes": [...], ...}
  Response: {"access_token": "...", ...} or {"error": "...", "error_description": "..."}
"""

import json
import sys

import msal


def error_response(code: str, description: str) -> dict:
    return {"error": code, "error_description": description}


def login(app: msal.PublicClientApplication, scopes: list[str]) -> dict:
    result = app.acquire_token_interactive(
        scopes=scopes,
        parent_window_handle=app.CONSOLE_WINDOW_HANDLE,
    )
    if "error" in result:
        return error_response(result["error"], result.get("error_description", ""))

    return token_response(result)


def acquire_silent(
    app: msal.PublicClientApplication,
    scopes: list[str],
    account_id: str,
    username: str,
) -> dict:
    accounts = app.get_accounts()
    account = find_account(accounts, account_id, username)
    if account is None:
        return error_response(
            "no_account",
            "No cached account found. Run `mog auth login` to sign in again.",
        )

    result = app.acquire_token_silent(scopes, account=account)
    if result is None:
        return error_response(
            "silent_failed",
            "Silent token acquisition returned no result. Run `mog auth login` to sign in again.",
        )
    if "error" in result:
        return error_response(result["error"], result.get("error_description", ""))

    return token_response(result)


def find_account(
    accounts: list[dict], account_id: str, username: str
) -> dict | None:
    if account_id:
        for a in accounts:
            if a.get("local_account_id", "").lower() == account_id.lower():
                return a
    if username:
        for a in accounts:
            if a.get("username", "").lower() == username.lower():
                return a
    if accounts:
        return accounts[0]
    return None


def token_response(result: dict) -> dict:
    scope = result.get("scope", [])
    if isinstance(scope, list):
        scope = " ".join(scope)

    resp = {
        "access_token": result.get("access_token", ""),
        "token_type": result.get("token_type", "Bearer"),
        "scope": scope,
        "expires_in": result.get("expires_in", 3600),
        "id_token": result.get("id_token", ""),
    }

    claims = result.get("id_token_claims")
    if claims and isinstance(claims, dict):
        resp["id_token_claims"] = {
            "oid": claims.get("oid", ""),
            "sub": claims.get("sub", ""),
            "preferred_username": claims.get("preferred_username", ""),
            "email": claims.get("email", ""),
            "upn": claims.get("upn", ""),
            "tid": claims.get("tid", ""),
        }

    return resp


def main() -> None:
    try:
        request = json.load(sys.stdin)
    except (json.JSONDecodeError, ValueError) as e:
        json.dump(error_response("invalid_request", str(e)), sys.stdout)
        sys.exit(1)

    action = request.get("action", "")
    client_id = request.get("client_id", "")
    authority_value = request.get("authority", "organizations")
    scopes = request.get("scopes", [])

    if not client_id:
        json.dump(error_response("missing_client_id", "client_id is required"), sys.stdout)
        sys.exit(1)

    authority = f"https://login.microsoftonline.com/{authority_value}"

    app = msal.PublicClientApplication(
        client_id,
        authority=authority,
        enable_broker_on_windows=True,
    )

    if action == "login":
        result = login(app, scopes)
    elif action == "acquire_silent":
        account_id = request.get("account_id", "")
        username = request.get("username", "")
        result = acquire_silent(app, scopes, account_id, username)
    else:
        result = error_response("unknown_action", f"Unknown action: {action}")

    json.dump(result, sys.stdout)
    if "error" in result:
        sys.exit(1)


if __name__ == "__main__":
    main()
