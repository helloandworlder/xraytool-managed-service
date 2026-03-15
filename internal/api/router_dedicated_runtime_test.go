package api

import (
	"strings"
	"testing"

	"xraytool/internal/model"
)

func TestBuildDedicatedLinksCompatVlessUsesShareParamsAndSingleProtocol(t *testing.T) {
	links, err := buildDedicatedLinksCompat("VLESS", model.DedicatedIngress{
		Name:        "HKNode1-Vless-Short",
		Domain:      "isp.vikingar.top",
		IngressPort: 48082,
		DedicatedInbound: model.DedicatedInbound{
			ID:            2,
			Protocol:      model.DedicatedFeatureVless,
			VlessSecurity: "none",
			VlessType:     "tcp",
		},
	}, "user-demo", "pass-demo", "21d88af8-0179-404e-ab18-18a91b66d121")
	if err != nil {
		t.Fatalf("buildDedicatedLinksCompat returned error: %v", err)
	}
	if len(links) != 1 {
		t.Fatalf("expected single VLESS link, got %d", len(links))
	}
	link := links[0]
	if !strings.HasPrefix(link, "vless://21d88af8-0179-404e-ab18-18a91b66d121@isp.vikingar.top:48082?") {
		t.Fatalf("unexpected VLESS link: %s", link)
	}
	if !strings.Contains(link, "encryption=none") {
		t.Fatalf("expected encryption=none in VLESS link, got %s", link)
	}
	if strings.Contains(link, "type=tcp") || strings.Contains(link, "security=none") {
		t.Fatalf("expected default TCP/none params to be normalized out, got %s", link)
	}
}

func TestBuildDedicatedLinksCompatRejectsProtocolIngressMismatch(t *testing.T) {
	_, err := buildDedicatedLinksCompat("VMESS", model.DedicatedIngress{
		Name:        "HKNode1-Vless-Short",
		Domain:      "isp.vikingar.top",
		IngressPort: 48082,
		DedicatedInbound: model.DedicatedInbound{
			ID:       2,
			Protocol: model.DedicatedFeatureVless,
		},
	}, "user-demo", "pass-demo", "21d88af8-0179-404e-ab18-18a91b66d121")
	if err == nil {
		t.Fatal("expected protocol mismatch error")
	}
}
