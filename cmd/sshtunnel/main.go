// Команда sshtunnel — графическое приложение для создания TCP-туннеля
// через промежуточный (jump) SSH-сервер.
package main

import "sshtunnel/internal/ui"

// main запускает окно приложения.
func main() {
	ui.Run()
}
