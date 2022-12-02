package goundetectedchromedriver

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestChromeDriverTestChromeDriver(t *testing.T) {
	driver, err := NewChromeDriver(
		WithDebug(),
		// 		WithUserDataDir("/tmp/chrome-data"),
	)

	require.NoError(t, err, "create chrome driver")

	require.NoError(t, driver.Get("https://nowsecure.nl"), "navigate url")

	time.Sleep(15 * time.Second)
}
