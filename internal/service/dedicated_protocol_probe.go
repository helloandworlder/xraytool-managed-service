package service

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	core "github.com/xtls/xray-core/core"
	_ "github.com/xtls/xray-core/main/distro/all"
	"golang.org/x/net/proxy"
)

type DedicatedProtocolProbeRequest struct {
	RouteType string
	Protocol  string
	IP        string
	Port      int
	Username  string
	Password  string
	VmessUUID string
}

type DedicatedProtocolProbeResult struct {
	ConnectivityOK bool
	ExitIP         string
	CountryCode    string
	Region         string
	Message        string
	ErrorCode      string
}

var uuidRegex = regexp.MustCompile(`(?i)^[0-9a-f]{8}-[0-9a-f]{4}-[1-5][0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`)
var errDedicatedUUIDRequired = errors.New("uuid is required for vmess/vless protocol probe")

func ProbeDedicatedWithXrayCore(ctx context.Context, req DedicatedProtocolProbeRequest) DedicatedProtocolProbeResult {
	localPort, err := reserveLocalTCPPort()
	if err != nil {
		return DedicatedProtocolProbeResult{
			ConnectivityOK: false,
			Message:        fmt.Sprintf("reserve local probe port failed: %v", err),
			ErrorCode:      "LOCAL_BIND_FAILED",
		}
	}

	outbound, err := buildDedicatedProbeOutbound(req)
	if err != nil {
		if errors.Is(err, errDedicatedUUIDRequired) {
			return DedicatedProtocolProbeResult{
				ConnectivityOK: false,
				Message:        err.Error(),
				ErrorCode:      "UUID_REQUIRED",
			}
		}
		return DedicatedProtocolProbeResult{
			ConnectivityOK: false,
			Message:        err.Error(),
			ErrorCode:      "PROBE_CONFIG_INVALID",
		}
	}

	res, err := runProbeCore(ctx, localPort, outbound)
	if err != nil {
		return DedicatedProtocolProbeResult{
			ConnectivityOK: false,
			Message:        fmt.Sprintf("start probe runtime failed: %v", err),
			ErrorCode:      "PROTOCOL_HANDSHAKE_FAILED",
		}
	}
	return res
}

func reserveLocalTCPPort() (int, error) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return 0, err
	}
	defer ln.Close()
	addr, ok := ln.Addr().(*net.TCPAddr)
	if !ok || addr.Port <= 0 {
		return 0, errors.New("invalid tcp listener address")
	}
	return addr.Port, nil
}

func runProbeCore(ctx context.Context, localPort int, outbound map[string]any) (DedicatedProtocolProbeResult, error) {
	payload := map[string]any{
		"log": map[string]any{
			"loglevel": "warning",
		},
		"inbounds": []map[string]any{
			{
				"tag":      "probe-in",
				"listen":   "127.0.0.1",
				"port":     localPort,
				"protocol": "socks",
				"settings": map[string]any{
					"auth": "noauth",
					"udp":  false,
				},
			},
		},
		"outbounds": []map[string]any{
			outbound,
		},
		"routing": map[string]any{
			"domainStrategy": "AsIs",
			"rules": []map[string]any{
				{
					"type":        "field",
					"inboundTag":  []string{"probe-in"},
					"outboundTag": "probe-out",
				},
			},
		},
	}

	confObj, err := decodeConfig(payload)
	if err != nil {
		return DedicatedProtocolProbeResult{}, err
	}
	coreCfg, err := confObj.Build()
	if err != nil {
		return DedicatedProtocolProbeResult{}, err
	}
	instance, err := core.New(coreCfg)
	if err != nil {
		return DedicatedProtocolProbeResult{}, err
	}
	defer instance.Close()

	if err := instance.Start(); err != nil {
		return DedicatedProtocolProbeResult{}, err
	}

	probeCtx, cancel := context.WithTimeout(ctx, 6*time.Second)
	defer cancel()

	for {
		select {
		case <-probeCtx.Done():
			return DedicatedProtocolProbeResult{}, probeCtx.Err()
		default:
			conn, dialErr := (&net.Dialer{Timeout: 300 * time.Millisecond}).DialContext(probeCtx, "tcp", net.JoinHostPort("127.0.0.1", strconv.Itoa(localPort)))
			if dialErr == nil {
				_ = conn.Close()
				return probeExitViaLocalSocks(ctx, localPort), nil
			}
			time.Sleep(80 * time.Millisecond)
		}
	}
}

