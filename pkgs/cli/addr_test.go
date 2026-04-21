package cli

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestAddressToCVString_ShortAddress(t *testing.T) {
	cvString, err := addressToCVString(125)
	assert.NoError(t, err)
	assert.Equal(t, "cv1=125, cv17=0, cv18=0, cv29=0", cvString)
}

func TestAddressToCVString_LongAddressAboveShortRange(t *testing.T) {
	cvString, err := addressToCVString(178)
	assert.NoError(t, err)
	assert.Equal(t, "cv17=192, cv18=178, cv29=32", cvString)
}

func TestAddressToCVString_LongAddressUpperBoundary(t *testing.T) {
	cvString, err := addressToCVString(10239)
	assert.NoError(t, err)
	assert.Equal(t, "cv17=231, cv18=255, cv29=32", cvString)
}

func TestAddressToCVString_LongAddressZero(t *testing.T) {
	cvString, err := addressToCVString(0)
	assert.NoError(t, err)
	assert.Equal(t, "cv17=192, cv18=0, cv29=32", cvString)
}
