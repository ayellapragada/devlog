package install

type Context struct {
	Interactive bool
	ConfigDir   string
	DataDir     string
	HomeDir     string
	Log         func(format string, args ...interface{})
}
