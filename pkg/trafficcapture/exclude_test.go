package trafficcapture

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestExcludePatternsMatchPathAndQuery(t *testing.T) {
	patterns, err := normalizeExcludePatterns([]string{
		"favicon.ico",
		"/static/*",
		"/api/v?.json",
		"Token=Secret",
	})
	require.NoError(t, err)
	matcher := compileExcludePatterns(patterns)

	require.True(t, matcher.matches("/favicon.ico", ""))
	require.True(t, matcher.matches("/assets/Favicon.ICO", ""))
	require.True(t, matcher.matches("/static/css/app.css", ""))
	require.True(t, matcher.matches("/api/v1.json", ""))
	require.True(t, matcher.matches("/callback", "token=secret&next=/"))
	require.False(t, matcher.matches("/api/v1/items.json", ""))
	require.False(t, matcher.matches("/healthz", ""))
}

func TestNormalizeExcludePatternsTrimsDeduplicatesAndLimits(t *testing.T) {
	patterns, err := normalizeExcludePatterns([]string{" favicon.ico ", "", "FAVICON.ICO", "/static/*"})
	require.NoError(t, err)
	require.Equal(t, []string{"favicon.ico", "/static/*"}, patterns)

	tooMany := make([]string, maxExcludePatterns+1)
	for i := range tooMany {
		tooMany[i] = fmt.Sprintf("rule-%d", i)
	}
	_, err = normalizeExcludePatterns(tooMany)
	require.ErrorContains(t, err, "最多支持")

	_, err = normalizeExcludePatterns([]string{string(make([]byte, maxExcludePatternSize+1))})
	require.ErrorContains(t, err, "不能超过")
}
