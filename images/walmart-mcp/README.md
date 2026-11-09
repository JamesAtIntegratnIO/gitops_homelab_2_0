# walmart-mcp

Read-only Walmart catalog tools for Hermes Agent, so Hermes can turn a
grocery list into one add-to-cart link that James opens and checks out
himself. Nothing here logs in to walmart.com, holds Walmart cookies, or
changes a cart.

| Tool | What it does |
|---|---|
| `search_products` | Text search of the walmart.com catalog (Walmart I/O Affiliate API `/search`), third-party sellers left out by default |
| `lookup_items` | Price and availability for up to 20 item ids (`/items`), for store 500 / ZIP 79924 by default |
| `cart_link` | Builds `https://affil.walmart.com/cart/addToCart?items=<itemId>\|<qty>,...`; no request to Walmart |

All three declare `readOnlyHint`.

## Build

`.github/workflows/build-walmart-mcp-image.yaml` (amd64 + arm64), on a push
to `main` under `images/walmart-mcp/`. Image
`ghcr.io/jamesatintegratnio/gitops_homelab_2_0/walmart-mcp:v0.1.0`.

## Credentials

1Password item `walmart-io` (vault `homelab`), created by James:

| Field | From |
|---|---|
| `consumer-id` | walmart.io dashboard, after uploading the public key |
| `key-version` | walmart.io dashboard (usually `1`) |
| `private-key` | the PEM private key from Walmart's key tutorial (`openssl genrsa` + `openssl pkcs8 -topk8 -nocrypt`) |

Only the public key goes to walmart.io. The private key is read from env at
start and removed from the process environment.

## Run locally

```sh
pip install --no-deps -r requirements.txt
WALMART_CONSUMER_ID=... WALMART_KEY_VERSION=1 WALMART_PRIVATE_KEY="$(cat key.pem)" \
  WALMART_STORE_ID=500 WALMART_ZIP=79924 python server.py   # :8000/mcp
```

## Checked so far

Against mcp 2.3.0, locally: the three tools list with `readOnlyHint`,
`cart_link` builds the link, and `search_products` reaches Walmart with signed
headers (a made-up consumer id gets `401 Signature verification failed`, as it
should). Not yet checked with real credentials: the response field names
(`itemId`, `salePrice`, `sellerInfo`, ...) and that `itemId` is the id the
cart link accepts.
