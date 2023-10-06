package main

import (
	"fmt"
	"github.com/fatih/color"
	"github.com/mattn/go-isatty"
	"io"
	"os"
	"os/exec"
	"strings"
)

var inputFilename string

// openFileOrStdin opens a file, or if filename is "-" stdin
func openFileOrStdin(filename string) (*os.File, error) {
	if filename != "-" && filename != "" {
		inputFilename = filename
		file, err := os.Open(filename)
		if err != nil {
			return nil, fmt.Errorf("failed to open input: %w", err)
		}
		return file, nil
	}
	inputFilename = "stdin"
	stat, err := os.Stdin.Stat()
	if err != nil {
		return nil, fmt.Errorf("while opening input: %w", err)
	}
	if (stat.Mode() & os.ModeCharDevice) != 0 {
		return nil, fmt.Errorf("stdin doesn't look like something I can read")
	}
	return os.Stdin, nil
}

// readFileOrStdin reads a file, or stdin if filename is "-"
func readFileOrStdin(filename string) ([]byte, error) {
	f, err := openFileOrStdin(filename)
	if err != nil {
		return nil, err
	}
	return io.ReadAll(f)
}

// openInput opens a file or stdin, based on arguments
func openInput(args []string) (*os.File, error) {
	if len(args) == 0 {
		return openFileOrStdin("-")
	}
	return openFileOrStdin(args[0])
}

// readInput reads a file or stdin, based on arguments
func readInput(args []string) ([]byte, error) {
	if len(args) == 0 {
		return readFileOrStdin("-")
	}
	return readFileOrStdin(args[0])
}

func inputFile(args []string) string {
	if len(args) == 0 {
		return "-"
	}
	return args[0]
}

var outputFilename string

// outputFile provides a writer for stdout or the output
// filename defined by flags
func outputFile() (io.Writer, error) {
	if outputFilename == "" || outputFilename == "-" {
		return os.Stdout, nil
	}
	file, err := os.Create(outputFilename)
	if err != nil {
		return nil, fmt.Errorf("couldn't create output: %w", err)
	}
	return file, nil
}

func outputFilePretty() string {
	if outputFilename == "" || outputFilename == "-" {
		return "stdout"
	}
	return outputFilename
}

func UseColor(w io.Writer) bool {
	if forceColor {
		color.NoColor = false
		return true
	}
	if os.Getenv("CLICOLOR_FORCE") != "" {
		color.NoColor = false
		return true
	}
	if os.Getenv("NO_COLOR") != "" {
		return false
	}
	underlyingFile, ok := w.(*os.File)
	if !ok {
		color.NoColor = true
		return false
	}
	return isatty.IsTerminal(underlyingFile.Fd())
}

func ColorFPrintf(w io.Writer, color *color.Color, format string, a ...interface{}) {
	if !UseColor(w) {
		_, _ = fmt.Fprintf(w, format, a...)
		return
	}
	_, _ = color.Fprintf(w, format, a...)
}

func ErrorExit(err error) {
	ColorFPrintf(os.Stderr, color.New(color.FgRed), "ERROR: %s\n", err)
	os.Exit(1)
}

func Error(msg string, args ...any) {
	ColorFPrintf(os.Stderr, color.New(color.FgRed), msg, args...)
}

func Warn(msg string, args ...any) {
	ColorFPrintf(os.Stderr, color.New(color.FgYellow), msg, args...)
}

func Info(msg string, args ...any) {
	ColorFPrintf(os.Stderr, color.New(color.FgMagenta), msg, args...)
}

func Success(msg string, args ...any) {
	ColorFPrintf(os.Stderr, color.New(color.FgGreen), msg, args...)
}

func ShowDoc(doc string) {
	if !isatty.IsTerminal(os.Stdout.Fd()) {
		_, _ = os.Stdout.WriteString(doc)
		return
	}
	if pageDoc(os.Getenv("MANPAGER"), doc) {
		return
	}
	if pageDoc(os.Getenv("PAGER"), doc) {
		return
	}
	if pageDoc("pager -is", doc) {
		return
	}
	if pageDoc("less -is", doc) {
		return
	}
	_, _ = os.Stdout.WriteString(doc)
}

func pageDoc(pager, doc string) bool {
	args := strings.Fields(pager)
	if len(args) == 0 {
		return false
	}
	file, err := exec.LookPath(args[0])
	if err != nil {
		return false
	}
	cmd := exec.Command(file, args[1:]...)
	cmd.Stdin = strings.NewReader(doc)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	err = cmd.Run()
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "Failed to spawn '%s': %v", pager, err)
	}
	return true
}
