package character

import "testing"

func TestParseTraceLoginEvent(t *testing.T) {
	line := "[2026-04-26 11:48:57.116] GameChatManager::SetLoginSucceed true PlayerName[甄怼怼]"

	evt, ok := parseTraceLoginEvent(line)
	if !ok {
		t.Fatalf("expected trace line to be parsed")
	}
	if !evt.LoginSucceeded {
		t.Fatalf("expected LoginSucceeded=true")
	}
	if evt.PlayerName != "甄怼怼" {
		t.Fatalf("unexpected PlayerName: %q", evt.PlayerName)
	}
}

func TestParseTraceLoginEventFalse(t *testing.T) {
	line := "[2026-04-26 11:56:23.844] GameChatManager::SetLoginSucceed false PlayerName[甄怼怼]"

	evt, ok := parseTraceLoginEvent(line)
	if !ok {
		t.Fatalf("expected trace line to be parsed")
	}
	if evt.LoginSucceeded {
		t.Fatalf("expected LoginSucceeded=false")
	}
}

func TestParseTraceLoginEventIgnoreOtherLines(t *testing.T) {
	if _, ok := parseTraceLoginEvent("[2026-04-26] flow SERVER_ENTRY_SCENE begin"); ok {
		t.Fatalf("unexpected parse success for unrelated line")
	}
}
