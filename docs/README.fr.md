<p align="center">
  <img src="img/logo.jpg" alt="DiscoDrive" width="100%">
</p>

# DiscoDrive CLI

[English](../README.md) · [Deutsch](README.de.md) · [Українська](README.uk.md) · [**Français**](README.fr.md) · [Español](README.es.md) · [Русский](README.ru.md) · [Српски](README.sr.md)

**Votre DiscoDrive dans le terminal — et dans votre assistant IA.**

DiscoDrive CLI est une paire de petits outils en ligne de commande pour votre cloud [DiscoDrive](https://github.com/discodrive-cloud/discodrive) :

- **`dd-cli`** — utilisez votre DiscoDrive directement depuis le terminal : parcourir et transférer des fichiers, créer des liens de partage, et gérer votre calendrier, vos contacts, vos tâches, vos marque-pages, votre musique et vos livres. Chaque commande dispose d'une `--help` détaillée avec des exemples.
- **`dd-mcp`** — permet à un assistant IA comme Claude d'agir dans votre DiscoDrive à votre place. Par défaut, il peut lire et ajouter de nouveaux éléments (dossiers, événements, tâches, marque-pages) ; le téléversement de fichiers, les modifications de données existantes et tout ce qui amène le serveur à récupérer une URL (enregistrement de pages, téléchargements, radio, podcasts) nécessitent `--allow-changes`.

Vous vous connectez une fois avec votre compte DiscoDrive, et les deux outils s'en souviennent. Rien ne s'exécute dans le cloud sans votre contrôle — les outils communiquent simplement avec votre propre serveur.

---

## Ce que vous pouvez faire

### 📁 Fichiers

- Parcourir les dossiers, téléverser et télécharger des fichiers, créer des dossiers, déplacer, renommer et supprimer.
- Créer des **liens de partage** publics — avec une date d'expiration et un accès en lecture seule.
- Tout correspond aux mêmes fichiers et dossiers que vous voyez dans les applications DiscoDrive.

### 📅 Calendrier, contacts et tâches

- Lister et créer des **événements de calendrier**, avoir un aperçu de la semaine à venir, supprimer des événements.
- Lister, ajouter et supprimer des **contacts**.
- Lister et ajouter des **tâches**, définir des échéances et des priorités, et les marquer comme terminées.

### 🔖 Marque-pages, lecture différée et téléchargements

- Parcourir et modifier les **marque-pages de navigateur** que votre extension DiscoDrive synchronise — l'arborescence entière, dossiers compris : ajouter, renommer, déplacer entre dossiers, supprimer.
- Enregistrer une page pour la **lire plus tard** : le serveur la récupère, la convertit en markdown et la range ; relisez-la ensuite dans le terminal ou redirigez-la vers un fichier.
- Confier une URL au serveur pour qu'il **télécharge à votre place**, directement dans votre dossier de téléchargements — le transfert continue après que vous avez refermé votre portable.

### 🎵 Musique et 📚 livres

- Parcourir votre bibliothèque **musicale**, lister et ajouter des **stations de radio** et des **podcasts**, et relancer une analyse de votre dossier de musique.
- Rechercher dans votre bibliothèque de **livres** par auteur, série ou genre, télécharger un livre, et consulter ou modifier ses métadonnées.

### 🤖 Votre assistant IA, dans votre cloud

- Connectez Claude Desktop, Claude Code ou tout autre assistant compatible MCP (Cursor, Zed, VS Code et d'autres) à votre DiscoDrive.
- Demandez à votre assistant de trouver un fichier, de lire une note, d'ajouter un événement au calendrier ou une tâche — il travaille sur vos données réelles.
- **Sûr par défaut :** l'assistant peut lire, rechercher et créer, mais le téléversement de fichiers, la suppression, le déplacement, le renommage et tout ce qui amène le serveur à récupérer une URL restent désactivés jusqu'à ce que vous les autorisiez.

### 🌍 Votre langue

- L'interface et les messages sont disponibles en **7 langues** : anglais, allemand, ukrainien, français, espagnol, russe et serbe.

---

## Installation

Les deux outils vont ensemble : chacune des méthodes d'installation ci-dessous vous fournit `dd-cli` et `dd-mcp`.

### Homebrew (macOS et Linux)

```sh
brew install discodrive-cloud/tap/discodrive-cli
```

Pour mettre à jour plus tard : `brew upgrade discodrive-cli`.

### Binaires précompilés

Téléchargez l'archive pour votre système depuis [Releases](https://github.com/discodrive-cloud/discodrive-cli/releases) — macOS, Linux et Windows, Intel et ARM. Décompressez-la et placez `dd-cli` et `dd-mcp` dans un dossier de votre `PATH` (par exemple `~/bin` ou `/usr/local/bin`). Le fichier `checksums.txt` à côté des archives vous permet de vérifier le téléchargement.

### Avec Go

Avec [Go](https://go.dev/dl/) 1.25 ou plus récent :

```sh
go install github.com/discodrive-cloud/discodrive-cli/cmd/dd-cli@latest
go install github.com/discodrive-cloud/discodrive-cli/cmd/dd-mcp@latest
```

Les binaires sont placés dans `$(go env GOPATH)/bin` — assurez-vous que ce dossier est dans votre `PATH`.

Vérifiez l'installation avec `dd-cli --version`.

## Démarrage rapide

```sh
dd-cli login --server https://drive.example   # se connecter et relier l'assistant IA
dd-cli ls                                      # contenu de la racine du stockage
dd-cli put ./report.pdf docs/report.pdf        # téléverser un fichier
dd-cli get docs/report.pdf                     # télécharger un fichier
dd-cli share docs/report.pdf --expires 24h     # lien public pour 24 heures
```

Serveur avec un certificat auto-signé ? Ajoutez `--insecure` à `login` — le réglage est enregistré dans le profil, et toutes les commandes suivantes ainsi que la connexion de l'assistant l'utilisent automatiquement.

Exécutez `dd-cli <commande> --help` pour les détails et des exemples sur n'importe quelle commande. Quelques commandes du quotidien :

```sh
dd-cli contacts                                # vos contacts
dd-cli cal events                              # événements de la semaine à venir
dd-cli tasks add --summary "Appeler Bob" --due 2026-07-05T09:00:00Z
dd-cli music list                              # parcourir votre musique
dd-cli books list --author "Knuth"             # rechercher dans la bibliothèque
dd-cli bookmarks                               # votre arborescence de marque-pages
dd-cli pocket add https://example.org/article  # enregistrer une page pour la lire plus tard
dd-cli download add https://example.org/x.iso  # laisser le serveur récupérer un fichier
```

Les téléchargements n'ont pas de liste sur le serveur : `dd-cli` tient donc un petit journal local de ceux mis en file depuis cette machine (`downloads.json`, à côté de votre configuration). `dd-cli download list` le relit et vérifie chacun d'eux ; `dd-cli download status <id>` fonctionne pour n'importe quel id, où qu'il ait été mis en file.

## Relier votre assistant IA

`dd-cli login` relie déjà Claude pour vous. Pour modifier la connexion plus tard ou relier d'autres assistants :

```sh
dd-cli setup-claude                 # relier Claude (outils de lecture et de création)
dd-cli setup-claude --allow-changes # autoriser les téléversements, les modifications de données existantes et la récupération d'URL
dd-cli print-config                 # afficher un extrait de configuration pour tout client MCP
```

Après avoir changé de mode, redémarrez votre assistant.

Vous préférez configurer la connexion vous-même, sans installer `dd-cli` ? `dd-mcp` est autonome :

```sh
dd-mcp login --server https://drive.example    # se connecter et relier Claude
dd-mcp                                          # le lancer
dd-mcp logout                                   # se déconnecter
```

### Accès à distance

Par défaut, la connexion de l'assistant est locale et ne nécessite aucune configuration supplémentaire. Si vous l'exposez sur le réseau, protégez-la avec un jeton :

```sh
dd-mcp --http 127.0.0.1:8080                       # local uniquement
dd-mcp --http 0.0.0.0:8080 --http-token <secret>   # sur le réseau — jeton requis
```

Une adresse réseau ne démarrera pas sans jeton. Pour un accès depuis l'extérieur de votre machine, placez-la derrière un reverse proxy avec TLS.

## Langue

Les deux outils parlent 7 langues : **en** (par défaut), **de**, **es**, **fr**, **ru**, **sr**, **uk**. La langue est choisie à partir de la première source définie :

```sh
dd-cli --lang de ls                 # ponctuellement, en allemand
DISCODRIVE_LANG=fr dd-cli whoami    # via une variable d'environnement
```

Sinon, elle suit la locale de votre système (`LC_ALL` / `LANG`) et revient à l'anglais.

---

## Sécurité des transferts et du calendrier

Les téléchargements ne remplacent les fichiers locaux qu'une fois le transfert complet et la fermeture du fichier réussis.
Déplacer et renommer en une seule commande `mv` nécessite un serveur disposant du point de terminaison atomique
`PATCH /files/{id}/relocate`. Les serveurs plus anciens renvoient une erreur sans rien
déplacer ; il n'y a pas de repli vers un déplacement partiel.

MCP `dd_files_read` accepte jusqu'à 1 MiB de texte. Utilisez `dd-cli get` pour les fichiers plus volumineux.
`dd_files_upload` nécessite `--allow-changes`, car un téléversement peut écraser un fichier.

Pour les événements récurrents du calendrier, utilisez `--tz Europe/Berlin` (MCP : `tz`) afin de conserver
les heures locales des réunions lors des changements d'heure. Les nouvelles séries utilisent par défaut
le fuseau IANA de la machine ; s'il ne peut pas être déterminé, indiquez-en un explicitement. `--until`
est la dernière date du calendrier, incluse, par exemple `--until 2026-10-10`.
Le fuseau nommé d'un événement existant est conservé par le serveur.

## Sécurité

- **Ce qui est stocké sur votre machine.** `dd-cli login` enregistre un jeton d'appareil dans `~/.config/discodrive/cli.json` (lisible par vous seul) ; un jeton de session de courte durée est mis en cache dans `~/.cache/discodrive/`. Toute personne pouvant lire ces fichiers peut agir en votre nom dans DiscoDrive — gardez-les privés.
- **Se déconnecter.** `dd-cli logout` (ou `dd-mcp logout`) révoque l'appareil sur le serveur et l'oublie localement. Un appareil perdu peut aussi être déconnecté dans l'application web DiscoDrive, dans les Paramètres.
- **Utilisez https.** Avec une adresse `http://` simple, le jeton et vos données transitent sans chiffrement ; `login` vous en avertit. `--insecure` ne fait qu'ignorer la vérification du certificat pour un serveur auto-signé auquel vous faites confiance.
- **Une connexion par serveur.** `--server` choisit la connexion enregistrée pour cette adresse ; un jeton n'est jamais envoyé à un serveur qui ne l'a pas émis.
- **Assistants IA et contenu non fiable.** Les pages web enregistrées et les fichiers partagés peuvent contenir du texte écrit pour manipuler un assistant (injection de prompt). C'est pourquoi les modifications, les téléversements et tout ce qui amène le serveur à récupérer une URL nécessitent `--allow-changes`. Ne l'activez que si vous faites confiance à ce que lit l'assistant, et vérifiez ce qu'il fait.
- **Accès à distance.** `dd-mcp --http` reste sur `127.0.0.1` tant que vous ne lui donnez pas de jeton ; voir [Accès à distance](#accès-à-distance).

**Signaler une vulnérabilité :** écrivez à [info@discodrive.app](mailto:info@discodrive.app) plutôt que d'ouvrir une issue publique.

## Contribuer

Les rapports de bugs et les idées sont les bienvenus dans les [Issues](https://github.com/discodrive-cloud/discodrive-cli/issues). Les pull requests ne sont pas acceptées pour le moment.

---

## Licence & usage commercial

DiscoDrive CLI est **source-available** sous la licence [PolyForm Noncommercial License 1.0.0](../LICENSE).

- ✅ **Gratuit pour tout usage non commercial** — installez-le pour vous-même, votre famille, vos loisirs, vos études ou vos expériences. C'est tout l'intérêt.
- ✅ **Modifiez-le comme bon vous semble** — à condition de conserver la mention d'attribution obligatoire.
- ❌ **L'usage commercial n'est pas autorisé.**

Besoin d'un usage commercial ? Une licence commerciale distincte est disponible — écrivez à [info@discodrive.app](mailto:info@discodrive.app).

---

## ❤️ Soutenir le projet

DiscoDrive est développé par ses propres moyens, sans investisseurs. Les retours et suggestions sont les bienvenus — écrivez à [info@discodrive.app](mailto:info@discodrive.app).

Et si DiscoDrive vous plaît et que vous souhaitez le voir grandir — soutenez le projet :

- **[Mettez une étoile sur GitHub](https://github.com/discodrive-cloud/discodrive)** ⭐
- **[Ko-fi](https://ko-fi.com/kosmosoid)**
- **[Boosty](https://boosty.to/kosmosoid/donate)**
- **[Cryptomonnaie](https://nowpayments.io/donation?api_key=7P893T0-48B4KZH-Q6PXBBT-D79X9TK)** — via NOWPayments

Merci ! Chaque contribution fait avancer le projet.
