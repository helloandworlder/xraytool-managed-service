package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"xraytool/internal/db"
	"xraytool/internal/service"

	"go.uber.org/zap"
)

func main() {
	dbPath := flag.String("db", "", "Path to sqlite database")
	orderID := flag.Uint("order", 0, "Order ID to export")
	outputPath := flag.String("output", "", "Output xlsx path")
	includeRaw := flag.Bool("include-raw", false, "Include raw socks5 column")
	flag.Parse()

	if *dbPath == "" || *orderID == 0 || *outputPath == "" {
		fmt.Fprintln(os.Stderr, "usage: codex-export-order -db <db> -order <id> -output <path> [-include-raw]")
		os.Exit(2)
	}

	database, err := db.Open(*dbPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	svc := service.NewOrderService(database, nil, zap.NewNop())
	body, filename, _, err := svc.ExportOrderArtifact(uint(*orderID), service.ExportOrderOptions{
		Count:   0,
		Shuffle: false,
	}, *includeRaw)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	if err := os.MkdirAll(filepath.Dir(*outputPath), 0o755); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if err := os.WriteFile(*outputPath, body, 0o644); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	fmt.Printf("written %s\n", *outputPath)
	fmt.Printf("filename %s\n", filename)
}
