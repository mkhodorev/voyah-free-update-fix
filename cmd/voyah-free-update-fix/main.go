package main

import (
	"bufio"
	"flag"
	"fmt"
	"os"
	"runtime"

	"voyah-free-update-fix/internal/app"

	_ "modernc.org/sqlite"
)

func main() {
	cfg := app.DefaultRuntimeConfig()

	flag.StringVar(&cfg.IPAddress, "ip", cfg.IPAddress, "T-Box IP address")
	flag.StringVar(&cfg.SSHPort, "port", cfg.SSHPort, "T-Box SSH port")
	flag.StringVar(&cfg.Username, "username", cfg.Username, "SSH username")
	flag.StringVar(&cfg.Password, "password", cfg.Password, "SSH password")
	flag.StringVar(&cfg.BackupDir, "backup-dir", cfg.BackupDir, "local directory for context backup archives")
	flag.Parse()

	if err := app.RunWithConfig(cfg); err != nil {
		fmt.Printf("%v\n", err)
		waitForExitOnWindows()

		os.Exit(1)
	}

	waitForExitOnWindows()
}

func waitForExitOnWindows() {
	if runtime.GOOS != "windows" {
		return
	}

	stdinInfo, err := os.Stdin.Stat()
	if err != nil {
		return
	}

	if (stdinInfo.Mode() & os.ModeCharDevice) == 0 {
		return
	}

	fmt.Print("Press any key to exit...")

	reader := bufio.NewReader(os.Stdin)
	_, _ = reader.ReadByte()
}
