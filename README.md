<p align="center">
  <img src="docs/img/logo.jpg" alt="DiscoDrive" width="100%">
</p>

# DiscoDrive CLI

[**English**](README.md) · [Deutsch](docs/README.de.md) · [Українська](docs/README.uk.md) · [Français](docs/README.fr.md) · [Español](docs/README.es.md) · [Русский](docs/README.ru.md) · [Српски](docs/README.sr.md)

**Your DiscoDrive from the terminal — and inside your AI assistant.**

DiscoDrive CLI is a pair of small command-line tools for your [DiscoDrive](https://github.com/discodrive-cloud/discodrive) cloud:

- **`dd-cli`** — work with your DiscoDrive straight from the terminal: browse and transfer files, share links, and manage your calendar, contacts, tasks, bookmarks, music and books. Every command has detailed `--help` with examples.
- **`dd-mcp`** — lets an AI assistant such as Claude act inside your DiscoDrive for you. By default it can read and add new items (folders, events, tasks, bookmarks); file uploads, changes to existing data, and anything that makes the server fetch a URL (saving pages, downloads, radio, podcasts) require `--allow-changes`.

You sign in once with your DiscoDrive account and both tools remember it. Nothing runs in the cloud that you do not control — these tools simply talk to your own server.

---

## What you can do

### 📁 Files

- Browse folders, upload and download files, create folders, move, rename, and delete.
- Create public **share links** — with an expiry date and read-only access.
- Everything maps to the same files and folders you see in the DiscoDrive apps.

### 📅 Calendar, contacts and tasks

- List and create **calendar events**, browse the week ahead, and remove events.
- List, add and remove **contacts**.
- List and add **tasks**, set due dates and priorities, and mark them done.

### 🔖 Bookmarks, read-later and downloads

- Browse and edit the **browser bookmarks** your DiscoDrive extension syncs — the whole tree, folders and all: add, rename, move between folders, delete.
- Save a page to **read later**: the server fetches it, converts it to markdown and files it away; read it back in the terminal or pipe it to a file.
- Hand a URL to the server to **download on your behalf**, straight into your Downloads folder — the transfer keeps running after you close the laptop.

### 🎵 Music and 📚 books

- Browse your **music** library, list and add **radio stations** and **podcasts**, and trigger a rescan of your music folder.
- Search your **book** library by author, series or genre, download a book, and view or edit its metadata.

### 🤖 Your AI assistant, inside your cloud

- Connect Claude Desktop, Claude Code, or any other MCP-compatible assistant (Cursor, Zed, VS Code and more) to your DiscoDrive.
- Ask your assistant to find a file, read a note, add a calendar event or a task — it works on your real data.
- **Safe by default:** the assistant can read, search and create, but file uploads, deleting, moving, renaming and anything that makes the server fetch a URL stay off until you allow them.

### 🌍 Your language

- The interface and messages are available in **7 languages**: English, German, Ukrainian, French, Spanish, Russian and Serbian.

---

## Install

Both tools come together: every install method below gives you `dd-cli` and `dd-mcp`.

### Homebrew (macOS and Linux)

```sh
brew install discodrive-cloud/tap/discodrive-cli
```

Update later with `brew upgrade discodrive-cli`.

### Prebuilt binaries

Download the archive for your system from [Releases](https://github.com/discodrive-cloud/discodrive-cli/releases) — macOS, Linux and Windows, Intel and ARM. Unpack it and put `dd-cli` and `dd-mcp` in a folder on your `PATH` (for example `~/bin` or `/usr/local/bin`). `checksums.txt` next to the archives lets you verify the download.

### With Go

With [Go](https://go.dev/dl/) 1.25 or newer:

```sh
go install github.com/discodrive-cloud/discodrive-cli/cmd/dd-cli@latest
go install github.com/discodrive-cloud/discodrive-cli/cmd/dd-mcp@latest
```

The binaries land in `$(go env GOPATH)/bin` — make sure it is on your `PATH`.

Check the install with `dd-cli --version`.

## Quick start

```sh
dd-cli login --server https://drive.example   # sign in and connect your AI assistant
dd-cli ls                                      # list the root of your storage
dd-cli put ./report.pdf docs/report.pdf        # upload a file
dd-cli get docs/report.pdf                     # download a file
dd-cli share docs/report.pdf --expires 24h     # public link for 24 hours
```

Server with a self-signed certificate? Add `--insecure` to `login` — the setting is saved in the profile, so every later command and the assistant connection use it automatically.

Run `dd-cli <command> --help` for details and examples on any command. A few everyday commands:

```sh
dd-cli contacts                                # your contacts
dd-cli cal events                              # events for the week ahead
dd-cli tasks add --summary "Call Bob" --due 2026-07-05T09:00:00Z
dd-cli music list                              # browse your music
dd-cli books list --author "Knuth"             # search your library
dd-cli bookmarks                               # your bookmark tree
dd-cli pocket add https://example.org/article  # save a page to read later
dd-cli download add https://example.org/x.iso  # let the server fetch a file
```

Downloads have no listing on the server, so `dd-cli` keeps a small local journal of the ones queued from this machine (`downloads.json`, next to your config). `dd-cli download list` reads it back and checks each one; `dd-cli download status <id>` works for any id, wherever it was queued.

## Connect your AI assistant

`dd-cli login` already connects Claude for you. To change the connection later, or to connect other assistants:

```sh
dd-cli setup-claude                 # connect Claude (read/create tools)
dd-cli setup-claude --allow-changes # allow uploads, changes to existing data and URL fetching
dd-cli print-config                 # print a config snippet for any MCP client
```

After changing the mode, restart your assistant.

Prefer to run the assistant connection yourself, without installing `dd-cli`? `dd-mcp` is self-contained:

```sh
dd-mcp login --server https://drive.example    # sign in and connect Claude
dd-mcp                                          # run it
dd-mcp logout                                   # sign out
```

### Remote access

By default the assistant connection is local and needs no extra setup. If you expose it over the network, protect it with a token:

```sh
dd-mcp --http 127.0.0.1:8080                       # local only
dd-mcp --http 0.0.0.0:8080 --http-token <secret>   # over the network — token required
```

A network address will not start without a token. For access from outside your machine, put it behind a reverse proxy with TLS.

## Language

Both tools speak 7 languages: **en** (default), **de**, **es**, **fr**, **ru**, **sr**, **uk**. The language is chosen from the first of these that is set:

```sh
dd-cli --lang de ls                 # once, in German
DISCODRIVE_LANG=fr dd-cli whoami    # via an environment variable
```

Otherwise it follows your system locale (`LC_ALL` / `LANG`), and falls back to English.

---

## Transfer and calendar safety

Downloads replace local files only after the entire transfer and file close succeed.
Moving and renaming in one `mv` command requires a server with the atomic
`PATCH /files/{id}/relocate` endpoint. Older servers return an error without moving
anything; there is no fallback to a partial move.

MCP `dd_files_read` accepts up to 1 MiB of text. Use `dd-cli get` for larger files.
`dd_files_upload` requires `--allow-changes`, since uploading can overwrite a file.

For recurring calendar events, use `--tz Europe/Berlin` (MCP: `tz`) to preserve
local meeting times across daylight-saving changes. New series default to the
machine's IANA zone; if it cannot be determined, specify one explicitly. `--until`
is the inclusive last calendar date, for example `--until 2026-10-10`.
An existing event's named zone is preserved by the server.

## Security

- **What is stored on your machine.** `dd-cli login` saves a device token in `~/.config/discodrive/cli.json` (readable only by you); a short-lived session token is cached in `~/.cache/discodrive/`. Anyone who can read these files can act as you in DiscoDrive — keep them private.
- **Signing out.** `dd-cli logout` (or `dd-mcp logout`) revokes the device on the server and forgets it locally. A lost device can also be disconnected in the DiscoDrive web app, under Settings.
- **Use https.** With a plain `http://` address the token and your data travel unencrypted; `login` warns you about it. `--insecure` only skips certificate checks for a self-signed server you trust.
- **One login per server.** `--server` picks the saved login for that address; a token is never sent to a server it was not issued by.
- **AI assistants and untrusted content.** Saved web pages and shared files can contain text written to manipulate an assistant (prompt injection). That is why changes, uploads and anything that makes the server fetch a URL require `--allow-changes`. Enable it only if you trust what the assistant reads, and review what it does.
- **Remote access.** `dd-mcp --http` stays on `127.0.0.1` unless you give it a token; see [Remote access](#remote-access).

**Reporting a vulnerability:** please write to [info@discodrive.app](mailto:info@discodrive.app) instead of opening a public issue.

## Contributing

Bug reports and ideas are welcome in [Issues](https://github.com/discodrive-cloud/discodrive-cli/issues). Pull requests are not accepted at the moment.

---

## License & commercial use

DiscoDrive CLI is **source-available** under the [PolyForm Noncommercial License 1.0.0](LICENSE).

- ✅ **Free for any non-commercial use** — install it for yourself, your family, hobby, study or experiments. That's the whole point.
- ✅ **Modify it however you like** — as long as you keep the required attribution notice.
- ❌ **Commercial use is not allowed.**

Need commercial use? A separate commercial license is available — write to [info@discodrive.app](mailto:info@discodrive.app).

---

## ❤️ Support the project

DiscoDrive is developed independently, with no outside investment. Feedback and suggestions are welcome — write to [info@discodrive.app](mailto:info@discodrive.app).

And if you enjoy DiscoDrive and want to see it keep growing — support the project:

- **[Star it on GitHub](https://github.com/discodrive-cloud/discodrive)** ⭐
- **[Ko-fi](https://ko-fi.com/kosmosoid)**
- **[Boosty](https://boosty.to/kosmosoid/donate)**
- **[Cryptocurrency](https://nowpayments.io/donation?api_key=7P893T0-48B4KZH-Q6PXBBT-D79X9TK)** — via NOWPayments

Thank you! Every contribution helps the project move forward.
