package errors

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestErrorString(t *testing.T) {
	assert.Equal(t, "", ErrorString(nil))
	assert.Equal(t, "boom", ErrorString(errors.New("boom")))
}
