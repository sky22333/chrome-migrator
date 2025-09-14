package config

type BrowserType int

const (
	BrowserChrome BrowserType = iota
	BrowserEdge
	BrowserBoth
)

const (
	OutputBaseDir = "C:\\chrome-backup"
	TempDir       = "C:\\chrome-backup\\temp"
	
	MaxRetries    = 3
	RetryDelay    = 1000
	
	RequiredDiskSpaceMultiplier = 2
)

type Config struct {
	OutputDir     string
	TempDir       string
	Silent        bool
	MaxRetries    int
	RetryDelay    int
	BrowserType   BrowserType
	ShowProgress  bool
}

func DefaultConfig() *Config {
	return &Config{
		OutputDir:    OutputBaseDir,
		TempDir:      TempDir,
		Silent:       false,
		MaxRetries:   MaxRetries,
		RetryDelay:   RetryDelay,
		BrowserType:  BrowserChrome,
		ShowProgress: true,
	}
}

func (bt BrowserType) String() string {
	switch bt {
	case BrowserChrome:
		return "Chrome"
	case BrowserEdge:
		return "Edge"
	case BrowserBoth:
		return "Both"
	default:
		return "Unknown"
	}
}