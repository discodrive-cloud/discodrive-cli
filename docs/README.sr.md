<p align="center">
  <img src="img/logo.jpg" alt="DiscoDrive" width="100%">
</p>

# DiscoDrive CLI

[English](../README.md) · [Deutsch](README.de.md) · [Українська](README.uk.md) · [Français](README.fr.md) · [Español](README.es.md) · [Русский](README.ru.md) · [**Српски**](README.sr.md)

**Vaš DiscoDrive iz terminala — i unutar vašeg AI asistenta.**

DiscoDrive CLI je par malih alata komandne linije za vaš [DiscoDrive](https://github.com/discodrive-cloud/discodrive) oblak:

- **`dd-cli`** — radite sa svojim DiscoDrive-om direktno iz terminala: pregled i prenos datoteka, linkovi za deljenje, kao i upravljanje kalendarom, kontaktima, zadacima, obeleživačima, muzikom i knjigama. Svaka komanda ima detaljan `--help` sa primerima.
- **`dd-mcp`** — omogućava AI asistentu poput Claude-a da deluje u vašem DiscoDrive-u umesto vas. Podrazumevano može da čita i dodaje nove stavke (foldere, događaje, zadatke, obeleživače); otpremanje datoteka, izmene postojećih podataka i sve što navodi server da preuzme URL (čuvanje stranica, preuzimanja, radio, podkasti) zahtevaju `--allow-changes`.

Prijavljujete se jednom sa svojim DiscoDrive nalogom i oba alata to pamte. U oblaku se ne izvršava ništa što vi ne kontrolišete — alati jednostavno komuniciraju sa vašim sopstvenim serverom.

---

## Šta možete da radite

### 📁 Datoteke

- Pregled foldera, otpremanje i preuzimanje datoteka, kreiranje foldera, premeštanje, preimenovanje i brisanje.
- Kreiranje javnih **linkova za deljenje** — sa datumom isteka i pristupom samo za čitanje.
- Sve odgovara istim datotekama i folderima koje vidite u DiscoDrive aplikacijama.

### 📅 Kalendar, kontakti i zadaci

- Izlistavanje i kreiranje **događaja u kalendaru**, pregled naredne nedelje, brisanje događaja.
- Izlistavanje, dodavanje i uklanjanje **kontakata**.
- Izlistavanje i dodavanje **zadataka**, postavljanje rokova i prioriteta, i označavanje kao završeno.

### 🔖 Obeleživači, čitanje kasnije i preuzimanja

- Pregled i izmena **obeleživača pregledača** koje sinhronizuje vaše DiscoDrive proširenje — čitavog stabla, zajedno sa folderima: dodavanje, preimenovanje, premeštanje između foldera, brisanje.
- Čuvanje stranice da biste je **pročitali kasnije**: server je preuzima, konvertuje u markdown i odlaže; kasnije je pročitajte u terminalu ili je preusmerite u datoteku.
- Predajte serveru URL da **preuzme umesto vas**, pravo u vaš folder za preuzimanja — prenos se nastavlja i nakon što zatvorite laptop.

### 🎵 Muzika i 📚 knjige

- Pregled vaše **muzičke** biblioteke, izlistavanje i dodavanje **radio stanica** i **podkasta**, i pokretanje ponovnog skeniranja foldera sa muzikom.
- Pretraga vaše biblioteke **knjiga** po autoru, seriji ili žanru, preuzimanje knjige, i pregled ili izmena njenih metapodataka.

### 🤖 Vaš AI asistent, u vašem oblaku

- Povežite Claude Desktop, Claude Code ili bilo kog drugog MCP-kompatibilnog asistenta (Cursor, Zed, VS Code i druge) sa svojim DiscoDrive-om.
- Zamolite asistenta da pronađe datoteku, pročita belešku, doda događaj u kalendar ili zadatak — radi sa vašim stvarnim podacima.
- **Bezbedno podrazumevano:** asistent može da čita, pretražuje i kreira, ali otpremanje datoteka, brisanje, premeštanje, preimenovanje i sve što navodi server da preuzme URL ostaju isključeni dok ih ne dozvolite.

### 🌍 Vaš jezik

- Interfejs i poruke dostupni su na **7 jezika**: engleskom, nemačkom, ukrajinskom, francuskom, španskom, ruskom i srpskom.

---

## Instalacija

Oba alata dolaze zajedno: svaki od načina instalacije ispod daje vam i `dd-cli` i `dd-mcp`.

### Homebrew (macOS i Linux)

```sh
brew install discodrive-cloud/tap/discodrive-cli
```

Kasnije ažurirajte komandom `brew upgrade discodrive-cli`.

### Gotove binarne datoteke

Preuzmite arhivu za svoj sistem sa stranice [Releases](https://github.com/discodrive-cloud/discodrive-cli/releases) — macOS, Linux i Windows, Intel i ARM. Raspakujte je i stavite `dd-cli` i `dd-mcp` u folder koji je u vašem `PATH`-u (na primer `~/bin` ili `/usr/local/bin`). `checksums.txt` pored arhiva omogućava da proverite preuzimanje.

### Preko Go-a

Sa [Go](https://go.dev/dl/) 1.25 ili novijim:

```sh
go install github.com/discodrive-cloud/discodrive-cli/cmd/dd-cli@latest
go install github.com/discodrive-cloud/discodrive-cli/cmd/dd-mcp@latest
```

Binarne datoteke završavaju u `$(go env GOPATH)/bin` — uverite se da je taj folder u vašem `PATH`-u.

Proverite instalaciju komandom `dd-cli --version`.

## Brzi početak

```sh
dd-cli login --server https://drive.example   # prijava i povezivanje AI asistenta
dd-cli ls                                      # sadržaj korena skladišta
dd-cli put ./report.pdf docs/report.pdf        # otpremi datoteku
dd-cli get docs/report.pdf                     # preuzmi datoteku
dd-cli share docs/report.pdf --expires 24h     # javni link na 24 sata
```

Server sa samopotpisanim sertifikatom? Dodajte `--insecure` uz `login` — podešavanje se čuva u profilu, pa ga sve kasnije komande i veza sa asistentom koriste automatski.

Pokrenite `dd-cli <komanda> --help` za detalje i primere za bilo koju komandu. Nekoliko svakodnevnih komandi:

```sh
dd-cli contacts                                # vaši kontakti
dd-cli cal events                              # događaji za narednu nedelju
dd-cli tasks add --summary "Pozovi Boba" --due 2026-07-05T09:00:00Z
dd-cli music list                              # pregled muzike
dd-cli books list --author "Knuth"             # pretraga biblioteke
dd-cli bookmarks                               # vaše stablo obeleživača
dd-cli pocket add https://example.org/article  # sačuvaj stranicu za kasnije čitanje
dd-cli download add https://example.org/x.iso  # neka server preuzme datoteku
```

Preuzimanja nemaju listu na serveru, pa `dd-cli` vodi mali lokalni dnevnik onih koja su stavljena u red sa ove mašine (`downloads.json`, pored vaše konfiguracije). `dd-cli download list` ga čita i proverava svako od njih; `dd-cli download status <id>` radi za bilo koji id, gde god da je stavljen u red.

## Povezivanje AI asistenta

`dd-cli login` već povezuje Claude za vas. Da biste kasnije promenili vezu ili povezali druge asistente:

```sh
dd-cli setup-claude                 # poveži Claude (čitanje i kreiranje)
dd-cli setup-claude --allow-changes # dozvoli otpremanje, izmene postojećih podataka i preuzimanje URL-ova
dd-cli print-config                 # ispiši isečak konfiguracije za bilo kog MCP klijenta
```

Nakon promene režima, ponovo pokrenite asistenta.

Više volite da vezu podesite sami, bez instaliranja `dd-cli`? `dd-mcp` je samostalan:

```sh
dd-mcp login --server https://drive.example    # prijava i povezivanje Claude-a
dd-mcp                                          # pokreni ga
dd-mcp logout                                   # odjava
```

### Daljinski pristup

Podrazumevano je veza asistenta lokalna i ne zahteva dodatno podešavanje. Ako je izložite mreži, zaštitite je tokenom:

```sh
dd-mcp --http 127.0.0.1:8080                       # samo lokalno
dd-mcp --http 0.0.0.0:8080 --http-token <tajna>    # preko mreže — token obavezan
```

Mrežna adresa se neće pokrenuti bez tokena. Za pristup izvan vaše mašine, postavite je iza reverse proxy-ja sa TLS-om.

## Jezik

Oba alata govore 7 jezika: **en** (podrazumevano), **de**, **es**, **fr**, **ru**, **sr**, **uk**. Jezik se bira prema prvom postavljenom izvoru:

```sh
dd-cli --lang de ls                 # jednokratno, na nemačkom
DISCODRIVE_LANG=fr dd-cli whoami    # preko promenljive okruženja
```

U suprotnom prati vašu sistemsku lokalizaciju (`LC_ALL` / `LANG`) i vraća se na engleski.

---

## Bezbednost prenosa i kalendara

Preuzimanja zamenjuju lokalne datoteke tek kada se ceo prenos i zatvaranje datoteke
uspešno završe. Premeštanje i preimenovanje jednom `mv` komandom zahteva server sa
atomskim `PATCH /files/{id}/relocate` endpointom. Stariji serveri vraćaju grešku bez
premeštanja bilo čega; nema rezervnog delimičnog premeštanja.

MCP `dd_files_read` prihvata do 1 MiB teksta. Za veće datoteke koristite `dd-cli get`.
`dd_files_upload` zahteva `--allow-changes`, jer otpremanje može da prepiše datoteku.

Za ponavljajuće događaje u kalendaru koristite `--tz Europe/Berlin` (MCP: `tz`) da biste
sačuvali lokalno vreme sastanaka pri prelasku na letnje ili zimsko računanje vremena.
Nove serije podrazumevano koriste IANA zonu mašine; ako se ona ne može odrediti, navedite
je izričito. `--until` je poslednji kalendarski datum, uključujući i njega, na primer
`--until 2026-10-10`. Server čuva imenovanu zonu postojećeg događaja.

## Bezbednost

- **Šta se čuva na vašoj mašini.** `dd-cli login` čuva token uređaja u `~/.config/discodrive/cli.json` (čitljiv samo vama); kratkotrajni token sesije kešira se u `~/.cache/discodrive/`. Svako ko može da pročita ove datoteke može da deluje kao vi u DiscoDrive-u — čuvajte ih privatnim.
- **Odjava.** `dd-cli logout` (ili `dd-mcp logout`) opoziva uređaj na serveru i zaboravlja ga lokalno. Izgubljeni uređaj možete da isključite i u DiscoDrive veb aplikaciji, u Podešavanjima.
- **Koristite https.** Sa običnom `http://` adresom token i vaši podaci putuju nešifrovano; `login` vas na to upozorava. `--insecure` samo preskače proveru sertifikata za samopotpisani server kojem verujete.
- **Jedna prijava po serveru.** `--server` bira sačuvanu prijavu za tu adresu; token se nikada ne šalje serveru koji ga nije izdao.
- **AI asistenti i nepouzdan sadržaj.** Sačuvane veb stranice i deljene datoteke mogu da sadrže tekst napisan da bi manipulisao asistentom (prompt injection). Zato izmene, otpremanja i sve što navodi server da preuzme URL zahtevaju `--allow-changes`. Uključite ga samo ako verujete onome što asistent čita, i proveravajte šta radi.
- **Daljinski pristup.** `dd-mcp --http` ostaje na `127.0.0.1` dok mu ne date token; pogledajte [Daljinski pristup](#daljinski-pristup).

**Prijava ranjivosti:** pišite na [info@discodrive.app](mailto:info@discodrive.app) umesto da otvarate javni issue.

## Doprinos

Prijave grešaka i ideje su dobrodošle u [Issues](https://github.com/discodrive-cloud/discodrive-cli/issues). Pull request-ovi se trenutno ne prihvataju.

---

## Licenca i komercijalna upotreba

DiscoDrive CLI se distribuira kao **source-available** pod licencom [PolyForm Noncommercial License 1.0.0](../LICENSE).

- ✅ **Besplatno za svaku nekomercijalnu upotrebu** — instalirajte ga za sebe, porodicu, hobi, učenje ili eksperimente. Upravo zbog toga je i napravljen.
- ✅ **Menjajte kako god želite** — uz uslov da zadržite obavezno navođenje autorstva.
- ❌ **Komercijalna upotreba nije dozvoljena.**

Treba vam komercijalna upotreba? Dostupna je zasebna komercijalna licenca — pišite na [info@discodrive.app](mailto:info@discodrive.app).

---

## ❤️ Podržite projekat

DiscoDrive se razvija sopstvenim snagama, bez investicija. Povratne informacije i predlozi su dobrodošli — pišite na [info@discodrive.app](mailto:info@discodrive.app).

A ako vam se DiscoDrive sviđa i želite da raste i dalje — podržite projekat:

- **[Dajte zvezdicu na GitHub-u](https://github.com/discodrive-cloud/discodrive)** ⭐
- **[Ko-fi](https://ko-fi.com/kosmosoid)**
- **[Boosty](https://boosty.to/kosmosoid/donate)**
- **[Kriptovalute](https://nowpayments.io/donation?api_key=7P893T0-48B4KZH-Q6PXBBT-D79X9TK)** — preko NOWPayments

Hvala! Svaki doprinos pomaže projektu da ide napred.
