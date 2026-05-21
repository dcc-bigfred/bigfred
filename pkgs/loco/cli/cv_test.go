package cli

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestParseArgsAsCVs_SimpleArgs(t *testing.T) {
	args := []string{"12", "34"}
	result, err := parseArgsAsCVs(args)
	assert.Equal(t, nil, err, "unexpected error")
	assert.Equal(t, "12 34", result, "result mismatch")
}

func TestParseArgsAsCVs_EmptyArgs(t *testing.T) {
	args := []string{}
	_, err := parseArgsAsCVs(args)
	assert.NotNil(t, err, "expected error for empty args")
}

func TestParseArgsAsCVs_Stdin(t *testing.T) {
	stdinContent := "cv1=161\ncv5\n"

	// mocking
	originalStdin := os.Stdin
	r, w, _ := os.Pipe()
	w.WriteString(stdinContent)
	w.Close()
	os.Stdin = r
	defer func() { os.Stdin = originalStdin }() // restore original after the test is done

	args := []string{"12", "-"}
	result, err := parseArgsAsCVs(args)
	assert.Equal(t, nil, err, "unexpected error")
	assert.Contains(t, result, "cv1=161", "expected stdin content in result")
	assert.Contains(t, result, "cv5", "expected stdin content in result")
}

func TestParseArgsAsCVs_IgnoreEmptyStrings(t *testing.T) {
	args := []string{"hell", "", "o"}
	result, err := parseArgsAsCVs(args)
	assert.Equal(t, nil, err, "unexpected error")
	assert.Equal(t, "hell o", result, "result mismatch")
}

func TestTrackOrDefault_ValidPom(t *testing.T) {
	track, err := TrackOrDefault("pom", 1)
	assert.Equal(t, nil, err, "unexpected error")
	assert.Equal(t, "pom", track, "track mismatch")
}

func TestTrackOrDefault_ValidProg(t *testing.T) {
	track, err := TrackOrDefault("prog", 1)
	assert.Equal(t, nil, err, "unexpected error")
	assert.Equal(t, "prog", track, "track mismatch")
}

func TestTrackOrDefault_InvalidTrack(t *testing.T) {
	_, err := TrackOrDefault("invalid", 1)
	assert.NotNil(t, err, "expected error for invalid track")
}

func TestTrackOrDefault_DefaultPom(t *testing.T) {
	track, err := TrackOrDefault("", 1)
	assert.Equal(t, nil, err, "unexpected error")
	assert.Equal(t, "pom", track, "track mismatch")
}

func TestTrackOrDefault_DefaultProg(t *testing.T) {
	track, err := TrackOrDefault("", 0)
	assert.Equal(t, nil, err, "unexpected error")
	assert.Equal(t, "prog", track, "track mismatch")
}
