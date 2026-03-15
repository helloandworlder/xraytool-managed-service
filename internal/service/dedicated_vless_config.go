package service

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"net/url"
	"strings"

	"xraytool/internal/model"

	"golang.org/x/crypto/curve25519"
)

const (
	dedicatedVlessSecurityNone    = "none"
	dedicatedVlessSecurityTLS     = "tls"
	dedicatedVlessSecurityReality = "reality"
	dedicatedVlessTypeTCP         = "tcp"
	defaultRealityFingerprint     = "chrome"
)

func normalizeDedicatedVlessInbound(row *model.DedicatedInbound) error {
	if row == nil {
		return nil
	}
	if !strings.EqualFold(strings.TrimSpace(row.Protocol), model.DedicatedFeatureVless) {
		clearDedicatedVlessInbound(row)
		return nil
	}

	security, err := normalizeDedicatedVlessSecurity(row.VlessSecurity)
	if err != nil {
		return err
	}
	vType, err := normalizeDedicatedVlessType(row.VlessType)
	if err != nil {
		return err
	}

	row.VlessSecurity = security
	row.VlessFlow = strings.TrimSpace(row.VlessFlow)
	row.VlessType = vType
	row.VlessSNI = strings.TrimSpace(row.VlessSNI)
	row.VlessHost = strings.TrimSpace(row.VlessHost)
	row.VlessPath = strings.TrimSpace(row.VlessPath)
	row.VlessFingerprint = strings.TrimSpace(row.VlessFingerprint)
	row.VlessTLSCertFile = strings.TrimSpace(row.VlessTLSCertFile)
	row.VlessTLSKeyFile = strings.TrimSpace(row.VlessTLSKeyFile)
	row.RealityTarget = strings.TrimSpace(row.RealityTarget)
	row.RealityServerNames = strings.Join(splitAndTrimNonEmpty(row.RealityServerNames), ",")
	row.RealityPrivateKey = strings.TrimSpace(row.RealityPrivateKey)
	shortIDs, err := normalizeRealityShortIDs(row.RealityShortIDs)
	if err != nil {
		return err
	}
	row.RealityShortIDs = strings.Join(shortIDs, ",")
	row.RealitySpiderX = strings.TrimSpace(row.RealitySpiderX)
	row.RealityMinClientVer = strings.TrimSpace(row.RealityMinClientVer)
	row.RealityMaxClientVer = strings.TrimSpace(row.RealityMaxClientVer)
	row.RealityMLDSA65Seed = strings.TrimSpace(row.RealityMLDSA65Seed)
	row.RealityMLDSA65Verify = strings.TrimSpace(row.RealityMLDSA65Verify)

	switch security {
	case dedicatedVlessSecurityTLS:
		if row.VlessTLSCertFile == "" || row.VlessTLSKeyFile == "" {
			return errors.New("vless tls requires cert_file and key_file")
		}
	case dedicatedVlessSecurityReality:
		if vType != dedicatedVlessTypeTCP {
			return errors.New("vless reality currently requires tcp transport")
		}
		if row.RealityTarget == "" {
			return errors.New("vless reality target is required")
		}
		if row.RealityPrivateKey == "" {
			privateKey, publicKey, genErr := GenerateRealityKeyPair()
			if genErr != nil {
				return genErr
			}
			row.RealityPrivateKey = privateKey
			row.RealityPublicKey = publicKey
		}
		if len(realityServerNames(row)) == 0 {
			return errors.New("vless reality requires sni or server_names")
		}
		if row.VlessFingerprint == "" {
			row.VlessFingerprint = defaultRealityFingerprint
		}
	}

	fillDedicatedInboundDerivedFields(row)
	if security == dedicatedVlessSecurityReality && row.RealityPublicKey == "" {
		return errors.New("vless reality private_key invalid")
	}
	return nil
}

func GenerateRealityKeyPair() (string, string, error) {
	privateKey := make([]byte, 32)
	if _, err := rand.Read(privateKey); err != nil {
		return "", "", err
	}
	private := base64.RawURLEncoding.EncodeToString(privateKey)
	public, err := deriveRealityPublicKey(private)
	if err != nil {
		return "", "", err
	}
	return private, public, nil
}

