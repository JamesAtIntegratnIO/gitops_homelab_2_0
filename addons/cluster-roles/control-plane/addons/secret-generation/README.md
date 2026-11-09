# secret-generation

Lets a PR create a NEW, random secret in 1Password without anyone pasting a
value, and without the value passing through Hermes or git.

## How it works

1. A PR adds a `Password` generator and a `PushSecret` in the consumer's
   namespace (template in the `secret-requests` skill).
2. ESO generates the value and writes a new item to the 1Password vault
   `generated` through the ClusterSecretStore `onepassword-generated`.
3. The consumers' ExternalSecrets read the item back through the same store.

## Limits (what stops it doing more)

- **One vault.** The Connect token behind the store reaches only `generated`.
  Items in `homelab` (every other credential in the cluster) are out of its
  reach, for reading and writing.
- **Create only.** Kyverno `restrict-pushsecret` rejects any PushSecret that
  is not `updatePolicy: IfNotExists`, `deletionPolicy: None` and aimed at
  `onepassword-generated`.
- **Ask first.** Hermes may author one only after James says yes to a message
  describing the item, its fields, why it is needed and what it will contain
  (skill `secret-requests`). Merging is still James's.

## Setup (once, by hand)

1. In 1Password create a vault named `generated`.
2. On the Connect server (connect.integratn.tech), create a token for that
   vault with read and write access.
3. Save it in the `homelab` vault as item `eso-generated-token`, field
   `token`.

The existing read store keeps its own token and vault.

## Consuming a generated item

An ExternalSecret names `secretStoreRef: onepassword-generated` instead of
`onepassword-store`, and the item key and field label the PushSecret wrote.
Rotation and deletion are done in 1Password by hand; the PushSecret never
overwrites or removes an item.

## If it fails

- `kubectl get clustersecretstore onepassword-generated`: not Ready means the
  token Secret is missing (item or ExternalSecret `eso-onepassword-generated-token`)
  or the token cannot reach the vault.
- A PushSecret rejected on apply names the Kyverno policy: fix the manifest,
  do not loosen the policy.
- A PushSecret Ready but the item missing: check the ESO controller logs for
  the Connect provider's error.

## Not verified when this was written

Nothing here has run. Open points: whether the Connect provider of ESO 2.12
creates a multi-field item from one PushSecret and honours `IfNotExists` per
field, whether the Password generator's value stays stable across refreshes
under `IfNotExists`, and whether the `platform-services` ArgoCD project allows
the cluster-scoped ClusterSecretStore. The first real request is the test.
