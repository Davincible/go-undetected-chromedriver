// Package goundetectedchromedriver provides a chrome driver.
package goundetectedchromedriver

import (
	"errors"
	"fmt"
	"net"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"golang.org/x/exp/slog"

	"github.com/Xuanwo/go-locale"
	"github.com/tebeka/selenium"
	"github.com/tebeka/selenium/chrome"

	"github.com/Davincible/go-undetected-chromedriver/patcher"
)

var (
	ErrChromeNotFound = errors.New("chrome executable not found, please install or provide a path")
)

type Chrome struct {
	selenium.WebDriver

	driverPath string
	driverArgs []string
	chromePath string
	chromeArgs []string

	userDataDir string
	headless    bool

	port         string
	debuggerAddr string

	chrome *exec.Cmd
	driver *exec.Cmd

	config Config
}

func NewChromeDriver(opts ...Option) (Chrome, error) {
	c := Chrome{config: NewConfig()}

	for _, o := range opts {
		o(&c.config)
	}

	if c.config.Debug {
		handler := (slog.HandlerOptions{Level: slog.DebugLevel}).NewTextHandler(os.Stdout)
		slog.SetDefault(slog.New(handler))
	}

	version := c.config.Version
	if version == 0 {
		var err error

		version, err = getChromeVersion()
		if err != nil {
			return c, err
		}
	}

	p, err := patcher.New(c.config.DriverExecutable, version)
	if err != nil {
		return c, err
	}

	slog.Debug("patching binary")

	c.driverPath, err = p.Patch()
	if err != nil {
		return c, err
	}

	dHost, dPort, err := c.getDebuggerAddress()
	if err != nil {
		return c, err
	}

	c.chromeArgs = c.config.ChromeArgs
	c.chromeArgs = append(c.chromeArgs,
		"--remote-debugging-host="+dHost,
		"--remote-debugging-port="+dPort,
	)
	c.debuggerAddr = dHost + ":" + dPort

	if len(c.config.UserDataDir) > 0 {
		c.userDataDir = c.config.UserDataDir
	} else {
		c.userDataDir, err = os.MkdirTemp("", "undetected-chromedriver-userdata-*")
		if err != nil {
			return c, fmt.Errorf("failed to create temp userdata dir: %w", err)
		}
	}

	// TODO: maybe make userdatadir a local var
	c.chromeArgs = append(c.chromeArgs,
		"--user-data-dir="+c.userDataDir,
	)

	lang := "en-US"
	if tag, err := locale.Detect(); err != nil && len(tag.String()) > 0 {
		lang = tag.String()
	}

	c.chromeArgs = append(c.chromeArgs,
		"--lang="+lang,
	)

	if c.config.SuppressWelcome {
		c.chromeArgs = append(c.chromeArgs,
			"--no-default-browser-check", "--no-first-run",
		)
	}

	if !c.config.Sandbox {
		c.chromeArgs = append(c.chromeArgs,
			"--no-sandbox", "--test-type",
		)
	}

	if !c.config.Headless {
		c.headless = true
		c.chromeArgs = append(c.chromeArgs,
			"--window-size=1920,1080",
			"--start-maximized",
			"--start-maximized",
		)
	}

	c.chromeArgs = append(c.chromeArgs,
		"--log-level="+strconv.Itoa(c.config.LogLevel),
	)

	// TODO: tab restore nag

	if err := c.startChrome(); err != nil {
		return c, err
	}

	if err := c.startDriver(); err != nil {
		return c, err
	}

	time.Sleep(3 * time.Second)

	if err := c.connect(); err != nil {
		return c, err
	}

	// TODO: cdp events

	// TODO: config headless

	return c, nil
}

func (c *Chrome) Get(url string) error {
	if c.getCdcProps() {
		c.removeCdcProps()
	}

	return c.WebDriver.Get(url)
}

func (c *Chrome) removeCdcProps() {
	// TODO: implement
}

