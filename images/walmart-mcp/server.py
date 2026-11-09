"""walmart-mcp: read-only Walmart catalog tools for Hermes Agent.

Three tools, none of which touches a Walmart account:
  search_products  text search of the walmart.com catalog (Affiliate API)
  lookup_items     price and availability for up to 20 item ids
  cart_link        builds the affiliate add-to-cart URL; James opens it himself

Walmart I/O Affiliate API: every request is signed with the RSA private key
whose public half James uploaded at walmart.io. String to sign is the values
of WM_CONSUMER.ID, WM_CONSUMER.INTIMESTAMP and WM_SEC.KEY_VERSION, sorted by
header name, each followed by "\n"; SHA256withRSA (PKCS#1 v1.5), base64. A
signature lives 180 s, so one is made per request.

Environment:
  WALMART_CONSUMER_ID   consumer id (UUID) from the walmart.io dashboard
  WALMART_KEY_VERSION   private key version from the dashboard (e.g. "1")
  WALMART_PRIVATE_KEY   the private key itself (not a path): PEM (PKCS#8 or
                        PKCS#1) or OpenSSH format, line breaks optional
  WALMART_STORE_ID      default store for availability (James: 500)
  WALMART_ZIP           default ZIP for availability (James: 79924)
  WALMART_PUBLISHER_ID  optional Impact publisher id (not needed)
  WALMART_API_BASE      Affiliate API base URL. Default is production
                        (developer.api.walmart.com). Stage consumer ids only
                        work against https://developer.api.stg.walmart.com/
                        api-proxy/service/affil/product/v2.
  MCP_ALLOWED_HOSTS     comma-separated Host headers to accept
"""

from __future__ import annotations

import base64
import os
import re
import time
from typing import Any

import httpx
from cryptography.hazmat.primitives import hashes, serialization
from cryptography.hazmat.primitives.asymmetric import padding
from mcp.server.mcpserver import MCPServer
from mcp.server.mcpserver.exceptions import ToolError
from mcp.server.transport_security import TransportSecuritySettings
from mcp_types import ToolAnnotations

API = os.environ.get(
    "WALMART_API_BASE", "https://developer.api.walmart.com/api-proxy/service/affil/product/v2"
).rstrip("/")
CART_BASE = "https://affil.walmart.com/cart/addToCart?items="
# Walmart and browsers handle URLs well past this; split beyond it.
CART_URL_LIMIT = 3800

CONSUMER_ID = os.environ.get("WALMART_CONSUMER_ID", "").strip()
KEY_VERSION = os.environ.get("WALMART_KEY_VERSION", "1").strip()
STORE_ID = os.environ.get("WALMART_STORE_ID", "").strip()
ZIP = os.environ.get("WALMART_ZIP", "").strip()
PUBLISHER_ID = os.environ.get("WALMART_PUBLISHER_ID", "").strip()


def _normalize_pem(raw: str) -> str:
    """Rebuild a PEM whatever a paste did to its line breaks.

    A 1Password text field is single-line: pasting a key there turns its
    newlines into spaces (or a literal "\\n", or CRLF), which cryptography
    rejects as MalformedFraming. Take the BEGIN/END labels and the base64
    body, drop all whitespace from the body, and re-wrap it at 64 columns.
    """
    text = raw.replace("\\n", "\n").replace("\r", "").strip()
    m = re.search(r"-----BEGIN ([A-Z ]+)-----(.*?)-----END \1-----", text, re.S)
    if not m:
        return text
    label, body = m.group(1), re.sub(r"\s+", "", m.group(2))
    lines = [body[i : i + 64] for i in range(0, len(body), 64)]
    return f"-----BEGIN {label}-----\n" + "\n".join(lines) + f"\n-----END {label}-----\n"


_pem = _normalize_pem(os.environ.pop("WALMART_PRIVATE_KEY", ""))
PRIVATE_KEY = None
if _pem:
    if "OPENSSH PRIVATE KEY" in _pem:
        PRIVATE_KEY = serialization.load_ssh_private_key(_pem.encode(), password=None)
    else:
        PRIVATE_KEY = serialization.load_pem_private_key(_pem.encode(), password=None)
del _pem

READ_ONLY = ToolAnnotations(readOnlyHint=True, destructiveHint=False, idempotentHint=True, openWorldHint=True)


def _headers() -> dict[str, str]:
    if PRIVATE_KEY is None or not CONSUMER_ID:
        raise ToolError(
            "walmart-mcp has no credentials: set WALMART_CONSUMER_ID, WALMART_KEY_VERSION "
            "and WALMART_PRIVATE_KEY (1Password item walmart-io)."
        )
    ts = str(int(time.time() * 1000))
    signed = {"WM_CONSUMER.ID": CONSUMER_ID, "WM_CONSUMER.INTIMESTAMP": ts, "WM_SEC.KEY_VERSION": KEY_VERSION}
    to_sign = "".join(signed[k] + "\n" for k in sorted(signed))
    sig = PRIVATE_KEY.sign(to_sign.encode(), padding.PKCS1v15(), hashes.SHA256())  # type: ignore[union-attr]
    return {**signed, "WM_SEC.AUTH_SIGNATURE": base64.b64encode(sig).decode(), "Accept": "application/json"}


