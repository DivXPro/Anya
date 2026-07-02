package applog

import (
	"fmt"
	"log"
	"os"
	"strings"
	"testing"
)

func TestInitWritesLogFileAndReadTail(t *testing.T) {
	t.Cleanup(func() {
		log.SetOutput(os.Stderr)
	})

	dir := t.TempDir()
	if err := Init(dir); err != nil {
		t.Fatalf("init: %v", err)
	}
	log.Print("first line")
	for i := 0; i < 20; i++ {
		log.Printf("line %02d", i)
	}

	info := GetInfo()
	if info.Path == "" || info.Size == 0 {
		t.Fatalf("unexpected info: %+v", info)
	}
	got, err := ReadTail(80)
	if err != nil {
		t.Fatalf("read tail: %v", err)
	}
	if !strings.Contains(got, "line 19") {
		t.Fatalf("tail did not include latest line: %q", got)
	}
}

func TestReadTailCapsLargeRequests(t *testing.T) {
	t.Cleanup(func() {
		log.SetOutput(os.Stderr)
	})

	dir := t.TempDir()
	if err := Init(dir); err != nil {
		t.Fatalf("init: %v", err)
	}
	for i := 0; i < 1000; i++ {
		log.Print(fmt.Sprintf("entry-%04d", i))
	}

	got, err := ReadTail(maxTailBytes + 1)
	if err != nil {
		t.Fatalf("read tail: %v", err)
	}
	if !strings.Contains(got, "entry-0999") {
		t.Fatalf("tail did not include latest entry")
	}
}
