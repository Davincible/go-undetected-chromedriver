package patcher

import (
	"io"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestPatcherLatest(t *testing.T) {
	// CI is slow
	RequestTimeout = 30 * time.Second

	p, err := NewPatcher("", 0)
	require.NoError(t, err, "create patcher")

	path, err := p.Patch()
	require.NoError(t, err, "patch")
	t.Log(path)

	file, err := os.Open(path)
	require.NoError(t, err, "open driver")

	driver, err := io.ReadAll(file)
	require.NoError(t, err, "read driver")

	re := regexp.MustCompile("cdc_.{22}")
	require.Equal(t, false, re.Match(driver))

	_, err = exec.Command(path, "--version").Output()
	require.NoError(t, err, "execute")
}

func TestPatcherVersionPin(t *testing.T) {
	// CI is slow
	RequestTimeout = 30 * time.Second

	p, err := NewPatcher("", 105)
	require.NoError(t, err, "create patcher")

	path, err := p.Patch()
	require.NoError(t, err, "patch")
	t.Log(path)

	file, err := os.Open(path)
	require.NoError(t, err, "open driver")

	driver, err := io.ReadAll(file)
	require.NoError(t, err, "read driver")

	re := regexp.MustCompile("cdc_.{22}")
	require.Equal(t, false, re.Match(driver))

	output, err := exec.Command(path, "--version").Output()
	require.NoError(t, err, "check version")
	require.Equal(t, true, strings.Contains(string(output), "105"))
}