func ValidateDedicatedInboundInput(in DedicatedInboundInput) (*model.DedicatedInbound, error) {
	row, err := normalizeDedicatedInboundInput(in)
	if err != nil {
		return nil, err
	}
	fillDedicatedInboundDerivedFields(&row)
	return &row, nil
}

func clearDedicatedVlessInbound(row *model.DedicatedInbound) {
	row.VlessSecurity = ""
	row.VlessFlow = ""
	row.VlessType = ""
	row.VlessSNI = ""
	row.VlessHost = ""
	row.VlessPath = ""
	row.VlessFingerprint = ""
	row.VlessTLSCertFile = ""
	row.VlessTLSKeyFile = ""
	row.RealityShow = false
	row.RealityTarget = ""
	row.RealityServerNames = ""
	row.RealityPrivateKey = ""
	row.RealityPublicKey = ""
	row.RealityShortIDs = ""
	row.RealitySpiderX = ""
	row.RealityXver = 0
	row.RealityMaxTimeDiff = 0
	row.RealityMinClientVer = ""
	row.RealityMaxClientVer = ""
	row.RealityMLDSA65Seed = ""
	row.RealityMLDSA65Verify = ""
}

func normalizeDedicatedVlessSecurity(raw string) (string, error) {
	v := strings.ToLower(strings.TrimSpace(raw))
	if v == "" {
		v = dedicatedVlessSecurityNone
	}
	switch v {
	case dedicatedVlessSecurityNone, dedicatedVlessSecurityTLS, dedicatedVlessSecurityReality:
		return v, nil
	default:
		return "", fmt.Errorf("unsupported vless security %s", raw)
	}
}

func normalizeDedicatedVlessType(raw string) (string, error) {
	v := strings.ToLower(strings.TrimSpace(raw))
	if v == "" || v == "raw" {
		v = dedicatedVlessTypeTCP
	}
	switch v {
	case dedicatedVlessTypeTCP, "ws", "grpc", "httpupgrade", "xhttp":
		return v, nil
	default:
		return "", fmt.Errorf("unsupported vless type %s", raw)
	}
}

func fillDedicatedInboundDerivedFields(row *model.DedicatedInbound) {
	if row == nil {
		return
	}
	row.RealityPublicKey = ""
	if !strings.EqualFold(strings.TrimSpace(row.Protocol), model.DedicatedFeatureVless) {
		return
	}
	if !strings.EqualFold(strings.TrimSpace(row.VlessSecurity), dedicatedVlessSecurityReality) {
		return
	}
	pub, err := deriveRealityPublicKey(row.RealityPrivateKey)
	if err != nil {
		return
	}
	row.RealityPublicKey = pub
}

func fillDedicatedInboundDerivedFieldsForList(rows []model.DedicatedInbound) {
	for idx := range rows {
		fillDedicatedInboundDerivedFields(&rows[idx])
	}
}

func deriveRealityPublicKey(privateKey string) (string, error) {
	privateKey = strings.TrimSpace(privateKey)
	if privateKey == "" {
		return "", nil
	}
	decoded, err := decodeRealityKey(privateKey)
	if err != nil {
		return "", err
	}
	if len(decoded) != 32 {
		return "", errors.New("reality key must decode to 32 bytes")
	}
	publicKey, err := curve25519.X25519(decoded, curve25519.Basepoint)
	if err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(publicKey), nil
}

func decodeRealityKey(raw string) ([]byte, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, nil
	}
	encodings := []*base64.Encoding{
		base64.RawURLEncoding,
		base64.URLEncoding,
		base64.RawStdEncoding,
		base64.StdEncoding,
	}
	for _, enc := range encodings {
		decoded, err := enc.DecodeString(raw)
		if err == nil {
			return decoded, nil
		}
	}
	return nil, errors.New("invalid reality key encoding")
}

