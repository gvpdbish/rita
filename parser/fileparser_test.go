package parser

import (
	"github.com/stretchr/testify/require"
	"testing"
)

func TestStripObjType(t *testing.T) {
	testCasesIn := []string{"http", "conn", "dns", "http_eth0", "conn_eth0", "http_abcd-efgh/ijkl"}
	testCasesOut := []string{"http", "conn", "dns", "http", "conn", "http"}
	for i := range testCasesIn {
		require.Equal(t, testCasesOut[i], stripObjType(testCasesIn[i]))
	}
}
