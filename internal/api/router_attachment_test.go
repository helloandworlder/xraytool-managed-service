package api

import (
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestSetAttachmentFilenameSupportsChineseRFC5987(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	setAttachmentFilename(c, "客户-美国-导出.xlsx")
	header := w.Header().Get("Content-Disposition")
	if header == "" {
		t.Fatalf("missing Content-Disposition header")
	}
	if !strings.Contains(header, "filename*=UTF-8''") {
		t.Fatalf("header should contain filename* utf-8 extension, got: %s", header)
	}
	if !strings.Contains(header, "%E5%AE%A2%E6%88%B7") {
		t.Fatalf("header should contain encoded chinese filename, got: %s", header)
	}
}

func TestAsciiFilenameFallbackIsSafeASCII(t *testing.T) {
	name := asciiFilenameFallback("客户-美国-导出.xlsx")
	if name == "" {
		t.Fatalf("fallback filename should not be empty")
	}
	if !strings.HasSuffix(name, ".xlsx") {
		t.Fatalf("fallback should preserve extension, got: %s", name)
	}
	for _, r := range name {
		if r > 127 {
			t.Fatalf("fallback should be ascii only, got rune %q in %s", r, name)
		}
	}
}
