package cli

import (
	"flag"
	"fmt"
	"io"
	"runtime"
	"strings"
)

const (
	ExitSuccess       = 0
	ExitRuntime       = 1
	ExitAccessibility = 2
	ExitNoFocusedApp  = 3
	ExitUsage         = 64
)

type Flags struct {
	Config  string
	Policy  string
	Verbose bool
	DryRun  bool
	Help    bool
}

const usage = `Usage: same-window-switcher <command> [flags]

Commands:
  next      raise the next window of the focused app
  prev      raise the previous window of the focused app
  list      list eligible windows
  doctor    diagnose permissions and environment
  version   print version
  help      print this help

Flags:
  --config PATH    path to config file
  --policy NAME    sort policy override
  -v, --verbose    verbose output
  --dry-run        select but do not raise
  -h, --help       print this help
`

func printUsage(w io.Writer) {
	fmt.Fprint(w, usage)
}

func flagNeedsValue(arg string) bool {
	if strings.Contains(arg, "=") {
		return false
	}
	switch arg {
	case "--config", "-config", "--policy", "-policy":
		return true
	default:
		return false
	}
}

func splitCommand(args []string) (cmd string, flagArgs []string) {
	flagArgs = make([]string, 0, len(args))
	for i := 0; i < len(args); i++ {
		a := args[i]
		if cmd == "" && a == "--" {
			if i+1 < len(args) {
				cmd = args[i+1]
				flagArgs = append(flagArgs, args[i+2:]...)
			}
			return cmd, flagArgs
		}
		if cmd == "" && !strings.HasPrefix(a, "-") {
			cmd = a
			continue
		}
		flagArgs = append(flagArgs, a)
		if flagNeedsValue(a) && i+1 < len(args) {
			i++
			flagArgs = append(flagArgs, args[i])
		}
	}
	return cmd, flagArgs
}

func Parse(args []string) (cmd string, flags Flags, rest []string, err error) {
	cmd, flagArgs := splitCommand(args)
	fs := flag.NewFlagSet("same-window-switcher", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	fs.StringVar(&flags.Config, "config", "", "path to config file")
	fs.StringVar(&flags.Policy, "policy", "", "sort policy override")
	fs.BoolVar(&flags.Verbose, "verbose", false, "verbose output")
	fs.BoolVar(&flags.Verbose, "v", false, "verbose output")
	fs.BoolVar(&flags.DryRun, "dry-run", false, "do not raise")
	fs.BoolVar(&flags.Help, "help", false, "print help")
	fs.BoolVar(&flags.Help, "h", false, "print help")
	if err := fs.Parse(flagArgs); err != nil {
		return cmd, Flags{}, nil, err
	}
	return cmd, flags, fs.Args(), nil
}

func Run(args []string, stdout, stderr io.Writer, version string) int {
	if len(args) > 0 {
		args = args[1:]
	}
	cmd, flags, rest, err := Parse(args)
	if err != nil {
		if err == flag.ErrHelp {
			printUsage(stdout)
			return ExitSuccess
		}
		fmt.Fprintf(stderr, "%v\n", err)
		printUsage(stderr)
		return ExitUsage
	}
	if flags.Help || cmd == "help" {
		printUsage(stdout)
		return ExitSuccess
	}
	if cmd == "" {
		fmt.Fprintln(stderr, "missing command")
		printUsage(stderr)
		return ExitUsage
	}
	if len(rest) > 0 {
		fmt.Fprintf(stderr, "unexpected arguments: %s\n", strings.Join(rest, " "))
		printUsage(stderr)
		return ExitUsage
	}
	switch cmd {
	case "version":
		fmt.Fprintf(stdout, "same-window-switcher %s %s/%s\n", version, runtime.GOOS, runtime.GOARCH)
		return ExitSuccess
	case "next", "prev", "list", "doctor":
		return runCommand(cmd, flags, stdout, stderr)
	default:
		fmt.Fprintf(stderr, "unknown command: %s\n", cmd)
		printUsage(stderr)
		return ExitUsage
	}
}
