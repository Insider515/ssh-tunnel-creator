// Command sshtunnel launches the graphical application for creating a TCP tunnel
// through an intermediate (jump) SSH server.
package main

import "sshtunnel/internal/ui"

// main starts the application window.
func main() {
	ui.Run()
}
