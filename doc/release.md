<!-- SPDX-FileCopyrightText: 2026 GSI Helmholtz Centre for Heavy Ion Research GmbH <http://www.gsi.de> -->
<!-- SPDX-License-Identifier: Apache-2.0 -->

# Releases

A release is a signed tag ([decision 4](decisions.md#4-a-release-is-a-signed-tag)). Only the
maintainers create tags.

## Cutting a release

1. Pick the commit on `main`, with CI green on it, and the version: a
   breaking change in v0 raises the minor version, and so does a higher
   `go` line.
2. Write the message to a file. Its first line is the title; the rest is
   Markdown and becomes the release notes:

   ```markdown
   go-nodeset v0.2.0

   ## Changes

   - ...
   ```

3. Tag and push. git's default cleanup deletes every line that starts with
   `#`, Markdown headings included, so keep whitespace cleanup only:

   ```console
   $ git tag -s v0.2.0 --cleanup=whitespace -F notes.md <commit>
   $ git push origin v0.2.0
   ```

The Release workflow verifies the tag, tests the tagged commit on both Go
lines, publishes the GitHub release and fetches the version through the
module proxy, after which pkg.go.dev lists it.

## Withdrawing a release

Never move or delete a pushed tag: the module proxy and the checksum
database keep it regardless. Add a `retract` directive with the reason to
`go.mod` and ship it in the next release:

```go
retract v0.2.0 // Tagged from the wrong commit.
```

## Setting up verification

The workflow checks the signature against two repository variables, and
publishes nothing while both are empty. They are variables rather than files,
because a file in the tagged commit could name its own signers. Set them
under *Settings → Secrets and variables → Actions → Variables*. A variable is
no secret, since a workflow can print it and GitHub does not mask it in the
log, so these hold public keys only.

`RELEASE_ALLOWED_SIGNERS` lists the SSH keys, one line per signer in the
format of git's `gpg.ssh.allowedSignersFile`:

```
name@example.org namespaces="git" ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAA...
```

Sign with the matching key:

```console
$ git config gpg.format ssh
$ git config user.signingKey ~/.ssh/id_ed25519.pub
```

`RELEASE_ALLOWED_PGP_KEYS` holds the OpenPGP public keys, ASCII armored, one
block after the other, as `gpg --armor --export <fingerprint>` prints them.
Any key in it may sign, however gpg would otherwise trust it. The workflow
knows only what the variable holds: a key that has expired no longer
verifies, but a revoked key verifies until the variable holds its
revocation, so export the key again after revoking it, or remove it. Sign
with the key:

```console
$ git config gpg.format openpgp
$ git config user.signingKey <fingerprint>
```

The publishing job runs in the `release` environment. Restrict its
deployment branches and tags to `v*`, and protect `v*` tags with a ruleset
that lets only the maintainers create them and nobody update or delete
them.
