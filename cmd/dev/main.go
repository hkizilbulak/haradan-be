package main

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"time"

	"github.com/joho/godotenv"
)

const (
	localAPIAddress = ":3001"
	shutdownTimeout = 10 * time.Second
)

type environmentValue struct {
	name  string
	value string
}

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(args []string) error {
	if len(args) != 1 || (args[0] != "api" && args[0] != "worker") {
		return errors.New("kullanım: go run ./cmd/dev <api|worker>")
	}

	root, err := findRepositoryRoot()
	if err != nil {
		return err
	}

	envPath := filepath.Join(root, ".env")
	fileEnvironment, err := godotenv.Read(envPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("yerel başlangıç için haradan-be/.env gereklidir: %s", envPath)
		}
		return fmt.Errorf("BE .env okunamadı: %w", err)
	}

	processEnvironment := os.Environ()
	caseInsensitiveEnvironment := runtime.GOOS == "windows"
	environment := mergeEnvironment(fileEnvironment, processEnvironment, caseInsensitiveEnvironment)
	if args[0] == "api" {
		setLocalAPIAddress(environment, processEnvironment, caseInsensitiveEnvironment)
	}

	temporaryDirectory, err := os.MkdirTemp("", "haradan-be-dev-")
	if err != nil {
		return fmt.Errorf("geçici build dizini oluşturulamadı: %w", err)
	}
	defer os.RemoveAll(temporaryDirectory)

	binaryName := "haradan-" + args[0]
	if runtime.GOOS == "windows" {
		binaryName += ".exe"
	}
	binaryPath := filepath.Join(temporaryDirectory, binaryName)
	targetPath := "." + string(filepath.Separator) + filepath.Join("cmd", args[0])

	buildCommand := exec.Command("go", "build", "-o", binaryPath, targetPath)
	buildCommand.Dir = root
	buildCommand.Stdin = os.Stdin
	buildCommand.Stdout = os.Stdout
	buildCommand.Stderr = os.Stderr
	if err := buildCommand.Run(); err != nil {
		return fmt.Errorf("%s build başarısız: %w", args[0], err)
	}

	child := exec.Command(binaryPath)
	child.Dir = root
	child.Env = environmentSlice(environment)
	child.Stdin = os.Stdin
	child.Stdout = os.Stdout
	child.Stderr = os.Stderr

	if err := runChild(child); err != nil {
		return fmt.Errorf("%s başarısız: %w", args[0], err)
	}
	return nil
}

func findRepositoryRoot() (string, error) {
	current, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("çalışma dizini belirlenemedi: %w", err)
	}
	current, err = filepath.Abs(current)
	if err != nil {
		return "", fmt.Errorf("çalışma dizini çözümlenemedi: %w", err)
	}

	for {
		if info, statErr := os.Stat(filepath.Join(current, "go.mod")); statErr == nil && !info.IsDir() {
			return current, nil
		}
		parent := filepath.Dir(current)
		if parent == current {
			return "", errors.New("haradan-be depo kökü bulunamadı; komutu depo içinden çalıştırın")
		}
		current = parent
	}
}

func mergeEnvironment(fileValues map[string]string, processValues []string, caseInsensitive bool) map[string]environmentValue {
	merged := make(map[string]environmentValue, len(fileValues)+len(processValues))
	for name, value := range fileValues {
		merged[canonicalEnvironmentName(name, caseInsensitive)] = environmentValue{name: name, value: value}
	}
	for _, item := range processValues {
		name, value, ok := strings.Cut(item, "=")
		if !ok {
			continue
		}
		merged[canonicalEnvironmentName(name, caseInsensitive)] = environmentValue{name: name, value: value}
	}
	return merged
}

func setLocalAPIAddress(environment map[string]environmentValue, processValues []string, caseInsensitive bool) {
	key := canonicalEnvironmentName("HTTP_ADDR", caseInsensitive)
	for _, item := range processValues {
		name, value, ok := strings.Cut(item, "=")
		if ok && canonicalEnvironmentName(name, caseInsensitive) == key && strings.TrimSpace(value) != "" {
			return
		}
	}
	environment[key] = environmentValue{name: "HTTP_ADDR", value: localAPIAddress}
}

func canonicalEnvironmentName(name string, caseInsensitive bool) string {
	if caseInsensitive {
		return strings.ToUpper(name)
	}
	return name
}

func environmentSlice(environment map[string]environmentValue) []string {
	keys := make([]string, 0, len(environment))
	for key := range environment {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	result := make([]string, 0, len(keys))
	for _, key := range keys {
		item := environment[key]
		result = append(result, item.name+"="+item.value)
	}
	return result
}

func runChild(child *exec.Cmd) error {
	if err := child.Start(); err != nil {
		return err
	}

	interrupts := make(chan os.Signal, 2)
	signal.Notify(interrupts, os.Interrupt)
	defer signal.Stop(interrupts)

	done := make(chan error, 1)
	go func() {
		done <- child.Wait()
	}()

	select {
	case err := <-done:
		return err
	case interrupt := <-interrupts:
		// Console interrupts are delivered to the whole foreground process tree.
		// Explicit forwarding covers POSIX signals sent only to this launcher.
		if runtime.GOOS != "windows" {
			_ = child.Process.Signal(interrupt)
		}

		timer := time.NewTimer(shutdownTimeout)
		defer timer.Stop()
		select {
		case <-done:
			return nil
		case <-interrupts:
			_ = child.Process.Kill()
			<-done
			return nil
		case <-timer.C:
			_ = child.Process.Kill()
			<-done
			return nil
		}
	}
}
