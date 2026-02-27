package service

import (
	"bufio"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"xraytool/internal/model"
)

const DedicatedShadowsocksMethod = "chacha20-ietf-poly1305"

var dedicatedFeatureOrder = []string{
	model.DedicatedFeatureMixed,
	model.DedicatedFeatureVmess,
	model.DedicatedFeatureVless,
	model.DedicatedFeatureShadowsocks,
}

func normalizeDedicatedProtocol(raw string) (string, error) {
	v := strings.ToLower(strings.TrimSpace(raw))
	if v == "socks5" {
		v = model.DedicatedFeatureMixed
	}
	switch v {
	case model.DedicatedFeatureMixed, model.DedicatedFeatureVmess, model.DedicatedFeatureVless, model.DedicatedFeatureShadowsocks:
		return v, nil
	default:
		return "", fmt.Errorf("unsupported protocol %s", raw)
	}
}

func dedicatedPortByProtocol(entry model.DedicatedEntry, protocol string) int {
	protocol = strings.ToLower(strings.TrimSpace(protocol))
	if protocol == model.DedicatedFeatureMixed {
		return entry.MixedPort
	}
	if protocol == model.DedicatedFeatureVmess {
		return entry.VmessPort
	}
	if protocol == model.DedicatedFeatureVless {
		return entry.VlessPort
	}
	if protocol == model.DedicatedFeatureShadowsocks {
		return entry.ShadowsocksPort
	}
	return 0
}

func normalizeDedicatedFeatures(raw []string) ([]string, error) {
	if len(raw) == 0 {
		return nil, errors.New("features is required")
	}
	seen := map[string]struct{}{}
	for _, row := range raw {
		v := strings.ToLower(strings.TrimSpace(row))
		if v == "" {
			continue
		}
		switch v {
		case model.DedicatedFeatureMixed, model.DedicatedFeatureVmess, model.DedicatedFeatureVless, model.DedicatedFeatureShadowsocks:
			seen[v] = struct{}{}
		default:
			return nil, fmt.Errorf("unsupported feature %s", v)
		}
	}
	if len(seen) == 0 {
		return nil, errors.New("features is required")
	}
	out := make([]string, 0, len(seen))
	for _, key := range dedicatedFeatureOrder {
		if _, ok := seen[key]; ok {
			out = append(out, key)
		}
	}
	return out, nil
}

func parseDedicatedFeatures(raw string) map[string]struct{} {
	out := map[string]struct{}{}
	parts := strings.Split(raw, ",")
	for _, part := range parts {
		v := strings.ToLower(strings.TrimSpace(part))
		if v == "" {
			continue
		}
		out[v] = struct{}{}
	}
	return out
}

func joinDedicatedFeatures(features []string) string {
	if len(features) == 0 {
		return ""
	}
	parts := make([]string, len(features))
	copy(parts, features)
	sort.Strings(parts)
	return strings.Join(parts, ",")
}

func hasDedicatedFeature(raw string, feature string) bool {
	_, ok := parseDedicatedFeatures(raw)[strings.ToLower(strings.TrimSpace(feature))]
	return ok
}

type DedicatedEgressLine struct {
	Address  string
	Port     int
	Username string
	Password string
}

type DedicatedEgressGeoLine struct {
	DedicatedEgressLine
	CountryCode string
	Region      string
}