func normalizeRealityShortIDs(raw string) ([]string, error) {
	items := splitAndTrimNonEmpty(raw)
	if len(items) == 0 {
		return nil, nil
	}
	out := make([]string, 0, len(items))
	for _, item := range items {
		item = strings.ToLower(strings.TrimSpace(item))
		if item == "" {
			continue
		}
		if len(item)%2 != 0 || len(item) > 16 {
			return nil, fmt.Errorf("invalid reality short id %q", item)
		}
		if _, err := hex.DecodeString(item); err != nil {
			return nil, fmt.Errorf("invalid reality short id %q", item)
		}
		out = append(out, item)
	}
	return out, nil
}

func splitAndTrimNonEmpty(raw string) []string {
	raw = strings.ReplaceAll(raw, "\n", ",")
	raw = strings.ReplaceAll(raw, "\r", ",")
	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		out = append(out, part)
	}
	return out
}

func realityServerNames(row *model.DedicatedInbound) []string {
	if row == nil {
		return nil
	}
	names := splitAndTrimNonEmpty(row.RealityServerNames)
	if len(names) > 0 {
		return names
	}
	if sni := strings.TrimSpace(row.VlessSNI); sni != "" {
		return []string{sni}
	}
	return nil
}

func realityShortIDs(row *model.DedicatedInbound) []string {
	if row == nil {
		return []string{""}
	}
	ids, err := normalizeRealityShortIDs(row.RealityShortIDs)
	if err != nil {
		return []string{""}
	}
	if len(ids) == 0 {
		return []string{""}
	}
	return ids
}

func primaryRealityShortID(row *model.DedicatedInbound) string {
	for _, item := range realityShortIDs(row) {
		item = strings.TrimSpace(item)
		if item != "" {
			return item
		}
	}
	return ""
}

func shareVlessSNI(row *model.DedicatedInbound, host string) string {
	if row != nil {
		if sni := strings.TrimSpace(row.VlessSNI); sni != "" {
			return sni
		}
		if names := realityServerNames(row); len(names) > 0 {
			return names[0]
		}
	}
	return strings.TrimSpace(host)
}

func shareVlessType(row *model.DedicatedInbound) string {
	if row == nil {
		return dedicatedVlessTypeTCP
	}
	vType, err := normalizeDedicatedVlessType(row.VlessType)
	if err != nil {
		return dedicatedVlessTypeTCP
	}
	return vType
}

func buildVlessStreamSettings(row *model.DedicatedInbound) (map[string]any, error) {
	if row == nil {
		return map[string]any{"network": dedicatedVlessTypeTCP, "security": dedicatedVlessSecurityNone}, nil
	}
	copyRow := *row
	if err := normalizeDedicatedVlessInbound(&copyRow); err != nil {
		return nil, err
	}
	stream := map[string]any{
		"network":  shareVlessType(&copyRow),
		"security": copyRow.VlessSecurity,
	}
	switch shareVlessType(&copyRow) {
	case "ws":
		ws := map[string]any{"path": defaultVlessPath(copyRow.VlessPath)}
		if copyRow.VlessHost != "" {
			ws["headers"] = map[string]any{"Host": copyRow.VlessHost}
		}
		stream["wsSettings"] = ws
	case "grpc":
		grpc := map[string]any{}
		if copyRow.VlessPath != "" {
			grpc["serviceName"] = copyRow.VlessPath
		}
		stream["grpcSettings"] = grpc
	case "httpupgrade":
		httpUpgrade := map[string]any{"path": defaultVlessPath(copyRow.VlessPath)}
		if copyRow.VlessHost != "" {
			httpUpgrade["host"] = copyRow.VlessHost
		}
		stream["httpupgradeSettings"] = httpUpgrade
	case "xhttp":
		xhttp := map[string]any{"path": defaultVlessPath(copyRow.VlessPath)}
		if copyRow.VlessHost != "" {
			xhttp["host"] = copyRow.VlessHost
		}
		stream["xhttpSettings"] = xhttp
	}
	if copyRow.VlessSecurity == dedicatedVlessSecurityTLS {
		stream["tlsSettings"] = map[string]any{
			"certificates": []map[string]any{{
				"certificateFile": copyRow.VlessTLSCertFile,
				"keyFile":         copyRow.VlessTLSKeyFile,
			}},
		}
	}
	if copyRow.VlessSecurity == dedicatedVlessSecurityReality {
		reality := map[string]any{
			"show":          copyRow.RealityShow,
			"target":        copyRow.RealityTarget,
			"xver":          copyRow.RealityXver,
			"serverNames":   realityServerNames(&copyRow),
			"privateKey":    copyRow.RealityPrivateKey,
			"shortIds":      realityShortIDs(&copyRow),
			"maxTimeDiff":   copyRow.RealityMaxTimeDiff,
			"minClientVer":  copyRow.RealityMinClientVer,
			"maxClientVer":  copyRow.RealityMaxClientVer,
			"spiderX":       copyRow.RealitySpiderX,
			"mldsa65Seed":   copyRow.RealityMLDSA65Seed,
			"mldsa65Verify": copyRow.RealityMLDSA65Verify,
		}
		stream["realitySettings"] = reality
	}
	return stream, nil
}

