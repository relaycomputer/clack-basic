# 🚀 Release-Management & Cross-Plattform Builds

Clack-Studio und Clack-BASIC verfügen über ein vollautomatisches Release- und Deployment-System für **Windows (x64)** und **Linux (x86_64)** über die Forgejo REST API.

---

## ⬇️ Vorkompilierte Binaries herunterladen

Alle Releases und fertigen Binärdateien können ohne Installation direkt im **Releases-Reiter** auf Forgejo heruntergeladen werden:
* 🌐 **Clack-Studio Releases:** `https://forge.scrapheap.eu/relay-computing/clack-studio/releases`
  * `clack-studio.exe` (Windows 64-Bit Portable – öffnet automatisch den Browser)
  * `clack-studio` (Linux x86_64 Portable)
* 📜 **Clack-BASIC Releases:** `https://forge.scrapheap.eu/relay-computing/clack-basic/releases`
  * `clack.exe` (Windows 64-Bit Compiler & USB-Flasher CLI)
  * `clack` (Linux x86_64 Compiler & USB-Flasher CLI)

---

## 🛠️ Automatisiertes Veröffentlichen (`scripts/publish-release.sh`)

In beiden Repositories liegt das Skript `scripts/publish-release.sh`, das in einem einzigen Durchlauf:
1. Das Svelte 5 Web-Frontend kompiliert (`npm run build`).
2. Native Binaries für Linux und Windows erzeugt (`go build` & `GOOS=windows go build`).
3. Über die Forgejo API das Release mit Versions-Tag anlegt.
4. Die Binärdateien als offizielle Release-Assets hochlädt.

### Verwendung:
```bash
# Syntax: ./scripts/publish-release.sh <Tag> "[Optionaler Titel]"

cd /home/veitscherbub/work/relay-computing/clack-studio
./scripts/publish-release.sh v2.3.0 "Release v2.3.0: Neue Features"
```

---

## 🔑 Authentifizierung (Forgejo Token)

Das Skript nutzt die Umgebungsvariable `FORGEJO_TOKEN` (bzw. den hinterlegten Personal Access Token mit `repository:write`-Rechten).
