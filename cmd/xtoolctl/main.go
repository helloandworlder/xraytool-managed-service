package main

import (
	"bufio"
	"flag"
	"fmt"
	"os"
	"strings"

	"xraytool/internal/auth"
	"xraytool/internal/config"
	"xraytool/internal/db"
	"xraytool/internal/store"
)

func main() {
	mode := flag.String("mode", "reset-admin", "operation mode")
	username := flag.String("username", "", "admin username")
	password := flag.String("password", "", "admin password")
	flag.Parse()

	if *mode != "reset-admin" {
		fmt.Println("supported mode: reset-admin")
		os.Exit(1)
	}

	cfg := config.Load()
	if err := config.EnsurePaths(cfg); err != nil {
		fmt.Println("ensure path failed:", err)
		os.Exit(1)
	}
	database, err := db.Open(cfg.DBPath)
	if err != nil {
		fmt.Println("open db failed:", err)
		os.Exit(1)
	}
	st := store.New(database)

	reader := bufio.NewReader(os.Stdin)
	name := strings.TrimSpace(*username)
	pass := strings.TrimSpace(*password)

	if name == "" {
		fmt.Print("Admin username: ")
		text, _ := reader.ReadString('\n')
		name = strings.TrimSpace(text)
	}
	if name == "" {
		name = "admin"
	}
	if pass == "" {
		fmt.Print("New password (>=8): ")
		text, _ := reader.ReadString('\n')
		pass = strings.TrimSpace(text)
	}
	if len(pass) < 8 {
		fmt.Println("password too short")
		os.Exit(1)
	}

	hash, err := auth.HashPassword(pass)
	if err != nil {
		fmt.Println("hash password failed:", err)
		os.Exit(1)
	}
	if err := st.ResetAdminPassword(name, hash); err != nil {
		fmt.Println("reset password failed:", err)
		os.Exit(1)
	}
	fmt.Println("reset admin password success")
}
