package goundetectedchromedriver

// Option is a functional option type.
type Option func(*Config)

// Config for Chrome config.
type Config struct {
	// DriverExecutable can optionally be set to provide a custom driver.
	// If the driver is not patched yet it will be patched automatically.
	DriverExecutable string

	BrowserExecurable string

	UserDataDir string

	// Port is the port the chromedriver will listen on
	Port int

	// DebuggerAddress is the address the chrome debugger will listen on
	DebuggerAddress string

	// ChromeArgs are additional arguments to pass to chrome.
	//
	// Note that if you provide options that will be set already, the will be
	// set twice.
	ChromeArgs []string

	// DriverArgs are additional arguments to pass to the chromedriver.
	//
	// To set a custom port, use the port argument instead of an arg here.
	DriverArgs []string

	// Language locale to pass to chrome. e.g.'en-US'
	Language string

	SuppressWelcome bool
	KeepAlive       bool
	LogLevel        int
	Headless        bool
	// Version is the main chromedriver version to use, e.g. 107.
	Version       int
	Debug         bool
	UseSubprocess bool // check this
	Sandbox       bool

	// Do we need these?
	EnableCDPEvents      bool
	ServiceArgs          []any
	ServiceCreationFlags []any
	ServiceLogPath       string
	AdvancedElements     bool
	PatcherForceClose    bool
}

// NewConfig creates new config object.
func NewConfig(opts ...Option) Config {
	c := Config{}

	for _, o := range opts {
		o(&c)
	}

	return c
}

// WithDebug sets the debug option.
func WithDebug() Option {
	return func(c *Config) {
		c.Debug = true
	}
}

// WithUserDataDir sets a directory to use as chrome user profile.
func WithUserDataDir(path string) Option {
	return func(c *Config) {
		c.UserDataDir = path
	}
}

// TODO: implement other options
