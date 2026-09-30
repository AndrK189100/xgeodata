# xgeodata

Generate `geoip.dat` and `geosite.dat` for [Xray-core](https://github.com/XTLS/Xray-core) from plain-text lists, and make Xray reload them automatically.

> Unofficial third-party tool. Not affiliated with the Xray project.

You keep your rules in simple text files (`name.domain`, `name.cidr`). `xgeodata` turns them into `.dat` files and, if asked, tells a running Xray to reload them through its gRPC API, without a restart and without waiting for a cron schedule. One place to edit, everything else happens by itself.

## How it works

```
data/
  allowed_clients.cidr   ->  geoip.dat    (geoip:allowed_clients)
  my_sites.domain        ->  geosite.dat  (geosite:my_sites)
```

1. Every `*.cidr` file becomes a list in `geoip.dat`, every `*.domain` file becomes a list in `geosite.dat`. The file name (without extension) is the list name.
2. In `-watch` mode the input directory is monitored. After a change (with a 500 ms debounce) the `.dat` files are regenerated.
3. After a successful generation, `xgeodata` calls `ReloadGeoData` on Xray's `RoutingService`. If generation fails, Xray is not touched.

If `-api` is empty, the tool works as a plain `.dat` generator and never contacts Xray.

## Requirements

- Go (see `go.mod` for the version) to build.
- To use `-api` you need an Xray build that has the `ReloadGeoData` method in `RoutingService`.
  This method is **not in official releases yet**. It is proposed here: <LINK TO YOUR PULL REQUEST>.
  Until it is merged, use a build of Xray-core with that patch.

## Build

```bash
git clone https://github.com/AndrK189100/xgeodata.git
cd xgeodata
go build -trimpath -ldflags "-s -w" -o xgeodata .
```

Cross-compile for Linux from Windows (PowerShell):

```powershell
$env:CGO_ENABLED = "0"; $env:GOOS = "linux"; $env:GOARCH = "amd64"
go build -trimpath -ldflags "-s -w" -o xgeodata .
Remove-Item Env:GOOS, Env:GOARCH, Env:CGO_ENABLED
```

## Usage

```
xgeodata [-in DIR] [-out DIR] [-api HOST:PORT] [-watch]
```

| Flag     | Default    | Description                                                        |
|----------|------------|--------------------------------------------------------------------|
| `-in`    | `./data`   | Input directory with `*.domain` and `*.cidr` files                 |
| `-out`   | `./output` | Output directory for `geoip.dat` and `geosite.dat`                 |
| `-api`   | empty      | Xray API address, e.g. `127.0.0.1:10085`. Empty = do not reload    |
| `-watch` | `false`    | Run as a daemon and regenerate on every change in the input dir    |

Examples:

```bash
# one-shot: generate only
xgeodata -in ./data -out ./output

# one-shot: generate and reload Xray
xgeodata -in /etc/xray/sets -out /etc/xray/sets -api 127.0.0.1:8888

# daemon
xgeodata -in /etc/xray/sets -out /etc/xray/sets -api 127.0.0.1:8888 -watch
```

The `.dat` files are generated at startup as well, so the first reload happens right after launch.

## Input files

Put one file per list into the input directory:

- `name.domain` goes to `geosite.dat` and is referenced in Xray as `geosite:name`
- `name.cidr` goes to `geoip.dat` and is referenced in Xray as `geoip:name`

Files with other extensions are ignored.

## Xray configuration

Xray must expose its API with `RoutingService` enabled. A minimal example:

```json
{
  "api": {
    "tag": "api",
    "listen": "127.0.0.1:8888",
    "services": [
      "ReflectionService",
      "RoutingService"
    ]
  }
}

```

**Security:** the Xray API has no authentication. Always bind it to `127.0.0.1` (or protect it with a tunnel/firewall). Anyone who can reach it can control routing.

Point the file locations Xray uses for geo data at the same directory you pass to `-out`, so that the reload picks up the regenerated files.

## Safety notes

- If a rule list that your Xray config references (for example `geoip:allowed_clients`) is missing from the generated `.dat`, a reload on a running Xray returns an error and the old rules stay active, but the next Xray **start** with such files will fail. Keep all referenced lists in the input directory.
- Reload errors are only logged; the daemon keeps running.
- A generation error stops the daemon (so that systemd reports it).

## Running as a systemd service

```ini
[Unit]
Description=xgeodata - geosite.dat/geoip.dat generator for Xray
After=network.target local-fs.target xray.service
Wants=xray.service
StartLimitIntervalSec=60
StartLimitBurst=5

[Service]
Type=simple
ExecStart=/usr/local/bin/xgeodata -in /etc/xray/sets -out /etc/xray/sets -api 127.0.0.1:8888 -watch
Restart=on-failure
RestartSec=5
NoNewPrivileges=true
ProtectSystem=strict
ReadWritePaths=/etc/xray/sets
ProtectHome=true
PrivateTmp=true
SyslogIdentifier=xgeodata

[Install]
WantedBy=multi-user.target
```

Adjust paths and the Xray unit name to your setup. Running as a dedicated unprivileged user (`User=`/`Group=`) with write access to the output directory is recommended.

```bash
systemctl daemon-reload
systemctl enable --now xgeodata
journalctl -u xgeodata -f
```