func defaultVlessPath(path string) string {
	path = strings.TrimSpace(path)
	if path == "" {
		return "/"
	}
	return path
}

func realityFingerprint(row *model.DedicatedInbound) string {
	if row == nil {
		return defaultRealityFingerprint
	}
	if fp := strings.TrimSpace(row.VlessFingerprint); fp != "" {
		return fp
	}
	return defaultRealityFingerprint
}

func appendVlessLinkParams(params url.Values, row *model.DedicatedInbound, host string) {
	if params == nil {
		return
	}
	params.Set("encryption", "none")
	vType := shareVlessType(row)
	if vType != dedicatedVlessTypeTCP {
		params.Set("type", vType)
	}
	security := dedicatedVlessSecurityNone
	if row != nil {
		security = strings.TrimSpace(row.VlessSecurity)
		if security == "" {
			security = dedicatedVlessSecurityNone
		}
	}
	if security != dedicatedVlessSecurityNone {
		params.Set("security", security)
		sni := shareVlessSNI(row, host)
		if sni != "" {
			params.Set("sni", sni)
		}
		if row != nil {
			if fp := strings.TrimSpace(row.VlessFingerprint); fp != "" {
				params.Set("fp", fp)
			}
		}
	}
	if row != nil {
		if flow := strings.TrimSpace(row.VlessFlow); flow != "" {
			params.Set("flow", flow)
		}
		switch vType {
		case "ws", "httpupgrade", "xhttp":
			if hostHeader := strings.TrimSpace(row.VlessHost); hostHeader != "" {
				params.Set("host", hostHeader)
			}
			params.Set("path", defaultVlessPath(row.VlessPath))
		case "grpc":
			if strings.TrimSpace(row.VlessPath) != "" {
				params.Set("serviceName", strings.TrimSpace(row.VlessPath))
			}
		}
		if security == dedicatedVlessSecurityReality {
			if row.RealityPublicKey != "" {
				params.Set("pbk", row.RealityPublicKey)
			}
			if sid := primaryRealityShortID(row); sid != "" {
				params.Set("sid", sid)
			}
			if spx := strings.TrimSpace(row.RealitySpiderX); spx != "" {
				params.Set("spx", spx)
			}
			if pqv := strings.TrimSpace(row.RealityMLDSA65Verify); pqv != "" {
				params.Set("pqv", pqv)
			}
			if params.Get("fp") == "" {
				params.Set("fp", realityFingerprint(row))
			}
		}
	}
}

func FillDedicatedInboundDerivedFieldsForShare(row *model.DedicatedInbound) {
	if row == nil {
		return
	}
	fillDedicatedInboundDerivedFields(row)
}

func AppendVlessLinkParamsForShare(params url.Values, row *model.DedicatedInbound, host string) {
	appendVlessLinkParams(params, row, host)
}
