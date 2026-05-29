package vaktvault

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestImportDotenv_ParsesKeyValues(t *testing.T) {
	content := `# comment
KEY1=value1
KEY2="quoted value"
KEY3='single quoted'
EMPTY=

INVALID_NO_EQUALS
`
	pairs := parseDotenv(content)
	assert.Equal(t, "value1", pairs["KEY1"])
	assert.Equal(t, "quoted value", pairs["KEY2"])
	assert.Equal(t, "single quoted", pairs["KEY3"])
	assert.Equal(t, "", pairs["EMPTY"])
	_, hasInvalid := pairs["INVALID_NO_EQUALS"]
	assert.False(t, hasInvalid)
}

func TestParseDotenv_IgnoresComments(t *testing.T) {
	content := `# this is a comment
## double hash

# KEY_IN_COMMENT=should_not_appear
REAL_KEY=real_value`
	pairs := parseDotenv(content)
	assert.Equal(t, 1, len(pairs))
	assert.Equal(t, "real_value", pairs["REAL_KEY"])
	_, inComment := pairs["KEY_IN_COMMENT"]
	assert.False(t, inComment)
}

func TestParseDotenv_ValueWithEquals(t *testing.T) {
	content := `DB_URL=postgres://user:pass@host/db?sslmode=disable`
	pairs := parseDotenv(content)
	assert.Equal(t, "postgres://user:pass@host/db?sslmode=disable", pairs["DB_URL"])
}