async def _get(path: str, params: dict[str, Any]) -> dict[str, Any]:
    if PUBLISHER_ID:
        params = {**params, "publisherId": PUBLISHER_ID}
    # ToolError, not a bare exception: mcp 2 shows the model only a generic
    # "Error executing tool" for anything else.
    try:
        async with httpx.AsyncClient(timeout=30) as client:
            r = await client.get(f"{API}/{path}", params=params, headers=_headers())
    except httpx.HTTPError as exc:
        raise ToolError(f"cannot reach the Walmart API: {exc!r}") from exc
    if r.status_code >= 400:
        raise ToolError(f"Walmart API {r.status_code} on /{path}: {r.text[:500]}")
    try:
        return r.json()
    except ValueError as exc:
        raise ToolError(f"Walmart API returned non-JSON on /{path}: {r.text[:300]}") from exc


def _item(i: dict[str, Any]) -> dict[str, Any]:
    """The fields that matter for picking groceries, nothing else."""
    return {
        # The affiliate API's itemId is the id in walmart.com/ip/<id>, which is
        # what the add-to-cart link takes.
        "itemId": i.get("itemId"),
        "name": i.get("name"),
        "brand": i.get("brandName"),
        "size": i.get("size"),
        "price": i.get("salePrice"),
        "stock": i.get("stock"),
        "availableOnline": i.get("availableOnline"),
        "offerType": i.get("offerType"),
        "seller": i.get("sellerInfo"),
        "marketplace": i.get("marketplace"),
        "rating": i.get("customerRating"),
        "reviews": i.get("numReviews"),
        "upc": i.get("upc"),
        "url": f"https://www.walmart.com/ip/{i.get('itemId')}",
    }


def _is_third_party(i: dict[str, Any]) -> bool:
    if i.get("marketplace") is True:
        return True
    seller = (i.get("sellerInfo") or "").strip().lower()
    return bool(seller) and seller not in ("walmart.com", "walmart")


def _allowed_hosts() -> list[str]:
    raw = os.environ.get("MCP_ALLOWED_HOSTS", "")
    return [h.strip() for h in raw.split(",") if h.strip()]


mcp = MCPServer(
    "walmart",
    instructions=(
        "Read-only Walmart catalog tools. Nothing here can change a Walmart cart or account. "
        "To fill a cart: search_products per grocery line, confirm picks, then cart_link with "
        "itemId and quantity. The user opens the link while signed in and checks out himself."
    ),
)


@mcp.tool(annotations=READ_ONLY)
async def search_products(query: str, count: int = 10, include_third_party: bool = False) -> dict[str, Any]:
    """Search the walmart.com catalog. Returns up to `count` (max 25) items with
    itemId, name, brand, size, price and stock. Third-party (marketplace) sellers
    are left out unless include_third_party is true."""
    data = await _get("search", {"query": query, "numItems": max(1, min(count, 25))})
    items = data.get("items") or []
    dropped = 0
    out = []
    for i in items:
        if not include_third_party and _is_third_party(i):
            dropped += 1
            continue
        out.append(_item(i))
    return {"query": query, "items": out, "thirdPartyDropped": dropped, "totalResults": data.get("totalResults")}


@mcp.tool(annotations=READ_ONLY)
async def lookup_items(item_ids: list[str], zip_code: str = "", store_id: str = "") -> dict[str, Any]:
    """Current price and availability for up to 20 item ids. Availability is for
    the default store/ZIP unless zip_code or store_id is given."""
    ids = [str(i).strip() for i in item_ids if str(i).strip()][:20]
    if not ids:
        return {"items": []}
    zip_code = zip_code or ZIP
    store_id = store_id or STORE_ID
    if len(ids) == 1:
        params: dict[str, Any] = {"storeId": store_id} if store_id else {}
        data = await _get(f"items/{ids[0]}", params)
        items = data.get("items") or [data]
    else:
        params = {"ids": ",".join(ids)}
        if zip_code:
            params["zipCode"] = zip_code
        data = await _get("items", params)
        items = data.get("items") or []
    return {"storeId": store_id, "zipCode": zip_code, "items": [_item(i) for i in items]}


@mcp.tool(annotations=READ_ONLY)
def cart_link(items: list[dict[str, Any]]) -> dict[str, Any]:
    """Build the add-to-cart link(s) for [{"itemId": "...", "quantity": 1}, ...].
    Opening a link ADDS these items to the cart of whoever is signed in in that
    browser (it never removes anything). Makes no request to Walmart."""
    entries: list[str] = []
    for it in items:
        item_id = str(it.get("itemId", "")).strip()
        qty = int(it.get("quantity", 1) or 1)
        if not item_id.isdigit() or qty < 1:
            raise ToolError(f"bad cart entry: {it!r} (itemId must be digits, quantity >= 1)")
        entries.append(f"{item_id}|{qty}")
    links: list[str] = []
    current: list[str] = []
    for e in entries:
        candidate = CART_BASE + ",".join(current + [e])
        if current and len(candidate) > CART_URL_LIMIT:
            links.append(CART_BASE + ",".join(current))
            current = [e]
        else:
            current.append(e)
    if current:
        links.append(CART_BASE + ",".join(current))
    return {
        "links": [{"label": f"Cart {n} of {len(links)}", "url": u} for n, u in enumerate(links, 1)],
        "itemCount": len(entries),
        "note": "Adds to the existing cart; items already in it stay.",
    }


def main() -> None:
    hosts = _allowed_hosts()
    security = (
        TransportSecuritySettings(enable_dns_rebinding_protection=True, allowed_hosts=hosts)
        if hosts
        else None
    )
    mcp.run(
        "streamable-http",
        host="0.0.0.0",
        port=int(os.environ.get("PORT", "8000")),
        stateless_http=True,
        transport_security=security,
    )


if __name__ == "__main__":
    main()
