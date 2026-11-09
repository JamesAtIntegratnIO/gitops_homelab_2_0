# secret-generation

Lets a PR create a NEW, random secret in the `homelab` 1Password vault
without anyone pasting a value, and without the value passing through git or
Hermes.

## How it works

1. A PR adds a `Password` generator and a `PushSecret` in the consumer's
   namespace (template in the `secret-requests` skill).
2. ESO generates the value and writes a new item to the `homelab` vault
   through the ClusterSecretStore `onepassword-generated`, which has its own
   write token.
3. The consumers' ExternalSecrets read the item back through the normal
   `onepassword-store`, exactly like any other credential.

## Limits (what stops it doing more)

The write token reaches the whole `homelab` vault, so the limits are policy,
not scope:

- **One store.** Kyverno `restrict-pushsecret` rejects any PushSecret not aimed
  only at `onepassword-generated`.
- **Create only.** The same policy requires `updatePolicy: IfNotExists` and
  `deletionPolicy: None`: a PushSecret never overwrites a field that exists and
  never deletes an item.
- **Own names.** The policy requires every `remoteKey` to start `generated-`,
  so a PushSecret cannot be aimed at an existing credential such as
  `hermes-agent`, whatever the provider does with `IfNotExists`. (A
  convention of this PR, not a requirement of ESO; drop the pattern in the
  policy to remove it.)
- **Ask first.** Hermes may author one only after James says yes to a message
  describing the item, its fields, why it is needed and what it will contain
  (skill `secret-requests`). Merging is still James's.

## Setup (once, by hand)

1. On the Connect server (connect.integratn.tech), create a token with read
   and write access to the `homelab` vault.
2. Save it in the `homelab` vault as item `eso-generated-token`, field
   `token`.

The existing read store keeps its own token and nothing about it changes.

## Consuming a generated item

An ordinary ExternalSecret on `onepassword-store`, naming the item
(`generated-...`) and the field label the PushSecret wrote. Rotation and
deletion are done in 1Password by hand; the PushSecret never overwrites or
removes an item.

## If it fails

- `kubectl get clustersecretstore onepassword-generated`: not Ready means the
  token Secret is missing (item or ExternalSecret
  `eso-onepassword-generated-token`) or the token cannot write to `homelab`.
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
