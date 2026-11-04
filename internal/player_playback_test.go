package internal

import (
	"errors"
	"fmt"
	"os"
	"testing"
	"time"
)

func TestWaitForMPVPlaybackStartSkipsNonIPCPlayers(t *testing.T) {
	timeout := MpvPlaybackStartTimeoutDuration(nil)
	if !WaitForMPVPlaybackStart("android-intent", timeout) {
		t.Fatal("expected android-intent playback wait to succeed immediately")
	}
	if !WaitForMPVPlaybackStart("", timeout) {
		t.Fatal("expected empty socket playback wait to succeed immediately")
	}
}

func TestMpvPlaybackStartTimeoutDurationUsesConfig(t *testing.T) {
	config := PopulateConfig(map[string]string{
		"MpvPlaybackStartTimeout": "45",
	})
	if got := MpvPlaybackStartTimeoutDuration(&config); got != 45*time.Second {
		t.Fatalf("expected 45s timeout, got %s", got)
	}
}

func TestIsMPVConnectionGoneError(t *testing.T) {
	for _, message := range []string{
		"dial unix /tmp/curd: connect: connection refused",
		"dial unix /tmp/curd: connect: no such file or directory",
		"open \\\\.\\pipe\\curd: The system cannot find the file specified.",
		"read \\\\.\\pipe\\curd: The pipe has been ended.",
		"write \\\\.\\pipe\\curd: The pipe is being closed.",
		"read \\\\.\\pipe\\curd: No process is on the other end of the pipe.",
	} {
		if !isMPVConnectionGoneError(errors.New(message)) {
			t.Fatalf("expected gone error for %q", message)
		}
	}

	if isMPVConnectionGoneError(errors.New("i/o timeout")) {
		t.Fatal("did not expect timeout to be treated as closed connection")
	}
}

func TestIsMPVConnectionGoneErrorWrappedErrNotExist(t *testing.T) {
	// On Windows a closed/lost named pipe surfaces as ERROR_FILE_NOT_FOUND,
	// which errors.Is maps to os.ErrNotExist regardless of the OS display
	// language (e.g. Italian "Impossibile trovare il file specificato.").
	// MPVSendCommand wraps the dial error twice, so make sure the typed
	// check survives the wrapping.
	err := fmt.Errorf("command failed after 3 attempts: %w",
		fmt.Errorf("connect: %w",
			&os.PathError{Op: "open", Path: `\\.\pipe\curd_mpvsocket_8798b6ec`, Err: os.ErrNotExist}))
	if !isMPVConnectionGoneError(err) {
		t.Fatalf("expected wrapped os.ErrNotExist to be treated as connection gone: %v", err)
	}
}
