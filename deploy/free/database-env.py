"""Write private SQL-tool environment values derived from DATABASE_URL."""
import os
import sys
from urllib.parse import parse_qs, unquote, urlparse

if len(sys.argv) != 2:
    raise SystemExit("Usage: python3 deploy/free/database-env.py /path/to/private.env")
url = urlparse(os.environ.get("DATABASE_URL", ""))
if url.scheme not in ("postgres", "postgresql") or not all((url.hostname, url.username, url.password, url.path.strip("/"))):
    raise SystemExit("Load a valid DATABASE_URL into the environment first")
values = {
    "SQL_HOST": url.hostname,
    "SQL_PORT": str(url.port or 5432),
    "SQL_USER": unquote(url.username),
    "SQL_PASSWORD": unquote(url.password),
    "SQL_DATABASE": unquote(url.path.strip("/")),
    "SQL_TLS": "false" if parse_qs(url.query).get("sslmode") == ["disable"] else "true",
}
if os.environ.get("SQL_TLS_CA_BASE64"):
    values["SQL_TLS_CA_BASE64"] = os.environ["SQL_TLS_CA_BASE64"]
if any("\n" in value or "\r" in value for value in values.values()):
    raise SystemExit("Docker env files do not support multiline connection values")
fd = os.open(sys.argv[1], os.O_WRONLY | os.O_CREAT | os.O_TRUNC, 0o600)
os.fchmod(fd, 0o600)
with os.fdopen(fd, "w") as file:
    file.write("".join(f"{key}={value}\n" for key, value in values.items()))
print("Saved private Temporal SQL environment file")
