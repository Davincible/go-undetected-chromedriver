package patcher

import (
	"io"
	"os"
	"regexp"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestPatcher(t *testing.T) {
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
}