func buildDedicatedProbeOutbound(req DedicatedProtocolProbeRequest) (map[string]any, error) {
	ip := strings.TrimSpace(req.IP)
	if ip == "" {
		return nil, errors.New("ip is required")
	}
	if req.Port <= 0 || req.Port > 65535 {
		return nil, errors.New("port must be between 1 and 65535")
	}

	protocol := strings.ToUpper(strings.TrimSpace(req.Protocol))
	switch protocol {
	case "SOCKS5_MIXED":
		return buildSocksOutbound(req), nil
	case "SHADOWSOCKS":
		return map[string]any{
			"tag":      "probe-out",
			"protocol": "shadowsocks",
			"settings": map[string]any{
				"servers": []map[string]any{
					{
						"address":  ip,
						"port":     req.Port,
						"method":   DedicatedShadowsocksMethod,
						"password": strings.TrimSpace(req.Password),
					},
				},
			},
		}, nil
	case "VMESS":
		uuid, ok := resolveProbeUUID(req)
		if !ok {
			return nil, errDedicatedUUIDRequired
		}
		return map[string]any{
			"tag":      "probe-out",
			"protocol": "vmess",
			"settings": map[string]any{
				"vnext": []map[string]any{
					{
						"address": ip,
						"port":    req.Port,
						"users": []map[string]any{
							{
								"id":       uuid,
								"alterId":  0,
								"security": "auto",
							},
						},
					},
				},
			},
			"streamSettings": map[string]any{
				"network":  "tcp",
				"security": "none",
			},
		}, nil
	case "VLESS":
		uuid, ok := resolveProbeUUID(req)
		if !ok {
			return nil, errDedicatedUUIDRequired
		}
		return map[string]any{
			"tag":      "probe-out",
			"protocol": "vless",
			"settings": map[string]any{
				"vnext": []map[string]any{
					{
						"address": ip,
						"port":    req.Port,
						"users": []map[string]any{
							{
								"id":         uuid,
								"encryption": "none",
							},
						},
					},
				},
			},
			"streamSettings": map[string]any{
				"network":  "tcp",
				"security": "none",
			},
		}, nil
	default:
		return nil, fmt.Errorf("unsupported protocol %s", req.Protocol)
	}
}

func buildSocksOutbound(req DedicatedProtocolProbeRequest) map[string]any {
	server := map[string]any{
		"address": strings.TrimSpace(req.IP),
		"port":    req.Port,
	}
	if strings.TrimSpace(req.Username) != "" || strings.TrimSpace(req.Password) != "" {
		server["users"] = []map[string]string{
			{
				"user": strings.TrimSpace(req.Username),
				"pass": strings.TrimSpace(req.Password),
			},
		}
	}
	return map[string]any{
		"tag":      "probe-out",
		"protocol": "socks",
		"settings": map[string]any{
			"servers": []map[string]any{server},
		},
	}
}

func resolveProbeUUID(req DedicatedProtocolProbeRequest) (string, bool) {
	v := strings.TrimSpace(req.VmessUUID)
	if uuidRegex.MatchString(v) {
		return strings.ToLower(v), true
	}
	return "", false
}

func probeExitViaLocalSocks(ctx context.Context, localPort int) DedicatedProtocolProbeResult {
	endpoint := net.JoinHostPort("127.0.0.1", strconv.Itoa(localPort))
	dialer, err := proxy.SOCKS5("tcp", endpoint, nil, proxy.Direct)
	if err != nil {
		return DedicatedProtocolProbeResult{ConnectivityOK: false, Message: err.Error(), ErrorCode: "LOCAL_SOCKS_DIALER_FAILED"}
	}

	transport := &http.Transport{Dial: dialer.Dial}
	defer transport.CloseIdleConnections()

	probeCtx, cancel := context.WithTimeout(ctx, 8*time.Second)
	defer cancel()
	request, err := http.NewRequestWithContext(probeCtx, http.MethodGet, "https://api.ipify.org", nil)
	if err != nil {
		return DedicatedProtocolProbeResult{ConnectivityOK: false, Message: err.Error(), ErrorCode: "PROBE_REQUEST_BUILD_FAILED"}
	}

	client := &http.Client{Timeout: 8 * time.Second, Transport: transport}
	response, err := client.Do(request)
	if err != nil {
		if isProbeTimeout(err) {
			return DedicatedProtocolProbeResult{
				ConnectivityOK: true,
				Message:        fmt.Sprintf("protocol handshake passed; EGRESS_PROBE_TIMEOUT: %s", err.Error()),
				ErrorCode:      "EGRESS_PROBE_TIMEOUT",
			}
		}
		return DedicatedProtocolProbeResult{ConnectivityOK: false, Message: err.Error(), ErrorCode: "EGRESS_PROBE_FAILED"}
	}
	defer response.Body.Close()

	if response.StatusCode >= 300 {
		return DedicatedProtocolProbeResult{
			ConnectivityOK: false,
			Message:        fmt.Sprintf("ip probe status %d", response.StatusCode),
			ErrorCode:      "EGRESS_PROBE_HTTP_ERROR",
		}
	}

	body, err := io.ReadAll(io.LimitReader(response.Body, 256))
	if err != nil {
		return DedicatedProtocolProbeResult{ConnectivityOK: false, Message: err.Error(), ErrorCode: "EGRESS_PROBE_READ_FAILED"}
	}
	exitIP := strings.TrimSpace(string(body))
	if exitIP == "" {
		return DedicatedProtocolProbeResult{ConnectivityOK: false, Message: "empty exit ip", ErrorCode: "EGRESS_PROBE_EMPTY_EXIT_IP"}
	}

	country, region, lookupErr := lookupCountryRegion(exitIP)
	if lookupErr != nil {
		country = ""
		region = ""
	}

	message := "protocol probe succeeded"
	if country != "" {
		message = fmt.Sprintf("protocol probe succeeded (%s)", strings.ToUpper(country))
	}

	return DedicatedProtocolProbeResult{
		ConnectivityOK: true,
		ExitIP:         exitIP,
		CountryCode:    strings.ToLower(strings.TrimSpace(country)),
		Region:         strings.TrimSpace(region),
		Message:        message,
	}
}

func isProbeTimeout(err error) bool {
	if err == nil {
		return false
	}
	var netErr net.Error
	if errors.As(err, &netErr) && netErr.Timeout() {
		return true
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "timeout") ||
		strings.Contains(msg, "deadline exceeded") ||
		strings.Contains(msg, "connection reset by peer") ||
		strings.Contains(msg, "eof")
}
