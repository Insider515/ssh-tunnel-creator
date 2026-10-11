package ui

import (
	"os"
	"path/filepath"
)

// sshDir возвращает каталог ~/.ssh, если он существует.
// Используется как начальный каталог в диалоге выбора файла приватного ключа.
func sshDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	dir := filepath.Join(home, ".ssh")
	if info, err := os.Stat(dir); err != nil || !info.IsDir() {
		return ""
	}
	return dir
}
