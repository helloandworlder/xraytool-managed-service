package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"sort"
	"sync"
	"time"

	"xraytool/internal/service"
)

type sampleRow struct {
	OrderID     uint   `json:"order_id"`
	SequenceNo  int    `json:"sequence_no"`
	OrderName   string `json:"order_name"`
	IngressHost string `json:"ingress_host"`
	IngressPort int    `json:"ingress_port"`
	Username    string `json:"username"`
	Password    string `json:"password"`
	VmessUUID   string `json:"vmess_uuid"`
}

type sampleResult struct {
	sampleRow
	CheckedAt      string `json:"checked_at"`
	ConnectivityOK bool   `json:"connectivity_ok"`
	ExitIP         string `json:"exit_ip,omitempty"`
	CountryCode    string `json:"country_code,omitempty"`
	Region         string `json:"region,omitempty"`
	Message        string `json:"message,omitempty"`
	ErrorCode      string `json:"error_code,omitempty"`
}

func main() {
	inputPath := flag.String("input", "", "Path to JSON input rows")
	outputPath := flag.String("output", "", "Optional path to JSON output")
	workers := flag.Int("workers", 4, "Concurrent probe workers")
	timeout := flag.Duration("timeout", 15*time.Second, "Per-probe timeout")
	flag.Parse()

	if *inputPath == "" {
		fmt.Fprintln(os.Stderr, "missing -input")
		os.Exit(2)
	}

	rows, err := readRows(*inputPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if len(rows) == 0 {
		fmt.Fprintln(os.Stderr, "input is empty")
		os.Exit(1)
	}

	if *workers <= 0 {
		*workers = 1
	}
	if *workers > len(rows) {
		*workers = len(rows)
	}

	jobs := make(chan sampleRow)
	results := make(chan sampleResult, len(rows))
	var wg sync.WaitGroup

	for i := 0; i < *workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for row := range jobs {
				ctx, cancel := context.WithTimeout(context.Background(), *timeout)
				probe := service.ProbeDedicatedWithXrayCore(ctx, service.DedicatedProtocolProbeRequest{
					RouteType: "SHORT_VIDEO",
					Protocol:  "VMESS",
					IP:        row.IngressHost,
					Port:      row.IngressPort,
					Username:  row.Username,
					Password:  row.Password,
					VmessUUID: row.VmessUUID,
				})
				cancel()
				results <- sampleResult{
					sampleRow:      row,
					CheckedAt:      time.Now().UTC().Format(time.RFC3339),
					ConnectivityOK: probe.ConnectivityOK,
					ExitIP:         probe.ExitIP,
					CountryCode:    probe.CountryCode,
					Region:         probe.Region,
					Message:        probe.Message,
					ErrorCode:      probe.ErrorCode,
				}
			}
		}()
	}

	go func() {
		for _, row := range rows {
			jobs <- row
		}
		close(jobs)
		wg.Wait()
		close(results)
	}()

	collected := make([]sampleResult, 0, len(rows))
	for row := range results {
		collected = append(collected, row)
	}

	sort.Slice(collected, func(i, j int) bool {
		if collected[i].SequenceNo == collected[j].SequenceNo {
			return collected[i].OrderID < collected[j].OrderID
		}
		return collected[i].SequenceNo < collected[j].SequenceNo
	})

	payload, err := json.MarshalIndent(collected, "", "  ")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	if *outputPath != "" {
		if err := os.WriteFile(*outputPath, payload, 0o644); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
	}

	fmt.Println(string(payload))
}

func readRows(path string) ([]sampleRow, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	rows := make([]sampleRow, 0)
	if err := json.Unmarshal(data, &rows); err != nil {
		return nil, err
	}
	return rows, nil
}
