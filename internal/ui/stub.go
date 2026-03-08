package ui

import (
	"fmt"
	"io"

	"tower/internal/contracts"
	"tower/internal/store"
)

// Stub keeps a tiny rendering surface in place until Bubble Tea lands.
type Stub struct{}

type BootstrapView struct {
	Layout            store.Layout
	ManagedEntrypoint string
	ObservedAdapters  []string
}

type ManagedLaunchView struct {
	Layout   store.Layout
	Snapshot contracts.SessionSnapshot
}

type DemoView struct {
	FixtureName string
	FixturePath string
	Sessions    []contracts.SessionSnapshot
}

func NewStub() *Stub {
	return &Stub{}
}

func (stub *Stub) RenderBootstrap(writer io.Writer, view BootstrapView) error {
	_, err := fmt.Fprintf(
		writer,
		"Tower bootstrap skeleton\n\nData root: %s\nDatabase : %s\nManaged  : %s\nObserved : %s\n\nBubble Tea, SQLite, and managed PTY wiring are intentionally left at the package boundary for the next implementation slice.\n",
		view.Layout.RootDir,
		view.Layout.DatabasePath,
		view.ManagedEntrypoint,
		joinOrNone(view.ObservedAdapters),
	)
	return err
}

func (stub *Stub) RenderManagedLaunch(writer io.Writer, view ManagedLaunchView) error {
	_, err := fmt.Fprintf(
		writer,
		"Managed launch registered\n\nSession  : %s\nRuntime  : %s\nAdapter  : %s (%s)\nWorkspace: %s\nLifecycle: %s\nData root: %s\n\nNext step: replace the bootstrap runtime manager with the PTY/ConPTY helper and terminal bridge defined in the engineering docs.\n",
		view.Snapshot.SessionID,
		view.Snapshot.RuntimeID,
		view.Snapshot.Adapter,
		view.Snapshot.Tool,
		view.Snapshot.WorkspaceRoot,
		view.Snapshot.Lifecycle,
		view.Layout.RootDir,
	)
	return err
}

func (stub *Stub) RenderDemo(writer io.Writer, view DemoView) error {
	managed := 0
	attention := 0
	for _, session := range view.Sessions {
		if session.ControlMode == contracts.ControlModeManaged {
			managed++
		}
		if session.Attention == contracts.AttentionNeedsUser || session.Attention == contracts.AttentionUrgent {
			attention++
		}
	}

	_, err := fmt.Fprintf(
		writer,
		"Loaded fixture: %s\nPath         : %s\nSessions     : %d\nManaged      : %d\nNeeds action : %d\n\nFixture replay is wired for the future Bubble Tea cockpit, but this bootstrap stops at parsing and summarizing the normalized session data.\n",
		view.FixtureName,
		view.FixturePath,
		len(view.Sessions),
		managed,
		attention,
	)
	return err
}

func joinOrNone(values []string) string {
	if len(values) == 0 {
		return "none"
	}

	result := values[0]
	for _, value := range values[1:] {
		result += ", " + value
	}
	return result
}