func parseDedicatedEgressLines(lines string) ([]DedicatedEgressLine, error) {
	scanner := bufio.NewScanner(strings.NewReader(lines))
	out := make([]DedicatedEgressLine, 0)
	for scanner.Scan() {
		raw := strings.TrimSpace(scanner.Text())
		if raw == "" {
			continue
		}
		row, err := parseDedicatedEgressLine(raw)
		if err != nil {
			return nil, err
		}
		out = append(out, row)
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	if len(out) == 0 {
		return nil, errors.New("no valid socks5 lines")
	}
	return out, nil
}

func parseDedicatedEgressGeoLines(lines string, defaultCountryCode string, defaultRegion string) ([]DedicatedEgressGeoLine, error) {
	scanner := bufio.NewScanner(strings.NewReader(lines))
	out := make([]DedicatedEgressGeoLine, 0)
	defaultCountryCode = strings.ToLower(strings.TrimSpace(defaultCountryCode))
	defaultRegion = strings.TrimSpace(defaultRegion)
	for scanner.Scan() {
		raw := strings.TrimSpace(scanner.Text())
		if raw == "" {
			continue
		}
		core := raw
		country := defaultCountryCode
		region := defaultRegion
		if strings.Contains(raw, "|") {
			parts := strings.Split(raw, "|")
			if len(parts) < 2 || len(parts) > 3 {
				return nil, fmt.Errorf("invalid geo mapping line %q, expect ip:port:user:pass|country|region", raw)
			}
			core = strings.TrimSpace(parts[0])
			country = strings.ToLower(strings.TrimSpace(parts[1]))
			if len(parts) == 3 {
				region = strings.TrimSpace(parts[2])
			} else {
				region = ""
			}
		}
		if strings.Contains(core, ",") {
			parts := strings.Split(core, ",")
			if len(parts) == 6 {
				core = strings.Join(parts[:4], ":")
				country = strings.ToLower(strings.TrimSpace(parts[4]))
				region = strings.TrimSpace(parts[5])
			}
		}
		line, err := parseDedicatedEgressLine(core)
		if err != nil {
			return nil, err
		}
		if country == "" {
			return nil, fmt.Errorf("invalid geo mapping line %q, country code required", raw)
		}
		out = append(out, DedicatedEgressGeoLine{
			DedicatedEgressLine: line,
			CountryCode:         country,
			Region:              region,
		})
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	if len(out) == 0 {
		return nil, errors.New("no valid geo mapping lines")
	}
	return out, nil
}

func parseDedicatedEgressLine(raw string) (DedicatedEgressLine, error) {
	parts := strings.Split(raw, ":")
	if len(parts) != 4 {
		return DedicatedEgressLine{}, fmt.Errorf("invalid line %q, expect ip:port:user:pass", raw)
	}
	port, err := strconv.Atoi(strings.TrimSpace(parts[1]))
	if err != nil || port <= 0 || port > 65535 {
		return DedicatedEgressLine{}, fmt.Errorf("invalid port in line %q", raw)
	}
	row := DedicatedEgressLine{
		Address:  strings.TrimLeft(strings.TrimSpace(parts[0]), "~"),
		Port:     port,
		Username: strings.TrimSpace(parts[2]),
		Password: strings.TrimSpace(parts[3]),
	}
	if row.Address == "" || row.Username == "" || row.Password == "" {
		return DedicatedEgressLine{}, fmt.Errorf("invalid line %q, address/user/pass required", raw)
	}
	return row, nil
}

type DedicatedCredentialLine struct {
	Username string
	Password string
	UUID     string
}

func parseDedicatedCredentialLines(lines string) ([]DedicatedCredentialLine, error) {
	scanner := bufio.NewScanner(strings.NewReader(lines))
	out := make([]DedicatedCredentialLine, 0)
	for scanner.Scan() {
		raw := strings.TrimSpace(scanner.Text())
		if raw == "" {
			continue
		}
		parts := strings.Split(raw, ":")
		if len(parts) != 2 && len(parts) != 3 {
			return nil, fmt.Errorf("invalid credential line %q, expect user:pass[:uuid]", raw)
		}
		row := DedicatedCredentialLine{
			Username: strings.TrimSpace(parts[0]),
			Password: strings.TrimSpace(parts[1]),
		}
		if len(parts) == 3 {
			row.UUID = strings.TrimSpace(parts[2])
		}
		if row.Username == "" || row.Password == "" {
			return nil, fmt.Errorf("invalid credential line %q, username/password required", raw)
		}
		if row.UUID == "" {
			row.UUID = randomUUID()
		}
		out = append(out, row)
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	if len(out) == 0 {
		return nil, errors.New("no valid credential lines")
	}
	return out, nil
}

func parseDedicatedCredentialLinesForProtocol(lines string, protocol string) ([]DedicatedCredentialLine, error) {
	protocol, err := normalizeDedicatedProtocol(protocol)
	if err != nil {
		return nil, err
	}
	if protocol == model.DedicatedFeatureMixed {
		return parseDedicatedCredentialLines(lines)
	}
	scanner := bufio.NewScanner(strings.NewReader(lines))
	out := make([]DedicatedCredentialLine, 0)
	for scanner.Scan() {
		raw := strings.TrimSpace(scanner.Text())
		if raw == "" {
			continue
		}
		if protocol == model.DedicatedFeatureVmess || protocol == model.DedicatedFeatureVless {
			out = append(out, DedicatedCredentialLine{UUID: raw})
			continue
		}
		out = append(out, DedicatedCredentialLine{Password: raw})
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	if len(out) == 0 {
		return nil, errors.New("no valid credential lines")
	}
	return out, nil
}

func chooseDedicatedPrimaryPort(entry model.DedicatedEntry) int {
	features := parseDedicatedFeatures(entry.Features)
	if _, ok := features[model.DedicatedFeatureMixed]; ok && entry.MixedPort > 0 {
		return entry.MixedPort
	}
	if _, ok := features[model.DedicatedFeatureVmess]; ok && entry.VmessPort > 0 {
		return entry.VmessPort
	}
	if _, ok := features[model.DedicatedFeatureVless]; ok && entry.VlessPort > 0 {
		return entry.VlessPort
	}
	if _, ok := features[model.DedicatedFeatureShadowsocks]; ok && entry.ShadowsocksPort > 0 {
		return entry.ShadowsocksPort
	}
	if entry.MixedPort > 0 {
		return entry.MixedPort
	}
	if entry.VmessPort > 0 {
		return entry.VmessPort
	}
	if entry.VlessPort > 0 {
		return entry.VlessPort
	}
	if entry.ShadowsocksPort > 0 {
		return entry.ShadowsocksPort
	}
	return 0
}
