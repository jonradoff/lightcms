package services

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"testing"

	"github.com/jonradoff/lightcms/v7/internal/testutil"
)

// TestMain keeps the suite from touching tracked files: the theme CSS is
// written relative to the working directory, which for this package's tests
// is internal/services, where static/css/theme-vars.css is checked in. It is
// pointed at a temp directory, and the run fails if any tracked file under
// the package has changed by the end.
func TestMain(m *testing.M) {
	tmp, err := os.MkdirTemp("", "lightcms-services-test-")
	if err != nil {
		fmt.Fprintln(os.Stderr, "TestMain:", err)
		os.Exit(1)
	}
	ThemeCSSFile = filepath.Join(tmp, "css", "theme-vars.css")
	// The package's webhook receivers are httptest servers on 127.0.0.1,
	// which the SSRF guard refuses. Tests of the guard itself put it back
	// (see useGuardedTransport).
	outboundTransport = http.DefaultTransport
	changed := testutil.SnapshotTrackedFiles()

	code := m.Run()

	os.RemoveAll(tmp)
	if dirty := changed(); len(dirty) > 0 {
		fmt.Fprintf(os.Stderr, "FAIL: the test run modified tracked files (tests must write to a temp directory): %v\n", dirty)
		if code == 0 {
			code = 1
		}
	}
	os.Exit(code)
}
