<p align="center">
  <img src="img/logo.jpg" alt="DiscoDrive" width="100%">
</p>

# DiscoDrive CLI

[English](../README.md) · [**Deutsch**](README.de.md) · [Українська](README.uk.md) · [Français](README.fr.md) · [Español](README.es.md) · [Русский](README.ru.md) · [Српски](README.sr.md)

**Ihr DiscoDrive im Terminal — und in Ihrem KI-Assistenten.**

DiscoDrive CLI ist ein Paar kleiner Kommandozeilen-Werkzeuge für Ihre [DiscoDrive](https://github.com/discodrive-cloud/discodrive)-Cloud:

- **`dd-cli`** — arbeiten Sie direkt im Terminal mit Ihrem DiscoDrive: Dateien durchsuchen und übertragen, Freigabelinks erstellen sowie Kalender, Kontakte, Aufgaben, Lesezeichen, Musik und Bücher verwalten. Jeder Befehl hat eine ausführliche `--help` mit Beispielen.
- **`dd-mcp`** — lässt einen KI-Assistenten wie Claude für Sie in Ihrem DiscoDrive handeln. Standardmäßig kann er lesen und neue Einträge hinzufügen (Ordner, Termine, Aufgaben, Lesezeichen); Datei-Uploads, Änderungen an bestehenden Daten und alles, was den Server eine URL abrufen lässt (Seiten speichern, Downloads, Radio, Podcasts), erfordern `--allow-changes`.

Sie melden sich einmal mit Ihrem DiscoDrive-Konto an, und beide Werkzeuge merken es sich. In der Cloud läuft nichts, was Sie nicht kontrollieren — die Werkzeuge sprechen einfach mit Ihrem eigenen Server.

---

## Was Sie tun können

### 📁 Dateien

- Ordner durchsuchen, Dateien hoch- und herunterladen, Ordner anlegen, verschieben, umbenennen und löschen.
- Öffentliche **Freigabelinks** erstellen — mit Ablaufdatum und Nur-Lese-Zugriff.
- Alles entspricht denselben Dateien und Ordnern, die Sie in den DiscoDrive-Apps sehen.

### 📅 Kalender, Kontakte und Aufgaben

- **Kalendertermine** auflisten und erstellen, die kommende Woche überblicken, Termine löschen.
- **Kontakte** auflisten, hinzufügen und entfernen.
- **Aufgaben** auflisten und hinzufügen, Fälligkeiten und Prioritäten setzen und als erledigt markieren.

### 🔖 Lesezeichen, Später-lesen und Downloads

- Die **Browser-Lesezeichen**, die Ihre DiscoDrive-Erweiterung synchronisiert, durchsuchen und bearbeiten — den ganzen Baum samt Ordnern: hinzufügen, umbenennen, zwischen Ordnern verschieben, löschen.
- Eine Seite zum **späteren Lesen** speichern: Der Server holt sie, wandelt sie in Markdown um und legt sie ab; lesen Sie sie später im Terminal oder leiten Sie sie in eine Datei um.
- Geben Sie dem Server eine URL, damit er **für Sie herunterlädt** — direkt in Ihren Downloads-Ordner; die Übertragung läuft weiter, nachdem Sie den Laptop zugeklappt haben.

### 🎵 Musik und 📚 Bücher

- Ihre **Musik**-Bibliothek durchsuchen, **Radiosender** und **Podcasts** auflisten und hinzufügen, einen erneuten Scan des Musikordners auslösen.
- Ihre **Bücher**-Bibliothek nach Autor, Reihe oder Genre durchsuchen, ein Buch herunterladen und seine Metadaten ansehen oder bearbeiten.

### 🤖 Ihr KI-Assistent, in Ihrer Cloud

- Verbinden Sie Claude Desktop, Claude Code oder jeden anderen MCP-kompatiblen Assistenten (Cursor, Zed, VS Code und mehr) mit Ihrem DiscoDrive.
- Bitten Sie Ihren Assistenten, eine Datei zu finden, eine Notiz zu lesen, einen Kalendertermin oder eine Aufgabe hinzuzufügen — er arbeitet mit Ihren echten Daten.
- **Sicher standardmäßig:** Der Assistent kann lesen, suchen und erstellen, aber Datei-Uploads, Löschen, Verschieben, Umbenennen und alles, was den Server eine URL abrufen lässt, bleiben aus, bis Sie sie erlauben.

### 🌍 Ihre Sprache

- Oberfläche und Meldungen sind in **7 Sprachen** verfügbar: Englisch, Deutsch, Ukrainisch, Französisch, Spanisch, Russisch und Serbisch.

---

## Installation

Beide Werkzeuge kommen zusammen: Jede der folgenden Installationsmethoden liefert Ihnen `dd-cli` und `dd-mcp`.

### Homebrew (macOS und Linux)

```sh
brew install discodrive-cloud/tap/discodrive-cli
```

Später aktualisieren Sie mit `brew upgrade discodrive-cli`.

### Vorkompilierte Binärdateien

Laden Sie das Archiv für Ihr System unter [Releases](https://github.com/discodrive-cloud/discodrive-cli/releases) herunter — macOS, Linux und Windows, Intel und ARM. Entpacken Sie es und legen Sie `dd-cli` und `dd-mcp` in einen Ordner in Ihrem `PATH` (zum Beispiel `~/bin` oder `/usr/local/bin`). Mit `checksums.txt` neben den Archiven können Sie den Download überprüfen.

### Mit Go

Mit [Go](https://go.dev/dl/) 1.25 oder neuer:

```sh
go install github.com/discodrive-cloud/discodrive-cli/cmd/dd-cli@latest
go install github.com/discodrive-cloud/discodrive-cli/cmd/dd-mcp@latest
```

Die Binärdateien landen in `$(go env GOPATH)/bin` — stellen Sie sicher, dass dieser Ordner in Ihrem `PATH` liegt.

Prüfen Sie die Installation mit `dd-cli --version`.

## Schnellstart

```sh
dd-cli login --server https://drive.example   # anmelden und KI-Assistenten verbinden
dd-cli ls                                      # Inhalt des Speicher-Stammverzeichnisses
dd-cli put ./report.pdf docs/report.pdf        # eine Datei hochladen
dd-cli get docs/report.pdf                     # eine Datei herunterladen
dd-cli share docs/report.pdf --expires 24h     # öffentlicher Link für 24 Stunden
```

Server mit selbstsigniertem Zertifikat? Fügen Sie `--insecure` zu `login` hinzu — die Einstellung wird im Profil gespeichert, sodass alle späteren Befehle und die Assistenten-Verbindung sie automatisch verwenden.

Führen Sie `dd-cli <Befehl> --help` für Details und Beispiele zu jedem Befehl aus. Ein paar alltägliche Befehle:

```sh
dd-cli contacts                                # Ihre Kontakte
dd-cli cal events                              # Termine der kommenden Woche
dd-cli tasks add --summary "Bob anrufen" --due 2026-07-05T09:00:00Z
dd-cli music list                              # Musik durchsuchen
dd-cli books list --author "Knuth"             # in der Bibliothek suchen
dd-cli bookmarks                               # Ihr Lesezeichen-Baum
dd-cli pocket add https://example.org/article  # eine Seite zum späteren Lesen speichern
dd-cli download add https://example.org/x.iso  # den Server eine Datei holen lassen
```

Für Downloads gibt es auf dem Server keine Auflistung, deshalb führt `dd-cli` ein kleines lokales Journal über die von diesem Rechner eingereihten Downloads (`downloads.json`, neben Ihrer Konfiguration). `dd-cli download list` liest es zurück und prüft jeden Eintrag; `dd-cli download status <id>` funktioniert für jede id, ganz gleich, wo sie eingereiht wurde.

## KI-Assistenten verbinden

`dd-cli login` verbindet Claude bereits für Sie. Um die Verbindung später zu ändern oder andere Assistenten zu verbinden:

```sh
dd-cli setup-claude                 # Claude verbinden (Lese- und Erstell-Tools)
dd-cli setup-claude --allow-changes # Uploads, Änderungen an bestehenden Daten und URL-Abrufe erlauben
dd-cli print-config                 # Konfigurations-Snippet für jeden MCP-Client ausgeben
```

Starten Sie Ihren Assistenten nach dem Moduswechsel neu.

Möchten Sie die Assistenten-Verbindung selbst einrichten, ohne `dd-cli` zu installieren? `dd-mcp` ist eigenständig:

```sh
dd-mcp login --server https://drive.example    # anmelden und Claude verbinden
dd-mcp                                          # starten
dd-mcp logout                                   # abmelden
```

### Fernzugriff

Standardmäßig ist die Assistenten-Verbindung lokal und benötigt keine weitere Einrichtung. Wenn Sie sie über das Netzwerk verfügbar machen, schützen Sie sie mit einem Token:

```sh
dd-mcp --http 127.0.0.1:8080                       # nur lokal
dd-mcp --http 0.0.0.0:8080 --http-token <Geheimnis> # über das Netzwerk — Token erforderlich
```

Eine Netzwerkadresse startet ohne Token nicht. Für den Zugriff von außerhalb Ihres Rechners setzen Sie einen Reverse-Proxy mit TLS davor.

## Sprache

Beide Werkzeuge sprechen 7 Sprachen: **en** (Standard), **de**, **es**, **fr**, **ru**, **sr**, **uk**. Die Sprache wird aus der ersten gesetzten Quelle gewählt:

```sh
dd-cli --lang de ls                 # einmalig, auf Deutsch
DISCODRIVE_LANG=fr dd-cli whoami    # über eine Umgebungsvariable
```

Andernfalls folgt sie Ihrer System-Locale (`LC_ALL` / `LANG`) und fällt auf Englisch zurück.

---

## Sicherheit bei Übertragungen und im Kalender

Downloads ersetzen lokale Dateien erst, nachdem die gesamte Übertragung und das Schließen der Datei erfolgreich waren.
Verschieben und Umbenennen in einem einzigen `mv`-Befehl erfordert einen Server mit dem atomaren
Endpunkt `PATCH /files/{id}/relocate`. Ältere Server geben einen Fehler zurück, ohne etwas
zu verschieben; einen Rückfall auf ein teilweises Verschieben gibt es nicht.

MCP `dd_files_read` akzeptiert bis zu 1 MiB Text. Verwenden Sie `dd-cli get` für größere Dateien.
`dd_files_upload` erfordert `--allow-changes`, da ein Upload eine Datei überschreiben kann.

Verwenden Sie für wiederkehrende Kalendertermine `--tz Europe/Berlin` (MCP: `tz`), damit
lokale Besprechungszeiten über Sommer-/Winterzeitumstellungen hinweg erhalten bleiben. Neue Serien verwenden standardmäßig
die IANA-Zone des Rechners; lässt sie sich nicht ermitteln, geben Sie eine ausdrücklich an. `--until`
ist das letzte Kalenderdatum einschließlich, zum Beispiel `--until 2026-10-10`.
Die benannte Zone eines bestehenden Termins wird vom Server beibehalten.

## Sicherheit

- **Was auf Ihrem Rechner gespeichert wird.** `dd-cli login` speichert ein Geräte-Token in `~/.config/discodrive/cli.json` (nur für Sie lesbar); ein kurzlebiges Sitzungs-Token wird in `~/.cache/discodrive/` zwischengespeichert. Wer diese Dateien lesen kann, kann in DiscoDrive in Ihrem Namen handeln — halten Sie sie privat.
- **Abmelden.** `dd-cli logout` (oder `dd-mcp logout`) widerruft das Gerät auf dem Server und vergisst es lokal. Ein verlorenes Gerät können Sie auch in der DiscoDrive-Web-App unter Einstellungen trennen.
- **Verwenden Sie https.** Bei einer einfachen `http://`-Adresse werden das Token und Ihre Daten unverschlüsselt übertragen; `login` warnt Sie davor. `--insecure` überspringt lediglich die Zertifikatsprüfung für einen selbstsignierten Server, dem Sie vertrauen.
- **Eine Anmeldung pro Server.** `--server` wählt die gespeicherte Anmeldung für diese Adresse; ein Token wird nie an einen Server gesendet, der es nicht ausgestellt hat.
- **KI-Assistenten und nicht vertrauenswürdige Inhalte.** Gespeicherte Webseiten und geteilte Dateien können Text enthalten, der einen Assistenten manipulieren soll (Prompt-Injection). Deshalb erfordern Änderungen, Uploads und alles, was den Server eine URL abrufen lässt, `--allow-changes`. Aktivieren Sie es nur, wenn Sie dem vertrauen, was der Assistent liest, und prüfen Sie, was er tut.
- **Fernzugriff.** `dd-mcp --http` bleibt auf `127.0.0.1`, solange Sie kein Token angeben; siehe [Fernzugriff](#fernzugriff).

**Eine Sicherheitslücke melden:** Bitte schreiben Sie an [info@discodrive.app](mailto:info@discodrive.app), statt ein öffentliches Issue zu eröffnen.

## Mitwirken

Fehlerberichte und Ideen sind in den [Issues](https://github.com/discodrive-cloud/discodrive-cli/issues) willkommen. Pull Requests werden derzeit nicht angenommen.

---

## Lizenz & kommerzielle Nutzung

DiscoDrive CLI ist **source-available** unter der [PolyForm Noncommercial License 1.0.0](../LICENSE).

- ✅ **Kostenlos für jede nicht-kommerzielle Nutzung** — installieren Sie es für sich selbst, Ihre Familie, als Hobby, zum Lernen oder für Experimente. Genau dafür ist es gemacht.
- ✅ **Ändern Sie es, wie Sie möchten** — solange Sie den vorgeschriebenen Urheberrechtshinweis beibehalten.
- ❌ **Kommerzielle Nutzung ist nicht erlaubt.**

Sie brauchen kommerzielle Nutzung? Eine separate kommerzielle Lizenz ist verfügbar — schreiben Sie an [info@discodrive.app](mailto:info@discodrive.app).

---

## ❤️ Das Projekt unterstützen

DiscoDrive wird aus eigener Kraft entwickelt, ohne Investitionen. Rückmeldungen und Vorschläge sind willkommen — schreiben Sie an [info@discodrive.app](mailto:info@discodrive.app).

Und wenn Ihnen DiscoDrive gefällt und Sie möchten, dass es weiter wächst — unterstützen Sie das Projekt:

- **[Geben Sie ihm einen Stern auf GitHub](https://github.com/discodrive-cloud/discodrive)** ⭐
- **[Ko-fi](https://ko-fi.com/kosmosoid)**
- **[Boosty](https://boosty.to/kosmosoid/donate)**
- **[Kryptowährung](https://nowpayments.io/donation?api_key=7P893T0-48B4KZH-Q6PXBBT-D79X9TK)** — über NOWPayments

Danke! Jeder Beitrag bringt das Projekt voran.
