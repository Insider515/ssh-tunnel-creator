# SSH Tunnel

![SSH Tunnel](assets/screenshot.png)

A desktop application for creating a TCP tunnel through an intermediate (jump) SSH server. The jump server connects to `target_host:target_port`; local applications connect to `127.0.0.1:<local port>`.

The application is written in Go with Fyne. It supports macOS (Apple Silicon and Intel), Windows x64, and Linux x64.

## Features

- Password, private-key, and `ssh-agent` authentication.
- Connection checks for jump-server TCP reachability, SSH authentication, host-key verification, target reachability through the jump server, and local-port availability.
- Jump-server host-key verification using the application's known-hosts file and `~/.ssh/known_hosts`. A new key requires confirmation; a changed saved key is rejected.
- Keepalive, connection-loss detection, and optional automatic reconnect.
- Connection profiles. Passwords are not stored in profile files; optional password storage uses the operating system credential store.
- An in-window log with status, active and total connection counts, copy, export, and clear controls.
- English and Ukrainian interface languages; English is selected on first launch.
- Optional startup at login and automatic connection to a selected default profile.
- On launch, the configured default profile is selected if it exists; otherwise the first profile in alphabetical order is selected. Selection alone does not start a connection.

## Build and run

Requires Go 1.26 or newer and a C compiler for the current platform (Fyne uses CGO).

```bash
make all        # static analysis, tests, and build for the current OS
make run        # launch from source
make test       # tests with the race detector
make tools      # install Fyne packaging tools
make mac        # universal macOS binary and app bundle
make mac-dmg    # macOS disk image with first-launch instructions
make windows    # Windows x64 executable
make linux      # Linux x64 package using Docker and fyne-cross
make icon       # regenerate assets/icon.png
```

For macOS, `make mac` requires Xcode Command Line Tools. The app bundle is unsigned. The disk image includes `macos/FIRST-LAUNCH.txt`, which explains how to open the app using macOS's standard security prompt.

The Windows cross-build from macOS requires `mingw-w64`. The Linux cross-build requires Docker and `fyne-cross`; on Ubuntu itself, build with `go build ./cmd/sshtunnel` after installing `libgl1-mesa-dev xorg-dev libxkbcommon-dev`.

## Use

1. Create a profile and set its name and local listening port.
2. Enter the jump-server address, port, username, and authentication method.
3. Enter the target-server address and port. This is the address the jump server must be able to reach.
4. Configure host-key verification, automatic reconnect, and keepalive.
5. Save the profile. Use **Check connection** to test the route without leaving a tunnel running, then choose **Connect** to start it.
6. Connect local applications to `127.0.0.1:<local port>`. Use **Stop** to close the listener and active tunnel connections.
7. In application settings, choose English or Ukrainian, optionally enable startup at login, and select a default profile.
8. Enable automatic connection separately if the selected default profile should connect when the application starts. A default profile is required for this option.

The listener binds to `127.0.0.1` and is not exposed to the local network. If automatic reconnect is enabled, the local listener remains open while the application retries the SSH connection.

### Authentication and host keys

- **Password** sends the entered password through SSH password or keyboard-interactive authentication.
- **Private key** reads the selected key file; an encrypted key also requires its passphrase.
- **ssh-agent** uses keys available from the running agent.
- When host-key verification is enabled, known keys are read from the application's `known_hosts` file and, if present, `~/.ssh/known_hosts`. A previously unseen key is added to the application's file only after confirmation. The user's `~/.ssh/known_hosts` file is not modified.
- Disabling host-key verification permits server impersonation and is not recommended.
- Passwords and key passphrases are not written to profile files. If password remembering is enabled, the password is stored in the operating system credential store.

### Logs

The main window shows the selected profile's log. **Copy** places the displayed log in the clipboard, **Export** saves it as a text file, and **Clear** removes the in-memory entries. Up to 1,000 recent log lines are retained during a run.

## Data locations

The application uses the operating system's user configuration directory and stores its files in the `sshtunnel` subdirectory:

- `profiles.json` — connection profiles and non-secret connection options.
- `settings.json` — interface language, startup-at-login option, and default-profile settings.
- `known_hosts` — jump-server keys accepted by the user.

Remembered passwords are stored separately in the operating system credential store. Private-key files remain at their original paths. The user's `~/.ssh/known_hosts` file may be read for verification but is not modified.

## Project layout

```text
cmd/sshtunnel/        application entry point
internal/tunnel/      SSH tunnel and connection lifecycle
internal/config/      profiles, preferences, and credential-store access
internal/i18n/        English and Ukrainian translations
internal/startup/     per-user startup integration
internal/ui/          Fyne interface
tools/genicon/        icon generator
macos/                macOS first-launch instructions included in the DMG
```
