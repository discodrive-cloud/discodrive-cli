<p align="center">
  <img src="img/logo.jpg" alt="DiscoDrive" width="100%">
</p>

# DiscoDrive CLI

[English](../README.md) · [Deutsch](README.de.md) · [Українська](README.uk.md) · [Français](README.fr.md) · [**Español**](README.es.md) · [Русский](README.ru.md) · [Српски](README.sr.md)

**Tu DiscoDrive desde la terminal — y dentro de tu asistente de IA.**

DiscoDrive CLI es un par de pequeñas herramientas de línea de comandos para tu nube [DiscoDrive](https://github.com/discodrive-cloud/discodrive):

- **`dd-cli`** — trabaja con tu DiscoDrive directamente desde la terminal: explora y transfiere archivos, crea enlaces para compartir y gestiona tu calendario, contactos, tareas, marcadores, música y libros. Cada comando tiene una `--help` detallada con ejemplos.
- **`dd-mcp`** — permite que un asistente de IA como Claude actúe dentro de tu DiscoDrive por ti. Por defecto puede leer y añadir elementos nuevos (carpetas, eventos, tareas, marcadores); la subida de archivos, los cambios en datos existentes y todo lo que haga que el servidor obtenga una URL (guardar páginas, descargas, radio, pódcasts) requieren `--allow-changes`.

Inicias sesión una vez con tu cuenta de DiscoDrive y ambas herramientas la recuerdan. En la nube no se ejecuta nada que no controles: las herramientas simplemente hablan con tu propio servidor.

---

## Qué puedes hacer

### 📁 Archivos

- Explorar carpetas, subir y descargar archivos, crear carpetas, mover, renombrar y eliminar.
- Crear **enlaces de compartición** públicos — con fecha de caducidad y acceso de solo lectura.
- Todo se corresponde con los mismos archivos y carpetas que ves en las apps de DiscoDrive.

### 📅 Calendario, contactos y tareas

- Listar y crear **eventos de calendario**, ver de un vistazo la próxima semana, eliminar eventos.
- Listar, añadir y eliminar **contactos**.
- Listar y añadir **tareas**, establecer vencimientos y prioridades, y marcarlas como hechas.

### 🔖 Marcadores, leer más tarde y descargas

- Explorar y editar los **marcadores del navegador** que sincroniza tu extensión de DiscoDrive — todo el árbol, carpetas incluidas: añadir, renombrar, mover entre carpetas, eliminar.
- Guardar una página para **leerla más tarde**: el servidor la obtiene, la convierte a markdown y la archiva; luego la lees en la terminal o la rediriges a un archivo.
- Entregar una URL al servidor para que **descargue por ti**, directamente a tu carpeta de descargas: la transferencia sigue en marcha después de que cierres el portátil.

### 🎵 Música y 📚 libros

- Explorar tu biblioteca de **música**, listar y añadir **emisoras de radio** y **pódcasts**, y lanzar un reanálisis de tu carpeta de música.
- Buscar en tu biblioteca de **libros** por autor, serie o género, descargar un libro y ver o editar sus metadatos.

### 🤖 Tu asistente de IA, dentro de tu nube

- Conecta Claude Desktop, Claude Code o cualquier otro asistente compatible con MCP (Cursor, Zed, VS Code y más) a tu DiscoDrive.
- Pídele a tu asistente que encuentre un archivo, lea una nota, añada un evento al calendario o una tarea: trabaja con tus datos reales.
- **Seguro por defecto:** el asistente puede leer, buscar y crear, pero subir archivos, eliminar, mover, renombrar y todo lo que haga que el servidor obtenga una URL permanecen desactivados hasta que lo permitas.

### 🌍 Tu idioma

- La interfaz y los mensajes están disponibles en **7 idiomas**: inglés, alemán, ucraniano, francés, español, ruso y serbio.

---

## Instalación

Ambas herramientas vienen juntas: cualquiera de los métodos de instalación siguientes te da `dd-cli` y `dd-mcp`.

### Homebrew (macOS y Linux)

```sh
brew install discodrive-cloud/tap/discodrive-cli
```

Para actualizar más adelante: `brew upgrade discodrive-cli`.

### Binarios precompilados

Descarga el archivo para tu sistema desde [Releases](https://github.com/discodrive-cloud/discodrive-cli/releases) — macOS, Linux y Windows, Intel y ARM. Descomprímelo y coloca `dd-cli` y `dd-mcp` en una carpeta de tu `PATH` (por ejemplo `~/bin` o `/usr/local/bin`). El archivo `checksums.txt` junto a los archivos comprimidos te permite verificar la descarga.

### Con Go

Con [Go](https://go.dev/dl/) 1.25 o posterior:

```sh
go install github.com/discodrive-cloud/discodrive-cli/cmd/dd-cli@latest
go install github.com/discodrive-cloud/discodrive-cli/cmd/dd-mcp@latest
```

Los binarios se instalan en `$(go env GOPATH)/bin`; asegúrate de que esté en tu `PATH`.

Comprueba la instalación con `dd-cli --version`.

## Inicio rápido

```sh
dd-cli login --server https://drive.example   # inicia sesión y conecta el asistente de IA
dd-cli ls                                      # contenido de la raíz del almacenamiento
dd-cli put ./report.pdf docs/report.pdf        # subir un archivo
dd-cli get docs/report.pdf                     # descargar un archivo
dd-cli share docs/report.pdf --expires 24h     # enlace público durante 24 horas
```

¿Servidor con certificado autofirmado? Añade `--insecure` a `login`: la opción se guarda en el perfil y todos los comandos posteriores y la conexión del asistente la usan automáticamente.

Ejecuta `dd-cli <comando> --help` para ver detalles y ejemplos de cualquier comando. Algunos comandos del día a día:

```sh
dd-cli contacts                                # tus contactos
dd-cli cal events                              # eventos de la próxima semana
dd-cli tasks add --summary "Llamar a Bob" --due 2026-07-05T09:00:00Z
dd-cli music list                              # explorar tu música
dd-cli books list --author "Knuth"             # buscar en la biblioteca
dd-cli bookmarks                               # tu árbol de marcadores
dd-cli pocket add https://example.org/article  # guardar una página para leerla más tarde
dd-cli download add https://example.org/x.iso  # que el servidor descargue un archivo
```

Las descargas no tienen listado en el servidor, así que `dd-cli` mantiene un pequeño registro local de las que se han puesto en cola desde esta máquina (`downloads.json`, junto a tu configuración). `dd-cli download list` lo relee y comprueba cada una; `dd-cli download status <id>` funciona con cualquier id, esté donde esté puesto en cola.

## Conectar tu asistente de IA

`dd-cli login` ya conecta Claude por ti. Para cambiar la conexión más adelante o conectar otros asistentes:

```sh
dd-cli setup-claude                 # conectar Claude (herramientas de lectura y creación)
dd-cli setup-claude --allow-changes # permitir subidas, cambios en datos existentes y obtención de URL
dd-cli print-config                 # imprimir un fragmento de configuración para cualquier cliente MCP
```

Después de cambiar el modo, reinicia tu asistente.

¿Prefieres configurar la conexión tú mismo, sin instalar `dd-cli`? `dd-mcp` es autónomo:

```sh
dd-mcp login --server https://drive.example    # iniciar sesión y conectar Claude
dd-mcp                                          # ejecutarlo
dd-mcp logout                                   # cerrar sesión
```

### Acceso remoto

Por defecto, la conexión del asistente es local y no necesita configuración adicional. Si la expones a la red, protégela con un token:

```sh
dd-mcp --http 127.0.0.1:8080                       # solo local
dd-mcp --http 0.0.0.0:8080 --http-token <secreto>  # por la red — token obligatorio
```

Una dirección de red no arrancará sin un token. Para el acceso desde fuera de tu máquina, ponla detrás de un proxy inverso con TLS.

## Idioma

Ambas herramientas hablan 7 idiomas: **en** (por defecto), **de**, **es**, **fr**, **ru**, **sr**, **uk**. El idioma se elige a partir de la primera fuente definida:

```sh
dd-cli --lang de ls                 # una vez, en alemán
DISCODRIVE_LANG=fr dd-cli whoami    # mediante una variable de entorno
```

En caso contrario, sigue la configuración regional de tu sistema (`LC_ALL` / `LANG`) y recurre al inglés.

---

## Seguridad de transferencias y calendario

Las descargas reemplazan los archivos locales solo después de que la transferencia completa y el cierre del archivo hayan tenido éxito.
Mover y renombrar en un solo comando `mv` requiere un servidor con el endpoint atómico
`PATCH /files/{id}/relocate`. Los servidores más antiguos devuelven un error sin mover
nada; no hay recurso a un movimiento parcial.

MCP `dd_files_read` acepta hasta 1 MiB de texto. Usa `dd-cli get` para archivos más grandes.
`dd_files_upload` requiere `--allow-changes`, ya que una subida puede sobrescribir un archivo.

Para eventos de calendario recurrentes, usa `--tz Europe/Berlin` (MCP: `tz`) para conservar
la hora local de las reuniones a través de los cambios de horario de verano. Las series nuevas usan por defecto
la zona IANA de la máquina; si no se puede determinar, indica una explícitamente. `--until`
es la última fecha del calendario, inclusive, por ejemplo `--until 2026-10-10`.
El servidor conserva la zona con nombre de un evento existente.

## Seguridad

- **Qué se guarda en tu máquina.** `dd-cli login` guarda un token de dispositivo en `~/.config/discodrive/cli.json` (legible solo por ti); un token de sesión de corta duración se almacena en caché en `~/.cache/discodrive/`. Cualquiera que pueda leer estos archivos puede actuar como tú en DiscoDrive: mantenlos privados.
- **Cerrar sesión.** `dd-cli logout` (o `dd-mcp logout`) revoca el dispositivo en el servidor y lo olvida localmente. Un dispositivo perdido también se puede desconectar en la app web de DiscoDrive, en Ajustes.
- **Usa https.** Con una dirección `http://` simple, el token y tus datos viajan sin cifrar; `login` te avisa de ello. `--insecure` solo omite la comprobación del certificado para un servidor autofirmado en el que confías.
- **Un inicio de sesión por servidor.** `--server` elige el inicio de sesión guardado para esa dirección; un token nunca se envía a un servidor que no lo haya emitido.
- **Asistentes de IA y contenido no fiable.** Las páginas web guardadas y los archivos compartidos pueden contener texto escrito para manipular a un asistente (inyección de prompts). Por eso los cambios, las subidas y todo lo que haga que el servidor obtenga una URL requieren `--allow-changes`. Actívalo solo si confías en lo que lee el asistente, y revisa lo que hace.
- **Acceso remoto.** `dd-mcp --http` se queda en `127.0.0.1` a menos que le des un token; consulta [Acceso remoto](#acceso-remoto).

**Informar de una vulnerabilidad:** escribe a [info@discodrive.app](mailto:info@discodrive.app) en lugar de abrir un issue público.

## Contribuir

Los informes de errores y las ideas son bienvenidos en [Issues](https://github.com/discodrive-cloud/discodrive-cli/issues). Por ahora no se aceptan pull requests.

---

## Licencia y uso comercial

DiscoDrive CLI es **source-available** bajo la licencia [PolyForm Noncommercial License 1.0.0](../LICENSE).

- ✅ **Gratis para cualquier uso no comercial** — instálalo para ti, tu familia, aficiones, estudios o experimentos. Para eso está hecho.
- ✅ **Modifícalo como quieras** — siempre que conserves el aviso de atribución obligatorio.
- ❌ **El uso comercial no está permitido.**

¿Necesitas uso comercial? Hay disponible una licencia comercial aparte — escribe a [info@discodrive.app](mailto:info@discodrive.app).

---

## ❤️ Apoya el proyecto

DiscoDrive se desarrolla por sus propios medios, sin inversión externa. Los comentarios y sugerencias son bienvenidos — escribe a [info@discodrive.app](mailto:info@discodrive.app).

Y si te gusta DiscoDrive y quieres que siga creciendo — apoya el proyecto:

- **[Dale una estrella en GitHub](https://github.com/discodrive-cloud/discodrive)** ⭐
- **[Ko-fi](https://ko-fi.com/kosmosoid)**
- **[Boosty](https://boosty.to/kosmosoid/donate)**
- **[Criptomonedas](https://nowpayments.io/donation?api_key=7P893T0-48B4KZH-Q6PXBBT-D79X9TK)** — a través de NOWPayments

¡Gracias! Cada aportación ayuda al proyecto a seguir adelante.
