package app

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"tower/internal/contracts"
	"tower/internal/core"
	towerruntime "tower/internal/runtime"
	"tower/internal/store"
	"tower/internal/ui"
)

type Bootstrap struct {
	Layout store.Layout
	Store  store.Repository
	Engine *core.Engine
	UI     *ui.Stub
}

type DemoFixture struct {
	Name        string                      `json:"name"`
	Description string                      `json:"description,omitempty"`
	Sessions    []contracts.SessionSnapshot `json:"sessions"`
}

func NewBootstrap() (*Bootstrap, error) {
	layout, err := store.DefaultLayout()
	if err != nil {
		return nil, err
	}

	repository := store.NewMemoryRepository(layout)
	engine := core.NewEngine(repository, towerruntime.NewBootstrapManager())

	return &Bootstrap{
		Layout: layout,
		Store:  repository,
		Engine: engine,
		UI:     ui.NewStub(),
	}, nil
}

func RunCLI(ctx context.Context, stdout io.Writer, _ io.Writer, args []string) error {
	bootstrap, err := NewBootstrap()
	if err != nil {
		return err
	}

	if len(args) == 0 {
		return bootstrap.UI.RenderBootstrap(stdout, ui.BootstrapView{
			Layout:            bootstrap.Layout,
			ManagedEntrypoint: "tower run claude",
			ObservedAdapters:  []string{"copilot-cli", "vscode", "wsl"},
		})
	}

	switch args[0] {
	case "help", "-h", "--help":
		return bootstrap.UI.RenderBootstrap(stdout, ui.BootstrapView{
			Layout:            bootstrap.Layout,
			ManagedEntrypoint: "tower run claude",
			ObservedAdapters:  []string{"copilot-cli", "vscode", "wsl"},
		})
	case "run":
		return bootstrap.runManaged(ctx, stdout, args[1:])
	case "internal":
		return bootstrap.runInternal(stdout, args[1:])
	default:
		return fmt.Errorf("unknown command %q", args[0])
	}
}

func RunDemo(ctx context.Context, stdout io.Writer, _ io.Writer, args []string) error {
	bootstrap, err := NewBootstrap()
	if err != nil {
		return err
	}

	fixturePath := defaultDemoFixturePath()
	if len(args) > 1 {
		return errors.New("tower-demo accepts at most one fixture path")
	}
	if len(args) == 1 {
		fixturePath = args[0]
	}

	fixture, err := LoadDemoFixture(fixturePath)
	if err != nil {
		return err
	}

	return bootstrap.UI.RenderDemo(stdout, ui.DemoView{
		FixtureName: fixture.Name,
		FixturePath: fixturePath,
		Sessions:    fixture.Sessions,
	})
}

func LoadDemoFixture(path string) (DemoFixture, error) {
	contents, err := os.ReadFile(path)
	if err != nil {
		return DemoFixture{}, err
	}

	var fixture DemoFixture
	if err := json.Unmarshal(contents, &fixture); err != nil {
		return DemoFixture{}, err
	}
	if fixture.Name == "" {
		return DemoFixture{}, errors.New("demo fixture name is required")
	}

	return fixture, nil
}

func (bootstrap *Bootstrap) runManaged(ctx context.Context, stdout io.Writer, args []string) error {
	if len(args) == 0 {
		return errors.New("run requires a tool name")
	}
	if args[0] != "claude" {
		return fmt.Errorf("%q is not wired for managed launch yet", args[0])
	}

	workingDir, err := os.Getwd()
	if err != nil {
		return err
	}

	snapshot, err := bootstrap.Engine.LaunchManagedSession(
		ctx,
		"claude",
		args[1:],
		workingDir,
		sanitizedLaunchEnvironment(),
		detectTerminalMetadata(),
	)
	if err != nil {
		return err
	}

	return bootstrap.UI.RenderManagedLaunch(stdout, ui.ManagedLaunchView{
		Layout:   bootstrap.Layout,
		Snapshot: snapshot,
	})
}

func (bootstrap *Bootstrap) runInternal(stdout io.Writer, args []string) error {
	if len(args) == 1 && args[0] == "claude-runtime" {
		_, err := fmt.Fprintln(stdout, "tower internal claude-runtime: bootstrap placeholder for the hidden managed runtime helper.")
		return err
	}
	return errors.New("internal requires the claude-runtime subcommand")
}

func defaultDemoFixturePath() string {
	return filepath.Join("test", "fixtures", "demo", "six-session-mixed.json")
}

func sanitizedLaunchEnvironment() map[string]string {
	keys := []string{
		"TERM",
		"TERM_PROGRAM",
		"WT_SESSION",
		"COMSPEC",
		"PROMPT",
	}

	environment := make(map[string]string, len(keys))
	for _, key := range keys {
		if value := os.Getenv(key); value != "" {
			environment[key] = value
		}
	}
	return environment
}

func detectTerminalMetadata() contracts.TerminalMetadata {
	return contracts.TerminalMetadata{
		Program:       firstNonEmpty(os.Getenv("TERM_PROGRAM"), "unknown"),
		WindowSession: os.Getenv("WT_SESSION"),
		DeviceName:    firstNonEmpty(os.Getenv("TERM"), "unknown"),
	}
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}
