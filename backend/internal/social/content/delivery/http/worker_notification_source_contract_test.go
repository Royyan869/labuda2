package http

import (
	"os"
	"strings"
	"testing"
)

func TestNotificationWorkerHasNoContentKindPostRequestBranch(t *testing.T) {
	src, err := os.ReadFile("../../../../worker/notification_worker_social.go")
	if err != nil {
		t.Fatalf("read notification_worker_social.go: %v", err)
	}

	source := string(src)

	for _, forbidden := range []string{
		"contententity.TypePost",
		"contententity.TypeRequest",
		`"targetType": "post"`,
	} {
		if strings.Contains(source, forbidden) {
			t.Fatalf("worker source still contains forbidden token %q", forbidden)
		}
	}

	if got := strings.Count(source, `"targetType": "content"`); got < 3 {
		t.Fatalf("expected canonical content targetType in content notifications, got %d", got)
	}
}
