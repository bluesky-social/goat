`goat`: Go AT protocol CLI tool
===============================

This is a general purpose [atproto](https://atproto.com) CLI tool, sort of like `curl`. You can fetch `at://` URIs, monitor the full-network firehose, migrate accounts, administer PDS instances, and more.

## Install

If you use [homebrew](https://formulae.brew.sh/formula/goat), you can install directly:

```bash
brew install goat
```

Pre-built binaries and Debian/Ubuntu packages are available under [Releases](https://github.com/bluesky-social/goat/releases/tag/v0.2.0).

If you have the Go toolchain installed and configured correctly, you can directly build and install the tool for your local account:

```bash
go install github.com/bluesky-social/goat@latest
```

## Usage

`goat` is relatively self-documenting via help pages:

```bash
goat --help
goat bsky -h
goat help bsky
# etc
```

Most commands use public APIs are don't require authentication. Some commands, like creating records, require an atproto account. You can log in using an "app password" with `goat account login -u <handle> -p <app-password>`.

WARNING: `goat` will store both the app password and authentication tokens in the current users home directory, in cleartext. `goat logout` will wipe the file. Intention is to eventually support configuration via environment variables to keep sensitive state in a password manager or otherwise not-cleartext-on-disk.

Some commands output JSON, and you can use tools like `jq` to process them.

## Examples

Resolve an account's identity in the network:

```bash
$ goat resolve wyden.senate.gov
{
  "id": "did:plc:ydtsvzzsl6nlfkmnuooeqcmc",
  "alsoKnownAs": [
    "at://wyden.senate.gov"
  ],
  "verificationMethod": [
    {
      "id": "did:plc:ydtsvzzsl6nlfkmnuooeqcmc#atproto",
      "type": "Multikey",
      "controller": "did:plc:ydtsvzzsl6nlfkmnuooeqcmc",
      "publicKeyMultibase": "zQ3shuMW7q4KBdsFcdvebGi2EVv8KcqS24tF9Pg7Wh5NLB2NM"
    }
  ],
  "service": [
    {
      "id": "#atproto_pds",
      "type": "AtprotoPersonalDataServer",
      "serviceEndpoint": "https://shimeji.us-east.host.bsky.network"
    }
  ]
}
```

List record collection types for an account:

```bash
$ goat ls -c dril.bsky.social
app.bsky.actor.profile
app.bsky.feed.post
app.bsky.feed.repost
app.bsky.graph.follow
chat.bsky.actor.declaration
```

Fetch a record from the network as JSON:

```bash
$ goat get at://dril.bsky.social/app.bsky.feed.post/3kkreaz3amd27
{
  "$type": "app.bsky.feed.post",
  "createdAt": "2024-02-06T18:15:19.802Z",
  "langs": [
    "en"
  ],
  "text": "I do not Fucking recall them asking the blue sky elders permission to open registration to commoners ."
}
```

Make a public snapshot of your account:

```bash
$ goat repo export jay.bsky.team
downloading from https://morel.us-east.host.bsky.network to: jay.bsky.team.20240811183155.car

$ downloading blobs to: jay.bsky.team_blobs
jay.bsky.team_blobs/bafkreia2x4faux5y7v7v54yl5ebkbaek7z7nhmsd4cooubz3yj4zox34cq	downloaded
jay.bsky.team_blobs/bafkreia3qgbww7odprmysd6jcyxoh5sczkwoxinnmzpsp73gs623fqfm3a	downloaded
jay.bsky.team_blobs/bafkreia3rgnywdrysy65vid42ulyno2cybxhxrn3ragm7cw3smmsxzvbs4	downloaded
[...]
```

Show PLC history for a single account, or make a snapshot of all PLC records (this takes a while), or monitor new ops:

```bash
$ goat plc history atproto.com
[...]

$ goat plc dump | pv -l | gzip > plc_snapshot.json.gz
[...]

$ goat plc dump --cursor now --tail
[...]
```

Verify syntax and generate TIDs:

```bash
$ goat syntax handle check xn--fiqa61au8b7zsevnm8ak20mc4a87e.xn--fiqs8s
valid

$ goat syntax rkey check dHJ1ZQ==
error: recordkey syntax didn't validate via regex

$ goat syntax tid inspect 3kzifvcppte22
Timestamp (UTC): 2024-08-12T02:08:03.29Z
Timestamp (Local): 2024-08-11T19:08:03-07:00
ClockID: 0
uint64: 0x187dcbda2b5ca800
```

The `firehose` commands subscribes to the repo commit stream from a Relay. The default stream outputs event metadata, but doesn't include record blocks (bytes). The `--ops` variant will unpack records and output one line per record operation (instead of one line per commit event), and includes the record values themselves. Some example invocations:

```bash
# possible handle updates
$ goat firehose --account-events | jq .payload.handle
[...]

# text of posts (empty lines for post-deletions)
$ goat firehose - app.bsky.feed.post --ops | jq .record.text
[...]

# sample ratio of languages in current posts
$ goat firehose --ops -c app.bsky.feed.post | head -n100 | jq .record.langs[0] -c | sort | uniq -c | sort -nr
     51 "en"
     33 "ja"
      7 null
      3 "pt"
      2 "ko"
      1 "th"
      1 "id"
      1 "es"
      1 "am"
```

A minimal bsky posting interface, requires account login:

```bash
$ goat bsky post "hello from goat"
```

## Lexicon Development

In a project directory, download some existing schemas, which will get saved as JSON files in `./lexicons/`:

```
$ goat lex pull com.atproto.repo.strongRef com.atproto.moderation. app.bsky.actor.profile
 🟢 com.atproto.repo.strongRef
 🟢 com.atproto.moderation.defs
 🟢 com.atproto.moderation.createReport
 🟢 app.bsky.actor.profile
```

Create a new record schema and edit it:

```
$ goat lex new record dev.project.thing

$ $EDITOR ./lexicons/dev/project/thing.json
```

Lint all local lexicons:

```
$ goat lex lint
 🟢 lexicons/app/bsky/actor/profile.json
 🟢 lexicons/com/atproto/moderation/createReport.json
 🟢 lexicons/com/atproto/moderation/defs.json
 🟢 lexicons/com/atproto/repo/strongRef.json
 🟡 lexicons/dev/project/thing.json
    [missing-primary-description]: primary type missing a description
```

Check for differences against the live network, both for local edits or remote changes:

```
$ goat lex status
```

If you edited an existing schema, check schema evolution rules against the published version:

```
$ goat lex breaking
 🟡 app.bsky.actor.profile
    [object-required]: required fields change (main)
 🟢 com.atproto.repo.strongRef
```

Check DNS configuration before publishing to new Lexicon namespaces:

```
$ goat lex check-dns
Some lexicon NSIDs did not resolve via DNS:

    dev.project.*

To make these resolve, add DNS TXT entries like:

    _lexicon.project.dev	TXT	"did=did:web:lex.example.com"

(substituting your account DID for the example value)

Note that DNS management interfaces commonly require only the sub-domain parts of a name, not the full registered domain.
```

When ready, publish new or updated Lexicons:

```
# login with 'goat account login', or provide env vars (GOAT_USERNAME="user.example.com" and GOAT_PASSWORD="...")
$ goat lex publish
 🟢 dev.project.thing
```

## License

This project is dual-licensed under MIT and Apache 2.0 terms:

- MIT license ([LICENSE-MIT](https://github.com/bluesky-social/goat/blob/main/LICENSE-MIT) or http://opensource.org/licenses/MIT)
- Apache License, Version 2.0, ([LICENSE-APACHE](https://github.com/bluesky-social/goat/blob/main/LICENSE-APACHE) or http://www.apache.org/licenses/LICENSE-2.0)

Downstream projects and end users may chose either license individually, or both together, at their discretion. The motivation for this dual-licensing is the additional software patent assurance provided by Apache 2.0.
