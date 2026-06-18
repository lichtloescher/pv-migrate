package rsync

import (
	"strings"
	"testing"
)

func TestBuildBatchAppliesFlagsToEveryEntry(t *testing.T) {
	t.Parallel()

	c := &Cmd{
		Port:            2222,
		Delete:          true,
		DeleteAfter:     true,
		ExcludeSnapshot: true,
		NonRoot:         true,
		Compress:        true,
		SrcUseSSH:       true,
		SrcSSHHost:      "src-host",
		ExtraArgs:       "--iconv=utf8,iso88591",
	}

	entries := []BatchEntry{
		{SrcPath: "/src/a", DestPath: "/dst/a"},
		{SrcPath: "/src/b", DestPath: "/dst/b"},
		{SrcPath: "/src/c", DestPath: "/dst/c"},
	}

	out, err := c.BuildBatch(entries)
	if err != nil {
		t.Fatalf("BuildBatch returned error: %v", err)
	}

	segments := strings.Split(out, " && ")
	if len(segments) != len(entries) {
		t.Fatalf("expected %d rsync segments, got %d: %q", len(entries), len(segments), out)
	}

	wantFlags := []string{
		"--iconv=utf8,iso88591",
		"--delete-after",
		"--exclude='.snapshot'",
		"--omit-dir-times",
		"--delete",
		"-z",
	}

	for i, seg := range segments {
		for _, f := range wantFlags {
			if !strings.Contains(seg, f) {
				t.Errorf("segment %d missing %q: %q", i, f, seg)
			}
		}
	}
}

func TestBuildBatchAndBuildShareFlags(t *testing.T) {
	t.Parallel()

	base := Cmd{
		Delete:      true,
		DeleteAfter: true,
		Compress:    true,
		SrcUseSSH:   true,
		SrcSSHHost:  "src-host",
		ExtraArgs:   "--partial --inplace",
	}

	single := base
	single.SrcPath = "/src"
	single.DestPath = "/dst"

	singleOut, err := single.Build()
	if err != nil {
		t.Fatalf("Build returned error: %v", err)
	}

	batch := base
	batchOut, err := batch.BuildBatch([]BatchEntry{{SrcPath: "/src", DestPath: "/dst"}})
	if err != nil {
		t.Fatalf("BuildBatch returned error: %v", err)
	}

	if singleOut != batchOut {
		t.Errorf("single-entry batch should match Build():\n Build:      %q\n BuildBatch: %q", singleOut, batchOut)
	}
}