func (c *Chrome) getCdcProps() bool {
	script := `
  let objectToInspect = window,
      result = [];
  while(objectToInspect !== null)
  { result = result.concat(Object.getOwnPropertyNames(objectToInspect));
    objectToInspect = Object.getPrototypeOf(objectToInspect); }
  return result.filter(i => i.match(/.+_.+_(Array|Promise|Symbol)/ig))
	`

	resp, err := c.ExecuteScript(script, nil)
	if err != nil {
		slog.Error("failed to execute get cdc script", err)
		return false
	}

	return len(resp.([]any)) > 0
}

func (c *Chrome) startChrome() error {
	c.chromePath = findChrome()
	if len(c.config.BrowserExecurable) > 0 {
		c.chromePath = c.config.BrowserExecurable
	}

	if len(c.chromePath) == 0 {
		return ErrChromeNotFound
	}

	c.chrome = exec.Command(c.chromePath, c.chromeArgs...)

	slog.Debug("Starting Chrome", slog.String("cmd", c.chrome.String()))

	if c.config.Debug {
		c.chrome.Stdout = os.Stdout
		c.chrome.Stderr = os.Stderr
	}

	if err := c.chrome.Start(); err != nil {
		return fmt.Errorf("failed to start chrome: %w", err)
	}

	return nil
}

func (c *Chrome) startDriver() error {
	c.driverArgs = c.config.DriverArgs

	c.port = strconv.Itoa(c.config.Port)
	if c.config.Port == 0 {
		l, err := net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			return fmt.Errorf("failed to start listener: %w", err)
		}

		c.port = strings.Split(l.Addr().String(), ":")[1]

		l.Close() //nolint:errcheck,gosec
	}

	c.driverArgs = append(c.driverArgs, "--port="+c.port)

	c.driver = exec.Command(c.driverPath, c.driverArgs...)

	if c.config.Debug {
		c.driver.Stdout = os.Stdout
		c.driver.Stderr = os.Stderr
	}

	slog.Debug("Starting ChromeDriver", slog.String("cmd", c.driver.String()))

	if err := c.driver.Start(); err != nil {
		return fmt.Errorf("failed to start chromedriver: %w", err)
	}

	return nil
}

func (c *Chrome) connect() error {
	caps := selenium.Capabilities{
		"browserName":      "chrome",
		"pageLoadStrategy": "normal",
	}

	caps.AddChrome(chrome.Capabilities{
		Path:         c.chromePath,
		Args:         c.chromeArgs,
		DebuggerAddr: c.debuggerAddr,
	})

	addr := fmt.Sprintf("http://127.0.0.1:%s", c.port)

	slog.Debug("Connecting to driver", slog.String("addr", addr))

	driver, err := selenium.NewRemote(caps, addr)
	if err != nil {
		return fmt.Errorf("failed to connect to chromedriver: %w", err)
	}

	slog.Debug("Connected", slog.String("addr", addr))

	c.WebDriver = driver

	return nil
}

func (c *Chrome) getDebuggerAddress() (string, string, error) {
	var split []string

	host := "127.0.0.1"
	port := "0"

	if len(c.config.DebuggerAddress) > 0 {
		split = strings.Split(c.config.DebuggerAddress, ":")
	} else {
		addr := host + ":" + port
		l, err := net.Listen("tcp", addr)
		if err != nil {
			return "", "", fmt.Errorf("failed to start listener on '%s': %w", addr, err)
		}

		split = strings.Split(l.Addr().String(), ":")

		l.Close() //nolint:errcheck,gosec
	}

	if len(split) > 1 {
		host = split[0]
		port = split[1]
	} else {
		port = split[0]
	}

	return host, port, nil
}

func getChromeVersion() (int, error) {
	binary := findChrome()
	if len(binary) == 0 {
		return 0, ErrChromeNotFound
	}

	cmd := exec.Command(binary, "--version") //nolint:gosec

	out, err := cmd.Output()
	if err != nil {
		return 0, fmt.Errorf("failed to fetch chrome version: %w", err)
	}

	version := strings.Split(strings.Split(string(out), " ")[1], ".")[0]

	return strconv.Atoi(version)
}

func findChrome() string {
	binaries := []string{
		"google-chrome",
		"chromium",
		"chromium-browser",
		"chrome",
		"google-chrome-stable",
	}

	for _, bin := range binaries {
		if path, err := exec.LookPath(bin); err == nil {
			return path
		}
	}

	return ""
}
